<p align="center">
<img src="./docs/logo/banner.png" alt="elasti icon">
</p>

<p align="center">
 <a>
    <img src="https://goreportcard.com/badge/github.com/truefoundry/elasti" align="center">
 </a>
 <a>
    <img src="https://img.shields.io/badge/license-MIT-blue" align="center">
 </a>

</p>

> This project is in Alpha right now.

# Why use Elasti?

Kubernetes clusters can become costly, especially when running multiple services continuously. Elasti addresses this issue by giving you the confidence to scale down services during periods of low or no traffic, as it can bring them back up when demand increases. This optimization minimizes resource usage without compromising on service availability. Additionally, Elasti ensures reliability by acting as a proxy that queues incoming requests for scaled-down services. Once these services are reactivated, Elasti processes the queued requests, so that no request is lost. This combination of cost savings and dependable performance makes Elasti an invaluable tool for efficient Kubernetes service management.

> The name Elasti comes from a superhero "Elasti-Girl" from DC Comics. Her supower is to expand or shrink her body at will—from hundreds of feet tall to mere inches in height.

<div align="center"> <b> Demo </b></div>
<div align="center">
    <a href="https://www.loom.com/share/6dae33a27a5847f081f7381f8d9510e6">
      <img style="max-width:640px;" src="https://cdn.loom.com/sessions/thumbnails/6dae33a27a5847f081f7381f8d9510e6-adf9e85a899f85fd-full-play.gif">
    </a>
  </div>

# Contents

