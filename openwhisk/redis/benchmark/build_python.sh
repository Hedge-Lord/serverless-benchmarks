#!/bin/bash

set -e

# Configuration variables
DOCKER_IMAGE="redis-benchmark-python:latest"
REGISTRY_HOST="localhost:5000"
REGISTRY_IMAGE="${REGISTRY_HOST}/${DOCKER_IMAGE}"

# Navigate to the action directory
cd "$(dirname "$0")/actions"

echo "Building Python Redis benchmark Docker image..."

# Build the Docker image
docker build -t "${DOCKER_IMAGE}" -f Dockerfile.python .

# Tag the image for the local registry
docker tag "${DOCKER_IMAGE}" "${REGISTRY_IMAGE}"

# Push to local registry
echo "Pushing image to local registry at ${REGISTRY_HOST}..."
docker push "${REGISTRY_IMAGE}"

echo "Done! Python Redis benchmark image built and pushed to registry."
echo "Image: ${REGISTRY_IMAGE}" 