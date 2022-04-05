/*
Copyright © 2021 NAME HERE <EMAIL ADDRESS>

*/
package cmd

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ecr"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/archive"
	"github.com/spf13/cobra"
)

// helloCmd represents the hello command
var helloCmd = &cobra.Command{
	Use:   "hello",
	Short: "Hello world! command",
	Long:  `Hello world! command`,
	Run: func(cmd *cobra.Command, args []string) {

		ecrPassword, ecrServerAddress, err := getEcrPassword()

		if err != nil {
			fmt.Println(err)
			return
		}

		// This takes in the server address for our ecr which we get from previous command, the ecr repository name,
		// I also have afunction to create ECR repo, did not include it in the flow,
		// Lastly this takes the path of the folder where your docker file is localted.
		err = buildImage(ecrServerAddress, "prevue-test", "../Refresh/refresh-ui")

		if err != nil {
			fmt.Println(err)
			return
		}

		// Takes in the password and ecr server addrss from previous command and the ecr repository name
		err = pushImage(ecrPassword, ecrServerAddress, "prevue-test")

		if err != nil {
			fmt.Println(err)
			return
		}

		// Takes in the family name fo the task and returns its defintion, if no tag is provided takes in the latest

		describeTaskOutput, err := describeTask("first-run-task-definition")

		if err != nil {
			fmt.Println(err)
			return
		}

		// Takes in the image you want to deploy and the output recieved fro the describe task definition

		_, err = registerTask(ecrServerAddress+"/"+"prevue-test", describeTaskOutput)

		if err != nil {
			fmt.Println(err)
			return
		}

		// Takes in the cluster name, the service name and the task definition family.

		_, err = updateService("prevue", "prevue-service", "first-run-task-definition")

		if err != nil {
			fmt.Println(err)
			return
		}
	},
}

func init() {
	rootCmd.AddCommand(helloCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// helloCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
}

func getEcrPassword() (string, string, error) {
	svc := ecr.New(session.New(&aws.Config{
		Region: aws.String("us-east-1")}))
	input := &ecr.GetAuthorizationTokenInput{}

	result, err := svc.GetAuthorizationToken(input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case ecr.ErrCodeServerException:
				fmt.Println(ecr.ErrCodeServerException, aerr.Error())
			case ecr.ErrCodeInvalidParameterException:
				fmt.Println(ecr.ErrCodeInvalidParameterException, aerr.Error())
			default:
				fmt.Println(aerr.Error())
			}
		} else {
			// Print the error, cast err to awserr.Error to get the Code and
			// Message from an error.
			fmt.Println(err.Error())
		}
	}

	sEnc, err := base64.StdEncoding.DecodeString(*result.AuthorizationData[0].AuthorizationToken)

	if err != nil {
		return "", "", err
	}
	creds := strings.Split(string(sEnc), ":")
	endpoint := *result.AuthorizationData[0].ProxyEndpoint
	endpoint = endpoint[8:] // removing the "https://" from endpoint
	return creds[1], endpoint, nil
}

type ErrorLine struct {
	Error       string      `json:"error"`
	ErrorDetail ErrorDetail `json:"errorDetail"`
}

type ErrorDetail struct {
	Message string `json:"message"`
}

func buildImage(serverAddress, repositoryName, targetPath string) error {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return err
	}

	err = imageBuild(cli, serverAddress, repositoryName, targetPath)
	if err != nil {
		return err
	}
	return nil
}

func pushImage(password, serverAddress, repositoryName string) error {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return err
	}

	err = imagePush(cli, password, serverAddress, repositoryName)
	if err != nil {
		return err
	}
	return nil
}

func imageBuild(dockerClient *client.Client, serverAddress string, repositoryName string, targetPath string) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*120000)
	defer cancel()

	tar, err := archive.TarWithOptions(targetPath, &archive.TarOptions{})
	if err != nil {
		return err
	}

	opts := types.ImageBuildOptions{
		Dockerfile: "Dockerfile",
		//Tags:       []string{"809789629378.dkr.ecr.us-east-1.amazonaws.com/prevue-test"},
		Tags:   []string{serverAddress + "/" + repositoryName},
		Remove: true,
	}
	res, err := dockerClient.ImageBuild(ctx, tar, opts)
	if err != nil {
		return err
	}

	defer res.Body.Close()

	err = print(res.Body)
	if err != nil {
		return err
	}

	return nil
}

