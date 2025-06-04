# Makefile for Elasti E2E Testing Suite

# Variables
CLUSTER_NAME := elasti-e2e
NAMESPACE := default
ELASTI_NAMESPACE := elasti-system
PROMETHEUS_NAMESPACE := monitoring

.PHONY: all kind-up kind-down e2e-test apply-deps traffic check-deps verify-scale

all: check-deps kind-up apply-deps e2e-test

# Check for required dependencies
check-deps:
	@echo "Checking for required dependencies..."
	@command -v kind > /dev/null 2>&1 || { echo "Error: kind is required but not installed"; exit 1; }
	@command -v kubectl > /dev/null 2>&1 || { echo "Error: kubectl is required but not installed"; exit 1; }
	@command -v kuttl > /dev/null 2>&1 || { echo "Error: kuttl is required but not installed"; exit 1; }
	@echo "All dependencies are installed."

# Create Kind cluster
kind-up:
	@echo "Creating Kind cluster '$(CLUSTER_NAME)'..."
	@if ! kind get clusters | grep -q $(CLUSTER_NAME); then \
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
	@kubectl -n $(ELASTI_NAMESPACE) apply -f ../charts/elasti-operator/crds/
	@cd .. && make deploy-charts
	@echo "Waiting for Elasti Operator to be ready..."
	@kubectl -n $(ELASTI_NAMESPACE) wait --for=condition=ready pod -l app=elasti-operator --timeout=120s
	@echo "Elasti Operator installed successfully."

apply-prometheus:
	@echo "Deploying Prometheus..."
	@kubectl create namespace $(PROMETHEUS_NAMESPACE) --dry-run=client -o yaml | kubectl apply -f -
	@kubectl apply -f deploy-prometheus.yaml
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
	@kuttl test --start-kind=false --namespace=$(NAMESPACE) ./tests
	@echo "E2E Tests completed!"

# Apply the traffic job
traffic:
	@echo "Deploying traffic generator job..."
	@kubectl apply -f traffic-job.yaml
	@echo "Traffic job deployed. To monitor: kubectl logs -f job/traffic-generator"

# Clean up resources but keep the cluster
clean:
	@echo "Cleaning up test resources..."
	@kubectl delete -f test-elastiservice.yaml --ignore-not-found
	@kubectl delete -f test-deployment.yaml --ignore-not-found
	@kubectl delete -f traffic-job.yaml --ignore-not-found
	@echo "Resources cleaned up."
