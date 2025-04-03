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

