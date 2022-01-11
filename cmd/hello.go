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
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/archive"
	"github.com/spf13/cobra"
)

// helloCmd represents the hello command
var repo string
var helloCmd = &cobra.Command{
	Use:   "hello",
	Short: "Hello world! command",
	Long:  `Hello world! command`,
	Run: func(cmd *cobra.Command, args []string) {
		// pass, endPoint := getEcrPassword()
		// fmt.Println(pass, endPoint)
		// cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
		// if err != nil {
		// 	fmt.Println(err.Error())
		// 	return
		// }
		// imagePush(cli)
		svc := ecr.New(session.New(&aws.Config{
			Region: aws.String("us-east-1")}))
		input := &ecr.DescribeRepositoriesInput{}

		result, err := svc.DescribeRepositories(input)
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
			return
		}

		fmt.Println(result)
		//buildImage()
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

func getEcrPassword() (string, string) {
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
	creds := strings.Split(string(sEnc), ":")
	return creds[1], *result.AuthorizationData[0].ProxyEndpoint
}

var dockerRegistryUserID = ""

type ErrorLine struct {
	Error       string      `json:"error"`
	ErrorDetail ErrorDetail `json:"errorDetail"`
}

type ErrorDetail struct {
	Message string `json:"message"`
}

func buildImage() {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	err = imageBuild(cli)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
}

func imageBuild(dockerClient *client.Client) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*120)
	defer cancel()

	tar, err := archive.TarWithOptions("../node-hello/", &archive.TarOptions{})
	if err != nil {
		return err
	}

	opts := types.ImageBuildOptions{
		Dockerfile: "Dockerfile",
		Tags:       []string{"194428793924.dkr.ecr.us-east-1.amazonaws.com/prevue-test"},
		Remove:     true,
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

func imagePush(dockerClient *client.Client) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*120)
	defer cancel()

	var authConfig = types.AuthConfig{
		Username:      "AWS",
		Password:      "eyJwYXlsb2FkIjoidTUxTEppVzVtYnduNkN3bGVibWVBWTZMRnRDbm1WdjIzdzRzZjkrdkZVRmZ4UkRCbWNmbmxUYzAxd1JpR1p4WEZJTU5DczM4a05IeTRqYkdoamJwYmh6SlNmZ1NsRjJXTzJyTWNvTC8zQjU4bXg0V28yV2pobGtINDJXbTBzeHlpN3ROQ0dHb0lVYkZ3K1Fxbmh3UWVNejd5RDFBeS9sNVcvQXA3QWNlak91YmZkbDVTNDVYZlNKcGlBZmNlakN0NjJMYUpTejZ1WEdPTlE4YTIzcFM5QUFHc2pxMC9yejQyOUtGS2loVk0wcXRKSU5GS0xKVllPT2F6Qmo0dzhIeGpKQmZkMnJuWnIxeHp5RTZxZGdqd1BmcDlDUGZBaVQvbitnbmdpRjVmZWNScGpkKzQ1ZHM5MmtQcVV0dEVsN1ROVHJyamdDZHpueEp2QTJpNjltalk5eHVJUHA2TXExNXFtZTNNWi95dElZaDlWbUhNZzk3Nlk5NmozZG9rTTQ1N0lDMnh3RXlhWWFWck1ZTk9ldDRuVEJ0b21DZ2w1Y0Z3c2NWUFlpZWVKV2taTXROVllNZjFhaWdyeG93cEM1YnVmOG40MzV3M1UrVFJwZngxWDJ2enh4RDJSYlh6OWZhQ0lXYVk4OHdNTWtGdjVHS1RtbWliQ09QZ0JoRGxaMXVjZUxRT0dOVjVhRGJtZDloTytCN1RqeVdTWkJoWVd4clUyVUREQllIT0Z0YktYS2FiNEpna3ZjRjFKNVZQdFgvVmdHRlJTNkhhOWNvcENZN0Nub0RrWXlZazNNRVBUNmROMVJkVUUzYXMzVjFUSDVqVjJVelhvWTY2MXI0dkNjYkpwbE42bG0vc3d0aG5JN3V3S2U0Q3BUelpjeUVzdWdGQkh3Nzlvdmp0VlNHVDRCamJyWk9hZHo1M2YyeG1lNFgveWFyeFNyNlhnME4yWjlKQ0Z5TnZVSThxUEJuSjE5Q0FiNVpsVFpia3pVd2VkMFpaV3I1MmZDNTJMSHhzcXFaQWtPU2s1ZXBSV3RnVmVXd2lQWHRoZkorQ1htM1k2M3daVkUweVB0dVFBNlJndCtKbEF0TXhmYUpmbkltUmdpS1Z5S21JbGQwZS9FUmJyOW14K2EyQUd5azJNOFFnOFY3ZlFzMzBCTmlKTlZMTXRIOHlWd1lNMnBXZ3lBYUdLNlAzQmV2eDlCcDA5SGpmNE13YytEVGVtWmJBSnlmMEZQN1NjcjMwSjNYdHIxQTIxRjNkaHNRTFVVMjBJZ1JkV25nSEQ5TWgyR1RINnRQTWRqakk5Vm43NjUwKzh2WVRaMDV3OHo0amdObEJtT21tdWg1MG1qQ1IwVU5JR0tSVDZWeDRzLzRONWp4aGpTK2dzT1dXMkgrSEFocFNGaFdzWUVGWmc9PSIsImRhdGFrZXkiOiJBUUVCQUhod20wWWFJU0plUnRKbTVuMUc2dXFlZWtYdW9YWFBlNVVGY2U5UnE4LzE0d0FBQUg0d2ZBWUpLb1pJaHZjTkFRY0dvRzh3YlFJQkFEQm9CZ2txaGtpRzl3MEJCd0V3SGdZSllJWklBV1VEQkFFdU1CRUVES0ZlbmRSNXVKYUNnWDFvQ1FJQkVJQTdoUUNjTmFzejJ5S0ZwTFpJZWh4b2lsN2hDUGp5TFBRanR5aXhqYzFZR282c09WcGRCenBBTm8wZXpMUzNwckxMOCtoNEE4T3RBUklLRUFNPSIsInZlcnNpb24iOiIyIiwidHlwZSI6IkRBVEFfS0VZIiwiZXhwaXJhdGlvbiI6MTY0MTQyMDYzNH0=",
		ServerAddress: "194428793924.dkr.ecr.us-east-1.amazonaws.com",
	}

	authConfigBytes, _ := json.Marshal(authConfig)
	authConfigEncoded := base64.URLEncoding.EncodeToString(authConfigBytes)

	tag := "194428793924.dkr.ecr.us-east-1.amazonaws.com/prevue-test"
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
