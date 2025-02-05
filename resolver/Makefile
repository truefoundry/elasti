ENVFILE := .env
include $(ENVFILE)
export $(shell sed 's/=.*//' $(ENVFILE))

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

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

.PHONY: test
test: fmt vet ## Run tests.
	go test ./... -race

.PHONY: run
run: ## Run resolver locally
	go run ./cmd/

.PHONY: docker-build
docker-build: ## Build docker image for the resolver
	docker build -t ${IMG} -f ./Dockerfile ../

.PHONY: docker-push
docker-push: ## Publish docker image for the resolver
	docker push ${IMG}

PLATFORMS ?= linux/amd64 #,linux/s390x,linux/ppc64le,linux/arm64,
.PHONY: docker-buildx
docker-buildx: ## Build and push docker image for cross-platform support
	# copy existing Dockerfile and insert --platform=${BUILDPLATFORM} into Dockerfile.cross, and preserve the original Dockerfile
	# sed -e '1 s/\(^FROM\)/FROM --platform=\$$\{BUILDPLATFORM\}/; t' -e ' 1,// s//FROM --platform=\$$\{BUILDPLATFORM\}/' Dockerfile > Dockerfile.cross
	- $(CONTAINER_TOOL) buildx create --name project-resolver-v3-builder
	$(CONTAINER_TOOL) buildx use project-resolver-v3-builder
	- $(CONTAINER_TOOL) buildx build --push --platform=$(PLATFORMS) --tag ${IMG} -f Dockerfile ../
	- $(CONTAINER_TOOL) buildx rm project-resolver-v3-builder
	# rm Dockerfile.cross 

