.PHONY: build start dev stop test mocks lint clickhouse swagger

SHELL := /bin/bash

export APP_NAME=aura-go
export APP_VERSION=latest

build:
	@echo "building ${APP_NAME} with version ${APP_VERSION}"
	@echo "building docker image ${APP_IMAGE}"
	@docker build -f Dockerfile --ssh default . -t ${APP_NAME}:${APP_VERSION}

start:
	@docker-compose up -d

dev: clickhouse
	@docker-compose -f docker-compose-postgres.yml up -d postgres

clickhouse:
	@docker-compose -f docker-compose-clickhouse.yml up -d clickhouse

stop:
	@docker-compose stop

test:
	@go test -v ./...

mocks:
	# go install github.com/golang/mock/mockgen@v1.6.0
	@go generate ./...
lint:
	# go get -u github.com/golangci/golangci-lint/cmd/golangci-lint
	@golangci-lint run

proto:
	protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative internal/pkg/proto/aura.proto

swagger:
	@swag init -g internal/api/api.go --generatedTime --output internal/api/docs
	@swag fmt