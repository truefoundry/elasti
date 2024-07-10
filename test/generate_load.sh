#!/bin/sh

k6 run --vus 5 --duration 30s  ~/Workspace/go/src/github.com/truefoundry/elasti/test/load.js > k6_logs.log

# -o experimental-prometheus-rw-o experimental-prometheus-rw
