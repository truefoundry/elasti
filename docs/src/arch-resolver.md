
# Resolver Architecture


``` mermaid
flowchart LR
  %% ── USER & ENTRY ─────────────────────
  User(("Client")) --> RP["Proxy<br/>:8012"] --> Main["Main<br/>cmd/main.go"] --> IS["Metrics<br/>:8013"]

  %% ── K8s RESOURCES ────────────────────
  subgraph K8s["K8s"]
    Deploy["Deployment"] --> SVC["Service"]
    Deploy --> Mon["Monitoring<br/>configs"]
  end
  Main --> Deploy

  %% ── CORE MODULES ─────────────────────
  subgraph Mods["Core Modules"]
    Handler["Handler"]:::core
    Hosts["Hosts"]:::core
    Thr["Throttle"]:::core
    Oper["Operator Comm"]:::core
    Obs["Observability"]:::core
  end
  Main -- uses --> Handler & Hosts & Thr & Oper & Obs

  %% Request flow (compact arrows)
  Handler --> Hosts
  Handler --> Thr
  Thr --> Handler
  Handler --> Obs
  Handler -.-> Sentry["Sentry"]

  %% Operator comm
  Handler -.-> Oper
  Oper -.-> OpSvc["Operator Svc"]

  %% External deps
  Thr -.-> K8sAPI["K8s API"]
  Obs -.-> Prom["Prometheus"]

```

## 1. Major Containers and Deployment Components

### a. **Main Application Container**
- **Entry Point:** `cmd/main.go`
- **Responsibilities:**
    - Initializes configuration from environment variables.
    - Sets up external dependencies: Kubernetes client (`k8shelper`), operator RPC client, host manager, throttler, and transport.
    - Configures observability tools: Sentry, Prometheus metrics.
    - Starts two HTTP servers:
        - **Reverse Proxy Server** on port `:8012`:
            - Handles all external incoming requests.
            - Uses a custom handler (`handler.NewHandler`) that manages request flow, throttling, host resolution, and proxying.
        - **Internal Server** on port `:8013`:
            - Exposes metrics (`/metrics`) and internal status (`/queue-status`).

### b. **Kubernetes Resources**
- **Deployment:** `deployment.yaml`
- **Service:** `service.yaml`
- **Config & Monitoring:** `clusterRole.yaml`, `clusterRoleBinding.yaml`, `monitor.yaml`, `kustomization.yaml`
- These resources deploy the backend service in a Kubernetes cluster, with RBAC permissions, service exposure, and Prometheus monitoring.

---

## 2. Core Modules and Their Responsibilities

### a. **Request Handling & Proxying (`internal/handler`)**
- **Handler (`handler.go`):**
    - Acts as a reverse proxy, intercepting all incoming requests.
    - Resolves the target host based on request headers and internal host cache (`HostManager`).
    - Checks if traffic is allowed; if not, responds with 403.
    - Notifies the operator about incoming requests asynchronously.
    - Implements throttling via `throttler.Try()`.
    - Proxies requests to the target host using `httputil.ReverseProxy`.
    - Collects metrics for each request (latency, status, errors).

- **Request Proxying (`ProxyRequest`):**
    - Constructs a reverse proxy to the target host.
    - Uses a custom transport (`throttler.transport`) with connection pooling and HTTP/2 support.
    - Handles errors and panics gracefully.

- **Response Writer (`writer.go`):**
    - Wraps `http.ResponseWriter` to capture response status and body for metrics and error handling.

### b. **Host Management (`internal/hostmanager`)**
- **HostManager (`hostManager.go`):**
    - Maintains a cache (`sync.Map`) of host details (`messages.Host`).
    - Extracts namespace and service info from request headers or URL patterns.
    - Rewrites host URLs to internal/private service names.
    - Manages traffic enable/disable states with timers (`DisableTrafficForHost`, `enableTrafficForHost`).
    - Supports dynamic host resolution based on Kubernetes URL patterns.
    - Adds HTTP scheme if missing, removes wildcards or path suffixes.

