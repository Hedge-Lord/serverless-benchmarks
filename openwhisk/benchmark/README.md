# OpenWhisk S3 Access Benchmark

This benchmark is designed to test the performance of OpenWhisk actions accessing S3 storage, similar to the AWS Lambda implementation in the lambda-bc-opt directory. It provides a foundation for implementing batching at the runtime level in OpenWhisk.

## Overview

The benchmark mimics the data storage access pattern from the original lambda-bc-opt implementation:
- It accesses S3 storage to list objects
- Retrieves and reads the content of objects
- Processes the extracted data
- Supports multiple S3 calls per invocation

## Setup

### Prerequisites

- OpenWhisk deployment with `wsk` CLI configured
- Python 3.7+ with pip
- AWS credentials with S3 access

### Configuration

1. Create a `local.env` file from the template:

```bash
cp template.local.env local.env
```

2. Edit the `local.env` file with your AWS credentials and other configuration:

```
# AWS credentials
AWS_ACCESS_KEY_ID=your_access_key_here
AWS_SECRET_ACCESS_KEY=your_secret_key_here
AWS_REGION=us-east-1

# S3 configuration
S3_BUCKET=your_bucket_name

# OpenWhisk configuration
OPENWHISK_APIHOST=your_openwhisk_host
OPENWHISK_AUTH=your_auth_key
```
3. (For batching agent discovery) modify the RBAC so pods can discover the current node's IP:
```bash
kubectl apply -f pod-reader.yaml
```

### Installation

Deploy the S3 access action:

```bash
./deploy.sh
```

This will:
1. Create a Python virtual environment
2. Install dependencies from requirements.txt
3. Package the S3 access action with dependencies
4. Deploy the action to OpenWhisk with the credentials from local.env

## Running the Benchmark

### Direct Invocation

You can invoke the action directly for testing:

```bash
# Single S3 call
wsk action invoke s3-access --blocking --result

# Multiple S3 calls
wsk action invoke s3-access --blocking --result --param num_calls 5
```

### Benchmark Runner

For performance benchmarking, use the benchmark runner script:

```bash
python benchmark_runner.py --action s3-access --rate 10 --invocations 100 --calls 3
```

### Parameters

- `--action`: Name of the OpenWhisk action to invoke (default: `s3-access`)
- `--rate`: Rate of invocations per second (default: 10)
- `--invocations`: Total number of invocations to perform (default: 100)
- `--calls`: Number of S3 calls per invocation (default: 1)
- `--output`: Output file for results (default: `benchmark_results.txt`)

## Results

The benchmark generates a results file containing:

- Execution time percentiles (50th, 90th, 99th)
- Min, max, and mean execution times
- Raw results for all invocations

## Project Structure

```
openwhisk/benchmark/
├── actions/
│   ├── s3_access.py       # The OpenWhisk action implementation
│   └── requirements.txt   # Python dependencies
├── benchmark_runner.py    # Script to invoke actions and collect metrics
├── deploy.sh              # Script to package and deploy the action
├── template.local.env     # Template for configuration variables
└── README.md              # This documentation
```

## Future Work

This implementation provides the base functionality for S3 storage access. Future work will implement a batching service using a DaemonSet that runs per-node in the OpenWhisk deployment. 