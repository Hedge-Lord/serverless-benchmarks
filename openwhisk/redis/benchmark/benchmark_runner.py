#!/usr/bin/env python3

import argparse
import json
import time
import os
import sys
import statistics
import datetime
import subprocess
from concurrent.futures import ThreadPoolExecutor

# Load environment variables from local.env if it exists
def load_env_file(env_file):
    if not os.path.exists(env_file):
        print(f"Warning: {env_file} not found. Using default values.")
        return {}
    
    env_vars = {}
    with open(env_file, 'r') as f:
        for line in f:
            line = line.strip()
            if line and not line.startswith('#'):
                key, value = line.split('=', 1)
                env_vars[key] = value
    
    return env_vars

# Get script directory
script_dir = os.path.dirname(os.path.abspath(__file__))
local_env = os.path.join(script_dir, 'local.env')
env_vars = load_env_file(local_env)

def invoke_action(action_name, params, blocking=True):
    """Invoke an OpenWhisk action and return the result"""
    cmd = ["wsk", "action", "invoke", "--result"]
    
    if blocking:
        cmd.append("--blocking")
    
    cmd.append(action_name)
    
    if params:
        cmd.extend(["-p", "num_ops", str(params.get("num_ops", 1))])
        cmd.extend(["-p", "operation_type", params.get("operation_type", "set")])
        cmd.extend(["-p", "use_batching", str(params.get("use_batching", False)).lower()])
        cmd.extend(["-p", "parallel_calls", str(params.get("parallel_calls", 1))])
        if "key_prefix" in params:
            cmd.extend(["-p", "key_prefix", params["key_prefix"]])
    
    start_time = time.time()
    try:
        # Set OpenWhisk API host and auth if provided in local.env
        env = os.environ.copy()
        if 'OPENWHISK_APIHOST' in env_vars and env_vars['OPENWHISK_APIHOST']:
            env['WSK_APIHOST'] = env_vars['OPENWHISK_APIHOST']
        if 'OPENWHISK_AUTH' in env_vars and env_vars['OPENWHISK_AUTH']:
            env['WSK_AUTH'] = env_vars['OPENWHISK_AUTH']
            
        result = subprocess.run(cmd, check=True, capture_output=True, text=True, env=env)
        end_time = time.time()
        
        if result.stdout:
            response = json.loads(result.stdout)
            return {
                "status": "success",
                "total_time_ms": (end_time - start_time) * 1000,
                "action_time_ms": response.get("execution_time_ms", 0),
                "success_count": response.get("success_count", 0),
                "response": response
            }
        else:
            return {
                "status": "error",
                "message": "No output from action invocation",
                "total_time_ms": (end_time - start_time) * 1000
            }
    except subprocess.CalledProcessError as e:
        end_time = time.time()
        return {
            "status": "error",
            "message": e.stderr,
            "total_time_ms": (end_time - start_time) * 1000
        }

def run_benchmark(action_name, rate, num_invocations, params):
    """Run the benchmark with the specified parameters"""
    print(f"Running benchmark: {action_name}, Rate: {rate}/sec, Invocations: {num_invocations}")
    print(f"Parameters: {params}")
    
    results = []
    
    with ThreadPoolExecutor(max_workers=rate*2) as executor:
        futures = []
        
        for i in range(num_invocations):
            if i > 0 and i % rate == 0:
                time.sleep(1)  # Wait to maintain the rate
            
            # Submit task to executor
            future = executor.submit(invoke_action, action_name, params)
            futures.append(future)
            
            # Progress update
            if (i+1) % 10 == 0 or (i+1) == num_invocations:
                print(f"Submitted {i+1}/{num_invocations} invocations...")
        
        # Collect results as they complete
        for i, future in enumerate(futures):
            try:
                result = future.result()
                results.append(result)
                if (i+1) % 10 == 0 or (i+1) == num_invocations:
                    print(f"Completed {i+1}/{num_invocations} invocations...")
            except Exception as e:
                print(f"Error in invocation {i}: {str(e)}")
    
    return results

def analyze_results(results):
    """Analyze the benchmark results"""
    # Extract execution times
    action_times = [r.get("action_time_ms", 0) for r in results if r["status"] == "success"]
    total_times = [r.get("total_time_ms", 0) for r in results if r["status"] == "success"]
    
    if not action_times:
        return {
            "success_count": 0,
            "total_count": len(results),
            "message": "No successful invocations to analyze"
        }
    
    # Calculate statistics
    action_stats = {
        "min": min(action_times),
        "max": max(action_times),
        "mean": statistics.mean(action_times),
        "median": statistics.median(action_times),
        "p90": sorted(action_times)[int(len(action_times) * 0.9)],
        "p99": sorted(action_times)[int(len(action_times) * 0.99)] if len(action_times) >= 100 else None
    }
    
    total_stats = {
        "min": min(total_times),
        "max": max(total_times),
        "mean": statistics.mean(total_times),
        "median": statistics.median(total_times),
        "p90": sorted(total_times)[int(len(total_times) * 0.9)],
        "p99": sorted(total_times)[int(len(total_times) * 0.99)] if len(total_times) >= 100 else None
    }
    
    return {
        "success_count": len(action_times),
        "total_count": len(results),
        "action_time_stats": action_stats,
        "total_time_stats": total_stats
    }

