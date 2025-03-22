# In[]:
import subprocess
import re
import matplotlib.pyplot as plt
import time
import csv

# In[]:
# run wrk for batchservice
def run_wrk_batchservice(rps, action_url, thread_cnt=10, conn_cnt=20, duration=30):
    """Run wrk2 for a specific RPS and return the latency data."""
    command = f"wrk -t{thread_cnt} -c{conn_cnt} -d{duration}s -R{rps} --latency -s post_request.lua {action_url}"
    result = subprocess.run(command, shell=True, capture_output=True, text=True)
    return result.stdout

# In[]:
# run wrk
def run_wrk(rps, action_url, thread_cnt=10, conn_cnt=20, duration=30):
    """Run wrk2 for a specific RPS and return the latency data."""
    command = f"wrk -t{thread_cnt} -c{conn_cnt} -d{duration}s -R{rps} --latency {action_url}"
    result = subprocess.run(command, shell=True, capture_output=True, text=True)
    return result.stdout


# In[]:
# Function to extract latency percentiles from wrk output
def parse_latency_output(output):
    """Extract 50th, 90th, and 99th percentile latencies from wrk output."""
    latencies = {}
    match_50 = re.search(r'50.000%\s+([0-9\.]+)([a-z]+)', output)
    match_90 = re.search(r'90.000%\s+([0-9\.]+)([a-z]+)', output)
    match_99 = re.search(r'99.000%\s+([0-9\.]+)([a-z]+)', output)

    # Convert to milliseconds
    if match_50:
        latencies['50th'] = convert_to_milliseconds(float(match_50.group(1)), match_50.group(2))
    if match_90:
        latencies['90th'] = convert_to_milliseconds(float(match_90.group(1)), match_90.group(2))
    if match_99:
        latencies['99th'] = convert_to_milliseconds(float(match_99.group(1)), match_99.group(2))

    return latencies

# Helper function to convert latency to milliseconds
def convert_to_milliseconds(value, unit):
    """Convert the latency values to milliseconds."""
    if unit == 'ms':
        return value
    elif unit == 's':
        return value * 1000
    elif unit == 'us':
        return value / 1000
    return value

def plot(rps_values, latency_50th, latency_90th, latency_99th, name):
    plt.figure(figsize=(10, 6))
    plt.plot(rps_values, latency_50th, marker='o', label='50th Percentile')
    plt.plot(rps_values, latency_90th, marker='o', label='90th Percentile')
    plt.plot(rps_values, latency_99th, marker='o', label='99th Percentile')

    plt.xlabel('Requests per Second (RPS)')
    plt.ylabel('Latency (ms)')
    plt.title('Latency Percentiles vs RPS')
    plt.legend()
    plt.grid(True)
    plt.savefig(name)

def export_to_csv(rps_values, latency_50th, latency_90th, latency_99th, filename):
    """Export RPS and latency data to a CSV file."""
    headers = ['RPS', '50th Percentile Latency (ms)', '90th Percentile Latency (ms)', '99th Percentile Latency (ms)']
    with open(filename, mode='w', newline='') as file:
        writer = csv.writer(file)
        writer.writerow(headers)
        for rps, lat_50, lat_90, lat_99 in zip(rps_values, latency_50th, latency_90th, latency_99th):
            writer.writerow([rps, lat_50, lat_90, lat_99])
    print(f"Data successfully exported to {filename}")

def read_from_csv(filename):
    """Read the CSV file and return RPS values and latency percentiles."""
    rps_values = []
    latency_50th = []
    latency_90th = []
    latency_99th = []
    with open(filename, mode='r') as file:
        reader = csv.DictReader(file)
        for row in reader:
            rps_values.append(int(row['RPS']))
            latency_50th.append(float(row['50th Percentile Latency (ms)']))
            latency_90th.append(float(row['90th Percentile Latency (ms)']))
            latency_99th.append(float(row['99th Percentile Latency (ms)']))
    return rps_values, latency_50th, latency_90th, latency_99th

# In[]:
# LOCALLAMBDA
url = "http://localhost:8080/locallambda"
latency_50th = []
latency_90th = []
latency_99th = []
thread_cnt = 10
conn_cnt = 100
rps_values = [1000 * x for x in range(70,75)]
for rps in rps_values:
    print(f"Running wrk2 for {rps} requests per second...")
    output = run_wrk(rps, url, thread_cnt, conn_cnt, 30)
    time.sleep(5)
    latencies = parse_latency_output(output)
    print(f"laaatt => {output}")
    print(f"50th percentile: {latencies.get('50th', 'N/A')} ms")
    print(f"90th percentile: {latencies.get('90th', 'N/A')} ms")
    print(f"99th percentile: {latencies.get('99th', 'N/A')} ms")
    # Append the results
    latency_50th.append(latencies.get('50th', None))
    latency_90th.append(latencies.get('90th', None))
    latency_99th.append(latencies.get('99th', None))
