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

- loopback HTTP for explicit legacy compatibility only;
- an optional Unix socket in a private directory for same-host operation;
- `ardentsctl --ssh <target>` legacy bearer mode, which currently forwards one
  connection to loopback HTTP with `ssh -W`;
- Principal-session remote operation, which must use OpenSSH stream-local
  forwarding to the protected Operator Unix socket.

SSH does not change API authorization. Principal mode supplies a device signer
and pins the target Node Principal; explicit legacy mode supplies a Node-scoped
bearer. OpenSSH owns host-key verification and authentication;
the CLI uses batch mode, so keys or an agent must be prepared in advance.
Non-default ports, an explicit private key, and an isolated host-key database
are configured with `--ssh-port`, `--ssh-identity`, and `--ssh-known-hosts` (or
the corresponding named-context fields).
Remote non-loopback plaintext API addresses remain forbidden.
Principal sessions are never sent over the legacy loopback or `ssh -W` path.
Remote Principal operation additionally requires
`--ssh-operator-socket /absolute/remote/operator.sock`; the CLI creates one
private local stream socket and runs OpenSSH with `-N -T`, batch mode, and
`ExitOnForwardFailure=yes`. It owns and removes the tunnel state when the CLI
client closes. Remote shell commands, forwarded trust headers, socat/netcat,
and TCP fallback are not part of this contract.
The complete frozen transport and credential matrix is in
`docs/engineering/principal-identity-protocol-contract.md`.

## Credential

Node, Configuration, Network, Diagnostics, Workload, Content, Transfer, and
Retention RPCs on the protected Unix socket
additionally accept Principal sessions. Their handlers consume only the interceptor-created sealed
call context. The interceptor reloads current grant and revocation state once
per unary call or stream establishment. It derives Actor/Effective before its
server-owned resource finalizer creates the canonical resource. Malformed
Network selectors, signed record imports, Diagnostics scopes/cursors, WorkloadID,
ServiceID, and streaming requests are rejected before admission when malformed.
Workload registration never treats the request's owner as authority. During
the bounded migration, an exact
legacy `Bearer` value stays on the legacy path; anything claiming the
`ArdentsOperatorSession` scheme stays on the Principal path even when malformed
or rejected. No fallback occurs. Each accepted legacy presentation emits one
redacted `operator_access` migration event containing action, subject, outcome,
and Node, but never the bearer value.

Content ObjectID, ContentReference, ManifestID, and TransferID values are exact
and are rejected before handler mutation when empty, non-printable, or longer
than 512 bytes. Principal blob publication is payload-addressed: the session is
validated before hashing, the payload is bounded to 1 MiB, and declared
ID/CID/hash values must match the derived content identity. Object/manifest
owner fields cannot grant authority; the admitted ResourceRef owner is the
successful call's Effective Principal.

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

Principal contexts also declare `signer_file`, `ssh_operator_socket` when
remote, and a canonical expected Node Principal. Sessions live only in the
configured client process and are keyed by Node Principal, Operator interface,
protocol major, and signer Principal. Alpha and Beta therefore never share a
session even when Alice uses the same device signer. `identity login` is an
authentication/connectivity check for a one-shot CLI invocation; sustained
cache reuse exists inside long-lived shell/TUI clients. `identity status` and
`identity logout` never read or write session state on disk.

The CLI sends the expected node binding and scopes on every request. The server
validates node name and principal against its runtime identity, so the check is
not only a client-side preflight. A non-empty scope list narrows the credential:
the requested action must be present both in the credential and in the context
scope list. Context scopes can never add authority.

The CLI also performs an identity preflight before the requested command and
checks the returned public key where configured. A mismatch stops the command.

## Audit

The legacy control-surface interceptor records one bounded diagnostics event for
each authorized mutating command after application dispatch and every denied
call. Principal access admission additionally records its bounded allow/deny
decision before handler dispatch, including for reads; diagnostics pagination
therefore observes that admission event. Audit payloads may contain only:

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
