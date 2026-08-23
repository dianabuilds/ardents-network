# R-093 cross-host native leg

This disposable tracer tests one synthetic `ardents-interactive-route-v1`
carrier leg across separate hosts. It proves neither a complete Ardents Route
nor a production Node, Docker profile, NAT traversal, anonymity, or public
network operation. Its research question and falsification rules are in
[`R-093`](../../docs/research/records/r-093-cross-host-native-leg.md).

Each participant creates its own ephemeral Ed25519 TLS identity. Copy only the
public certificate to the opposite host; never copy a private key or retain it
in the repository.

## Local sanity run

In two temporary directories, create identities:

```sh
go run experiments/r-093-cross-host-native-leg/main.go -mode identity \
  -certificate C:/temp/r093-server-cert.pem -key C:/temp/r093-server-key.pem
go run experiments/r-093-cross-host-native-leg/main.go -mode identity \
  -certificate C:/temp/r093-client-cert.pem -key C:/temp/r093-client-key.pem
```

Start the finite server, with the client public certificate configured:

```sh
go run experiments/r-093-cross-host-native-leg/main.go -mode server \
  -listen 127.0.0.1:44393 -certificate C:/temp/r093-server-cert.pem \
  -key C:/temp/r093-server-key.pem -peer-certificate C:/temp/r093-client-cert.pem
```

In another shell, run the client:

```sh
go run experiments/r-093-cross-host-native-leg/main.go -mode client \
  -address 127.0.0.1:44393 -certificate C:/temp/r093-client-cert.pem \
  -key C:/temp/r093-client-key.pem -peer-certificate C:/temp/r093-server-cert.pem \
  -payload 65536
```

Both commands emit one JSON result. A passing result has TLS 1.3, the exact
ALPN, one carried connection, and equal send/echo SHA-256 digests.

## Binding-refusal run

Run the same server with `-expected-rejections 1`, then make the client use a
changed attachment identifier:

```sh
go run experiments/r-093-cross-host-native-leg/main.go -mode server \
  -listen 127.0.0.1:44394 -certificate C:/temp/r093-server-cert.pem \
  -key C:/temp/r093-server-key.pem -peer-certificate C:/temp/r093-client-cert.pem \
  -accepted-connections 0 -expected-rejections 1
go run experiments/r-093-cross-host-native-leg/main.go -mode client \
  -address 127.0.0.1:44394 -certificate C:/temp/r093-client-cert.pem \
  -key C:/temp/r093-client-key.pem -peer-certificate C:/temp/r093-server-cert.pem \
  -binding changed-attachment -expect-rejection
```

The server must report one rejection and zero carried payloads. The client
must report a rejection, not an echo.

## Cross-host procedure

Build the binary with `go build -trimpath` to an external temporary directory.
Create the server identity on the VPS and the client identity locally. Exchange
only `*-cert.pem` files, open one dedicated high TCP port for the finite test,
then run the server on the VPS and the client locally using its public IP and
that port. Retain both JSON outputs and environment facts outside Git.

The current initial topology is deliberately outbound-only from the local
machine to the VPS. It does not require exposing a Windows listener through
NAT. Use a fresh port and remove its firewall rule after the run.

## Falsification and disposition

The run fails on any TLS/ALPN/peer-key mismatch, nonreciprocal binding,
incorrect echo digest, unexpected successful changed binding, timeout, or
listener that does not stop after its finite connection count. No generated
identity, build output, capture, credential, or result belongs in Git. Retain
this spike only while R-093 is open.
