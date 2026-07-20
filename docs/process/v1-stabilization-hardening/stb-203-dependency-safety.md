# STB-203 Dependency Safety — Waku Log Suppression

Date: 2026-07-19

## Decision

Accept direct use of the already-pinned `go.uber.org/zap v1.27.0` dependency
only to supply a level-compatible discarding logger through Waku's supported
`WithLogger` option.

## Role

Waku logs subscription filters and message-derived fields at multiple levels.
An opaque selector is carrier-visible but the
`ardents-private/1` contract forbids copying it into Ardents diagnostics or
ordinary process logs. Enumerating and redacting every current and future
upstream field would be brittle. Waku substrate logs are therefore disabled;
Ardents retains operator truth through its own transport outcomes, readiness,
health signals, and stable privacy reason codes.

## Posture

- this is not a new module or version: zap was already selected transitively by
  Waku and pinned in the repository;
- the module is stable, actively released, MIT licensed, and broadly used;
- `govulncheck` reports no zap vulnerability in the repository;
- no logging or observability substrate is replaced;
- the control preserves Waku as canonical network foundation and uses Waku's
  supported configuration API.

## Result

Accepted. The dependency graph does not grow; `go mod tidy` promotes the
existing pin from indirect to direct because product code now imports it.
