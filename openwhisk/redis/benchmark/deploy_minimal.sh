#!/bin/bash

set -e

# Configuration
ACTION_NAME="redis_benchmark"
PACKAGE_NAME="redis_benchmark"

echo "Deploying minimal Redis benchmark action..."

# Create or update package
wsk package update ${PACKAGE_NAME}

# Deploy minimal action directly (no packaging)
wsk action update ${PACKAGE_NAME}/${ACTION_NAME} \
  --kind python:3 \
  actions/redis_minimal.py \
  -p REDIS_HOST "$REDIS_HOST" \
  -p REDIS_PORT "${REDIS_PORT:-6379}"

echo "Minimal Redis benchmark action deployed. Test with:"
echo "wsk action invoke ${PACKAGE_NAME}/${ACTION_NAME} -r -p num_ops 10 -p operation_type set"

echo "Done." 