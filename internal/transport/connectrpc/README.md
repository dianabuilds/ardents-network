# ConnectRPC Boundary

`internal/transport/connectrpc` is the protocol adapter from Ardents domain
contracts to the ConnectRPC protobuf surface.

It is responsible for:

- authentication-aware request handling;
- mapping domain-owned snapshots, events, commands, and queries to protobuf
  request and response messages;
- ConnectRPC-specific error translation and stream handling.

It is not responsible for:

- owning node, transport, diagnostics, workload, discovery, or data truth;
- composing domain state directly from internal packages;
- replacing domain-owned APIs as the owner of operator-facing semantics.

The package should remain a protocol-focused mapper over already-owned domain
models.
