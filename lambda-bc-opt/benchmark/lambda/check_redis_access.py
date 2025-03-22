# In[]:
import redis
import os
from dotenv import load_dotenv
from datetime import datetime

# In[]:
def get_redis_slowlog(host, port, password):
    try:
        result = []
        r = redis.Redis(host=host, port=port, password=password)

        if r.ping():
            print("Connected to Redis successfully!")
            slowlog = r.slowlog_get(num=1000)
            print(f"Number of SLOWLOG entries: {len(slowlog)}")
            for entry in slowlog:
                id = entry['id']
                timestamp = entry['start_time']
                timestamp =  datetime.fromtimestamp(timestamp).strftime('%Y-%m-%d %H:%M:%S')
                exec_time = entry['duration']
                command = entry['command']
                client_address = entry['client_address'].decode("utf-8")
                client_name = entry['client_name'].decode("utf-8")
                result.append((id,
                              timestamp,
                              exec_time,
                              command,
                              client_address,
                              client_name))
            return result
        else:
            print("Failed to connect to Redis.")
            return []
    except Exception as e:
        print(f"An error occurred: {e}")
        return []
def format_result(result):
    formatted_result = []
    for x in result:
        command_b = x[3]
        parts = command_b.split(b' ');
        command_type = parts[0]
        command_key = parts[1] if len(parts) >= 2 else b''
        command = ""
        if parts[0] in [b'SET', b'GET']:
            command = (command_type.decode("utf-8"), command_key.decode("utf-8"))
        else:
            command = (command_type.decode("utf-8"), command_b)
        formatted_result.append((x[0], x[1], x[2], command, x[4], x[5]))
    return formatted_result


# In[]:
load_dotenv()

redis_host = os.getenv("REDIS_HOST")
redis_port = int(os.getenv("REDIS_PORT"))  # Convert port to integer
redis_password = os.getenv("REDIS_PASSWORD")
result = get_redis_slowlog(redis_host, redis_port, redis_password)


# In[]:
def process_result_1(result):
    ans = {}
    for x in result:
        accessed_by = x[4]
        access_type = x[3][0]
        accessed_obj = x[3][1]
        k = accessed_obj
        v = (accessed_by, access_type)
        if k not in ans:
            ans[k] = []
        ans[k].append(v)
    return ans

def process_result_2(result):
    ans = {}
    for x in result:
        accessed_by = x[4]
        access_type = x[3][0]
        accessed_obj = x[3][1]
        k = (accessed_by, access_type)
        v = accessed_obj
        if k not in ans:
            ans[k] = []
        ans[k].append(v)
    return ans

# In[]:
result[200]

new_result = format_result(result)

get_set_commands = [x for x in new_result if x[3][0] in ["GET", "SET"]]


ff1 = process_result_1(get_set_commands)
ff2 = process_result_2(get_set_commands)
