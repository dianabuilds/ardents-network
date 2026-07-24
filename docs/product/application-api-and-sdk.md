# Application Interface And SDK

## Decision

The public SDK targets `ardents.application.v1`, not the administrative
`ardents.v1` Operator Interface. The two interfaces may use the same RPC
technology and the same `identity/access` implementation, but they retain
separate listeners, session schemes, authorization actions, generated packages,
interceptors, and handlers.

An Application installation is represented by its own Principal. It normally
talks to a Node on the same host through the dedicated permission-protected Unix
socket configured by `application_interface.socket_path`. The Operator
Interface has a different listener and rejects Application sessions. Plaintext
HTTP is not an authentication or product-call transport. A future remote
transport requires a separately approved mutually authenticated TLS profile.

Authentication proves the Application Principal; it grants no authority.
Application permissions come only from finite Node-issued Access Grants for the
Application Audience. An Application acts for another Principal only through a
valid one-hop, non-redelegable Delegation intersected with both Principals'
current grants on every call.

## Go SDK Shape

The Go SDK grows by complete vertical slices rather than empty packages:

```text
sdk/go/
  client/       connection, typed signers, session lifecycle, and domain clients
  content/      immutable content put/get
  errors/       stable typed errors
  discovery/    service discovery (next slice)
  messaging/    send and receive (later slice)
  hosting/      publish and serve an Application (later slice)
  internal/     wire adapters hidden from SDK consumers
  protocol/     generated application.v1 bindings; not SDK domain types
```

SDK domain types never alias generated protobuf messages and never import an
Ardents `internal/*` package. The Node-side Application adapter maps public wire
messages directly to narrow owner interfaces in Content, Discovery, Messaging,
and Hosting. It does not invoke Operator handlers.

## Principal Enrollment And Sessions

Each installation generates its root and device keys locally. An Operator
authorizes the exact prospective Application Principal and an exact subset of
Application actions, then creates a ten-minute one-use Application Enrollment
Ticket. The ticket is written only to a newly created protected file and is
never printed.

For the supported same-host handoff, the Application supplies the protected
ticket path and a typed `EnrollmentSigner` to
`client.EnrollApplicationFromFile`. The helper accepts only an absolute,
private, regular file containing the exact canonical ticket, proves root
possession through the existing Application Interface, and removes the same
file only after validating the committed response. A pre-commit failure retains
the file for retry. `EnrollmentFileCleanupError` returns the committed
`EnrollmentResult` when enrollment succeeds but safe cleanup does not; callers
must not repeat enrollment. Its `TicketFileState` is `retained` when the exact
file remains at `TicketPath`, `retired` when plaintext removal completed but
durability confirmation failed, and `unknown` when a concurrent path change or
filesystem failure prevents a safe assertion. Only the `retained` state permits
manual removal at the original path; `unknown` requires filesystem
investigation and must not trigger automatic path deletion.

`client.ParseApplicationEnrollmentTicket` and `client.EnrollApplication`
remain available for an embedding Application whose protected delivery
mechanism is not a file. Enrollment records the canonical root binding and
device Credential, consumes the ticket, and commits the initial Node-signed
Application Access Grant atomically. A failed transaction leaves no partial
enrollment and does not consume the durable ticket record.

The ticket is not a Principal, Credential, session, grant, owner, or normal
authentication mechanism. Only its domain-separated digest is durable.

Normal calls use a typed device signer:

```go
type SessionSigner interface {
    Principal(context.Context) (string, error)
    Credential(context.Context) (*identity.Artifact, error)
    SignAuthenticationChallenge(context.Context, identity.Challenge) ([]byte, error)
}

app, err := client.New(client.Config{
    SocketPath:    "/var/lib/ardents-applications/application.sock",
    NodePrincipal: expectedNodePrincipal,
    Signer:        applicationSigner,
})
if err == nil {
    err = app.Session.Authenticate(ctx)
}
```

`SessionSigner` and `EnrollmentSigner` expose only purpose-specific operations.
Neither offers generic `Sign([]byte)`. The SDK validates the exact Application
Audience, Unix transport profile, purpose, timestamps, peer binding, Principal,
Credential, and pinned Node before requesting a signature.

Sessions stay in memory, are keyed by exact Audience, and contain no
permissions. Concurrent authentication is single-flight. A unary call refreshes
once only after `Unauthenticated`; `PermissionDenied` is never treated as a
login signal. Application key storage belongs to the embedding Application;
the SDK does not provide a general file or root signer.

## First Vertical Slice: Content

The first interface is deliberately small:

```go
ref, err := app.Content.Put(ctx, payload, content.WithMediaType("text/plain"))
payload, err = app.Content.Get(ctx, ref)
```

`Put` derives ownership only from the admitted call's Effective Principal. For
a direct call, `Actor == Effective == Application`. For a delegated call,
`Actor == Application` and `Effective == Delegator`. No request field can select
either identity or the owner.

The Node stores a durable `(Owner Principal, Content Reference)` binding
separately from the deduplicated payload. `Get` requires that exact binding
before either local read or remote fetch. Knowledge of a CID proves payload
identity, not ownership or read authority; public failures do not disclose a
sibling Principal's binding.

`Put` succeeds only after durable local storage and returns a content-derived
reference. `Get` uses a verified local payload when present and otherwise asks
the Node to perform its normal source selection and network fetch. A successful
fetch may fill bytes for an existing binding but cannot create ownership.

Version 1 uses bounded unary payloads. Streaming content is a later additive
interface and must not silently change the ordering, limits, or retry semantics
of unary `Put` and `Get`.

## Installation And Socket Access

The native installer creates the `ardents-apps` system group and the protected
Application socket directory. Adding a local service account to that group
permits it to reach the socket; group membership does not authenticate a
Principal and grants no action. The installation creates no reusable
Application credential. Every Application installation enrolls its own
Principal and uses its own finite device Credential and short-lived session.

## Security And Evolution

- Application actions use the `application.*` namespace.
- `ArdentsApplicationSession` is accepted only on the Application listener;
  `ArdentsOperatorSession` is accepted only on the Operator listener.
- Unknown, malformed, expired, revoked, cross-Node, cross-interface, or
  cross-peer sessions fail closed without another credential path.
- Grants, Delegation, device revocation, and product Policy are re-evaluated on
  every call.
- Public errors have stable codes and retryability; internal paths, topology,
  credentials, tickets, proofs, sessions, Delegations, and raw policy failures
  are not returned or logged.
- `v1` evolves additively. A breaking wire change requires a new major protocol
  package.
- Generated bindings are conformance machinery, not the ergonomic SDK
  interface.

## Delivery Sequence

1. Content protocol, Node adapter, Go client, and contract tests.
2. Separate protected Application listener and Principal session flow.
3. One-use Operator-authorized Application enrollment and initial exact grant.
4. Owner-aware content admission and one-hop Delegation.
5. Discovery resolution.
6. Messaging send/receive with bounded cursors and backpressure.
7. Hosting registration, readiness, lease renewal, and drain.
8. Extract and publish the Go SDK as an independently versioned module after the
   repository remote and public module path are fixed.
