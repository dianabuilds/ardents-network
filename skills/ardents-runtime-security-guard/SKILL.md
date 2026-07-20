---
name: ardents-runtime-security-guard
description: Runtime security review workflow for Ardents. Use when a change touches encryption, key handling, retained data, relay behavior, diagnostics redaction, local secrets, access control, or other runtime security-sensitive paths.
---

# Ardents Runtime Security Guard

Use this skill when the code change can expose plaintext, secrets, or unsafe runtime behavior.

This skill is distinct from dependency review:
- `ardents-dependency-safety` evaluates external libraries and their security posture.
- `ardents-runtime-security-guard` evaluates our runtime behavior and secret/data handling.

## Read First

- `docs/engineering-constraints.md`
- the relevant domain document
- `docs/canonical-network-foundation.md` if the change touches network/messaging
- `docs/data-substrate-requirements.md` if the change touches retained or re-served data

## Review Workflow

1. State what sensitive asset is at risk: plaintext, keys, tokens, retained data, diagnostics output, authz outcome.
2. State which domain owns the behavior.
3. Check whether the change exposes plaintext where only encrypted retention is allowed.
4. Check whether secrets or key material can leak through logs, diagnostics, or API.
5. Check whether access control or relay behavior violates domain requirements.
6. Accept only if the runtime behavior preserves the security model.

## Mandatory Checks

- no plaintext exposure to relay-only holders
- no key material in logs, diagnostics, or API snapshots
- no secret mixed into temporary payload storage
- no default-open behavior for sensitive operations
- no diagnostics message that reveals what the security model forbids revealing

## Reject If

- the change weakens encrypted retention guarantees
- the change exposes secrets or key material
- the change relies on "we will harden this later"
- the security behavior cannot be explained from the code and docs
- diagnostics are made more informative by leaking sensitive data

## Output

When using this skill, produce:

- sensitive asset under review
- security invariant being checked
- pass/fail assessment
- exact runtime security risks if rejected
