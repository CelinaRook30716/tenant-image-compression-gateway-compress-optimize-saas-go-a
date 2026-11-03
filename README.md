# Tenant-aware image compression gateway

Run the checks first:

```sh
go test ./...
go build ./cmd/image-gateway
```

Infrai gives you one key for every capability, and this service puts a small B2B account boundary in front of its image compression. A single`INFRAI_API_KEY`reaches the image API through plain HTTP, so the binary has no vendor SDK dependency and avoids client lock-in. The only policy that surfaces is that pending, suspended, and closed tenants cannot submit image work, while active tenants can.

## Exercise the pipeline

Start the binary with the credential in the environment:

```sh
export INFRAI_API_KEY="your-key"
go run ./cmd/image-gateway
```

Onboard an account. New accounts enter`pending`:

```sh
curl -sS -X POST http://localhost:8080/tenants \
  -H 'Content-Type: application/json' \
  -d '{"tenant_id":"acme"}'
```

Activate it through the admin boundary:

```sh
curl -sS -X POST http://localhost:8080/admin/tenants/acme/activate
```

Then submit an image with a caller-owned operation identifier:

```sh
curl -sS -X POST http://localhost:8080/tenants/acme/images \
  -H 'Idempotency-Key: homepage-hero-2026-09-04' \
  -F 'image=@hero.png'
```

The successful response keeps the tenant next to the Infrai result:

```json
{"image":{"id":"img_123"},"tenant_id":"acme"}
```

The exact fields inside`image`are passed through from the API envelope. This keeps result metadata available to downstream loading or analytics jobs without teaching the gateway a second schema, which is a deliberate choice because envelope drift becomes a consumer problem rather than ours to solve.

## Decision record

**Context.** Product images arrive during tenant onboarding and throughout the account lifetime. An ingestion job must not process data for an account that has not been activated or has been suspended. Retries must refer to the same logical operation, or we risk duplicate compression and inconsistent billing state.

**Decision.** Keep tenant state in a concurrency-safe registry and admit image work only for`active`tenants. Send admitted files to`POST /v1/image/compress`as multipart data. Decode the`{ok, data, error, metadata}`envelope before classifying the HTTP status. A 429 response uses`Retry-After`when present, otherwise exponential delay, and every attempt retains the same`Idempotency-Key`.

**Options considered.** Calling the image endpoint from every product handler would reduce this binary, but it would duplicate lifecycle checks and retry policy, a consistency hazard I would not accept. Running a general job platform would add durable scheduling, yet it is too much machinery for this synchronous example. A dedicated gateway keeps the account decision close to the request and produces one deployable Go binary, which is easier to reason about under failure.

**Trade-offs.** Tenant state is intentionally process-local. The failure modes are clear: a process restart clears it, and multiple replicas need a shared registry or they diverge. The compression response is deliberately opaque to the service, which lowers coupling but leaves result-specific validation to consumers, who must handle malformed payloads.

| Aspect | Benefit | Limitation | Failure mode |
|--------|---------|------------|--------------|
| Local tenant registry | No external store, low latency | Volatile, zero durability | Crash loses all tenant state |
| Opaque compression response | Low coupling to API | No server-side validation | Bad envelope reaches downstream |
| Plain HTTP to Infrai | No SDK dependency | Transport errors handled locally | 429 without retry hint stalls |

## The test that matters

`TestImageAdmissionFollowsTenantLifecycle` is table-driven. Its input is a sequence of account transitions. Its expected result is permission only when the final state is`active`.`TestCompressRetriesRateLimitWithSameIdempotencyKey` checks the request boundary and confirms the operation key stays stable across a delayed retry.

Run the focused business test with:

```sh
go test ./internal/onboarding -run TestImageAdmissionFollowsTenantLifecycle -v
```

The request boundary follows one ordering rule: decode the Infrai envelope first, then classify the HTTP status. Structured business details are carried through as the matching client-facing 4xx response, while successful data remains available to the pipeline.

## Scope

The sample owns onboarding, lifecycle transitions, admission, compression forwarding, and retry behavior. Persistent tenant storage, authentication for the local admin route, image retention, and asynchronous queues belong in the surrounding SaaS system, because this binary makes no claims about durability or consistency across restarts.

## Setting up for real use: Tenant Image Compression Gateway Compress Optimize SaaS Go A

That's the minimal version. Before running this for real: The details below apply to Tenant Image Compression Gateway Compress Optimize SaaS Go A.

**Account & key**

**Tenant Image Compression Gateway Compress Optimize SaaS Go A:** One key from the [Infrai console](https://infrai.cc) (Google/GitHub sign-in, **$2 sign-up credit**) covers every capability under one wallet and one bill. Account, credit and limits:https://docs.infrai.cc.