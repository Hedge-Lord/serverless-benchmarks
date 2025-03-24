# CloudLab Setup for OpenWhisk

This guide covers setting up Kubernetes and OpenWhisk on CloudLab nodes.

## 1. Basic Host Prep and Kubernetes Installation

### (a) Load br_netfilter and set sysctls

```bash
sudo -i

modprobe br_netfilter
cat <<EOF | tee /etc/sysctl.d/k8s.conf
net.bridge.bridge-nf-call-iptables = 1
net.bridge.bridge-nf-call-ip6tables = 1
net.ipv4.ip_forward = 1
EOF
sysctl --system
```

### (b) Install containerd

```bash
# Install containerd
apt-get update
apt-get install -y containerd

# Make default config
containerd config default | tee /etc/containerd/config.toml

# (Optional) set SystemdCgroup = true in /etc/containerd/config.toml
# near [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.runc.options]

systemctl restart containerd
systemctl enable containerd
```

### (c) [Optional] Bind-mount containerd on a larger partition

If you need more space:

```bash
# Stop containerd, remove/move old data
systemctl stop containerd
mv /var/lib/containerd /var/lib/containerd.old

mkdir -p /mydata/containerd
mount --bind /mydata/containerd /var/lib/containerd
echo "/mydata/containerd /var/lib/containerd none bind 0 0" >> /etc/fstab

systemctl start containerd
```

### (d) Install kubeadm, kubelet, kubectl

```bash
curl -fsSLo /usr/share/keyrings/kubernetes-archive-keyring.gpg \
    https://packages.cloud.google.com/apt/doc/apt-key.gpg

cat <<EOF | tee /etc/apt/sources.list.d/kubernetes.list
deb [signed-by=/usr/share/keyrings/kubernetes-archive-keyring.gpg] \
  https://pkgs.k8s.io/core:/stable:/v1.28/deb/ /
EOF

apt-get update
apt-get install -y kubelet kubeadm kubectl
apt-mark hold kubelet kubeadm kubectl
```

## 2. kubeadm Init and Calico

### (a) Initialize the cluster on control-plane node

```bash
kubeadm init --pod-network-cidr=192.168.0.0/16 \
    --cri-socket unix:///run/containerd/containerd.sock
```

If the bridging sysctl is missing, you'll get the `bridge-nf-call-iptables does not exist` error.

When it's done, copy admin.conf so kubectl can talk to the cluster:

```bash
mkdir -p $HOME/.kube
cp -i /etc/kubernetes/admin.conf $HOME/.kube/config
# if you're root, just do: export KUBECONFIG=/etc/kubernetes/admin.conf
```

### (b) Install Calico CNI plugin

```bash
kubectl apply -f https://docs.projectcalico.org/manifests/calico.yaml
```

Wait until calico pods are up.

## 3. Join the Worker Node

On worker nodes, set up br_netfilter sysctls same as above, install containerd, kubeadm, etc. Then run the kubeadm join command that kubeadm init gave you, e.g.:

```bash
kubeadm join <masterIP>:6443 --token <...> \
    --discovery-token-ca-cert-hash sha256:<...> \
    --cri-socket unix:///run/containerd/containerd.sock
```

Verify:

```bash
kubectl get nodes
```

Both should be Ready once the CNI is working and the node has no disk/taint issues.

## 4. Label Worker for OpenWhisk, (Optional) Untaint Master

By default, OpenWhisk's invoker requires openwhisk-role=invoker. Label your worker:

```bash
kubectl label node <workerName> openwhisk-role=invoker
```

If you want to allow scheduling on the master, remove its control-plane taint:

```bash
kubectl taint node <masterName> node-role.kubernetes.io/control-plane:NoSchedule-
```

## 5. HostPath PV Setup for OpenWhisk (Storage)

Create a script (like "setup_storage.sh") that:

1. Creates directories on /mydata for each hostPath PV
2. Creates a StorageClass named "hostpath"
3. Creates persistent volumes referencing those directories

```bash
BASE_PATH="/mydata"
NUM_PVS=6

for i in $(seq 1 $NUM_PVS); do
  mkdir -p $BASE_PATH/hostpath-pv$i
  chmod 777 $BASE_PATH/hostpath-pv$i
done

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

for i in $(seq 1 $NUM_PVS); do
  cat <<EOF2 | kubectl apply -f -
apiVersion: v1
kind: PersistentVolume
metadata:
  name: hostpath-pv$i
spec:
  capacity:
    storage: 5Gi
  accessModes:
    - ReadWriteOnce
  persistentVolumeReclaimPolicy: Retain
  storageClassName: hostpath
  hostPath:
    path: "$BASE_PATH/hostpath-pv$i"
EOF2
done
```

## 6. Helm Install OpenWhisk

From your directory with the chart references:

```bash
helm repo add openwhisk https://openwhisk.apache.org/charts
helm repo update

# Example minimal install:
helm install owdev openwhisk/openwhisk -n openwhisk \
  --create-namespace \
  -f mycluster.yaml    # (If you have custom storageClass or other config)
```

Wait for pods:

```bash
kubectl get pods -n openwhisk
```

Eventually they become Running; if the "invoker" is stuck, check that the node is labeled correctly.

## 7. Set apihost and Port-Forward NGINX for wsk
```bash
# This makes OpenWhisk only accessible through the master node. 
# You can replace localhost with <master_node_public_ip> if you want to access it remotely
wsk property set --apihost http://localhost:3233  
```

OpenWhisk typically expects to talk to the NGINX front-end (port 80) inside the cluster:

```bash
kubectl port-forward -n openwhisk service/openwhisk-nginx 3233:80
```

(In a separate shell, keep it running.)

Then configure wsk and test:

```bash
wsk action create hello hello.js
wsk action invoke hello --result
```

It should succeed.
