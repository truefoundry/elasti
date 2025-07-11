# Playground

## 1. Local Cluster

If you don't already have a local Kubernetes cluster, you can set one up using Minikube, Kind or Docker-Desktop:

=== "Kind"

    ``` bash
    kind create cluster
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
docker run -d -p 5001:5000 --name registry registry:2
```

!!! tip "Add registry to Minikube and Kind"
    You will need to add this registry to Minikube and Kind. With Docker-Desktop, it is automatically picked up if running in same context.

!!! tip "Note"
    In MacOS, 5000 is not available, so we use 5001 instead.


## 3. [Optional] Install Ingress Controller

=== "NGINX"

      Install the NGINX Ingress Controller using Helm:
      ```bash
      helm repo add ingress-nginx https://kubernetes.github.io/ingress-nginx
      helm repo update
      kubectl create namespace nginx
      helm install ingress-nginx ingress-nginx/ingress-nginx -n nginx
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
      kubectl create namespace demo
      kubectl label namespace demo istio-injection=enabled

      # Create a gateway
      kubectl apply -f ./playground/config/gateway.yaml
      ```

## 4. Deploy a demo service

Run a demo application in your cluster.

```bash
kubectl create namespace demo
kubectl apply -f ./playground/config/demo-application.yaml -n demo

# Create a Virtual Service to expose the demo service if you are using istio
kubectl apply -f ./playground/config/demo-virtualService.yaml -n demo
```

## 5. Build & Publish Resolver

Go into the resolver directory and run the build and publish command.

```bash
cd resolver
make docker-build docker-push IMG=localhost:5001/elasti-resolver:v1alpha1
```

## 6. Build & Publish Operator

Go into the operator directory and run the build and publish command.

```bash
cd operator
make docker-build docker-push IMG=localhost:5001/elasti-operator:v1alpha1
```

## 7. Deploy Locally

Make sure you have configured the local context in kubectl. We will be using [`playground/infra/elasti-demo-values.yaml`](https://github.com/truefoundry/KubeElasti/blob/main/playground/infra/elasti-demo-values.yaml) for the helm installation. Configure the image uri according to the requirement. Post that follow below steps from the project home directory:

```bash
kubectl create namespace elasti
helm template elasti ./charts/elasti -n elasti -f ./playground/infra/elasti-demo-values.yaml | kubectl apply -f -
```

If you want to enable monitoring, please make `enableMonitoring` true in the values file.

## 8. Create ElastiService Resource

Using the [ElastiService Definition](./configure-elastiservice.md#configure-elastiservice), create a manifest file for your service and apply it. For demo, we use the below manifest.

```bash
kubectl -n demo apply -f ./playground/config/demo-elastiService.yaml
```

## 9. Test the service

### 9.1 Create a watch on the service

```bash
kubectl -n demo get elastiservice httpbin -w
```

### 9.2 Scale down the service

```bash
kubectl -n demo scale deployment httpbin --replicas=0
```

### 9.3 Create a load on the service

```bash
kubectl run -it --rm curl --image=alpine/curl -- http://httpbin.demo.svc.cluster.local/headers
```

You should see the target service pod getting scaled up and response from the new pod.
