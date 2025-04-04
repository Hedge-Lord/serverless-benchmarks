# Redis Benchmark for OpenWhisk

This directory contains a Redis benchmark implementation for OpenWhisk to test and compare performance between direct Redis access and batched Redis access via a Redis batching agent.

## Deployment

### Prerequisites

1. OpenWhisk should be installed and configured
2. `wsk` CLI should be configured with proper credentials
3. Docker should be installed and running
4. A local container registry at `localhost:5000` should be running
5. A Redis instance should be accessible from the OpenWhisk cluster
6. Redis batching agent should be deployed (for batched access mode)

### Deploying the Redis Benchmark

To deploy the Redis benchmark:

```bash
# Set required environment variables
export REDIS_HOST=<redis-server-ip>  # Required for direct Redis access

# Optional environment variables
export REDIS_PORT=6379              # Optional, defaults to 6379
export REDIS_PASSWORD=<password>    # Optional
export BATCHING_AGENT_HOST=<host>   # Optional, will auto-detect if not specified

# Deploy the action
./deploy.sh
```

## Usage

The Redis benchmark action supports both direct Redis access and batched access via the Redis batching agent. You can control the behavior using the `use_batching` parameter when invoking the action.

### Invoking with Direct Redis Access (non-batched)

```bash
# Default parameters
wsk action invoke redis_benchmark/redis_benchmark -r -p num_ops 10 -p operation_type set -p use_batching false -p parallel_calls 5

# With additional parameters
wsk action invoke redis_benchmark/redis_benchmark -r \
  -p num_ops 100 \
  -p operation_type set \
  -p use_batching false \
  -p parallel_calls 10 \
  -p key_prefix test_key
```

### Invoking with Batched Redis Access

```bash
# Default parameters
wsk action invoke redis_benchmark/redis_benchmark -r -p num_ops 10 -p operation_type set -p use_batching true -p parallel_calls 5

# With additional parameters
wsk action invoke redis_benchmark/redis_benchmark -r \
  -p num_ops 100 \
  -p operation_type set \
  -p use_batching true \
  -p parallel_calls 10 \
  -p key_prefix test_key
```

### Using curl

```bash
# Get the web action URL (The deploy script will show this)
WEB_URL="https://your-openwhisk-host/api/v1/web/guest/redis_benchmark/redis_benchmark.json?blocking=true"
AUTH="your-auth-key"

# Non-batched invocation
curl -u ${AUTH} -X POST ${WEB_URL} \
  -H 'Content-Type: application/json' \
  -d '{"num_ops": 10, "operation_type": "set", "use_batching": false, "parallel_calls": 5}'

# Batched invocation
curl -u ${AUTH} -X POST ${WEB_URL} \
  -H 'Content-Type: application/json' \
  -d '{"num_ops": 10, "operation_type": "set", "use_batching": true, "parallel_calls": 5}'
```

## Parameters

The benchmark action accepts the following parameters:

| Parameter | Description | Default |
|-----------|-------------|---------|
| `num_ops` | Number of Redis operations to perform | 1 |
| `operation_type` | Type of operation: `get`, `set`, `del`, or `exists` | `get` |
| `use_batching` | Whether to use the batching agent | `false` |
| `parallel_calls` | Number of concurrent operations | 1 |
| `key_prefix` | Prefix for Redis keys | `test_key` |
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