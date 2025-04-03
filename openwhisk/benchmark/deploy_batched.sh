#!/bin/bash

# Set these environment variables or modify the script directly
: ${WSK:="wsk"}
: ${NAMESPACE:="guest"}
: ${PACKAGE_NAME:="s3benchmark"}
: ${ACTION_NAME:="s3_access_batched"}
: ${AWS_ACCESS_KEY_ID:="${AWS_ACCESS_KEY_ID}"}
: ${AWS_SECRET_ACCESS_KEY:="${AWS_SECRET_ACCESS_KEY}"}
: ${AWS_REGION:="us-east-1"}
: ${BATCHING_AGENT_PORT:="8080"}

# Print settings for debugging
echo "Deploying with the following settings:"
echo "  Package name: $PACKAGE_NAME"
echo "  Action name: $ACTION_NAME"
echo "  AWS Region: $AWS_REGION"
echo "  Batching agent port: $BATCHING_AGENT_PORT"
echo "  Batching agent host: Auto-detected from Kubernetes API"

# Check if AWS credentials are set
if [ -z "$AWS_ACCESS_KEY_ID" ] || [ -z "$AWS_SECRET_ACCESS_KEY" ]; then
    echo "Error: AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY must be set."
    exit 1
fi

# Prepare parameters array
PARAMS=()
PARAMS+=(--param AWS_ACCESS_KEY_ID "$AWS_ACCESS_KEY_ID")
PARAMS+=(--param AWS_SECRET_ACCESS_KEY "$AWS_SECRET_ACCESS_KEY")
PARAMS+=(--param AWS_REGION "$AWS_REGION")
PARAMS+=(--param BATCHING_AGENT_PORT "$BATCHING_AGENT_PORT")

# Create the package if it doesn't exist
$WSK package update "$PACKAGE_NAME" "${PARAMS[@]}"

if [ $? -ne 0 ]; then
    echo "Error: Failed to update package $PACKAGE_NAME"
    exit 1
fi

# Deploy the action
$WSK action update "$PACKAGE_NAME/$ACTION_NAME" \
    actions/s3_access_batched.py \
    --kind python:3 \
    --memory 256 \
    --timeout 60000 \
    --web true \
    "${PARAMS[@]}" \
    --annotation require-whisk-auth true

if [ $? -ne 0 ]; then
    echo "Error: Failed to update action $PACKAGE_NAME/$ACTION_NAME"
    exit 1
fi

echo "Action $PACKAGE_NAME/$ACTION_NAME deployed successfully!"
echo "You can invoke it with: $WSK action invoke $PACKAGE_NAME/$ACTION_NAME -r"
echo "Or use the web URL: curl -X GET $(wsk action get "$PACKAGE_NAME/$ACTION_NAME" --url | tail -1)"
echo ""
echo "The action will automatically detect the node's IP by querying the Kubernetes API." 