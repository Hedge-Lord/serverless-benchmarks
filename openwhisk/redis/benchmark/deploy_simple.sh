#!/bin/bash

set -e

# Configuration
ACTION_NAME="redis_benchmark"
PACKAGE_NAME="redis_benchmark"

echo "Deploying simple test action..."

# Create or update package
wsk package update ${PACKAGE_NAME}

# Deploy simple action directly (no packaging)
wsk action update ${PACKAGE_NAME}/${ACTION_NAME} \
  actions/simple_redis.py \
  --kind python:3

echo "Simple test action deployed. Invoking..."

# Test the action
wsk action invoke ${PACKAGE_NAME}/${ACTION_NAME} -r

echo "Done." 