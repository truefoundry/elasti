# Dev Environment

Setting up your development environment for KubeElasti involves preparing your local setup for building, testing, and contributing to the project. Follow these steps to get started:

## 1. Get required tools

Ensure you have the following tools installed:

- **Go:** The programming language used for KubeElasti. Download and install it from [golang.org](https://golang.org/dl/).
- **Docker:** For containerization and building Docker images. Install it from [docker.com](https://www.docker.com/get-started).
- **kubectl:** Command-line tool for interacting with Kubernetes. Install it from [kubernetes.io](https://kubernetes.io/docs/tasks/tools/).
- **Helm:** Package manager for Kubernetes. Install it from [helm.sh](https://helm.sh/docs/intro/install/).
- **Docker Desktop/Kind/Minikube:** A local kubernetes cluster. Make sure you have the local cluster running before development.
- **Make:** Helps in working with the project.
- **Istio:** Required to test the project with istio. Install from [istio.io](https://istio.io/)
- **k6:** Required to load test the project. Install from [k6.io](https://k6.io/)

## 2. Clone the Repository

Clone the KubeElasti repository from GitHub to your local machine:

```bash
git clone https://github.com/truefoundry/KubeElasti.git
cd KubeElasti
```

!!! tip "Make sure you check out the documentation and architecture before making your changes."

## 3. Repository Structure

Understanding the repository structure will help you navigate and contribute effectively to the KubeElasti project. Below is an overview of the key directories and files in the repository:

```bash
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

### Main Modules

- **`./operator`:** Contains the code for Kubernetes operator, created using kubebuilder.
  ```bash
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

### Other Directories

- **`./playground`:** Code to setup a playground to try and test KubeElasti.
- **`./test`:** Load testing scripts.
- **`./pkg`:** Common packages, shared by Operator and Resolve.
- **`./charts`:** Helm chart template.
- **`./docs`:** Detailed documentation on the HLD, LLD and Architecture of KubeElasti.
