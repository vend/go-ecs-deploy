package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ecs"
)

type arrayFlag []string

func (flags *arrayFlag) String() string {
	return strings.Join(*flags, ",")
}

func (flags *arrayFlag) Set(value string) error {
	*flags = append(*flags, value)
	return nil
}

func (flags *arrayFlag) Specified() bool {
	return len(*flags) > 0
}

var (
	clusterName    = flag.String("c", "", "Cluster name to deploy to")
	repoName       = flag.String("i", "", "Container repo to pull from e.g. quay.io/username/reponame")
	environment    = flag.String("e", "", "Application environment, e.g. production")
	sha            = flag.String("s", "", "Tag, usually short git SHA to deploy")
	region         = flag.String("r", "", "AWS region")
	webhook        = flag.String("w", "", "Webhook (slack) URL to post to")
	targetImage    = flag.String("t", "", "Target image (overrides -s and -i)")
	preflightURL   = flag.String("p", "", "Preflight URL, if this url returns anything but 200 deploy is aborted")
	debug          = flag.Bool("d", false, "enable Debug output")
	multiContainer = flag.Bool("m", false, "Multicontainer service")
)

var channels arrayFlag
var apps arrayFlag

func fail(s string) {
	fmt.Printf(s)
	sendWebhooks(s)
	os.Exit(2)
}

type SlackMessage struct {
	Text     string  `json:"text"`
	Username string  `json:"username"`
	Channel  *string `json:"channel,omitempty"`
}

func sendWebhook(message string, url *string, channel *string) {
	json, _ := json.Marshal(SlackMessage{
		Text:     message,
		Username: "GO ECS Deploy",
		Channel:  channel,
	})
	reader := bytes.NewReader(json)
	http.Post(*url, "application/json", reader)
}

func sendWebhooks(message string) {
	if len(channels) > 0 {
		for _, channel := range channels {
			sendWebhook(message, webhook, &channel)
		}
	} else {
		sendWebhook(message, webhook, nil)
	}
}

func init() {
	flag.Var(&channels, "C", "Slack channels to post to (can be specified multiple times)")
	flag.Var(&apps, "a", "Application names (can be specified multiple times)")

}

