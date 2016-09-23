# go-ecs-deploy [![Build Status](https://travis-ci.org/vend/go-ecs-deploy.svg)](https://travis-ci.org/vend/go-ecs-deploy)
deploy a hosted docker container to an existing ecs cluster

Allows deployment of ECS microservices straight from the command line!

## Installation

```
go get github.com/vend/go-ecs-deploy
```

## Requirements

You need:

- Valid AWS credentials for the place you're deploying to (in your ENV)
- An existing task definition - this won't create one for you

## Usage

The full list of options is:

```
Usage of ./go-ecs-deploy:
  -C value
        Slack channels to post to (can be specified multiple times)
  -a string
        Application name
  -c string
        Cluster name to deploy to
  -d    enable Debug output
  -e string
        Application environment, e.g. production
  -i string
        Container repo to pull from e.g. quay.io/username/reponame
  -r string
        AWS region
  -s string
        Tag, usually short git SHA to deploy
  -t string
        Target image (overrides -s and -i)
  -w string
        Webhook (slack) URL to post to
```

### Example

```
AWS_PROFILE=production go-ecs-deploy \
  -c vend-production \
  -a authome \
  -i quay.io/username/reponame \
  -e production \
  -s 5304a1b \
  -r us-west-2
```

## Development

To update dependencies, open up `glide.yaml` and update the `version:` field for
the relevant package(s).

Then run `glide up`
