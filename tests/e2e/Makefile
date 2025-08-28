# Variables
CLUSTER_NAME := elasti-e2e
NAMESPACE := test
ELASTI_NAMESPACE := elasti
T ?= ./tests
REGISTRY_HOST := localhost:5002
REGISTRY_NAME := kind-registry
ISTIO_VERSION := 1.25.0
PROMETHEUS_VERSION := 73.2.3
PROMETHEUS_PUSHGATEWAY_VERSION := 3.4.1
KEDA_VERSION := 2.17.1



.PHONY: all setup destroy reset-kind reset-setup
all: setup test
setup: start-registry build-images kind-up chmod-reset apply-deps
destroy: kind-down stop-registry 
# in reset-kind, we destroy the cluster.
reset-kind: kind-down kind-up apply-deps
# In reset-setup, we destroy the whole setup, and setup again.
reset-setup: destroy setup

.PHONY: chmod-reset
chmod-reset:
	@echo "Creating reset executable..."
	@chmod +x ./manifest/reset.sh

.PHONY: start-registry
start-registry: ## Setup docker registry, where we publish our images
	@echo "Setting up registry..."
	@if docker ps -a | grep -q "$(REGISTRY_NAME)"; then \
		echo "Registry already exists, ensuring it's running..."; \
		docker start $(REGISTRY_NAME) 2>/dev/null || true; \
	else \
		echo "Creating new registry container..."; \
		docker run -d --restart=always -p 5002:5000 --name $(REGISTRY_NAME) registry:2; \
	fi
	@docker network inspect kind >/dev/null 2>&1 || docker network create kind
	@docker network inspect kind -f "{{range .Containers}}{{.Name}}{{end}}" | grep -q $(REGISTRY_NAME) || \
		docker network connect "kind" $(REGISTRY_NAME)
	@echo "Registry setup complete"

.PHONY: stop-registry
stop-registry: ## Stop docker registry
	@echo "Stopping registry..."
	@if docker ps -a | grep -q "$(REGISTRY_NAME)"; then \
		echo "Registry container found, stopping it..."; \
		docker stop $(REGISTRY_NAME) 2>/dev/null || true; \
	else \
		echo "Registry container not found, skipping stop."; \
	fi
	@docker network inspect kind >/dev/null 2>&1 && docker network disconnect kind $(REGISTRY_NAME)
	@docker network rm kind
	@echo "Registry stopped and network cleaned up"

.PHONY: build-images
build-images: ## Build and push images
	$(MAKE) -C ../../operator docker-build docker-push IMG=$(REGISTRY_HOST)/elasti-operator:v1alpha1
	$(MAKE) -C ../../resolver docker-build docker-push IMG=$(REGISTRY_HOST)/elasti-resolver:v1alpha1

.PHONY: kind-up
kind-up: ## Create Kind cluster
	@echo "Creating Kind cluster '$(CLUSTER_NAME)'..."
	@if ! kind get clusters | grep -q $(CLUSTER_NAME); then \
		kind create cluster --config ./kind-config.yaml; \
		kubectl config use-context kind-$(CLUSTER_NAME); \
		echo "Kind cluster '$(CLUSTER_NAME)' created successfully."; \
	else \
		echo "Kind cluster '$(CLUSTER_NAME)' already exists."; \
		kubectl config use-context kind-$(CLUSTER_NAME); \
	fi

.PHONY: kind-down
kind-down:
	@echo "Deleting kind cluster '$(CLUSTER_NAME)'..."
	@if kind get clusters | grep -q $(CLUSTER_NAME); then \
		kind delete cluster --name $(CLUSTER_NAME); \
		echo "Kind cluster '$(CLUSTER_NAME)' deleted successfully."; \
	else \
		echo "Kind cluster '$(CLUSTER_NAME)' does not exist."; \
	fi

.PHONY: apply-deps
apply-deps: apply-ingress apply-prometheus apply-elasti apply-keda

.PHONY: apply-elasti
apply-elasti:
	@echo "Installing Elasti Operator..."
	@helm upgrade --install --create-namespace elasti ../../charts/elasti -n $(ELASTI_NAMESPACE) -f ./manifest/values-elasti.yaml

