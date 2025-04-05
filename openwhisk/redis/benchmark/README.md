# Redis Benchmarks for OpenWhisk

This directory contains benchmarking tools for evaluating Redis performance within OpenWhisk actions, with support for both direct Redis operations and operations through a batching agent.

## Implementations

The Redis benchmark is available in two implementations:

1. **Go Implementation**: The original implementation with high performance.
2. **Python Implementation**: An alternative implementation that may be easier to modify.

Both implementations maintain the same functionality and can be selected at deployment time.

## Prerequisites

- OpenWhisk CLI (`wsk`) installed and configured
- Access to a Redis server
- (Optional) Redis batching agent deployed
- Docker (for building the Go implementation)
- Python 3.9+ (for the Python implementation)

## Quick Start

1. Clone the repository
2. Build the Docker image (for Go implementation only)
3. Deploy the OpenWhisk action
4. Run benchmarks

## Building (Go Implementation Only)

To build the Docker image for the Go implementation, run:

```sh
# On each worker node
./build.sh
```

This builds the Docker image and pushes it to the local registry.

## Deployment

To deploy the OpenWhisk action, use the `deploy.sh` script:

```sh
# Deploy the Go implementation (default)
REDIS_HOST=<redis-ip> ./deploy.sh

# Deploy the Python implementation
REDIS_HOST=<redis-ip> ./deploy.sh -l python

# Specify batching agent (optional)
REDIS_HOST=<redis-ip> BATCHING_AGENT_HOST=<agent-ip> ./deploy.sh -l go
```

## Configuration

Create a `local.env` file based on the provided `local.env.template`:

```sh
cp local.env.template local.env
```

Edit `local.env` to set your configuration values:

```
# Required
REDIS_HOST=10.0.0.1

# Optional
REDIS_PORT=6379
REDIS_PASSWORD=your_password
BATCHING_AGENT_HOST=10.0.0.2
```

## Running Benchmarks

Use the `benchmark_runner.py` script to run benchmarks:

```sh
# Basic benchmark (direct Redis access)
./benchmark_runner.py --ops 100 --operation set

# With batching
./benchmark_runner.py --ops 100 --operation set --batching

# Advanced options
./benchmark_runner.py --ops 100 --operation set --batching --rate 20 --invocations 500 --parallel 10
```

### Benchmark Parameters

| Parameter      | Description                                      | Default              |
|----------------|--------------------------------------------------|----------------------|
| --action       | OpenWhisk action name                            | redis_benchmark/redis_benchmark |
| --rate         | Rate of invocations per second                   | 10                   |
| --invocations  | Total number of invocations to run               | 100                  |
| --ops          | Number of Redis operations per invocation        | 10                   |
| --operation    | Redis operation type (get, set, del, exists)     | set                  |
| --batching     | Use batching agent                               | false                |
| --parallel     | Parallel calls within each action                | 5                    |
| --key-prefix   | Prefix for Redis keys                            | benchmark            |
| --output       | Output file for results                          | redis_benchmark_results.json |
| --env          | Path to environment file                         | local.env            |

## Results Analysis

The benchmark results are saved to a JSON file with detailed metrics, including:
- Success rate
- Min, max, mean, median, and percentile latencies
- Raw results for further analysis

Example summary output:

```
Benchmark Summary:
Successful invocations: 100/100
Action execution time (ms):
  Min: 45.23
  Max: 210.76
  Mean: 120.45
  Median: 118.32
  90th percentile: 180.41
  99th percentile: 205.89
```

## Implementation Details

Both implementations:
- Support direct and batched Redis operations
- Cache the node IP to optimize batching agent discovery
- Provide detailed performance metrics
- Support parallel Redis operations
- Include diagnostic logging

## Troubleshooting

If you encounter issues:

1. Check that the Redis server is accessible
2. Verify that the batching agent is deployed (if using batching)
3. Ensure the action has been deployed correctly
4. Check the OpenWhisk logs for detailed error messages

## License

This project is licensed under the Apache License 2.0. 