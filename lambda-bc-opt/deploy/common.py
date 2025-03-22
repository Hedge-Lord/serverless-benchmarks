import os
import xml.etree.ElementTree as ET
import subprocess

def project_path():
    return os.popen("git rev-parse --show-toplevel --show-superproject-working-tree").read().strip()

PROJECT_PATH = project_path()

def addresses_from_manifest(manifest_file: str) -> "list[str]":
    tree = ET.parse(manifest_file)
    root = tree.getroot()
    addresses = []
    for child in root:
        # print(child.tag)
        if child.tag.endswith("node"):
            component_id = child.attrib["component_id"]
            # print(component_id)
            node_name = component_id.split("+")[-1]
            location = component_id.split("+")[1]
            address = f'{node_name}.{location}'
            # print(address)
            addresses.append(address)
    return addresses

def username():
    try:
        with open(f'{PROJECT_PATH}/deploy/cloudlab-username', 'r') as file:
            username = file.read().strip()
        return username
    except FileNotFoundError:
        return "Error: 'cloudlab-username' file not found."
    except Exception as e:
        return f"Error reading file: {str(e)}"

SERVERS = addresses_from_manifest(f'{PROJECT_PATH}/manifest.xml')
USERNAME = username()

print("PROJECT_PATH => " + PROJECT_PATH)
print("SERVERS => " + SERVERS[0])
print("USERNAME => " + USERNAME)
HOST_SERVERS = [f'{USERNAME}@{s}' for s in SERVERS]

def run_shell(cmd):
    res = subprocess.run(cmd, stdout=subprocess.PIPE, shell=True)
    return res.stdout.decode('utf-8').strip()

KEYFILE = os.getenv("private_key")
print("KEYFILE => " + KEYFILE)
