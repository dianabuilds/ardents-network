---
name: ardents-release-bug-hunt
description: Failure-oriented release bug-hunting workflow for Ardents. Use before a release or after a risky merge when you need to search for latent bugs, unsafe assumptions, broken edge cases, stale runtime truth, race-prone state transitions, or cross-domain regressions that ordinary review might miss.
---

# Ardents Release Bug Hunt

Use this skill to actively search for breakage, not to validate a happy path.

## Read First

- `docs/system-concept.md`
- `docs/system-properties.md`
- `docs/development-contract.md`
- the relevant domain document
- `docs/reference-invariants.md` if the bug hunt touches network/discovery/messaging/publication

## Hunting Goal

Find bugs that let the product look assembled while violating runtime truth.

Search especially for:
- state transition bugs
- persistence restore bugs
- stale publication or stale discovery data
- trust results that do not affect behaviour
- mismatches between desired and observed state
- lifecycle bugs around startup, restart, stop, or recovery
- races between diagnostics, persistence, and runtime actions

## Workflow

1. State the domain and the risky runtime flows.
2. Enumerate the product truths that must stay true in those flows.
3. Probe what happens on invalid input, partial state, restart, timeout, missing dependency, corrupted file, empty data, and repeated operations.
4. Trace whether the code can drift into a state the docs say is impossible.
5. Check whether diagnostics would expose the bug or mask it.
6. Record concrete bug candidates with reproduction logic.

## Mandatory Bug Categories

- startup/recovery drift
- shutdown persistence loss
- local API reporting stale or false state
- discovery records surviving after runtime truth changed
- service publication not matching real workload/runtime state
- trust or policy computed but not enforced
- corrupted or missing retained state leading to silent success
- repeated command handling producing duplicate or contradictory outcomes

## Reject If

- a critical flow can silently fail
- a product truth can become stale without diagnostics showing it
- runtime state and published/discoverable state can diverge
- restart or recovery can damage authoritative state
- the system can report ready while a required plane is not operational

## Output

When using this skill, produce:

- bug-hunt scope
- bug candidates ordered by severity
- likely trigger or reproduction condition for each finding
- release impact: blocks release or monitor-only
