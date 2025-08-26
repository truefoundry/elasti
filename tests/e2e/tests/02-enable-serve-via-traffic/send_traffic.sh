#!/bin/sh
for i in 1 2 3 4 5; do
    code=$(kubectl exec -n target curl-target-gw -- curl --max-time 30 -s -o /dev/null -w "%{http_code}" "${1}")
    result=$?

    echo "$result / $code"

    if [ "$result" != "0" ]; then exit 1; fi
    if [ "$code" != "200" ]; then exit 2; fi

    sleep 1
done
