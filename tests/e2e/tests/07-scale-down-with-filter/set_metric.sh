#!/bin/sh

COMMAND="echo '${1} ${2}' | curl --data-binary @- http://prometheus-pushgateway.monitoring.svc.cluster.local:9091/metrics/job/some_job"

kubectl exec -n target curl-target-gw -- sh -c "${COMMAND}"
