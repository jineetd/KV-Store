# Create the kind cluster
kind create cluster
kubectl cluster-info --context kind-kind

# Create the k8s namespace.
kubectl create ns test-ns

# Add the helm charts for etcd.
# NOTE: Bitnami moved many public tags to the legacy repo; the default
# docker.io/bitnami/etcd:<tag> may not exist anymore.
helm upgrade --install -n test-ns etcd bitnami/etcd \
  --set auth.rbac.create=false \
  --set image.repository=bitnamilegacy/etcd

# Load the control-manager and worker docker images into the cluster.
kind load docker-image control-manager:latest
kind load docker-image worker:latest

# Apply the YAML file for deployment.
kubectl -n test-ns apply -f cluster_setup.yaml