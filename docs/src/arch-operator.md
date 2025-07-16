
# Operator Architecture

``` mermaid

flowchart LR

%% === API ===
subgraph "API Layer"
  CRD["ElastiService CRD<br/>api/v1alpha1"]:::core
  GroupVersion["Group/Version<br/>groupversion_info.go"]:::core
end

%% === CONTROLLER ===
subgraph "Controller Layer"
  direction TB
  Reconciler["Reconciler<br/>elastiservice_controller.go"]:::core
  Lifecycle["Lifecycle Mgmt<br/>opsCRD.go"]:::core
  Deploy["Deploy Mgmt<br/>opsDeployment.go"]:::core
  SVCs["Service Mgmt<br/>opsServices.go"]:::core
  EPSlices["EndpointSlice Mgmt<br/>opsEndpointslices.go"]:::core
  Rollouts["Rollout Mgmt<br/>opsRollout.go"]:::core
  Modes["Mode Switching<br/>opsModes.go"]:::core
  Informers["Informer Interface<br/>opsInformer.go"]:::core
end

CRD -->|watches| Reconciler
GroupVersion --> Reconciler
Reconciler -->|manages| Deploy & SVCs & EPSlices & Rollouts & Modes
Reconciler -->|uses| Informers

%% === RESOURCE MGMT ===
subgraph "Resource Management"
  CRDReg["CRD Registry<br/>crddirectory/"]:::core
  Server["ElastiServer<br/>elastiserver/"]:::http
  Prom["Prometheus Client<br/>prom/"]:::metrics
end

Reconciler -->|updates| CRDReg
Server -->|scale requests| Reconciler
Prom -->|collects| Reconciler

%% === INFRASTRUCTURE ===
subgraph "Infrastructure & Boot"
  Main["Entry Point<br/>main.go"]:::core
  InfMgr["Informer Manager<br/>informer/"]:::core
end

Main -->|initializes| InfMgr -->|manages| Informers

%% === OBSERVABILITY ===
subgraph "Observability"
  Metrics["/metrics endpoint"]:::metrics
end

Reconciler -->|exposes| Metrics
Prom -->|scrapes| Metrics

%% === DATA FLOW ===
subgraph "Scaling Logic"
  ScaleLogic["ScaleTargetFromZero"]:::core
end

Server -->|trigger scale| ScaleLogic
Reconciler -->|syncs state| ScaleLogic

%% === EXTERNAL ===
subgraph "External Dependencies"
  K8s["Kubernetes API<br/>client-go"]:::external
  Kustomize["Kustomize"]:::external
  Sentry["Sentry"]:::external
end

Reconciler -->|uses| K8s
Main --> Kustomize & Sentry

```


## High-Level Summary

The system is a Kubernetes operator that manages `ElastiService` CRDs, which encapsulate the desired state of elastiservices. It leverages a layered architecture with clear separation of concerns:

- **API Layer**: Defines the CRD schema (`api/v1alpha1`) with types, versioning, and deep copy functions.
- **Controller Layer**: Implements reconciliation logic (`internal/controller`) to maintain resource state, handle lifecycle events, and coordinate scaling and mode switching.
- **Resource Management**: Manages Kubernetes resources such as Deployments, Services, EndpointSlices, and CRDs, ensuring they reflect the desired state.
- **Informers & Watchers**: Uses Kubernetes informers (`internal/informer`) for efficient event-driven updates, with a singleton manager to prevent redundant watches.
- **External Integration**: Includes a custom HTTP server (`internal/elastiserver`) for handling scaling requests from an external resolver, with Sentry for error tracking.
- **Metrics & Observability**: Prometheus metrics are integrated for observability, tracking reconciliation durations, CRD updates, informer activity, and scaling events.
- **Deployment & Configuration**: Uses Kustomize (`config/`) for managing deployment manifests, RBAC, CRDs, and monitoring configurations.
