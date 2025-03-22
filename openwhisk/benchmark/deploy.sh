#!/bin/bash

# Script to deploy OpenWhisk actions for the S3 access benchmark

set -e

# Get the directory of this script
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"

# Check if local.env exists, otherwise prompt to create it
if [ ! -f "$SCRIPT_DIR/local.env" ]; then
    echo "local.env file not found. Please create it based on template.local.env"
    echo "cp template.local.env local.env"
    echo "Then edit local.env with your AWS credentials and OpenWhisk configuration"
    exit 1
fi

# Load environment variables from local.env
source "$SCRIPT_DIR/local.env"

# Configuration
ACTION_NAME=${1:-s3-access}
ACTION_DIR="$SCRIPT_DIR/actions"
ZIP_FILE="s3_action.zip"

echo "Deploying OpenWhisk S3 access benchmark action..."

# Create zip package with dependencies
echo "Creating action package..."
cd $ACTION_DIR
pip install -t . -r requirements.txt
zip -r ../$ZIP_FILE *.py boto3 botocore s3transfer jmespath dateutil urllib3 six python_dateutil

# Go back to previous directory
cd ..

# Check if action exists and delete it
if wsk action get $ACTION_NAME > /dev/null 2>&1; then
    echo "Action $ACTION_NAME exists, updating..."
    wsk action delete $ACTION_NAME
fi

# Create the action with parameters from local.env
echo "Creating action $ACTION_NAME with parameters from local.env..."
wsk action create $ACTION_NAME --kind python:3 $ZIP_FILE \
    --memory 512 --timeout 60000 --web false \
    --param AWS_ACCESS_KEY_ID "$AWS_ACCESS_KEY_ID" \
    --param AWS_SECRET_ACCESS_KEY "$AWS_SECRET_ACCESS_KEY" \
    --param AWS_REGION "$AWS_REGION" \
    --param bucket "$S3_BUCKET"

# Verify the action was created
echo "Verifying action deployment..."
wsk action get $ACTION_NAME

echo "Deployment complete. You can now run the benchmark using:"
echo "python benchmark_runner.py --action $ACTION_NAME --rate 10 --invocations 100 --calls 1"

# Cleanup the zip file
rm -f $ZIP_FILE

echo "Done." 