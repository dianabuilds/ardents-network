# PW3-15: CGA-05 renew grants and separate channel classes

Status: ready-for-human
State: open
Labels: ready-for-human
Research class: R0 lifecycle implementation with security review

## Parent

`../PRD.md`

## What to build

Renew 30-day grants inside the 24-hour renewal threshold while preserving
strictly independent discovery, relationship-scoped data, capability-control
and future Application channel classes. Each class has distinct
IDs/secrets/generations/membership/replay/audit and policy admission.

## Acceptance criteria

- [x] Renewal returns a fresh bounded sender snapshot and preserves one
      idempotent operation across retries/restarts.
- [x] Cross-scope and cross-channel use is denied before cryptographic or replay
      processing.
- [x] Selector, key, generation, replay and audit separation is proven for
      every supported class.
- [x] Expiry degrades only the affected channel and reports stable redacted
      readiness/reason state.
- [x] Thirty-day validity, 24-hour renewal threshold and all cardinality/time
      bounds fail closed.
- [x] `channel.application` consumes policy supplied by DR-01; this slice does
      not invent conversation identity, group policy or messaging semantics.
- [x] Lifecycle integration/security checks pass and `Q` remains `no`.

## Blocked by

- PW3-14 / CGA-04 accepted.
- DR-01 creation/membership policy contract for `channel.application`.

## Comments

- Published as a blocked canonical slice. Application Messaging remains
  outside Authority ownership.
- 2026-07-28 predecessor and research gates satisfied:
  - the maintainer explicitly accepted CGA-04 implementation commit
    `cfbbacd2dcf9044c98f4d132c1d5b90743e030d7`;
  - the accepted DR-01 research result defines Operator-owned conversation
    creation/membership and consumes a generic `channel.application` authority
    lifecycle without assigning conversation identity, group policy or message
    semantics to Authority;
  - ADR-0015 remains Proposed. Admitting CGA-05 neither accepts that ADR nor
    implements Application Messaging, qualification, deployment change or
    push.
- 2026-07-28 CGA-05 implementation handoff:
  - exact starting commit:
    `ce0c23c5db534de9ec621e939a7612cbc33284c6`;
  - exact implementation tip:
    `c8f87f69ff27a14902a628822b49bab60fa0dd38`;
  - implementation commits:
    `9cf8394`, `176958f`, `f228951`, `c8f87f6`;
  - protected Operator `RenewChannelGrants` reuses the accepted
    `realm.channel.generation.rotate` action on the exact channel. It admits
    only unexpired current grants with at most 24 hours remaining, issues
    fresh IDs/secrets and generation, fixes validity at 30 days, and retains
    one restart-idempotent `channel_renewal` operation with a complete bounded
    sender snapshot;
  - discovery, data exchange, Application and capability-control channels now
    have independent Channel IDs, secrets, selectors, envelope keys,
    generation/membership state, capability-derived replay namespaces and
    class/generation audit attribution. The accepted v1 selector/KDF and
    private-envelope wire format remain unchanged;
  - cross-channel routing fails before envelope decryption, and cross-scope
    use fails at the supported Channel capability resolver before envelope or
    replay admission. Product Policy class denial precedes attestation
    verification and cannot replay a retained secret-bearing result;
  - `channel.application` is only a generic lifecycle class admitted by the
    DR-01 Product Policy seam. Authority owns no conversation identity, group
    policy, recipient selection or message semantics;
  - exact-channel `InspectChannel` reports only redacted class, generations,
    member count, expiry, renew-by and stable readiness/reason values. Expiry
    has priority over pending/runtime-adoption state and affects neither
    sibling Channel readiness nor Realm readiness;
  - existing schema-v1 rotations remain readable and ordinary rotation and
    membership retries preserve their pre-CGA-05 payload hashes. A Principal
    already counted at the Realm limit can still join an independent sibling
    channel without exceeding the Realm-member bound;
  - the protected-process integration completes renewal through delivery
    install, acknowledgement, checkpoint activation, runtime adoption and
    terminal active acknowledgement;
  - commit-bound validation passed:
    - `go test ./... -count=1`;
    - `go test -race ./internal/authority
      ./internal/identity/capability ./internal/messaging
      ./internal/channeldelivery ./internal/localapi/authority
      ./internal/localapi/channeldelivery -count=1`;
    - `go test -tags=integration ./tests/integration/localapi
      ./tests/integration/messaging -count=1`;
    - `go vet ./...`;
    - `scripts/generate-api.ps1 -Check`;
    - `go run ./tests/tooling/capabilitycatalog -check`;
    - `go test ./tests/tooling/archaccept
      ./tests/tooling/doccontract -count=1`;
    - `git diff --check ce0c23c...HEAD`;
  - independent repeat Standards and Spec reviews on the exact implementation
    tip both returned `PASS` with no remaining actionable findings;
  - the capability catalogue remains `24 capabilities, 8 domains, 0
    qualified`; canonical qualification is unchanged and `Q=no`;
  - ADR-0015 remains Proposed. CGA-06 remains gated on explicit maintainer
    acceptance of the exact CGA-05 implementation tip. No production
    deployment was changed and nothing was pushed.
- 2026-07-28 maintainer disposition: accepted exact implementation commit
  `c8f87f69ff27a14902a628822b49bab60fa0dd38` after the commit-bound handoff
  and clean repeat Standards/Spec reviews. CGA-06 may consume this commit;
  later qualification remains governed by its own matching-commit gates.
