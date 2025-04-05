import redis
import requests
import time
import os
import sys
import json
import logging
from datetime import datetime
import concurrent.futures
import socket
import ssl

# Configure logging
logging.basicConfig(level=logging.INFO, 
                    format='%(asctime)s - %(levelname)s - %(message)s')
logger = logging.getLogger()

# Global variable to cache node IP
CACHED_NODE_IP = None

def get_node_ip():
    """Get the IP address of the current node, with caching"""
    global CACHED_NODE_IP
    
    # If we already have a cached IP, return it
    if CACHED_NODE_IP:
        logger.info(f"Using cached node IP: {CACHED_NODE_IP}")
        return CACHED_NODE_IP
        
    # Check if batching_agent_host was provided as a parameter
    batching_host = os.environ.get("BATCHING_AGENT_HOST")
    if batching_host:
        logger.info(f"Using BATCHING_AGENT_HOST environment variable: {batching_host}")
        CACHED_NODE_IP = batching_host
        return CACHED_NODE_IP
    
    # Try to get the node IP using Kubernetes API
    try:
        ip = fetch_node_ip_from_kubernetes_api()
        if ip:
            logger.info(f"Successfully retrieved node IP from Kubernetes API: {ip}")
            CACHED_NODE_IP = ip
            return CACHED_NODE_IP
    except Exception as e:
        logger.warning(f"Failed to get node IP from Kubernetes API: {str(e)}")
    
    # Fallback: check for environment variables
    for env_var in ["NODE_IP", "KUBERNETES_NODE_IP", "HOST_IP", "HOSTNAME"]:
        ip = os.environ.get(env_var)
        if ip:
            logger.info(f"Using {env_var} environment variable: {ip}")
            CACHED_NODE_IP = ip
            return CACHED_NODE_IP
    
    # Final fallback: use a default hostname for the node
    logger.warning("No node IP could be determined. Using localhost as fallback.")
    CACHED_NODE_IP = "localhost"
    return CACHED_NODE_IP

def fetch_node_ip_from_kubernetes_api():
    """Fetch the node IP from Kubernetes API"""
    # Get pod name from hostname
    hostname = socket.gethostname()
    
    # Check if service account token exists
    token_file = "/var/run/secrets/kubernetes.io/serviceaccount/token"
    if not os.path.exists(token_file):
        raise Exception("Service account token not found")
    
    # Read the service account token
    with open(token_file, 'r') as f:
        token = f.read()
    
    # Get Kubernetes API server address
    kube_host = os.environ.get("KUBERNETES_SERVICE_HOST")
    kube_port = os.environ.get("KUBERNETES_SERVICE_PORT")
    if not kube_host or not kube_port:
        raise Exception("Kubernetes service host or port not found")
    
    # Read namespace
    namespace_file = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"
    with open(namespace_file, 'r') as f:
        namespace = f.read()
    
    # Create request to Kubernetes API
    url = f"https://{kube_host}:{kube_port}/api/v1/namespaces/{namespace}/pods/{hostname}"
    
    headers = {
        "Authorization": f"Bearer {token}"
    }
    
    logger.info(f"Attempting to query Kubernetes API at: {url}")
    
    # Send request with SSL verification disabled
    session = requests.Session()
    session.verify = False
    
    # Disable SSL warnings
    requests.packages.urllib3.disable_warnings()
    
    response = session.get(url, headers=headers)
    
    if response.status_code != 200:
        raise Exception(f"Failed to get pod info: {response.status_code} - {response.text}")
    
    pod_info = response.json()
    
    # Extract hostIP from status
    if "status" not in pod_info:
        raise Exception("Status not found in pod info")
    
    host_ip = pod_info["status"].get("hostIP")
    if not host_ip:
        raise Exception("hostIP not found in pod status")
    
    return host_ip

def direct_redis_operation(redis_client, op_type, key, value=None):
    """Perform a Redis operation directly"""
    try:
        if op_type == "get":
            result = redis_client.get(key)
            if result is not None:
                result = result.decode('utf-8')
            return result
        elif op_type == "set":
            return redis_client.set(key, value)
        elif op_type == "del":
            return str(redis_client.delete(key))
        elif op_type == "exists":
            return str(redis_client.exists(key))
        else:
            raise ValueError(f"Unsupported operation type: {op_type}")
    except Exception as e:
        raise Exception(f"Redis operation failed: {str(e)}")

