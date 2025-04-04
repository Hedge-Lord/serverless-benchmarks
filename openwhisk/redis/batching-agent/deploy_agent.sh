#!/bin/bash
set -e

echo "Deleting existing daemonset and service (if they exist)..."
kubectl delete daemonset redis-batching-agent -n openwhisk || true
kubectl delete service redis-batching-agent -n openwhisk || true

# Wait for a short period to allow cleanup
WAIT_TIME=3
echo "Waiting ${WAIT_TIME} seconds for resources to be fully removed..."
sleep ${WAIT_TIME}

echo "Applying service configuration..."
kubectl apply -f kubernetes/service.yaml

echo "Applying daemonset configuration..."
kubectl apply -f kubernetes/daemonset.yaml

echo "Checking the status of the pods..."
kubectl get pods -l app=redis-batching-agent -n openwhisk -o wide

echo "Redis batching agent deployment complete."
echo ""
echo "You can test the batching agent with:"
echo "kubectl port-forward service/redis-batching-agent 8080:8080 -n openwhisk"
echo "curl http://localhost:8080/health"
echo ""
echo "To deploy the OpenWhisk batched action, run:"
echo "cd ../benchmark"
echo "./deploy_batched.sh" 