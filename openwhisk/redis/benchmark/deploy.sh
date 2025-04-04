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

# Check if a specific batching agent host is provided
if [ -n "$BATCHING_AGENT_HOST" ]; then
  echo "Using provided batching agent host: $BATCHING_AGENT_HOST"
else
  echo "No specific batching agent host provided, the action will auto-detect the host if batching is enabled."
fi

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

# Create or update package
echo "Creating/updating package..."
wsk package update ${PACKAGE_NAME}

# Deploy the action
echo "Deploying Redis benchmark action..."
wsk action update ${PACKAGE_NAME}/${ACTION_NAME} \
  --docker ${REGISTRY_IMAGE} \
  --memory 512 \
  --timeout 60000 \
  --web true \
  -p REDIS_HOST "$REDIS_HOST" \
  -p REDIS_PORT "$REDIS_PORT" \
  ${REDIS_PASSWORD:+-p REDIS_PASSWORD "$REDIS_PASSWORD"} \
  ${BATCHING_AGENT_HOST:+-p batching_agent_host "$BATCHING_AGENT_HOST"}

# Get the action URL
URL=$(wsk action get ${PACKAGE_NAME}/${ACTION_NAME} --url | tail -n1)
AUTH=$(wsk property get --auth | awk '{print $3}')
WEB_URL="${URL}.json?blocking=true"

echo "
Redis benchmark action has been deployed!

You can invoke it in various modes:

1. Standard Redis access (non-batched):
   wsk action invoke ${PACKAGE_NAME}/${ACTION_NAME} -r -p num_ops 10 -p operation_type set -p use_batching false -p parallel_calls 5

2. Batched Redis access:
   wsk action invoke ${PACKAGE_NAME}/${ACTION_NAME} -r -p num_ops 10 -p operation_type set -p use_batching true -p parallel_calls 5

3. Using curl (non-batched):
   curl -u ${AUTH} -X POST ${WEB_URL} -H 'Content-Type: application/json' -d '{\"num_ops\": 10, \"operation_type\": \"set\", \"use_batching\": false, \"parallel_calls\": 5}'

4. Using curl (batched):
   curl -u ${AUTH} -X POST ${WEB_URL} -H 'Content-Type: application/json' -d '{\"num_ops\": 10, \"operation_type\": \"set\", \"use_batching\": true, \"parallel_calls\": 5}'

Additional parameters:
- operation_type: 'get', 'set', 'del', or 'exists'
- key_prefix: Prefix for Redis keys (default: 'test_key')
- parallel_calls: Number of concurrent operations (default: 1)
" 