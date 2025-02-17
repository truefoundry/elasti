# Getting Started

With Elasti, you can easily manage and scale your Kubernetes services by using a proxy mechanism that queues and holds requests for scaled-down services, bringing them up only when needed. Get started by following below steps:

## Prerequisites

- **Kubernetes Cluster:** You should have a running Kubernetes cluster. You can use any cloud-based or on-premises Kubernetes distribution.
- **kubectl:** Installed and configured to interact with your Kubernetes cluster.
- **Helm:** Installed for managing Kubernetes applications.

## Install

### 1. Install Elasti using helm

Use Helm to install elasti into your Kubernetes cluster. Replace `<release-name>` with your desired release name and `<namespace>` with the Kubernetes namespace you want to use:

```bash
helm install <release-name> oci://tfy.jfrog.io/tfy-helm/elasti --namespace <namespace> --create-namespace
```
Check out [values.yaml](./charts/elasti/values.yaml) to see config in the helm value file.

### 2. Verify the Installation

Check the status of your Helm release and ensure that the elasti components are running:

```bash
helm status <release-name> --namespace <namespace>
kubectl get pods -n <namespace>
```

You will see 2 components running.

1.  **Controller/Operator:** `elasti-operator-controller-manager-...` is to switch the traffic, watch resources, scale etc.
2.  **Resolver:** `elasti-resolver-...` is to proxy the requests.

Refer to the [Docs](./docs/architecture) to know how it works.

## Configuration

To configure a service to handle its traffic via elasti, you'll need to create and apply a `ElastiService` custom resource:

### 1. Define an ElastiService

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
  
Below is an example configuration for an ElastiService.
```yaml
apiVersion: elasti.truefoundry.com/v1alpha1
kind: ElastiService
metadata:
  name: httpbin
spec:
  service: httpbin
  minTargetReplicas: 1
  cooldownPeriod: 300
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployments
    name: httpbin
  triggers:
    - type: prometheus
      metadata:
        query: sum(rate(istio_requests_total{destination_service="httpbin.demo.svc.cluster.local"}[1m])) or vector(0)
        serverAddress: http://prometheus-server.prometheus.svc.cluster.local:9090
        threshold: "0.01"
```

### 2. Apply the configuration

Apply the configuration to your Kubernetes cluster:

```bash
kubectl apply -f <service-name>-elasti-CRD.yaml
```

### 3. Check Logs

You can view logs from the controller to watchout for any errors.

```bash
kubectl logs -f deployment/elasti-operator-controller-manager -n <namespace>
```

## Uninstall

To uninstall Elasti, **you will need to remove all the installed ElastiServices first.** Then, simply delete the installation file.

```bash
kubectl delete elastiservices --all
helm uninstall <release-name> -n <namespace>
kubectl delete namespace <namespace>
```