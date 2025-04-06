#!/usr/bin/env python3
import time
import os
import json
import socket

def main(args):
    """Minimal Redis benchmark action without dependencies"""
    start_time = time.time()
    
    # Display environment for debugging
    env_info = {
        "activation_id": os.environ.get("__OW_ACTIVATION_ID", "unknown"),
        "pid": os.getpid(),
        "hostname": socket.gethostname(),
        "python_path": os.environ.get("PYTHONPATH", ""),
        "sys_path": os.environ.get("PATH", "")
    }
    
    # Get args with defaults
    num_ops = args.get("num_ops", 1)
    operation_type = args.get("operation_type", "get")
    use_batching = args.get("use_batching", False)
    
    # Echo parameters
    params = {
        "num_ops": num_ops,
        "operation_type": operation_type,
        "use_batching": use_batching,
        "redis_host": args.get("REDIS_HOST", os.environ.get("REDIS_HOST", "")),
        "redis_port": args.get("REDIS_PORT", os.environ.get("REDIS_PORT", "6379"))
    }
    
    # Simulate Redis operations
    results = []
    for i in range(num_ops):
        key = f"test_key_{i}"
        value = f"value_{i}" if operation_type == "set" else None
        
        op_result = {
            "key": key,
            "operation": operation_type,
            "success": True,
            "value": value if operation_type == "set" else f"simulated_value_{i}"
        }
        
        results.append(op_result)
    
    # Calculate execution time
    execution_time_ms = (time.time() - start_time) * 1000
    
    return {
        "status": "success",
        "message": "Minimal Redis benchmark (simulation only)",
        "environment": env_info,
        "parameters": params,
        "execution_time_ms": execution_time_ms,
        "success_count": num_ops,
        "results": results[:5]  # Return at most 5 results to avoid large response
    } 