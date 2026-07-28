# PW3-15: CGA-05 renew grants and separate channel classes

Status: ready-for-agent
State: open
Labels: ready-for-agent
Research class: R0 lifecycle implementation with security review

## Parent

`../PRD.md`

## What to build

Renew 30-day grants inside the 24-hour renewal threshold while preserving
strictly independent discovery, relationship-scoped data, capability-control
and future Application channel classes. Each class has distinct
IDs/secrets/generations/membership/replay/audit and policy admission.

## Acceptance criteria

- [ ] Renewal returns a fresh bounded sender snapshot and preserves one
      idempotent operation across retries/restarts.
- [ ] Cross-scope and cross-channel use is denied before cryptographic or replay
      processing.
- [ ] Selector, key, generation, replay and audit separation is proven for
      every supported class.
- [ ] Expiry degrades only the affected channel and reports stable redacted
      readiness/reason state.
- [ ] Thirty-day validity, 24-hour renewal threshold and all cardinality/time
      bounds fail closed.
- [ ] `channel.application` consumes policy supplied by DR-01; this slice does
      not invent conversation identity, group policy or messaging semantics.
- [ ] Lifecycle integration/security checks pass and `Q` remains `no`.

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
