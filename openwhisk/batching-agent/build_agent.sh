docker build --no-cache -t s3-batching-agent:latest .
docker tag s3-batching-agent:latest localhost:5000/s3-batching-agent:latest
docker push localhost:5000/s3-batching-agent:latest
kubectl delete daemonset s3-batching-agent
kubectl apply -f kubernetes/daemonset.yaml

