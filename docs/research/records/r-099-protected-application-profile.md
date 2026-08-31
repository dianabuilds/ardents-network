---
id: R-099
title: First protected application profile
status: open
owner: Product Owner and Codex
started: 2026-08-24
reviewed: 2026-08-24
---

# R-099 — Is there one narrow application job and supported platform on which an OS-enforced or brokered boundary can deny ordinary-network escape well enough to support a bounded Application-level location claim?

## Decision this unlocks

Select one H4-7A Reference Application/platform experiment, or defer protected
mode while retaining the useful generic Browser Adapter profile.

## Current contract

- Generic local IPC/loopback and browser adapters are unqualified; they do not
  constrain a browser/application's ordinary networking.
- A claim-bearing boundary covers an Application Principal and complete helper
  process tree, scoped Ardents attachment, ordinary ingress/egress denial, and
  isolated origin/cache/storage.
- No Windows or Ubuntu protected profile/mechanism is selected.

## Hypotheses

- **H1:** a deterministic one-response Reference Site/client on one platform
  can be enclosed and falsified with an attainable maintenance burden.
- **H2:** a native application helper is viable but a general browser is not.
- **H0:** no claim-bearing local isolation profile is viable for the actual
  team; protected mode must be deferred.

## Evaluation criteria

- exact application tree, grant, storage, allowed Ardents path and claim;
- DNS, socket, HTTP, WebRTC, QUIC, callback/SSRF, listener, child-process,
  malicious-sibling, cache and restart/revocation escape tests;
- supported OS primitives, bypass/failure behavior, update burden and cleanup;
- conditions and limitations for the claim, including endpoint compromise.

## Evidence plan

### Primary sources

- Official Windows and Ubuntu OS isolation, process, firewall/network namespace,
  sandboxing, and application-launch documentation after a job is selected.
- Current threat model, H4-7 and Application Principal contracts.

### Experiment

Choose one job and one platform first. Build a disposable harness that attempts
every denied path while independently observing host networking and process
tree. A failed escape test rejects the selected profile/claim.

### Failure scenarios

- A child/helper, browser component, DNS prefetch, WebRTC, callback or direct
  socket exits the boundary.
- A same-user sibling reaches grant/storage/loopback state.
- Restart/revocation leaves an old process or cache authorized.

## Findings

- **Current-contract fact:** R-096's leading generic-browser candidate is an
  explicit Endpoint-to-loopback handoff. It deliberately changes neither the
  browser's ordinary network path nor its process tree, so it cannot satisfy
  this record's ordinary-egress-denial requirement.
- **Inference:** selecting an OS sandbox before a concrete H4-3 workload would
  reverse the required order and create a generic isolation framework with no
  bounded claim to test. No platform research or mechanism selection starts
  until the Product Owner chooses a valuable, exact protected job after the
  ordinary H4-3 path is evidenced.
- **Current-contract fact:** H4-7A requires the complete first job, process
  tree, storage behavior, destination set, user journey, and claim before an
  operating-system mechanism is chosen. H4-7C separately makes a protected
  browser conditional on evidence from both the generic browser path and a
  narrow protected profile. [C0 Application boundary](../../product/scope.md#c0-application-and-browser-candidate)
  (inspected 2026-08-24).
- **Inference:** there is no present protected job: the R-096 fixture proves
  only static rendering on a loopback origin, and it deliberately forbids the
  scripts, helpers, external resources, and broad browser behavior that would
  determine an isolation mechanism's attack surface. A Windows Sandbox,
  AppContainer, Linux namespace, firewall, browser extension, or proxy
  experiment now would therefore test an arbitrary mechanism instead of the
  H4-7 claim.

## Options

1. Defer protected mode; retain generic alpha compatibility.
2. One deterministic Reference Application on one OS.
3. General protected browser profile.

## Recommendation

Choose option 1 for the current alpha: retain generic Browser Adapter
compatibility and make no Application-level location claim. H4-7A may begin
only when the Product Owner names one valuable exact job after H4-3 evidence.
Its record must choose one OS, enumerate the complete process tree and storage
surface, and state the allowed local attachment before any sandbox mechanism is
researched. Option 3 is not a default candidate.

## Disposition

Deferred behind a selected job/platform, with the generic-browser boundary
confirmed. Promotion requires a selected job/platform and a falsifiable
experiment; it cannot promote an OS primitive alone into a privacy claim.
