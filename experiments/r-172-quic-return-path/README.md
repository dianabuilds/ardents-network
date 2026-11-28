# R-172 candidate checks

The question is whether the pinned public Go candidate delivers only the bytes admitted for a path probe, and can use the current Ardents Node transport authentication. The release still requires signed QUIC duties to refuse before effects under ADR-0122.

## Criteria defined before execution

- With an explicit allowance, the real candidate packer, send queue and UDP writer must deliver the expected payload length to the selected loopback socket. Insufficient allowance must produce no datagram. The default peer must receive none. Exercise native UDP and the generic PacketConn backend. Any extra, misdirected or oversized output fails the check.
- The ordinary candidate connection must negotiate the current Route TLS profile. The shared classifier must distinguish Node and direct principals and reject an unauthorized Node key. An authorized client and server must exchange identical application bytes.
- The current Linux Node process must preserve its signed QUIC refusal and pre-effect controls.

The send check starts with a supplied budget. It isolates the accounting-to-socket boundary. The real candidate connection is tested separately. Neither test sends network traffic to addresses outside its own loopback sockets.

## Return-path criteria defined before execution

The oracle is [RFC 9000 section 8](https://www.rfc-editor.org/rfc/rfc9000.html#section-8):
after receiving packets from an address that is not yet validated, an endpoint
MUST limit the data it sends to the unvalidated address to three times the
data received from it. Section 8.2.1 permits a smaller probe only when that
limit forbids 1200 bytes and requires a second expanded validation afterwards.

`return_path_test.go.in` runs one real TLS/QUIC session. The client is the
implementation's own client with the current Ardents Node TLS configuration.
A user-space NAT owns two sockets in front of it, as in the historical probe.
After a quiet handshake and echo on the first mapping, the NAT moves the client
to the second mapping and keeps everything the client writes. It then delivers
unchanged client packets from the second mapping, one at a time. The server
sees a new peer address that never answers. The test reads the server's
datagrams at both NAT sockets and counts complete UDP payload bytes.

- **E1, the historical case.** The first delivered datagram is a genuine
  1-RTT packet of 32 bytes. Before validation, the server must send at most 96
  bytes to the rebound mapping in total, and it must send something there.
  A larger total, a reply on the original mapping instead, or no reply fails.
- **E2, cumulative.** Three further 32-byte packets arrive while the address
  stays unvalidated. After each one the total sent to the rebound mapping must
  stay within three times the total received there.
- **E3, padding is not removed.** A separate session delivers one packet of
  about 400 bytes. The limit then exceeds 1200, so the server must send one
  datagram of at least 1200 bytes and no more than the limit.
- **E4, validation and padding control.** A separate session moves the client
  and keeps forwarding. Its first packet from the new address is again 32
  bytes. The first reply must be within 96 bytes, the server may send 1200
  bytes only after it receives a PATH_RESPONSE, it must move to the rebound
  mapping, and the application exchange must continue there.
- **E5, both writers.** E1–E4 hold with the server's native UDP writer and
  with a wrapper that selects its generic PacketConn writer.

The same source runs against the pinned official releases. A control is
discriminating when each official release fails E1 by exceeding the limit and
the candidate passes. A control that satisfies the bound, a candidate run that
fails any criterion, or a run that fails for another reason is recorded with
its output and is not counted as a pass.

## Return-path run

From the checkout run `python3 experiments/r-172-quic-return-path/run.py
return-path --module candidate`. The runner renders `return_path_test.go.in`
into a temporary module that imports the current Ardents transport owners and
verifies the published checksum first. `--module v0.62.0`, `v0.63.0` or
`v0.66.0` runs the unchanged source against a pinned official release;
`--race`, `--backend` and `--case` narrow one run. `run.py controls` runs all
three official releases and succeeds only when each fails the 32-byte cases by
exceeding the limit. The plain `run.py` keeps the socket, library and Node TLS
checks and then runs the candidate return path.

Nothing in the return-path test replaces an implementation part. The server is
the implementation's own listener with the current `SharedServerTLS`
configuration, budget, packet packer, send queue and UDP writer; the client is
its own dialer with `NodeClientTLS`. Each endpoint's tracer reports the sizes
and frame types it received and packed, next to the relay's socket counts.
The test supplies self-signed Ed25519 certificates, a local Node key verifier,
an echo application and the relay, which forwards, withholds and releases
datagrams without changing a byte. Both endpoints use loopback sockets. The
relay hides the client's UDP socket type, so only the server's two writers are
compared. Path MTU discovery is disabled in both configurations.

## Run

From the checkout, with Python 3 and the repository Go toolchain, run
`python3 experiments/r-172-quic-return-path/run.py`. The runner checks the
published module checksum, adds a test to a temporary source copy, and uses
a separate temporary module to import the current Ardents transport owners.
It needs the pinned dependencies and permission to bind local UDP sockets.

The send fixture uses the candidate's test sealer and packet-number fixtures.
Both UDP sockets are bound locally. The client interoperability check uses real
TLS, the library's ordinary path-probe and switch APIs, and a bidirectional stream.
The second client socket counts ordinary UDP payload bytes through completion of
the path probe. A cumulative reply greater than three times the client's sends
at any observed read, or an empty exchange, fails the check. The server uses its
native socket backend. This interval uses a compliant client probe and does not
identify the server's internal address-validation instant.

For the Linux process control, run
`go test -count=1 -v -run '^TestRouteRefusesSignedQUICBeforeEffects$' ./tests/e2e/routeprocess`.
[Measured results and disposition](../../docs/research/records/r-172-quic-return-path-budget.md).

The 96-byte output case corresponds to the RFC allowance for the historical
32-byte input while the new address is still unvalidated. The native and
generic writers must deliver exactly 96 bytes to the selected port and zero
to the original port in this one-response fixture. The runner also executes
the pinned module's two existing server budget and shared-allowance tests.
Their mock unpacker and packer isolate the input-side calculation. The
output case supplies the grant explicitly.
