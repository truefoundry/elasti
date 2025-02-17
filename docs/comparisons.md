# Comparisons with Other Solutions

This document compares Elasti with other popular serverless and scale-to-zero solutions in the Kubernetes ecosystem.

## Knative

### Overview
Knative is a comprehensive platform for deploying and managing serverless workloads on Kubernetes. It provides a complete serverless experience with features like scale-to-zero, request-based autoscaling, and traffic management.

### Key Differences
- **Complexity**: Knative is a full-featured platform that requires significant setup and maintenance. Elasti is focused solely on scale-to-zero functionality and can be added to existing services with minimal configuration.
- **Integration**: Knative requires services to be deployed as Knative services. Elasti works with existing Kubernetes deployments and Argo Rollouts without modification.
- **Learning Curve**: Knative has a steeper learning curve due to its many concepts and components. Elasti follows familiar Kubernetes patterns with simple CRD-based configuration.

## OpenFaaS

### Overview
OpenFaaS is a framework for building serverless functions with Docker and Kubernetes, making it easy to deploy serverless functions to any cloud or on-premises.

### Key Differences
- **Purpose**: OpenFaaS is primarily designed for Function-as-a-Service (FaaS) workloads. Elasti is built for existing HTTP services.
- **Architecture**: OpenFaaS requires functions to be written and packaged in a specific way. Elasti works with any HTTP service without code changes.
- **Scaling**: OpenFaaS uses its own scaling mechanisms. Elasti integrates with existing autoscalers (HPA/KEDA) while adding scale-to-zero capability.

## KEDA HTTP Add-on

### Overview
KEDA HTTP Add-on is an extension to KEDA that enables HTTP-based scaling, including scale-to-zero functionality.

### Key Differences
- **Maturity**: KEDA HTTP Add-on is in beta and not recommended for production use
- **Request Handling**: 
  - KEDA http add-on inserts itself in the http path and handles requests even when the service has been scaled up.
  - Elasti takes itself out of the http path once the service has been scaled up.
- **Integration**:
  - KEDA HTTP Add-on requires KEDA installation and configuration.
  - Elasti can work standalone or integrate with KEDA if needed.

## Feature Comparison Table

| Feature | Elasti | Knative | OpenFaaS | KEDA HTTP Add-on |
|---------|---------|----------|-----------|------------------|
| Scale to Zero | ✅ | ✅ | ✅ | ✅ |
| Works with Existing Services | ✅ | ❌ | ❌ | ✅ |
| Resource Footprint | Low | High | Medium | Low |
| Setup Complexity | Low | High | Medium | Medium |

## When to Choose Elasti

Elasti is the best choice when you:
1. Need to add scale-to-zero capability to existing HTTP services
2. Want to ensure zero request loss during scaling operations
3. Prefer a lightweight solution with minimal configuration
4. Need integration with existing autoscalers (HPA/KEDA)