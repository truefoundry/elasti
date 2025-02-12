<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
**Table of Contents**  *generated with [DocToc](https://github.com/thlorenz/doctoc)*

- [Development Guide](#development-guide)
  - [Dev Environment](#dev-environment)
    - [1. Get required tools](#1-get-required-tools)
    - [2. Clone the Repository](#2-clone-the-repository)
    - [3. Repository Structure](#3-repository-structure)
  - [Setup Playground](#setup-playground)
    - [1. Local Cluster](#1-local-cluster)
    - [2. Start a Local Docker Registry](#2-start-a-local-docker-registry)
    - [3. Install Istio Gateway to work with istio](#3-install-istio-gateway-to-work-with-istio)
    - [4. Deploy a demo service](#4-deploy-a-demo-service)
    - [5. Build & Publish Resolver](#5-build--publish-resolver)
    - [6. Build & Publish Operator](#6-build--publish-operator)
    - [7. Deploy Locally](#7-deploy-locally)
    - [8. Setup a Trigger for Elasti](#8-setup-a-trigger-for-elasti)
    - [9. Create ElastiService Resource](#9-create-elastiservice-resource)
    - [10. Test the service](#10-test-the-service)
      - [10.1 Create a watch on the service](#101-create-a-watch-on-the-service)
      - [10.2 Scale down the service](#102-scale-down-the-service)
      - [10.3 Create a load on the service](#103-create-a-load-on-the-service)
  - [Testing](#testing)
  - [Monitoring](#monitoring)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

# Development Guide

Setting up your development environment for Elasti involves preparing your local setup for building, testing, and contributing to the project. Follow these steps to get started:

## Dev Environment

### 1. Get required tools

Ensure you have the following tools installed:

- **Go:** The programming language used for Elasti. Download and install it from [golang.org](https://golang.org/dl/).
- **Docker:** For containerization and building Docker images. Install it from [docker.com](https://www.docker.com/get-started).
- **kubectl:**: Command-line tool for interacting with Kubernetes. Install it from [kubernetes.io](https://kubernetes.io/docs/tasks/tools/).
- **Helm:** Package manager for Kubernetes. Install it from [helm.sh](https://helm.sh/docs/intro/install/).
- **Docker Desktop/Kind/Minikube:** A local kubernetes cluster. Make sure you have the local cluster running before development.
- **Make:** Helps in working with the project.
- **Istio:** Required to test the project with istio. Install from [istio.io](https://istio.io/)
- **k6:** Required to load test the project. Install from [k6.io](https://k6.io/)

### 2. Clone the Repository

Clone the Elasti repository from GitHub to your local machine:

```
git clone https://github.com/truefoundry/elasti.git
cd elasti
```

> Make sure you check out the documentation and architecture before making your changes.

### 3. Repository Structure

Understanding the repository structure will help you navigate and contribute effectively to the Elasti project. Below is an overview of the key directories and files in the repository:

```
.
├── LICENSE
├── Makefile
├── README.md
├── charts
├── docs
├── go.work
├── go.work.sum
├── kustomization.yaml
├── operator
├── pkg
├── playground
├── resolver
└── test
```

2 Main Modules:

- **`./operator`:** Contains the code for Kubernetes operator, created using kubebuilder.
  ```
  .
  ├── Dockerfile
  ├── Makefile
  ├── api
  ├── cmd
  ├── config
  ├── go.mod
  ├── go.sum
  ├── internal
  └── test
  ```
  - **`./api`:** Contains the folder named after the apiVersion, and has custom resource type description.
  - **`./config`:** Kubernetes manifest files.
  - **`./cmd`:** Main files for the tool.
  - **`./internal`:** Internal packages of the program.
  - **`./Makefile`:** Helps with working with the program. Use `make help` to see all the available commands.
- **`./resolver`:** Contains the code for resolver.
  - File structure of it is similar to that of Operator.

Other Directories:

- **`./playground`:** Code to set up a playground to try and test elasti.
- **`./test`:** Load testing scripts.
- **`./pkg`:** Common packages, shared via Operator and Resolve.
- **`./charts`:** Helm chart template.
- **`./docs`:** Detailed documentation on the HLD, LLD and Architecture of elasti.

## Setup Playground

### 1. Local Cluster

If you don't already have a local Kubernetes cluster, you can set one up using Minikube, Kind or Docker-Desktop:

```
minikube start
```

or

```
kind create cluster --config=playground/config/kind-cluster-config.yaml
```

or

Enable it in Docker-Desktop

### 2. Start a Local Docker Registry

Run a local Docker registry container, to push our images locally and access them in our cluster.

```
docker run -d -p 5000:5000 --name registry registry:2
```

> You will need to add this registry to Minikube and Kind, with Docker-Desktop, it is automatically picked up if running in same context.

<!-- ### 3. Install NGINX Ingress Controller:
Install the NGINX Ingress Controller using Helm:
```
helm repo add ingress-nginx https://kubernetes.github.io/ingress-nginx
helm repo update
kubectl create namespace nginx
helm install ingress-nginx ingress-nginx/ingress-nginx -n nginx
``` -->

### 3. Install Istio Gateway to work with istio

```bash
# Download the latest Istio release from the official Istio website.
curl -L https://istio.io/downloadIstio | sh -
# Move it to home directory
mv istio-x.xx.x ~/.istioctl
export PATH=$HOME/.istioctl/bin:$PATH

istioctl install --set profile=default -y

# Label the namespace where you want to deploy your application to enable Istio sidecar Injection
kubectl create namespace demo
kubectl label namespace demo istio-injection=enabled

# Create a gateway
kubectl apply -f ./playground/config/gateway.yaml
```

### 4. Deploy a demo service

Run a demo application in your cluster.

```bash
kubectl create namespace demo
kubectl apply -f ./playground/config/demo-application.yaml -n demo

# Create a Virtual Service to expose the demo service if you are using istio
kubectl apply -f ./playground/config/demo-virtualService.yaml
```

### 5. Build & Publish Resolver

Go into the resolver directory and run the build and publish command.

```bash
cd resolver
make docker-build docker-push IMG=localhost:5000/elasti-resolver:v1alpha1
```

If using a kind cluster
```bash
docker network connect "kind" registry
```

### 6. Build & Publish Operator

Go into the operator directory and run the build and publish command.

```bash
cd operator
make docker-build docker-push IMG=localhost:5000/elasti-operator:v1alpha1
```

### 7. Deploy Locally

Make sure you have configured the local context in kubectl. We will be using [`./playground/infra/elasti-demo-values.yaml`](./playground/infra/elasti-demo-values.yaml) for the helm installation. Configure the image uri according to the requirement. Post that follow below steps from the project home directory:

```bash
kubectl create namespace elasti
helm template elasti ./charts/elasti -n elasti -f ./playground/infra/elasti-demo-values.yaml | kubectl apply -f -
```

If you want to enable monitoring, please make `enableMonitoring` true in the values file.

### 8. Setup a Trigger for Elasti

We will use Prometheus as the trigger for Elasti.

```bash
# First, add the prometheus-community Helm repository.
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm repo update


# Install the kube-prometheus-stack chart. This chart includes Prometheus and Grafana.
kubectl create namespace prometheus
helm install prometheus-stack prometheus-community/kube-prometheus-stack -n prometheus
```

### 9. Create ElastiService Resource

Using the [ElastiService Definition](README.md#1-define-an-elastiservice), create a manifest file for your service and apply it. For demo, we use the below manifest.

```bash
kubectl -n demo apply -f ./playground/config/demo-elastiService.yaml
```

### 10. Test the service

#### 10.1 Create a watch on the service

```bash
kubectl -n demo get elastiservice httpbin -w
```

#### 10.2 Scale down the service

```bash
kubectl -n demo scale deployment httpbin --replicas=0
```

#### 10.3 Create a load on the service

```bash
kubectl run -it --rm curl --image=alpine/curl -- http://httpbin.demo.svc.cluster.local/headers
```

You should see the target service pod getting scaled up and response from the new pod.

## Testing

This section outlines how to run integration tests, and performance tests using k6.

1. **Update k6 tests**

   Update the `./test/load.js` file, to add your url for testing, and update other configurations in the same file.

2. **Run load.js**

   Run the following command to run the test.

   ```
   chmod +x ./test/generate_load.sh
   cd ./test
   ./generate_load.sh
   ```

## Monitoring

Install prometheus if not already installed.
```bash
# First, add the prometheus-community Helm repository.
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm repo update


# Install the kube-prometheus-stack chart. This chart includes Prometheus and Grafana.
kubectl create namespace prometheus
helm install prometheus-stack prometheus-community/kube-prometheus-stack -n prometheus
```
Set up monitoring.
```bash
# Port-forward to access the dashboard
kubectl port-forward -n prometheus services/prometheus-stack-grafana 3000:80

# Get the admin user.
kubectl get secret --namespace prometheus prometheus-stack-grafana -o jsonpath="{.data.admin-user}" | base64 --decode ; echo
# Get the admin password.
kubectl get secret --namespace prometheus prometheus-stack-grafana -o jsonpath="{.data.admin-password}" | base64 --decode ; echo
```

Post this, you can use [`./playground/infra/elasti-dashboard.yaml`](./playground/infra/elasti-dashboard.yaml) to import the elasti dashboard.