### c. **Throttling & Request Control (`internal/throttler`)**
- **Throttler (`throttler.go`):**
    - Combines a `Breaker` (concurrency and queue limit) and a `Semaphore` (connection pooling).
    - Implements `Try()` method:
        - Checks service readiness via Kubernetes (`k8sUtil.CheckIfServiceEndpointActive`).
        - Manages request queue size.
        - Enforces concurrency limits.
        - Re-enqueues requests with retries and delays.
    - Tracks queue sizes and service readiness with `sync.Map`.
    - Uses `transport.go` for connection pooling with HTTP/2 and backoff dialing.

- **Breaker (`breaker.go`):**
    - Enforces maximum concurrency and queue depth.
    - Uses atomic counters and semaphores for thread-safe limits.
    - Fails fast if limits exceeded (`ErrRequestQueueFull`).

- **Semaphore (`semaphore.go`):**
    - Manages capacity for concurrent requests.
    - Supports dynamic capacity updates.
    - Uses channels and atomic operations for thread safety.

### d. **Operator Communication (`internal/operator`)**
- **RPC Client (`RPCClient.go`):**
    - Sends request count info to an external operator service.
    - Implements retry and mutex locking per service to avoid flooding.
    - Uses HTTP POST with JSON payload (`messages.RequestCount`).

### e. **Metrics & Observability (`internal/prom`)**
- Defines Prometheus metrics:
    - Host extraction counts (cache hits/misses, errors).
    - Queued request gauges.
    - Request latency histograms.
    - Traffic switch counters.
    - Operator RPC counters.
- Metrics are collected at various points:
    - Host resolution.
    - Request queuing.
    - Request handling duration.
    - Traffic switching events.
    - Operator RPC calls.

---

## 3. External Dependencies & Integrations

- **Kubernetes API (`k8shelper`)**:
    - Checks if service endpoints are active.
    - Used by throttler to determine service readiness.
- **Operator Service**:
    - RPC client communicates with operator at `http://elasti-operator-controller-service:8013`.
    - Sends incoming request info asynchronously.
- **Prometheus**:
    - Metrics endpoint exposed at `/metrics`.
    - Monitors host extraction, request queue, latency, traffic switches, and RPCs.
- **Sentry**:
    - Error tracking integrated via `sentry-go`.
    - Wraps HTTP handlers for request error reporting.
- **Transport Layer (`transport.go`)**:
    - Custom HTTP transport supporting connection pooling, HTTP/2, and backoff dialing.
    - Ensures efficient and resilient outbound connections.

---

## 4. Data & Control Flow

### a. **Request Lifecycle**
1. **Incoming Request**:
    - Received by the reverse proxy server (`main.go`).
    - Passed to `handler.ServeHTTP`.
2. **Host Resolution**:
    - `HostManager.GetHost` extracts host info from headers or URL patterns.
    - Checks cache; if miss, parses URL with regex patterns.
    - Constructs `messages.Host` with source/target service and host URLs.
3. **Traffic Check & Notification**:
    - If traffic is disabled for host, responds with 403.
    - Otherwise, asynchronously notifies operator of incoming request.
4. **Throttling & Queueing**:
    - Calls `throttler.Try()`:
        - Checks service readiness.
        - Enforces concurrency and queue limits.
    - Retries with delays if limits exceeded.
5. **Proxying**:
    - On success, `ProxyRequest` creates a reverse proxy to the target host.
    - Forwards the request, captures response.
6. **Metrics & Logging**:
    - Records latency, status, errors.
    - Updates Prometheus metrics.
7. **Response**:
    - Sends proxied response back to client.

### b. **Host Traffic Management**
- Hosts can be disabled temporarily (`DisableTrafficForHost`) and re-enabled after a timeout.
- Host info is cached and updated dynamically based on URL patterns and headers.

### c. **Operator Interaction**
- Sends minimal info about incoming requests asynchronously.
- Ensures no flooding via mutex locks per service.
- Handles failures gracefully with retries.

### d. **Metrics & Internal Monitoring**
- Exposes internal metrics and status endpoints.
- Tracks host extraction, request queue, latency, traffic switches, and RPC counts.
