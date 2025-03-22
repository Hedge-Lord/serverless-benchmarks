#!/bin/bash
# Script to package and deploy the full S3 access action

set -e

# Create a temporary directory
TEMP_DIR=$(mktemp -d)
cd $TEMP_DIR

# Create a virtual environment
echo "Creating virtual environment..."
python3 -m venv venv
source venv/bin/activate

# Install boto3 version compatible with Python 3.7
echo "Installing boto3 compatible with Python 3.7..."
pip install 'boto3<1.26.0' 'botocore<1.29.0'

# Create the action directory
mkdir -p action

# Create the main action file
cat > action/__main__.py << 'EOF'
import boto3
import time
import os
import sys
import traceback
from datetime import datetime, timedelta, timezone

def main(params):
    """Main entry point for the OpenWhisk action"""
    start_time = time.time()
    
    try:
        # Get parameters or use defaults
        num_calls = params.get('num_calls', 1)
        target_bucket = params.get('bucket', 'ow-benchmark-test')
        
        # Set AWS credentials from parameters
        aws_access_key = params.get('AWS_ACCESS_KEY_ID')
        aws_secret_key = params.get('AWS_SECRET_ACCESS_KEY')
        aws_region = params.get('AWS_REGION', 'us-east-1')
        
        # Validate credentials
        if not aws_access_key or not aws_secret_key:
            return {
                'statusCode': 400,
                'error': 'Missing AWS credentials',
                'message': 'AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY must be provided'
            }
        
        # Setup
        end_time = datetime.now(timezone.utc)
        start_date = end_time - timedelta(days=6)
        
        s3_results = []
        
        # Access S3 storage for the requested number of calls
        for i in range(num_calls):
            result = access_s3(target_bucket, start_date, end_time, 
                              aws_access_key, aws_secret_key, aws_region)
            s3_results.append(result)
        
        execution_time = time.time() - start_time
        
        return {
            'statusCode': 200,
            'execution_time_ms': execution_time * 1000,
            'num_calls': num_calls,
            'results_count': len(s3_results),
            'boto3_version': boto3.__version__,
            'python_version': f"{sys.version_info.major}.{sys.version_info.minor}.{sys.version_info.micro}",
            'results': s3_results
        }
    except Exception as e:
        # Catch any unexpected errors and return them
        execution_time = time.time() - start_time
        exc_type, exc_value, exc_traceback = sys.exc_info()
        traceback_details = traceback.format_exception(exc_type, exc_value, exc_traceback)
        
        return {
            'statusCode': 500,
            'error': str(e),
            'execution_time_ms': execution_time * 1000,
            'traceback': traceback_details
        }

def extract_from_response(response):
    """Extract information from the S3 object content"""
    try:
        log_content = response['Body'].read().decode('utf-8')
        for line in log_content.splitlines():
            if not line.strip():
                continue
                
            # Try to parse log fields (assuming space-separated format)
            log_fields = line.split()
            if len(log_fields) < 9:  # Need at least 9 fields for our extraction
                continue
                
            accessed_by = log_fields[5]
            access_type = log_fields[7]
            accessed_obj = log_fields[8]
            return (accessed_by, access_type, accessed_obj, log_fields)
    except Exception as e:
        return ("error", "error", str(e), [])
    
    return ("unknown", "unknown", "no-content", [])

def process_results(results):
    """Process the extracted results into summary dictionaries"""
    # Process results by accessed object
    objects_dict = {}
    for result in results:
        accessed_by = result[0]
        access_type = result[1]
        accessed_obj = result[2]
        
        key = accessed_obj
        value = (accessed_by, access_type)
        
        if key not in objects_dict:
            objects_dict[key] = []
        objects_dict[key].append(value)
    
    # Process results by accessor and action type
    accessors_dict = {}
    for result in results:
        accessed_by = result[0]
        access_type = result[1]
        accessed_obj = result[2]
        
        key = (accessed_by, access_type)
        value = accessed_obj
        
        if key not in accessors_dict:
            accessors_dict[key] = []
        accessors_dict[key].append(value)
    
    return {
        "by_object": objects_dict,
        "by_accessor": accessors_dict
    }

