# Setup

Get started by following below steps:

## Prerequisites

- **Kubernetes Cluster:** You should have a running Kubernetes cluster. You can use any cloud-based or on-premises Kubernetes distribution.
- **kubectl:** Installed and configured to interact with your Kubernetes cluster.
- **Helm:** Installed for managing Kubernetes applications.
- **Prometheus:** You should have a prometheus installed in your cluster.
??? example "Installing Prometheus"
    We will setup a sample prometheus to read metrics from the ingress controller.

    ```bash
    helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
    helm repo update
    helm install kube-prometheus-stack prometheus-community/kube-prometheus-stack \
      --namespace monitoring \
      --create-namespace \
      --set alertmanager.enabled=false \
      --set grafana.enabled=false \
      --set prometheus.prometheusSpec.serviceMonitorSelectorNilUsesHelmValues=false
    ```
- **Ingress Controller:** You should have an ingress controller installed in your cluster.
??? example "Installing Ingress Controller"
    
    === "NGINX"
        ```bash
          helm repo add ingress-nginx https://kubernetes.github.io/ingress-nginx
          helm repo update
          helm upgrade --install nginx-ingress ingress-nginx/ingress-nginx \
            --namespace nginx \
            --set controller.metrics.enabled=true \
            --set controller.metrics.serviceMonitor.enabled=true \
            --create-namespace
        ```

    === "Istio"
        ```shell
        # Download the latest Istio release from the official Istio website.
        curl -L https://istio.io/downloadIstio | sh -
        # Move it to home directory
        mv istio-x.xx.x ~/.istioctl
        export PATH=$HOME/.istioctl/bin:$PATH

        istioctl install --set profile=default -y

        # Label the namespace where you want to deploy your application to enable Istio sidecar Injection
        kubectl create namespace <NAMESPACE>
        kubectl label namespace <NAMESPACE> istio-injection=enabled

        # Create a gateway
        kubectl apply -f ./playground/config/gateway.yaml -n <NAMESPACE>
        ```

- **KEDA*:** [Optional] You can have a KEDA installed in your cluster, else HPA can be used.
??? example "Installing KEDA"
    We will setup a sample KEDA to scale the target deployment.

    ```bash
    helm repo add kedacore https://kedacore.github.io/charts
    helm repo update
    helm upgrade --install keda kedacore/keda --namespace keda --create-namespace --wait --timeout 180s
    ```

## Install

### **1. Install KubeElasti using helm**

Use Helm to install KubeElasti into your Kubernetes cluster. 

```bash
helm install elasti oci://tfy.jfrog.io/tfy-helm/elasti --namespace elasti --create-namespace
```

Check out [values.yaml](https://github.com/truefoundry/KubeElasti/blob/main/charts/elasti/values.yaml) to see configuration options in the helm value file.

<br>

### **2. Verify the Installation**

Check the status of your Helm release and ensure that the KubeElasti components are running:

```bash
helm status elasti --namespace elasti
kubectl get pods -n elasti
```

You will see 2 components running.

1.  **Controller/Operator:** `elasti-operator-controller-manager-...` is to switch the traffic, watch resources, scale etc.
2.  **Resolver:** `elasti-resolver-...` is to proxy the requests.

<br>

### **3. Define an ElastiService**

To configure a service to handle its traffic via elasti, you'll need to create and apply a `ElastiService` custom resource.

Here we are creating it for httpbin service.   

Create a file named `elasti-service.yaml` and apply the configuration.

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
      uptimeFilter: <uptime-filter> # (12)
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
12. Replace it with the uptime filter of your TSDB instance.  Default: `container="prometheus"`.
13. Replace it with the autoscaler name. In this case, it is the name of the KEDA ScaledObject.
14. Replace it with the autoscaler type. In this case, it is `keda`.


??? example "Demo ElastiService"
    ```yaml title="elasti-service.yaml" linenums="1"
    apiVersion: elasti.truefoundry.com/v1alpha1
    kind: ElastiService
    metadata:
      name: target-elastiservice
      namespace: target
    spec:
      cooldownPeriod: 5
      minTargetReplicas: 1
      scaleTargetRef:
        apiVersion: apps/v1
        kind: deployments
        name: target-deployment
      service: target-deployment
      triggers:
        - metadata:
            query: round(sum(rate(envoy_http_downstream_rq_total{container="istio-proxy"}[1m])),0.001) or vector(0)
            serverAddress: http://prometheus-operated.monitoring.svc.cluster.local:9090
            threshold: "0.01"
          type: prometheus
      autoscaler:
        name: target-scaled-object
        type: keda
    ```

<br>

### **4. Apply the KubeElasti service configuration**

Apply the configuration to your Kubernetes cluster:

```bash
kubectl apply -f elasti-service.yaml -n <service-namespace>
```

The pod will be scaled down to 0 replicas if there is no traffic.

<br>

### **5. Test the setup**

You can test the setup by sending requests to the nginx load balancer service.

```bash
# For NGINX
kubectl port-forward svc/nginx-ingress-ingress-nginx-controller -n nginx 8080:80

# For Istio
kubectl port-forward svc/istio-ingressgateway -n istio-system 8080:80
```

Start a watch on the target deployment.

```bash
kubectl get pods -n <NAMESPACE> -w
```

Send a request to the service.

```bash
curl -v http://localhost:8080/httpbin
```

You should see the pods being created and scaled up to 1 replica. A response from the   target service should be visible for the curl command.
The target service should be scaled down to 0 replicas if there is no traffic for `cooldownPeriod` seconds.

<br>

## Uninstall

To uninstall Elasti, **you will need to remove all the installed ElastiServices first.** Then, simply delete the installation file.

```bash
kubectl delete elastiservices --all
helm uninstall elasti -n elasti
kubectl delete namespace elasti
```