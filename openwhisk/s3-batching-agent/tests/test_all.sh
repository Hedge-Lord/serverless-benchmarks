#!/bin/bash

# Set up terminal colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${BLUE}=== Testing S3 Batching Agent ===${NC}"

# Give the service some time to fully initialize if needed
echo -e "${YELLOW}Waiting for service to be ready...${NC}"
sleep 2

# Test 1: List Buckets
echo -e "${GREEN}Running List Buckets test...${NC}"
bash test_list_buckets.sh
echo

# Test 2: List Objects
echo -e "${GREEN}Running List Objects test...${NC}"
bash test_list_objects.sh
echo

# Test 3: Get Object
echo -e "${GREEN}Running Get Object test...${NC}"
bash test_get_object.sh
echo

echo -e "${BLUE}=== All tests completed ===${NC}" 