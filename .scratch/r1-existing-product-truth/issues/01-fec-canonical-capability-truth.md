# FEC-001: Establish one canonical capability truth

Status: ready-for-agent
State: closed
Labels: ready-for-agent
Research class: R1

## Parent

`../PRD.md`

## User story

As an engineer or product owner, I can resolve every current Ardents
capability by stable ID in one machine-readable and human-readable source.

## What to build

Create strict `docs/engineering/capabilities.json`, migrate the 24 initial
capability IDs and eight required domains, generate the Markdown register, and
fail closed on invalid ownership, interfaces, statuses, links, paths, domains,
IDs, or projection drift.

## Acceptance criteria

- JSON is the only editable catalogue source.
- The generated Markdown projection is deterministic and byte-checked.
- Every required capability and domain is present with stable ownership.
- All current capabilities remain `Q=no`.
- Static CI runs the validator.

## Blocked by

Nothing.

## Comments

- Completed in `5cd480d`.
- Canonical post-R1 product status is reported at
  `8b9f8ad87fb78fccd7a73d445f2d72dbf2e51b4c`.
