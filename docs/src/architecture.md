# KubeElasti Architecture


KubeElasti comprises two main components: operator and resolver.

- **Controller/Operator**: A Kubernetes controller built using kubebuilder. It monitors ElastiService resources and scales them to 0 or 1 as needed.

- **Resolver**: A service that intercepts incoming requests for scaled-down services, queues them, and notifies the elasti-controller to scale up the target service.

``` mermaid
graph LR
  %% ───────────────────────────
  %%  SUBGRAPHS (logical zones)
  %% ───────────────────────────
  subgraph INGRESS [" "]
    Gateway[Gateway]
  end



  subgraph CONTROL_PLANE ["KubeElasti"]
    Operator[Operator]
    Resolver[Resolver]
  end

  LoadGen[Internal-load-generator-pod]

  subgraph ELASTI_CRD ["CRDs"]
    ESCRD((ElastiService<br>CRD))
  end



    TargetSVC{Target-SVC}
    TargetSVC_PVT{Target‑SVC<br>Private}
  

  subgraph ENDPOINT ["Endpoints"]
    SVC_EP([target-SVC<br>endpoints])
    SVC_EPS([target-SVC-y2b93<br>endpointSlice])
    ResEPS([target-SVC-to-resolver<br>endpointSlice])
  end

  %% ───────────────────────────
  %%  TRAFFIC FLOWS (solid)
  %% ───────────────────────────
  Gateway -->|1: traffic| TargetSVC
  LoadGen -->|1: traffic| TargetSVC

  %% Serve‑mode path
  TargetSVC -->|2: Serve Mode| SVC_EP
  SVC_EP --> SVC_EPS
  SVC_EPS --> Pod

  %% Proxy‑mode path
  TargetSVC -->|7: Proxy Mode| ResEPS
  ResEPS -->|9: Req| Resolver
  Resolver -->|11: Send Proxy Request<br>once pod ready| TargetSVC_PVT
  TargetSVC_PVT -->|12: Send and Receive Req| Pod

  %% ───────────────────────────
  %%  OPERATOR / WATCH (dashed)
  %% ───────────────────────────
  ESCRD -. "0: Watch CRD" .-> Operator
  Operator -. "0: Watch ScaleTargetRef" .-> Operator
  Operator -. "4: When target scaled to 0" .-> Operator
  Operator -. "4: Create + add resolver POD IPs" .-> ResEPS
  Operator -. "5: Watch Resolver → endpointslice" .-> Resolver
  Operator -. "6: Create PVT SVC" .-> TargetSVC_PVT
  Operator -. "7: Watch public SVC → private SVC" .-> TargetSVC
  Operator -. "12: Send Traffic Info" .-> Resolver
  Resolver -. "13: Scale ScaleTargetRef<br> and reverse 4,5" .-> Operator

  Pod -. "3: Pod scaled to 0 via HPA/KEDA" .-> Operator

```

## Operator Architecture

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

## Resolver Architecture


``` mermaid
flowchart LR
  %% ── USER & ENTRY ─────────────────────
  User(("Client")) --> RP["Proxy<br/>:8012"] --> Main["Main<br/>cmd/main.go"] --> IS["Metrics<br/>:8013"]

  %% ── K8s RESOURCES ────────────────────
  subgraph K8s["K8s"]
    Deploy["Deployment"] --> SVC["Service"]
    Deploy --> Mon["Monitoring<br/>configs"]
  end
  Main --> Deploy

  %% ── CORE MODULES ─────────────────────
  subgraph Mods["Core Modules"]
    Handler["Handler"]:::core
    Hosts["Hosts"]:::core
    Thr["Throttle"]:::core
    Oper["Operator Comm"]:::core
    Obs["Observability"]:::core
  end
  Main -- uses --> Handler & Hosts & Thr & Oper & Obs

  %% Request flow (compact arrows)
  Handler --> Hosts
  Handler --> Thr
  Thr --> Handler
  Handler --> Obs
  Handler -.-> Sentry["Sentry"]

  %% Operator comm
  Handler -.-> Oper
  Oper -.-> OpSvc["Operator Svc"]

  %% External deps
  Thr -.-> K8sAPI["K8s API"]
  Obs -.-> Prom["Prometheus"]



```

## Flow Description