func main() {
	flag.Parse()

	// First check is to the preflight URL
	if *preflightURL != "" {
		resp, err := http.Get(*preflightURL)
		if err != nil {
			fail(fmt.Sprintf("failed to check %s, received error %v", *preflightURL, err))
		}
		if resp.StatusCode != 200 {
			fail(fmt.Sprintf("failed to check %s, received status [%s] with headers %v", *preflightURL, resp.Status, resp.Header))
		}
	}

	if *clusterName == "" || !apps.Specified() || *environment == "" || *region == "" {
		flag.Usage()
		fail(fmt.Sprintf("Failed deployment of apps %s : missing parameters\n", apps))
	}

	if (*repoName == "" || *sha == "") && *targetImage == "" {
		flag.Usage()
		fail(fmt.Sprintf("Failed deployment %s : no repo name, sha or target image specified\n", apps))
	}

	// Take the first app specified and use it for creating the task definitions for all services.
	exemplarServiceName := apps[0] + "-" + *environment
	cfg := &aws.Config{
		Region: aws.String(*region),
	}
	if *debug {
		cfg = cfg.WithLogLevel(aws.LogDebug)
	}

	svc := ecs.New(session.New(), cfg)

	if *targetImage == "" {
		fmt.Printf("Request to deploy sha: %s to %s at %s \n", *sha, *environment, *region)
	} else {
		fmt.Printf("Request to deploy target image: %s to %s at %s \n", *targetImage, *environment, *region)
	}
	fmt.Printf("Describing services for cluster %s and service %s \n", *clusterName, exemplarServiceName)

	serviceDesc, err :=
		svc.DescribeServices(
			&ecs.DescribeServicesInput{
				Cluster:  clusterName,
				Services: []*string{&exemplarServiceName},
			})
	if err != nil {
		fail(fmt.Sprintf("Failed to describe %s \n`%s`", exemplarServiceName, err.Error()))
	}

	if len(serviceDesc.Services) < 1 {
		msg := fmt.Sprintf("No service %s found on cluster %s", exemplarServiceName, *clusterName)
		fail("Failed: " + msg)
	}

	service := serviceDesc.Services[0]
	if exemplarServiceName != *service.ServiceName {
		msg := fmt.Sprintf("Found the wrong service when looking for %s found %s \n", exemplarServiceName, *service.ServiceName)
		fail("Failed: " + msg)
	}

	fmt.Printf("Found existing ARN %s for service %s \n", *service.ClusterArn, *service.ServiceName)

	taskDesc, err :=
		svc.DescribeTaskDefinition(
			&ecs.DescribeTaskDefinitionInput{
				TaskDefinition: service.TaskDefinition})
	if err != nil {
		fail(fmt.Sprintf("Failed: deployment %s \n`%s`", exemplarServiceName, err.Error()))
	}

	if *debug {
		fmt.Printf("Current task description: \n%+v \n", taskDesc)
	}

	var containerDef *ecs.ContainerDefinition
	var oldImage *string
	// multiContainer service
	if *multiContainer {
		fmt.Printf("Task definition has multiple containers \n")
		var i int
		for i, containerDef = range taskDesc.TaskDefinition.ContainerDefinitions {
			oldImage = containerDef.Image
			x := *targetImage
			if *targetImage == "" {
				// Split repoName and Tag
				imageString := *oldImage
				pair := strings.Split(imageString, ":")
				if (*debug) {
					fmt.Printf("imageString: %s\n", imageString)
				}
				if len(pair) == 2 {
					fmt.Printf("Updating sha on repo: %s \n", pair[0])
					x = fmt.Sprintf("%s:%s", pair[0], *sha)
				} else {
					fmt.Printf("Using repo name passed in as argument: %s \n", *repoName)
					x = fmt.Sprintf("%s:%s", *repoName, *sha)
				}
			}
			containerDef.Image = &x
			taskDesc.TaskDefinition.ContainerDefinitions[i] = containerDef
		}
	} else {
		containerDef = taskDesc.TaskDefinition.ContainerDefinitions[0]
		oldImage = containerDef.Image
		x := *targetImage
		if *targetImage == "" {
			x = fmt.Sprintf("%s:%s", *repoName, *sha)
		}
		containerDef.Image = &x
	}

	futureDef := &ecs.RegisterTaskDefinitionInput{
		ContainerDefinitions:    taskDesc.TaskDefinition.ContainerDefinitions,
		Family:                  taskDesc.TaskDefinition.Family,
		Volumes:                 taskDesc.TaskDefinition.Volumes,
		NetworkMode:             taskDesc.TaskDefinition.NetworkMode,
		TaskRoleArn:             taskDesc.TaskDefinition.TaskRoleArn,
		Cpu:                     taskDesc.TaskDefinition.Cpu,
		Memory:                  taskDesc.TaskDefinition.Memory,
		RequiresCompatibilities: taskDesc.TaskDefinition.RequiresCompatibilities,
		ExecutionRoleArn:        taskDesc.TaskDefinition.ExecutionRoleArn,
	}

	if *debug {
		fmt.Printf("Future task description: \n%+v \n", futureDef)
	}

	registerRes, err :=
		svc.RegisterTaskDefinition(futureDef)
	if err != nil {
		fail(fmt.Sprintf("Failed: deployment %s for %s to %s \n`%s`", *containerDef.Image, exemplarServiceName, *clusterName, err.Error()))
	}

	newArn := registerRes.TaskDefinition.TaskDefinitionArn

	fmt.Printf("Registered new task for %s:%s \n", *sha, *newArn)

	// Get first container definition to create slack message
	containerDef = taskDesc.TaskDefinition.ContainerDefinitions[0]

	// update services to use new definition
	for _, appName := range apps {
		serviceName := appName + "-" + *environment

		_, err = svc.UpdateService(
			&ecs.UpdateServiceInput{
				Cluster:        clusterName,
				Service:        &serviceName,
				DesiredCount:   service.DesiredCount,
				TaskDefinition: newArn,
			})
		if err != nil {
			fail(fmt.Sprintf("Failed: deployment %s for %s to %s as %s \n`%s`", *containerDef.Image, appName, *clusterName, *newArn, err.Error()))
		}

		slackMsg := fmt.Sprintf("Deployed %s for *%s* to *%s* as `%s`", *containerDef.Image, appName, *clusterName, *newArn)

		// extract old image sha, and use it to generate a git compare URL
		if *oldImage != "" && *sha != "" {
			parts := strings.Split(*oldImage, ":")
			if len(parts) == 2 {
				// possibly a tagged image "def15c31-php5.5"
				parts = strings.Split(parts[1], "-")
				if gitURL, err := gitURL(parts[0], *sha); err == nil {
					slackMsg += " (<" + gitURL + "|diff>)"
				}
			}
		}
		sendWebhooks(slackMsg)

		fmt.Printf("Updated %s service to use new ARN: %s \n", serviceName, *newArn)
	}

}

// gitURL uses git since the program runs in many CI environments
func gitURL(startSHA string, endSHA string) (string, error) {
	var project string

	if travisSlug, ok := os.LookupEnv("TRAVIS_REPO_SLUG"); ok {
		project = travisSlug
	}

	if werckerOwner, ok := os.LookupEnv("WERCKER_GIT_OWNER"); ok {
		if werckerRepo, ok := os.LookupEnv("WERCKER_GIT_REPOSITORY"); ok {
			project = werckerOwner + "/" + werckerRepo
		}
	}

	if project == "" {
		return "", errors.New("nope")
	}

	url := "https://github.com/" + project + "/compare/" + startSHA + "..." + endSHA
	return url, nil
}
