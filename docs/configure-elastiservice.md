# Configure ElastiService

To enable scale to 0 on any deployment, we will need to create an ElastiService custom resource for that deployment. 

A ElastiService custom resource has the following structure:

```yaml
apiVersion: elasti.truefoundry.com/v1alpha1
kind: ElastiService
metadata:
  name: <service-name>
  namespace: <service-namespace>
spec:
  minTargetReplicas: <min-target-replicas>
  service: <service-name>
  cooldownPeriod: <cooldown-period>
  scaleTargetRef:
    apiVersion: <apiVersion>
    kind: <kind>
    name: <deployment-or-rollout-name>
  triggers:
  - type: <trigger-type>
    metadata:
      <trigger-metadata>
  autoscaler:
    name: <autoscaler-object-name>
    type: <autoscaler-type>
```
The key fields to be specified in the spec are:

- `<service-name>`: Replace it with the service you want managed by elasti.
- `<service-namespace>`: Replace by namespace of the service.
- `<min-target-replicas>`: Min replicas to bring up when first request arrives.
- `<scaleTargetRef>`: Reference to the scale target similar to the one used in HorizontalPodAutoscaler.
- `<kind>`: Replace by `rollouts` or `deployments`
- `<apiVersion>`: Replace with `argoproj.io/v1alpha1` or `apps/v1`
- `<deployment-or-rollout-name>`: Replace with name of the rollout or the deployment for the service. This will be scaled up to min-target-replicas when first request comes
- `cooldownPeriod`: Minimum time (in seconds) to wait after scaling up before considering scale down
- `triggers`: List of conditions that determine when to scale down (currently supports only Prometheus metrics)
- `autoscaler`: **Optional** integration with an external autoscaler (HPA/KEDA) if needed
  - `<autoscaler-type>`: keda
  - `<autoscaler-object-name>`: Name of the KEDA ScaledObject

The configuration helps define the different functionalities of Elasti.

### Which service to apply elasti on

This is defined using the `scaleTargetRef` field in the spec. 

- `scaleTargetRef.kind`: should be either be  `deployments` or `rollouts` (in case you are using Argo Rollouts). 
- `scaleTargetRef.apiVersion` will be `apps/v1` if you are using deployments or `argoproj.io/v1alpha1` in case you are using argo-rollouts. 
- `scaleTargetRef.name` should exactly match the name of the deployment or rollout. 

### When to scale down the service to 0

This is defined uing the triggers field in the spec. Currently, Elasti supports only one trigger type - `prometheus`. The metadata field of the trigger defines the trigger data. The `query` field is the prometheus query to use for the trigger. The `serverAddress` field is the address of the prometheus server. The `threshold` field is the threshold value to use for the trigger. So we can define a query to check for the number of requests per second and the threshold to be 0. Elasti will check this metric every 30 seconds and if the values is less than 0(`threshold`) it will scale down the service to 0.

An example trigger is as follows:

```
triggers:
- type: prometheus
    metadata:
    query: sum(rate(nginx_ingress_controller_nginx_process_requests_total[1m])) or vector(0)
    serverAddress: http://kube-prometheus-stack-prometheus.monitoring.svc.cluster.local:9090
    threshold: 0.5
```

Once the service is scaled down to 0, we also need to pause the current autoscaler to make sure it doesn't scale up the service again. While this is not a problem with HPA, Keda will scale up the service again since the min replicas is 1. Hence Elasti needs to know about the Keda scaled object so that it can pause it. This information is provided in the `autoscaler` field of the ElastiService. The autoscaler type supported as of now is only keda. 

- autoscaler.name: Name of the keda scaled object
- autoscaler.type: keda

### When to scale up the service to 1

As soon as the service is scaled down to 0, Elasti resolved will start accepting requests for that service. On receiving the first request, it will scale up the service to `minTargetReplicas`. Once the pod is up, the new requests are handled by the service pods and do not pass through the elasti-resolver. The requests that came before the pod scaled up are held in memory of the elasti-resolver and are processed once the pod is up.

We can configure the `cooldownPeriod` to specify the minimum time (in seconds) to wait after scaling up before considering scale down.