
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

---

## 1. **System Containers & Logical Boundaries**

### **a. API & Schema Layer**
- **`api/v1alpha1/`**: Contains the CRD schema (`ElastiService`), including spec, status, versioning, and deep copy functions.
- **`groupversion_info.go`**: Registers API group/version with Kubernetes scheme.

### **b. Controller & Reconciliation Layer**
- **`internal/controller/`**: Implements core reconciliation logic for `ElastiService` resources.
    - **`elastiservice_controller.go`**: Main controller logic, event handlers, and lifecycle management.
    - **`opsCRD.go`**: Handles CRD lifecycle, finalization, and status updates.
    - **`opsDeployment.go`**: Manages Deployment resources, scaling, and mode switching.
    - **`opsServices.go`**: Manages Services, including private/public service synchronization.
    - **`opsEndpointslices.go`**: Manages EndpointSlices for resolver communication.
    - **`opsRollout.go`**: Handles Argo Rollouts for advanced deployment strategies.
    - **`opsModes.go`**: Manages mode switching between `Serve` and `Proxy`.
    - **`opsInformer.go`**: Manages Kubernetes informers for resource event handling.
  - **`suite_test.go` & `elastiservice_controller_test.go`**: Test suites for unit and integration testing.

### **c. Resource & External System Management**
- **`crddirectory/`**: Central registry of active CRDs, their state, and metadata.
- **`internal/elastiserver/`**: HTTP server for external trigger-based scaling requests.
- **`internal/prom/`**: Prometheus metrics collection and exposure.

### **d. Infrastructure & Utilities**
- **`cmd/main.go`**: Entry point, initializes the manager, informers, controllers, web server, and metrics.
- **`internal/informer/`**: Singleton informer manager to prevent redundant watches, with lifecycle control.
- **`internal/prom/`**: Metrics definitions and instrumentation.

---

## 2. **Major System Components & Their Responsibilities**

### **a. Kubernetes CRD & API Types**
- **`ElastiService`**: Defines the desired state (spec) and observed state (status). Includes fields for scaling (`scaleTargetRef`), autoscaling (`autoscaler`), triggers, and mode.
- **Versioning & Schema**: Ensures backward compatibility and schema validation.

### **b. Controller & Reconciliation Logic**
- **`ElastiServiceReconciler`**:
    - Watches `ElastiService` resources.
    - Handles creation, updates, and deletion.
    - Manages finalizers for cleanup.
    - Coordinates with resource managers to create/update/delete Deployments, Services, EndpointSlices.
    - Implements mode switching (`Serve` vs `Proxy`) via `switchMode()`.
    - Watches external resources (Deployments, Rollouts, Services) for state changes.
    - Maintains a cache of CRD details in `crddirectory`.

- **Reconciliation Workflow**:
    - Fetches the CRD.
    - Checks for deletion; if so, cleans up resources.
    - Adds finalizers if needed.
    - Sets up watches for associated resources.
    - Updates status and metrics.
    - Reconciles existing resources on startup.

### **c. Resource Management & Lifecycle**
- **Deployment Management (`opsDeployment.go`)**:
    - Handles scaling logic based on Deployment status.
    - Switches between `Serve` and `Proxy` modes depending on deployment readiness.
- **Service Management (`opsServices.go`)**:
    - Creates private services for internal communication.
    - Synchronizes private services with public services.
- **EndpointSlice Management (`opsEndpointslices.go`)**:
    - Creates/updates EndpointSlices for resolver communication.
    - Retrieves resolver pod IPs for dynamic endpoint updates.
- **Rollout Management (`opsRollout.go`)**:
    - Monitors Argo Rollouts for deployment health.
    - Switches modes based on rollout status.

### **d. Mode Switching (`opsModes.go`)**
- Switches between:
    - **Serve Mode**: Directly exposes the deployment via a Service, disables resolver endpoint slices.
    - **Proxy Mode**: Uses EndpointSlices to route traffic through a resolver, enabling dynamic scaling from zero.

### **e. External HTTP Server (`elastiserver`)**
- Listens for scaling requests from an external resolver.
- Validates requests, updates last scaled time, and triggers scaling via `scaling.ScaleHandler`.
- Supports graceful shutdown and observability.

### **f. Informer Management (`internal/informer`)**
- Singleton manager to prevent multiple watches on the same resource.
- Manages lifecycle, restarts, and health checks.
- Provides resource-specific watches (Deployments, Services, CRDs).

### **g. Metrics & Observability (`internal/prom`)**
- Tracks reconciliation durations, CRD updates, informer activity, scaling events.
- Exposes `/metrics` endpoint for Prometheus scraping.

### **h. CRD Lifecycle & Finalization (`opsCRD.go`)**
- Adds/removes finalizers.
- Handles cleanup during deletion (EndpointSlices, private services, registry cleanup).
- Updates CRD status with last reconciled time and mode.

---

## 3. **External Dependencies & Integrations**

- **Kubernetes API & Client**:
    - Uses `client-go` and `controller-runtime` for resource management, event handling, and controller lifecycle.
- **Prometheus**:
    - Metrics collection for reconciliation, informer activity, scaling, and mode.
- **Kustomize**:
    - Deployment manifests, RBAC, CRDs, and monitoring configurations.
- **Sentry**:
    - Error tracking integrated into main process.
- **External HTTP Resolver**:
    - Custom server (`elastiserver`) for scaling triggers, integrating with external orchestrators or autoscalers.

---

## 4. **Data & Control Flow**

### **a. Resource Lifecycle & Reconciliation**
- On startup:
    - Loads existing `ElastiService` CRDs.
    - Reconciles each resource to ensure Deployment, Service, EndpointSlices are aligned.
    - Sets up watches for Deployment, Rollouts, Services, and external resolver.
- During runtime:
    - Event-driven updates via informers trigger reconciliation.
    - Mode switching occurs based on Deployment/ Rollout status.
    - External HTTP requests trigger scaling actions, updating the CRD status and scaling the target.

### **b. Mode Switching & Resource Adjustment**
- **Serve Mode**:
    - Deletes EndpointSlices to resolver.
    - Ensures services are exposed directly.
- **Proxy Mode**:
    - Creates/updates EndpointSlices with resolver pod IPs.
    - Sets up watches on public services to sync private services.

### **c. External Scaling Requests**
- External resolver sends POST requests to `/informer/incoming-request`.
- Handler:
    - Validates request.
    - Looks up CRD in directory.
    - Updates last scaled time.
    - Unpauses Keda scaled objects if applicable.
    - Initiates scaling via `scaling.ScaleTargetFromZero`.

