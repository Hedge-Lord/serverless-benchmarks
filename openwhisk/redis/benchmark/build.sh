#!/bin/bash

set -e

# Configuration variables
DOCKER_IMAGE="redis-benchmark:latest"
REGISTRY_HOST="localhost:5000"
REGISTRY_IMAGE="${REGISTRY_HOST}/${DOCKER_IMAGE}"

# Navigate to the actions directory
cd "$(dirname "$0")/actions"

# Build the Docker image
echo "Building Docker image..."
if ! docker build --no-cache -t ${DOCKER_IMAGE} .; then
  echo "Docker build failed. Please check the error messages above."
  exit 1
fi

# Tag and push to local registry
echo "Pushing to local registry..."
if ! docker tag ${DOCKER_IMAGE} ${REGISTRY_IMAGE}; then
  echo "Failed to tag the Docker image."
  exit 1
fi

if ! docker push ${REGISTRY_IMAGE}; then
  echo "Failed to push to local registry. Is the registry running?"
  echo "You can start a local registry with: docker run -d -p 5000:5000 --name registry registry:2"
  exit 1
fi

echo "Build and push completed successfully!"
echo "The image ${REGISTRY_IMAGE} is now available for deployment."
echo "To deploy the action, run deploy.sh on the master node with kubectl access." 