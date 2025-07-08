# Elasti Architecture


Elasti comprises of two main components: operator and resolver.

- **Controller**: A Kubernetes controller built using kubebuilder. It monitors ElastiService resources and scaled them to 0 or 1 as needed.
- **Resolver**: A service that intercepts incoming requests for scaled-down services, queues them, and notifies the elasti-controller to scale up the target service.

## Flow Description


 ![Image title](../images/architecture/flow.png){ loading=lazy align=left width="400" }

When we enable Elasti on a service, the service operates in 3 modes:


1. **Steady State**: The service is receiving traffic and doesn't need to be scaled down to 0.
2. **Scale Down to 0**: The service hasn't received any traffic for the configured duration and can be scaled down to 0.
3. **Scale up from 0**: The service receives traffic again and can be scaled up to the configured minTargetReplicas.

<br><br>

### Steady state flow of requests to service

In this mode, all the requests are handled directly by the service pods. The Elasti resolved doesn't come into the picture. Elasti controller keeps polling prometheus with the configured query and check the result with threshold value to see if the service can be scaled down.

<div align="center">
<img src="../images/architecture/1.png">
</div>

### Scale down to 0 when there are no requests

If the query from prometheus returns a value less than the threshold, Elasti will scale down the service to 0. Before it scales to 0, it redirects the requests to be forwarded to the Elasti resolver and then modified the Rollout/deployment to have 0 replicas. It also then pauses Keda (if Keda is being used) to prevent it from scaling the service up since Keda is configured with minReplicas as 1. 

<div align="center">
<img src="../images/architecture/2.png">
</div>

### Scale up from 0 when the first request arrives.

Since the service is scaled down to 0, all requests will hit the Elasti resolver. When the first request arrives, Elasti will scale up the service to the configured minTargetReplicas. It then resumes Keda to continue autoscaling in case there is a sudden burst of requests. It also changes the service to point to the actual service pods once the pod is up. The requests which came to ElastiResolver are retried till 5 mins and the response is sent back to the client. If the pod takes more than 5 mins to come up, the request is dropped.

<div align="center">
<img src="../images/architecture/3.png">
</div>


<div align="center">
<img src="../images/architecture/4.png">
</div>