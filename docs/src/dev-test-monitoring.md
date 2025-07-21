
# Test Monitoring

```bash
# First, add the prometheus-community Helm repository.
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm repo update

# Install the kube-prometheus-stack chart. This chart includes Prometheus and Grafana.
kubectl create namespace prometheus
helm install prometheus-stack prometheus-community/kube-prometheus-stack -n prometheus

# Port-forward to access the dashboard
kubectl port-forward -n prometheus services/prometheus-stack-grafana 3000:80

# Get the admin user.
kubectl get secret --namespace prometheus prometheus-stack-grafana -o jsonpath="{.data.admin-user}" | base64 --decode ; echo
# Get the admin password.
kubectl get secret --namespace prometheus prometheus-stack-grafana -o jsonpath="{.data.admin-password}" | base64 --decode ; echo
```

After this, you can use [`./playground/infra/elasti-dashboard.yaml`](https://github.com/truefoundry/KubeElasti/blob/main/playground/infra/elasti-dashboard.yaml) to import the KubeElasti dashboard.
