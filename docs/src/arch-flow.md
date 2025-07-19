
# **Flow Description**

``` mermaid
graph TB
  A["Steady State (regular traffic flow)"] --> B["Scale to 0: No Traffic"]
  B --> C["Scale up from 0: New Incoming Traffic"]
  C --> A
```

When we enable KubeElasti on a service, the service operates in 3 modes:

1. **Steady State**: The service is receiving traffic and doesn't need to be scaled down to 0.
2. **Scale Down to 0**: The service hasn't received any traffic for the configured duration and can be scaled down to 0.
3. **Scale up from 0**: The service receives traffic again and can be scaled up to the configured minTargetReplicas.
   


## **1. Steady State:** Flow of requests to service

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


## **2. Scale Down to 0:** when there are no requests

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
    E -->|Scale replicas to 0| D[Pods]
    C -.->|Inactive| D

```
### How it works? 

#### 1. Switching to Proxy Mode

This is how we decide to switch to proxy mode.

```mermaid
sequenceDiagram
    loop Background Tasks
    Operator-->>ElastiCRD: Watch CRD for changes in ScaleTargetRef. 
    
    Note right of Operator:  Watch ScaleTargetRef and Triggers
    Operator-->>TargetService: Watch if ScaleTargetRef is scaled to 0 by <Br>any external component
    Operator-->>Prometheus: Poll configured metric every 30 seconds<br> to check if the ScaleTargetRef has not received any traffic
    
    Note right of Operator: If not traffic received for the configured <br> time period, Operator will switch to proxy mode.

    Operator->>TargetService: Scale replicas to 0
    Operator->>ElastiCRD: Switch to proxy mode.
    end
```


#### 2. Redirecting requests to resolver
This is how we redirect requests to resolver.

```mermaid
sequenceDiagram

Note right of Operator: When in Proxy Mode

    Operator->>EndpointSlice: Create EndpointSlice for TargetService<br> which we want to point to resolver POD IPs

    loop Background Tasks
    Operator-->>Resolver: Watch Resolver POD IPs for changes
    Operator-->>EndpointSlice: Update EndpointSlice with new POD IPs
    end
```

#### 3. Sync Private Service to Public Service
This is how we send traffic to target pod, even if the public service is pointing to resolver. We create a Private Service, as in Proxy Mode, we redirect the traffic to Resolver, <br> so we need to point the public service to resolver POD IPs.

```mermaid
sequenceDiagram

Note right of Operator: When in Proxy Mode

    Operator->>TargetPrivateService: Create private service
    loop Background Tasks

    Operator-->>TargetService: Watch changes in label and IPs <br> in public service.
    Operator-->>TargetPrivateService: Update label and IPs in <Br> private service to match Public Service.
    end
```


## **3. Scale up from 0:** when the first request arrives

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


### How it works? 

#### 1. Bring the pod up

```mermaid
sequenceDiagram    
    Note right of Operator: When in Proxy Mode

    Gateway->>TargetService: 1. External or Internal traffic
    TargetService->>Resolver: 2. Forward request
    par 
        Resolver->>Resolver: 3. Queue requests <br>in-memory (Req remains alive)
        Resolver->>Operator: 4. Inform about the incoming request
    end

    par
        Operator->>TargetService: 5. Scale up via HPA or KEDA
        Operator->>Resolver: 6. Send info about target private service
    end

```

#### 2. Resolving queued requests

```mermaid
sequenceDiagram 
    loop
        Resolver->>Pod: 7: Check if pod is up
    end

    par
        Resolver->>TargetSvcPvt: 8: Send proxy request
        TargetSvcPvt->>Pod: 9: Send & receive req
    end

    Note right of Resolver: Once pod is up, switch to serve mode

```

