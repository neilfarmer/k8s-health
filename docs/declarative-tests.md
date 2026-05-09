# Declarative `HealthTest` format

`khealth test run` consumes YAML files that look like Kubernetes resources but
are *not* applied to the cluster as CRDs — they are parsed locally by `khealth`
and drive the test runner.

The format intentionally mirrors familiar Kubernetes shape (apiVersion, kind,
metadata, spec) so it's instantly readable.

## Schema (v1alpha1)

```yaml
apiVersion: khealth.io/v1alpha1
kind: HealthTest
metadata:
  name: string                  # unique per run
  labels: { string: string }    # optional, used for filtering
spec:
  description: string           # human summary
  timeout: duration             # overall budget, default 5m
  setup:                        # optional resources to apply before steps
    - apply:
        manifest: <path>        # path to YAML/JSON, or
        inline: |               # inline manifest
          apiVersion: v1
          kind: Pod
          ...
      waitFor:                  # optional readiness gate after apply
        condition: Ready        # or Available, or jsonPath
        timeout: 60s
  steps:                        # ordered, each produces one Finding
    - name: string
      timeout: duration         # default 30s
      <one of: http | exec | dns | log | k8sObject>
  cleanup:
    onSuccess: true             # default true
    onFailure: false            # default false (keep for debugging)
    delete:                     # explicit deletes (in addition to setup reverse)
      - manifest: <path>
```

### Step kinds

#### `http` — HTTP probe via ephemeral pod

`khealth` schedules a short-lived `curl` pod in the target namespace and runs
the probe from inside the cluster.

```yaml
- name: api-returns-200
  http:
    url: http://api.payments.svc.cluster.local/healthz
    method: GET                        # default GET
    headers:
      X-Smoke-Test: khealth
    expect:
      statusCode: 200
      body:
        contains: "ok"
        # one of: equals, contains, matchesRegex, jsonPath
      latency:
        lessThan: 500ms
```

#### `exec` — exec into a pod

```yaml
- name: dns-resolves-kubernetes
  exec:
    pod:
      namespace: kube-system
      labelSelector: app=khealth-probe
    command: ["nslookup", "kubernetes.default.svc.cluster.local"]
    expect:
      exitCode: 0
      stdout:
        contains: "kubernetes.default.svc"
      stderr:
        notContains: "NXDOMAIN"
```

#### `dns` — convenience for DNS

Sugar over `exec` that doesn't require the user to bring their own pod.

```yaml
- name: external-dns-works
  dns:
    name: example.com
    recordType: A
    expect:
      minAnswers: 1
```

#### `log` — assert on pod logs

```yaml
- name: no-panic-in-controller
  log:
    pod:
      namespace: kube-system
      name: my-controller-0
    since: 2m
    expect:
      notMatches: "panic:"
```

#### `k8sObject` — assert on a live object

```yaml
- name: deployment-fully-rolled-out
  k8sObject:
    apiVersion: apps/v1
    kind: Deployment
    namespace: storefront
    name: web
    expect:
      jsonPath:
        - path: ".status.readyReplicas"
          equals: 4
        - path: ".status.conditions[?(@.type=='Available')].status"
          equals: "True"
```

## Lifecycle

```
1. Parse + validate (no cluster I/O).
2. Apply spec.setup resources via server-side apply.
3. Wait for declared waitFor conditions.
4. Run steps in order. Each produces a Finding.
5. Cleanup according to spec.cleanup.
6. Emit Report.
```

If step N fails and `--fail-fast` is set, subsequent steps are marked
`SKIPPED`. Cleanup still runs unless `--keep-on-failure` was passed.

## Worked example

```yaml
# examples/healthtests/post-deploy-canary.yaml
apiVersion: khealth.io/v1alpha1
kind: HealthTest
metadata:
  name: storefront-canary
  labels:
    suite: post-deploy
spec:
  description: |
    Sanity-check the storefront after a deploy: deployment is rolled out,
    the public endpoint returns 200, and no panics in the last 2 minutes
    of logs.
  timeout: 3m
  setup:
    - apply:
        inline: |
          apiVersion: v1
          kind: Pod
          metadata:
            name: khealth-curl
            namespace: storefront
          spec:
            restartPolicy: Never
            containers:
              - name: curl
                image: curlimages/curl:8.6.0
                command: ["sleep", "300"]
      waitFor:
        condition: Ready
        timeout: 30s
  steps:
    - name: deployment-rolled-out
      k8sObject:
        apiVersion: apps/v1
        kind: Deployment
        namespace: storefront
        name: web
        expect:
          jsonPath:
            - path: ".status.readyReplicas"
              equalsField: ".spec.replicas"

    - name: public-endpoint-200
      http:
        url: http://web.storefront.svc.cluster.local/health
        expect:
          statusCode: 200
          latency: { lessThan: 800ms }

    - name: no-panic-in-logs
      log:
        pod:
          namespace: storefront
          labelSelector: app=web
        since: 2m
        expect:
          notMatches: "panic:"
  cleanup:
    onSuccess: true
    onFailure: false
```

## Why not real CRDs?

- `khealth` should run against a brand-new cluster without first installing
  CRDs.
- The runner is a CLI, not a controller — there's no reconciliation loop.
- Local-only YAML keeps the iteration loop fast (`test lint` is offline).

A future phase may add an *optional* operator that watches a real
`HealthTest` CRD and runs the same code path on a schedule.