def batched_redis_operation(batching_url, op_type, key, value=None):
    """Perform a Redis operation through the batching agent"""
    try:
        if op_type == "get":
            url = f"{batching_url}/redis/get?key={key}"
            method = "GET"
        elif op_type == "set":
            url = f"{batching_url}/redis/set?key={key}&value={value}"
            method = "POST"
        elif op_type == "del":
            url = f"{batching_url}/redis/del?key={key}"
            method = "DELETE"
        elif op_type == "exists":
            url = f"{batching_url}/redis/exists?key={key}"
            method = "GET"
        else:
            raise ValueError(f"Unsupported operation type: {op_type}")
        
        logger.debug(f"Making request to batching agent: {method} {url}")
        
        response = requests.request(method=method, url=url, timeout=5)
        
        logger.debug(f"Received response from batching agent: status={response.status_code}")
        
        if response.status_code != 200:
            logger.error(f"Non-OK response from batching agent: status={response.status_code} body={response.text}")
            raise Exception(f"Request failed with status: {response.status_code}")
        
        result = response.json()
        
        if op_type == "get":
            return_value = result.get("value", "")
        elif op_type == "set":
            return_value = result.get("result", "")
        elif op_type == "del":
            return_value = result.get("deleted", "")
        elif op_type == "exists":
            return_value = result.get("exists", "")
        
        logger.debug(f"Operation {op_type} completed successfully, result: {return_value}")
        return return_value
    
    except Exception as e:
        logger.error(f"Error during batched operation: {str(e)}")
        raise Exception(f"Batched Redis operation failed: {str(e)}")

def worker_function(config, use_batching, batching_url, redis_client, start_idx, end_idx):
    """Worker function to perform Redis operations in parallel"""
    results = []
    success_count = 0
    
    for i in range(start_idx, end_idx):
        key = f"{config['key_prefix']}_{i}"
        value = f"value_{i}"
        
        result = {
            "key": key,
            "status": "success",
            "value": "",
            "error": "",
            "duration_ms": 0
        }
        
        start_time = time.time()
        
        try:
            if use_batching:
                # Use batching agent
                result_value = batched_redis_operation(
                    batching_url, 
                    config['operation_type'], 
                    key, 
                    value
                )
                result["value"] = result_value
                success_count += 1
            else:
                # Direct Redis access
                result_value = direct_redis_operation(
                    redis_client, 
                    config['operation_type'], 
                    key, 
                    value
                )
                result["value"] = result_value if result_value is not None else ""
                success_count += 1
        except Exception as e:
            result["status"] = "error"
            result["error"] = str(e)
        
        result["duration_ms"] = (time.time() - start_time) * 1000
        results.append(result)
    
    return results, success_count

