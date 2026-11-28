# R-172 candidate checks

The question is whether a Go QUIC implementation bounds its reply to a small packet that arrives from a new address before that address is validated, from the server's receive path through its budget, packet packer, send queue and UDP writer, and whether it can use the current Ardents Node transport authentication. Two implementations are measured: the pinned public fork `qoke/mp-quic-go v0.3.1`, and official quic-go v0.66.0 with the retained correction. Until the return of QUIC under ADR-0136 is qualified (#1081, #1082), the release still requires signed QUIC duties to refuse before effects.

The hypothesis is that the candidate keeps the reply within [RFC 9000 section 8](https://www.rfc-editor.org/rfc9000.html#section-8) where the official releases do not. It is falsified by any reply to the unvalidated address above three times the payload received there, by a missing reply, by a reply on the original address instead, by an expanded probe before validation, or by a session that fails to continue after a validated rebinding.

## Criteria defined before execution

- **Supplied grant.** With an explicit allowance, the real candidate packer, send queue and UDP writer must deliver the expected payload length to the selected loopback socket. Insufficient allowance must produce no datagram. The default peer must receive none. Exercise native UDP and the generic PacketConn backend. Any extra, misdirected or oversized output fails the check.
- **Node authentication.** The ordinary candidate connection must negotiate the current Route TLS profile. The shared classifier must distinguish Node and direct principals and reject an unauthorized Node key. An authorized client and server must exchange identical application bytes.
- **Signed refusal.** The current Linux Node process must preserve its signed QUIC refusal and pre-effect controls.
- **E1, the historical case.** A genuine 1-RTT packet of 32 bytes is the first datagram the server receives from a new address. Before validation, the server must send at most 96 bytes to that address in total, and it must send something there.
- **E2, cumulative.** Three further 32-byte packets arrive while the address stays unvalidated. After each one the total sent to the new address must stay within three times the total received there.
- **E3, padding is not removed.** A separate session delivers one packet of 404 bytes. The limit then exceeds 1200, so the server must send one datagram of at least 1200 bytes and no more than the limit.
- **E4, validation.** A separate session moves the client and keeps forwarding. Its first packet from the new address is again 32 bytes. The first reply must be within three times the payload the client had sent there when it arrived, the server may send 1200 bytes only after it receives a PATH_RESPONSE, it must move to the new address, and the application exchange must continue there. This runs for a Node client with a certificate and for a certificate-free direct role client.
- **E5, idle keep-alive.** The retired Ardents QUIC configuration sent a PING every second on an idle connection. With that period, the first packet from the new address is a keep-alive PING of 25 bytes. The first reply must be within 75 bytes.
- **E6, both writers.** E1–E5 hold with the server's native UDP writer and with a wrapper that selects its generic PacketConn writer.
- **E7, the official release with the correction.** Official quic-go v0.66.0 with `quic-go-v0.66.0-probe-budget.patch` must pass E1–E6 with the same sizes as the candidate, and upstream's own tests must pass on the patched tree.

The same source runs against pinned official releases. A control is discriminating when each official release fails E1, E4 and E5 by exceeding the limit and the candidate passes. A control that satisfies the bound, a candidate run that fails any criterion, or a run that fails for another reason is recorded with its output and is not counted as a pass.

## Run

From the checkout, with Python 3 and the repository Go toolchain, run
`python3 experiments/r-172-quic-return-path/run.py`. It needs the pinned
dependencies and permission to bind local UDP sockets, and it keeps builds and
caches outside the repository. The runner checks the published module
checksum, adds a test to a temporary source copy of the candidate, and uses a
separate temporary module to import the current Ardents transport owners.

- The supplied-grant fixture uses the candidate's test sealer and packet-number fixtures. Both UDP sockets are bound locally. The runner also executes the pinned module's two existing server budget and shared-allowance tests. Their mock unpacker and packer isolate the input-side calculation; the output fixture supplies the grant explicitly. The 96-byte output case corresponds to the RFC allowance for the historical 32-byte input.
- The Node authentication check uses real TLS, the library's ordinary path-probe and switch APIs and a bidirectional stream. The second client socket counts UDP payload bytes through completion of the path probe. The server uses its native socket backend. This interval uses a compliant client probe and does not identify the server's internal address-validation instant.
- `run.py return-path --module candidate` renders `return_path_test.go.in`. `--module v0.62.0`, `v0.63.0` or `v0.66.0` runs the unchanged source against a pinned official release; `--race`, `--backend` and `--case` narrow one run. `run.py controls` runs all three official releases and succeeds only when each fails the 32-byte and 25-byte cases by exceeding the limit.
- `run.py patched` downloads official quic-go v0.66.0, checks its published checksum, applies `quic-go-v0.66.0-probe-budget.patch` to a temporary copy with the runner's own context-exact applier and runs the return-path test against it. The applier refuses any hunk whose context does not match, so the run also proves the identity of the base.
- For the Linux process control, run
  `go test -count=1 -v -run '^TestRouteRefusesSignedQUICBeforeEffects$' ./tests/e2e/routeprocess`.

## Captured evidence

In the return-path test a user-space NAT owns two loopback sockets in front of the implementation's own client. After a quiet handshake and echo on the first mapping it moves the client to the second mapping. It then either withholds everything the client writes and delivers unchanged client packets one at a time, so the server sees a new peer address that never answers, or keeps forwarding. The relay reads the server's datagrams at both sockets and counts complete UDP payload bytes: the QUIC header, its authentication data and padding, without IP and UDP headers. Each endpoint's tracer reports the sizes and frame types it received and packed.

Nothing in the return-path test replaces an implementation part. The server is the implementation's own listener with the current `SharedServerTLS` configuration; the client is its own dialer with `NodeClientTLS` or `RoleClientTLS`. The test supplies self-signed Ed25519 certificates, a local Node key verifier, an echo application and the relay, which forwards, withholds and releases datagrams without changing a byte. The relay hides the client's UDP socket type, so only the server's two writers are compared. Path MTU discovery is disabled in both configurations. No test sends network traffic to addresses outside its own loopback sockets.

## Result

[Measured results and disposition](../../docs/research/records/r-172-quic-return-path-budget.md).

## Disposition

The candidate and the patched official release pass E1–E7 on macOS arm64 and on the Linux x86-64 stands; each unpatched official release fails E1, E4 and E5 there with a 1200-byte reply. The candidate corrects the specific finding and is not a library Ardents can select: its maintenance and support record fails the dependency acceptance rule. The same correction is retained as a patch of three production files on the official release, so that it can be offered upstream or carried as a minimal fork by ADR. On 2026-11-28 the Product Owner kept TCP/TLS as the Carrier and then stated that a need for QUIC exists; the route is to be decided. The experiment stays as the regression for either route.
