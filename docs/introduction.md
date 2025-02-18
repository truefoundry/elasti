# Introduction

Elasti is a Kubernetes-native solution that offers true scale-to-zero functionality while ensuring zero request loss. By dynamically managing service replicas based on real-time traffic conditions, Elasti dramatically reduces resource usage during idle periods without compromising the responsiveness or reliability of your applications.

## How It Works

Elasti continuously monitors an ElastiService by evaluating a set of custom triggers defined in its configuration. These triggers represent various conditions—such as traffic metrics or other custom signals—that determine whether a service should be active or scaled down.

- **Scaling Down:**  
  When **all** triggers indicate inactivity or low demand, Elasti scales the target service down to 0 replicas. During this period, elasti switches into **proxy mode** and queues incoming requests instead of dropping them.

- **Traffic Queueing in Proxy Mode:**  
  In Proxy Mode, Elasti intercepts and queues incoming requests directed at the scaled-down service. This ensures that no request is lost, even when the service is scaled down to 0.

- **Scaling Up:**  
  If **any** trigger signals a need for activity, Elasti immediately scales the service back up to its minimum replicas. As the service comes online, Elasti switches to **Serve Mode**.

- **Serve Mode:**  
  In Serve Mode, the active service handles all incoming traffic directly. Meanwhile, any queued requests accumulated during Proxy Mode are processed, ensuring a seamless return to full operational capacity.

This allows Elasti to optimize resource consumption by scaling services down when unneeded, while its request queueing mechanism preserves user interactions and guarantees prompt service availability when conditions change.
<div align="center">
<img src="./docs/assets/modes.png" width="400px">
</div>

## Key Features

- **Seamless Integration:** Elasti integrates effortlessly with your existing Kubernetes setup. It takes just a few steps to enable scale to zero for any service.

- **Development and Argo Rollouts Support:** Elasti supports two target references: Deployment and Argo Rollouts, making it versatile for various deployment scenarios.

- **HTTP API Support:** Currently, Elasti supports only HTTP API types, ensuring straightforward and efficient handling of web traffic.

- **Prometheus Metrics Export:** Elasti exports Prometheus metrics for easy out-of-the-box monitoring. You can also import a pre-built dashboard into Grafana for comprehensive visualization.

- **Istio Support:** Elasti is compatible with Istio. It also supports East-West traffic using cluster-local service DNS, ensuring robust and flexible traffic management across your services.


# Why use Elasti?

Kubernetes clusters can become costly, especially when running multiple services continuously. Elasti addresses this issue by giving you the confidence to scale down services during periods of low or no traffic, as it can bring them back up when demand increases. This optimization minimizes resource usage without compromising on service availability. Additionally, Elasti ensures reliability by acting as a proxy that queues incoming requests for scaled-down services. Once these services are reactivated, Elasti processes the queued requests, so that no request is lost. This combination of cost savings and dependable performance makes Elasti an invaluable tool for efficient Kubernetes service management.

> The name Elasti comes from a superhero "Elasti-Girl" from DC Comics. Her superpower is to expand or shrink her body at will—from hundreds of feet tall to mere inches in height.

<div align="center"> <b> Demo </b></div>
<div align="center">
    <a href="https://www.loom.com/share/6dae33a27a5847f081f7381f8d9510e6">
      <img style="max-width:640px;" src="https://cdn.loom.com/sessions/thumbnails/6dae33a27a5847f081f7381f8d9510e6-adf9e85a899f85fd-full-play.gif">
    </a>
  </div>
