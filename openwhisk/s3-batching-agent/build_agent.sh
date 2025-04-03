#!/bin/bash
set -e  # 

echo "Building Docker image with no cache..."
docker build --no-cache -t s3-batching-agent:latest .

echo "Tagging image as localhost:5000/s3-batching-agent:latest..."
docker tag s3-batching-agent:latest localhost:5000/s3-batching-agent:latest

echo "Pushing image to local registry..."
docker push localhost:5000/s3-batching-agent:latest