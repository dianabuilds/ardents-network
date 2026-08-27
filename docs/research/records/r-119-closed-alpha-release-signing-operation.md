---
id: R-119
title: Closed-alpha release signing operation
status: decided
owner: Product Owner and Codex
started: 2026-08-27
reviewed: 2026-08-27
---

# R-119 — Which minimal Product Owner-operated signing and custody operation can create the first H4-1/H4-6A alpha inputs without turning a test fixture, GitHub, or a project VPS into an undeclared release authority?

## Decision this unlocks

The real H4-8A A4 artifact: one immutable Ubuntu Portable archive, its exact
TUF Release input, separately rooted H4-6A catalog/component inputs, the
Alpha Enrollment Pin, and their retained non-secret receipt.

## Current contract

H4-1 selects one Ubuntu `linux-amd64` Portable artifact and an independently
delivered, one-bundle Alpha Enrollment Pin. The pin authenticates the first
bundle inventory only; it is not a reusable release key. ADR-0015 assigns
Release verification and local floors to `internal/release`, not repository
administration. ADR-0038 keeps the H4-6A disclosure catalog and its three
component roots separate from Release and Endpoint admission.

`internal/release` currently verifies a fixed TUF-compatible profile with five
top-level keys and a 3-of-5 ordinary threshold (4-of-5 emergency threshold).
It deliberately has no key input, signer, metadata writer, downloader, or
repository operator. Test helpers construct ephemeral private keys and must not
be promoted into release material. The existing alpha-bundle assembler accepts
only already-authenticated static inputs and is likewise not a signer.

The active team is the Product Owner and Codex. No independent custodian,
release engineer, online timestamp service, or external participant may be
assumed.

## Hypotheses

- **H1:** an external, owner-controlled offline release workspace can make one
  short-lived bounded alpha input set using the maintained verifier profile,
  while the repository continues to contain no secrets or repository-admin
  capability.
- **H2:** a maintained in-repository release-administration/signer module is
  required before a truthful first alpha can be created.
- **H0:** neither approach can satisfy the selected alpha boundary with one
  Product Owner, so alpha distribution must remain unexecuted.

## Evaluation criteria

- The exact Ubuntu artifact, TUF target metadata, catalog, Release, Network,
  Compatibility, and optional corpus components are independently verifiable
  before publishing.
- Private signing material never appears in the repository, its history,
  generated bundle, CI logs, GitHub Release assets, shell history, or an
  unprotected long-lived VPS path.
- No signing key crosses the existing `internal/release`, Endpoint, Update,
  Route, or H4-6A reader boundaries; root, TUF role, disclosure, and component
  authority remain visibly distinct.
- The procedure is feasible for the declared one-to-one operation, permits a
  bounded expiry/emergency-stop response, and records a recovery/rotation owner
  without claiming independent or threshold control.
- A verifier rejects an incomplete, expired, malformed, altered, mixed, or
  lower-floor input set. Publishing and first-contact evidence remain distinct.

## Evidence plan

### Primary sources

- [TUF specification, roles, repository operation, and key-management
  sections](https://theupdateframework.github.io/specification/latest/),
  accessed 2026-08-27.
- [TUF metadata overview](https://theupdateframework.io/docs/metadata/),
  accessed 2026-08-27.
- [ADR-0015](../../adr/0015-separate-release-decision-from-local-activation.md),
  [ADR-0038](../../adr/0038-alpha-control-disclosure-reader-v1.md),
  `internal/release`, and `packaging/alpha-bundle`, inspected 2026-08-27.

### Experiment

After the Product Owner records the physical workspace and secret-entry method,
run one disposable, non-public ceremony with fresh keys outside the repository.
Build the exact `linux-amd64` command bytes, create TUF and H4-6A inputs under
separate roots, assemble twice with a fixed `SOURCE_DATE_EPOCH`, and verify the
unpacked inventory plus Release and H4-6A reader outcomes before any upload.
Destroy the disposable proof material or retain it only under the selected
custody process. Do not use a test fixture, publish it, or contact a participant
during this experiment.

### Failure scenarios

- A signer, catalog key, or component key substitutes a different authority or
  artifact.
- An incomplete/expired/mixed TUF set, changed static byte, lower floor, or
  emergency metadata is accepted.
- A secret is retained in repository state, an unencrypted VPS path, CI output,
  a bundle, or a release asset.
- The key holder is unavailable, loses access, or cannot execute the declared
  revoke/expiry response.
- A published archive or direct message is mistakenly treated as proof of
  independent participant enrollment.

## Findings

- **Sourced fact:** TUF has distinct Root, Targets, Snapshot, and Timestamp
  roles. Targets authenticates target descriptions, Snapshot binds the current
  metadata set, and Timestamp binds Snapshot; root keys are expected to remain
  offline, while timestamp compromise has intentionally narrower impact.
- **Sourced fact:** TUF explicitly does not bootstrap authenticity for an
  arbitrary first manual download. The selected independent Alpha Enrollment
  Pin therefore remains necessary even when valid TUF metadata exists.
- **Current-contract fact:** the maintained Release module validates inputs but
  does not sign them; its tests generate ephemeral synthetic keys. The bundle
  assembler copies and hashes already-authenticated files but creates no
  authority.
- **Inference:** adding generic release signing to the current runtime modules
  would violate their ownership boundaries and create a new secret-storage and
  release-administration product. It cannot be treated as an incidental helper.
- **Inference:** an external offline workspace is the smallest candidate for
  one fixed closed alpha because it adds no runtime authority or repository
  secret. It still needs an explicit Product Owner custody declaration before
  it can generate anything real.

## Options

1. **External offline Product Owner release workspace.** A non-repository,
   access-controlled workspace holds separate release/disclosure/component
   signing material only during an explicit ceremony. The repository retains
   verifier code, deterministic assembly, and public receipts. This fits the
   current one-to-one capacity but does not provide independent or threshold
   authority.
2. **New maintained release-administration module.** Add a dedicated signing
   operation, key-storage profile, repository writer, rotation, audit receipt,
   and qualification. This could reduce manual work, but it is a consequential
   new custody/product surface and needs a separate accepted ADR and native
   security qualification.
3. **Reuse tests, GitHub, CI, or the VPS as an implicit signer.** Rejected.
   Test keys are ephemeral evidence; distributors are not first-install
   authority; and an undeclared always-available VPS key contradicts the
   offline/custody boundary.

## Recommendation

Choose option 1 for the first fixed closed alpha, subject to the Product Owner
recording the physical workspace, accountable custodian, separate key roles,
secret-entry method, recovery/rotation owner, expiry, emergency-stop process,
and alpha topology. The release ceremony document is the non-secret procedure;
no secret or actual artifact may be created until that declaration exists.

**Confidence:** high that no current maintained command can create real Release
inputs and that test keys cannot supply them; medium that a manual ceremony will
remain manageable for the first cohort. **Strongest argument against the
recommendation:** manual one-person key custody has a single operational point
of failure and provides no independent control; it is acceptable only while the
claim stays bounded and provisional.

## Disposition

Decided on 2026-08-27 after the Product Owner selected the local Windows
workspace and themself as custodian. ADR-0050 owns the separate encrypted seed
record and its one interactive initializer. The next release-signing operation
must consume that record only through a new exact bounded interface; it may not
become a generic signer. Real signed inputs, alpha topology, publication, and
independent participant evidence remain implementation gates.
