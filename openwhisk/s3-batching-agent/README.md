# S3 Batching Agent for OpenWhisk

The S3 Batching Agent is a DaemonSet that runs on each Kubernetes node and aggregates S3 requests from OpenWhisk actions to improve performance.

## Overview

The batching agent:

1. Runs as a DaemonSet on each node in the Kubernetes cluster
2. Exposes an HTTP API that mimics a subset of the S3 API
3. Aggregates similar requests within a configurable time window
4. Forwards batched requests to S3 and returns the results

By batching multiple identical or similar requests, the agent reduces the number of API calls to S3, improving performance and reducing costs.

## Deployment

### Prerequisites

- Kubernetes cluster with OpenWhisk deployed
- AWS credentials with access to S3

### Build and Deploy

1. Build the Docker image on each worker node (for now. Will need to set up a docker registry on each):

Setting up the local registry:
```bash
docker run -d -p 5000:5000 --restart=always --name registry registry:2
docker ps
```

Build the agent:
```bash
./build_agent.sh
```

2. Create AWS credentials secret:

```bash
# Base64 encode your AWS credentials
echo -n 'YOUR_ACCESS_KEY' | base64
echo -n 'YOUR_SECRET_KEY' | base64

# Update the secret.yaml file with the base64 encoded values
kubectl apply -f kubernetes/secret.yaml
```

3. Deploy the DaemonSet (on master node only):

```bash
kubectl apply -f kubernetes/daemonset.yaml
```

## Usage

OpenWhisk actions can use the batching agent by sending requests to the agent's endpoint on the node instead of directly to S3.

The agent exposes the following endpoints:

- `GET /health` - Health check endpoint
- `GET /s3/listBuckets` - List S3 buckets
- `GET /s3/listObjects?bucket=<bucket>&prefix=<prefix>` - List objects in a bucket
- `GET /s3/getObject?bucket=<bucket>&key=<key>` - Get an object from S3

### Example in an OpenWhisk Action

In your OpenWhisk action, you can get the local node's IP using the Kubernetes Downward API and send requests to the batching agent:

```javascript
function main(params) {
    // Get node IP from environment variable (injected by Kubernetes)
    const nodeIP = process.env.NODE_IP || 'localhost';
    const agentPort = 8080;
    const agentUrl = `http://${nodeIP}:${agentPort}`;
    
    // Example: Get object from S3 via batching agent
    const bucket = params.bucket || 'default-bucket';
    const key = params.key || 'example.txt';
    
    const url = `${agentUrl}/s3/getObject?bucket=${bucket}&key=${key}`;
    
    // Make HTTP request to the agent
    return new Promise((resolve, reject) => {
        const http = require('http');
        http.get(url, (res) => {
            let data = '';
            res.on('data', (chunk) => { data += chunk; });
            res.on('end', () => {
                resolve({ body: data });
            });
        }).on('error', (err) => {
            reject({ error: err.message });
        });
    });
}
```

## Configuration

The batching agent can be configured using command-line flags:

- `--port` - Port to listen on (default: 8080)
- `--batching` - Enable request batching (default: true)
- `--batch-window` - Batch window duration (default: 100ms)
- `--max-batch-size` - Maximum batch size (default: 10)
- `--debug` - Enable debug mode (default: false)
- `--aws-region` - AWS region (default: us-east-1)
- `--default-bucket` - Default S3 bucket name

## How It Works

1. The agent receives requests from multiple OpenWhisk actions
2. Similar requests within the same batch window are grouped together
3. For each group, the agent executes only one request to S3
4. The response is copied to all requests in the group
5. This reduces the number of API calls to S3

## Monitoring

When debug mode is enabled, the agent exposes a `/debug/config` endpoint that shows the current configuration.

## Limitations

- Currently supports a subset of the S3 API (ListBuckets, ListObjects, GetObject)
- Does not support authentication between actions and the agent (assumes trusted network)
- Batching introduces a small latency due to the batch window 