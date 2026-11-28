# R-172 candidate checks

The question is whether the pinned public Go candidate delivers only the bytes admitted for a path probe, and can use the current Ardents Node transport authentication. The release still requires signed QUIC duties to refuse before effects under ADR-0122.

## Criteria defined before execution

- With an explicit allowance, the real candidate packer, send queue and UDP writer must deliver the expected payload length to the selected loopback socket. Insufficient allowance must produce no datagram. The default peer must receive none. Exercise native UDP and the generic PacketConn backend. Any extra, misdirected or oversized output fails the check.
- The ordinary candidate connection must negotiate the current Route TLS profile. The shared classifier must distinguish Node and direct principals and reject an unauthorized Node key. An authorized client and server must exchange identical application bytes.
- The current Linux Node process must preserve its signed QUIC refusal and pre-effect controls.

The send check starts with a supplied budget. It isolates the accounting-to-socket boundary. The real candidate connection is tested separately. Neither test sends network traffic to addresses outside its own loopback sockets.

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
