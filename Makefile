GO ?= go
CONTROLLER_GEN ?= $(shell $(GO) env GOPATH)/bin/controller-gen
CONTROLLER_GEN_VERSION ?= v0.20.0
ENVTEST ?= $(shell $(GO) env GOPATH)/bin/setup-envtest
ENVTEST_K8S_VERSION ?= 1.36.x
BIN ?= bin/bedrock

.PHONY: build test lint generate crds envtest-assets e2e-kind controller-gen

build:
	CGO_ENABLED=0 $(GO) build -trimpath -o $(BIN) ./cmd/bedrock

test: envtest-assets
	KUBEBUILDER_ASSETS="$$($(ENVTEST) use $(ENVTEST_K8S_VERSION) -p path)" $(GO) test ./... -count=1

lint:
	$(GO) vet ./...
	test -z "$$(gofmt -l .)"

controller-gen:
	@test -x $(CONTROLLER_GEN) || $(GO) install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_GEN_VERSION)

generate: controller-gen
	$(CONTROLLER_GEN) object paths=./api/...

crds: controller-gen
	$(CONTROLLER_GEN) crd paths=./api/... output:crd:artifacts:config=manifests/00-crds

envtest-assets:
	@test -x $(ENVTEST) || $(GO) install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest
	@$(ENVTEST) use $(ENVTEST_K8S_VERSION) -p path >/dev/null

e2e-kind: build crds
	hack/e2e-kind.sh
