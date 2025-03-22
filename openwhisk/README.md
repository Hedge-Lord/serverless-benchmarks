# OpenWhisk Benchmarks and Tools

This directory contains benchmarks and tools for OpenWhisk serverless platform.

## Directory Structure

- **benchmark/**: S3 access benchmark implementation
- **cluster-setup/**: Scripts and documentation for setting up OpenWhisk on Kubernetes
- **batching-agent/**: S3 request batching agent for optimizing S3 access performance

## Getting Started

### Setting up the Cluster

1. **Setting up the cluster**: 
   - Follow the instructions in `cluster-setup/cloudlab.md` to set up a Kubernetes cluster
   - Use the `cluster-setup/setup_storage.sh` script to create required persistent volumes

### Optimizing S3 Performance with Batching Agent

The batching agent improves S3 performance by:
1. Running as a DaemonSet on each node
2. Batching similar S3 requests over a short time window
3. Reducing the number of API calls to S3

To deploy the batching agent:

1. Configure AWS credentials:
   ```bash
   cd batching-agent
   # Base64 encode your AWS credentials and update secret.yaml
   ```

2. Build and deploy:
   ```bash
   make docker-build
   make deploy
   ```

3. To update your OpenWhisk actions to use the batching agent, see examples in `batching-agent/examples/`.

### Running the Benchmark

2. **Running the benchmark**:
   - Configure AWS credentials in `benchmark/local.env`
   - Deploy the S3 access action using `benchmark/deploy.sh`
   - Run the benchmark using `benchmark/benchmark_runner.py`

Refer to the README in each directory for more detailed instructions. 