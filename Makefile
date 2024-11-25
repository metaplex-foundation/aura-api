#colors:
B = \033[1;94m#   BLUE
G = \033[1;92m#   GREEN
Y = \033[1;93m#   YELLOW
R = \033[1;31m#   RED
M = \033[1;95m#   MAGENTA
C = \033[1;96m#   CYAN
K = \033[K#       ERASE END OF LINE
D = \033[0m#      DEFAULT
A = \007#         BEEP

.PHONY: format build image push clean start dev stop test mocks lint clickhouse swagger

SHELL := /bin/bash

REGISTRY=ghcr.io/adm-metaex/aura-api
APP=$(shell basename -s .git $(shell git remote get-url origin))
VERSION=$(shell git describe --tags --abbrev=0)-$(shell git rev-parse --short HEAD)
TARGETARCH=amd64
TARGETOS=linux
BASEPATH = ${REGISTRY}/${APP}:${VERSION}

format: 
	gofmt -s -w ./
	@echo -e "${G}Code formatted successfully!${D}"

build: format
	@echo -e "${M}Starting build for application: ${APP}${D}"
	@echo -e "${C}Build details:${D}"
	@echo -e "${C}- Application: ${APP}${D}"
	@echo -e "${C}- Registry: ${REGISTRY}${D}"
	@echo -e "${C}- Version (tag): ${VERSION}${D}"
	CGO_ENABLED=0 GOOS=${TARGETOS} go build -a -v -installsuffix cgo ./cmd/api
	@echo -e "${G}Application built successfully!${D}"

image:
	@echo -e "${M}Building Docker image for application: ${APP}${D}"
	@echo -e "${C}Image details:${D}"
	@echo -e "${C}- Version: ${VERSION}${D}"
	@echo -e "${C}- OS: ${TARGETOS}${D}"
	@echo -e "${C}- Architecture: ${TARGETARCH}${D}"
	docker build . -t ${BASEPATH} --build-arg TARGETOS=${TARGETOS} --build-arg TARGETARCH=${TARGETARCH}
	@echo -e "${G}Docker image built successfully!${D}"

push:
	@echo -e "${M}Pushing Docker image to the registry: ${BASEPATH}${D}"
	@docker push ${BASEPATH} || { echo -e "${R}Error: Failed to push the image.${D}"; exit 1; }
	@echo -e "${G}Image pushed successfully!${D}"

clean:
	@echo -e "${M}Cleaning up Docker images created for ${APP} version ${VERSION}...${D}"
	@if docker images ${BASEPATH} -q | grep -q '.' ; then \
		echo -e "${C}Removing Docker image ${BASEPATH}${D}"; \
		docker rmi ${BASEPATH} || { echo -e "${R}Error: Failed to remove the image.${D}"; exit 1; } \
	else \
		echo -e "${R}No Docker image found for ${BASEPATH}${D}"; \
	fi
	@echo -e "${G}Cleanup completed for ${APP} version ${VERSION}!${D}"

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
	protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative pkg/proto/aura.proto

swagger:
	@swag init -g internal/api/api.go --generatedTime --output internal/api/docs
	@swag fmt