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

.PHONY: run-load-generator
run-load-generator: ## Run load-generator locally
	go run ./cmd/load-generator

.PHONY: run-target
run-target: ## Run target locally
	go run ./cmd/target

.PHONY: docker-context 
docker-context: ## Switch to docker desktop context
	docker context use desktop-linux

.PHONY: setup-registry
setup-registry: ## Setup docker registery, where we publish our images
	docker run -d -p 5001:5000 --name elasti-registry registry:2 

.PHONY: docker-build-load-generator
docker-build-load-generator: ## Build docker image for the load-generator
	docker build -t load-generator -f ./build/load-generator/Dockerfile .          
	docker tag load-generator localhost:5001/load-generator

.PHONY: docker-publish-load-generator
docker-publish-load-generator: ## Publish docker image for the load-generator
	docker push localhost:5001/load-generator:latest

.PHONY: docker-build-target
docker-build-target: ## Build docker image for the target
	docker build -t target -f ./build/target/Dockerfile .          
	docker tag target localhost:5001/target

.PHONY: docker-publish-target
docker-publish-target: ## Publish docker image for the target
	docker push localhost:5001/target:latest

.PHONY: deploy-load-generator
deploy-load-generator: ## Build docker image for load-generator, publish it, and deploy it on k8s
	kubectl apply -f ./infra/load-generator.yaml -n non-istio

.PHONY: deploy-target
deploy-target: ## Build docker image for target, publish it, and deploy it on k8s
	kubectl apply -f ./infra/target.yaml -n default

.PHONY: build-deploy-load-generator
build-deploy-load-generator: docker-build-load-generator docker-publish-load-generator ## & Deploy to k8s
	kubectl delete -f ./infra/load-generator.yaml -n non-istio
	kubectl apply -f ./infra/load-generator.yaml -n non-istio

.PHONY: build-deploy-target
build-deploy-target: docker-build-target docker-publish-target ## & Deploy to k8s
	kubectl delete -f ./infra/target.yaml -n default
	kubectl apply -f ./infra/target.yaml -n default

.PHONY: demo-reset 
demo-reset: ## Reset the environment for demo 
	kubectl scale --replicas=1 deployment target 
	kubectl scale --replicas=0 deployment load-generator -n non-istio
	kubectl apply -f ../infra-values/watch-crd.yaml  
	kubectl rollout restart -n elasti deployment activator
	kubectl rollout restart -n elasti-operator-system deployment elasti-operator-controller-manager 
	
.PHONY: demo-1
demo-1: ## Set the part 1 of the demo
	kubectl scale --replicas=0 deployment target 
	kubectl scale --replicas=1 deployment load-generator -n non-istio

