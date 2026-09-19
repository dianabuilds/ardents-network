# R-156 — Realistic consumer completion policy

Baseline: dead4efc922cd97400837c960bb2eaea8b51d49c, 2026-09-14.

Question: does lost-Terminal recovery still complete if each local consumer
ends its stream context immediately after its own successful RunBounded?
The normal Endpoint reader closes its Service stream after document completion.

Predeclared oracle: both peers complete the same one-shot byte exchange despite
the selected lost receipt/confirmation, with no Application replay. A failure
falsifies this composition expectation, not the native protocol in isolation.
The existing test retains both terminal tails until both peers' success is seen.

The probe reuses that real native fault adapter and carrier/continuity owners,
changes each consumer to its own child context and cancels only that context
after local success. It does not cancel both peers together or change the wire.
It is a model of Endpoint completion policy, not an installed Linux worker test.

Copy terminal_consumer_test.go.txt to
internal/service/connection/r156_consumer_terminal_test.go in a clean baseline
extraction; run:
go test ./internal/service/connection -run '^TestR156CloseAfterLocalTerminalSuccess$' -v -count=1 -timeout=20s

Control:
go test ./internal/service/connection -run '^TestRunBoundedRecoversLostTerminalReceiptAndConfirmation$' -v -count=1 -timeout=20s

Result: after fixing one unused import in the research harness, both the new
consumer-policy probe and the existing lost-receipt/confirmation control passed
(exit 0), Go 1.26.8 windows/amd64, GOPROXY=off, GOSUMDB=off. The first build
failed before any behavioral test ran; it is retained separately.

This specific schedule does not reproduce the suspected consumer-lifetime
failure. No new bug or Terminal redesign is justified by this result. It is
not coverage of every timing, separate final-confirmation loss, Linux Endpoint
cleanup or installed worker lifecycle.

Evidence outside Git, under:
C:\Users\vitek\AppData\Local\Temp\ardents-r156-sequence-39aea9097a1f44d5bb3df3f73de76163\snapshot
- r156-terminal-first-build.txt: initial unused-import build failure.
- r156-terminal-consumer-output.txt: behavioral probe and control PASS.
  SHA256 337cb1e68543036213bc24a840051f5ac2400ff41d22aae4dc39637bd91c45be.

Disposition: retain the narrow negative finding as research evidence; ordinary
consumer qualification remains in the existing full lifecycle acceptance.
