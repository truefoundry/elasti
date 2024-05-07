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

.PHONY: run-activator
run-activator: ## Run activator locally
	go run ./cmd/activator

.PHONY: docker-context 
docker-context: ## Switch to docker desktop context
	docker context use desktop-linux

.PHONY: setup-registry
setup-registry: ## Setup docker registery, where we publish our images
	docker run -d -p 5000:5000 --name registry registry:2 

.PHONY: docker-build-activator
docker-build-activator: ## Build docker image for the activator
	docker build -t activator -f ./build/activator/Dockerfile .          
	docker tag activator localhost:5000/activator

.PHONY: docker-publish
docker-publish-activator: ## Publish docker image for the activator
	docker push localhost:5000/activator:latest



.PHONY: deploy-activator
build-deploy-activator: docker-build-activator docker-publish-activator ## & Deploy to k8s
	kubectl delete -f activator.yaml -n elasti
	kubectl apply -f activator.yaml -n elasti

.PHONY: deploy-activator
deploy-activator: ## Build docker image for activator, publish it, and deploy it on k8s
	kubectl delete -f activator.yaml -n elasti
	kubectl apply -f activator.yaml -n elasti

