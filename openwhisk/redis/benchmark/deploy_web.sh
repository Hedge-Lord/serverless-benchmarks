#!/bin/bash

set -e

# Configuration
ACTION_NAME="redis_benchmark"
PACKAGE_NAME="redis_benchmark"

echo "Deploying web action test..."

# Create or update package
wsk package update ${PACKAGE_NAME}

# Deploy simple web action
wsk action update ${PACKAGE_NAME}/${ACTION_NAME} \
  actions/setup_web_action.py \
  --kind python:3 \
  --web true

# Get the web action URL
URL=$(wsk action get ${PACKAGE_NAME}/${ACTION_NAME} --url | tail -1)
WEB_URL="${URL}.json"

echo "Web action deployed. Access at: ${WEB_URL}" 