def run_benchmark(config):
    """Run the Redis benchmark with the given configuration"""
    start_time = time.time()
    
    response = {
        "statusCode": 200,
        "execution_time_ms": 0,
        "num_ops": config["num_ops"],
        "operation_type": config["operation_type"],
        "parallel_calls": config["parallel_calls"],
        "use_batching": config["use_batching"],
        "success_count": 0,
        "results": []
    }
    
    # Use default values if not provided
    if not config.get("operation_type"):
        config["operation_type"] = "get"
    if not config.get("num_ops") or config["num_ops"] <= 0:
        config["num_ops"] = 1
    if not config.get("parallel_calls") or config["parallel_calls"] <= 0:
        config["parallel_calls"] = 1
    if not config.get("key_prefix"):
        config["key_prefix"] = "test_key"
    
    redis_client = None
    batching_url = None
    
    try:
        if config["use_batching"]:
            # Use batching agent
            batching_host = config.get("batching_host")
            batching_port = config.get("batching_port", "8080")
            
            # If host not provided, detect node IP
            if not batching_host:
                logger.info("No batching agent host provided, attempting to auto-detect")
                batching_host = get_node_ip()
                logger.info(f"Auto-detected batching agent host: {batching_host}")
            else:
                logger.info(f"Using provided batching agent host: {batching_host}")
            
            batching_url = f"http://{batching_host}:{batching_port}"
            response["batching_url"] = batching_url
            logger.info(f"Using Redis batching agent at {batching_url}")
            
            # Test the connection to the batching agent
            try:
                test_url = f"{batching_url}/health"
                test_response = requests.get(test_url, timeout=2)
                if test_response.status_code != 200:
                    logger.warning(f"Batching agent health check returned non-OK status: {test_response.status_code}")
                else:
                    logger.info(f"Successfully connected to batching agent at {batching_url}")
            except Exception as e:
                logger.warning(f"Failed to connect to batching agent health endpoint: {str(e)}")
        else:
            # Direct Redis access
            redis_host = config.get("redis_host", "localhost")
            redis_port = int(config.get("redis_port", 6379))
            redis_password = config.get("redis_password", None)
            
            redis_client = redis.Redis(
                host=redis_host,
                port=redis_port,
                password=redis_password,
                db=0,
                socket_timeout=5,
                socket_connect_timeout=2
            )
            
            # Test the connection
            try:
                redis_client.ping()
                response["redis_host"] = redis_host
                logger.info(f"Connected to Redis at {redis_host}:{redis_port}")
            except Exception as e:
                error_msg = f"Failed to connect to Redis: {str(e)}"
                logger.error(error_msg)
                response["statusCode"] = 500
                response["error"] = error_msg
                return response
        
        # Run the benchmark operations
        all_results = []
        success_count = 0
        
        # Create worker tasks
        ops_per_worker = max(1, config["num_ops"] // config["parallel_calls"])
        workers_count = min(config["parallel_calls"], config["num_ops"])
        
        with concurrent.futures.ThreadPoolExecutor(max_workers=workers_count) as executor:
            futures = []
            
            for i in range(workers_count):
                start_idx = i * ops_per_worker
                end_idx = min((i + 1) * ops_per_worker, config["num_ops"])
                
                if i == workers_count - 1:
                    end_idx = config["num_ops"]  # Ensure all ops are processed
                
                future = executor.submit(
                    worker_function,
                    config,
                    config["use_batching"],
                    batching_url,
                    redis_client,
                    start_idx,
                    end_idx
                )
                futures.append(future)
            
            # Collect results
            for future in concurrent.futures.as_completed(futures):
                results, worker_success_count = future.result()
                all_results.extend(results)
                success_count += worker_success_count
        
        response["results"] = all_results
        response["success_count"] = success_count
        
    except Exception as e:
        error_msg = f"Benchmark error: {str(e)}"
        logger.error(error_msg)
        response["statusCode"] = 500
        response["error"] = error_msg
    finally:
        # Close Redis client if used
        if redis_client:
            redis_client.close()
    
    response["execution_time_ms"] = (time.time() - start_time) * 1000
    return response

def main(params):
    """Main function for OpenWhisk action"""
    # Log activation info
    activation_id = os.environ.get("__OW_ACTIVATION_ID", "unknown")
    logger.info(f"===== ACTIVATION START {activation_id} =====")
    logger.info(f"Process ID: {os.getpid()}")
    logger.info(f"Container hostname: {socket.gethostname()}")
    logger.info(f"Python version: {sys.version}")
    
    # Extract parameters
    config = {
        "num_ops": int(params.get("num_ops", 1)),
        "operation_type": params.get("operation_type", "get"),
        "use_batching": params.get("use_batching", False),
        "batching_host": params.get("batching_agent_host", os.environ.get("BATCHING_AGENT_HOST", "")),
        "batching_port": params.get("batching_agent_port", os.environ.get("BATCHING_AGENT_PORT", "8080")),
        "redis_host": params.get("REDIS_HOST", ""),
        "redis_port": params.get("REDIS_PORT", "6379"),
        "redis_password": params.get("REDIS_PASSWORD", ""),
        "key_prefix": params.get("key_prefix", "test_key"),
        "parallel_calls": int(params.get("parallel_calls", 1))
    }
    
    # Store batching host in environment if provided
    if config["batching_host"]:
        os.environ["BATCHING_AGENT_HOST"] = config["batching_host"]
    
    # Run the benchmark
    response = run_benchmark(config)
    
    logger.info(f"===== ACTIVATION END {activation_id} =====")
    return response 