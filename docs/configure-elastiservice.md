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


The ElastiService defines the following parameters for the elasti-controller:

**1. Which service to apply elasti on** 

This is defined using the `scaleTargetRef` field in the spec. This `scaleTargetRef.kind` should be either be  `deployments` or `rollouts` (in case you are using Argo Rollouts). The `scaleTargetRef.apiVersion` will be `apps/v1` if you are using deployments or `argoproj.io/v1alpha1` in case you are using argo-rollouts. The `scaleTargetRef.name` should exactly match the name of the deployment or rollout. 

**1. When to scale down the service to 0**

This is defined uing the triggers field in the spec. Currently, Elasti supports only one trigger type - `prometheus`. The metadata field of the trigger defines the trigger data. The `query` field is the prometheus query to use for the trigger. The `serverAddress` field is the address of the prometheus server. The `threshold` field is the threshold value to use for the trigger. So we can define a query to check for the number of requests per second and the threshold to be 0. Elasti will check this metric every 30 seconds and if the values is less than 0(`threshold`) it will scale down the service to 0.

**2. When to scale up the service to 1**


The description of all the fields are mentioned below:

- `<service-name>`: Replace it with the service you want managed by elasti.
- `<min-target-replicas>`: Min replicas to bring up when first request arrives.
- `<service-namespace>`: Replace by namespace of the service.
- `<scaleTargetRef>`: Reference to the scale target similar to the one used in HorizontalPodAutoscaler.
- `<kind>`: Replace by `rollouts` or `deployments`
- `<apiVersion>`: Replace with `argoproj.io/v1alpha1` or `apps/v1`
- `<deployment-or-rollout-name>`: Replace with name of the rollout or the deployment for the service. This will be scaled up to min-target-replicas when first request comes
- `cooldownPeriod`: Minimum time (in seconds) to wait after scaling up before considering scale down
- `triggers`: List of conditions that determine when to scale down (currently supports only Prometheus metrics)
- `autoscaler`: **Optional** integration with an external autoscaler (HPA/KEDA) if needed
  - `<autoscaler-type>`: hpa/keda
  - `<autoscaler-object-name>`: name of the KEDA ScaledObject or HPA HorizontalPodAutoscaler object