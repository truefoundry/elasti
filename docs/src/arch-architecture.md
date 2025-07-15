# KubeElasti Architecture

<img src="../../images/hld.png" alt="Unified Architecture of KubeElasti" >

KubeElasti comprises two main components: operator and resolver.

- **Controller/Operator**: A Kubernetes controller built using kubebuilder. It monitors ElastiService resources and scales them to 0 or 1 as needed.

- **Resolver**: A service that intercepts incoming requests for scaled-down services, queues them, and notifies the elasti-controller to scale up the target service.




## Architecture [Serve Mode]

```mermaid
flowchart TB
  %% === Zones ===
  LoadGen[[Load Generator]]
  subgraph INGRESS ["Ingress"]
    Gateway[Gateway]
  end

  subgraph ElastiPlane ["KubeElasti"]
    Operator[Operator]
    Resolver[Resolver]
  end
  
      ESCRD((ElastiService CRD))

subgraph Triggers ["Triggers"]
  Prom{{Prometheus}}
end


%%   subgraph ELASTI_CRD ["ElastiService CRD"]
    
%%   end

subgraph Services
  TargetSVC{Target-SVC}
end

  subgraph Endpoints
    SVC_EPS([EndpointSlice])
  end

  Pod[[Target Pod]]

  %% === Traffic Flow ===
  Gateway -->|1: traffic| TargetSVC
  LoadGen -->|1: traffic| TargetSVC
  TargetSVC -->|2: Serve Mode| SVC_EPS --> Pod

  %% === Operator Flow ===
  ESCRD -. "0: Watch CRD" .-> Operator
  Operator -->|4: Scale to 0| Pod
  Operator -->|3: Poll configured metric every 30 seconds to check if the service can be scaled to 0| Triggers
  Operator -->|5: Switch to Proxy Mode| ESCRD
```

## Architecture [Proxy Mode]
```mermaid
flowchart TB

  LoadGen[[Load Generator]]
  subgraph Ingress
    Gateway[Gateway]
  end

  subgraph CONTROL_PLANE ["KubeElasti"]
    Operator[Operator]
    Resolver[Resolver]
  end

  subgraph Services
    TargetSVC{Target-SVC}
    TargetSVC_PVT{Target-SVC-Private}
  end

  subgraph ENDPOINTS ["Endpoints"]
    ResEPS([to-resolver EndpointSlice])
  end


  subgraph scalers ["scalers"]
    Keda{{"KEDA"}}
    HPA{{"HPA"}}
  end

  Pod[[Target Pod]]
  ESCRD((ElastiService CRD))

  Gateway -->|1: traffic| TargetSVC
  LoadGen -->|1: traffic| TargetSVC

  TargetSVC -->|2: Proxy Mode| ResEPS 

  %% === Proxy Flow ===
  ResEPS -->|3: Req| Resolver
  Resolver -->|4: Proxy Request| TargetSVC_PVT 
  TargetSVC_PVT-->|4: Send and Receive| Pod
  Resolver -. "5: Inform about the request" .-> Operator
  Operator -. "6: Map request to Target and send Target-SVC-Prive" .-> Resolver

  %% === Operator & Control Logic ===
  Operator --> |7: Request for scale| scalers
  scalers --> |8: Scale to 1| Pod
  Operator -. "9: Watch if scaled to 1" .-> Pod
  Operator -. "10: Switch to Serve Mod" .-> ESCRD
```



<!-- graph LR

  subgraph dependencies ["dependencies"]
    Prom{{"Prometheus <br><br> Used as a trigger for scale-to-zero"}}
    Keda{{"KEDA <br><br> Used as a scaler"}}
    HPA{{"HPA <br><br> Used as a scaler"}}
  end


  subgraph services ["Services & EndpointSlice"]
    TargetSVC_PVT{Target‑SVC<br><br>Private service created by Operator to send traffic to Target Pod}
     ResEPS([target-SVC-to-resolver<br><br>endpointSlice created by Operator to point traffic from Target Service to Resolver.])
  end

    subgraph CONTROL_PLANE ["KubeElasti"]
    Operator["Operator <br><br> A Kubernetes controller built using kubebuilder. It monitors ElastiService resources and scales them to 0 or 1 as needed."]
    Resolver["Resolver <br><br> A service that intercepts incoming requests for scaled-down services, queues them, and notifies the elasti-controller to scale up the target service."] 
    ESCRD((ElastiService<br>CRD Created to make Target serverless))
  end -->

