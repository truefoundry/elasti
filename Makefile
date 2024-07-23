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


.PHONY: helm-chart 
helm-chart: ## Create Helm chart for the operator
	kustomize build . | helmify ./charts/elasti

.PHONY: generate-manifest
generate-manifest: ## Generate deploy manifest
	kustomize build . > ./install.yaml

.PHONY: deploy
deploy: ## Deploy the operator and resolver
	kubectl apply -f ./install.yaml

.PHONY: undeploy
undeploy: ## Undeploy the operator and resolver
	kubectl delete -f ./install.yaml
