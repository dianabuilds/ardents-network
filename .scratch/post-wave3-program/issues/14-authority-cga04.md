# PW3-14: CGA-04 change membership with revocation and fencing

Status: ready-for-human
State: open
Labels: ready-for-human
Research class: R1 adversarial multi-host E2E

## Parent

`../PRD.md`

## What to build

Add or remove one member with a fresh channel generation, signed revocation,
candidate/active/suspended/removed states, survivor activation receipts and an
explicit deployment-fencing evidence boundary. Terminal results must remain
truthful under success, partition, timeout and malicious old traffic.

## Acceptance criteria

- [x] An added member never receives an old generation and a removed member
      never receives the next secret.
- [x] Removed senders fail authorization before replay processing.
- [x] Every membership mutation rotates the affected channel and preserves
      exact audit Actor/Effective attribution.
- [x] A missing survivor acknowledgement prevents completion unless valid
      deployment fencing evidence covers that approved host.
- [x] A receipt from an unapproved or suspect survivor never substitutes for
      fencing.
- [x] Candidate, active, suspended and removed transitions are versioned,
      bounded, idempotent and restart-safe.
- [x] Adversarial multi-host tests cover partition, stale traffic, forged
      receipt assertions, fencing and rejoin without premature qualification.

## Blocked by

- PW3-13 / CGA-03 accepted.
- Accepted DR-04 `DeploymentFenceEvidence` and supported
  reachability/fencing procedure.

## Comments

- Published as a blocked canonical slice. The Authority owns terminal
  membership truth; Deployment owns host isolation evidence.
- 2026-07-28 predecessor and boundary gates satisfied:
  - the maintainer explicitly accepted CGA-03 implementation commit
    `34dafff129e4dc26fe42932946df566e0295c84d`;
  - the accepted DR-04 research result defines the bounded, versioned
    `DeploymentFenceEvidence/v1` seam and supported fencing procedure consumed
    by this Authority slice;
  - ADR-0013 remains Proposed. Admitting CGA-04 neither accepts that ADR nor
    claims MR implementation, real-host deployment fencing, qualification,
    production deployment change or push.
- 2026-07-28 CGA-04 implementation handoff:
  - exact starting commit:
    `7794ea302ff7e45310c65d3b705ff17d2a815584`;
  - exact implementation tip:
    `cfbbacd2dcf9044c98f4d132c1d5b90743e030d7`;
  - implementation commits:
    `737f66b`, `69440c0`, `dbd332a`, `afdb6ec`, `cfbbacd`;
  - protected Operator-only membership change adds or removes exactly one
    member through a fresh channel generation. Add delivery includes the
    candidate without exposing any prior generation; remove delivery excludes
    the target and commits its signed grant revocation;
  - candidate, active, suspended and removed states are retained with an
    authority-sequence membership version. Rejoin after missed generations
    replaces the stable grant and clears any obsolete receive-only predecessor;
  - removal completes only when every survivor has an authenticated,
    deployment-approved active receipt or exact
    `DeploymentFenceEvidence/v1`, and the removed target is fenced. New
    evidence is bounded by the operation and drain deadlines; an identical
    retained evidence replay remains idempotent after expiry;
  - fencing uses the accepted ADR-0011
    `realm.channel.membership.change` action on the exact channel resource for
    both membership procedures. Audit records retain exact Actor, Effective
    and target Principal attribution;
  - admission reserves `2 * recipient_count + 3` audit and outbox records.
    Exact operation, rotation, member, control and evidence bounds reject
    before partial state can be admitted;
  - authority restart/crash tests cover both ledger/checkpoint boundaries for
    membership change and fence submission. Protected three-host integration
    covers add, partitioned remove, stale removed traffic before replay,
    forged receipt MAC, forged fence Actor, fencing and rejoin;
  - commit-bound validation passed:
    - `go test ./... -count=1`;
    - `go test -race ./internal/authority
      ./internal/identity/capability ./internal/messaging
      ./internal/channeldelivery ./internal/localapi/authority
      ./internal/localapi/channeldelivery -count=1`;
    - `go test -tags=integration ./tests/integration/localapi -count=1`;
    - `go vet ./...`;
    - `scripts/generate-api.ps1 -Check`;
    - `go run ./tests/tooling/capabilitycatalog -check`;
    - `git diff --check 7794ea3...HEAD`;
  - independent repeat Standards and Spec reviews on the exact implementation
    tip both returned `PASS` with no remaining actionable findings;
  - the capability catalogue remains `24 capabilities, 8 domains, 0
    qualified`; canonical qualification is unchanged and `Q=no`;
  - ADR-0013 remains Proposed. CGA-05 remains gated on explicit maintainer
    acceptance of the exact CGA-04 implementation tip. No production
    deployment was changed and nothing was pushed.
