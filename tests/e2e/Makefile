# Variables
CLUSTER_NAME := elasti-e2e
NAMESPACE := default
ELASTI_NAMESPACE := elasti

.PHONY: all kind-up kind-down e2e-test apply-deps traffic

setup: start-registry kind-up apply-deps

.PHONY: start-registry
start-registry: ## Setup docker registery, where we publish our images
	docker ps -a | grep -q "kind-registry" || docker run -d --restart=always -p 5002:5000 --name kind-registry registry:2
	docker start kind-registry 2>/dev/null || true

.PHONY: stop-registry
stop-registry: ## Stop docker registery
	docker stop kind-registry

.PHONY: build-images
build-images: ## Build and push images
	cd ../../operator && make docker-build docker-push IMG=localhost:5002/elasti-operator:v1alpha1
	cd ../../resolver && make docker-build docker-push IMG=localhost:5002/elasti-resolver:v1alpha1

# Create Kind cluster
kind-up:
	@echo "Creating Kind cluster '$(CLUSTER_NAME)'..."
	@if ! kind get clusters | grep -q $(CLUSTER_NAME); then \
		docker network connect "kind" kind-registry; \
		kind create cluster --config ./kind-config.yaml; \
		kubectl config use-context kind-$(CLUSTER_NAME); \
		echo "Kind cluster '$(CLUSTER_NAME)' created successfully."; \
	else \
		echo "Kind cluster '$(CLUSTER_NAME)' already exists."; \
		kubectl config use-context kind-$(CLUSTER_NAME); \
	fi

# Delete Kind cluster
kind-down:
	@echo "Deleting Kind cluster '$(CLUSTER_NAME)'..."
	@kind delete cluster --name $(CLUSTER_NAME)
	@echo "Kind cluster '$(CLUSTER_NAME)' deleted."

# Install Elasti and dependencies
apply-deps: apply-elasti

apply-elasti:
	@echo "Installing Elasti Operator..."
	@kubectl create namespace $(ELASTI_NAMESPACE) --dry-run=client -o yaml | kubectl apply -f -
	@helm template elasti ../../charts/elasti -n $(ELASTI_NAMESPACE) -f ./values/elasti-chart-values.yaml | kubectl apply -f -
	@echo "Waiting for Elasti Operator to be ready..."
	@kubectl -n $(ELASTI_NAMESPACE) wait --for=condition=ready pod -l app.kubernetes.io/name=elasti --timeout=120s
	@echo "Elasti Operator installed successfully."

# Run E2E tests
e2e-test:
	@echo "Running KUTTL tests..."
	@kubectl kuttl test --start-kind=false --namespace=$(NAMESPACE) ./tests
	@echo "E2E Tests completed!"

