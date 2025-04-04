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
        cmd.extend(["-p", "num_calls", str(params.get("num_calls", 1))])
        if "bucket" in params:
            cmd.extend(["-p", "bucket", params["bucket"]])
    
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

def run_benchmark(action_name, rate, num_invocations, num_calls):
    """Run the benchmark with the specified parameters"""
    print(f"Running benchmark: {action_name}, Rate: {rate}/sec, Invocations: {num_invocations}, Calls/Invocation: {num_calls}")
    
    params = {"num_calls": num_calls}
    
    # Use bucket from local.env if available
    if 'S3_BUCKET' in env_vars and env_vars['S3_BUCKET']:
        params["bucket"] = env_vars['S3_BUCKET']
    
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

def calculate_percentiles(results):
    """Calculate percentiles from benchmark results"""
    total_times = [r["total_time_ms"] for r in results if r["status"] == "success"]
    action_times = [r["action_time_ms"] for r in results if r["status"] == "success" and "action_time_ms" in r]
    
    if not total_times:
        return {"error": "No successful results to calculate percentiles"}
    
    total_times.sort()
    action_times.sort() if action_times else []
    
    p50_index = int(len(total_times) * 0.5)
    p90_index = int(len(total_times) * 0.9)
    p99_index = int(len(total_times) * 0.99)
    
    result = {
        "total_times": {
            "p50": total_times[p50_index],
            "p90": total_times[p90_index],
            "p99": total_times[p99_index],
            "min": min(total_times),
            "max": max(total_times),
            "mean": statistics.mean(total_times)
        }
    }
    
    if action_times:
        ap50_index = int(len(action_times) * 0.5)
        ap90_index = int(len(action_times) * 0.9)
        ap99_index = int(len(action_times) * 0.99)
        
        result["action_times"] = {
            "p50": action_times[ap50_index],
            "p90": action_times[ap90_index],
            "p99": action_times[ap99_index],
            "min": min(action_times),
            "max": max(action_times),
            "mean": statistics.mean(action_times)
        }
    
    return result

def write_results_to_file(results, percentiles, output_file):
    """Write results to a file"""
    with open(output_file, "w") as f:
        f.write("Benchmark Results\n")
        f.write("================\n\n")
        
        f.write("Percentile,Total Time (ms),Action Time (ms)\n")
        f.write(f"50th,{percentiles['total_times']['p50']:.2f},{percentiles.get('action_times', {}).get('p50', 'N/A')}\n")
        f.write(f"90th,{percentiles['total_times']['p90']:.2f},{percentiles.get('action_times', {}).get('p90', 'N/A')}\n")
        f.write(f"99th,{percentiles['total_times']['p99']:.2f},{percentiles.get('action_times', {}).get('p99', 'N/A')}\n")
        f.write(f"min,{percentiles['total_times']['min']:.2f},{percentiles.get('action_times', {}).get('min', 'N/A')}\n")
        f.write(f"max,{percentiles['total_times']['max']:.2f},{percentiles.get('action_times', {}).get('max', 'N/A')}\n")
        f.write(f"mean,{percentiles['total_times']['mean']:.2f},{percentiles.get('action_times', {}).get('mean', 'N/A')}\n")
        
        f.write("\n\nRaw Results:\n")
        json.dump(results, f, indent=2)
    
    print(f"Results written to {output_file}")

def main():
    """Main entry point for the benchmark runner"""
    # Parse command line arguments
    parser = argparse.ArgumentParser(description='Run OpenWhisk benchmarks')
    parser.add_argument('--action', type=str, required=True,
                      help='Name of the action to benchmark')
    parser.add_argument('--rate', type=int, default=10,
                      help='Rate of invocations per second')
    parser.add_argument('--invocations', type=int, default=100,
                      help='Total number of invocations to run')
    parser.add_argument('--calls', type=int, default=1,
                      help='Number of calls per invocation')
    parser.add_argument('--env', type=str, default='local.env',
                      help='Path to environment file')
    args = parser.parse_args()

    # Load environment variables
    load_env_file(args.env)

    # Get OpenWhisk configuration
    wsk = os.environ.get('WSK', 'wsk')
    namespace = os.environ.get('NAMESPACE', 'guest')
    package = os.environ.get('PACKAGE_NAME', 's3benchmark')
    action = args.action

    # Print benchmark configuration
    print(f"Starting benchmark at {datetime.datetime.now().isoformat()}")
    print(f"Running benchmark: {action}, Rate: {args.rate}/sec, "
          f"Invocations: {args.invocations}, Calls/Invocation: {args.calls}")

    # Run the benchmark
    results = run_benchmark(action, args.rate, args.invocations, args.calls)

    # Print summary
    print("\nBenchmark Summary:")
    print(f"Successful invocations: {sum(1 for r in results if r['status'] == 'success')}/{len(results)}")
    
    # Only print statistics if we have successful invocations
    if sum(1 for r in results if r['status'] == 'success') > 0:
        # Calculate percentiles
        percentiles = calculate_percentiles(results)
        
        # Print statistics
        print("\nStatistics:")
        print(f"50th percentile (total time): {percentiles['total_times']['p50']:.2f} ms")
        print(f"90th percentile (total time): {percentiles['total_times']['p90']:.2f} ms")
        print(f"95th percentile (total time): {percentiles['total_times']['p99']:.2f} ms")
        
        print("\nExecution Times:")
        print(f"50th percentile (execution): {percentiles['action_times']['p50']:.2f} ms")
        print(f"90th percentile (execution): {percentiles['action_times']['p90']:.2f} ms")
        print(f"95th percentile (execution): {percentiles['action_times']['p99']:.2f} ms")
        
        print("\nInit Times:")
        print(f"50th percentile (init): {percentiles['total_times']['p50']:.2f} ms")
        print(f"90th percentile (init): {percentiles['total_times']['p90']:.2f} ms")
        print(f"95th percentile (init): {percentiles['total_times']['p99']:.2f} ms")
    else:
        print("\nNo successful invocations to calculate statistics from.")
        print("Check the action logs for error details.")

    # Write results to file
    write_results_to_file(results, percentiles, "benchmark_results.txt")
    
    print(f"Benchmark completed at {datetime.datetime.now().isoformat()}")

if __name__ == '__main__':
    main() 