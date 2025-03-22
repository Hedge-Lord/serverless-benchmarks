#!/bin/bash
# This script creates directories for hostPath PVs, applies a default StorageClass,
# and creates a specified number of PersistentVolumes for OpenWhisk using loops.

set -e

# Default number of PVs if none is provided
NUM_PVS=${1:-6}

# Base name and base directory
BASE_NAME="hostpath-pv"
BASE_PATH="/mydata"

echo "Setting up $NUM_PVS PersistentVolumes..."

# Loop to create directories for each PV
for (( i=1; i<=NUM_PVS; i++ )); do
    PV_NAME="${BASE_NAME}${i}"
    PV_DIR="${BASE_PATH}/${PV_NAME}"
    if [ ! -d "$PV_DIR" ]; then
        sudo mkdir -p "$PV_DIR"
        sudo chmod 777 "$PV_DIR"
        echo "Created directory: $PV_DIR"
    else
        echo "Directory already exists: $PV_DIR"
    fi
done

echo "Applying default StorageClass..."
# Apply a default StorageClass that uses hostPath (for testing purposes)
cat <<EOF | kubectl apply -f -
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: hostpath
  annotations:
    storageclass.kubernetes.io/is-default-class: "true"
provisioner: kubernetes.io/no-provisioner
volumeBindingMode: WaitForFirstConsumer
EOF

echo "Creating PersistentVolumes..."
# Build the YAML for each PV in a loop
PV_YAML=""
for (( i=1; i<=NUM_PVS; i++ )); do
    PV_NAME="${BASE_NAME}${i}"
    PV_DIR="${BASE_PATH}/${PV_NAME}"
    PV_YAML+="
---
apiVersion: v1
kind: PersistentVolume
metadata:
  name: ${PV_NAME}
spec:
  capacity:
    storage: 5Gi
  accessModes:
    - ReadWriteOnce
  persistentVolumeReclaimPolicy: Retain
  storageClassName: hostpath
  hostPath:
    path: \"${PV_DIR}\"
"
done

# Apply the generated YAML
echo "$PV_YAML" | kubectl apply -f -

echo "Storage setup complete."
kubectl get storageclass
kubectl get pv
