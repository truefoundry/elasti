# Triggers

## Trigger with Prometheus

ElastiService uses Prometheus queries to determine when to scale down services to zero. This section provides guidance on how to create effective queries for your ElastiService configuration.

### Finding the Right Prometheus Query

1. **Identify the Metric**: First, determine which metric best represents your service activity. Common metrics include:
      - HTTP request rates (`requests_total`, `request_count`)
      - Connection counts
      - Processing rates
      - Custom application metrics

2. **Access Prometheus UI**: 
      - Connect to your Prometheus instance (typically available at `http://<prometheus-service>.<namespace>.svc.cluster.local:9090`)
      - For local development, you may need to port-forward: `kubectl port-forward svc/prometheus-server -n monitoring 9090:9090`

3. **Explore Available Metrics**:
      - In the Prometheus UI, go to the "Graph" tab
      - Click on the "Insert metric at cursor" dropdown to see available metrics
      - Filter by typing part of your service name

4. **Test Your Query**:
      - For HTTP request rates, a common pattern is: `sum(rate(http_requests_total{service="your-service"}[1m]))`
      - For Nginx Ingress: `sum(rate(nginx_ingress_controller_requests_total{service="your-service"}[1m]))`
      - Always add a fallback: `or vector(0)` to handle cases with no data

5. **Visualize and Refine**:
      - Execute the query in the Prometheus UI
      - Adjust the time range to see historical patterns
      - Refine labels and functions until you get the expected results

### Example Queries

=== "NGINX Ingress Controller Requests"

    ```promql
    sum(rate(nginx_ingress_controller_requests_total{namespace="your-namespace", ingress="your-ingress"}[1m])) or vector(0)
    ```

=== "Istio Request Count"

    ```promql
    sum(rate(istio_requests_total{destination_service_name="your-service"}[1m])) or vector(0)
    ```

=== "Custom Application Metric"

    ```promql
    sum(rate(app_metric_name{service="your-service"}[1m])) or vector(0)
    ```

### Setting the Threshold

The `threshold` value in your ElastiService configuration determines when scaling to zero occurs:

- If your query returns the request rate per second, a threshold of `0.5` means scale to zero when fewer than 0.5 requests per second occur
- For absolute counts, set the threshold accordingly (e.g., `5` for 5 total connections)
- Consider setting a non-zero threshold (like `0.1`) to provide a buffer before scaling down

### Complete Trigger Configuration Example

```yaml
triggers:
- type: prometheus
  metadata:
    query: sum(rate(nginx_ingress_controller_requests_total{namespace="your-namespace", service="your-service"}[1m])) or vector(0)
    serverAddress: http://kube-prometheus-stack-prometheus.monitoring.svc.cluster.local:9090
    threshold: 0.5
```

### Verifying Your Query

Before applying your ElastiService configuration:

1. Test the query in Prometheus UI during both active and inactive periods
2. Ensure it returns numeric values (not errors or empty results)
3. Verify the query captures all relevant traffic to your service
4. Check that the `or vector(0)` fallback works when there's no data

### Common Query Patterns

| Metric Source | Query Pattern |
|---------------|---------------|
| Nginx Ingress | `sum(rate(nginx_ingress_controller_requests_total{namespace="ns", ingress="name"}[1m])) or vector(0)` |
| Istio | `sum(rate(istio_requests_total{destination_service_name="name"}[1m])) or vector(0)` |
| Kubernetes API Server | `sum(apiserver_request_total{resource="your-resource"}) or vector(0)` |
| Custom App Metric | `sum(rate(app_metric_name{service="name"}[1m])) or vector(0)` |
