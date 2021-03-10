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

function exit_trap {
    if [[ "${DEBUG:-false}" == "true" ]]; then
        set +o xtrace
    fi
    printf "CPU usage: "
    grep 'cpu ' /proc/stat | awk '{usage=($2+$4)*100/($2+$4+$5)} END {print usage " %"}'
    printf "Memory free(Kb): "
    awk -v low="$(grep low /proc/zoneinfo | awk '{k+=$2}END{print k}')" '{a[$1]=$2}  END{ print a["MemFree:"]+a["Active(file):"]+a["Inactive(file):"]+a["SReclaimable:"]-(12*low);}' /proc/meminfo
}

function info {
    _print_msg "INFO" "$1"
}

function error {
    _print_msg "ERROR" "$1"
    exit 1
}

function _print_msg {
    echo "$(date +%H:%M:%S) - $1: $2"
}

function assert_equals {
    local input=$1
    local expected=$2

    if [ "$input" != "$expected" ]; then
        error "Go $input expected $expected"
    fi
}

function assert_contains {
    local input=$1
    local expected=$2

    if ! echo "$input" | grep -q "$expected"; then
        error "Got $input expected $expected"
    fi
}

function assert_non_empty {
    local input=$1

    if [ -z "$input" ]; then
        error "Empty input value"
    fi
}

nse_injector_pod="$(kubectl get pods -l=app=sidecar-injector -o jsonpath='{.items[0].metadata.name}')"

info "Namespace has NSE sidecar injection enabled"
assert_contains "$(kubectl get namespaces default --show-labels --no-headers)" "nse-sidecar-injection=enabled"

info "Verify that NSE Webhook server is up and running"
assert_contains "$(kubectl logs "$nse_injector_pod")" "NSE webhook injector has started"

info "Deploy example pod"
kubectl apply -f test/
trap "kubectl delete -f test/" EXIT

info "Wait for pod's readiness"
kubectl wait --for=condition=ready pods example --timeout=3m

info "Verify that Admission review was received"
assert_contains "$(kubectl logs "$nse_injector_pod")" "Admission Review received"

info "Verify that Admission response was sent"
assert_contains "$(kubectl logs "$nse_injector_pod")" "Admission response added to the admission review"

info "Validate sidecar was injected"
assert_contains "$(kubectl get pods example -o jsonpath='{.spec.containers[*].name}')" "sidecar"
assert_contains "$(kubectl logs example -c sidecar)" "NSE: channel has been successfully advertised, waiting for connection from NSM..."
