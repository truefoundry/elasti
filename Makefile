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
setup-registry: ## Setup docker registery, where we publish our images
	docker run -d -p 5001:5000 --name registry registry:2 

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

