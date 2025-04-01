#!/bin/bash

# Set up terminal colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Number of concurrent requests to make
NUM_REQUESTS=5

echo -e "${BLUE}=== Testing Redis Batching Agent ===${NC}"

# Test health endpoint
echo -e "${YELLOW}Testing health endpoint...${NC}"
curl -s http://localhost:8080/health
echo

# Test SET operation - fill with some test data
echo -e "${YELLOW}Setting test values...${NC}"
for i in $(seq 1 3); do
    curl -s -X POST "http://localhost:8080/redis/set?key=testkey$i&value=testvalue$i"
    echo
done

# Function to make a single GET request and log the timing
make_get_request() {
    local key=$1
    local id=$2
    local start_time=$(date +%s.%N)
    curl -s "http://localhost:8080/redis/get?key=$key" > /dev/null
    local end_time=$(date +%s.%N)
    local duration=$(echo "$end_time - $start_time" | bc)
    echo "GET Request $id completed in ${duration}s"
}

# Test concurrent GET requests (should be batched)
echo -e "${GREEN}Testing concurrent GET requests (should be batched)...${NC}"
for i in $(seq 1 $NUM_REQUESTS); do
    make_get_request "testkey1" $i &
    pids[$i]=$!
done

# Wait for all requests to complete
for pid in ${pids[*]}; do
    wait $pid
done

# Test EXISTS operation
echo -e "${YELLOW}Testing EXISTS operation...${NC}"
curl -s "http://localhost:8080/redis/exists?key=testkey1"
echo

# Function to make a single DEL request and log the timing
make_del_request() {
    local key=$1
    local id=$2
    local start_time=$(date +%s.%N)
    curl -s -X DELETE "http://localhost:8080/redis/del?key=$key" > /dev/null
    local end_time=$(date +%s.%N)
    local duration=$(echo "$end_time - $start_time" | bc)
    echo "DEL Request $id completed in ${duration}s"
}

# Test concurrent DEL requests (should be batched)
echo -e "${GREEN}Testing concurrent DEL requests (should be batched)...${NC}"
for i in $(seq 1 $NUM_REQUESTS); do
    make_del_request "testkey$i" $i &
    pids[$i]=$!
done

# Wait for all requests to complete
for pid in ${pids[*]}; do
    wait $pid
done

echo -e "${BLUE}=== All tests completed ===${NC}" 