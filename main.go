package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ecs"
)

var (
	clusterName = flag.String("c", "", "Cluster name to deploy to")
	repoName    = flag.String("i", "", "Container repo to pull from e.g. quay.io/username/reponame")
	appName     = flag.String("a", "", "Application name")
	environment = flag.String("e", "", "Application environment, e.g. production")
	sha         = flag.String("s", "", "Tag, usually short git SHA to deploy")
	region      = flag.String("r", "", "AWS region")
	webhook     = flag.String("w", "", "Webhook (slack) to post to")
	debug       = flag.Bool("d", false, "enable Debug output")
)

func fail(s string) {
	fmt.Printf(s)
	webhookFunc(s)
	os.Exit(2)
}

func webhookFunc(s string) {
	if *webhook == "" {
		return
	}

	json, _ := json.Marshal(
		struct {
			Text     string `json:"text"`
			Username string `json:"username"`
		}{
			s,
			"go-ecs-deploy",
		},
	)

	reader := bytes.NewReader(json)
	http.Post(*webhook, "application/json", reader)
}

func main() {
	flag.Parse()

	if *clusterName == "" || *repoName == "" || *appName == "" || *environment == "" || *sha == "" || *region == "" {
		flag.Usage()
		fail(fmt.Sprintf("Failed deployment %s \n`bad paramaters`", *appName))
	}

	serviceName := *appName + "-" + *environment

	svc := ecs.New(session.New(), &aws.Config{Region: aws.String(*region)})

	fmt.Printf("Request to deploy sha: %s to %s at %s \n", *sha, *environment, *region)
	fmt.Printf("Describing services for cluster %s and service %s \n", *clusterName, serviceName)

	serviceDesc, err :=
		svc.DescribeServices(
			&ecs.DescribeServicesInput{
				Cluster:  clusterName,
				Services: []*string{&serviceName},
			})
	if err != nil {
		fail(fmt.Sprintf("Failed: deployment %s \n`%s`", *appName, err.Error()))
	}

	if len(serviceDesc.Services) < 1 {
		msg := fmt.Sprintf("No service %s found on cluster %s", serviceName, *clusterName)
		fail("Failed: " + msg)
	}

	service := serviceDesc.Services[0]
	if serviceName != *service.ServiceName {
		msg := fmt.Sprintf("Found the wrong service when looking for %s found %s \n", serviceName, *service.ServiceName)
		fail("Failed: " + msg)
	}

	fmt.Printf("Found existing ARN %s for service %s \n", *service.ClusterArn, *service.ServiceName)

	taskDesc, err :=
		svc.DescribeTaskDefinition(
			&ecs.DescribeTaskDefinitionInput{
				TaskDefinition: service.TaskDefinition})
	if err != nil {
		fail(fmt.Sprintf("Failed: deployment %s \n`%s`", *appName, err.Error()))
	}

	if *debug {
		fmt.Printf("Current task description: \n%+v \n", taskDesc)
	}

	containerDef := taskDesc.TaskDefinition.ContainerDefinitions[0]
	{
		x := fmt.Sprintf("%s:%s", *repoName, *sha)
		containerDef.Image = &x
	}

	futureDef := &ecs.RegisterTaskDefinitionInput{
		ContainerDefinitions: taskDesc.TaskDefinition.ContainerDefinitions,
		Family:               appName,
		Volumes:              taskDesc.TaskDefinition.Volumes,
		NetworkMode:          taskDesc.TaskDefinition.NetworkMode,
		TaskRoleArn:          taskDesc.TaskDefinition.TaskRoleArn,
	}

	if *debug {
		fmt.Printf("Future task description: \n%+v \n", futureDef)
	}

	registerRes, err :=
		svc.RegisterTaskDefinition(futureDef)
	if err != nil {
		fail(fmt.Sprintf("Failed: deployment %s for %s to %s \n`%s`", *containerDef.Image, *appName, *clusterName, err.Error()))
	}

	newArn := registerRes.TaskDefinition.TaskDefinitionArn

	fmt.Printf("Registered new task for %s:%s \n", *sha, *newArn)

	// update service to use new definition
	_, err = svc.UpdateService(
		&ecs.UpdateServiceInput{
			Cluster:        clusterName,
			Service:        &serviceName,
			DesiredCount:   service.DesiredCount,
			TaskDefinition: newArn,
		})
	if err != nil {
		fail(fmt.Sprintf("Failed: deployment %s for %s to %s as %s \n`%s`", *containerDef.Image, *appName, *clusterName, *newArn, err.Error()))
	}

	webhookFunc(fmt.Sprintf("Deployed %s for %s to %s as %s", *containerDef.Image, *appName, *clusterName, *newArn))

	fmt.Printf("Updated %s service to use new ARN: %s \n", serviceName, *newArn)

}
