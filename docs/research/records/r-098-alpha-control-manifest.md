---
id: R-098
title: Verifiable alpha-control manifest
status: decided
owner: Product Owner and Codex
started: 2026-08-24
reviewed: 2026-08-28
---

# R-098 — What minimum signed alpha-control manifest and independently runnable reader let a participant inspect accepted release, network, profile, and name-corpus inputs without making project control appear independent?

## Decision this unlocks

Select H4-6A's alpha-control artifact and reader, or defer alpha distribution
until its accepted inputs can be exposed honestly.

## Current contract

- Alpha may use known project control but must disclose it and fail explicitly
  on missing/stale/conflicting control.
- Release safety, Network Epoch, Namespace materialization, and protocol/build
  compatibility are distinct transitions with separate roots/floors.
- Existing release code verifies bounded TUF-compatible offline inputs but does
  not provide a participant bootstrap or public control plane.

## Hypotheses

- **H1:** one immutable signed manifest plus a small independently runnable
  reader can expose exact accepted alpha inputs and limitations without secrets.
- **H2:** separate release/network/name artifacts need several readers, but can
  still share a coherent participant-visible report.
- **H0:** a useful alpha cannot make its provisional roots legible without
  exposing protected operational material or implying independent authority.

## Evaluation criteria

- roots, artifact identity, state/profile digest, validity, floor, alpha scope,
  limitations, and failure states are explicit;
- no reader runs or trusts the candidate Endpoint executable it is inspecting;
- manifest disclosure does not expose Authority, User, Service, or sensitive
  topology material;
- stale, forked, withheld, revoked, and unavailable cases are reproducible;
- tooling, rotation, and support burden remain finite for one-to-one operation.

## Evidence plan

### Primary sources

- TUF specification/documentation, current release module, H4-6, release and
  operating contracts.

### Experiment

Construct harmless synthetic alpha artifacts/state inputs and a reader that
reports accepted/rejected conditions. Mutate every root, digest, validity,
profile and floor field; compare two distributors with identical/conflicting
bytes. Do not use an actual Authority or public network.

### Failure scenarios

- Reader accepts a mismatch, older floor, expired input, or unknown root.
- One all-powerful manifest key silently authorizes name seizure or code install.
- Diagnostics reveal protected topology or participant data.

## Findings

- **Sourced fact:** TUF separates signed roles, expiry, and a consistent
  snapshot; its role model limits which party may provide which information and
  its snapshot prevents mixing metadata that never existed together. [TUF roles
  and metadata](https://theupdateframework.io/docs/metadata/) (accessed
  2026-08-24). The current release module already uses a bounded
  TUF-compatible input, but TUF is a software-update framework rather than a
  ready-made Network State or Namespace authority.
