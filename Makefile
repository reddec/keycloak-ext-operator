VERSION ?= $(shell git )0.0.1
LOCALBIN := $(PWD)/.bin
CONTROLLER_GEN := $(LOCALBIN)/controller-gen

.PHONY: manifests
manifests: $(CONTROLLER_GEN) ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	$(CONTROLLER_GEN) rbac:roleName=manager-role crd webhook paths="./..." output:crd:artifacts:config=config/crd/bases

.PHONY: generate
generate: $(CONTROLLER_GEN) ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

.PHONY: run
run: manifests generate
	direnv exec . go run ./main.go

bundle:
	rm -rf build && mkdir build
	kustomize build config/default > build/keycloak-ext-operator.yaml
	cd build && kustomize edit set image controller=${IMG}
.PHONY: bundle

$(CONTROLLER_GEN):
	@mkdir -p $(LOCALBIN)
	GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-tools/cmd/controller-gen@v0.9.2