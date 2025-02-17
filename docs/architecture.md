---
title: Elasti Design
---

<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
**Table of Contents**  *generated with [DocToc](https://github.com/thlorenz/doctoc)*

- [Elasti Design](#elasti-design)
  - [1. Introduction](#1-introduction)
    - [Overview](#overview)
    - [Key Components](#key-components)
  - [2. Modes](#2-modes)
    - [Sequence of Events](#sequence-of-events)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

# Elasti Design

## 1. Introduction
Elasti is a Kubernetes operator that provides scale to zero capability for services. It depends on a few external components to successfully provide scale to zero capability for Kubernetes services. In a typical Elasti setup, the components are deployed as follows:

### Key Components
- **Operator**: A Kubernetes controller built using kubebuilder. It monitors the trigger metrics to scale the target service from and to 0 replicas.
- **Resolver**: A service that intercepts incoming requests for scaled-down services, queues them, and notifies the elasti-operator to scale up the target service.
- **Ingress solution**: An ingress solution that handles the routing of requests to the target service and can emit metrics to prometheus which can be used by the operator to make scaling decisions. Eg - Istio, Nginx ingress controller etc
- **Prometheus**: A monitoring system that provides metrics for the target service to the operator to make scaling decisions.
- **[Optional] Keda/HPA**: A Kubernetes component that takes care of scaling the target service beyond the minReplicas.

## 2. Modes
<div align="center">
<img src="./assets/modes.png" width="1000px">
</div>
A service managed by Elasti can be in one of the following modes:
- **Serve**: The service is serving requests normally.
- **Proxy**: The service is scaled to 0 replicas and the resolver is in the path to intercept requests and queue them.

## 3. Sequence of Events

1. Operator scales down the target service to 0 replicas if the trigger metric is below the threshold. Service is moved to `Proxy` mode.
2. A new request comes in.
3. Resolver intercepts the request and queues it.
4. Operator scales up the target service to `minReplicas`
5. Resolver forwards the request to the target service and returns the response to the client.
5. Operator removes the resolver from the http path and the service is moved to `Serve` mode.
