# SPDX-license-identifier: Apache-2.0
##############################################################################
# Copyright (c) 2021
# All rights reserved. This program and the accompanying materials
# are made available under the terms of the Apache License, Version 2.0
# which accompanies this distribution, and is available at
# http://www.apache.org/licenses/LICENSE-2.0
##############################################################################

export CGO_ENABLED ?= 0
DOCKER_CMD ?= $(shell which docker 2> /dev/null || which podman 2> /dev/null || echo docker)
KIND_CMD ?= $(shell which kind 2> /dev/null || echo kind)
GO_CMD ?= $(shell which go 2> /dev/null || echo go)
GOLANGCI_VERSION = 1.37.0
IMAGE_NAME=gwtester/nse-sidecar-injector:0.0.2
KIND_CLUSTER_NAME?="nsm"
NAMESPACE_NSE_INJECTION_ENABLED?="default"

test:
	$(GO_CMD) test -v ./...
run:
	$(GO_CMD) run cmd/main.go
.PHONY: build
build:
	sudo -E $(DOCKER_CMD) build -t $(IMAGE_NAME) .
	sudo -E $(DOCKER_CMD) image prune --force
push: test build
	docker-squash $(IMAGE_NAME)
	sudo -E $(DOCKER_CMD) push $(IMAGE_NAME)
load: build
	sudo -E $(KIND_CMD) load docker-image $(IMAGE_NAME) --name $(KIND_CLUSTER_NAME)

bin/golangci-lint: bin/golangci-lint-${GOLANGCI_VERSION}
	@ln -sf golangci-lint-${GOLANGCI_VERSION} bin/golangci-lint
bin/golangci-lint-${GOLANGCI_VERSION}:
	@mkdir -p bin
	curl -sfL https://install.goreleaser.com/github.com/golangci/golangci-lint.sh | bash -s -- -b ./bin/ v${GOLANGCI_VERSION}
	@mv bin/golangci-lint $@
.PHONY: lint
lint: bin/golangci-lint
	bin/golangci-lint run --enable-all ./...

deploy:
	./scripts/webhook-create-signed-cert.sh
	kubectl apply -f deployments/k8s.yml
	< ./deployments/mutatingwebhook.yaml ./scripts/webhook-patch-ca-bundle.sh | kubectl apply -f -
	kubectl label namespace $(NAMESPACE_NSE_INJECTION_ENABLED) nse-sidecar-injection=enabled
undeploy:
	kubectl delete -f deployments/k8s.yml
	< ./deployments/mutatingwebhook.yaml ./scripts/webhook-patch-ca-bundle.sh | kubectl delete -f -
	kubectl label namespace $(NAMESPACE_NSE_INJECTION_ENABLED) nse-sidecar-injection-

system-test:
	@vagrant up --no-destroy-on-error
