#!/bin/bash
# SPDX-license-identifier: Apache-2.0
##############################################################################
# Copyright (c)
# All rights reserved. This program and the accompanying materials
# are made available under the terms of the Apache License, Version 2.0
# which accompanies this distribution, and is available at
# http://www.apache.org/licenses/LICENSE-2.0
##############################################################################

set -o pipefail
set -o errexit
set -o nounset
if [[ "${DEBUG:-false}" == "true" ]]; then
    set -o xtrace
fi

export KIND_CLUSTER_NAME=nsm
kind_node="kindest/node:v1.19.1"

function exit_trap {
    if [[ "${DEBUG:-true}" == "true" ]]; then
        set +o xtrace
    fi
    printf "CPU usage: "
    grep 'cpu ' /proc/stat | awk '{usage=($2+$4)*100/($2+$4+$5)} END {print usage " %"}'
    printf "Memory free(Kb): "
    awk -v low="$(grep low /proc/zoneinfo | awk '{k+=$2}END{print k}')" '{a[$1]=$2}  END{ print a["MemFree:"]+a["Active(file):"]+a["Inactive(file):"]+a["SReclaimable:"]-(12*low);}' /proc/meminfo
    if command -v docker; then
        sudo docker ps
    fi
}

trap exit_trap ERR

# Deploy Kubernetes cluster
if ! sudo kind get clusters | grep -q "$KIND_CLUSTER_NAME"; then
    sudo docker pull "$kind_node"
    cat << EOF | sudo kind create cluster --name $KIND_CLUSTER_NAME --config=-
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
kubeadmConfigPatches:
  - |
    apiVersion: kubeadm.k8s.io/v1beta2
    kind: ClusterConfiguration
    metadata:
      name: config
    apiServer:
      extraArgs:
        "enable-admission-plugins": "NamespaceLifecycle,LimitRanger,ServiceAccount,TaintNodesByCondition,Priority,DefaultTolerationSeconds,DefaultStorageClass,PersistentVolumeClaimResize,MutatingAdmissionWebhook,ValidatingAdmissionWebhook,ResourceQuota"
nodes:
  - role: control-plane
    image: $kind_node
  - role: worker
    image: $kind_node
EOF
    mkdir -p "$HOME/.kube"
    sudo cp /root/.kube/config "$HOME/.kube/config"
    sudo chown -R "$USER" "$HOME/.kube/"
fi
for node in $(kubectl get node -o jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}'); do
    kubectl wait --for=condition=ready "node/$node" --timeout=3m
done
kubectl taint node "$KIND_CLUSTER_NAME-control-plane" node-role.kubernetes.io/master:NoSchedule-

# Deploy NSM services using master branch
# TODO: Use a stable release once it's available
if [ ! -d /opt/nsm ]; then
    sudo git clone --depth 1 https://github.com/networkservicemesh/networkservicemesh /opt/nsm
    sudo chown -R "$USER:" /opt/nsm
fi
pushd /opt/nsm
NSM_NAMESPACE=default SPIRE_ENABLED=false INSECURE=true sudo -E make helm-install-nsm
popd

newgrp docker <<EONG
docker pull gwtester/nse:0.0.1
kind load docker-image gwtester/nse:0.0.1 --name "$KIND_CLUSTER_NAME"
EONG

# Build Webhook container and load the image into KinD cluster
make load

# Generate the TLS Certificates
./scripts/webhook-create-signed-cert.sh

# Create Webhook service
kubectl apply -f deployments/k8s.yml
kubectl rollout status deployment/nse-sidecar-injector-webhook-deployment --timeout=3m

# Register the Mutating Webhook service
< ./deployments/mutatingwebhook.yaml ./scripts/webhook-patch-ca-bundle.sh | kubectl apply -f -

# Enabled NSE sidecar injection in default namespace
kubectl label namespace default nse-sidecar-injection=enabled