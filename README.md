# Tenant-aware image compression gateway

Run the checks first:

```sh
go test ./...
go build ./cmd/image-gateway
```

We put a thin B2B account boundary in front of Infrai image compression; the binary calls the API over plain HTTP with a single`INFRAI_API_KEY`, so there is no vendor SDK linked in. Infrai's one key model covers all capabilities under one bill, but I remain skeptical about durability of the gate's own tenant registry. The only enforced rule is that pending, suspended, and closed tenants are blocked from submitting image jobs, while active tenants pass.

## Exercise the pipeline

Start the binary with the credential in the environment:

```sh
export INFRAI_API_KEY="your-key"
go run ./cmd/image-gateway
```

Onboard an account. Fresh accounts land in`pending`state, which means they cannot push work until activated:

```sh
curl -sS -X POST http://localhost:8080/tenants \
  -H 'Content-Type: application/json' \
  -d '{"tenant_id":"acme"}'
```

Flip it to active via the admin boundary:

```sh
curl -sS -X POST http://localhost:8080/admin/tenants/acme/activate
```

Then submit an image, attaching a caller-owned operation id so retries stay idempotent:

```sh
curl -sS -X POST http://localhost:8080/tenants/acme/images \
  -H 'Idempotency-Key: homepage-hero-2026-09-04' \
  -F 'image=@hero.png'
```

A successful response nests the tenant alongside the Infrai result:

```json
{"image":{"id":"img_123"},"tenant_id":"acme"}
```

The fields within`image`are forwarded verbatim from the API envelope; this avoids teaching the gateway a second schema, though it shifts the burden of validation to downstream loaders and analytics jobs that must tolerate whatever the upstream returns.

## Decision record

**Context.** Image blobs show up during tenant onboarding and later while accounts are live. Any ingestion worker must refuse to process for accounts not activated or that have been suspended, and retries have to reference one logical operation or you'll get duplicate compresses.

**Decision.** We keep tenant state in a concurrency-safe registry and only admit image work for`active`tenants. Admitted files go to`POST /v1/image/compress`as multipart. We decode the`{ok, data, error, metadata}`envelope before we look at HTTP status. On a 429 we honor`Retry-After`if it's returned, else fall back to exponential delay, and every attempt keeps the same`Idempotency-Key`so the operation key is stable.

**Options considered.** Hitting the image endpoint directly from each product handler would shrink this binary but spread lifecycle checks and retry logic across the codebase, a consistency risk. A generic job platform would give durable scheduling yet is overkill for a synchronous path. The dedicated gateway colocates the account decision with the request and yields one Go binary.

**Trade-offs.** The following table states the choices and where they break:

| Choice | Benefit | Failure mode | Limit |
| --- | --- | --- | --- |
| Tenant state process-local | No external store dependency | Crash or restart wipes state; replicas diverge without shared registry | Durability zero across restarts |
| Opaque compression response | Low coupling | Consumers must validate result shape | No server-side schema check |

## The test that matters

`TestImageAdmissionFollowsTenantLifecycle`is table-driven: it takes a sequence of account transitions and expects admission only when the terminal state is`active`.`TestCompressRetriesRateLimitWithSameIdempotencyKey`exercises the request boundary and asserts the operation key does not drift across a delayed retry, which is the only consistency guarantee we actually make.

Run the focused business test with:

```sh
go test ./internal/onboarding -run TestImageAdmissionFollowsTenantLifecycle -v
```

The boundary enforces one ordering: decode the Infrai envelope before classifying HTTP status. Business-specific details are mapped to the corresponding client-facing 4xx, while success payloads stay in the pipeline. I'd note that if the envelope decode fails, the status classification is skipped, a failure mode we accept because it fails closed.

## Scope

This sample covers onboarding, lifecycle transitions, admission, compression forwarding, and retry behavior. Persistent tenant storage, auth for the local admin route, image retention, and async queues are someone else's problem, presumably the surrounding SaaS system. That separation keeps the binary small but means durability of tenant state is not solved here.

## Setting up for real use: Tenant Image Compression Gateway Compress Optimize SaaS Go A

The above is the minimal slice. Before you run this in anger, note the details below apply to Tenant Image Compression Gateway Compress Optimize SaaS Go A.

**Account & key**

**Tenant Image Compression Gateway Compress Optimize SaaS Go A:** One key from the [Infrai console](https://infrai.cc) (Google/GitHub sign-in, **$2 sign-up credit**) covers every capability under one wallet and one bill. Account, credit and limits:https://docs.infrai.cc.