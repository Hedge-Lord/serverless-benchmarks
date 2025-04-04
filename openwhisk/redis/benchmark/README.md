# Redis Benchmark for OpenWhisk

This benchmark is designed to test the performance of OpenWhisk actions accessing Redis, both directly and through a batching agent. It provides a foundation for implementing and measuring batching optimizations at the runtime level in OpenWhisk.

## Overview

The benchmark can be run in two modes:
- **Direct Redis access**: Action directly connects to Redis server
- **Batched Redis access**: Action connects to a Redis batching agent, which batches operations

The benchmark supports various Redis operations:
- `set`: Set key-value pairs
- `get`: Retrieve key-value pairs
- `del`: Delete keys
- `exists`: Check if keys exist

## Setup

### Prerequisites

- OpenWhisk deployment with `wsk` CLI configured
- Python 3.7+ with pip
- Docker installed on all worker nodes
- Local Docker registry at `localhost:5000` on each node
- Redis server accessible from OpenWhisk
- Redis batching agent deployed (for batched mode)

### Configuration

1. Create a `local.env` file from the template:

```bash
cp template.local.env local.env
```

2. Edit the `local.env` file with your configuration:

```
# Redis server configuration
REDIS_HOST=your_redis_server_ip
REDIS_PORT=6379
REDIS_PASSWORD=your_redis_password_if_any

# Redis batching agent configuration (optional)
BATCHING_AGENT_HOST=your_batching_agent_ip
BATCHING_AGENT_PORT=8080

# OpenWhisk configuration
OPENWHISK_APIHOST=your_openwhisk_host
OPENWHISK_AUTH=your_auth_key
```

### Build and Deployment

The benchmark uses a two-step build and deployment process:

1. Build the image on each worker node:

```bash
# On each worker node
./build.sh
```

2. Deploy the action from the master node:

```bash
# On the master node (with kubectl & wsk access)
export REDIS_HOST=your_redis_server_ip
./deploy.sh
```

This approach ensures that the Docker image is available locally on each worker node, minimizing image pull latency when actions are invoked.

## Running the Benchmark

### Direct Invocation

You can invoke the action directly for testing:

```bash
# Single operation, direct Redis access
wsk action invoke redis_benchmark/redis_benchmark -r -p num_ops 1 -p operation_type set -p use_batching false

# Multiple operations, batched Redis access
wsk action invoke redis_benchmark/redis_benchmark -r -p num_ops 10 -p operation_type set -p use_batching true -p parallel_calls 5
```

### Benchmark Runner

For performance benchmarking, use the benchmark runner script:

```bash
# Direct Redis access benchmark
python benchmark_runner.py --action redis_benchmark/redis_benchmark --rate 10 --invocations 100 --ops 10 --operation set

# Batched Redis access benchmark
python benchmark_runner.py --action redis_benchmark/redis_benchmark --rate 10 --invocations 100 --ops 10 --operation set --batching
```

### Parameters

- `--action`: Name of the OpenWhisk action to invoke (default: `redis_benchmark/redis_benchmark`)
- `--rate`: Rate of invocations per second (default: 10)
- `--invocations`: Total number of invocations to perform (default: 100)
- `--ops`: Number of Redis operations per invocation (default: 10)
- `--operation`: Redis operation type: get, set, del, exists (default: set)
- `--batching`: Enable batching mode (default: False)
- `--parallel`: Number of parallel calls within each action (default: 5)
- `--key-prefix`: Prefix for Redis keys (default: benchmark)
- `--output`: Output file for results (default: redis_benchmark_results.json)
- `--env`: Path to environment file (default: local.env)

## Comparing Performance

To compare direct vs. batched performance:

```bash
# Run direct Redis access benchmark
python benchmark_runner.py --operation set --ops 100 --output direct_results.json

# Run batched Redis access benchmark
python benchmark_runner.py --operation set --ops 100 --batching --output batched_results.json

# Compare the results
python compare_results.py direct_results.json batched_results.json
```

## Results

The benchmark generates a results file containing:

- Execution time percentiles (50th, 90th, 99th)
- Min, max, and mean execution times
- Success and error rates
- Raw results for all invocations

## Project Structure

```
openwhisk/redis/benchmark/
├── actions/
│   ├── redis_benchmark.go   # The Go implementation of the Redis benchmark
│   ├── Dockerfile           # Dockerfile for building the action
│   └── go.mod               # Go module definition
├── benchmark_runner.py      # Script to invoke actions and collect metrics
├── build.sh                 # Script to build and push the Docker image to local registry
├── deploy.sh                # Script to deploy the action to OpenWhisk
├── template.local.env       # Template for configuration variables
└── README.md                # This documentation
```

## Troubleshooting

### Connection Issues

If the action fails to connect to Redis or the batching agent:

1. Verify the Redis server is accessible from OpenWhisk
2. Check the Redis port is open and not blocked by firewalls
3. For batched mode, ensure the batching agent is running and accessible

### Image Pull Issues

If you see "ImagePullBackOff" errors in the OpenWhisk logs:

1. Make sure the build script was run on all worker nodes
2. Verify the local registry is running on each node
3. Check that the image name and tag match between build and deploy scripts

### Performance Issues

If batching doesn't show performance improvement:

1. Check the batch window setting in the batching agent
2. Increase parallel operations to see more significant batching effects
3. Try operations that have higher Redis latency (like complex queries) 