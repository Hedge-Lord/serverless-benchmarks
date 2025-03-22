import boto3
import time
import os
from datetime import datetime, timedelta, timezone

def main(params):
    start_time = time.time()
    
    # Get parameters or use defaults
    num_calls = params.get('num_calls', 1)
    target_bucket = params.get('bucket', 'ggtest-benchmark-logs')
    
    # Set AWS credentials from parameters
    aws_access_key = params.get('AWS_ACCESS_KEY_ID')
    aws_secret_key = params.get('AWS_SECRET_ACCESS_KEY')
    aws_region = params.get('AWS_REGION', 'us-east-2')
    
    # Setup
    end_time = datetime.now(timezone.utc)
    start_date = end_time - timedelta(days=6)
    
    s3_results = []
    
    # Access S3 storage for the requested number of calls
    for i in range(num_calls):
        s3_results.append(access_s3(target_bucket, start_date, end_time, 
                                    aws_access_key, aws_secret_key, aws_region))
    
    execution_time = time.time() - start_time
    
    return {
        'statusCode': 200,
        'execution_time_ms': execution_time * 1000,
        'num_calls': num_calls,
        'results_count': len(s3_results),
        'results': s3_results
    }

def access_s3(bucket_name, start_date, end_date, aws_access_key=None, aws_secret_key=None, aws_region=None):
    try:
        # Create S3 client with credentials if provided
        if aws_access_key and aws_secret_key:
            s3 = boto3.client(
                's3',
                aws_access_key_id=aws_access_key,
                aws_secret_access_key=aws_secret_key,
                region_name=aws_region
            )
        else:
            # Otherwise use the default credentials
            s3 = boto3.client('s3')
        
        # List objects in the bucket
        response = s3.list_objects_v2(Bucket=bucket_name, MaxKeys=20)
        
        if 'Contents' not in response:
            return {
                'status': 'error',
                'message': f'No objects found in bucket {bucket_name}'
            }
        
        objects = response.get('Contents', [])
        
        # Process and filter objects by date
        filtered_objects = []
        for obj in objects:
            last_modified = obj['LastModified']
            if start_date <= last_modified <= end_date:
                filtered_objects.append({
                    'key': obj['Key'],
                    'last_modified': str(last_modified),
                    'size': obj['Size']
                })
        
        return {
            'status': 'success',
            'objects_count': len(filtered_objects),
            'filtered_objects': filtered_objects[:5]  # Limit results for response size
        }
        
    except Exception as e:
        return {
            'status': 'error',
            'message': str(e)
        } 