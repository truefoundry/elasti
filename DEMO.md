## Commands

```
Get Logs of Target
Get Logs of Load generator 
Get Logs of Activator
Get Logs of Controller 
Tarminal to apply changes
```

## Start 



// curl --location 'http://elasti-operator-controller-manager-metrics-service.elasti-operator-system.svc.cluster.local:8080/request-count' --header 'Content-Type: application/json' --data '{	"count": 1, "svc": "target-service" , "namespace": "default"}'