export_to_csv(rps_values, latency_50th, latency_90th, latency_99th, "w3.csv")
plot(rps_values, latency_50th, latency_90th, latency_99th, "w3.png")

# In[]:
# BATCHSERVICE
url = "http://127.0.0.1:8090/get"
latency_50th = []
latency_90th = []
latency_99th = []
thread_cnt = 10
conn_cnt = 100
rps_values = [20000 * x for x in range(10,11)]
for rps in rps_values:
    print(f"Running wrk2 for {rps} requests per second...")
    output = run_wrk_batchservice(rps, url, thread_cnt, conn_cnt, 30)
    time.sleep(5)
    latencies = parse_latency_output(output)
    print(f"laaatt => {output}")
    print(f"50th percentile: {latencies.get('50th', 'N/A')} ms")
    print(f"90th percentile: {latencies.get('90th', 'N/A')} ms")
    print(f"99th percentile: {latencies.get('99th', 'N/A')} ms")
    # Append the results
    latency_50th.append(latencies.get('50th', None))
    latency_90th.append(latencies.get('90th', None))
    latency_99th.append(latencies.get('99th', None))
export_to_csv(rps_values, latency_50th, latency_90th, latency_99th, "batchservice.csv")
plot(rps_values, latency_50th, latency_90th, latency_99th, "batchservice.png")

# In[]:
# Code-level batch call optimization
url = "http://localhost:8080/getterMock"
latency_50th = []
latency_90th = []
latency_99th = []
rps_values = [10000 * x for x in range(9,10)]
thread_cnt = 3
conn_cnt = 10
for rps in rps_values:
    print(f"Running wrk2 for {rps} requests per second...")
    output = run_wrk(rps, url, thread_cnt, conn_cnt)
    time.sleep(20)
    latencies = parse_latency_output(output)
    print(f"laaatt => {output}")
    print(f"50th percentile: {latencies.get('50th', 'N/A')} ms")
    print(f"90th percentile: {latencies.get('90th', 'N/A')} ms")
    print(f"99th percentile: {latencies.get('99th', 'N/A')} ms")
    # Append the results
    latency_50th.append(latencies.get('50th', None))
    latency_90th.append(latencies.get('90th', None))
    latency_99th.append(latencies.get('99th', None))
export_to_csv(rps_values, latency_50th, latency_90th, latency_99th, "getter-mock.csv")
plot(rps_values, latency_50th, latency_90th, latency_99th, "getter-mock.png")

# In[]:
# Code-level batch call optimization
url = "http://localhost:8080/getterNaive"
latency_50th = []
latency_90th = []
latency_99th = []
rps_values = [5000 * x for x in range(1,10)]
thread_cnt = 3
conn_cnt = 10
for rps in rps_values:
    print(f"Running wrk2 for {rps} requests per second...")
    output = run_wrk(rps, url, thread_cnt, conn_cnt)
    time.sleep(20)
    latencies = parse_latency_output(output)
    print(f"laaatt => {output}")
    print(f"50th percentile: {latencies.get('50th', 'N/A')} ms")
    print(f"90th percentile: {latencies.get('90th', 'N/A')} ms")
    print(f"99th percentile: {latencies.get('99th', 'N/A')} ms")
    # Append the results
    latency_50th.append(latencies.get('50th', None))
    latency_90th.append(latencies.get('90th', None))
    latency_99th.append(latencies.get('99th', None))
export_to_csv(rps_values, latency_50th, latency_90th, latency_99th, "getter-naive-pool-20.csv")
plot(rps_values, latency_50th, latency_90th, latency_99th, "getter-naive-pool-20.png")

# In[]:
# Code-level batch call optimization
url = "http://localhost:8080/getterBatched"
latency_50th = []
latency_90th = []
latency_99th = []
rps_values = [5000 * x for x in range(1,10)]
thread_cnt = 3
conn_cnt = 10
for rps in rps_values:
    print(f"Running wrk2 for {rps} requests per second...")
    output = run_wrk(rps, url, thread_cnt, conn_cnt)
    time.sleep(20)
    latencies = parse_latency_output(output)
    print(f"laaatt => {output}")
    print(f"50th percentile: {latencies.get('50th', 'N/A')} ms")
    print(f"90th percentile: {latencies.get('90th', 'N/A')} ms")
    print(f"99th percentile: {latencies.get('99th', 'N/A')} ms")
    # Append the results
    latency_50th.append(latencies.get('50th', None))
    latency_90th.append(latencies.get('90th', None))
    latency_99th.append(latencies.get('99th', None))
export_to_csv(rps_values, latency_50th, latency_90th, latency_99th, "getter-batched-pool-20.csv")
plot(rps_values, latency_50th, latency_90th, latency_99th, "getter-batched-pool-20.png")

# %%
