#!/bin/bash

set -e

# Configuration variables
ACTION_NAME="redis_benchmark"
PACKAGE_NAME="redis_benchmark"
DOCKER_IMAGE="redis-benchmark:latest"
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

# Navigate to the actions directory
cd "$(dirname "$0")/actions"

# Build the Docker image
echo "Building Docker image..."
docker build -t ${DOCKER_IMAGE} .

# Tag and push to local registry
echo "Pushing to local registry..."
docker tag ${DOCKER_IMAGE} ${REGISTRY_IMAGE}
docker push ${REGISTRY_IMAGE}

# Create or update package
echo "Creating/updating package..."
wsk package update ${PACKAGE_NAME}

# Deploy the action
echo "Deploying action..."
wsk action update ${PACKAGE_NAME}/${ACTION_NAME} \
  --docker ${REGISTRY_IMAGE} \
  --memory 512 \
  --timeout 60000 \
  --web true \
  -p REDIS_HOST "$REDIS_HOST" \
  -p REDIS_PORT "$REDIS_PORT" \
  ${REDIS_PASSWORD:+-p REDIS_PASSWORD "$REDIS_PASSWORD"}

# Get the action URL
URL=$(wsk action get ${PACKAGE_NAME}/${ACTION_NAME} --url | tail -n1)
AUTH=$(wsk property get --auth | awk '{print $3}')
WEB_URL="${URL}.json?blocking=true"

echo "
Action has been deployed!

You can invoke it using the OpenWhisk CLI:
  wsk action invoke ${PACKAGE_NAME}/${ACTION_NAME} -r -p num_ops 10 -p operation_type set -p use_batching false -p parallel_calls 5

Or using curl:
  curl -u ${AUTH} -X POST ${WEB_URL} -H 'Content-Type: application/json' -d '{\"num_ops\": 10, \"operation_type\": \"set\", \"use_batching\": false, \"parallel_calls\": 5}'
" 