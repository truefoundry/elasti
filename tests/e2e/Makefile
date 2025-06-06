# Makefile for Elasti E2E Testing Suite

# Variables
CLUSTER_NAME := elasti-e2e
NAMESPACE := default
ELASTI_NAMESPACE := elasti
PROMETHEUS_NAMESPACE := monitoring

.PHONY: all kind-up kind-down e2e-test apply-deps traffic check-deps verify-scale

all: check-deps kind-up start-registry apply-deps e2e-test

# Check for required dependencies
check-deps:
	@echo "Checking for required dependencies..."
	@command -v kind > /dev/null 2>&1 || { echo "Error: kind is required but not installed"; exit 1; }
	@command -v kubectl > /dev/null 2>&1 || { echo "Error: kubectl is required but not installed"; exit 1; }
	@command -v kubectl kuttl > /dev/null 2>&1 || { echo "Error: kuttl is required but not installed"; exit 1; }
	@echo "All dependencies are installed."

.PHONY: start-registry
start-registry: ## Setup docker registery, where we publish our images
	docker ps -a | grep -q "kind-registry" || docker run -d --restart=always -p 5002:5000 --name kind-registry registry:2
	docker start kind-registry 2>/dev/null || true

.PHONY: stop-registry
stop-registry: ## Stop docker registery
	docker stop kind-registry

.PHONY: build-images
build-images: ## Build and push images
	cd ../operator && make docker-build docker-push IMG=localhost:5002/elasti-operator:v1alpha1
	cd ../resolver && make docker-build docker-push IMG=localhost:5002/elasti-resolver:v1alpha1

# Create Kind cluster
kind-up:
	@echo "Creating Kind cluster '$(CLUSTER_NAME)'..."
	@if ! kind get clusters | grep -q $(CLUSTER_NAME); then \
		docker network connect "kind" kind-registry; \
		kind create cluster --config kind-config.yaml; \
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
apply-deps: apply-elasti apply-prometheus

apply-elasti:
	@echo "Installing Elasti Operator..."
	@kubectl create namespace $(ELASTI_NAMESPACE) --dry-run=client -o yaml | kubectl apply -f -
	@helm template elasti ../charts/elasti -n $(ELASTI_NAMESPACE) -f ./setup/elasti-chart-values.yaml | kubectl apply -f -
	@echo "Waiting for Elasti Operator to be ready..."
	@kubectl -n $(ELASTI_NAMESPACE) wait --for=condition=ready pod -l app.kubernetes.io/name=elasti --timeout=120s
	@echo "Elasti Operator installed successfully."

apply-prometheus:
	@echo "Deploying Prometheus..."
	@kubectl create namespace $(PROMETHEUS_NAMESPACE) --dry-run=client -o yaml | kubectl apply -f -
	@kubectl apply -f ./setup/deploy-prometheus.yaml
	@kubectl -n $(PROMETHEUS_NAMESPACE) wait --for=condition=available deployment/prometheus-server --timeout=120s
	@echo "Prometheus deployed successfully."

# Verify scale replicas
verify-scale:
	@if [ -z "$(REPLICAS)" ]; then \
		echo "Error: REPLICAS not provided"; \
		echo "Usage: make verify-scale REPLICAS=<expected_replicas>"; \
		exit 1; \
	fi
	@echo "Verifying deployment has $(REPLICAS) replicas..."
	@CURRENT_REPLICAS=$$(kubectl get deployment test-app -o=jsonpath='{.spec.replicas}'); \
	if [ "$$CURRENT_REPLICAS" -eq "$(REPLICAS)" ]; then \
		echo "✅ Deployment has $(REPLICAS) replicas as expected"; \
	else \
		echo "❌ Deployment has $$CURRENT_REPLICAS replicas, expected $(REPLICAS)"; \
		echo "Deployment status:"; \
		kubectl get deployment test-app -o yaml; \
		exit 1; \
	fi

# Run E2E tests
e2e-test:
	@echo "Running KUTTL tests..."
	@kubectl kuttl test --start-kind=false --namespace=$(NAMESPACE) ./tests
	@echo "E2E Tests completed!"

# Apply the traffic job
traffic:
	@echo "Deploying traffic generator job..."
	@kubectl apply -f ./setup/traffic-job.yaml
	@echo "Traffic job deployed. To monitor: kubectl logs -f job/traffic-generator"

# Clean up resources but keep the cluster
clean:
	@echo "Cleaning up test resources..."
	@kubectl delete -f ./setup/dummy-elastiservice.yaml --ignore-not-found
	@kubectl delete -f ./setup/dummy-deployment.yaml --ignore-not-found
	@kubectl delete -f ./setup/traffic-job.yaml --ignore-not-found
	@echo "Resources cleaned up."
