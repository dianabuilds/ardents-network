# OCS-05: Prove Principal access administration procedures

Status: ready-for-agent
State: closed
Labels: ready-for-agent
Research class: R1

## Parent

`../PRD.md`

## What to build

Prove supported Operator procedures for later Principal enrollment, grant
list/issue/revoke, device revocation, Delegation revocation import, and session
login/status/logout with redacted, attributable, retry-safe outcomes.

## Acceptance criteria

- [x] Exact Identity actions and request IDs are asserted.
- [x] SSH uses stream-local forwarding.
- [x] Secrets and protected paths remain redacted.
- [x] Logout distinguishes local cleanup from unconfirmed server invalidation.
- [x] Offline custody remains covered by unit evidence.

## Blocked by

- OCS-01

## Comments

- Completed in `8b9f8ad`.
