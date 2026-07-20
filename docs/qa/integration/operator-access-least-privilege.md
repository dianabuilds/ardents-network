# Scenario ID

`OAI-001`

## Layer

`integration`

## Domain

`Local Control Surface`

## Category

Least-privilege operator authentication, target binding, authorization, and
audit.

## Goal

Prove through the real ConnectRPC handler that an operator credential grants
only exact actions, a CLI context can only narrow that credential, node and
principal bindings are enforced by the server, expired credentials fail, and
allow/deny audit records do not retain secrets or request payloads.

## Preconditions

- a runtime-backed node is started with a stable node name and principal;
- the ConnectRPC handler uses the canonical procedure access catalog;
- the runtime diagnostics recorder is wired as the operator audit sink.

## Steps

1. Start a handler whose credential contains only `node.status`.
2. Call `GetNodeStatus` with matching node/principal bindings and a
   `node.status` context scope.
3. Call the authorized mutating action `StartNode`, then call sibling query
   `GetNodeCapabilities` with the same credential.
4. Repeat `GetNodeStatus` with a wrong node and then a wrong principal.
5. Start a handler with an already expired credential and call
   `GetNodeStatus`.
6. Inspect runtime diagnostics directly for allowed and denied audit events.

## Expected Result

- `GetNodeStatus` and the explicitly granted `StartNode` are allowed;
- the sibling action and binding mismatches are `permission_denied`;
- the expired credential is `unauthenticated`;
- audit contains the allowed mutating command and all denials with stable
  action, access, subject, outcome, reason, and node
  fields;
- audit contains no bearer token, authorization header, request payload, or
  secret path.

## Failure/Degraded Variant

An unknown procedure, an action missing from either credential capabilities or
context scopes, and unavailable target identity all fail closed before domain
application code runs.

## Related Tests

- `tests/integration/local-control-surface/operator_access_test.go::TestOperatorAccessLeastPrivilegeAndAudit`
- `tests/integration/local-control-surface/operator_access_test.go::TestOperatorAccessRejectsExpiredCredential`

## False Positive Risk

Calling interceptor helpers directly would miss generated handler wiring. The
test must send actual ConnectRPC requests through an HTTP test server.

## False Negative Risk

Checking only status codes could miss audit leakage. The test serializes the
recorded audit envelopes and searches them for supplied secret values.