``` mermaid
graph LR
  A["Steady State(regular traffic flow)"] --> B["No Traffic, scale service to 0"]
  B --> C["New Incoming Traffic, scale service to 1"]
  C --> A
```

When we enable KubeElasti on a service, the service operates in 3 modes:

1. **Steady State**: The service is receiving traffic and doesn't need to be scaled down to 0.
2. **Scale Down to 0**: The service hasn't received any traffic for the configured duration and can be scaled down to 0.
3. **Scale up from 0**: The service receives traffic again and can be scaled up to the configured minTargetReplicas.

### Steady state flow of requests to service

In this mode, all requests are handled directly by the service pods; the KubeElasti **resolver** is not involved. The KubeElasti controller continually polls Prometheus with the configured query and checks the result against the threshold value to decide whether the service can be scaled down.

``` mermaid
---
title: No incoming requests for the configured time period
displayMode: compact
config:
  layout: elk
  look: classic
  theme: dark
---

graph LR
    A[User Request] --> B[Ingress]
    B --> C[Service]
    C -->|Active| D[Pods]

    subgraph Elasti Components
        E[Elasti Controller]
        F[Elasti Resolver]
    end

    C -.->|Inactive| F

    E -->|Poll configured metric every 30 seconds to check if the service can be scaled to 0| S[Prometheus]

```

### Scale down to 0 when there are no requests

If the query from prometheus returns a value less than the threshold, KubeElasti will scale down the service to 0. Before it scales to 0, it redirects all requests to the KubeElasti resolver, then sets the rollout/deployment replicas to 0. It also pauses KEDA (if in use) to prevent it from scaling the service up, because KEDA is configured with `minReplicas: 1`.

``` mermaid
---
title: No incoming requests for the configured time period
displayMode: compact
config:
  layout: elk
  look: classic
  theme: dark
---

graph LR
    A[User Request] --> B[Ingress]
    B --> C[Service]

    subgraph Elasti Components
        E[Elasti Controller]
        F[Elasti Resolver]
    end

    C -->|Active| F
    E -->|Scale replicas to 0| D
    C -.->|Inactive| D

```

### Scale up from 0 when the first request arrives

Since the service is scaled down to 0, all requests will hit the KubeElasti resolver. When the first request arrives, KubeElasti will scale up the service to the configured minTargetReplicas. It then resumes Keda to continue autoscaling in case there is a sudden burst of requests. It also changes the service to point to the actual service pods once the pod is up. Requests reaching the KubeElasti resolver are retried for up to five minutes before a response is returned to the client. If the pod takes more than 5 mins to come up, the request is dropped.

``` mermaid
---
title: First request to pod arrives
displayMode: compact
config:
  layout: elk
  look: classic
  theme: dark
---

graph LR
    A[User Request] --> B[Ingress]
    B --> C[Service]

    C -.->|Inactive| F[0 Pods]

    subgraph Elasti Components
        D[Elasti Controller]
        E[Elasti Resolver]
    end

    C -->|Active| E
    E -->|Hold request in memory and forward once ready| F
    D -->|Scale replicas up from 0| F

```


``` mermaid
---
title: State after the first replica is up
displayMode: compact
config:
  layout: elk
  look: classic
  theme: dark
---

graph LR
    A[User] -->|Request| B[Ingress]
    B --> C[Service]

    subgraph Elasti Components
        E[Elasti Controller]
        F[Elasti Resolver]
    end

    C -->|Active| G[Pods]
    E -->|Check metric if workload can be scaled to 0| H[Prometheus]
    C -.- |Inactive| F

```
<!-- 
    style D fill:#e0e0e0,stroke:#000
    style E fill:#e0e0e0,stroke:#000
    style B fill:#e0e0ff,stroke:#000
    style C fill:#dddddd,stroke:#000
    style F fill:#bdbdbd,stroke:#000

    classDef active fill:#a6f3a6;
    classDef inactive stroke-dasharray: 5 5, color:red;

    style E fill:#e0e0e0,stroke:#000
    style F fill:#e0e0e0,stroke:#000
    style H fill:#e0e0ff,stroke:#000
    style B fill:#e0e0ff,stroke:#000
    style G fill:#fff3b0,stroke:#000
    style C fill:#dddddd,stroke:#000 
    
     classDef active fill:#a6f3a6;
    classDef inactive stroke-dasharray: 5 5, color:red;
    class C active; 
    
    
-->