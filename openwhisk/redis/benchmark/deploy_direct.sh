#!/bin/bash

set -e

# Configuration
ACTION_NAME="redis_benchmark"
PACKAGE_NAME="redis_benchmark"

# Check if REDIS_HOST is provided
if [ -z "$REDIS_HOST" ]; then
  echo "Error: REDIS_HOST environment variable is required."
  echo "Please set it before running this script:"
  echo "  export REDIS_HOST=<redis-server-ip>"
  exit 1
fi

echo "Deploying Redis benchmark action with direct implementation..."

# Create or update package
wsk package update ${PACKAGE_NAME}

# Deploy the action
wsk action update ${PACKAGE_NAME}/${ACTION_NAME} \
  actions/redis_direct.py \
  --kind python:3 \
  --memory 512 \
  --timeout 60000 \
  --web true \
  -p REDIS_HOST "$REDIS_HOST" \
  -p REDIS_PORT "${REDIS_PORT:-6379}" \
  ${REDIS_PASSWORD:+-p REDIS_PASSWORD "$REDIS_PASSWORD"} \
  ${BATCHING_AGENT_HOST:+-p batching_agent_host "$BATCHING_AGENT_HOST"}

echo "Redis benchmark action (direct implementation) has been deployed!"
echo "Invoke with: wsk action invoke ${PACKAGE_NAME}/${ACTION_NAME} -r -p num_ops 10 -p operation_type set -p use_batching false -p parallel_calls 5" 