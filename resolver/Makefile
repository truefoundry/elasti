ENVFILE := .env
include $(ENVFILE)
export $(shell sed 's/=.*//' $(ENVFILE))

.PHONY: help
help:
	@echo "Available targets:"
	@awk '/^[a-zA-Z0-9_-]+:.*?##/ { \
		nb = index($$0, "##"); \
		target = substr($$0, 1, nb - 2); \
		helpMsg = substr($$0, nb + 3); \
		printf "  %-15s %s\n", target, helpMsg; \
	}' $(MAKEFILE_LIST) | column -s ':' -t

.PHONY: run-resolver
run-resolver: ## Run resolver locally
	go run ./cmd/resolver

.PHONY: docker-build-resolver
docker-build-resolver: ## Build docker image for the resolver
	docker build -t resolver -f ./build/resolver/Dockerfile .          
	docker tag resolver localhost:5001/resolver

.PHONY: docker-publish-resolver
docker-publish-resolver: ## Publish docker image for the resolver
	docker push localhost:5001/resolver:latest

.PHONY: deploy-resolver
deploy-resolver: ## Build docker image for resolver, publish it, and deploy it on k8s
	kubectl apply -f ./infra/resolver-deployment.yaml -n elasti
	kubectl apply -f ./infra/resolver-service.yaml -n elasti

