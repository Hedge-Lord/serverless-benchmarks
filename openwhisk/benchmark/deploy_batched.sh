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

# Check if AWS credentials are set
if [ -z "$AWS_ACCESS_KEY_ID" ] || [ -z "$AWS_SECRET_ACCESS_KEY" ]; then
    echo "Error: AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY must be set."
    exit 1
fi

# Create the package if it doesn't exist
$WSK package update "$PACKAGE_NAME" \
    --param AWS_ACCESS_KEY_ID "$AWS_ACCESS_KEY_ID" \
    --param AWS_SECRET_ACCESS_KEY "$AWS_SECRET_ACCESS_KEY" \
    --param AWS_REGION "$AWS_REGION" \
    --param BATCHING_AGENT_PORT "$BATCHING_AGENT_PORT"

if [ $? -ne 0 ]; then
    echo "Error: Failed to update package $PACKAGE_NAME"
    exit 1
fi

# Deploy the action with annotation to inject environment variables from the Kubernetes Downward API
$WSK action update "$PACKAGE_NAME/$ACTION_NAME" \
    actions/s3_access_batched.py \
    --kind python:3 \
    --memory 256 \
    --timeout 60000 \
    --web true \
    --env AWS_ACCESS_KEY_ID "$AWS_ACCESS_KEY_ID" \
    --env AWS_SECRET_ACCESS_KEY "$AWS_SECRET_ACCESS_KEY" \
    --env AWS_REGION "$AWS_REGION" \
    --env BATCHING_AGENT_PORT "$BATCHING_AGENT_PORT" \
    --annotation require-whisk-auth true

if [ $? -ne 0 ]; then
    echo "Error: Failed to update action $PACKAGE_NAME/$ACTION_NAME"
    exit 1
fi

echo "Action $PACKAGE_NAME/$ACTION_NAME deployed successfully!"
echo "You can invoke it with: $WSK action invoke $PACKAGE_NAME/$ACTION_NAME -r"
echo "Or use the web URL: curl -X GET $(wsk action get "$PACKAGE_NAME/$ACTION_NAME" --url | tail -1)"
echo ""
echo "Note: The BATCHING_AGENT_HOST environment variable will be set automatically"
echo "to the node's IP address in the OpenWhisk invoker pods using the Kubernetes Downward API." 