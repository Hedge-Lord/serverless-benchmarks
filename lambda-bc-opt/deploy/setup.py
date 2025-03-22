import subprocess
import paramiko
from fabric import Connection, ThreadingGroup
from common import *

def master_conn():
    return Connection(HOST_SERVERS[0], connect_kwargs={"key_filename": KEYFILE})

def main():
    print(f"K => {KEYFILE} {HOST_SERVERS[0]}")
    conn = master_conn()

    try:
        remote_script_path = f"lambda-bc-opt/deploy/setup.sh"

        print(f"Making {remote_script_path} executable...")
        conn.run(f"chmod +x {remote_script_path}")

        print(f"Running {remote_script_path} on master...")
        res = conn.run(f"bash {remote_script_path}", pty=True)
        print("Script Output:\n", res.stdout)

    except Exception as e:
        print(f"Error while installing Docker on master: {e}")

    finally:
        conn.close()

if __name__ == '__main__':
    main()
