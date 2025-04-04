# Redis Benchmark for OpenWhisk

This directory contains a Redis benchmark implementation for OpenWhisk to test and compare performance between direct Redis access and batched Redis access via a Redis batching agent.

## Deployment

### Prerequisites

1. OpenWhisk should be installed and configured
2. `wsk` CLI should be configured with proper credentials
3. Docker should be installed and running
4. A local container registry at `localhost:5000` should be running
5. A Redis instance should be accessible from the OpenWhisk cluster

### Deploying the Standard Redis Benchmark

To deploy the standard (non-batched) Redis benchmark:

```bash
# Set environment variables
export REDIS_HOST=<redis-server-ip>  # Required
export REDIS_PORT=6379               # Optional, defaults to 6379
export REDIS_PASSWORD=<password>     # Optional

# Deploy the action
./deploy.sh
```

### Deploying the Batched Redis Benchmark

To deploy the batched Redis benchmark:

```bash
# Optionally specify a specific batching agent host
export BATCHING_AGENT_HOST=<batching-agent-ip>  # Optional, will auto-detect if not specified

# Deploy the action
./deploy_batched.sh
```

## Usage

### Invoking the Standard Redis Benchmark

```bash
# Invoke with default parameters
wsk action invoke redis_benchmark/redis_benchmark -r -p num_ops 10 -p operation_type set -p parallel_calls 5

# Specify additional parameters
wsk action invoke redis_benchmark/redis_benchmark -r \
  -p num_ops 100 \
  -p operation_type set \
  -p parallel_calls 10 \
  -p key_prefix test_key
```

### Invoking the Batched Redis Benchmark

```bash
# Invoke with default parameters
wsk action invoke redis_benchmark/redis_benchmark_batched -r -p num_ops 10 -p operation_type set -p parallel_calls 5

# Run in non-batched mode for comparison
wsk action invoke redis_benchmark/redis_benchmark_batched -r \
  -p num_ops 10 \
  -p operation_type set \
  -p use_batching false \
  -p parallel_calls 5
```

## Parameters

The benchmark actions accept the following parameters:

| Parameter | Description | Default |
|-----------|-------------|---------|
| `num_ops` | Number of Redis operations to perform | 1 |
| `operation_type` | Type of operation: `get`, `set`, `del`, or `exists` | `get` |
| `use_batching` | Whether to use the batching agent | `false` for standard, `true` for batched |
| `parallel_calls` | Number of concurrent operations | 1 |
| `key_prefix` | Prefix for Redis keys | `test_key` |
| `REDIS_HOST` | Redis server hostname or IP | `localhost` |
| `REDIS_PORT` | Redis server port | `6379` |
| `REDIS_PASSWORD` | Redis server password | `` |
| `batching_agent_host` | Batching agent hostname or IP | Auto-detected |
| `batching_agent_port` | Batching agent port | `8080` |

## Response Format

The benchmark will return a JSON response with the following structure:

```json
{
  "statusCode": 200,
  "execution_time_ms": 123.45,
  "num_ops": 10,
  "operation_type": "set",
  "parallel_calls": 5,
  "use_batching": true,
  "batching_url": "http://node-ip:8080",
  "redis_host": "redis-ip",
  "success_count": 10,
  "results": [
    {
      "key": "test_key_0",
      "status": "success",
      "value": "OK",
      "duration_ms": 12.34
    },
    ...
  ]
}
``` 