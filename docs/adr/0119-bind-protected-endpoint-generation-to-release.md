---
status: accepted
date: 2026-09-30
---

# ADR-0119 — Authenticate the protected Endpoint generation in the same Release set

## Context

[R-170](../research/records/r-170-unified-protected-endpoint-artifact.md)
established that enrollment-v3 authenticates declared bytes but has no protected
worker inventory projection. Existing replacement authorization covers only the
executable. Root-installed worker digests establish local integrity, not Release
provenance. The Product Owner accepted the contract on 2026-09-30.

## Decision

1. Distribute one authenticated bundle with one independently delivered initial
   enrollment pin. Retain Network enrollment-v3 and its retired-version refusal.
   No second worker pin or new trust root is introduced. The pin remains
   first-install-only and never authorizes a successor.
2. Add `protected-endpoint.json` as a flat enrollment companion, projected
   outside Release metadata. Authenticate its bytes as target
   `ardents/linux-amd64/protected-endpoint` in the same signed metadata set as
   the Endpoint executable target. Existing Release custom identity and builder
   commitment semantics remain unchanged; no field is overloaded.
3. The canonical descriptor has exactly `schema`, `platform`,
   `release_identity`, `release_version`, `files`; schema is
   `ardents-protected-endpoint-artifact-v1`, maximum 16 KiB. `files` contains
   exact lowercase SHA-256 digests for `ardents-linux-amd64`,
   `ardents-text-linux-amd64`, `ardents-text-reader@.service`,
   `ardents-text-publisher@.service`, both corresponding `.socket` files,
   `50-ardents-text.rules`, `ardents-text.conf`, and
   `ardents-endpoint.service`. Reject unknown/duplicate fields, noncanonical
   bytes and incomplete inventories. No mutable plans, roots, permissions or
   private keys occur in it. Exclude enrollment manifest/metadata digests to
   avoid a signing cycle. The complete distribution counts 27 manifest entries
   within the existing 32-entry cap; individual file bounds remain unchanged.
4. The protected installation owner requires two fresh opaque authorizations
   from identical admitted metadata, local environment and reference time.
   Require equal Targets floors/digest, release identity/version, platform,
   architecture, environment and Network. Match descriptor facts to both
   Decisions, its Endpoint digest to the executable Decision, and every actual
   resource byte to its descriptor digest. A second `no-update` evaluation
   after shared floors commit does not waive authorization.
5. Existing general/headless v3 acceptance retains its existing scope, never
   protected readiness. Protected installation requires the complete group;
   partial group presence refuses at every maintained enrollment boundary.
   Authentication alone never grants execution. Existing executable replacement
   cannot substitute a generation authorization for its program proof.
6. A root-controlled local installation binding records generation digest,
   both target facts and exact installed plan/unit-output digests; it grants no
   authority independently. Changed resources with unchanged executable still
   require a strictly newer accepted generation release. Equal version with a
   different generation refuses. Restart re-verifies the selected generation
   and real unit/account/executable/MainPID/InvocationID, retaining all durable
   floors and honest Service continuity limitations.
7. Replacement joins the predecessor fixed system unit before transition,
   retains recoverable complete generations and refuses mixed-generation
   readiness. Filesystem publication and system-manager transitions are not
   one atomic transaction. Exact staging/recovery commands remain bounded
   implementation handoff work under the current owners.

## Consequences and verification

The installation owner must compose two authenticated targets; enrollment-only
or packaging-only changes cannot close that boundary. Authentic signed metadata
delivery remains an explicit operator prerequisite, not a new signing service.
Project-controlled builder records are not independent reviewers or builders.

R-170's disposable probes prove current manifest/projection and shared-floor
Release behavior, including different target bytes and substitution refusals.
They do not implement this descriptor or qualify installed Ubuntu behavior.
Required causal controls cover incomplete inventory, substitution, mismatched
proofs/metadata/release/version, crash recovery and actual unit identity before
effects. Full installed two-Endpoint publish/read/refresh/withdraw/restart on
both Carriers and systemd/cgroup containment remain separate acceptance.
