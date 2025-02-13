---
title: Elasti
layout: no_title
---

<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
**Table of Contents**  *generated with [DocToc](https://github.com/thlorenz/doctoc)*

- [Introduction](#introduction)
  - [How It Works](#how-it-works)
  - [Key Features](#key-features)
- [Why use Elasti?](#why-use-elasti)
- [Getting Started](#getting-started)
  - [Prerequisites](#prerequisites)
  - [Install](#install)
    - [1. Install Elasti using helm](#1-install-elasti-using-helm)
    - [2. Verify the Installation](#2-verify-the-installation)
  - [Configuration](#configuration)
    - [1. Define an ElastiService](#1-define-an-elastiservice)
    - [2. Apply the configuration](#2-apply-the-configuration)
    - [3. Check Logs](#3-check-logs)
  - [Monitoring](#monitoring)
  - [Uninstall](#uninstall)
- [Development](#development)
- [Contribution](#contribution)
  - [Getting Started](#getting-started-1)
  - [Getting Help](#getting-help)
  - [Acknowledgements](#acknowledgements)
- [Future Developments](#future-developments)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

<p align="center">
<img src="./docs/logo/banner.png" alt="elasti icon">
</p>

<p align="center">
 <a>
    <img src="https://img.shields.io/badge/license-MIT-blue" align="center">
 </a>

</p>

> This project is in Alpha right now.

# Introduction

Elasti is a Kubernetes-native solution that offers true scale-to-zero functionality while ensuring zero request loss. By dynamically managing service replicas based on real-time traffic conditions, Elasti dramatically reduces resource usage during idle periods without compromising the responsiveness or reliability of your applications.

## How It Works

Elasti continuously monitors an ElastiService by evaluating a set of custom triggers defined in its configuration. These triggers represent various conditions—such as traffic metrics or other custom signals—that determine whether a service should be active or scaled down.

- **Scaling Down:**  
  When **all** triggers indicate inactivity or low demand, Elasti scales the target service down to 0 replicas. During this period, elasti switches into **proxy mode** and queues incoming requests instead of dropping them.

- **Traffic Queueing in Proxy Mode:**  
  In Proxy Mode, Elasti intercepts and queues incoming requests directed at the scaled-down service. This ensures that no request is lost, even when the service is scaled down to 0.

- **Scaling Up:**  
  If **any** trigger signals a need for activity, Elasti immediately scales the service back up to its minimum replicas. As the service comes online, Elasti switches to **Serve Mode**.

- **Serve Mode:**  
  In Serve Mode, the active service handles all incoming traffic directly. Meanwhile, any queued requests accumulated during Proxy Mode are processed, ensuring a seamless return to full operational capacity.

This allows Elasti to optimize resource consumption by scaling services down when unneeded, while its request queueing mechanism preserves user interactions and guarantees prompt service availability when conditions change.
<div align="center">
<img src="./docs/assets/modes.png" width="400px">
</div>

## Key Features

- **Seamless Integration:** Elasti integrates effortlessly with your existing Kubernetes setup. It takes just a few steps to enable scale to zero for any service.

- **Development and Argo Rollouts Support:** Elasti supports two target references: Deployment and Argo Rollouts, making it versatile for various deployment scenarios.

- **HTTP API Support:** Currently, Elasti supports only HTTP API types, ensuring straightforward and efficient handling of web traffic.

- **Prometheus Metrics Export:** Elasti exports Prometheus metrics for easy out-of-the-box monitoring. You can also import a pre-built dashboard into Grafana for comprehensive visualization.

- **Istio Support:** Elasti is compatible with Istio. It also supports East-West traffic using cluster-local service DNS, ensuring robust and flexible traffic management across your services.


# Why use Elasti?

Kubernetes clusters can become costly, especially when running multiple services continuously. Elasti addresses this issue by giving you the confidence to scale down services during periods of low or no traffic, as it can bring them back up when demand increases. This optimization minimizes resource usage without compromising on service availability. Additionally, Elasti ensures reliability by acting as a proxy that queues incoming requests for scaled-down services. Once these services are reactivated, Elasti processes the queued requests, so that no request is lost. This combination of cost savings and dependable performance makes Elasti an invaluable tool for efficient Kubernetes service management.

> The name Elasti comes from a superhero "Elasti-Girl" from DC Comics. Her superpower is to expand or shrink her body at will—from hundreds of feet tall to mere inches in height.

<div align="center"> <b> Demo </b></div>
<div align="center">
    <a href="https://www.loom.com/share/6dae33a27a5847f081f7381f8d9510e6">
      <img style="max-width:640px;" src="https://cdn.loom.com/sessions/thumbnails/6dae33a27a5847f081f7381f8d9510e6-adf9e85a899f85fd-full-play.gif">
    </a>
  </div>

# Getting Started

With Elasti, you can easily manage and scale your Kubernetes services by using a proxy mechanism that queues and holds requests for scaled-down services, bringing them up only when needed. Get started by following below steps:

## Prerequisites

- **Kubernetes Cluster:** You should have a running Kubernetes cluster. You can use any cloud-based or on-premises Kubernetes distribution.
- **kubectl:** Installed and configured to interact with your Kubernetes cluster.
- **Helm:** Installed for managing Kubernetes applications.

## Install

### 1. Install Elasti using helm

Use Helm to install elasti into your Kubernetes cluster. Replace `<release-name>` with your desired release name and `<namespace>` with the Kubernetes namespace you want to use:

```bash
helm install <release-name> oci://tfy.jfrog.io/tfy-helm/elasti --namespace <namespace> --create-namespace
```
Check out [values.yaml](./charts/elasti/values.yaml) to see config in the helm value file.

### 2. Verify the Installation

Check the status of your Helm release and ensure that the elasti components are running:

```bash
helm status <release-name> --namespace <namespace>
kubectl get pods -n <namespace>
```

You will see 2 components running.

1.  **Controller/Operator:** `elasti-operator-controller-manager-...` is to switch the traffic, watch resources, scale etc.
2.  **Resolver:** `elasti-resolver-...` is to proxy the requests.

Refer to the [Docs](./docs/architecture) to know how it works.

## Configuration

To configure a service to handle its traffic via elasti, you'll need to create and apply a `ElastiService` custom resource:

### 1. Define an ElastiService

```yaml
apiVersion: elasti.truefoundry.com/v1alpha1
kind: ElastiService
metadata:
  name: <service-name>
  namespace: <service-namespace>
spec:
  minTargetReplicas: <min-target-replicas>
  service: <service-name>
  cooldownPeriod: <cooldown-period>
  scaleTargetRef:
    apiVersion: <apiVersion>
    kind: <kind>
    name: <deployment-or-rollout-name>
  triggers:
  - type: <trigger-type>
    metadata:
      <trigger-metadata>
  autoscaler:
    name: <autoscaler-object-name>
    type: <autoscaler-type>
```

- `<service-name>`: Replace it with the service you want managed by elasti.
- `<min-target-replicas>`: Min replicas to bring up when first request arrives.
- `<service-namespace>`: Replace by namespace of the service.
- `<scaleTargetRef>`: Reference to the scale target similar to the one used in HorizontalPodAutoscaler.
- `<kind>`: Replace by `rollouts` or `deployments`
- `<apiVersion>`: Replace with `argoproj.io/v1alpha1` or `apps/v1`
- `<deployment-or-rollout-name>`: Replace with name of the rollout or the deployment for the service. This will be scaled up to min-target-replicas when first request comes
- `cooldownPeriod`: Minimum time (in seconds) to wait after scaling up before considering scale down
- `triggers`: List of conditions that determine when to scale down (currently supports only Prometheus metrics)
- `autoscaler`: **Optional** integration with an external autoscaler (HPA/KEDA) if needed
  - `<autoscaler-type>`: hpa/keda
  - `<autoscaler-object-name>`: name of the KEDA ScaledObject or HPA HorizontalPodAutoscaler object
  
Below is an example configuration for an ElastiService.
```yaml
apiVersion: elasti.truefoundry.com/v1alpha1
kind: ElastiService
metadata:
  name: httpbin
spec:
  service: httpbin
  minTargetReplicas: 1
  cooldownPeriod: 300
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployments
    name: httpbin
  triggers:
    - type: prometheus
      metadata:
        query: sum(rate(istio_requests_total{destination_service="httpbin.demo.svc.cluster.local"}[1m])) or vector(0)
        serverAddress: http://prometheus-server.prometheus.svc.cluster.local:9090
        threshold: "0.01"
```

### 2. Apply the configuration

Apply the configuration to your Kubernetes cluster:

```bash
kubectl apply -f <service-name>-elasti-CRD.yaml
```

### 3. Check Logs

You can view logs from the controller to watchout for any errors.

```bash
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
helm uninstall <release-name> -n <namespace>
kubectl delete namespace <namespace>
```

# Development

Refer to [DEVELOPMENT.md](./DEVELOPMENT.md) for more details.

# Contribution

We welcome contributions from the community to improve Elasti. Whether you're fixing bugs, adding new features, improving documentation, or providing feedback, your efforts are appreciated. Follow the steps below to contribute effectively to the project.

## Getting Started

Follows the steps mentioned in [development](DEVELOPMENT.md) to set up the development environment.

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
- Separate queue for different services.
- Unit test coverage.
