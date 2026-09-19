# R-156 — Admission ownership and failed rollback

Baseline: 1343683e3df319de457dc8e78c8e2d93f8393d0a, 2026-09-14.

Decision question: do the local interfaces enforce unique admission ownership
and preserve failure of compensating cleanup, or must every caller know more
than the interface promises?

Before execution, the expected invariants are:
1. Copying the value returned by admission cannot create two live forwarding
   owners of the same reserved channel. Cancelling one owner cannot leave
   another accepting bytes after aggregate reservation release.
2. If admission refuses after obtaining host capacity, a failed capacity
   release remains distinguishable in the returned local error.

Falsification: either probe violates its corresponding invariant. These are
assertion tests and should fail on the suspect implementation.

Copy admission_ownership_test.go.txt into
internal/route/r156_admission_ownership_test.go in a clean temporary extraction,
then run:
```text
go test ./internal/route -run '^TestR156Admission' -v -count=1 -timeout=30s
```

The first probe starts from a real internal duty reservation, as maintained
forwarding tests do. The second uses real admission and spend owners with
explicit exporter/verifier/release test callbacks; it is not cryptographic
verification or an installed hosting filesystem fault. Neither proves that
an external peer can copy a Go value, or that the current Node caller does so.

Linux Endpoint issuance recovery is inspected separately in R-156. These
portable probes do not run it or qualify an installed Ubuntu network.

Result on 2026-09-14: both assertion probes failed (exit 1), reproducing the
two local interface defects. Go 1.26.8 windows/amd64; GOPROXY=off, GOSUMDB=off;
cached dependencies, immutable source extraction.

Observed:
- The copied admission created a second forwarding owner. After first Cancel:
  channels=0 children=0 second_accept=<nil>.
- Failed Release was called once, but the returned error was only
  "closed admission token is unavailable".

Raw output outside Git:
C:\Users\vitek\AppData\Local\Temp\ardents-r156-state-515ce606bbb340de975743886cb782bb\snapshot\r156-admission-output.txt
SHA256: a6c0dae957e0520e3d071b9102b675044f48c4f799ac210e544f239c080f2029

Control execution passed (exit 0) on the same extraction:
```text
go test ./internal/route -run '^(TestClosedAdmissionChannelBindsHELLOExporterBeforeBurningToken|TestClosedAdmissionHostRefusalDoesNotBurnToken|TestClosedForwardingChannelBoundsAuthorizedOddChild)$' -v -count=1 -timeout=30s
```

The three controls check existing admission binding, receiver host refusal,
and ordinary forwarding ownership. They do not cover copied admissions or
failed compensating release. The receiver-side retry control also does not
prove that a real Endpoint can replay an already presented token.

Disposition: keep reproducible research evidence. Fixes belong to bounded
maintained changes with regression tests at the real caller seam. No production
code, integration check, race run, installed Ubuntu qualification or remote
exploit claim is supplied by these probes.



## Freshness recheck

Repeated on dead4efc922cd97400837c960bb2eaea8b51d49c, 2026-09-14.
The former non-replenishing constructor was removed. Copy
admission_ownership_current_test.go.txt instead: it exercises the remaining
NewReplenishableClosedForwardingChannel with an unused explicit refill callback.
No admission-transfer logic was changed by the probe.

Both assertion probes still FAIL (exit 1) with the same observed defects.
The original probe and its original receipt above are preserved.
Raw result:
C:\Users\vitek\AppData\Local\Temp\ardents-r156-sequence-39aea9097a1f44d5bb3df3f73de76163\snapshot\r156-admission-current-output.txt
SHA256 b140f26ff26dc25836f9fa45a99650d20aa0ad5c783be11e8ade954102382b35.

Maintained repair tracking: issues #71 and #72. Later working-tree changes
are not covered by this execution.
