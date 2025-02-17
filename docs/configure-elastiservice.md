# Configure ElastiService

To enable scale to 0 on any deployment, we will need to create an ElastiService custom resource for that deployment. The ElastiService defines the rules for scaling the deployment and the metrics to use for scaling. 

## Create an ElastiService

Create an ElastiService custom resource for your deployment. 

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