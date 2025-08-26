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
# Convert ISO 8601 timestamp to epoch seconds (portable across platforms)
iso_to_epoch() {
    local iso_date="$1"
    # Remove Z suffix and any fractional seconds
    iso_date=$(echo "$iso_date" | sed 's/\.[0-9]*Z$/Z/' | sed 's/Z$//')
    
    # Use python if available (most portable)
    if command -v python3 >/dev/null 2>&1; then
        python3 -c "import datetime; print(int(datetime.datetime.fromisoformat('$iso_date'.replace('Z', '+00:00')).timestamp()))"
    elif command -v gdate >/dev/null 2>&1; then
        gdate -d "$1" +%s
    else
        # macOS date fallback
        date -j -f "%Y-%m-%dT%H:%M:%SZ" "$1" +%s 2>/dev/null || date -j -f "%Y-%m-%dT%H:%M:%S" "${1%Z}" +%s
    fi
}

CRD_CREATION_SECONDS=$(iso_to_epoch "$CRD_CREATION_DATE")
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
