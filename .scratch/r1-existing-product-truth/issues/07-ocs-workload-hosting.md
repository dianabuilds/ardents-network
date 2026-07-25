# OCS-03: Prove workload and hosted-service procedures

Status: ready-for-agent
State: closed
Labels: ready-for-agent
Research class: R1

## Parent

`../PRD.md`

## What to build

Prove a complete Operator workload procedure across list, get, register, start,
stop, restart, hosted service, and publication views while preserving the
distinction between runtime readiness and service publication.

## Acceptance criteria

- [x] The real CLI and Operator API are exercised.
- [x] Human and JSON outcomes are asserted.
- [x] Runtime readiness and publication remain distinct.
- [x] A rejected mutation has a nonzero outcome.
- [x] Docker-dependent qualification remains explicitly R3.

## Blocked by

- OCS-01

## Comments

- Completed in `5aa5c59`.
