# STB-602 Evidence

Date: 2026-07-20
Status: completed

## Outcome

The local ConnectRPC boundary now enforces least privilege for exact operator
actions. Every generated RPC is closed by default and mapped to one explicit
action, owning domain, and read/write class. CLI contexts transmit node and
principal bindings plus exact scopes that can only narrow the credential.
Mutating commands and all denials produce bounded, redacted audit events.

## Implemented Contract

- One version-controlled catalog covers all 48 generated Ardents RPC
  procedures. A descriptor-backed test fails if an RPC is missing, duplicated,
  or has incomplete access metadata.
- Identity authorization supports exact action capabilities. A right such as
  `workload.start` does not imply `workload.stop`, another workload action, or
  a coarse domain wildcard.
- Operator credentials carry a stable subject, explicit actions, optional
  RFC3339 expiry, and the node name/principal on which they are valid. Unknown
  configured actions and production wildcards fail validation.
- Operator configuration maps `api.operator_subject`, `api.capabilities`, and
  `api.credential_expires_at` into the real server credential. An omitted
  capability list resolves to the explicit built-in local-admin action list,
  never to `*`.
- CLI contexts support expected node name, principal, public key, and exact
  action scopes. Node/principal/scope headers are enforced by the server;
  public-key preflight remains a client-side identity proof.
- Non-empty context scopes are checked as a subset of the credential and then
  become the effective action set for the call. They cannot grant authority.
- Successful read-only queries do not mutate the diagnostics stream they read.
  Authorized mutating commands are audited after application dispatch; every
  denial is audited before dispatch.

## Failure And Security Proof

- Missing, malformed, wrong, and expired tokens return `unauthenticated`.
- Wrong node, wrong principal, excessive context scope, unknown procedure, and
  missing exact action return `permission_denied` before domain dispatch.
- An allowed sibling action does not authorize the requested action.
- Audit payloads contain only action, domain, access class, operator subject,
  outcome, stable reason, and node name. They do not contain bearer tokens,
  authorization headers, request/response payloads, selectors, object meaning,
  or secret filesystem paths.
- The first E2E run exposed an observer-ordering bug: pre-dispatch audit for
  `node.start` overwrote a not-yet-loaded recovery ledger in the test lifecycle.
  Audit was moved post-dispatch for authorized commands. NRE-001 now proves the
  saved recovering operation survives start and restart while command audit is
  retained.

## Acceptance Checks

- Focused Docker tests passed for Identity authorization, runtime config,
  ConnectRPC, CLI/client, and `cmd/ardd`.
- OAI-001 exercised the real HTTP/ConnectRPC handler for exact allow/deny,
  scoped contexts, wrong node, wrong principal, expired credential, and audit
  redaction.
- Final Docker fast suite passed on the final code.
- Full Docker integration passed 131/131 scenarios with zero failures and
  356.013 seconds of aggregate test duration.
- Full Docker E2E passed 16/16 scenarios with zero failures and 126.028 seconds
  of aggregate test duration.
- `go vet ./...`, `go mod verify`, import-boundary checks, QA catalog validation,
  and production code-size validation passed.
- QA catalog reports 147 tests, 45 scenarios, 147 formal bindings, and zero
  issues or missing bindings.
- Code-size review split operator credential/node mapping and CLI shell input
  handling; no soft or hard production breach remains in the checked paths.

## Resource And Orchestration Truth

All test execution ran in Linux Docker containers with persistent module/build
caches and explicit Linux `timeout` bounds. Long suites ran in detached named
containers. Progress came from raw report counts and `summary.json`, not from a
blocking `docker wait` or live `docker logs` call.

During integration and E2E runs, test containers stayed around 85-190 MiB and
showed active CPU/IO when compiling or running. The final host snapshot records
approximately 4.38 GiB for `vmmemWSL` and 212.67 GiB free on drive C. No CPU,
memory, disk, or OOM exhaustion occurred.

## Evidence Surface

- `docs/operator-access-contract.md`
- `docs/qa/integration/operator-access-least-privilege.md`
- `internal/identity/api/authorization.go`
- `internal/transport/connectrpc/access_catalog.go`
- `internal/transport/connectrpc/access_interceptor.go`
- `internal/transport/connectrpc/auth_config.go`
- `internal/transport/connectrpc/auth_token.go`
- `boundary/cli/config.go`
- `boundary/cli/client/client.go`
- `tests/integration/local-control-surface/operator_access_test.go`
- `tests/.artifacts/reports/stb-602-integration-final2/summary.json`
- `tests/.artifacts/reports/stb-602-integration-final2/junit.xml`
- `tests/.artifacts/reports/stb-602-e2e-final2/summary.json`
- `tests/.artifacts/reports/stb-602-e2e-final2/junit.xml`
- `tests/.artifacts/resources/stb-602-final.json`

## Acceptance Decision

Passed. Exact operator permissions, credential lifetime and target binding,
context narrowing, read/write separation, and safe command/denial audit are
enforced through the real local control boundary with no deferred critical
behavior.
