#!/bin/sh

# Apply ElastiService
kubectl apply -f  $1/target-elastiservice.yaml -n target

# Scale target-deployment back to 1
kubectl scale deployment target-deployment -n target --replicas=1

# Reset the service selector and port
kubectl patch service target-deployment -n target --type=merge -p '{"spec":{"selector":{"app":"target-deployment"},"ports":[{"port":80,"targetPort":8080}]}}'

# Scale elasti-resolver back to 1
kubectl scale deployment elasti-resolver -n elasti --replicas=1

# Wait for resources to be ready
kubectl wait pods -l app=target-deployment -n target --for=condition=Ready --timeout=30s
kubectl wait pods -l app=elasti-resolver -n elasti --for=condition=Ready --timeout=30s
kubectl get elastiservice -n target target-elastiservice || exit 1
