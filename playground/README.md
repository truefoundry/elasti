# Playground Setup

We will set up a playground, to do POCs, Development, and Testing of Elasti. 

# Pre-Requisites 

You need to be familiar with the following technologies to understand and work with this project. 

- Golang
- Docker 
- Make
- Kubernetes
- Keda
- Istio


# Setup

## Setup Golang and Docker

You can use their official documentation to setup them for your OS. Here are the links to their docs. 

- [Docker Docs](https://www.docker.com/)
- [Golang Docs](https://go.dev/)

## Setup Kubernetes 

We are using Kubernetes supported by Docker Desktop, but you are free to use other tools like Kind, Minikube etc for this. 

| The following steps assume that you have setup a Kubernetes cluster, and Kubectl is configured to point to this cluster.

## Setup Keda

 
### Add and install via Helm chart
```
helm repo add kedacore https://kedacore.github.io/chart
helm repo update
helm install keda kedacore/keda --namespace keda --create-namespace --values infra-values/keda-values.yaml
```

## Setup Istio

### Install istioctl
```bash
curl -L https://istio.io/downloadIstio | sh -

// Move it to home directory
mv istio-x.xx.x ~/.istioctl

// Add below line to your
export PATH=$HOME/.istioctl/bin:$PATH
```
| For Linux and macOS

### Install Istio via Istioctl
```bash
// Add default namespace to istio injection
kubectl label namespace default istio-injection=enabled

// We are installing the demo profile for testing
istioctl install --set profile=demo -y
```

## Setup Demo Application

For this, we will use the demo applications provided via Istio to demonstrate its features. 
### Git clone istio repo in the playground folder. 
```bash
git clone https://github.com/istio/istio 
```

### Apply YAML files
```bash
// Create a istio-demo namespace and use that to deploy below services
kubectl create namespace istio-demo
kubectl label namespace istio-demo istio-injection=enabled

// This creates the required services, deployments etc
kubectl apply -f istio/samples/bookinfo/platform/kube/bookinfo.yaml -n istio-demo

// This will expose the service to the outside traffic.
kubectl apply -f istio/samples/bookinfo/networking/bookinfo-gateway.yaml -n istio-demo

// This is to ensure if no issues are present in the istio proxies 
istioctl analyze

// If all is good, you should be able to access the demo here
http://localhost/productpage
```

## Build and Publish Activator

You can use the Make command to build and publish Activator in the Activator folder.

1. If you are using Docker Desktop, you will need to change your context to use the docker engine use by Docker Desktop.
```bash
$ make docker-context
```

2. We need a local registry to publish our build and pull them in Kubernetes. 
```bash
$ make docker-registry
```

3. Build the docker images.
```bash
$ make docker-build
```

4. Publish images to the local registry.
```bash
$ make docker-publish
```

You will need to repeat the 3rd and 4th steps when making any changes in the activator and need to test it inside k8s.

## Deploy Activator

Apply activator yaml in elasti namespace.
```bash
$ kubectl create namespace elasti
$ kubectl label namespace elasti istio-injection=enabled

$ kubectl apply -f activator.yaml -n elasti
```

## Expose your Activator 

We will apply gateway.yaml, which will create a gateway and virtualService for activator. 
```bash
$ kubectl apply -f gateway.yaml -n elasti

```

If you have followed so far, you should be able to access the demo and activator based on the endpoints exposed in the virtualService.
- http://localhost/ping
- http://localhost/productpage


## Setup Istio Addons for dashboard access

#### Apply Addons
```bash
kubectl apply -f istio/samples/addons

// Access dashboard
istioctl dashboard kiali
```

This will open the kiali dashboard with following traffic graph.
![kiali](../assets/kiali-first-map.png)

## Working with Controller

- Go version, 1.21. 
- There is bug in 1.22, so if you have that, please downgrade. 

- Install steps: [Get Operator SDK](https://sdk.operatorframework.io/docs/installation/#install-from-github-release)

- Step to initiate operator
```bash

mkdir elasti-operator
cd elasti-operator

operator-sdk init --domain=elasti.truefoundry.com --repo=github.com/truefoundry/elasti-operator
 operator-sdk create api --group=core --version=v1alpha1 --kind=Pod --resource --controller --verbose  

// Make your changes

// Build and Publish
make docker-build docker-push IMG="example.com/memcached-operator:v0.0.1"
make deploy IMG="localhost:5000/elasti-operator:latest"   

// Run Locally
make install run
```

https://github.com/operator-framework/operator-sdk/issues/1335


## Define Custom Resources

We will need to define some custom resources, so we can apply CRDs and work with our controller. 

The CRs defined in controller-cr folder, apply them. 

## Add cluster-admin to activator
```
 k create clusterrolebinding default-admin --clusterrole=cluster-admin --serviceaccount=elasti:default    

```

## Run Fake API for testing

```
docker run -d -p 1090:1090 --name fake-api reachfive/fake-api-server:latest 
```


