# Challengs

- resolver Switching Traffic using Knative Code
- Identify services based on the label using custom CRDs
- resolver scaling 0 -> 1
- KEDA and resolver Communication

### Scaling 0 to N can happen from keda? resolver doesn't need to do that
- We are using resolver to make sure 0 to 1 goes through faster

### A controller will be separate from the resolver and will act to switch traffic
- Why?
  - Will be difficult to scale to multiple replicas
- We can achieve certain single-threaded functionality via locks etc in resolver now, but will eventually separate it out in a controller. - RT
- This is decided only to save time of development.  - RT

### What happens for east-west traffic?
- Open question?
- We will answer this as we go, however, the first solution which pops is to identify the outside and internal traffic, and handle them separately. -RT 

### Instead of doing a label based thing, can the controller act whenever a service is scaled down to 0? There is nothing we save from excluding few of the services
- We want to select a subset. We can use * to select all the service
- "?

| Switching should happen a bit before the service is scaled to 0 - probably just when the pod is getting the terminating signal - and we have the 30 seconds drain period from Kubernetes. 

### What is the cron in CRDs? The informer takes care of letting the controller know of changes automatically
- No cron in CRDs, we will depend on the informer only, but a handler will be introduced for those changes. Cron was used to represent the handler, I will correct that. - RT 

### How does the resolver scale? What should be the scaling criteria?
- Will be covered Testing phase
- We will decide the criteria as we go, it can be memory or CPU, or open connections, or queue size etc. - RT



# Components

## resolver
 
- Read labels to identify watched services.
- Get the Destination to those services, to watch the traffic to those routes. 
- Track request count to those destinations.
  - If 0 req -> Request KEDA to scale deployment to 0.
  - If req > 0 but < N -> Scale service directly from deployment 0 to 1.
  - If req > N -> Request KEDA to scale deployment N1.
- Handle CRDs to get the latest configurations.
  - How long to wait for requests to remain 0.
  - How many instances to create when a request comes in after a long time.
  - How long to hold the request in the resolver, timeout. 
- Create a queue for the requests. 
- APIs to take the incoming request, and the kind of request we can take in. 
- Switch the destination in VirtualService to the point between resolver and the actual service. Utilise Knative code to make the resolver behave like a proxy. 


| We can later decide to move out some responsibilities from resolver to Controller, like watching the traffic and switching the traffic.  

## CRDs

- Create ScaledObjects CRD to register the resolver as a Scalar.
- Automatically configure ScaledJobs CRD to match the resolver config, like the number of pods etc, which will overwrite the KEDA configuration for that service. 
- CRDs to configure Elasti, and identify deployments, and their rules. 

## Traffic Monitoring

![traffic monitoring](./assets/traffic-monitoring.png)


# Resources

- [Write controller for pod labels](https://kubernetes.io/blog/2021/06/21/writing-a-controller-for-pod-labels/)
- [K8s Operator Pattern](https://iximiuz.com/en/posts/kubernetes-operator-pattern/)

---

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

## Build and Publish resolver

You can use the Make command to build and publish resolver in the resolver folder.

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

You will need to repeat the 3rd and 4th steps when making any changes in the resolver and need to test it inside k8s.

## Deploy resolver

Apply resolver yaml in elasti namespace.
```bash
$ kubectl create namespace elasti
$ kubectl label namespace elasti istio-injection=enabled

$ kubectl apply -f playground.yaml -n elasti
```

## Expose your resolver 

We will apply gateway.yaml, which will create a gateway and virtualService for resolver. 
```bash
$ kubectl apply -f gateway.yaml -n elasti

```

If you have followed so far, you should be able to access the demo and resolver based on the endpoints exposed in the virtualService.
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

elasti-operator-sdk init --domain=elasti.truefoundry.com --repo=github.com/truefoundry/elasti-operator
 elasti-operator-sdk create api --group=core --version=v1alpha1 --kind=Pod --resource --controller --verbose  

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

## Add cluster-admin to resolver
```
 k create clusterrolebinding default-admin --clusterrole=cluster-admin --serviceaccount=elasti:default    

 k create clusterrolebinding elasti-operator-admin --clusterrole=cluster-admin --serviceaccount=elasti-operator-system:elasti-operator-controller-manager


```

## Run Fake API for testing

```
docker run -d -p 1090:1090 --name fake-api reachfive/fake-api-server:latest 
```

## resolver needs cluster-admin permission

```
k create clusterrolebinding default-admin --clusterrole=cluster-admin --serviceaccount=elasti:default
```
