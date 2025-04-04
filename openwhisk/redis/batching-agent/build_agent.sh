#!/bin/bash
set -e

go mod tidy

echo "Building Redis batching agent Docker image with no cache..."
docker build --no-cache -t redis-batching-agent:latest .

echo "Tagging image as localhost:5000/redis-batching-agent:latest..."
docker tag redis-batching-agent:latest localhost:5000/redis-batching-agent:latest

echo "Pushing image to local registry..."
docker push localhost:5000/redis-batching-agent:latest

echo "Image build and push complete."

