# OpenWhisk S3 Access Benchmark

This benchmark is designed to test the performance of OpenWhisk actions accessing S3 storage, similar to the AWS Lambda implementation in the lambda-bc-opt directory.

## Setup

### Prerequisites

- OpenWhisk CLI (`wsk`) configured with your OpenWhisk deployment
- Python 3.6+ with pip
- AWS credentials configured for S3 access

### Configuration

1. Create a `local.env` file from the template:

```bash
cp template.local.env local.env
```

2. Edit the `local.env` file with your AWS credentials and other configuration:

```
# AWS credentials
AWS_ACCESS_KEY_ID=your_access_key_here
AWS_SECRET_ACCESS_KEY=your_secret_key_here
AWS_REGION=us-east-2

# S3 configuration
S3_BUCKET=your_bucket_name

# OpenWhisk configuration
OPENWHISK_APIHOST=your_openwhisk_host
OPENWHISK_AUTH=your_auth_key
```

### Installation

1. Deploy the S3 access action:

```bash
chmod +x deploy.sh
./deploy.sh s3-access
```

This will create an OpenWhisk action named `s3-access` that performs S3 operations using the credentials from your `local.env` file.

## Running the Benchmark

Use the benchmark runner script to invoke the action multiple times and collect performance metrics:

```bash
python benchmark_runner.py --action s3-access --rate 10 --invocations 100 --calls 1
```

The runner will automatically use settings from your `local.env` file.

### Parameters

- `--action`: Name of the OpenWhisk action to invoke (default: `s3-access`)
- `--rate`: Rate of invocations per second (default: 10)
- `--invocations`: Total number of invocations to perform (default: 100)
- `--calls`: Number of S3 calls per invocation (default: 1)
- `--output`: Output file for results (default: `benchmark_results.txt`)
- `--bucket`: Override the S3 bucket name from local.env

## Results

The benchmark will generate a results file containing:

- Execution time percentiles (50th, 90th, 99th)
- Min, max, and mean execution times
- Raw results for all invocations

## Advanced Configuration

If you don't want to use the `local.env` file, you can also provide AWS credentials and configure S3 directly when creating the OpenWhisk action:

```bash
wsk action create s3-access s3_action.zip \
    --param AWS_ACCESS_KEY_ID your_access_key \
    --param AWS_SECRET_ACCESS_KEY your_secret_key \
    --param AWS_REGION your_region \
    --param bucket your_bucket_name
``` 