---
id: R-120
title: Bounded alpha input signing
status: open
owner: Product Owner and Codex
started: 2026-08-27
reviewed: 2026-08-27
---

# R-120 — Which exact local operation may consume the first-alpha encrypted seed record to create the fixed TUF and H4-6A input set, while refusing arbitrary signing, unrecorded topology, and publication?

## Decision this unlocks

The first real H4-8A A4 static input directory: initial TUF root plus
timestamp/snapshot/targets metadata for the exact Ubuntu Portable Endpoint,
and the independently rooted H4-6A catalog, Release, Network, and
Compatibility statements. The decision does not authorize publication,
participant contact, an online signer, or a generic signing facility.

## Current contract

ADR-0050 stores exactly five TUF top-level seeds, one disclosure seed, and
three H4-6A component seeds in one owner-only local encrypted record. Its
current `Initialize` interface cannot decrypt or sign. `internal/release`
verifies rather than writes the H3 TUF profile; `internal/alphacontrol`
provides canonical ACA1/ACS1 codecs but its signing functions are currently
used only by controlled fixtures. The closed-alpha ceremony requires actual
Network State/topology bytes and a verifier preflight before bundle assembly.

The active team is the Product Owner and Codex. The Windows workspace is local
custody; the project VPS, GitHub, CI, bundle, and repository are not secret
stores or signing authorities. ADR-0051 additionally permits only a
passphrase-authenticated public receipt recovery for that record; it is not a
signing or export operation. The first release remains one-person provisional
control, not a threshold or independent-control claim.

## Hypotheses

- **H1:** one new deep local Module can expose a single fixed
  `BuildAlphaInputs` Interface, consume the encrypted record only through a
  no-echo terminal Adapter, create exactly the selected TUF/ACA1/ACS1 files in
  an empty external output directory, preflight the result with maintained
  verifiers, and return a public receipt.
- **H2:** a documented collection of external TUF/cryptographic tools can
  safely create the same material without a maintained Module.
- **H0:** no candidate meets the fixed-alpha contract until an actual signed
  Network State/topology and two-builder release descriptor are available.

## Evaluation criteria

- The operation signs no caller-supplied opaque bytes and accepts no arbitrary
  role, path, metadata version, root, key, mirror, upload target, or command.
- It constructs all four TUF top-level roles with the established 3-of-5
  ordinary threshold and builds the Release target identity required by
  `internal/release` for the exact immutable artifact.
- It creates the disclosure catalog and its three distinct component statements
  only from independently validated Release and Network evidence, with a
  Compatibility statement binding both outcomes.
- The operation fails before publication on altered, incomplete, expired,
  mixed-generation, unrecorded-authority, wrong-artifact, or wrong-topology
  inputs. It publishes no partial visible output.
- Private seeds, passphrases, decrypted bytes, derived keys, and generic
  signing callbacks remain unavailable to Release Decision, Endpoint, Update,
  Network State, the VPS, repository, CI, artifacts, and receipts.
- The public receipt binds source revision, exact input/output digests,
  validity interval, TUF/ACA1 generations, and the preflight result without
  being first-contact authority.

## Evidence plan

### Primary sources

- [The Update Framework specification](https://theupdateframework.github.io/specification/latest/),
  version 1.0.36, accessed 2026-08-27: top-level roles, threshold signatures,
  canonical signed metadata, repository operation, and the explicit absence of
  first-download bootstrap authenticity.
- [TUF roles and metadata overview](https://theupdateframework.io/docs/metadata/),
  accessed 2026-08-27.
- ADR-0015, ADR-0038, ADR-0050, R-119, `internal/release`,
  `internal/alphacontrol`, and `packaging/alpha-bundle`, inspected 2026-08-27.

### Experiment

After a concrete alpha Network State/topology and two-builder descriptor exist,
run one non-public external-workspace ceremony. The candidate builds the exact
static directory twice from the same recorded inputs, verifies it through
`internal/release` and `ardents-control`, and deliberately changes one artifact,
one component root, one component body, one expiry, and one Network input to
prove rejection. Retain only the selected encrypted record and non-secret
receipt under Product Owner custody. Never upload or contact a participant in
this experiment.

### Failure scenarios

- The input asks the operation to sign another target, role, root, key, or
  opaque message.
- A valid TUF target is combined with a catalog/component for another artifact
  or Network State.
- A partial directory survives an interruption and is accidentally assembled.
- A stale release, emergency/expiry conflict, or lower generation is emitted.
- A private seed, passphrase, plaintext, or generic signing capability escapes
  the local process.

## Findings

- **Sourced fact:** TUF defines Root, Targets, Snapshot, and Timestamp as
  separate top-level roles, each with one or more keys and a threshold. Root
  private keys are expected to remain offline; TUF itself does not establish
  trust for an arbitrary manually downloaded first executable.
- **Current-contract fact:** `internal/release` requires a complete offline
  byte set and authenticates an exact H3 target identity, including two
  project-controlled builder attestations. It deliberately has no metadata
  writer or key input.
- **Current-contract fact:** ACA1 binds three fixed components under distinct
  independently pinned roots; its Release/Network/Compatibility evidence
  codecs already reject malformed or unbound bodies.
- **Inference:** shelling together general TUF and Ed25519 utilities would
  make the caller responsible for role ordering, custom target fields,
  cross-component binding, atomicity, and verifier preflight. That is a shallow
  and error-prone Interface for a one-person operation.
- **Inference:** a single fixed operation can offer materially more depth than
  a generic signer, but it cannot fabricate the actual Network State/topology,
  two-builder identity, or independent participant evidence.

## Options

1. **Fixed maintained alpha-input Module.** One terminal caller supplies a
   tightly validated public alpha-input description and the selected encrypted
   record. The Module constructs only the prescribed files, performs local
   verifier preflight, writes an all-or-nothing external directory, and returns
   a public receipt. It is the leading option if an explicit code/interface
   review and behavior evidence accept it.
2. **External generic tools and handwritten ceremony.** This avoids source
   changes but disperses policy and verification across commands/history. It is
   rejected unless it can prove the same refusal, receipt, and reproducibility
   properties without becoming an undocumented signing operation.
3. **Stop before metadata signing.** Retain seed custody only until an actual
   alpha topology and release descriptor exist. This is required if the missing
   public inputs cannot be supplied truthfully.

## Recommendation

Run the named fixed-Module design and behavior experiment, but do not generate
real metadata yet. First record the actual Network State/topology, release
identity, validity/emergency times, and two-builder descriptor for the
candidate. Then decide whether option 1's exact Interface can satisfy all
criteria; otherwise choose option 3. **Confidence:** high that generic tools
are insufficient for the declared operation; medium that the fixed Module can
remain small enough. The strongest argument against option 1 is that any
in-repository signer expands the trusted local surface and needs a separate ADR
and native qualification.

## Disposition

Open. R-120 is the next H4-8A A4 decision gate. No signing code, real key,
metadata, artifact publication, or participant contact is authorized by this
record. Retain no experiment directory until real non-secret inputs exist.
