# R-056 direct-binary and browser handoff model

This disposable logic prototype answers one decision-relevant question from
[R-056](../../docs/research/records/r-056-stage-7-desktop-browser-integration.md):

> Is one Ardents executable with first-class `connect`/`accept` byte-stream
> commands and a `browse` Service-Link handoff sufficient for Installed and
> Portable use, while browser integration remains optional and never changes or
> bypasses the host's Internet/VPN policy?

It is a state model, not a browser, isolation mechanism, installer, Endpoint,
or Stage 7 implementation. It creates no registration, network connection,
profile, protected state, package, or project API.

## Hypothesis and falsification

The retained O1 topology is internally sufficient if every modeled state keeps
these invariants:

1. direct `connect` and `accept` remain available in both Distribution Profiles
   without a browser or URI registration;
2. explicit `browse` remains available without URI registration, while OS
   handoff requires a separately requested per-user registration;
3. generic browser use is always `application-networking-unverified`;
4. every Stage 7 isolated-browser request returns `isolation-unsupported`
   without launching a listener/browser or falling back to generic mode;
5. blocked Carrier networking is reported unavailable and never becomes direct
   fallback; and
6. no action mutates DNS, routes, system proxy, default browser, or VPN policy.

The hypothesis is falsified if the state machine needs an installer-only
runtime feature, makes direct binary operation depend on browser state, silently
downgrades isolated to generic, bypasses a blocked Carrier, or needs ambient
network changes to complete an Ardents action.

## Run

Requirements: the repository's selected Go toolchain. From the repository root:

```text
go run ./experiments/r-056-stage-7-desktop-browser-integration/main.go ./experiments/r-056-stage-7-desktop-browser-integration/session.go
```

Enter one command at a time. `h` shows the controls. A deterministic walkthrough
can be run without interaction:

```text
go run ./experiments/r-056-stage-7-desktop-browser-integration/main.go ./experiments/r-056-stage-7-desktop-browser-integration/session.go c p c u o m o v o v o
```

## Inputs, measurements, and evidence

Inputs are only symbolic states selected in the TUI: Distribution Profile,
optional URI registration, browser availability/mode, and whether current host
policy permits the Endpoint Carrier. No real account,
Service, Authority, VPN credential, browser history, or network data is read.

For each action, inspect the visible entry point, result class, claim ceiling,
fallback, and host-policy mutation inventory. Retained evidence is this source,
the exact scripted command above, and [NOTES.md](NOTES.md). Generated build
caches and console transcripts stay outside the repository.

## Actual result

Run on 2026-08-20 with Go `1.26.6 windows/amd64`. The executed `main.go` SHA-256
was `217a3c364749b3a12b9906c73d50b730de4ad4c4056d5213f0382cb7c6acdc78` and
`session.go` was
`c533713f9daf1075186ded4a0876cc7cf842a24f93cfd7e672277c9f6c6751d8`.
The scripted walkthrough and negative sequence recorded in `NOTES.md` exited
`0` with no invariant failure.

Direct `connect` and `accept` ignored Distribution Profile, browser, and URI
registration state. Direct `browse` worked without registration; OS handoff did
not. Missing default browser stopped only generic browsing. Every isolated-
browser request returned `isolation-unsupported`, including under permitting
VPN and blocked-Carrier states, without generic fallback. A permitting VPN state
was preserved, while blocked Carrier state returned evidenced Route unavailable
for direct/generic Carrier use. Every failure retained `fallback=none`, and only
the explicit registration action reported one scoped host change.

This result supports logical sufficiency but cannot prove URI quoting, browser
launch flags, loopback-origin confinement, OS isolation, cleanup, or exact
platform behavior.

## Disposition

Retain temporarily as R-056 design evidence. Delete it, or absorb only the
accepted contract into maintained Stage 7 commands, after the platform
experiment answers R-056. Do not promote this package or its names as a project
API.