func print(rd io.Reader) error {
	var lastLine string

	scanner := bufio.NewScanner(rd)
	for scanner.Scan() {
		lastLine = scanner.Text()
		fmt.Println(scanner.Text())
	}

	errLine := &ErrorLine{}
	json.Unmarshal([]byte(lastLine), errLine)
	if errLine.Error != "" {
		return errors.New(errLine.Error)
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	return nil
}

func imagePush(dockerClient *client.Client, password string, serverAddress string, repositoryName string) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*12000)
	defer cancel()

	var authConfig = types.AuthConfig{
		Username: "AWS",
		Password: password,
		//ServerAddress: "809789629378.dkr.ecr.us-east-1.amazonaws.com",
		ServerAddress: serverAddress,
	}

	authConfigBytes, _ := json.Marshal(authConfig)
	authConfigEncoded := base64.URLEncoding.EncodeToString(authConfigBytes)

	//tag := "809789629378.dkr.ecr.us-east-1.amazonaws.com/prevue-test"
	tag := serverAddress + "/" + repositoryName
	opts := types.ImagePushOptions{RegistryAuth: authConfigEncoded}
	rd, err := dockerClient.ImagePush(ctx, tag, opts)
	if err != nil {
		return err
	}

	defer rd.Close()

	err = print(rd)
	if err != nil {
		return err
	}

	return nil
}

func registerTask(image string, inp *ecs.DescribeTaskDefinitionOutput) (*ecs.RegisterTaskDefinitionOutput, error) {

	svc := ecs.New(session.New(&aws.Config{
		Region: aws.String("us-east-1")}))

	containerDef := inp.TaskDefinition.ContainerDefinitions
	containerDef[0].Image = aws.String(image)
	input := &ecs.RegisterTaskDefinitionInput{
		ContainerDefinitions:    inp.TaskDefinition.ContainerDefinitions,
		Family:                  aws.String(*inp.TaskDefinition.Family),
		ExecutionRoleArn:        aws.String(*inp.TaskDefinition.ExecutionRoleArn),
		Cpu:                     aws.String(*inp.TaskDefinition.Cpu),
		Memory:                  aws.String(*inp.TaskDefinition.Memory),
		NetworkMode:             aws.String(*inp.TaskDefinition.NetworkMode),
		RequiresCompatibilities: *&inp.TaskDefinition.RequiresCompatibilities,

		//need to see other fields that can be added here or how do they work
	}

	var result *ecs.RegisterTaskDefinitionOutput
	result, err := svc.RegisterTaskDefinition(input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case ecs.ErrCodeServerException:
				fmt.Println(ecs.ErrCodeServerException, aerr.Error())
			case ecs.ErrCodeClientException:
				fmt.Println(ecs.ErrCodeClientException, aerr.Error())
			case ecs.ErrCodeInvalidParameterException:
				fmt.Println(ecs.ErrCodeInvalidParameterException, aerr.Error())
			default:
				fmt.Println(aerr.Error())
			}
		} else {
			// Print the error, cast err to awserr.Error to get the Code and
			// Message from an error.
			fmt.Println(err.Error())
		}
		return result, err
	}

	return result, nil
}

func describeTask(taskDefinitionFamily string) (*ecs.DescribeTaskDefinitionOutput, error) {
	svc := ecs.New(session.New(&aws.Config{
		Region: aws.String("us-east-1")}))
	input := &ecs.DescribeTaskDefinitionInput{
		TaskDefinition: aws.String(taskDefinitionFamily),
	}
	var result *ecs.DescribeTaskDefinitionOutput
	result, err := svc.DescribeTaskDefinition(input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case ecs.ErrCodeServerException:
				fmt.Println(ecs.ErrCodeServerException, aerr.Error())
			case ecs.ErrCodeClientException:
				fmt.Println(ecs.ErrCodeClientException, aerr.Error())
			case ecs.ErrCodeInvalidParameterException:
				fmt.Println(ecs.ErrCodeInvalidParameterException, aerr.Error())
			default:
				fmt.Println(aerr.Error())
			}
		} else {
			// Print the error, cast err to awserr.Error to get the Code and
			// Message from an error.
			fmt.Println(err.Error())
			return result, err
		}
	}

	return result, nil
}

