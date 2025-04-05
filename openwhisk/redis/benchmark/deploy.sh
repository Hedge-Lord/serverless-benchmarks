#!/bin/bash

set -e

# Configuration variables
ACTION_NAME="redis_benchmark"
PACKAGE_NAME="redis_benchmark"
REGISTRY_HOST="localhost:5000"

# Show usage if requested
print_usage() {
  echo "Usage: $0 [OPTIONS]"
  echo ""
  echo "Deploy the Redis benchmark action to OpenWhisk"
  echo ""
  echo "Options:"
  echo "  --language, -l LANG   Specify the implementation language (go or python, default: go)"
  echo "  --help, -h            Display this help message"
  echo ""
  echo "Environment variables:"
  echo "  REDIS_HOST            Redis server hostname/IP (required)"
  echo "  REDIS_PORT            Redis server port (default: 6379)"
  echo "  REDIS_PASSWORD        Redis server password (optional)"
  echo "  BATCHING_AGENT_HOST   Batching agent hostname/IP (optional)"
  echo "  BATCHING_AGENT_PORT   Batching agent port (default: 8080)"
  echo ""
  echo "Example:"
  echo "  REDIS_HOST=10.0.0.1 ./deploy.sh -l python"
}

# Parse command line arguments
LANGUAGE="go"  # Default to Go

while [[ $# -gt 0 ]]; do
  case "$1" in
    -l|--language)
      LANGUAGE="$2"
      shift 2
      ;;
    -h|--help)
      print_usage
      exit 0
      ;;
    *)
      echo "Error: Unknown option $1"
      print_usage
      exit 1
      ;;
  esac
done

# Validate language
if [[ "$LANGUAGE" != "go" && "$LANGUAGE" != "python" ]]; then
  echo "Error: Language must be either 'go' or 'python'"
  print_usage
  exit 1
fi

# Check if REDIS_HOST is provided
if [ -z "$REDIS_HOST" ]; then
  echo "Error: REDIS_HOST environment variable is required."
  echo "Please set it before running this script:"
  echo "  export REDIS_HOST=<redis-server-ip>"
  exit 1
fi

# Use default port if not provided
REDIS_PORT=${REDIS_PORT:-6379}
echo "Using Redis server at $REDIS_HOST:$REDIS_PORT"

# Check if a specific batching agent host is provided
if [ -n "$BATCHING_AGENT_HOST" ]; then
  echo "Using provided batching agent host: $BATCHING_AGENT_HOST"
else
  echo "No specific batching agent host provided, the action will auto-detect the host if batching is enabled."
fi

# Deploy the appropriate implementation
echo "Deploying Redis benchmark with $LANGUAGE implementation..."

if [ "$LANGUAGE" == "go" ]; then
  # Go implementation uses Docker
  DOCKER_IMAGE="redis-benchmark:latest"
  REGISTRY_IMAGE="${REGISTRY_HOST}/${DOCKER_IMAGE}"
  
  # Check if the image exists in the registry
  echo "Verifying image exists in registry..."
  if ! curl -s "http://${REGISTRY_HOST}/v2/redis-benchmark/tags/list" | grep -q "latest"; then
    echo "Warning: Image ${REGISTRY_IMAGE} not found in registry."
    echo "Make sure build.sh has been run on all worker nodes before deploying."
    echo "Continuing with deployment anyway..."
  fi
  
  # Create or update package
  echo "Creating/updating package..."
  wsk package update ${PACKAGE_NAME}
  
  # Deploy the action
  echo "Deploying Go Redis benchmark action..."
  wsk action update ${PACKAGE_NAME}/${ACTION_NAME} \
    --docker ${REGISTRY_IMAGE} \
    --memory 512 \
    --timeout 60000 \
    --web true \
    -p REDIS_HOST "$REDIS_HOST" \
    -p REDIS_PORT "$REDIS_PORT" \
    ${REDIS_PASSWORD:+-p REDIS_PASSWORD "$REDIS_PASSWORD"} \
    ${BATCHING_AGENT_HOST:+-p batching_agent_host "$BATCHING_AGENT_HOST"}
    
else
  # Python implementation uses virtual env and zip package
  echo "Creating virtual environment for Python implementation..."
  
  # Navigate to the actions directory
  cd "$(dirname "$0")/actions"
  
  # Set up temporary directory
  TEMP_DIR="tmp"
  rm -rf "$TEMP_DIR"
  mkdir -p "$TEMP_DIR"
  
  # Create virtual environment and install dependencies
  python3 -m venv "$TEMP_DIR/venv"
  source "$TEMP_DIR/venv/bin/activate"
  pip install --upgrade pip
  pip install -r requirements.txt
  
  # Create zip package
  cp redis_benchmark.py "$TEMP_DIR/"
  cd "$TEMP_DIR"
  zip -r ../action.zip redis_benchmark.py venv/lib/python*/site-packages
  cd ..
  
  # Deactivate virtual environment
  deactivate
  
  # Create or update package
  echo "Creating/updating package..."
  wsk package update ${PACKAGE_NAME}
  
  # Deploy the action
  echo "Deploying Python Redis benchmark action..."
  wsk action update ${PACKAGE_NAME}/${ACTION_NAME} \
    --kind python:3.9 \
    --main main \
    --memory 512 \
    --timeout 60000 \
    --web true \
    action.zip \
    -p REDIS_HOST "$REDIS_HOST" \
    -p REDIS_PORT "$REDIS_PORT" \
    ${REDIS_PASSWORD:+-p REDIS_PASSWORD "$REDIS_PASSWORD"} \
    ${BATCHING_AGENT_HOST:+-p batching_agent_host "$BATCHING_AGENT_HOST"}
  
  # Clean up
  rm -rf "$TEMP_DIR" action.zip
fi

# Get the action URL
URL=$(wsk action get ${PACKAGE_NAME}/${ACTION_NAME} --url | tail -n1)
AUTH=$(wsk property get --auth | awk '{print $3}')
WEB_URL="${URL}.json?blocking=true"

echo "
Redis benchmark action has been deployed using ${LANGUAGE} implementation!

You can invoke it in various modes:

1. Standard Redis access (non-batched):
   wsk action invoke ${PACKAGE_NAME}/${ACTION_NAME} -r -p num_ops 10 -p operation_type set -p use_batching false -p parallel_calls 5

2. Batched Redis access:
   wsk action invoke ${PACKAGE_NAME}/${ACTION_NAME} -r -p num_ops 10 -p operation_type set -p use_batching true -p parallel_calls 5

3. Using curl (non-batched):
   curl -u ${AUTH} -X POST ${WEB_URL} -H 'Content-Type: application/json' -d '{\"num_ops\": 10, \"operation_type\": \"set\", \"use_batching\": false, \"parallel_calls\": 5}'

4. Using curl (batched):
   curl -u ${AUTH} -X POST ${WEB_URL} -H 'Content-Type: application/json' -d '{\"num_ops\": 10, \"operation_type\": \"set\", \"use_batching\": true, \"parallel_calls\": 5}'

Additional parameters:
- operation_type: 'get', 'set', 'del', or 'exists'
- key_prefix: Prefix for Redis keys (default: 'test_key')
- parallel_calls: Number of concurrent operations (default: 1)
" 