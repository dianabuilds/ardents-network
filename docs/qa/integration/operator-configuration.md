# Scenario OCI-001

## Title

Validated operator configuration inspection and atomic reload.

## Layer

integration

## Domain

Node Runtime, Policy, Diagnostics, and canonical local control surface.

## Purpose

Prove that an operator can inspect a redacted effective configuration, apply a
reloadable change through the owning runtime service, and reject an invalid
candidate without replacing the active generation.

## Steps And Assertions

1. Start a node with a validated versioned document and configuration manager.
2. Read the effective configuration through `ard --output json config show`.
3. Assert the active generation is visible and the token path is redacted.
4. Change a reloadable Policy setting and invoke
   `ard --output json config reload`.
5. Assert the outcome is `applied` and the active generation advances.
6. Replace the source with an invalid version and reload again.
7. Assert the invalid candidate is rejected and the active generation remains
   unchanged.

## Failure Meaning

Failure means the operator surface can leak protected configuration, report
the wrong generation, or replace working runtime behavior with an invalid
candidate.

## Related Tests

- `tests/integration/local-control-surface/configuration_test.go::TestOperatorConfigurationCLIShowsReloadsAndRejectsInvalidCandidate`

## False Positive Risk

A CLI response could claim reload success without changing active runtime
truth. The scenario checks the effective generation before and after the
reload and verifies the rejected candidate does not replace it.

## False Negative Risk

Filesystem observation could race an editor write. The test writes complete
candidate documents atomically and invokes reload explicitly.
