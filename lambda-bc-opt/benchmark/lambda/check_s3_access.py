# In[]:
import boto3
from datetime import datetime, timedelta, UTC
from pytz import utc

# In[]:
target_bucket = "ggtest-mbroughani81-logs"
# start_time = datetime.now(UTC)
# end_time = start_time + timedelta(minutes=20)
end_time = datetime.now(UTC)
start_time = end_time - timedelta(days=6)
lambda_roles = ["arn:aws:sts::741448956691:assumed-role/LambdaRole/gg-lambda-function",
                "arn:aws:iam::741448956691:user/MohammadBroughani"]

print(end_time)
print(start_time)

# In[]:
s3 = boto3.client('s3')
response = s3.list_objects_v2(Bucket=target_bucket, MaxKeys=200)
response_contents = response.get('Contents', [])


# In[]:
def extract_from_response(response):
    log_content = response['Body'].read().decode('utf-8')
    for line in log_content.splitlines():
        log_fields = line.split()
        # print(log_fields)
        accessed_by = log_fields[5]
        access_type = log_fields[7]
        accessed_obj = log_fields[8]
        return (accessed_by, access_type, accessed_obj, log_fields)

# In[]:
result = []
for obj in response_contents:
    key = obj['Key']
    last_modified = obj['LastModified']
    # if start_time <= last_modified <= end_time:
    if start_time <= last_modified:
        response = s3.get_object(Bucket=target_bucket, Key=key)
        t = extract_from_response(response)
        (accessed_by, access_type, accessed_obj, log_fields) = t
        if accessed_by in lambda_roles:
            result.append(t)
            # print(f"{last_modified}: {accessed_obj} {accessed_by}, {access_type}")

# In[]:
def process_result_1(result):
    ans = {}
    for x in result:
        accessed_by = x[0]
        access_type = x[1]
        accessed_obj = x[2]
        k = accessed_obj
        v = (accessed_by, access_type)
        if k not in ans:
            ans[k] = []
        ans[k].append(v)
    return ans

def process_result_2(result):
    ans = {}
    for x in result:
        accessed_by = x[0]
        access_type = x[1]
        accessed_obj = x[2]
        k = (accessed_by, access_type)
        v = accessed_obj
        if k not in ans:
            ans[k] = []
        ans[k].append(v)
    return ans
d1 = process_result_1(result)
d2 = process_result_2(result)
