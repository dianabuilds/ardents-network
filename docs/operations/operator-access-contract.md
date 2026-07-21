# Operator Access Contract

Status: canonical for the `v1` local control surface.

## Purpose

The local ConnectRPC API is an authenticated operator boundary. Possession of
an API token identifies one operator credential; it does not grant every
operation in a domain. Authorization is closed by default and is evaluated for
the exact RPC action before application code runs.

This boundary reuses the canonical Identity `SubjectRef`, `CallContext`, and
authorization decision types. Product policy remains owned by the Policy
domain and is evaluated after operator-boundary authorization where relevant.

## Transports

The same authenticated ConnectRPC boundary is available over:

- loopback HTTP for local compatibility and SSH forwarding;
- an optional Unix socket in a private directory for same-host operation;
- `ardentsctl --ssh <target>`, which asks the system OpenSSH client to forward
  one connection to the node's loopback API with `ssh -W`.

SSH does not change API authorization and does not make the bearer credential
optional. The client must still provide a node-scoped token and should pin the
expected node identity. OpenSSH owns host-key verification and authentication;
the CLI uses batch mode, so keys or an agent must be prepared in advance.
Non-default ports, an explicit private key, and an isolated host-key database
are configured with `--ssh-port`, `--ssh-identity`, and `--ssh-known-hosts` (or
the corresponding named-context fields).
Remote non-loopback plaintext API addresses remain forbidden.

## Credential

An operator credential contains:

- an opaque bearer secret;
- a stable operator subject;
- an explicit set of action capabilities;
- an optional expiry time;
- the node name and node principal on which the credential is valid.

An expired credential is unauthenticated. A credential configured for another
node name or principal is rejected before dispatch. Token values are compared
in constant time and are never included in diagnostics.

## Exact Actions

Every generated RPC procedure is present in one version-controlled access
catalog. Each entry declares:

- the exact action capability, such as `workload.start` or `data.get_blob`;
- the owning domain;
- whether the action is read-only or mutating.

Unknown procedures and catalog entries with missing metadata are denied. A
capability for one action never implies a sibling action. `*` is reserved for
explicit test or break-glass credentials and is not used by the normal `ardentsd`
operator credential.

Diagnostics queries are read-only. Node lifecycle, configuration reload,
discovery import, workload commands, publication, fetch, retention, pin, and
drop actions are mutating even when their RPC response is a snapshot.

## Scoped CLI Contexts

A named CLI context may declare:

- a local API address and optional OpenSSH target;
- an expected node name;
- an expected node principal and public key;
- a list of exact action scopes.

The CLI sends the expected node binding and scopes on every request. The server
validates node name and principal against its runtime identity, so the check is
not only a client-side preflight. A non-empty scope list narrows the credential:
the requested action must be present both in the credential and in the context
scope list. Context scopes can never add authority.

The CLI also performs an identity preflight before the requested command and
checks the returned public key where configured. A mismatch stops the command.

## Audit

The control-surface interceptor records one bounded diagnostics event for each
authorized mutating command after application dispatch and every denied call. Successful read-only queries
do not mutate the diagnostics stream they are reading. Audit payloads may contain only:

- action, domain, and read/write class;
- operator subject;
- outcome and stable reason code;
- node name.

The bearer token, authorization header, request/response payloads, filesystem
paths, selectors, object identifiers, and secret material are forbidden from
audit events. Audit recording must not turn an allowed command into a failure;
the diagnostics recorder is an in-process bounded sink.

## Failure Semantics

- missing, malformed, unknown, or expired credentials: `Unauthenticated`;
- unknown procedure, missing exact action, scope narrowing, or binding mismatch:
  `PermissionDenied`;
- application policy denials retain their domain-specific error contract after
  operator authorization succeeds.

## Acceptance

Docker/Linux tests must prove catalog completeness, exact sibling-action
denial, read/write separation, scoped-context narrowing, expired credentials,
wrong node, wrong principal, allowed and denied audit events, and audit
redaction. CLI integration tests must prove that configured identity and scope
bindings are transmitted and enforced.
