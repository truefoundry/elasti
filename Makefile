CONTAINER_TOOL ?= docker

.PHONY: help
help:
	@echo "Available targets:"
	@awk '/^[a-zA-Z0-9_-]+:.*?##/ { \
		nb = index($$0, "##"); \
		target = substr($$0, 1, nb - 2); \
		helpMsg = substr($$0, nb + 3); \
		printf "  %-15s %s\n", target, helpMsg; \
	}' $(MAKEFILE_LIST) | column -s ':' -t

.PHONY: generate-manifest
generate-manifest: ## Generate deploy manifest
	kustomize build . > ./install.yaml

.PHONY: setup-registry
setup-registry: ## Setup docker registry, where we publish our images
	docker run -d -p 5001:5000 --name registry registry:2

.PHONY: stop-registry
stop-registry: ## Stop docker registry
	docker stop registry


.PHONY: deploy
deploy: ## Deploy the operator and resolver
	kubectl apply -f ./install.yaml

.PHONY: undeploy
undeploy: ## Undeploy the operator and resolver
	kubectl delete -f ./install.yaml

.PHONY: test
test: test-operator test-resolver test-pkg ## Run all tests

.PHONY: test-operator
test-operator: ## Run operator tests
	cd operator && make test

.PHONY: test-resolver
test-resolver: ## Run resolver tests
	cd resolver && make test

.PHONY: test-pkg
test-pkg: ## Run pkg tests
	cd pkg && make test

.PHONY: serve-docs
serve-docs: ## Serve docs
	@command -v mkdocs >/dev/null 2>&1 || { \
	  echo "mkdocs not found - please install it (pip install mkdocs-material)"; exit 1; } ; \
	mkdocs serve

.PHONY: build-docs
build-docs: ## Build docs
	@command -v mkdocs >/dev/null 2>&1 || { \
	  echo "mkdocs not found - please install it (pip install mkdocs-material)"; exit 1; } ; \
	mkdocs build

.PHONY: fetch-contributors
fetch-contributors: ## Fetch contributors
	python3 docs/scripts/fetch_contributors.py


# If you wish to build the manager image targeting other platforms you can use the --platform flag.
# (i.e. docker build --platform linux/arm64). However, you must enable docker buildKit for it.
# More info: https://docs.docker.com/develop/develop-images/build_enhancements/
.PHONY: docker-build
docker-build: ## Build docker image with the manager.
	$(CONTAINER_TOOL) build -t ${IMG} -f ./Dockerfile .

.PHONY: docker-push
docker-push: ## Push docker image with the manager.
	$(CONTAINER_TOOL) push ${IMG}

# PLATFORMS defines the target platforms for the manager image be built to provide support to multiple
# architectures. (i.e. make docker-buildx IMG=myregistry/mypoperator:0.0.1). To use this option you need to:
# - be able to use docker buildx. More info: https://docs.docker.com/build/buildx/
# - have enabled BuildKit. More info: https://docs.docker.com/develop/develop-images/build_enhancements/
# - be able to push the image to your registry (i.e. if you do not set a valid value via IMG=<myregistry/image:<tag>> then the export will fail)
# To adequately provide solutions that are compatible with multiple platforms, you should consider using this option.
PLATFORMS ?= linux/amd64 #,linux/s390x,linux/ppc64le,linux/arm64,
.PHONY: docker-buildx
docker-buildx: ## Build and push docker image for the manager for cross-platform support
	# copy existing Dockerfile and insert --platform=${BUILDPLATFORM} into Dockerfile.cross, and preserve the original Dockerfile
	#sed -e '1 s/\(^FROM\)/FROM --platform=\$$\{BUILDPLATFORM\}/; t' -e ' 1,// s//FROM --platform=\$$\{BUILDPLATFORM\}/' Dockerfile > Dockerfile.cross
	- $(CONTAINER_TOOL) buildx create --name project-v3-builder
	$(CONTAINER_TOOL) buildx use project-v3-builder
	- $(CONTAINER_TOOL) buildx build --push --platform=$(PLATFORMS) --tag ${IMG} -f Dockerfile ../
	- $(CONTAINER_TOOL) buildx rm project-v3-builder
	#rm Dockerfile.cross
