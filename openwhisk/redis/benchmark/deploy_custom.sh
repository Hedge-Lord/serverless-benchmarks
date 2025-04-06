#!/bin/bash

set -e

# Configuration
ACTION_NAME="redis_benchmark"
PACKAGE_NAME="redis_benchmark"
DOCKER_IMAGE="redis-benchmark-custom:latest"
REGISTRY_HOST="localhost:5000"
REGISTRY_IMAGE="${REGISTRY_HOST}/${DOCKER_IMAGE}"

# Check if REDIS_HOST is provided
if [ -z "$REDIS_HOST" ]; then
  echo "Error: REDIS_HOST environment variable is required."
  echo "Please set it before running this script:"
  echo "  export REDIS_HOST=<redis-server-ip>"
  exit 1
fi

# Use default port if not provided
REDIS_PORT=${REDIS_PORT:-6379}
echo "Using Redis server at $REDIS_HOST:$REDIS_PORT"

echo "Building custom Docker image..."
cd "$(dirname "$0")/actions"
docker build -t ${DOCKER_IMAGE} -f Dockerfile.custom .

echo "Pushing image to registry..."
docker tag ${DOCKER_IMAGE} ${REGISTRY_IMAGE}
docker push ${REGISTRY_IMAGE}

# Create or update package
echo "Creating/updating package..."
wsk package update ${PACKAGE_NAME}

# Deploy the action
echo "Deploying Redis benchmark action using custom Docker image..."
wsk action update ${PACKAGE_NAME}/${ACTION_NAME} \
  --docker ${REGISTRY_IMAGE} \
  --memory 512 \
  --timeout 60000 \
  --web true \
  -p REDIS_HOST "$REDIS_HOST" \
  -p REDIS_PORT "$REDIS_PORT" \
  ${REDIS_PASSWORD:+-p REDIS_PASSWORD "$REDIS_PASSWORD"} \
  ${BATCHING_AGENT_HOST:+-p batching_agent_host "$BATCHING_AGENT_HOST"}

echo "Custom Docker-based Redis benchmark action has been deployed!"
echo "Invoke with: wsk action invoke ${PACKAGE_NAME}/${ACTION_NAME} -r -p num_ops 10 -p operation_type set -p use_batching false -p parallel_calls 5" 