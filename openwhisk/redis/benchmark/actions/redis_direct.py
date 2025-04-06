#!/usr/bin/env python3
import time
import os
import sys
import json
import socket
import concurrent.futures

# Simple Redis client implementation without redis-py dependency
class SimpleRedisClient:
    def __init__(self, host='localhost', port=6379, password=None, timeout=10, max_retries=2):
        self.host = host
        self.port = port
        self.password = password
        self.timeout = timeout
        self.max_retries = max_retries
        self.socket = None
        
    def connect(self):
        """Connect to Redis server"""
        try:
            if self.socket:
                try:
                    self.socket.close()
                except:
                    pass
                self.socket = None
                
            self.socket = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
            self.socket.settimeout(self.timeout)
            self.socket.connect((self.host, self.port))
            
            # Authenticate if password provided
            if self.password:
                auth_cmd = f"AUTH {self.password}\r\n"
                self.socket.sendall(auth_cmd.encode())
                response = self.socket.recv(1024).decode()
                if not response.startswith('+OK'):
                    raise Exception(f"Authentication failed: {response}")
                
            return True
        except Exception as e:
            print(f"Redis connection error: {str(e)}")
            if self.socket:
                try:
                    self.socket.close()
                except:
                    pass
                self.socket = None
            return False
            
    def close(self):
        """Close the connection"""
        if self.socket:
            try:
                self.socket.close()
            except:
                pass
            self.socket = None
            
    def _send_command(self, command):
        """Send command to Redis server with retry logic"""
        retries = 0
        while retries <= self.max_retries:
            if not self.socket:
                if not self.connect():
                    raise Exception("Not connected to Redis")
            
            try:
                self.socket.sendall(command.encode())
                response = self.socket.recv(1024).decode()
                return response
            except Exception as e:
                retries += 1
                print(f"Redis command error (attempt {retries}/{self.max_retries+1}): {str(e)}")
                self.close()
                
                # Only retry on timeout or connection errors
                if retries <= self.max_retries:
                    print(f"Retrying connection...")
                    time.sleep(0.5)  # Short delay before retry
                else:
                    raise Exception(f"Redis command error after {retries} attempts: {str(e)}")
            
    def ping(self):
        """Test the connection"""
        response = self._send_command("PING\r\n")
        return response.startswith('+PONG')
        
    def get(self, key):
        """Get a value from Redis"""
        command = f"GET {key}\r\n"
        response = self._send_command(command)
        
        if response.startswith('$-1'):
            return None
        
        if response.startswith('$'):
            # Extract the value from bulk string response
            parts = response.split('\r\n')
            if len(parts) >= 2:
                return parts[1]
        
        return None
        
    def set(self, key, value):
        """Set a value in Redis"""
        command = f"SET {key} {value}\r\n"
        response = self._send_command(command)
        return response.startswith('+OK')
        
    def delete(self, key):
        """Delete a key from Redis"""
        command = f"DEL {key}\r\n"
        response = self._send_command(command)
        if response.startswith(':'):
            return int(response[1:].strip())
        return 0
        
    def exists(self, key):
        """Check if key exists in Redis"""
        command = f"EXISTS {key}\r\n"
        response = self._send_command(command)
        if response.startswith(':'):
            return int(response[1:].strip()) > 0
        return False

# Implementation for direct Redis operations
def direct_redis_operation(redis_client, op_type, key, value=None):
    """Perform a Redis operation directly"""
    try:
        if op_type == "get":
            return redis_client.get(key)
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

# Batch Redis operation simulation
def batched_redis_operation(host, port, op_type, key, value=None):
    """Simulate batched Redis operation (without batching agent)"""
    try:
        # Create a temporary client for this operation
        client = SimpleRedisClient(host=host, port=port)
        if not client.connect():
            raise Exception("Failed to connect to Redis for batched operation")
        
        # Perform the operation
        result = direct_redis_operation(client, op_type, key, value)
        client.close()
        return result
    except Exception as e:
        raise Exception(f"Batched Redis operation failed: {str(e)}")

def worker_function(config, use_batching, redis_client, start_idx, end_idx):
    """Worker function to perform Redis operations in parallel"""
    results = []
    success_count = 0
    
    # Create a batching client if needed
    batching_client = None
    if use_batching:
        try:
            batching_client = SimpleRedisClient(
                host=config['redis_host'],
                port=int(config['redis_port']),
                password=config.get('redis_password', ''),
                timeout=10,
                max_retries=2
            )
            batching_client.connect()
        except Exception as e:
            print(f"Failed to create batching client: {str(e)}")
    
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
                # Use batching client (or fallback to temp client if needed)
                if batching_client and batching_client.socket:
                    result_value = direct_redis_operation(
                        batching_client,
                        config['operation_type'],
                        key,
                        value
                    )
                else:
                    # Fallback to creating a temporary client
                    result_value = batched_redis_operation(
                        config['redis_host'],
                        int(config['redis_port']),
                        config['operation_type'],
                        key,
                        value
                    )
                result["value"] = result_value if result_value is not None else ""
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
    
    # Close batching client if created
    if batching_client:
        try:
            batching_client.close()
        except:
            pass
    
    return results, success_count

