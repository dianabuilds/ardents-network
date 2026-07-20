# STB-104 Evidence — Local Control Surface Hardening

Date: 2026-07-18

## Boundary Changes

- Plaintext API configuration now accepts only IPv4/IPv6 loopback or
  `localhost`. Wildcard, remote IP, empty-host, and arbitrary hostname binds
  fail during configuration before Node Runtime starts. There is no hidden
  remote-TLS mode; remote ingress remains unsupported until explicitly
  designed.
- Product assembly replaced wildcard `*` authority with explicit read/write
  capabilities for node, diagnostics, workload, data, discovery, and transport
  domains. The bearer credential represents the single local administrator.
- Credential comparison uses constant-time comparison of fixed-size SHA-256
  digests.
- The server accepts exactly one token source: `ARDENTS_API_TOKEN` or
  `ARDENTS_API_TOKEN_FILE`. Secret files must be regular, non-symlink files;
  Unix group/other permission bits are rejected.
- HTTP limits now cover body size, header size, header read, full read, unary
  execution, write, and idle durations. The authenticated node-event stream is
  explicitly exempt from unary/write deadlines and remains bounded by its
  client context.
- API error mapping preserves stable code/domain/operation but no longer echoes
  raw internal error text, credentials, authorization headers, or payloads.
- Docker multi-node configuration no longer contains hardcoded tokens, exposes
  no host API ports, binds API to container loopback, and mounts required
  per-node secrets read-only with mode 0400. `docker compose config --quiet`
  passed.

## Negative Evidence

Automated tests prove:

- remote plaintext and wildcard addresses are rejected;
- production admin capabilities contain no wildcard;
- a credential for another local principal is unauthenticated and neither
  credential appears in the error;
- missing and ambiguous credential sources fail closed;
- regular token-file loading succeeds;
- oversized bodies receive HTTP 413;
- unary timeout receives HTTP 503 without reflecting request secrets;
- node-event streaming is not assigned the unary deadline;
- mapped internal errors do not reveal a bearer credential;
- all server timeout/header limits are non-zero.

Validation after the change:

- `go test ./cmd/ardd ./internal/transport/connectrpc`: passed;
- formatting and code-size gates: passed;
- `go vet ./...`: passed;
- full fast suite: passed;
- full tagged `tests/integration/local-control-surface`: passed in 41.9 seconds.

## Runtime Security Guard Assessment

- Sensitive assets: operator bearer credential, internal error text, request
  payloads, and administrative authority.
- Owner: local control boundary with Identity authorization and Diagnostics
  projection.
- Invariant: no default remote plaintext authority; no wildcard production
  authority; no credential/raw internal detail in API failures; finite resource
  use for unauthenticated and authenticated requests.
- Assessment: pass. The default is loopback-only and fail-closed; secrets are
  separated from the Docker environment; rejections remain generic and
  structured without leaking credentials.

## Gate Result

Passed. Unauthorized, wrong-credential, oversized, timed-out, and remote
plaintext cases fail safely; production authority is explicit; diagnostics and
API failures do not reveal credentials.
