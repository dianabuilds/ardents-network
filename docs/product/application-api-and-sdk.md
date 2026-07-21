# Application Interface And SDK

## Decision

The public SDK targets `ardents.application.v1`, not the administrative
`ardents.v1` Operator Interface. The two interfaces may use the same RPC
technology, but they do not share credentials, authorization actions, generated
packages, or handlers.

An Application normally talks to a Node on the same host through a private Unix
socket. A future remote transport requires an explicit mutually authenticated
TLS profile; binding the existing plaintext Operator Interface to a remote
address is not an Application transport.

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
2. Application credential issuance, revocation, and separate Unix listener.
3. Discovery resolution.
4. Messaging send/receive with bounded cursors and backpressure.
5. Hosting registration, readiness, lease renewal, and drain.
6. Extract and publish the Go SDK as an independently versioned module after the
   repository remote and public module path are fixed.