def main(args):
    """Main function for OpenWhisk action"""
    start_time = time.time()
    
    # Display environment for debugging
    env_info = {
        "activation_id": os.environ.get("__OW_ACTIVATION_ID", "unknown"),
        "pid": os.getpid(),
        "hostname": socket.gethostname()
    }
    
    # Get args with defaults
    config = {
        "num_ops": int(args.get("num_ops", 1)),
        "operation_type": args.get("operation_type", "get"),
        "parallel_calls": int(args.get("parallel_calls", 1)),
        "use_batching": args.get("use_batching", False),
        "key_prefix": args.get("key_prefix", "test_key"),
        "redis_host": args.get("REDIS_HOST", os.environ.get("REDIS_HOST", "localhost")),
        "redis_port": args.get("REDIS_PORT", os.environ.get("REDIS_PORT", 6379)),
        "redis_password": args.get("REDIS_PASSWORD", os.environ.get("REDIS_PASSWORD", ""))
    }
    
    print(f"Starting Redis benchmark with host={config['redis_host']}, " +
          f"port={config['redis_port']}, ops={config['num_ops']}, " +
          f"type={config['operation_type']}, batching={config['use_batching']}, " +
          f"parallel={config['parallel_calls']}")
    
    response = {
        "environment": env_info,
        "config": config,
        "execution_time_ms": 0,
        "success_count": 0,
        "results": []
    }
    
    try:
        # Create Redis client for direct operations
        redis_client = None
        if not config["use_batching"]:
            print(f"Creating direct Redis client to {config['redis_host']}:{config['redis_port']}")
            redis_client = SimpleRedisClient(
                host=config["redis_host"],
                port=int(config["redis_port"]),
                password=config["redis_password"],
                timeout=10,
                max_retries=2
            )
            
            # Test connection
            print("Testing Redis connection...")
            if not redis_client.connect() or not redis_client.ping():
                raise Exception(f"Failed to connect to Redis at {config['redis_host']}:{config['redis_port']}")
            print("Redis connection successful")
        
        # Prepare for parallel execution
        operations_per_worker = max(1, config["num_ops"] // config["parallel_calls"])
        ops_remainder = config["num_ops"] % config["parallel_calls"]
        
        print(f"Executing {config['num_ops']} operations with {config['parallel_calls']} worker(s)")
        
        # Execute operations
        with concurrent.futures.ThreadPoolExecutor(max_workers=config["parallel_calls"]) as executor:
            futures = []
            
            start_idx = 0
            for i in range(config["parallel_calls"]):
                # Distribute remainder ops across workers
                ops_for_this_worker = operations_per_worker
                if i < ops_remainder:
                    ops_for_this_worker += 1
                
                end_idx = start_idx + ops_for_this_worker
                
                if end_idx > start_idx:
                    print(f"Submitting worker {i+1} for operations {start_idx}-{end_idx-1}")
                    futures.append(
                        executor.submit(
                            worker_function,
                            config,
                            config["use_batching"],
                            redis_client,
                            start_idx,
                            end_idx
                        )
                    )
                    
                start_idx = end_idx
            
            # Collect results
            print("Waiting for workers to complete...")
            for future in concurrent.futures.as_completed(futures):
                worker_results, worker_success_count = future.result()
                response["results"].extend(worker_results)
                response["success_count"] += worker_success_count
                
        # Close Redis client if used
        if redis_client:
            print("Closing Redis connection")
            redis_client.close()
            
    except Exception as e:
        print(f"ERROR: {str(e)}")
        response["error"] = str(e)
        response["status"] = "error"
    
    # Calculate total execution time
    execution_time_ms = (time.time() - start_time) * 1000
    response["execution_time_ms"] = execution_time_ms
    
    # Limit results to avoid large responses
    if len(response["results"]) > 10:
        print(f"Truncating {len(response['results'])} results to 10")
        response["results"] = response["results"][:10]
        response["results_truncated"] = True
    
    print(f"Benchmark completed in {execution_time_ms:.2f}ms with {response['success_count']} successful operations")
    return response 