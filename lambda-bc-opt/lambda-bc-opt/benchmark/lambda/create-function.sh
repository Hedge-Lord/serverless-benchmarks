#!/bin/bash

if [ "$#" -ne 3 ]; then
    echo "Usage: $0 <function-name> <zip-file-name> <security-group>"
    exit 1
fi

FUNCTION_NAME=$1
ZIP_FILE_NAME=$2
SECURITY_GROUP=$3

if [ "$SECURITY_GROUP" == "batch-service-client" ]; then
    SECURITY_GROUP_ID="sg-088c90e01b94d6087"
elif [ "$SECURITY_GROUP" == "redis-client" ]; then
    SECURITY_GROUP_ID="sg-0d5f75228af30b37d"
else
    echo "Invalid security group. Please use 'batch-service-client' or 'redis-client'."
    exit 1
fi

aws lambda create-function --function-name $FUNCTION_NAME \
    --runtime provided.al2023 --handler bootstrap \
    --architectures x86_64 \
    --role arn:aws:iam::741448956691:role/LambdaRole \
    --zip-file fileb://$ZIP_FILE_NAME \
    --vpc-config SubnetIds=subnet-0b92ffc80d59dc87a,subnet-0f43a06abbf588583,subnet-0e3ef02e1cd5584bd,SecurityGroupIds=$SECURITY_GROUP_ID
