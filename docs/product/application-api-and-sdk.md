# Application Interface And SDK

## Decision

The public SDK targets `ardents.application.v1`, not the administrative
`ardents.v1` Operator Interface. The two interfaces may use the same RPC
technology, but they do not share credentials, authorization actions, generated
packages, or handlers.

An Application normally talks to a Node on the same host through the dedicated
private Unix socket configured by `application_interface.socket_path`. The
Operator Interface has a different listener and rejects Application
Credentials. A future remote transport requires an explicit mutually authenticated
TLS profile; binding the existing plaintext Operator Interface to a remote
address is not an Application transport.

`ardentsd init` creates an initial least-privilege Application Credential at
`application-token` alongside `application.sock`. This bootstrap credential is
limited to `application.content.put` and `application.content.get`. It is not an
Operator credential and grants no lifecycle, configuration, diagnostics, or
workload authority.

## Go SDK Shape

The Go SDK grows by complete vertical slices rather than empty packages:

```text
sdk/go/
  client/       connection, credentials, lifecycle, and domain clients
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

## First Vertical Slice: Content

The first interface is deliberately small:

```go
ref, err := app.Content.Put(ctx, payload, content.WithMediaType("text/plain"))
payload, err = app.Content.Get(ctx, ref)
```

For a native Linux installation, construct the client with the provisioned
socket and credential:

```go
credential, err := client.FileCredential(
    "/var/lib/ardents-applications/application-token",
)
if err != nil {
    return err
}
app, err := client.New(client.Config{
    SocketPath: "/var/lib/ardents-applications/application.sock",
    Credential: credential,
})
```

PIA-011A provides the typed Principal authentication SDK flow for an
Application installation with an enrolled Principal and Key Credential:

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

`SessionSigner` has no generic byte-signing method. The SDK validates the exact
Application Audience, Unix transport profile, purpose, timestamps, peer
binding, Principal, and pinned Node before asking it to sign. Sessions remain
in memory, concurrent login is single-flight, and a unary call refreshes once
only after `Unauthenticated`. Key storage belongs to the embedding Application;
there is no built-in file signer until cross-platform custody rules can be
tested.

PIA-011B adds a distinct one-use enrollment flow. An Operator authorizes the
exact prospective Application Principal and initial `application.content.*`
actions, then writes the returned ten-minute ticket to a protected file. The
Application parses that ticket and supplies a typed `EnrollmentSigner` with
only `Principal`, `Credential`, and `SignEnrollmentChallenge` operations to
`client.EnrollApplication`. The SDK never receives a generic signing oracle and
does not provide a root/file signer. Enrollment, the initial Application grant,
and retirement of that exact installation's legacy token are atomic.
The ticket is never an `application-token`, Principal, session, owner, or
delegation.

PIA-012 propagates a Principal `AuthorizedCall` through content handlers, but
production activation remains deliberately blocked until PIA-014 supplies
durable owner-aware Blob/content access. Knowledge of a CID alone is not
ownership proof or authorization to read. Provisioning therefore continues to
create the legacy token and the example above remains the normal content path
for this staged release. PIA-017 later owns the monotonic surface
retirement state machine and default removal; neither is silently introduced by
PIA-011B.

The native installer creates the `ardents-apps` system group. An operator grants
a local service access by adding its Unix account to that group; the Application
directory is setgid `0750`, the bootstrap token is `0640`, and the socket is
`0660`. Membership grants only the configured Application actions, never
Operator authority.

The native bootstrap admission is time-bounded and can be extended locally
through the installer `renew-application` operation. This is an operational
bridge, not the final per-Application issuance, rotation, and revocation API.

`Put` succeeds only after durable local storage and returns a content-derived
reference. `Get` uses a verified local payload when present and otherwise asks
the Node to perform its normal source selection and network fetch. The SDK does
not expose peer selection, CID calculation, storage layout, ConnectRPC, or
protobuf.

Version 1 uses bounded unary payloads. Streaming content is a later additive
interface and must not silently change the ordering, limits, or retry semantics
of unary `Put` and `Get`.

## Compatibility And Security

- Application actions use the `application.*` namespace.
- An Application Credential is never accepted by the Operator Interface, and an
  Operator credential is never accepted by the Application Interface.
- Unknown methods and missing actions fail closed.
- Public errors have stable codes and retryability; internal paths, topology,
  credentials, and raw policy failures are not returned.
- `v1` evolves additively. A breaking wire change requires a new major protocol
  package.
- Generated bindings are conformance machinery, not the ergonomic SDK interface.

## Delivery Sequence

1. Content protocol, Node adapter, Go client, and contract test.
2. Separate listener and bootstrap Application Credential (implemented).
3. Online credential issuance, listing, rotation, and revocation through the
   Operator Interface.
4. Discovery resolution.
5. Messaging send/receive with bounded cursors and backpressure.
6. Hosting registration, readiness, lease renewal, and drain.
7. Extract and publish the Go SDK as an independently versioned module after the
   repository remote and public module path are fixed.
