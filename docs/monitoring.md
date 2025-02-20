## Monitoring

Set `.global.enableMonitoring` to `true` in the [values.yaml](./charts/elasti/values.yaml) file to enable monitoring.

This will create two ServiceMonitor custom resources to enable Prometheus to discover the Elasti components. To verify this, you can open your Prometheus interface and search for metrics prefixed with `elasti_`, or navigate to the Targets section to check if Elasti is listed.

Once verification is complete, you can use the [provided Grafana dashboard](./playground/infra/elasti-dashboard.yaml) to monitor the internal metrics and performance of Elasti.

<div align="center">
<img src="./docs/assets/grafana-dashboard.png" width="800px">
</div>