## 30th April 2024

- CRD fields -
  - Let's call it **ElastiService**
    - VirtualServices: []
    - ReplicaSets/ScaledObjects: []
        - Why replicaset?
            - We need to work with deployment and rollouts
        - Should we rather work with ScaledObject?
            - ScaledObject has a reference to scaleTargetRef. It might be possible to get to the pods from there.
    - Timeout to hold the request, Freq, Limit to check if the pods are ready
- Controller -
    - Exists as a single replica deployment in the cluster watching over all the services using a CRD
    - Scale down
        - Identifies the services that are being scaled to zero
        - Receive an event of being scaled to zero
            - What event to look for to know about scale to zero?
                - Traffic metrics
                - Maybe keda emits some event when a service is scaled down to zero
                - Look for a replicaset being scaled down to 0
                    - How can this be correlated to the services we are watching?
                        - Maybe the name of the replicaset owner (Deployment) is available in the CRD we are watching
        - Manipulate the virtual service to point to the activator/proxy and add a header to incoming requests of the `elastiService` (this needs to be done using weights, the service will continue to be there in the elastiService)
            - Is there a chance that the deployment has been scaled down and the switch has not been made yet?
                - There is a chance, although unlikely, since killing the pod is going to take longer than manipulating virtual service and configuration being applied
                    - If controller is down, this will definitely happen
                        - controller init needs to take care of reconciling the vs
                - Can we somehow do this before the scale to zero event comes along?
                    - We might be able to, by replicating the functionality of keda and keeping the timeout a little < than keda. Does not justify the complexity yet
    - Scale up
        - Receives a callback from activator that a request has arrived for an `elastiService` that had been scaled down to zero
        - Watch for the target service to have scaled up
            - How does scale up happen:
                - Option 1:
                    - Scale the replicaset to 1
                    - KEDA reacts to finished requests by scaling to as much as needed
                - Option 2:
                    - Expose one of the endpoints on activator as scalar to signal to keda to scale up
            - How do you watch for the target to have scaled up?
                - We watch for the pods to get ready. This would need the replicaset again, which can only come from CRD
            - Do we watch for all the pods to become available?
                - Lets say yes. Has to match the number is replicaset. We are not worried about autoscaling for now
        - Forward traffic to the target service
            - We just change the virtual service from activator to the actual service
                - How does controller know which service is the actual one to return to?
        - receives the request (service name) → waiting for the service to available → service →
        - CRD →
            - Not scaled to 0
            - Scaling to 0
            - Scaled to 0
            - Scaling back to non-0
            - Not scaled to 0
- Activator
    - A reverse proxy
    - Scale down
        - At some point controller switches traffic for a service to the activator
    - Scale up
        - Activator receives a request for a service
        - Activator accepts the request and pushes in a queue
            - What is the structure of the queue?
                - Per ElastiService
                    - How does activator get to know which service a request belongs to?
                        - [https://www.notion.so/Clear-out-elasti-architecture-10dbd6b5368748f88556892206e13910?pvs=4#e3413d5f17cd430d977b2394dc111313](https://www.notion.so/10dbd6b5368748f88556892206e13910?pvs=21)
        - Activator has a goroutine running which keeps retrying the target service until success
            - Which target service to try?
                - Can be fetched from the elastiService object.
                    - Activator needs to be able to read the elastiService object from the cluster
            - Till when, with what frequency to try
                - Same, get from elastiService
        - Activator calls the controller in another goroutine notifying that a particular elastiService needs to be scaled up
        - How does activator know that the request has come for a service that has been scaled down to 0
            - That request will contain the header of `elasti-service`. If not, the request is immediately rejected

Notes -

- Scaling up or down is completely up to keda
- Should activator be CRD aware?
    - Advantages -
        - Can create separate queues per ElastiService
        - The callback to controller can contain the specific ElastiService for which the request has come in
    - Disadvantages -
        - Additional complexity in the component which should be limited to being a reverse proxy
- How do we lookup which ElastiService to enable for a particular request?
    - Request contains, host, path, port etc. All the match criteria are valid
    - How do we match the specific request to which ElastiService that relates to?
        - Lookup into all the virtual services?
    - Eg -
        - VS -
            - name: example-vs
            - host: https://example.com
            - port: 8080
            - target: svc-example <> activator
            - header:
                
                ```yaml
                headers:
                	request:
                		add:
                			elasti-service: example-vs
                ```
                
        - ElastiService
            - name: svc-example
            - virtualServices: [example-vs]
    - This header can be read by the activator and the appropriate elasti service can be sent to the controller
- How does east-west traffic work?