func updateService(clusterName, serviceName, taskDefinitionFamily string) (*ecs.UpdateServiceOutput, error) {
	svc := ecs.New(session.New(&aws.Config{
		Region: aws.String("us-east-1")}))
	input := &ecs.UpdateServiceInput{
		Cluster:            aws.String(clusterName),
		Service:            aws.String(serviceName),
		TaskDefinition:     aws.String(taskDefinitionFamily),
		ForceNewDeployment: aws.Bool(true),
	}
	var result *ecs.UpdateServiceOutput
	result, err := svc.UpdateService(input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case ecs.ErrCodeServerException:
				fmt.Println(ecs.ErrCodeServerException, aerr.Error())
			case ecs.ErrCodeClientException:
				fmt.Println(ecs.ErrCodeClientException, aerr.Error())
			case ecs.ErrCodeInvalidParameterException:
				fmt.Println(ecs.ErrCodeInvalidParameterException, aerr.Error())
			case ecs.ErrCodeClusterNotFoundException:
				fmt.Println(ecs.ErrCodeClusterNotFoundException, aerr.Error())
			case ecs.ErrCodeServiceNotFoundException:
				fmt.Println(ecs.ErrCodeServiceNotFoundException, aerr.Error())
			case ecs.ErrCodeServiceNotActiveException:
				fmt.Println(ecs.ErrCodeServiceNotActiveException, aerr.Error())
			case ecs.ErrCodePlatformUnknownException:
				fmt.Println(ecs.ErrCodePlatformUnknownException, aerr.Error())
			case ecs.ErrCodePlatformTaskDefinitionIncompatibilityException:
				fmt.Println(ecs.ErrCodePlatformTaskDefinitionIncompatibilityException, aerr.Error())
			case ecs.ErrCodeAccessDeniedException:
				fmt.Println(ecs.ErrCodeAccessDeniedException, aerr.Error())
			default:
				fmt.Println(aerr.Error())
			}
		} else {
			// Print the error, cast err to awserr.Error to get the Code and
			// Message from an error.
			fmt.Println(err.Error())
		}
		return result, err
	}

	return result, nil
}

func DescribeRepositories() (*ecr.DescribeRepositoriesOutput, error) {
	svc := ecr.New(session.New(&aws.Config{
		Region: aws.String("us-east-1")}))
	input := &ecr.DescribeRepositoriesInput{}
	var result *ecr.DescribeRepositoriesOutput
	_, err := svc.DescribeRepositories(input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case ecr.ErrCodeServerException:
				fmt.Println(ecr.ErrCodeServerException, aerr.Error())
			case ecr.ErrCodeInvalidParameterException:
				fmt.Println(ecr.ErrCodeInvalidParameterException, aerr.Error())
			case ecr.ErrCodeRepositoryNotFoundException:
				fmt.Println(ecr.ErrCodeRepositoryNotFoundException, aerr.Error())
			default:
				fmt.Println(aerr.Error())
			}
		} else {
			// Print the error, cast err to awserr.Error to get the Code and
			// Message from an error.
			fmt.Println(err.Error())
		}
		return result, err
	}

	return result, nil
}

func CreateRepository(name string) (*ecr.CreateRepositoryOutput, error) {
	svc := ecr.New(session.New(&aws.Config{
		Region: aws.String("us-east-1")}))
	input := &ecr.CreateRepositoryInput{
		RepositoryName: aws.String(name),
	}
	var result *ecr.CreateRepositoryOutput
	result, err := svc.CreateRepository(input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case ecr.ErrCodeServerException:
				fmt.Println(ecr.ErrCodeServerException, aerr.Error())
			case ecr.ErrCodeInvalidParameterException:
				fmt.Println(ecr.ErrCodeInvalidParameterException, aerr.Error())
			case ecr.ErrCodeInvalidTagParameterException:
				fmt.Println(ecr.ErrCodeInvalidTagParameterException, aerr.Error())
			case ecr.ErrCodeTooManyTagsException:
				fmt.Println(ecr.ErrCodeTooManyTagsException, aerr.Error())
			case ecr.ErrCodeRepositoryAlreadyExistsException:
				fmt.Println(ecr.ErrCodeRepositoryAlreadyExistsException, aerr.Error())
			case ecr.ErrCodeLimitExceededException:
				fmt.Println(ecr.ErrCodeLimitExceededException, aerr.Error())
			case ecr.ErrCodeKmsException:
				fmt.Println(ecr.ErrCodeKmsException, aerr.Error())
			default:
				fmt.Println(aerr.Error())
			}
		} else {
			// Print the error, cast err to awserr.Error to get the Code and
			// Message from an error.
			fmt.Println(err.Error())
		}
		return result, err
	}
	return result, nil
}
