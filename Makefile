GIT_TAG := $(shell git tag --points-at HEAD)
VERSION ?= $(shell echo $${GIT_TAG:-0.0.0} | sed s/v//g)
IMAGE ?= gchr.io/reddec/keycloak-ext-operator:$(VERSION)
LOCALBIN := $(PWD)/.bin
CONTROLLER_GEN := $(LOCALBIN)/controller-gen

info:
	@echo $(IMAGE)

.PHONY: manifests
manifests: $(CONTROLLER_GEN) ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	$(CONTROLLER_GEN) rbac:roleName=manager-role crd webhook paths="./..." output:crd:artifacts:config=config/crd/bases

.PHONY: generate
generate: $(CONTROLLER_GEN) ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

.PHONY: run
run: manifests generate
	direnv exec . go run ./main.go

install: manifests generate
	kubectl apply -k config/crd

.PHONY: install

bundle: manifests generate
	rm -rf build && mkdir build
	cp -rv config ./build/
	cd build/config/default && kustomize edit set image controller=${IMAGE}
	kustomize build build/config/default > build/keycloak-ext-operator.yaml
	rm -rf build/config

.PHONY: bundle

install:
	goreleaser build --rm-dist --snapshot --single-target

$(CONTROLLER_GEN):
	@mkdir -p $(LOCALBIN)
	GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-tools/cmd/controller-gen@v0.9.2