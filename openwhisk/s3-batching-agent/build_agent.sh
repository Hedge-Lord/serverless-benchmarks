#!/bin/bash
set -e  # 

NODE_IP=$(hostname -I | awk '{print $1}')
echo "Current node IP: $NODE_IP"

echo "Building Docker image with no cache..."
docker build --no-cache -t s3-batching-agent:latest .

echo "Tagging image as $NODE_IP:5000/s3-batching-agent:latest..."
docker tag s3-batching-agent:latest $NODE_IP:5000/s3-batching-agent:latest

echo "Pushing image to local registry..."
docker push $NODE_IP:5000/s3-batching-agent:latest