#!/bin/bash
# Script to package and deploy the Redis benchmark action

set -e

# Get the directory of this script
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"

# Check if REDIS_HOST is provided
if [ -z "$REDIS_HOST" ]; then
  echo "Error: REDIS_HOST environment variable is required."
  echo "Please set it before running this script:"
  echo "  export REDIS_HOST=<redis-server-ip>"
  exit 1
fi

# Configuration
ACTION_NAME="redis_benchmark"
PACKAGE_NAME="redis_benchmark"
ACTION_DIR="$SCRIPT_DIR/actions"
TEMP_DIR=$(mktemp -d)
ZIP_FILE="redis_action.zip"

echo "Deploying Redis benchmark action..."

# Create a virtual environment for dependencies
echo "Setting up Python environment..."
python3 -m venv $TEMP_DIR/venv
source $TEMP_DIR/venv/bin/activate

# Install dependencies from requirements.txt
echo "Installing dependencies from requirements.txt..."
pip install -r $ACTION_DIR/requirements.txt

# Create action package dir
mkdir -p $TEMP_DIR/package

# Copy the action file
echo "Copying action code..."
cp $ACTION_DIR/redis_benchmark.py $TEMP_DIR/package/__main__.py

# Copy dependencies
echo "Copying dependencies..."
cp -r $TEMP_DIR/venv/lib/python*/site-packages/* $TEMP_DIR/package/

# Create the zip package
echo "Creating action package..."
cd $TEMP_DIR/package
zip -r $SCRIPT_DIR/$ZIP_FILE * > /dev/null

# Go back to script directory
cd $SCRIPT_DIR

# Create or update package
echo "Creating/updating package..."
wsk package update ${PACKAGE_NAME}

# Deploy the action
echo "Deploying Redis benchmark action..."
wsk action update ${PACKAGE_NAME}/${ACTION_NAME} \
  --kind python:3 $ZIP_FILE \
  --memory 512 --timeout 60000 --web true \
  -p REDIS_HOST "$REDIS_HOST" \
  -p REDIS_PORT "${REDIS_PORT:-6379}" \
  ${REDIS_PASSWORD:+-p REDIS_PASSWORD "$REDIS_PASSWORD"} \
  ${BATCHING_AGENT_HOST:+-p batching_agent_host "$BATCHING_AGENT_HOST"}

# Clean up
rm -rf $TEMP_DIR
rm -f $ZIP_FILE

# Verify the action was created
echo "Verifying action deployment..."
wsk action get ${PACKAGE_NAME}/${ACTION_NAME}

# Get the action URL
URL=$(wsk action get ${PACKAGE_NAME}/${ACTION_NAME} --url | tail -n1)
AUTH=$(wsk property get --auth | awk '{print $3}')
WEB_URL="${URL}.json?blocking=true"

echo "
Redis benchmark action has been deployed!

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

echo "Done." 