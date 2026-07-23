# Operator Access Contract

Status: canonical for the Principal-only `v1` control surface.

## Purpose

The Operator Interface is a protected ConnectRPC boundary. An Operator is a
Principal holding current administrative Access Grants for this Node; it is not
an account type and the CLI process is not the Operator. Authentication and
authorization are separate:

1. the client proves a root-authorized device Credential and receives a
   short-lived Operator Session;
2. every protected call presents that Session;
3. the interceptor reloads current device-revocation, grant, and Policy state
   before dispatch.

Sessions contain no permissions. Actor and Effective are created only by a
successful admission. They are equal on the Operator Interface because
Delegation is an Application-only mechanism.

There is no permanent Operator token, token file, plaintext protected route, or
break-glass credential in `v1`.

## Transport And Audience

The normal same-host transport is the permission-protected Operator Unix
socket configured by `api.socket_path`. The server derives the exact Audience
and transport binding from the accepting listener:

```text
(Node Principal, operator interface, protocol major 1, transport peer)
```

For remote administration, `ardentsctl --ssh` creates an OpenSSH stream-local
forward directly to that protected remote Unix socket. It does not use
loopback HTTP, `ssh -W`, a remote shell, forwarded trust headers, socat/netcat,
or TCP fallback. OpenSSH owns host-key verification and SSH authentication.
`--ssh-port`, `--ssh-identity`, `--ssh-known-hosts`, and the required absolute
`--ssh-operator-socket` select that transport.

Principal authentication endpoints and protected calls are not exposed over
plaintext HTTP or non-loopback TCP. A Session issued for another Node,
interface, protocol major, or transport peer is unauthenticated. A malformed or
failed Session presentation never selects another credential path.

Operator and Application listeners, wire packages, credential schemes, and
action catalogues remain separate. An Application Session is rejected on the
Operator Interface and vice versa.

## Enrollment And Credentials

`ardentsd init` creates the Node Principal and writes one random, short-lived,
one-use Bootstrap Ticket beside the protected Operator socket. The Ticket
authorizes only the first Principal enrollment. It is not a Principal,
Credential, Session, Access Grant, or recovery credential.

The prospective Operator creates an offline root signer and a routine device
signer, then enrolls through the protected socket. The root signs only the
typed, domain-separated enrollment challenge. Normal login uses the
root-signed finite device Credential and device signer; the root is not loaded
for routine calls. Successful first enrollment atomically consumes the Ticket,
and the provisioning workflow deletes its file.

Later enrollments and all grant/device administration require an admitted
Operator Session. Each Application installation has its own Principal and
enrolls only through a separate one-use Application Enrollment Ticket issued
by an Operator for that exact Principal and initial Application actions.

## Sessions

Begin/Complete authentication validates the typed challenge, Credential,
device proof, Audience, expiry, replay state, and server-derived transport
peer. Session secrets exist only in client and daemon memory, are short-lived,
and are never persisted or printed. Restart invalidates every Session while
durable Credentials, grants, enrollment records, and revocations remain.

The CLI session cache key includes Node Principal, Operator interface, protocol
major, and signer Principal. Alpha and Beta therefore never share a Session
when Alice operates both Nodes. Concurrent callers in one client share a
single Begin/Complete exchange. `identity logout` invokes `EndSession` for each
live bound Session, then best-effort zeros the local secret; closing a client
does the same. `identity status` exposes only public cache-key facts.

On `Unauthenticated`, the client may discard only the failed generation,
authenticate once, and replay one unary call once. A second failure stops.
`PermissionDenied` and other failures do not trigger authentication fallback.
A stream may be replayed only before its first event.

## Exact Actions And Resources

Every generated protected RPC procedure is present in one version-controlled
Operator action catalogue. Each entry declares its exact action, owning domain,
read/mutation class, and resource extraction/finalization rule. Unknown
procedures, actions, resource kinds, identifiers, scopes, versions, or missing
metadata fail closed. Authority for one action never implies a sibling action.

Request owner strings, display names, Unix user IDs, Waku Peer IDs,
WorkloadIDs, ServiceIDs, CIDs, and other resource identifiers never become a
Principal or confer authority. For server-owned resources, admission resolves
Actor/Effective before the finalizer creates the canonical resource. Knowledge
of a Content Reference is not proof of ownership or read authority.

Content identifiers are validated before mutation. Principal content
publication derives the immutable Content Reference from the bounded payload;
declared reference/hash values must match, and owner binding comes from the
admitted Effective Principal.

## CLI Contexts

A named Principal context may contain:

- the Operator Unix-socket address, or an SSH target and remote socket path;
- the expected canonical Node Principal and optional public-key pin;
- the device signer file;
- optional exact action scopes that can only narrow authority.

The CLI performs a public Node identity preflight and stops on a pin mismatch.
The Node and interface bindings are also enforced by session admission, not
only by the client.

## Audit And Redaction

Principal admission records bounded allow/deny evidence before protected
handler dispatch. Audit may include the exact action, resource kind, Actor,
Effective, Node Principal, outcome, and stable reason code. It must not include
private keys, device signatures, Credentials, session secrets or IDs,
Bootstrap/Application Enrollment Tickets, proofs, Delegations, channel
secrets, bearer values used by the separate observability boundary, request
payloads, or protected filesystem paths.

## Failure Semantics

- missing, malformed, expired, revoked, unknown, replayed, or cross-Audience
  authentication evidence: `Unauthenticated`;
- unknown procedure/action/resource/scope, missing current grant, scope
  mismatch, or sibling action: `PermissionDenied`;
- malformed requests fail before product mutation;
- product Policy denials retain their domain error contract after identity
  admission succeeds;
- unavailable or corrupt identity state fails closed.

## Acceptance

Tests must prove successful device authentication, first enrollment, exact
grant enforcement, sibling denial, Alpha/Beta isolation, cross-interface and
cross-peer rejection, expiry boundaries, replay/concurrent completion,
next-call grant/device revocation, `EndSession`, restart invalidation, durable
grant recovery, atomic administration, and secret redaction.
