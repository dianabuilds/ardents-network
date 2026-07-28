# PW3-14: CGA-04 change membership with revocation and fencing

Status: ready-for-agent
State: open
Labels: ready-for-agent
Research class: R1 adversarial multi-host E2E

## Parent

`../PRD.md`

## What to build

Add or remove one member with a fresh channel generation, signed revocation,
candidate/active/suspended/removed states, survivor activation receipts and an
explicit deployment-fencing evidence boundary. Terminal results must remain
truthful under success, partition, timeout and malicious old traffic.

## Acceptance criteria

- [ ] An added member never receives an old generation and a removed member
      never receives the next secret.
- [ ] Removed senders fail authorization before replay processing.
- [ ] Every membership mutation rotates the affected channel and preserves
      exact audit Actor/Effective attribution.
- [ ] A missing survivor acknowledgement prevents completion unless valid
      deployment fencing evidence covers that approved host.
- [ ] A receipt from an unapproved or suspect survivor never substitutes for
      fencing.
- [ ] Candidate, active, suspended and removed transitions are versioned,
      bounded, idempotent and restart-safe.
- [ ] Adversarial multi-host tests cover partition, stale traffic, forged
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
