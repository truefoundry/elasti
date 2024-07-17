#!/bin/sh
echo "k6 test started..."

export K6_WEB_DASHBOARD=true 
export K6_WEB_DASHBOARD_PORT=5665
# -o experimental-prometheus-rw

K6_WEB_DASHBOARD_EXPORT=logs/report.html k6 run --vus 10 --duration 60s  ~/Workspace/go/src/github.com/truefoundry/elasti/test/load.js > logs/k6_logs.log & 
PID1=$!

wait $PID1

echo "k6 test have completed. Output in logs folder."
read -p "Press Enter to exit"
