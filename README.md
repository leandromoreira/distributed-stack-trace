# Stack Trace for Distributed Systems

Traditional distributed tracing flows **forward** and relies on **head-sampling**, often missing the very errors you need to debug because the decision to trace was made before the failure occurred.

> There are [efforts to bring a sampling tail](https://opentelemetry.io/blog/2022/tail-sampling/) to distributed tracing.

This project introduces **Backward Error Accumulation**. By intercepting and bubbling error metadata up the call chain, it creates a "Distributed Stack Trace" that captures 100% of failure context without the cost of full tracing.

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

---

## The "Opaque 500"
In microservice architectures, an error at the root often masks the true cause.
* **The Context Gap:** Root calls return a generic `500 Internal Server Error`, losing the specifics of the downstream failure.
* **Sampling Issues:** Distributed tracing is expensive and usually sampled (e.g., 1%). If the root call decides not to sample, the root error requires much more time and effort.
* **Error Drift:** Downstream errors are often remapped (e.g., a `PermissionDenied` becomes a `NotFound`), making root-cause analysis nearly impossible without checking multiple logs.

![microservice tree call error-fail-close](/error-fail-close.gif)
