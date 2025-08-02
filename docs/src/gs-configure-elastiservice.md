# Configure ElastiService

To enable scale to 0 on any deployment, we will need to create an ElastiService custom resource for that deployment. 

An ElastiService custom resource has the following structure:

```yaml title="elasti-service.yaml" linenums="1"
apiVersion: elasti.truefoundry.com/v1alpha1
kind: ElastiService
metadata:
  name: <service-name> # (1)
  namespace: <service-namespace> # (2)
spec:
  minTargetReplicas: <min-target-replicas> # (3)
  service: <service-name>
  cooldownPeriod: <cooldown-period> # (4)
  scaleTargetRef:
    apiVersion: <apiVersion> # (5)
    kind: <kind> # (6)
    name: <deployment-or-rollout-name> # (7)
  triggers:
  - type: <trigger-type> # (8)
    metadata:
      query: <query> # (9)
      serverAddress: <server-address> # (10)
      threshold: <threshold> # (11)
      uptimeFilter: <uptime-filter> #(12)
  autoscaler:
    name: <autoscaler-object-name> # (13)
    type: <autoscaler-type> # (14)
```

1. Replace it with the service you want managed by elasti.
2. Replace it with the namespace of the service.
3. Replace it with the min replicas to bring up when first request arrives. Minimum: 1
4. Replace it with the cooldown period to wait after scaling up before considering scale down. Default: 900 seconds (15 minutes) | Maximum: 604800 seconds (7 days) | Minimum: 1 second (1 second)
5. ApiVersion should be `apps/v1` if you are using deployments or `argoproj.io/v1alpha1` in case you are using argo-rollouts. 
6. Kind should be either `Deployment` or `Rollout` (in case you are using Argo Rollouts).
7. Name should exactly match the name of the deployment or rollout.
8. Replace it with the trigger type. Currently, KubeElasti supports only one trigger type - `prometheus`. 
9. Replace it with the trigger query. In this case, it is the number of requests per second.
10. Replace it with the trigger server address. In this case, it is the address of the prometheus server.
11. Replace it with the trigger threshold. In this case, it is the number of requests per second.
12. Replace it with the uptime filter of your TSDB instance. Default: `container="prometheus"`.
13. Replace it with the autoscaler name. In this case, it is the name of the KEDA ScaledObject.
14. Replace it with the autoscaler type. In this case, it is `keda`.

The key fields to be specified in the spec are:

- `<service-name>`: Replace it with the service you want managed by elasti.
- `<service-namespace>`: Replace by namespace of the service.
- `<min-target-replicas>`: Min replicas to bring up when first request arrives.
    - Minimum: 1
- `<scaleTargetRef>`: Reference to the scale target similar to the one used in HorizontalPodAutoscaler.
- `<kind>`: Replace by `rollouts` or `deployments`
- `<apiVersion>`: Replace with `argoproj.io/v1alpha1` or `apps/v1`
- `<deployment-or-rollout-name>`: Replace with name of the rollout or the deployment for the service. This will be scaled up to min-target-replicas when first request comes
- `cooldownPeriod`: Minimum time (in seconds) to wait after scaling up before considering scale down. 
    - Default: 900 seconds (15 minutes)
    - Maximum: 604800 seconds (7 days)
    - Minimum: 1 seconds (1 second)
- `triggers`: List of conditions that determine when to scale down (currently supports only Prometheus metrics)
- `autoscaler`: **Optional** integration with an external autoscaler (HPA/KEDA) if needed
    - `<autoscaler-type>`: keda
    - `<autoscaler-object-name>`: Name of the KEDA ScaledObject

---

## Configuration Explanation

The section below explains how the different configuration options are used in KubeElasti.

### **1. scaleTargetRef: Which service KubeElasti should manage**

This is defined using the `scaleTargetRef` field in the spec. 

- `scaleTargetRef.kind`: should be either be  `deployments` or `rollouts` (in case you are using Argo Rollouts). 
- `scaleTargetRef.apiVersion` will be `apps/v1` if you are using deployments or `argoproj.io/v1alpha1` in case you are using argo-rollouts. 
- `scaleTargetRef.name` should exactly match the name of the deployment or rollout. 

<br>

### **2. Triggers: When to scale down the service to 0**

This is defined using the `triggers` field in the spec. Currently, KubeElasti supports only one trigger type - `prometheus`. 
The `metadata` section holds trigger-specific data:  

- **query** - the Prometheus query to evaluate  
- **serverAddress** - address of the Prometheus server  
- **threshold** - numeric threshold that triggers scale-down  

For example, you can query the number of requests per second and set the threshold to `0`.  
KubeElasti polls this metric every 30 seconds, and if the **value** is below the threshold it scales the service to 0.

An example trigger is as follows:

```yaml
triggers:
- type: prometheus
    metadata:
    query: sum(rate(nginx_ingress_controller_nginx_process_requests_total[1m])) or vector(0)
    serverAddress: http://kube-prometheus-stack-prometheus.monitoring.svc.cluster.local:9090
    threshold: 0.5
```

<br>

### **3. Scalers: How to scale up the service to 1**

Once the service is scaled down to 0, we also need to pause the current autoscaler to make sure it doesn't scale up the service again. While this is not a problem with HPA, Keda will scale up the service again since the min replicas is 1. Hence, KubeElasti needs to know about the **KEDA** ScaledObject so that it can pause it. This information is provided in the `autoscaler` field of the ElastiService. Currently, the only supported autoscaler type is **keda**.

```yaml
autoscaler:
  name: <autoscaler-object-name>
  type: keda
```

<br>

### **4. CooldownPeriod: Minimum time (in seconds) to wait after scaling up before considering scale down**

As soon as the service is scaled down to 0, KubeElasti **resolver** will start accepting requests for that service. On receiving the first request, it will scale up the service to `minTargetReplicas`. Once the pod is up, the new requests are handled by the service pods and do not pass through the elasti-resolver. The requests that came before the pod scaled up are held in memory of the elasti-resolver and are processed once the pod is up.

We can configure the `cooldownPeriod` to specify the minimum time (in seconds) to wait after scaling up before considering scale down.