#!/bin/sh

# NOTE: We are taking the argument as the path to the manifest directory.
# This is needed as the script is called from multiple places and has different paths.
# Maybe in future we can fix this, but for now this is fine.

# Apply ElastiService
kubectl apply -f  "$1/target-elastiservice.yaml" -n target

# Wait for ElastiService to be old enough
CRD_CONTENT=$(kubectl get elastiservices/target-elastiservice -n target -o json)
CRD_COOLDOWN_PERIOD=$(printf "%s" "$CRD_CONTENT" | jq -r '.spec.cooldownPeriod')
CRD_CREATION_DATE=$(printf "%s" "$CRD_CONTENT" | jq -r '.metadata.creationTimestamp')
CRD_CREATION_SECONDS=$(date -d "$CRD_CREATION_DATE" +%s)
SECONDS_NOW=$(date +%s)
CRD_AGE=$(($SECONDS_NOW - $CRD_CREATION_SECONDS))
if [ "$CRD_AGE" -lt "$CRD_COOLDOWN_PERIOD" ]; then
    sleep $(($CRD_COOLDOWN_PERIOD - $CRD_AGE))
fi

# Reset target deployment, service & ingress back to defaults
kubectl replace -f "$1/target-deployment.yaml" -n target

# Reset keda ScaledObject
kubectl replace -f "$1/keda-scaledObject-Target.yaml" -n target

# Scale elasti-resolver back to 1
kubectl scale deployment elasti-resolver -n elasti --replicas=1
kubectl wait pods -l app=elasti-resolver -n elasti --for=condition=Ready --timeout=120s

# Wait for resources to be ready
kubectl get elastiservice -n target target-elastiservice || exit 1
