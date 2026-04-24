# Stack Trace for Distributed Systems

Traditional distributed tracing flows **forward** and relies on **head-sampling**, often missing the very errors you need to debug because the decision to trace was made before the failure occurred.

> There are [efforts to bring a sampling tail](https://opentelemetry.io/blog/2022/tail-sampling/) to distributed tracing.

This project creates a lab where one can arrange the services to mount any tree call pattern they need, and also to simulate errors. Creating the tree-like call pattern using the docker-compose/ENV plus HTTP Headers

There are two middelwares (inbound/outbound) that perform a  **backward error accumulation**; by intercepting and bubbling error metadata up the call chain, it creates a `distributed stack trace` (in form of a tree) that captures 100% of failure context without the cost of full tracing.

![microservice tree call error-fail-close](/error-fail-close.gif)

The final result is an HTTP header (`x-error-tree`) containing the error tree, encoded as JSON.

```json
{
  "service": "root",
  "status": "error",
  "code": "not-found",
  "error": "an upstream dependency failed in service service-b",
  "children": [
    {
      "service": "service-b",
      "status": "error",
      "code": "not-found",
      "error": "[DRIFT: permission-denied -> not-found] an upstream dependency failed in service service-b",
      "children": [
        {
          "service": "service-e",
          "status": "error",
          "code": "permission-denied",
          "error": "an error occurred in service service-e"
        }
      ]
    }
  ]
}
```

## Why

In microservice architectures, an error at the root often masks the true cause.
* **The Context Gap:** Root calls return a generic `500 Internal Server Error`, losing the specifics of the downstream failure.
* **Sampling Issues:** Distributed tracing is expensive and usually sampled (e.g., 1%). If the root call decides not to sample, the root error requires much more time and effort.
* **Error Drift:** Downstream errors are often remapped (e.g., a `PermissionDenied` becomes a `NotFound`), making root-cause analysis nearly impossible without checking multiple logs.

# Demo

https://github.com/user-attachments/assets/49eedfa0-4359-48ad-8b35-05eaa292c40d

# Requirements

* docker
* docker compose
* jq
* curl

# Running

## Starting

* make logs

## Testing

* make test
* make test-success
* make test-fail-open
  
---

## Challenges & TODOs
* Changing encoding/decoder to something more performant, in terms of allocation/CPU usage (like raw proto)
* Timeout might create split-brain
