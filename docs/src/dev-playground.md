# Playground

This guide will help you set up a local Kubernetes cluster to test the elasti operator and resolver. Follow these steps from the project home directory.

## 1. Local Cluster

If you don't already have a local Kubernetes cluster, you can set one up using Minikube, Kind or Docker-Desktop:

=== "Kind"

    ``` bash
    kind create cluster --config ./playground/infra/kind-config.yaml
    ```

=== "Minikube"

    ``` bash
    minikube start
    ```

=== "Docker-Desktop"

    Enable it in Docker-Desktop

## 2. Start a Local Docker Registry

Run a local Docker registry container, to push our images locally and access them in our cluster.

``` bash
docker run -d --restart=always -p 5001:5000 --name kind-registry registry:2;

# If you are using kind, connect the registry to kind network
docker network create kind
docker network connect "kind" kind-registry
```

!!! tip "Add registry to Minikube and Kind"
    You will need to add this registry to Minikube and Kind. With Docker-Desktop, it is automatically picked up.

!!! Note "In MacOS, 5000 is not available, so we use 5001 instead."


## 3. Build & Publish Resolver & Operator

Once you have made the necessary changes to the resolver or operator, you can build and publish it using the following commands:

```bash
make -C resolver docker-build docker-push IMG=localhost:5001/elasti-resolver:v1alpha1
make -C operator docker-build docker-push IMG=localhost:5001/elasti-operator:v1alpha1
```

!!! Note "Make sure you have configured the local context in kubectl. With kind, it is automatically picked up,"

## 4. Setup Prometheus

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

## 5. Install Ingress Controller

=== "NGINX"

      Install the NGINX Ingress Controller using Helm:
      ```bash
      helm repo add ingress-nginx https://kubernetes.github.io/ingress-nginx
      helm repo update
      helm upgrade --install nginx-ingress ingress-nginx/ingress-nginx \
        --namespace nginx \
        --set controller.metrics.enabled=true \
        --set controller.metrics.serviceMonitor.enabled=true \
        --create-namespace
      ```

      This will deploy a nginx ingress controller in the `ingress-nginx` namespace.

=== "Istio"

      Install the Istio Ingress Controller using Helm:
      ```shell
      # Download the latest Istio release from the official Istio website.
      curl -L https://istio.io/downloadIstio | sh -
      # Move it to home directory
      mv istio-x.xx.x ~/.istioctl
      export PATH=$HOME/.istioctl/bin:$PATH

      istioctl install --set profile=default -y

      # Label the namespace where you want to deploy your application to enable Istio sidecar Injection
      kubectl create namespace demo
      kubectl label namespace demo istio-injection=enabled

      # Create a gateway
      kubectl apply -f ./playground/config/gateway.yaml
      ```

      This will deploy a Istio ingress controller in the `istio-system` namespace.

## 6. Deploy KubeElasti Locally

We will be using [`playground/infra/elasti-demo-values.yaml`](https://github.com/truefoundry/KubeElasti/blob/main/playground/infra/elasti-demo-values.yaml) for the helm installation. Configure the image uri according to the requirement. Post that follow below steps from the project home directory:

```bash
kubectl create namespace elasti
helm template elasti ./charts/elasti -n elasti -f ./playground/infra/elasti-demo-values.yaml | kubectl apply -f -
```

If you want to enable monitoring, please make `enableMonitoring` true in the values file.

## 7. Deploy a demo service

Run a demo application in your cluster. We will use a sample httpbin service to demonstrate how to configure a service to handle its traffic via elasti.

```bash
kubectl create namespace target
kubectl apply -f ./playground/config/demo-application.yaml -n target
```

Add virtual service if you are using istio.

```bash
# ONLY IF YOU ARE USING ISTIO
# Create a Virtual Service to expose the demo service
kubectl apply -f ./playground/config/demo-virtualService.yaml -n target
```

This will deploy a httpbin service in the `demo` namespace.

## 8. Create ElastiService Resource

Using the [ElastiService Definition](/src/gs-configure-elastiservice/), create a manifest file for your service and apply it. For demo, we use the below manifest.

```bash
kubectl -n target apply -f ./playground/config/demo-elastiService.yaml
```


## 9. Test the service

### 9.1 Scale down the service

```bash
kubectl -n target scale deployment httpbin --replicas=0
```

### 9.2 Send request to the service while target is scaled down

```bash
curl -v http://localhost:8080/httpbin

# kubectl run -it --rm curl --image=alpine/curl -- http://httpbin.target.svc.cluster.local/headers
```

You should see the target service pod getting scaled up and response from the new pod.
