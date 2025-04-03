import requests
import time
import os
import sys
import json
import traceback
from datetime import datetime, timedelta, timezone

# Cache the node IP after first successful lookup
_cached_node_ip = None

def get_node_ip():
    """Get the node IP by querying the Kubernetes API directly"""
    global _cached_node_ip
    
    # Return cached IP if available
    if _cached_node_ip is not None:
        print(f"Using cached node IP: {_cached_node_ip}")
        return _cached_node_ip
    
    try:
        # Get the pod name from hostname
        pod_name = os.environ.get('HOSTNAME')
        if not pod_name:
            print("HOSTNAME environment variable not found")
            return None

        # Get the service account token
        token_path = '/var/run/secrets/kubernetes.io/serviceaccount/token'
        if not os.path.exists(token_path):
            print(f"Service account token not found at {token_path}")
            return None

        with open(token_path, 'r') as f:
            token = f.read().strip()

        # Get the CA certificate path
        ca_cert = '/var/run/secrets/kubernetes.io/serviceaccount/ca.crt'
        if not os.path.exists(ca_cert):
            print(f"CA certificate not found at {ca_cert}")
            return None

        # Get the Kubernetes service host
        k8s_host = os.environ.get('KUBERNETES_SERVICE_HOST')
        if not k8s_host:
            print("KUBERNETES_SERVICE_HOST environment variable not found")
            return None

        # Get the namespace
        namespace_path = '/var/run/secrets/kubernetes.io/serviceaccount/namespace'
        if not os.path.exists(namespace_path):
            print(f"Namespace file not found at {namespace_path}")
            return None

        with open(namespace_path, 'r') as f:
            namespace = f.read().strip()

        # Construct the API URL
        api_url = f'https://{k8s_host}/api/v1/namespaces/{namespace}/pods/{pod_name}'
        
        # Make the request
        headers = {'Authorization': f'Bearer {token}'}
        response = requests.get(api_url, headers=headers, verify=ca_cert, timeout=5)
        
        if response.status_code != 200:
            print(f"Failed to get pod info: {response.status_code} - {response.text}")
            return None

        # Parse the response
        pod_info = response.json()
        node_ip = pod_info.get('status', {}).get('hostIP')
        
        if not node_ip:
            print("No hostIP found in pod status")
            return None

        print(f"Successfully retrieved node IP: {node_ip}")
        # Cache the IP for future invocations
        _cached_node_ip = node_ip
        return node_ip

    except Exception as e:
        print(f"Error getting node IP: {str(e)}")
        return None

def main(params):
    """Main entry point for the OpenWhisk action"""
    start_time = time.time()
    
    try:
        # Get parameters or use defaults
        num_calls = int(params.get('num_calls', 1))
        target_bucket = params.get('bucket', 'ow-benchmark-test')
        
        # Get batching agent endpoint 
        batching_agent_host = get_node_ip()
        if not batching_agent_host:
            print("Failed to get node IP, using default")
            batching_agent_host = "node0.ggz-248982.ucla-progsoftsys-pg0.utah.cloudlab.us"
            
        batching_agent_port = params.get('BATCHING_AGENT_PORT', '8080')
        batching_agent_url = f"http://{batching_agent_host}:{batching_agent_port}"
        
        # Print the agent URL for debugging
        print(f"Using batching agent at {batching_agent_url}")
        
        # Validate AWS credentials
        aws_access_key = params.get('AWS_ACCESS_KEY_ID')
        aws_secret_key = params.get('AWS_SECRET_ACCESS_KEY')
        if not aws_access_key or not aws_secret_key:
            return {
                'statusCode': 400,
                'error': 'Missing AWS credentials',
                'message': 'AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY must be provided'
            }
        
        # Set AWS region
        aws_region = params.get('AWS_REGION', 'us-east-1')
        
        # Setup
        end_time = datetime.now(timezone.utc)
        start_date = end_time - timedelta(days=6)
        
        s3_results = []
        
        # Access S3 storage through the batching agent for the requested number of calls
        for i in range(num_calls):
            result = access_s3_through_agent(batching_agent_url, target_bucket, start_date, end_time)
            s3_results.append(result)
        
        execution_time = time.time() - start_time
        
        return {
            'statusCode': 200,
            'execution_time_ms': execution_time * 1000,
            'num_calls': num_calls,
            'results_count': len(s3_results),
            'batching_agent_url': batching_agent_url,
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

def extract_from_response(response_content):
    """Extract information from the S3 object content"""
    try:
        log_content = response_content
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

def access_s3_through_agent(agent_url, bucket_name, start_date, end_time):
    try:
        # First list the buckets using the batching agent
        buckets_response = requests.get(f"{agent_url}/s3/listBuckets")
        if buckets_response.status_code != 200:
            return {
                'status': 'error',
                'message': f'Failed to list buckets through batching agent: {buckets_response.text}',
                'status_code': buckets_response.status_code
            }
        
        buckets_data = buckets_response.json()
        buckets = [bucket['Name'] for bucket in buckets_data.get('Buckets', [])]
        
        # Check if our target bucket exists
        if bucket_name not in buckets:
            return {
                'status': 'error',
                'message': f'Bucket {bucket_name} not found. Available buckets: {buckets}'
            }
        
        # List objects in the bucket (limit to 20 for benchmark purposes)
        try:
            list_objects_response = requests.get(
                f"{agent_url}/s3/listObjects?bucket={bucket_name}&maxKeys=20"
            )
            if list_objects_response.status_code != 200:
                return {
                    'status': 'error',
                    'operation': 'list_objects',
                    'message': f'Failed to list objects: {list_objects_response.text}',
                    'status_code': list_objects_response.status_code
                }
            
            response = list_objects_response.json()
        except Exception as e:
            return {
                'status': 'error',
                'operation': 'list_objects',
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
            # Convert the LastModified string to datetime
            last_modified_str = obj.get('LastModified')
            if last_modified_str:
                try:
                    # First try with microseconds format
                    last_modified = datetime.strptime(last_modified_str, "%Y-%m-%dT%H:%M:%S.%fZ").replace(tzinfo=timezone.utc)
                except ValueError:
                    try:
                        # If that fails, try without microseconds
                        last_modified = datetime.strptime(last_modified_str, "%Y-%m-%dT%H:%M:%SZ").replace(tzinfo=timezone.utc)
                    except ValueError:
                        # If all else fails, just use current time and log the error
                        last_modified = datetime.now(timezone.utc)
                        print(f"Error parsing timestamp: {last_modified_str}")
                
                if start_date <= last_modified <= end_time:
                    object_details.append({
                        'key': obj['Key'],
                        'last_modified': last_modified_str,
                        'size': obj.get('Size', 0)
                    })
                    
                    try:
                        # Get the object content through batching agent
                        obj_response = requests.get(
                            f"{agent_url}/s3/getObject?bucket={bucket_name}&key={obj['Key']}"
                        )
                        if obj_response.status_code != 200:
                            extracted_results.append(("error", "get_object", f"Status code: {obj_response.status_code}", []))
                            continue
                        
                        # Extract information from the object
                        extracted_info = extract_from_response(obj_response.text)
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