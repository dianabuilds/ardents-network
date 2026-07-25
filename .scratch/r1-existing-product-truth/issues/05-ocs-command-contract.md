# OCS-01: Publish a fail-closed Operator command contract

Status: ready-for-agent
State: closed
Labels: ready-for-agent
Research class: R1

## Parent

`../PRD.md`

## What to build

Publish one closed Operator command registry that makes every supported leaf
discoverable offline and binds it to its procedure, action, transport, output
family, and smoke owner. Keep successful command names and JSON payloads
compatible while making catalogue, help, parser, action, and evidence drift
fail closed.

## Acceptance criteria

- [x] All 68 command leaves are represented in the closed registry.
- [x] Root, group, nested, and shell help are reachable offline.
- [x] Action, procedure, evidence, parser, and JSON-family contracts fail closed.
- [x] Existing successful command names and JSON payloads remain compatible.

## Blocked by

None - completed.

## Comments

- Completed in `a2ecb12`.
