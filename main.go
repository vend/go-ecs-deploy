package main

import (
	"flag"
	"fmt"
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
)

func main() {
	flag.Parse()

	if *clusterName == "" || *repoName == "" || *appName == "" || *environment == "" || *sha == "" || *region == "" {
		flag.Usage()
		os.Exit(-1)
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
		println(err.Error())
		os.Exit(2)
	}

	if len(serviceDesc.Services) < 1 {
		fmt.Printf("No service %s found on cluster %s\n", serviceName, *clusterName)
		os.Exit(2)
	}

	service := serviceDesc.Services[0]
	if serviceName != *service.ServiceName {
		fmt.Printf("Found the wrong service when looking for %s found %s \n", serviceName, *service.ServiceName)
		os.Exit(2)
	}

	fmt.Printf("Found existing ARN %s for service %s \n", *service.ClusterArn, *service.ServiceName)

	taskDesc, err :=
		svc.DescribeTaskDefinition(
			&ecs.DescribeTaskDefinitionInput{
				TaskDefinition: service.TaskDefinition})
	if err != nil {
		println(err.Error())
		os.Exit(2)
	}

	containerDef := taskDesc.TaskDefinition.ContainerDefinitions[0]
	{
		x := fmt.Sprintf("%s:%s", *repoName, *sha)
		containerDef.Image = &x
	}

	registerRes, err :=
		svc.RegisterTaskDefinition(
			&ecs.RegisterTaskDefinitionInput{
				ContainerDefinitions: taskDesc.TaskDefinition.ContainerDefinitions,
				Family:               appName,
				NetworkMode:          taskDesc.TaskDefinition.NetworkMode,
				TaskRoleArn:          taskDesc.TaskDefinition.TaskRoleArn,
			})
	if err != nil {
		println(err.Error())
		os.Exit(2)
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
		println(err.Error())
		os.Exit(2)
	}

	fmt.Printf("Updated %s service to use new ARN: %s \n", serviceName, *newArn)

}
