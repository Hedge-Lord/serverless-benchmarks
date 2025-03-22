# OpenWhisk Benchmarks and Tools

This directory contains benchmarks and tools for OpenWhisk serverless platform.

## Directory Structure

- **benchmark/**: S3 access benchmark implementation
- **cluster-setup/**: Scripts and documentation for setting up OpenWhisk on Kubernetes

## Getting Started

1. **Setting up the cluster**: 
   - Follow the instructions in `cluster-setup/cloudlab.md` to set up a Kubernetes cluster
   - Use the `cluster-setup/setup_storage.sh` script to create required persistent volumes

2. **Running the benchmark**:
   - Configure AWS credentials in `benchmark/local.env`
   - Deploy the S3 access action using `benchmark/deploy.sh`
   - Run the benchmark using `benchmark/benchmark_runner.py`

Refer to the README in each directory for more detailed instructions. 