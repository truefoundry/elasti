#!/bin/sh

# NOTE: We are taking the argument as the path to the manifest directory.
# This is needed as the script is called from multiple places and has different paths.
# Maybe in future we can fix this, but for now this is fine.

# Apply ElastiService
kubectl apply -f  "$1/target-elastiservice.yaml" -n target

# Reset target deployment, service & ingress back to defaults
kubectl replace -f "$1/target-deployment.yaml" -n target

# Reset keda ScaledObject
kubectl replace -f "$1/keda-scaledObject-Target.yaml" -n target

# Scale elasti-resolver back to 1
kubectl scale deployment elasti-resolver -n elasti --replicas=1
kubectl wait pods -l app=elasti-resolver -n elasti --for=condition=Ready --timeout=120s

# Wait for resources to be ready
kubectl get elastiservice -n target target-elastiservice || exit 1
