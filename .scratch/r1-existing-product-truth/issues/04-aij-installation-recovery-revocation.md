# AIJ-02: Prove installation recovery and revocation

Status: ready-for-agent
State: closed
Labels: ready-for-agent
Research class: R1

## Parent

`../PRD.md`

## What to build

Drive the real Operator CLI and public Application SDK through ticket
replacement, same-ticket transactional retry, protected-file cleanup,
Node/Application restart, existing-content retrieval, grant revocation, and
device revocation so the supported installation lifecycle has distinct,
observable outcomes.

## Acceptance criteria

- [x] The process tracer bullet uses real Operator and Application interfaces.
- [x] Ticket replacement and safe same-ticket retry are covered.
- [x] Restart preserves committed enrollment, Credential, grants, and content.
- [x] Grant and device revocation produce distinct supported outcomes.
- [x] No internal access-service shortcut is used.

## Blocked by

- AIJ-01

## Comments

- Completed in `36eb47d`.
- Current-head release qualification remains R3.
