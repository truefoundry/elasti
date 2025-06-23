# KUTTL Test Cheat Sheet

A **KUTTL** test suite consists of **test cases**, each containing ordered **test steps** defined by YAML files. KUTTL applies resources, waits for asserted conditions, and reports failures. The cheat sheet below summarizes key concepts with examples and citations to the [official docs](https://kuttl.dev/docs).

## Test Steps (apply, delete, commands)

- **Step grouping**: In a test case directory, all files with the same numeric prefix (e.g. `00-foo.yaml`, `00-bar.yaml`) form one test step. Steps run in index order (00, 01, …). Files without a numeric prefix are ignored.
- **Applying resources**: By default, any Kubernetes manifest in a step is *applied*. If the resource does not exist it's created; if it exists, KUTTL merges in the changes (a strategic-merge patch). E.g. to scale a Deployment, a step file might contain just:
    
    ```yaml
    apiVersion: apps/v1
    kind: Deployment
    metadata:
      name: my-deployment
    spec:
      replicas: 4
    
    ```
    
    KUTTL will update only the replicas field, leaving other fields untouched.
    
- **Deleting resources**: You can delete objects *before* applying a step by including a `TestStep` object with a `delete:` list. Each entry specifies a resource (by `apiVersion`, `kind`, and optional `name`/`labels`). For example:
    
    ```yaml
    apiVersion: kuttl.dev/v1beta1
    kind: TestStep
    delete:
    - apiVersion: v1
      kind: Pod
      name: old-pod
    - apiVersion: v1
      kind: Pod
      labels:
        app: nginx
    
    ```
    
    This deletes the named Pod and any Pods with `app=nginx` in the test namespace. KUTTL waits for the deletions to complete (otherwise the step fails).
    
- **Running commands**: Use a `commands:` list in a `TestStep` to run shell commands before applying the rest of the step. Each entry can be a `command:` or `script:`. For example:
    
    ```yaml
    apiVersion: kuttl.dev/v1beta1
    kind: TestStep
    commands:
    - command: kubectl apply -f config.yaml
      namespaced: true
    - script: echo "Done!"
    
    ```
    
    Commands with `namespaced: true` automatically get `--namespace=<test-ns>` added. Inline shell scripts (with `script:`) are also allowed but ignore `namespaced`. Environment variables like `$NAMESPACE`, `$KUBECONFIG` are expanded inside commands.
    
- **Example TestStep**: Combining the above:
    
    ```yaml
    apiVersion: kuttl.dev/v1beta1
    kind: TestStep
    apply:
    - deployment.yaml
    - service.yaml
    delete:
    - apiVersion: v1
      kind: Pod
      name: cleanup-pod
    commands:
    - command: kubectl apply -f extra-config.yaml
    
    ```
    
    This step applies `deployment.yaml` and `service.yaml`, deletes the named Pod, and runs the extra `kubectl` command.
    

## Assertions (field assertions) and Errors

- **Assert files**: After resources are applied in a step, KUTTL looks for a corresponding assert file named `<index>-assert.yaml`. It contains Kubernetes objects whose specified fields define the desired state. Any field *not* given in the assert file is ignored during comparison. The harness waits (by default up to 30s) for the cluster to match all asserts.
- **By-name vs wildcard**: In an assert object, if `metadata.name` is set, KUTTL waits for *that specific object* to match the given state. If `name` is omitted, it treats the object as a wildcard: KUTTL will succeed when **any** object of that `kind` meets the criteria in the test namespace.
- **Field matching examples**: Common usage is to assert on status fields. For example, asserting a Pod is running:
    
    ```yaml
    apiVersion: v1
    kind: Pod
    metadata:
      name: my-pod
    status:
      phase: Running
    
    ```
    
    This waits until `status.phase == Running` on pod `my-pod`. Or, without a name:
    
    ```yaml
    apiVersion: v1
    kind: Pod
    status:
      phase: Running
    
    ```
    
    waits until **any** Pod has `phase=Running`. Similarly, to assert a Deployment is fully scaled:
    
    ```yaml
    apiVersion: apps/v1
    kind: Deployment
    metadata:
      name: example-deployment
    status:
      readyReplicas: 3
    
    ```
    
- **Error files**: Analogous to asserts, you can define `<index>-errors.yaml` with objects/states that *must NOT* occur. If any object in an errors file matches the cluster state, the step fails. For example:
    
    ```yaml
    apiVersion: v1
    kind: Pod
    metadata:
      name: bad-pod
    status:
      phase: Failed
    
    ```
    
    This would error if the Pod `bad-pod` ever has `status.phase=Failed`.
    
- **TestAssert object**: You can also include a `TestAssert` in your assert file to customize timeout or run diagnostics. For instance:
    
    ```yaml
    apiVersion: kuttl.dev/v1beta1
    kind: TestAssert
    timeout: 60
    collectors:
    - type: pod
      pod: my-pod
    
    ```
    
    Here the step will wait up to 60s instead of 30s, and if it fails, will collect logs from `my-pod`.
    

## Patching Objects

- **Strategic-merge patch**: KUTTL's default update mechanism is a strategic merge patch. Simply applying a partial YAML (as in the example above) updates only the fields you specify. For example, to change replicas:
    
    ```yaml
    apiVersion: apps/v1
    kind: Deployment
    metadata:
      name: my-deploy
    spec:
      replicas: 5
    
    ```
    
    KUTTL will merge this, leaving other fields (like labels, template) intact.
    
- **JSON Patch**: To use a JSON patch (or other patch types), you can run `kubectl patch` in a TestStep command. For example:
    
    ```yaml
    apiVersion: kuttl.dev/v1beta1
    kind: TestStep
    commands:
    - command: kubectl patch deployment my-deploy --type='json' -p='[{"op":"replace","path":"/spec/replicas","value":10}]'
    
    ```
    
    This command will JSON-patch the deployment. (KUTTL itself does not provide a special JSON patch field, but this `kubectl` invocation achieves it.)
    

## Directory Layout & Execution Order

- **Structure**: Organize your tests into *suites* and *cases*. A **test suite** directory (e.g. `tests/e2e/`) contains multiple test case subdirectories (e.g. `example-test/`). Each test case has step files (`00-*.yaml`, `01-*.yaml`, etc.). The KUTTL CLI (`kubectl kuttl test`) discovers test cases via the `testDirs` in `kuttl-test.yaml` or command-line argument.
- **Step files**: As mentioned, prefix files with a numeric index. All files starting with the same index form one step, applied in lexicographic order. For example, `00-deploy.yaml` and `00-service.yaml` form step 00. Steps execute sequentially; if any step fails, the test case fails.
- **Execution**: Test suites run test cases *in parallel* (up to `parallel` limit) and within each case steps run serially. By default, KUTTL creates a fresh namespace for each test case, runs all steps in it, then deletes it. Resources that explicitly set a namespace are respected.
- **Example**: From the docs, creating a suite and case:
    
    ```
    tests/e2e/
      example-test/
        00-install.yaml      # Step 00: create Deployment
        00-assert.yaml       # Assert replicas ready
        01-scale.yaml        # Step 01: scale Deployment
        01-assert.yaml       # Assert new replica count
    
    ```
    
    See [KUTTL Test Harness](https://kuttl.dev/docs/kuttl-test-harness.html) for a full example.
    

## `kuttl-test.yaml` Configuration

The suite-level configuration file (`kuttl-test.yaml`) defines **TestSuite** settings. Key fields include:

- `testDirs:` - list of paths containing test case directories. (E.g. `./tests/e2e/`.)
- `startKIND:` or `startControlPlane:` - Whether to spin up a Kind cluster or a mocked API server. (Set via CLI `-start-kind` or `-start-control-plane` or in this file.)
- `timeout:` - Default step timeout in seconds (default 30). Can be overridden per `TestAssert`.
- `namespace:` - If set, all tests use this existing namespace (KUTTL will not create/delete it). If unset, KUTTL auto-generates and cleans up a namespace per test.
- `parallel:` - Max concurrent test cases (default 8).
- `crdDir:` and `manifestDirs:` - Paths to pre-apply CRDs or other manifests *before* running tests. KUTTL waits for CRDs from `crdDir` to be established.
- `commands:` - Commands to run once before any tests (e.g. installing helm charts or controllers).
- `kindConfig`, `kindContext`, `kindNodeCache`, `kindContainers` - Kind-specific options (cluster config file, caching images, preloading images).
- `skipDelete`/`skipClusterDelete` - For debugging, prevents deleting resources or cluster after tests.
- `artifactsDir` - Where to write cluster logs after a test run.
- Example `kuttl-test.yaml` snippet:
    
    ```yaml
    apiVersion: kuttl.dev/v1beta1
    kind: TestSuite
    testDirs:
      - ./tests/e2e/
    startKIND: true
    timeout: 120
    
    ```
    
    (This is from KUTTL's examples.)
    

## Namespaces, Timeouts & Environments

- **Namespaces**: By default, KUTTL creates a new namespace for each test case and deletes it afterward. Resources in step files with no namespace go into this namespace. To reuse a namespace, set `-namespace=<name>` or `TestSuite.namespace`. In that *single-namespace mode*, KUTTL will not create or delete the namespace (it must already exist) and all tests share it.
- **Timeouts**: Each assert step times out after 30 seconds by default. You can override this globally via `TestSuite.timeout` or per-step via `TestAssert.timeout`. If a step's asserts aren't satisfied in time, the test fails.
- **Test environments**: KUTTL can run against different environments:
    - **Live cluster**: default uses `kubectl`'s current context (or specify `-kubeconfig`).
    - **Kind**: Use `-start-kind=true` (or `startKIND: true`) to launch a temporary [Kind](https://kind.sigs.k8s.io/) cluster. You can customize with `kindConfig`, `kindContext`, and preload images via `kindContainers`.
    - **Mocked Control Plane**: Use `-start-control-plane=true` (or `startControlPlane: true`) to run only an etcd + kube-apiserver without nodes. Use this for unit-test style controller integration tests (pods will not actually run). You can include your controller binary in the TestSuite `commands` (with `background: true`) to have it running during tests.
- **Cluster setup**: Use `crdDir`, `manifestDirs`, and `commands` in the TestSuite to install prerequisites. For example, put CRD YAMLs in `crdDir:` (they will be applied and awaited before tests). Use `commands:` to run any setup CLI commands (e.g. installing Helm's Tiller or starting a custom manager).

## Test Suite Structure & Best Practices

- **Step isolation**: Each test step should set up *just enough* state and assert on the outcome. Keep steps small (create one resource or change one thing per step) for clarity.
- **Use prefixes and docs**: Name step files with zero-padded indexes (`00-foo.yaml`) and use filenames (or sibling non-prefixed files) for documentation or additional config. Unprefixed files are ignored in execution.
- **CRDs and ordering**: If your test needs a CRD, put the CRD in `crdDir` or at step 00 and then **assert** its availability before using it. (Kubernetes may take a moment to register a CRD.) For example, step 00 creates the CRD and step 01 waits on its `.status.acceptedNames`.
- **Controllers**: To test custom controllers/operators, start them in the TestSuite (via `commands`) so they reconcile your test resources. Use `background: true` if it's a long-running process. Alternatively, run tests against a live operator already in-cluster.
- **Helm charts**: You can test Helm charts by invoking `helm` in suite commands or step commands, then asserting on the deployed resources.
- **Debugging**: Use `skipDelete: true` (in `kuttl-test.yaml` or `-skip-delete`) to preserve resources on failure. Collectors and increased timeouts help diagnose flakiness.
- **Coverage**: Cover common K8s resources by asserting on meaningful fields. For example, check `status.readyReplicas` on `Deployment`, check `status.phase` or container readiness on `Pod`, etc.

## CRDs and Custom Controllers

- **CRD tests**: Place CustomResourceDefinitions in `crdDir` so they are installed before tests. This avoids explicit waiting. If you create a CRD in a step, include an assert to wait for it to become "Accepted". After the CRD is ready, subsequent steps can create custom resources of that type.
- **Custom resources**: Once CRDs are established, you can test your operator by creating CRs in a step and asserting on their status fields.
- **Controller processes**: In a Kind or control-plane environment, you must start your controller. In `kuttl-test.yaml`, add:
    
    ```yaml
    commands:
    - command: ./bin/my-controller
      background: true
    
    ```
    
    This launches your controller before tests. (Make sure to set `--namespace` in commands if needed.)
    
- **Environment setup**: When running a control-plane or local cluster, use the `commands` and `manifestDirs` in `kuttl-test.yaml` to install CRDs, rolebindings, or other necessary infrastructure.

## Examples: Pods, Deployments, Services

- **Pod**: Create a Pod in a step (e.g. `00-pod.yaml`), then assert on it. Example assert:
    
    ```yaml
    apiVersion: v1
    kind: Pod
    metadata:
      name: test-pod
    status:
      phase: Running
    
    ```
    
    This waits for `test-pod` to reach Running phase. You can also assert on `status.containerStatuses` for readiness.
    
- **Deployment**: Example step and assert:
    
    ```yaml
    # Step 00-*
    apiVersion: apps/v1
    kind: Deployment
    metadata:
      name: web
    spec:
      replicas: 2
      # ... (omitted template) ...
    
    # Assert 00-assert.yaml
    apiVersion: apps/v1
    kind: Deployment
    metadata:
      name: web
    status:
      readyReplicas: 2
    
    ```
    
    This checks that 2 replicas are available. To scale, a later step might patch `spec.replicas` (as shown above).
    
- **Service**: To test a Service, you can assert on its spec or clusterIP. For example:
    
    ```yaml
    apiVersion: v1
    kind: Service
    metadata:
      name: my-svc
    spec:
      type: ClusterIP
    
    ```
    
    Without a status, this assert means "wait for a Service named `my-svc` with type ClusterIP to exist." You could also assert on `status.loadBalancer.ingress` if using LoadBalancer type.
    
- **Events**: Remember that Kubernetes Events are regular objects. You can assert on events by kind:
    
    ```yaml
    apiVersion: v1
    kind: Event
    reason: Started
    source:
      component: kubelet
    
    ```
    
    This waits for a `Started` event from kubelet.
    

Each of the above patterns shows how to write KUTTL assertions for common resources. Refer to the [KUTTL docs](https://kuttl.dev/docs/#pre-requisites) for full syntax and more examples.