- **Sourced fact:** TUF explicitly does not solve the initial authenticity of
  arbitrary manually downloaded software; it expects a client to start with a
  trusted root. [TUF specification](https://theupdateframework.github.io/specification/latest/)
  (accessed 2026-08-24). R-098 therefore cannot resolve R-095's first-artifact
  provenance problem merely by adding a control manifest.
- **Current-contract fact:** `internal/release` verifies release roots and
  non-decreasing release floors, but does not download, operate a repository,
  or select a platform bootstrap; `naming/namespace` has no selected
  global-close producer for its current state. [Release/update/custody](../../technical/release-update-custody.md)
  and [naming contract](../../technical/naming.md) (inspected 2026-08-24).
- **Inference:** one signature must not become a hidden all-powerful alpha key.
  A release-acceptance key cannot by itself authorize a Network profile or seize
  a Name; conversely, a Network or alpha-corpus key cannot authorize executable
  bytes. The H4-6 reader must show each component's own authority and verifier
  outcome rather than treating its outer catalog signature as acceptance.
- **Inference:** the leading artifact shape is a small signed *disclosure
  catalog* with cohort identifier, schema version, explicit project-control
  limitation, and immutable digest/size/validity/reference for each component.
  Components are at least `release`, `network-profile`, `compatibility`, and
  optional `named-alpha-corpus`. Each has a distinct component class, root/key
  identifier, predecessor/floor reference, and reader outcome. The catalog is
  only a consistent, inspectable index; an Endpoint accepts work only after the
  component's own verifier accepts it.
- **Inference:** an independently runnable reader must be a separately
  versioned, release-verifiable artifact that consumes copies of the catalog and
  component bytes without starting the candidate Endpoint. Its own acquisition
  remains subject to the R-095 provenance path. A project web page or API may
  distribute bytes but cannot be the only verifier.
- **R-095 enrollment input:** three Ubuntu runs show that an independently
  delivered exact manifest digest can bind the first reader/Endpoint artifact,
  descriptor, and initial TUF-root bytes without executing project code. A
  self-consistent substituted or older bundle still failed the pin. This is the
  narrower recommended acquisition input, not catalog authority; the actual
  independent participant contact remains untested. A separate SSHSIG key also
  worked mechanically but would add reusable first-install code authority and
  is no longer the leading closed-alpha candidate.
- **Measurement:** the disposable synthetic catalog reader generated separate
  fresh Ed25519 disclosure, release, and network keys in memory. On 2026-08-24
  it reported: `valid=release:accepted,network-profile:accepted`;
  a changed release byte yielded `release:digest-mismatch` while the network
  component remained accepted; an independently re-signed expired network
  component yielded `network-profile:expired`; an independently re-signed
  lower release version yielded `release:lower-floor`; distinct valid catalog
  bytes yielded `catalog:conflict`; a catalog below its floor yielded
  `catalog:lower-floor`; an unknown required component yielded
  `catalog:unknown-component`; and absent catalog bytes yielded
  `catalog:unavailable`. The harness wrote no files and did not start an
  Endpoint or network listener. [Experiment README](https://github.com/dianabuilds/ardents-network/blob/fbb42034757513ac009114a00b933aefa76d8ddf/experiments/r-098-alpha-control-catalog/README.md)
- **Inference:** this result supports the proposed authority separation: the
  disclosure key binds one inspectable cohort view but cannot make changed,
  expired, or lower-floor component bytes acceptable. It is not format,
  bootstrap, Network State, or Namespace materialization evidence.
- **Measurement:** the R-098 follow-up wrote a synthetic three-file catalog to
  a newly created private temporary directory, then removed that exact
  directory on exit. `catalog.json`, `release.json`, and
  `network-profile.json` were each regular files limited to 4 KiB. Each file
  used an envelope with base64-encoded body bytes and a detached Ed25519
  signature over the exact decoded body bytes; the catalog bound the SHA-256
  digest of each complete component envelope. On 2026-08-24 the reader
  reported `bounded_files=release:accepted,network-profile:accepted` and,
  after replacing only `catalog.json` with 4,097 bytes,
  `oversized_catalog=catalog:too-large`. It derives the two component paths
  from fixed classes, rejects non-regular file objects, and performs the size
  check before decoding JSON. This is a bounded local-format result, not a
  selection of a root, wire format, or release artifact.
- **Measurement:** the same reader scans every JSON object before decoding and
  rejects repeated keys, unknown fields, and trailing JSON values. Five Go
  tests covered valid files, a changed component, oversized catalog rejection,
  a direct duplicate-key input, and a duplicate key in an otherwise
  signature-valid catalog envelope on 2026-08-24. This makes duplicate-key
  handling explicit for the experiment; it is not an independent parser audit
  or a decision to use JSON in the maintained artifact.

The Product Owner selected the authority shape on 2026-08-25: one signed
disclosure catalog plus separately verified components. The selection does not
choose a catalog wire format, a disclosure root, component roots, a reader
artifact, or a bootstrap path.

## Options

1. **One signed disclosure catalog plus separately verified components.** The
   catalog gives a consistent cohort view but has no authority to make an
   Endpoint accept any component outside that component's own verifier.
2. **Separate immutable manifests with a read-only aggregator.** Strongest
   separation but possibly too many artifacts and versions for an alpha user.
3. **One manifest/key accepted as every authority.** Rejected: it collapses
   release, network, compatibility, and Name power into an opaque project key.
4. **A hosted dashboard or project API as authority.** Rejected as a sole
   verifier; it is a distributor/presentation only.

## Accepted decision

H4-6A uses option 1: a signed disclosure catalog and component-local
verification. The catalog provides a consistent inspectable cohort view only.
It is not an authority for Release acceptance, Network State, compatibility,
Namespace, or any other component; an Endpoint accepts a component only through
that component's own verifier and root/floor rules.

The synthetic reader validates valid, expired, withheld, conflicting,
mismatched-digest, component-floor, catalog-replay, and unknown-component cases
without executing an Endpoint. Its 4 KiB bound, exact-byte envelope rule, and
duplicate-key rejection are experiment evidence, not selected production format
rules. Implementation still needs real component identity mapping, an
independently distributed reader artifact, parser/resource review, and R-095's
initial participant provenance path.

**Confidence:** high that one accepted all-powerful key would contradict the
H4-6 separation requirement; medium that a catalog remains small enough for
the alpha user journey. **Strongest argument against the recommendation:** a
catalog and standalone reader can duplicate much of the existing release
metadata experience while still leaving the unselected Network and Namespace
roots unresolved.

## Disposition

Decided for the H4-6A authority shape and implemented through ADR-0038's
maintained reader. On 2026-08-28 the immutable `h4-alpha-1-rc-1` published the
concrete catalog plus separately rooted Release, Network, and Compatibility
inputs. One cached repeat and two fresh enrollment-pinned standalone inspection
roots accepted the same three components and Network epoch. These reader roots
are physically distinct from Endpoint state. The bundle's `corpus.pub` is only a
manifest-pinned authority companion; no ACA2 or signed-corpus acceptance is
claimed. This closes the bounded functional-alpha H4-6A input gate; it does not
establish independent parser/security review, threshold custody, independent
control, or Public Beta governance.
