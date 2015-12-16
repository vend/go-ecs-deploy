# go-ecs-deploy
deploy a hosted docker container to an existing ecs cluster

Allows deployment of ECS microservices straight from the command line!

## Installation

    $ go get github.com/vend/go-ecs-deploy

## Usage

You need:
- Valid AWS credentials for the place you're deploying to (in your ENV)
- An existing task definition - this won't create one for you

Run the command:

```
AWS_PROFILE=production go-ecs-deploy -c vend-production -a authome -i quay.io/username/reponame -e production -s 5304a1b -r us-west-2
```