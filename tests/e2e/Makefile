# Variables
CLUSTER_NAME := elasti-e2e
NAMESPACE := default
ELASTI_NAMESPACE := elasti

.PHONY: all kind-up kind-down e2e-test apply-deps traffic

setup: start-registry kind-up apply-deps
reset-setup: kind-down kind-up apply-deps
all: start-registry build-images kind-up apply-deps e2e-test

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
		docker network create kind || true; \
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
apply-deps:apply-ingress  apply-prometheus apply-keda apply-elasti 

apply-elasti:
	@echo "Installing Elasti Operator..."
	@kubectl create namespace $(ELASTI_NAMESPACE) --dry-run=client -o yaml | kubectl apply -f -
	@helm template elasti ../../charts/elasti -n $(ELASTI_NAMESPACE) -f ./manifest/elasti-chart-values.yaml | kubectl apply -f -
	@echo "Waiting for Elasti Operator to be ready..."
	@kubectl -n $(ELASTI_NAMESPACE) wait --for=condition=ready pod -l app.kubernetes.io/name=elasti --timeout=180s
	@echo "Elasti Operator installed successfully."

# Setup Prometheus for metrics collection
apply-prometheus:
	@echo "Installing Prometheus Stack..."
	@kubectl create namespace monitoring --dry-run=client -o yaml | kubectl apply -f -
	@helm repo add prometheus-community https://prometheus-community.github.io/helm-charts --force-update
	@helm repo update
	@helm install kube-prometheus-stack prometheus-community/kube-prometheus-stack \
	  --namespace monitoring \
	  --create-namespace \
	  --set alertmanager.enabled=false \
	  --set grafana.enabled=false \
	  --set prometheus.prometheusSpec.serviceMonitorSelectorNilUsesHelmValues=false
	@kubectl -n monitoring wait --for=condition=available deployment/kube-prometheus-stack-operator --timeout=120s
	@echo "Prometheus Stack installed successfully."

# Setup Istio Service Mesh
apply-ingress:
	@echo "Setting up Istio..."
	@if ! command -v istioctl &> /dev/null; then \
		echo "istioctl not found, installing..."; \
		curl -L https://istio.io/downloadIstio | ISTIO_VERSION=1.20.0 sh -; \
		mv istio-1.20.0/bin/istioctl /usr/local/bin/ || sudo mv istio-1.20.0/bin/istioctl /usr/local/bin/; \
		echo "istioctl installed successfully."; \
	else \
		echo "istioctl already installed, skipping download."; \
	fi
	@echo "Installing Istio components..."
	@istioctl install --set profile=demo -y
	@kubectl label namespace default istio-injection=enabled --overwrite
	@echo "Waiting for Istio to be ready..."
	@kubectl -n istio-system wait --for=condition=available deployment/istiod --timeout=180s
	@kubectl -n istio-system wait --for=condition=available deployment/istio-ingressgateway --timeout=180s
	@echo "Istio installed successfully."

apply-keda: 
	@echo "Installing KEDA..."
	@kubectl create namespace keda --dry-run=client -o yaml | kubectl apply -f -
	@helm repo add kedacore https://kedacore.github.io/charts
	@helm repo update
	@helm install keda kedacore/keda --namespace keda --create-namespace
	@echo "Waiting for KEDA to be ready..."
	@kubectl -n keda wait --for=condition=ready pod -l app=keda-operator --timeout=180s
	@kubectl -n keda wait --for=condition=ready pod -l app=keda-admission-webhooks --timeout=180s
	@kubectl -n keda wait --for=condition=ready pod -l app=keda-operator-metrics-apiserver --timeout=180s
	@echo "KEDA installed successfully."

# Run E2E tests
e2e-test:
	@echo "Running KUTTL tests..."
	@kubectl kuttl test --start-kind=false --namespace=$(NAMESPACE) ./tests
	@echo "E2E Tests completed!"

# Portforward prometheus
pf-prom:
	@echo "Portforwarding prometheus on port 9090..."
	@kubectl --namespace monitoring port-forward service/prometheus-operated 9090:9090

pf-target: 
	@echo "Portforwarding target deployment on port 5050..."
	@kubectl --namespace default port-forward service/target-deployment 5050:80

pf-ingress: 
	@echo "Portforwarding ingress gateway on port 8080..."
	@kubectl --namespace istio-system port-forward service/istio-ingressgateway 8080:80



