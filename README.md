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
        Application name (can be specified multiple times)
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
  -m enable multi container deploy
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

Multi container task definition with side car container pushed by the same repo

```sh

av privileged ./go-ecs-deploy \
    -e production \
    -a "corp-blog" \
    -i "542640492856.dkr.ecr.us-west-2.amazonaws.com/corp-blog-nginx" \
    -c corporate-production \
    -r us-west-2 \
    -s c2b10cc \
    -d=true \
    -m=true

```

## Development

To update dependencies, open up `glide.yaml` and update the `version:` field for
the relevant package(s).

Then run `glide up`

To build `go-ecs-deploy` locally simply run `make build`.

