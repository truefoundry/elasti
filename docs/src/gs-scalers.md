---
title: Scalers
---


## Scaling with HPA
KubeElasti works seamlessly with the Horizontal Pod Autoscaler (HPA) and handles scaling to zero on its own. Since KubeElasti manages the scale-to-zero functionality, you can configure HPA to handle scaling based on metrics for any number of replicas **greater than zero**, while KubeElasti takes care of scaling to/from zero.

A setup is explained in the [getting started](getting-started.md) guide.


## Scaling with KEDA
KubeElasti takes care of scaling up and down a service when there is some traffic. KEDA is a good candidate for performing the scaling logic for the service from minReplicas to maxReplicas based on its triggers.

Here we will see how to integrate KubeElasti with KEDA to build a complete scaling solution.

## Prerequisites
- Make sure you have gone through the [getting started](getting-started.md) guide. We will extend the same setup for this integration.
- KEDA installed in the cluster - [KEDA Installation](https://keda.sh/docs/latest/deploy/)

## Steps

### 1. Create a keda scaler for the service

Let's create a keda scaler for the httpbin service.

``` shell
kubectl apply -f ./playground/config/demo-application-keda.yaml
```
Note that the same prometheus query is used as in the [getting started](getting-started.md) guide for ElastiService and the namespace is the same as the namespace that the ElastiService is created in.

Refer to the [keda documentation](https://keda.sh/docs/2.16/reference/scaledobject-spec/) for more details on configuring the ScaledObject.

### 2. Update ElastiService to work with the keda scaler

We will update the ElastiService to specify the keda scaler to work with. We will add the following fields to the ElastiService object:
```yaml
spec:
  autoscaler:
    name: httpbin-scaled-object
    type: keda
```

Patch the ElastiService object with the above changes.

``` shell
kubectl patch elastiservice httpbin-elasti -n elasti-demo -p '{"spec":{"autoscaler":{"name": "httpbin-scaled-object", "type": "keda"}}}' --type=merge
```

Now when KubeElasti scales down the service, it will pause the keda ScaledObject to prevent it from scaling up the service again, and when KubeElasti scales up the service, it will resume the ScaledObject.

With these changes, KubeElasti can reliably scale up the service when there is traffic and scale down the service to zero when there is no traffic while keda can handle the scaling logic for the service from minReplicas to maxReplicas based on its triggers.
