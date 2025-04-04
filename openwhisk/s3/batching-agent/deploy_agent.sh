#!/bin/bash
set -e  # Exit on any error

echo "Deleting existing daemonset and service..."
kubectl delete daemonset s3-batching-agent -n openwhisk || true
kubectl delete service s3-batching-agent -n openwhisk || true

# Wait for a short period to allow cleanup
WAIT_TIME=3
echo "Waiting ${WAIT_TIME} seconds for resources to be fully removed..."
sleep ${WAIT_TIME}

echo "Applying new service and daemonset..."
kubectl apply -f kubernetes/service.yaml
kubectl apply -f kubernetes/daemonset.yaml

echo "Deployment complete. Showing pod status..."
kubectl get pods -l app=s3-batching-agent -n openwhisk -o wide

echo ""
echo "The S3 batching agent should now be running on all nodes."
echo "To test the batching agent directly, run:"
echo "  curl http://<node-ip>:8080/health"
echo ""
echo "To deploy the OpenWhisk batched action, run:"
echo "  cd ../benchmark"
echo "  ./deploy_batched.sh"
echo ""
echo "For testing with a specific node, you can specify the host:"
echo "  BATCHING_AGENT_HOST=node0.ggz-248982.ucla-progsoftsys-pg0.utah.cloudlab.us ./deploy_batched.sh"

