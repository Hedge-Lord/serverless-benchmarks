#!/bin/bash

set -e

# Configuration variables
ACTION_NAME="redis_benchmark_batched"
PACKAGE_NAME="redis_benchmark"
DOCKER_IMAGE="redis-benchmark:latest"
REGISTRY_HOST="localhost:5000"
REGISTRY_IMAGE="${REGISTRY_HOST}/${DOCKER_IMAGE}"

# Check if a specific batching agent host is provided
if [ -n "$BATCHING_AGENT_HOST" ]; then
  echo "Using provided batching agent host: $BATCHING_AGENT_HOST"
else
  echo "No specific batching agent host provided, the action will auto-detect the host."
fi

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

# Deploy the action with batching enabled by default
echo "Deploying batched action..."
wsk action update ${PACKAGE_NAME}/${ACTION_NAME} \
  --docker ${REGISTRY_IMAGE} \
  --memory 512 \
  --timeout 60000 \
  --web true \
  -p use_batching true \
  ${BATCHING_AGENT_HOST:+-p batching_agent_host $BATCHING_AGENT_HOST}

# Get the action URL
URL=$(wsk action get ${PACKAGE_NAME}/${ACTION_NAME} --url | tail -n1)
AUTH=$(wsk property get --auth | awk '{print $3}')
WEB_URL="${URL}.json?blocking=true"

echo "
Batched action has been deployed!

You can invoke it using the OpenWhisk CLI:
  wsk action invoke ${PACKAGE_NAME}/${ACTION_NAME} -r -p num_ops 10 -p operation_type set -p parallel_calls 5

Or using curl:
  curl -u ${AUTH} -X POST ${WEB_URL} -H 'Content-Type: application/json' -d '{\"num_ops\": 10, \"operation_type\": \"set\", \"parallel_calls\": 5}'

For performance comparison, you can run the same benchmark without batching:
  wsk action invoke ${PACKAGE_NAME}/${ACTION_NAME} -r -p num_ops 10 -p operation_type set -p use_batching false -p parallel_calls 5
" 