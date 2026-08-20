# R-056 prototype notes

## Walkthrough

Run on 2026-08-20 from the repository root:

```text
go run ./experiments/r-056-stage-7-desktop-browser-integration/main.go ./experiments/r-056-stage-7-desktop-browser-integration/session.go c p c u o m o v o v o
go run ./experiments/r-056-stage-7-desktop-browser-integration/main.go ./experiments/r-056-stage-7-desktop-browser-integration/session.go o b d b m b v v c a
```

Both commands exited `0`; the model reported no invariant failure. Observed
cells included direct operations in both profiles, direct browse without
registration, OS handoff with/without registration, missing default browser,
explicit unsupported isolated-browser results under ordinary/VPN/blocked
Carrier states, and direct `connect`/`accept` under a Carrier block.

## Provisional answer

The model contains only one executable-facing product surface:
direct raw-stream operations and an explicit Service-Link browser handoff. The
Installed/Portable distinction does not occur in any runtime result predicate.
URI registration is a separate optional per-user convenience, not a Portable
artifact or installer dependency.

Generic browsing is never claim-bearing. Isolated browsing has no generic
fallback and is explicitly unsupported in Stage 7. Host networking policy is
an input to Carrier availability, never a setting the Adapter may change. The
provisional answer is yes: this topology is logically sufficient without an
installer-only client or browser-owned network stack.

## Threats to validity

This is a pure state machine. It does not validate command-line quoting, OS URI
activation, loopback authorization/origin behavior, hostile same-user
processes, unsupported-request side effects, VPN coexistence, or cleanup. Those
remain mandatory platform-experiment cells.