.PHONY: apply-prometheus
apply-prometheus:
	@echo "Installing Prometheus Stack..."
	@kubectl create namespace monitoring --dry-run=client -o yaml | kubectl apply -f -
	@helm repo add prometheus-community https://prometheus-community.github.io/helm-charts --force-update
	@helm repo update
	@helm upgrade --install kube-prometheus-stack prometheus-community/kube-prometheus-stack --version $(PROMETHEUS_VERSION) \
	  --namespace monitoring \
	  --create-namespace \
	  --set alertmanager.enabled=false \
	  --set grafana.enabled=true \
	  --set kubeStateMetrics.enabled=false \
	  --set nodeExporter.enabled=false \
	  --set prometheus.prometheusSpec.serviceMonitorSelectorNilUsesHelmValues=false --wait --timeout 180s
	@kubectl apply -f ./manifest/istio-servicemonitor.yaml
	@helm upgrade --install prometheus-pushgateway prometheus-community/prometheus-pushgateway --version $(PROMETHEUS_PUSHGATEWAY_VERSION) \
	  --namespace monitoring \
	  --set serviceMonitor.enabled=true \
	  --set serviceMonitor.interval=5s \
	  --wait --timeout 180s
	@echo "Prometheus Stack installed successfully."

.PHONY: apply-ingress
apply-ingress:
	@echo "Setting up Istio..."
	@helm repo add istio https://istio-release.storage.googleapis.com/charts
	@helm repo update
	@helm upgrade --install istio-base istio/base -n istio-system --create-namespace --version $(ISTIO_VERSION) --wait --timeout 180s
	@helm upgrade --install istiod istio/istiod -n istio-system --version $(ISTIO_VERSION) --wait --timeout 180s
	@helm upgrade --install istio-ingressgateway istio/gateway -n istio-system --version $(ISTIO_VERSION) 
	@kubectl -n istio-system wait --for=condition=available deployment/istio-ingressgateway --timeout=180s
	@kubectl create namespace target --dry-run=client -o yaml | kubectl apply -f -
	@kubectl label namespace target istio-injection=enabled --overwrite
	@echo "Istio installed successfully."

.PHONY: uninstall-ingress
uninstall-ingress:
	@echo "Uninstalling Istio..."
	@helm uninstall istio-base istiod istio-ingressgateway -n istio-system
	@kubectl delete namespace istio-system
	@echo "Istio uninstalled successfully."

.PHONY: apply-keda
apply-keda: 
	@echo "Installing KEDA..."
	@kubectl create namespace keda --dry-run=client -o yaml | kubectl apply -f -
	@helm repo add kedacore https://kedacore.github.io/charts
	@helm repo update
	@helm upgrade --install keda kedacore/keda --namespace keda --create-namespace --version $(KEDA_VERSION) --wait --timeout 180s
	@echo "KEDA installed successfully."

# This is for locally testing, not used in CI
.PHONY: uninstall-keda
uninstall-keda:
	@echo "Uninstalling KEDA..."
	@helm uninstall keda -n keda
	@kubectl delete namespace keda
	@echo "KEDA uninstalled successfully."

.PHONY: test
test:
	@echo "Running KUTTL tests..."
	@kubectl kuttl test --start-kind=false --namespace=$(NAMESPACE) --test $(T)
	@echo "E2E Tests completed!"

.PHONY: pf-prom
pf-prom:
	@echo "Portforwarding prometheus on port 9090..."
	@kubectl --namespace monitoring port-forward service/prometheus-operated 9090:9090

.PHONY: pf-graf
pf-graf: 
	@echo "Portforwarding grafana on port 9001..."
	@kubectl --namespace monitoring port-forward service/kube-prometheus-stack-grafana 9001:80

.PHONY: pf-ingress
pf-ingress: 
	@echo "Portforwarding ingress gateway on port 8080..."
	@kubectl --namespace istio-system port-forward service/istio-ingressgateway 8080:80

.PHONY: lint-kube
lint-kube:
	@echo "Linting k8s manifests and kuttl tests..."
	@kube-linter lint ./manifest
	@kube-linter lint ./tests

.PHONY: lint-yaml
lint-yaml:
	@echo "Linting k8s manifests and kuttl tests..."
	@yamllint ./manifest
	@yamllint ./tests
