# ADR-0120: Authorize an operator-prepared qualification release

Status: accepted by the Product Owner on 2026-10-01 in the active conversation.

## Context

The protected installation selected by ADR-0119 consumes an authenticated
Release. The maintained Release owner verifies metadata and the bundle adapter
assembles already authenticated inputs. Retired release seed ceremonies do not
authorize another release. Requiring the Product Owner to manufacture missing
files would hide an unavailable operation rather than complete installation.

## Decision

Authorize one explicitly recorded qualification release operation performed by
Codex for the Product Owner. Prefer a designated existing authority only when
its provenance and access can be established. Otherwise use a fresh isolated
qualification cohort with explicitly accepted new bootstrap trust. Never reset
or adopt existing roots, floors, permissions or historical cohort identities.

Prepare the exact executable, protected descriptor and resource inventory
before signing. Use maintained cryptographic and TUF libraries; do not add a
runtime signer, generic signing service or new cryptographic primitive. Keep
authority roles separate and private material protected outside the repository
and absent from logs, command arguments and public receipts. Existing Custody
interactive commitment and unlock controls remain binding.

Retain the verifier's five top-level keys and ordinary three-signature
threshold; one operator controlling them is not threshold independence.
Produce and retain two actual project-controlled rebuild records with distinct
record identities, exact source and build-input commitments, and matching
target digests. Do not fabricate rebuild evidence or change the existing
two-builder verification policy to make the release pass.

The bootstrap receipt must name the exact qualification admitted by the
signer: source identity, reproducible artifact bytes, maintained checks and
permission to execute these bytes for the isolated installation qualification.
The existing `qualified` Release field is an artifact admission assertion by
that signer, not evidence that the installed Service journey has passed.
Do not mark an artifact admitted until that receipt is accepted. Record the
uncompleted installed lifecycle and isolation checks alongside it. Do not use
`development-only` metadata in the alpha environment, falsify protocol overlap
dates, or invent emergency grounds. A new isolated cohort can use the existing
`announced` phase without pretending it completed a required transition.

Record the public authority commitments, exact artifact digests, metadata
versions and expiry, and the unpacked manifest digest. Present the completed
public receipt to the Product Owner for bootstrap acceptance before installed
execution. That acceptance is the authority decision for this isolated
qualification operation; it is not independent builder or reviewer evidence.
Do not describe a digest prepared and delivered by Codex as independent
delivery. ADR-0119's independent first-pin distribution contract for supported
participant enrollment remains unchanged and must be qualified separately.

Verify complete coherent executable and protected-generation targets with the
existing Release and enrollment owners, including altered-resource, mixed or
expired metadata, wrong-pin and interrupted/non-replacing-output refusals.
The first accepted manifest never authorizes successors. Subsequent releases
retain normal Release floors and root-rotation rules.

## Consequences

Codex owns technical preparation, signing-operation implementation, assembly
and verification. The Product Owner accepts the resulting bootstrap receipt.
Actual installation, Service lifecycle on both Carriers and real Ubuntu
isolation observations remain separate required evidence. Results from this
same-operator qualification cannot establish independent distribution or
independent security validation.