def save_results(results, analysis, params, output_file):
    """Save benchmark results to a file"""
    output = {
        "timestamp": datetime.datetime.now().isoformat(),
        "parameters": params,
        "analysis": analysis,
        "raw_results": results
    }
    
    with open(output_file, 'w') as f:
        json.dump(output, f, indent=2)
    
    print(f"Results saved to {output_file}")

def main():
    """Main entry point for the benchmark runner"""
    # Parse command line arguments
    parser = argparse.ArgumentParser(description='Run Redis OpenWhisk benchmarks')
    parser.add_argument('--action', type=str, default='redis_benchmark/redis_benchmark',
                      help='Name of the action to benchmark (default: redis_benchmark/redis_benchmark)')
    parser.add_argument('--rate', type=int, default=10,
                      help='Rate of invocations per second (default: 10)')
    parser.add_argument('--invocations', type=int, default=100,
                      help='Total number of invocations to run (default: 100)')
    parser.add_argument('--ops', type=int, default=10,
                      help='Number of Redis operations per invocation (default: 10)')
    parser.add_argument('--operation', type=str, default='set',
                      help='Redis operation type: get, set, del, exists (default: set)')
    parser.add_argument('--batching', action='store_true',
                      help='Use batching (default: False)')
    parser.add_argument('--parallel', type=int, default=5,
                      help='Parallel calls within each action (default: 5)')
    parser.add_argument('--key-prefix', type=str, default='benchmark',
                      help='Prefix for Redis keys (default: benchmark)')
    parser.add_argument('--output', type=str, default='redis_benchmark_results.json',
                      help='Output file for results (default: redis_benchmark_results.json)')
    parser.add_argument('--env', type=str, default='local.env',
                      help='Path to environment file (default: local.env)')
    
    args = parser.parse_args()

    # Load environment variables
    env_vars = load_env_file(args.env)
    
    # Set up parameters for the action
    params = {
        "num_ops": args.ops,
        "operation_type": args.operation,
        "use_batching": args.batching,
        "parallel_calls": args.parallel,
        "key_prefix": args.key_prefix
    }
    
    # Print benchmark configuration
    print(f"Starting Redis benchmark at {datetime.datetime.now().isoformat()}")
    print(f"Action: {args.action}, Rate: {args.rate}/sec, Invocations: {args.invocations}")
    print(f"Redis Operations: {args.ops}, Operation Type: {args.operation}, Batching: {args.batching}")
    print(f"Parallel Calls: {args.parallel}, Key Prefix: {args.key_prefix}")

    # Run the benchmark
    results = run_benchmark(args.action, args.rate, args.invocations, params)
    
    # Analyze results
    analysis = analyze_results(results)
    
    # Print summary
    print("\nBenchmark Summary:")
    print(f"Successful invocations: {analysis['success_count']}/{analysis['total_count']}")
    print(f"Action execution time (ms):")
    print(f"  Min: {analysis['action_time_stats']['min']:.2f}")
    print(f"  Max: {analysis['action_time_stats']['max']:.2f}")
    print(f"  Mean: {analysis['action_time_stats']['mean']:.2f}")
    print(f"  Median: {analysis['action_time_stats']['median']:.2f}")
    print(f"  90th percentile: {analysis['action_time_stats']['p90']:.2f}")
    if analysis['action_time_stats']['p99'] is not None:
        print(f"  99th percentile: {analysis['action_time_stats']['p99']:.2f}")
    
    print(f"Total invocation time (ms):")
    print(f"  Min: {analysis['total_time_stats']['min']:.2f}")
    print(f"  Max: {analysis['total_time_stats']['max']:.2f}")
    print(f"  Mean: {analysis['total_time_stats']['mean']:.2f}")
    print(f"  Median: {analysis['total_time_stats']['median']:.2f}")
    print(f"  90th percentile: {analysis['total_time_stats']['p90']:.2f}")
    if analysis['total_time_stats']['p99'] is not None:
        print(f"  99th percentile: {analysis['total_time_stats']['p99']:.2f}")
    
    # Save results
    save_results(results, analysis, params, args.output)

if __name__ == '__main__':
    main() 