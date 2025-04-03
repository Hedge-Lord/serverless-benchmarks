#!/bin/bash
set -e  # 

echo "Building Docker image with no cache..."
docker build --no-cache -t s3-batching-agent:latest .

echo "Tagging image as localhost:5000/s3-batching-agent:latest..."
docker tag s3-batching-agent:latest localhost:5000/s3-batching-agent:latest

echo "Pushing image to local registry..."
docker push localhost:5000/s3-batching-agent:latest

echo "Deleting existing daemonset and service..."
kubectl delete daemonset s3-batching-agent || true
kubectl delete service s3-batching-agent || true

# Wait for a short period to allow cleanup
WAIT_TIME=3
echo "Waiting ${WAIT_TIME} seconds for resources to be fully removed..."
sleep ${WAIT_TIME}

echo "Applying new service and daemonset..."
kubectl apply -f kubernetes/service.yaml
kubectl apply -f kubernetes/daemonset.yaml

echo "Deployment complete. Showing pod status..."
kubectl get pods -l app=s3-batching-agent -o wide
