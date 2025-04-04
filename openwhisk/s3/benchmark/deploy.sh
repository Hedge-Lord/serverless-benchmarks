#!/bin/bash
# Script to package and deploy the S3 access benchmark action

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
TEMP_DIR=$(mktemp -d)
ZIP_FILE="s3_action.zip"

echo "Deploying OpenWhisk S3 access benchmark action..."

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
cp $ACTION_DIR/s3_access.py $TEMP_DIR/package/__main__.py

# Copy dependencies
echo "Copying dependencies..."
cp -r $TEMP_DIR/venv/lib/python*/site-packages/* $TEMP_DIR/package/

# Create the zip package
echo "Creating action package..."
cd $TEMP_DIR/package
zip -r $SCRIPT_DIR/$ZIP_FILE *

# Go back to script directory
cd $SCRIPT_DIR

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

# Clean up
rm -rf $TEMP_DIR
rm -f $ZIP_FILE

# Verify the action was created
echo "Verifying action deployment..."
wsk action get $ACTION_NAME

echo "Deployment complete. You can now run the benchmark using:"
echo "python benchmark_runner.py --action $ACTION_NAME --rate 10 --invocations 100 --calls 1"
echo ""
echo "Or invoke directly with:"
echo "wsk action invoke $ACTION_NAME --blocking --result"
echo "wsk action invoke $ACTION_NAME --blocking --result --param num_calls 5"

echo "Done." 