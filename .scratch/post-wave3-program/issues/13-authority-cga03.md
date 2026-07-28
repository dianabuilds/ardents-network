# PW3-13: CGA-03 rotate a channel and attest activation

Status: ready-for-agent
State: open
Labels: ready-for-agent
Research class: R1 multi-host lifecycle with security review

## Parent

`../PRD.md`

## What to build

Rotate one channel end to end: create a fresh pending generation, deliver it to
every approved member, append the activation checkpoint/envelope, accept
active receipts, and retain the immediately previous generation as
receive-only for a bounded drain. Restarts resume the same phase and operation.

## Acceptance criteria

- [ ] Rotation always creates a fresh selector/key and never publishes with
      the previous generation after activation.
- [ ] Current, pending and one receive-only previous generation have explicit,
      bounded, versioned member and authority state.
- [ ] A member missing the activation checkpoint is not ready even if it has
      installed the pending secret.
- [ ] Crashes at every ledger/checkpoint/delivery/activation boundary resume
      one operation without a second generation or audit identity.
- [ ] Suspect or noncompliant members require deployment fencing; a valid MAC
      alone cannot establish honest activation.
- [ ] Only one pending generation is admitted and post-activation recovery
      rolls forward rather than rolling authority sequence back.
- [ ] Redaction, stable errors, restart, race and multi-host integration checks
      pass without changing `Q`.

## Blocked by

- PW3-12 / CGA-02 accepted with commit-bound evidence.

## Comments

- Published as the canonical successor slice. It remains `needs-info` until
  CGA-02 is accepted.
- 2026-07-28 predecessor gate satisfied: the maintainer explicitly accepted
  CGA-02 implementation commit
  `693ac7cb0e88661dccce8a97482ae14d53a5afd9`. CGA-03 is admitted for
  implementation; no CGA-04 behavior, qualification, deployment change or push
  is implied.
