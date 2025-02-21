<p align="center">
<img src="./docs/logo/banner.png" alt="elasti icon">
</p>

<p align="center">
 <a>
    <img src="https://img.shields.io/badge/license-MIT-blue" align="center">
 </a>

</p>

> This project is in Alpha right now.

# Why use Elasti?

Kubernetes clusters can become costly, especially when running multiple services continuously. Elasti addresses this issue by giving you the confidence to scale down services during periods of low or no traffic, as it can bring them back up when demand increases. This optimization minimizes resource usage without compromising on service availability. Additionally, Elasti ensures reliability by acting as a proxy that queues incoming requests for scaled-down services. Once these services are reactivated, Elasti processes the queued requests, so that no request is lost. This combination of cost savings and dependable performance makes Elasti an invaluable tool for efficient Kubernetes service management.

> The name Elasti comes from a superhero "Elasti-Girl" from DC Comics. Her superpower is to expand or shrink her body at will from hundreds of feet tall to mere inches in height.

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
- [Contribution](#contribution)
  - [Getting Started](#getting-started-1)
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

Details on how to install and configure Elasti can be found in the [Getting Started](./docs/getting-started.md) guide.

## Configuration

Check out the different ways to configure Elasti in the [Configuration](./docs/configure-elastiservice.md) guide.

## Monitoring

Monitoring details can be found in the [Monitoring](./docs/monitoring.md) guide.

# Development

Refer to [DEVELOPMENT.md](./DEVELOPMENT.md) for more details.

# Contribution

Contribution details can be found in the [Contribution](./CONTRIBUTING.md) guide.

# Getting Help

We have a dedicated [Discussions](https://github.com/truefoundry/elasti/discussions) section for getting help and discussing ideas.
