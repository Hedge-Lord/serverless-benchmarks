#!/bin/bash

# Number of concurrent requests to make
NUM_REQUESTS=5

# Function to make a single request and log the timing
make_request() {
    local start_time=$(date +%s.%N)
    curl -s http://localhost:8080/s3/listBuckets > /dev/null
    local end_time=$(date +%s.%N)
    local duration=$(echo "$end_time - $start_time" | bc)
    echo "Request $1 completed in ${duration}s"
}

# Make concurrent requests
for i in $(seq 1 $NUM_REQUESTS); do
    make_request $i &
    pids[$i]=$!
done

# Wait for all requests to complete
for pid in ${pids[*]}; do
    wait $pid
done 