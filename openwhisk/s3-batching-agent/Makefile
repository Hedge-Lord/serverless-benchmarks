# Makefile for S3 batching agent

# Variables
IMAGE_NAME := s3-batching-agent
IMAGE_TAG := latest
FULL_IMAGE_NAME := $(IMAGE_NAME):$(IMAGE_TAG)

# Go build variables
GOOS ?= linux
GOARCH ?= amd64
CGO_ENABLED ?= 0

# Kubernetes namespace
NAMESPACE := openwhisk

.PHONY: build docker-build deploy undeploy clean test run

# Build the Go binary
build:
	CGO_ENABLED=$(CGO_ENABLED) GOOS=$(GOOS) GOARCH=$(GOARCH) go build -o bin/agent ./cmd/agent

# Build Docker image
docker-build: build
	docker build -t $(FULL_IMAGE_NAME) .

# Load image to kind cluster (if using kind)
kind-load: docker-build
	kind load docker-image $(FULL_IMAGE_NAME)

# Deploy to Kubernetes
deploy:
	kubectl apply -f kubernetes/secret.yaml
	kubectl apply -f kubernetes/daemonset.yaml

# Remove from Kubernetes
undeploy:
	kubectl delete -f kubernetes/daemonset.yaml
	kubectl delete -f kubernetes/secret.yaml

# Run locally
run:
	go run ./cmd/agent/main.go --debug=true

# Test the agent
test:
	go test -v ./...

# Clean build artifacts
clean:
	rm -rf bin/

# View logs from all pods
logs:
	kubectl logs -l app=s3-batching-agent -n $(NAMESPACE) --tail=100 -f 