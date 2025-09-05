# Create the kind cluster
kind create cluster
kubectl cluster-info --context kind-kind

# Create the k8s namespace.
kubectl create ns test-ns

# Add the helm charts for etcd.
helm install -n test-ns etcd bitnami/etcd --set auth.rbac.create=false

# Load the control-manager and worker docker images into the cluster.
kind load docker-image control-manager:latest
kind load docker-image worker:latest

# Apply the YAML file for deployment.
kubectl -n test-ns apply -f cluster_setup.yaml