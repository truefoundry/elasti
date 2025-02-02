# Integration with KEDA
Elasti takes care of scaling up a service when there is some traffic. We need another component to scale down the service when there is no traffic. KEDA is a good candidate for this.
Here we will see how to integrate Elasti with KEDA to perform a complete scaling solution.

## Prerequisites
- Istio installed in the cluster - [Istio Installation](https://istio.io/latest/docs/setup/getting-started/)
- KEDA installed in the cluster - [KEDA Installation](https://keda.sh/docs/latest/deploy/)
- Elasti installed in the cluster - [Elasti Installation](https://github.com/truefoundry/elasti)
- Prometheus installed in the cluster - [Prometheus Installation](https://prometheus.io/docs/prometheus/latest/installation/)
- A service to scale

## Starting setup
Lets start with a setup where scaling down to zero is performed by KEDA for a service deployed behind istio.

![Starting setup](./assets/keda-scale-down.png)

- traffic is being routed to the service via istio
- keda queries prometheus for the number of requests to the service. The query we are using is
  ```
  sum(rate(istio_requests_total{destination_service="httpbin.demo.svc.cluster.local"}[1m]))
  ```
- keda scales down the service to zero replicas when there are no requests to the service

The keda scaler yaml for such a setup is as follows:

```yaml
apiVersion: keda.sh/v1alpha1
kind: ScaledObject
metadata:
  name: <service-name>-scaled-object
spec:
  cooldownPeriod: 60 # this is how long keda will wait before scaling down the service to 0 replicas
  idleReplicaCount: 0
  maxReplicaCount: 1
  minReplicaCount: 1
  pollingInterval: 30
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: <service-name>
  triggers:
  - metadata:
      query: sum(rate(istio_requests_total{destination_service="<service-name>.demo.svc.cluster.local"}[1m]))
      serverAddress: http://<prometheus-instance-url>:9090
      threshold: "0.01" # this is the threshold for the number of requests to the service
    type: prometheus
```

## Add Elasti to the service
We will create an ElastiService object for this service, update the keda scaler to query the Elasti metrics and add a scrapeconfig to the prometheus instance to scrape the Elasti metrics. The final setup will look like this:

![Final Setup](./assets/keda-with-elasti.png)

Here is the flow of requests:
- first request on istio will go to resolver
- controller will scale up the service to 1 replica
- resolver will forward the request to the pods for further processing

The complexity arises from the fact that controller and keda are both watching the same service. We need to make sure that keda does not scale down the service to 0 replicas when controller is scaling up the service.

To achieve this, we will have to do two things 
1. Add an additional trigger to the keda scaler to query the Elasti metrics. The keda scaler yaml for such a setup is as follows:
  ```yaml
  apiVersion: keda.sh/v1alpha1
  kind: ScaledObject
  metadata:
    name: <service-name>-scaled-object
  spec:
    triggers:
    - metadata:
        query: elasti_resolver_queued_count{namespace="<namespace>", source="<service-name>"}
          > 0
        serverAddress: http://<prometheus-instance-url>:9090
        threshold: "1" # this threshold makes sure that keda does not scale down while resolver is still processing the requests
      type: prometheus
  ```

2. Add a scrapeconfig to the prometheus instance to scrape the Elasti metrics. The scrapeconfig yaml for such a setup is as follows:
  ```yaml
  - scheme: http
    job_name: elasti-resolver
    honor_labels: true
    metrics_path: /metrics
    dns_sd_configs:
      - port: 8013
        type: A
        names:
          - elasti-resolver-service.elasti
    # the `scrape_interval` has to be less than the polling interval of keda. the lesser the interval, the more up to date the metrics will be
    scrape_interval: 1s
  ```

With these changes, elasti can reliably scale up the service when there is traffic and scale down the service to zero when there is no traffic.