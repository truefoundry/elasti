```
graph TB
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



``` 
---
title: KubeElasti Architecture
displayMode: compact
config:
  layout: elk
  look: classic
  theme: default
  securityLevel: loose
  flowchart:
    nodeSpacing: 30
    rankSpacing: 40
  fontFamily: "Inter, sans-serif"
  themeVariables:
    fontSize: "14px"
---

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
  TargetSVC --> SVC_EP
  SVC_EP -->|2: Serve Mode| SVC_EPS
  SVC_EPS --> Pod

  %% Proxy‑mode path
  SVC_EP -->|7: Proxy Mode| ResEPS
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