def access_s3(bucket_name, start_date, end_date, aws_access_key=None, aws_secret_key=None, aws_region=None):
    try:
        # Create S3 client with credentials
        s3 = boto3.client(
            's3',
            aws_access_key_id=aws_access_key,
            aws_secret_access_key=aws_secret_key,
            region_name=aws_region
        )
        
        # First list the buckets (just for verification)
        buckets_response = s3.list_buckets()
        buckets = [bucket['Name'] for bucket in buckets_response['Buckets']]
        
        # Check if our target bucket exists
        if bucket_name not in buckets:
            return {
                'status': 'error',
                'message': f'Bucket {bucket_name} not found. Available buckets: {buckets}'
            }
        
        # List objects in the bucket (limit to 20 for benchmark purposes)
        try:
            response = s3.list_objects_v2(Bucket=bucket_name, MaxKeys=20)
        except Exception as e:
            return {
                'status': 'error',
                'operation': 'list_objects_v2',
                'message': str(e)
            }
        
        if 'Contents' not in response:
            return {
                'status': 'success',
                'message': f'No objects found in bucket {bucket_name}',
                'buckets': buckets
            }
        
        objects = response.get('Contents', [])
        
        # For benchmark consistency, limit objects to 5 max
        if len(objects) > 5:
            objects = objects[:5]
        
        # Process and filter objects by date, then get and process their content
        extracted_results = []
        object_details = []
        
        for obj in objects:
            last_modified = obj['LastModified']
            if start_date <= last_modified <= end_date:
                object_details.append({
                    'key': obj['Key'],
                    'last_modified': str(last_modified),
                    'size': obj['Size']
                })
                
                try:
                    # Get the object content
                    obj_response = s3.get_object(Bucket=bucket_name, Key=obj['Key'])
                    # Extract information from the object
                    extracted_info = extract_from_response(obj_response)
                    if extracted_info:
                        extracted_results.append(extracted_info)
                except Exception as e:
                    # If we can't get the object, add an error entry
                    extracted_results.append(("error", "get_object", str(e), []))
        
        # Process the results into summary dictionaries if we have any
        processed_results = process_results(extracted_results) if extracted_results else {
            "by_object": {},
            "by_accessor": {}
        }
        
        return {
            'status': 'success',
            'bucket': bucket_name,
            'objects_count': len(objects),
            'objects': object_details,
            'processed_count': len(extracted_results),
            'sample_results': extracted_results[:3] if extracted_results else [],
            'processed_summary': {
                'objects_count': len(processed_results['by_object']),
                'accessors_count': len(processed_results['by_accessor'])
            }
        }
        
    except Exception as e:
        exc_type, exc_value, exc_traceback = sys.exc_info()
        traceback_details = traceback.format_exception(exc_type, exc_value, exc_traceback)
        
        return {
            'status': 'error',
            'message': str(e),
            'traceback': traceback_details
        }
EOF

# Copy dependencies from site-packages
echo "Copying dependencies..."
cp -R venv/lib/python*/site-packages/* action/

# Create zip package
echo "Creating action package..."
cd action
zip -r ../s3_action.zip *
cd ..

# Load environment variables if local.env exists
if [ -f ../local.env ]; then
    source ../local.env
fi

# Deploy the action
echo "Deploying s3-access action..."
wsk action update s3-access s3_action.zip --kind python:3 \
    --memory 512 --timeout 60000 \
    --param AWS_ACCESS_KEY_ID "${AWS_ACCESS_KEY_ID:-}" \
    --param AWS_SECRET_ACCESS_KEY "${AWS_SECRET_ACCESS_KEY:-}" \
    --param AWS_REGION "${AWS_REGION:-us-east-1}" \
    --param bucket "${S3_BUCKET:-ow-benchmark-test}"

# Clean up
cd ..
rm -rf $TEMP_DIR

echo "Done! Invoke the action with:"
echo "wsk action invoke s3-access --blocking --result"
echo ""
echo "For multiple calls:"
echo "wsk action invoke s3-access --blocking --result --param num_calls 5" 