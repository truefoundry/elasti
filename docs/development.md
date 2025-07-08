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

> Make sure you checkout the documentation and architecture before making your changes.

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

- **`./playground`:** Code to setup a playground to try and test elasti.
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
kind create cluster
```

or

Enable it in Docker-Desktop

### 2. Start a Local Docker Registry

Run a local Docker registry container, to push our images locally and access them in our cluster.

```
docker run -d -p 5001:5000 --name registry registry:2
```

> You will need to add this registry to Minikube and Kind. With Docker-Desktop, it is automatically picked up if running in same context.

> Note: In MacOS, 5000 is not available, so we use 5001 instead.

<!-- ### 3. Install NGINX Ingress Controller:
Install the NGINX Ingress Controller using Helm:
```
helm repo add ingress-nginx https://kubernetes.github.io/ingress-nginx
helm repo update
kubectl create namespace nginx
helm install ingress-nginx ingress-nginx/ingress-nginx -n nginx
``` -->

### 3. [Optional] Install Istio Gateway to work with istio

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
kubectl apply -f ./playground/config/demo-virtualService.yaml -n demo
```

### 5. Build & Publish Resolver

Go into the resolver directory and run the build and publish command.

```bash
cd resolver
make docker-build docker-push IMG=localhost:5001/elasti-resolver:v1alpha1
```

### 6. Build & Publish Operator

Go into the operator directory and run the build and publish command.

```bash
cd operator
make docker-build docker-push IMG=localhost:5001/elasti-operator:v1alpha1
```

### 7. Deploy Locally

Make sure you have configured the local context in kubectl. We will be using [`./playground/infra/elasti-demo-values.yaml`](./playground/infra/elasti-demo-values.yaml) for the helm installation. Configure the image uri according to the requirement. Post that follow below steps from the project home directory:

```bash
kubectl create namespace elasti
helm template elasti ./charts/elasti -n elasti -f ./playground/infra/elasti-demo-values.yaml | kubectl apply -f -
```

If you want to enable monitoring, please make `enableMonitoring` true in the values file.

### 8. Create ElastiService Resource

Using the [ElastiService Defination](#1-define-a-elastiservice), create a manifest file for your service and apply it. For demo, we use the below manifest.

```bash
kubectl -n demo apply -f ./playground/config/demo-elastiService.yaml
```

### 9. Test the service

#### 9.1 Create a watch on the service

```bash
kubectl -n demo get elastiservice httpbin -w
```

#### 9.2 Scale down the service

```bash
kubectl -n demo scale deployment httpbin --replicas=0
```

#### 9.3 Create a load on the service

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

3. **Run E2E tests**

Use the KUTTL framework to execute Elasti's end-to-end tests in a real Kubernetes environment:

```bash
cd ./tests/e2e
make setup   # Sets up environment
make test    # Runs tests
```

For detailed information about the E2E test framework, see [tests/e2e/README.md](./tests/e2e/README.md).

## Monitoring

```bash
# First, add the prometheus-community Helm repository.
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm repo update


# Install the kube-prometheus-stack chart. This chart includes Prometheus and Grafana.
kubectl create namespace prometheus
helm install prometheus-stack prometheus-community/kube-prometheus-stack -n prometheus

# Port-forward to access the dashboard
kubectl port-forward -n prometheus services/prometheus-stack-grafana 3000:80

# Get the admin user.
kubectl get secret --namespace prometheus prometheus-stack-grafana -o jsonpath="{.data.admin-user}" | base64 --decode ; echo
# Get the admin password.
kubectl get secret --namespace prometheus prometheus-stack-grafana -o jsonpath="{.data.admin-password}" | base64 --decode ; echo
```

Post this, you can use [`./playground/infra/elasti-dashboard.yaml`](./playground/infra/elasti-dashboard.yaml) to import the elasti dashboard.
