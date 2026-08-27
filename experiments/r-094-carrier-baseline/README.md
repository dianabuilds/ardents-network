# R-094 — TCP/TLS and QUIC carrier baseline

## Question

Can TCP/TLS and QUIC v1 expose the same local experiment oracle: reciprocal
TLS authentication, equal exporter material, and one ordered bidirectional
byte transcript?

## Hypothesis

For each transport, separate synthetic server/client certificates signed by an
ephemeral test CA authenticate both peers; both sides derive identical TLS
exporter bytes; one client-to-server transcript returns one exact response; and
a client certificate from an untrusted CA is rejected before a byte lane is
accepted. QUIC uses only one bidirectional stream, with 0-RTT and datagrams
disabled.

## Run

```sh
go run main.go
```

The experiment listens only on an ephemeral `127.0.0.1` port. It creates all
keys and certificates in memory, writes no files, uses no Ardents Authority or
Network State, and does not contact a public host.

## Evidence

Record the exact `key=value` output, toolchain, OS, and module version in
R-094. A passing local result does not prove Carrier seam design, State profile
authority, failover, UDP-blocked recovery, MTU/NAT behaviour, censorship
resistance, or production resource bounds.

## Disposition

Disposable research code only. It imports the already reviewed pinned
`quic-go v0.61.0` strictly for this experiment. Delete it when its unique
evidence is promoted or rejected.