- [Why use Elasti?](#why-use-elasti)
- [Contents](#contents)
- [Introduction](#introduction)
  - [Key Features](#key-features)
- [Getting Started](#getting-started)
  - [Prerequisites](#prerequisites)
  - [Install](#install)
    - [1. Add the Elasti Helm Repository](#1-add-the-elasti-helm-repository)
    - [2. Install Elasti](#2-install-elasti)
    - [3. Verify the Installation](#3-verify-the-installation)
  - [Configuration](#configuration)
    - [1. Define a ElastiService](#1-define-a-elastiservice)
    - [2. Apply the configuration](#2-apply-the-configuration)
    - [3. Check Logs](#3-check-logs)
  - [Monitoring](#monitoring)
  - [Uninstall](#uninstall)
- [Development](#development)
  - [Dev Environment](#dev-environment)
    - [1. Get required tools](#1-get-required-tools)
    - [2. Clone the Repository](#2-clone-the-repository)
    - [3. Repository Structure](#3-repository-structure)
  - [Setup Playground](#setup-playground)
    - [1. Local Cluster](#1-local-cluster)
    - [2. Start a Local Docker Registry](#2-start-a-local-docker-registry)
    - [3. Install Istio Gateway.](#3-install-istio-gateway)
    - [4. Deploy a demo service](#4-deploy-a-demo-service)
  - [Build](#build)
    - [1. Build \& Publish Resolver](#1-build--publish-resolver)
    - [2. Build \& Publish Operator](#2-build--publish-operator)
  - [Deploy Locally](#deploy-locally)
    - [3. Create ElastiService Resource](#3-create-elastiservice-resource)
  - [Testing](#testing)
  - [Monitoring](#monitoring-1)
- [Contribution](#contribution)
  - [Getting Started](#getting-started-1)
  - [Getting Help](#getting-help)
  - [Acknowledgements](#acknowledgements)
- [Future Developments](#future-developments)

# Introduction

Elasti monitors the target service for which you want to enable scale-to-zero. When the target service is scaled down to zero, Elasti automatically switches to Proxy mode, redirecting all incoming traffic to itself. In this mode, Elasti queues the incoming requests and scales up the target service. Once the service is back online, Elasti processes the queued requests, sending them to the now-active service. After the target service is scaled up, Elasti switches to Serve mode, where traffic is directly handled by the service, removing any redirection. This seamless transition between modes ensures efficient handling of requests while optimizing resource usage.

<div align="center">
<img src="./docs/assets/modes.png" width="400px">
</div>

## Key Features

- **Seamless Integration:** Elasti integrates effortlessly with your existing Kubernetes setup. It takes just a few steps to enable scale to zero for any service.

- **Development and Argo Rollouts Support:** Elasti supports two target references: Deployment and Argo Rollouts, making it versatile for various deployment scenarios.

- **HTTP API Support:** Currently, Elasti supports only HTTP API types, ensuring straightforward and efficient handling of web traffic.

- **Prometheus Metrics Export:** Elasti exports Prometheus metrics for easy out-of-the-box monitoring. You can also import a pre-built dashboard into Grafana for comprehensive visualization.

- **Istio Support:** Elasti is compatible with Istio. It also supports East-West traffic using cluster-local service DNS, ensuring robust and flexible traffic management across your services.

# Getting Started

With Elasti, you can easily manage and scale your Kubernetes services by using a proxy mechanism that queues and holds requests for scaled-down services, bringing them up only when needed. Get started by follwing below steps:

## Prerequisites

- **Kubernetes Cluster:** You should have a running Kubernetes cluster. You can use any cloud-based or on-premises Kubernetes distribution.
- **kubectl:** Installed and configured to interact with your Kubernetes cluster.
- **Helm:** Installed for managing Kubernetes applications.

## Install

### 1. Add the Elasti Helm Repository

Add the official elasti Helm chart repository to your Helm configuration:

```bash
helm repo add elasti https://charts.truefoundry.com/elasti
helm repo update
```

### 2. Install Elasti

Use Helm to install elasti into your Kubernetes cluster. Replace `<release-name>` with your desired release name and `<namespace>` with the Kubernetes namespace you want to use:

```bash
helm install <release-name> elasti/elasti --namespace <namespace>
```

Check out [docs](./docs/README.md#5-helm-values) to see config in the helm value file.

### 3. Verify the Installation

Check the status of your Helm release and ensure that the elasti components are running:

```bash
helm status <release-name> --namespace <namespace>
kubectl get pods -n <namespace>
```

You will see 2 components running.

1.  **Controller/Operator:** `elasti-operator-controller-manager-...` is to switch the traffic, watch resources, scale etc.
2.  **Resolver:** `elasti-resolver-...` is to proxy the requests.

Refer to the [Docs](./docs/README.md) to know how it works.

## Configuration

To configure a service to handle its traffic via elasti, you'll need to create and apply a `ElastiService` custom resource:

### 1. Define a ElastiService

```
apiVersion: elasti.truefoundry.com/v1alpha1
kind: ElastiService
metadata:
   name: <service-name>
   namespace: <service-namespace>
spec:
   minTargetReplicas: <min-target-replicas>
   service: <service-name>
   scaleTargetRef:
      apiVersion: <apiVersion>
      kind: <kind>
      name: <deployment-or-rollout-name>
```

- `<service-name>`: Replace it with the service you want managed by elasti.
- `<min-target-replicas>`: Min replicas to bring up when first request arrives.
- `<service-namespace>`: Replace by namespace of the service.
- `<kind>`: Replace by `rollouts` or `deployments`
- `<apiVersion>`: Replace by `argoproj.io/v1alpha1` or `apps/v1`
- `<deployment-or-rollout-name>`: Replace by name of the rollout or the deployment for the service. This will be scaled up to min-target-replicas when first request comes

### 2. Apply the configuration

Apply the configuration to your Kubernetes cluster:

```
kubectl apply -f <service-name>-elasti-CRD.yaml
```

### 3. Check Logs

You can view logs from the controller to watchout for any errors.

```
kubectl logs -f deployment/elasti-operator-controller-manager -n <namespace>
```

## Monitoring

During installation, two ServiceMonitor custom resources are created to enable Prometheus to discover the Elasti components. To verify this, you can open your Prometheus interface and search for metrics prefixed with elasti-, or navigate to the Targets section to check if Elasti is listed.

Once verification is complete, you can use the [provided Grafana dashboard](./playground/infra/elasti-dashboard.yaml) to monitor the internal metrics and performance of Elasti.

<div align="center">
<img src="./docs/assets/grafana-dashboard.png" width="800px">
</div>

## Uninstall

To uninstall Elasti, **you will need to remove all the installed ElastiServices first.** Then, simply delete the installation file.

```bash
kubectl delete elastiservices --all
helm uninstall elasti -n <namespace>
kubectl delete namespace <namespace>
```

# Development

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

### 3. Install Istio Gateway.

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

> For Linux and macOS

### 4. Deploy a demo service

Run a demo application in your cluster.

```bash
kubectl apply -f ./playground/config/demo-application.yaml -n demo

# Create a Virtual Service to expose the demo service
kubectl apply -f ./playground/config/demo-virtualService.yaml
```

## Build

### 1. Build & Publish Resolver

We will build and publish our resolver changes.

1. Go into the resolver directory.
2. Run the build and publish command.

```bash
make docker-build docker-push IMG=localhost:5000/elasti-resolver:v1alpha1

or

# Build the docker image
docker build -t localhost:5000/elasti-resolver:v1alpha1
# Push the image to local registry
docker push localhost:5000/elasti-resolver:v1alpha1
```

### 2. Build & Publish Operator

We will build and publish our Operator changes.

1. Go into the operator directory.
2. Run the build and publish command.

```bash
make docker-build IMG=localhost:5000/elasti-operator:v1alpha1

or
# Build the docker image
docker build -t localhost:5000/elasti-operator:v1alpha1
# Push the image to local registry
docker push localhost:5000/elasti-operator:v1alpha1
```

## Deploy Locally

Make sure you have configured the local context in kubectl. We will be using `./playground/infra/elast-demo-values.yaml` for the helm installation. Configure the image uri according to the requirement. Post that follow below steps from the project home directory:

```bash

helm template elasti ./charts/elasti -n elasti -f ./playground/infra/elasti-demo-values.yaml | kubectl apply -f -
```

If you want to enable monitoring, please make `enableMonitoring` true in the values file.

### 3. Create ElastiService Resource

Using the [ElastiService Defination](#1-define-a-elastiservice), create a manifest file for your service and apply it. For demo, we use the below manifest.

```bash
kubectl -n demo apply -f ./playground/config/demo-elastiService.yaml
```

## Testing

Testing is crucial to ensure the reliability and performance of Elasti. This section outlines how to run integration tests, and performance tests using k6.

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

Post this, you can use [`./playground/infra/elasti-dashboard.yaml`](./playground//infra/elasti-dashboard.yaml) to import the elasti dashboard.

# Contribution

We welcome contributions from the community to improve Elasti. Whether you're fixing bugs, adding new features, improving documentation, or providing feedback, your efforts are appreciated. Follow the steps below to contribute effectively to the project.

## Getting Started

Follows the steps mentioned in [development](#development) section. Post that follow:

1. **Fork the Repository:**
   Fork the Elasti repository to your own GitHub account:

   ```bash
   git clone https://github.com/your-org/elasti.git
   cd elasti
   ```

2. **Create a New Branch:**
   Create a new branch for your changes:

   ```bash
   git checkout -b feature/your-feature-name
   ```

3. **Code Changes:**
   Make your code changes or additions in the appropriate files and directories. Ensure you follow the project's coding standards and best practices.

4. **Write Tests:**
   Add unit or integration tests to cover your changes. This helps maintain code quality and prevents future bugs.

5. **Update Documentation:**
   If your changes affect the usage of Elasti, update the relevant documentation in README.md or other documentation files.

6. **Sign Your Commits & Push:**
   Sign your commits to certify that you wrote the code and have the right to pass it on as an open-source contribution:

   ```bash
   git commit -s -m "Your commit message"
   git push origin feature/your-feature-name
   ```

7. **Create a Pull Request:**
   Navigate to the original Elasti repository and submit a pull request from your branch. Provide a clear description of your changes and the motivation behind them. If your pull request addresses any open issues, link them in the description. Use keywords like fixes #issue_number to automatically close the issue when the pull request is merged.

8. **Review Process:**
   Your pull request will be reviewed by project maintainers. Be responsive to feedback and make necessary changes. Post review, it will be merged!

<div align="center"> <b> You just contributed to Elasti! </b></div>
<div align="center">

<img src="./docs/assets/awesome.gif" width="400px">
</div>

## Getting Help

If you need help or have questions, feel free to reach out to the community. You can:

- Open an issue for discussion or help.
- Join our community chat or mailing list.
- Refer to the FAQ and Troubleshooting Guide.

## Acknowledgements

Thank you for contributing to Elasti! Your contributions make the project better for everyone. We look forward to collaborating with you.

# Future Developments

- Support GRPC, Websockets.
- Test multiple ports in same service.
- Seperate queue for different services.
- Unit test coverage.
