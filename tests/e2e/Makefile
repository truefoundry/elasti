# Variables
CLUSTER_NAME := elasti-e2e
NAMESPACE := test
ELASTI_NAMESPACE := elasti
T ?= ./tests

.PHONY: all setup destroy reset-kind reset-setup
all: setup test
setup: start-registry build-images kind-up apply-deps
destroy: kind-down stop-registry 
# in reset-kind, we destroy the cluster.
reset-kind: kind-down kind-up apply-deps
# In reset-setup, we destroy the whole setup, and setup again.
reset-setup: destroy setup

.PHONY: start-registry
start-registry: ## Setup docker registry, where we publish our images
	@echo "Setting up registry..."
	@if docker ps -a | grep -q "kind-registry"; then \
		echo "Registry already exists, ensuring it's running..."; \
		docker start kind-registry 2>/dev/null || true; \
	else \
		echo "Creating new registry container..."; \
		docker run -d --restart=always -p 5002:5000 --name kind-registry registry:2; \
	fi
	@docker network inspect kind >/dev/null 2>&1 || docker network create kind
	@docker network inspect kind -f "{{range .Containers}}{{.Name}}{{end}}" | grep -q kind-registry || \
		docker network connect "kind" kind-registry
	@echo "Registry setup complete"

.PHONY: stop-registry
stop-registry: ## Stop docker registry
	@echo "Stopping registry..."
	@if docker ps -a | grep -q "kind-registry"; then \
		echo "Registry container found, stopping it..."; \
		docker stop kind-registry 2>/dev/null || true; \
	else \
		echo "Registry container not found, skipping stop."; \
	fi
	@docker network inspect kind >/dev/null 2>&1 && docker network disconnect kind kind-registry
	@docker network rm kind
	@echo "Registry stopped and network cleaned up"

.PHONY: build-images
build-images: ## Build and push images
	cd ../../operator && make docker-build docker-push IMG=localhost:5002/elasti-operator:v1alpha1
	cd ../../resolver && make docker-build docker-push IMG=localhost:5002/elasti-resolver:v1alpha1

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
apply-deps: apply-ingress apply-prometheus apply-elasti 

.PHONY: apply-elasti
apply-elasti:
	@echo "Installing Elasti Operator..."
	@kubectl create namespace $(ELASTI_NAMESPACE) --dry-run=client -o yaml | kubectl apply -f -
	@if ! kubectl get deployment -n $(ELASTI_NAMESPACE) | grep -q "elasti"; then \
		helm template elasti ../../charts/elasti -n $(ELASTI_NAMESPACE) -f ./manifest/values-elasti.yaml | kubectl apply -f -; \
		echo "Waiting for Elasti Operator to be ready..."; \
		kubectl -n $(ELASTI_NAMESPACE) wait --for=condition=ready pod -l app.kubernetes.io/name=elasti --timeout=180s; \
		echo "Elasti Operator installed successfully."; \
	else \
		echo "Elasti Operator already exists, skipping installation."; \
	fi

.PHONY: apply-prometheus
apply-prometheus:
	@echo "Installing Prometheus Stack..."
	@kubectl create namespace monitoring --dry-run=client -o yaml | kubectl apply -f -
	@helm repo add prometheus-community https://prometheus-community.github.io/helm-charts --force-update
	@helm repo update
	@helm upgrade --install kube-prometheus-stack prometheus-community/kube-prometheus-stack --version 73.2.3 \
	  --namespace monitoring \
	  --create-namespace \
	  --set alertmanager.enabled=false \
	  --set grafana.enabled=true \
	  --set kubeStateMetrics.enabled=false \
	  --set prometheus.prometheusSpec.serviceMonitorSelectorNilUsesHelmValues=true
	@kubectl apply -f ./manifest/istio-servicemonitor.yaml
	@kubectl -n monitoring wait --for=condition=available deployment/kube-prometheus-stack-operator --timeout=120s
	@echo "Prometheus Stack installed successfully."

.PHONY: apply-ingress
apply-ingress:
	@echo "Setting up Istio..."
	@helm repo add istio https://istio-release.storage.googleapis.com/charts
	@helm repo update
	@helm upgrade --install istio-base istio/base -n istio-system --create-namespace
	@helm upgrade --install istiod istio/istiod -n istio-system
	@helm upgrade --install istio-ingressgateway istio/gateway -n istio-system -f ./manifest/values-istio-ingress.yaml
	@kubectl create namespace target --dry-run=client -o yaml | kubectl apply -f -
	@kubectl label namespace target istio-injection=enabled --overwrite
	@echo "Waiting for Istio to be ready..."
	@kubectl -n istio-system wait --for=condition=available deployment/istiod --timeout=180s
	@kubectl -n istio-system wait --for=condition=available deployment/istio-ingressgateway --timeout=180s
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
	@helm upgrade --install keda kedacore/keda --namespace keda --create-namespace --version 2.17.1
	@echo "Waiting for KEDA to be ready..."
	@kubectl -n keda wait --for=condition=ready pod -l app=keda-operator --timeout=180s
	@kubectl -n keda wait --for=condition=ready pod -l app=keda-admission-webhooks --timeout=180s
	@kubectl -n keda wait --for=condition=ready pod -l app=keda-operator-metrics-apiserver --timeout=180s
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



