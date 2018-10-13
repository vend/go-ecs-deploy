
DIR := ${CURDIR}
GO_VERSION := "1.9.4"

#if make is typed with no further arguments, then show a list of available targets
default:
	@awk -F\: '/^[a-z_]+:/ && !/default/ {printf "- %-20s %s\n", $$1, $$2}' Makefile

help:
	@echo ""
	@echo "make compose_upd: start all containers"
	@echo "make stop [CONTAINER]: stop [CONTAINER]"
	@echo "make start [CONTAINER]: start [CONTAINER]"

bootstrap:
	pip install --user awscli
	mkdir -p tmp/bin
	curl -O https://dl.google.com/go/go1.9.4.linux-amd64.tar.gz
	tar -xf go1.9.4.linux-amd64.tar.gz -C tmp/bin

build:
	CGO_ENABLED=0 go build

