---
id: R-113
title: Alpha Name Corpus distribution and durable floor
status: decided; implementation-linked
owner: Product Owner and Codex
started: 2026-08-25
reviewed: 2026-08-26
---

# R-113 — How can an H4-4A participant receive an Alpha Name Corpus and retain a restart-safe floor without making a project disclosure key a Namespace or Endpoint authority?

## Decision this unlocks

Select the authority/artifact boundary that turns the maintained local alpha
corpus and session floor into a participant-visible, restart-safe H4-4A input.

## Current contract

- ADR-0040 fixes a non-Namespace `ardents-alpha://` overlay, a finite signed
  Alpha Name Corpus, explicit expiry/withdrawal, local localhost browser
  presentation, and no Target fallback.
- The maintained alpha corpus code verifies its own authority signature and
  provides both a test-local session floor and an Endpoint-owned durable
  serial/digest floor. Neither is a distributor or Endpoint-startup authority.
- ADR-0038 fixes `ACA1` alpha-control v1 to exactly Release, Network, and
  Compatibility components. It explicitly excludes H4-4, and it cannot gain a
  name corpus by an optional input or an unversioned parser change.
- Canonical Namespace's Epoch/current-proof rules remain separate and cannot
  be replaced with a project catalog.

## Hypotheses

- **H1:** a versioned alpha-control successor can disclose an independently
  signed corpus component and its authority root, while an Endpoint-owned
  durable corpus floor verifies only that component and never gives the
  disclosure catalog Endpoint or Namespace authority.
- **H2:** an independently pinned corpus companion plus Endpoint-owned floor
  is smaller for the alpha, but its additional first-contact root and rotation
  story are unjustified.
- **H0:** no current alpha delivery choice preserves the disclosed authority
  boundaries; H4-4A must remain a local/fixture slice until H4-6 evolves.

## Evaluation criteria

- A fresh participant can identify the corpus authority, exact bytes, cohort,
  Network, validity, serial, predecessor/floor, withdrawal, and refusal state.
- Restart rejects stale, same-serial-conflicting, forged, cross-cohort,
  cross-Network, expired, withdrawn, unavailable, and path-substituted inputs.
- Neither the disclosure key, release key, nor Network State key can silently
  retarget a name; the corpus key cannot authorize an executable, Node duty,
  or canonical Namespace claim.
- The artifact count, storage, rotation, recovery, and support procedure stay
  feasible for one Product Owner and Codex.

## Evidence plan

### Primary sources

- ADR-0038, ADR-0040, R-097, R-108, and the current `alphacontrol`, `alpha`,
  Endpoint lifecycle, and threat-model contracts (inspected 2026-08-25).

### Experiment

Construct one versioned corpus-disclosure candidate and an Endpoint-owned
private floor root. Exercise clean acceptance, restart, lower serial,
same-serial conflict, forged root, changed bytes, expiry, total withdrawal,
unavailability, and atomic interruption. The experiment must not start a
network listener or authorize Endpoint readiness.

### Failure scenarios

- A release/disclosure/catalog key makes a corpus accepted without its own
  authority signature.
- A copied older corpus becomes valid again after restart.
- A changed corpus at the retained serial wins by arrival order.
- Corpus storage becomes a hidden shared registry or deletes protected
  Endpoint state during renewal/removal.

## Findings

- **Current-contract fact:** `ACA1` is a closed three-entry catalog and
  ADR-0038 requires a later accepted versioned decision before H4-4 can add a
  component. Extending it in place would violate its no-unknown-component and
  authority-boundary rules.
- **Inference:** adding a corpus as a release metadata field would make release
  safety implicitly select naming authority and contradict R-098's separation.
- **Inference:** retaining only the current session floor leaves a restart
  rollback window, so it is an implementation tracer—not H4-4A promotion
  evidence.
- **Implementation fact:** the durable floor creates or validates an
  owner-only root, takes its sole platform lease before cleaning an interrupted
  successor, and accepts a received corpus only after its explicit decision
  time is valid. Thus a concurrent opener cannot remove an active writer's
  temporary file, and an expired or not-yet-valid higher serial cannot poison
  a restart floor.

## Options

1. **Versioned alpha-control successor (`ACA2`) with a fixed signed corpus
   component and independent corpus root.** Preserve `ACA1` unchanged; a new
   reader/profile verifies its exact corpus component and an Endpoint-owned
   floor consumes only verified corpus bytes.
2. **A separately enrollment-pinned corpus companion/root.** Keep `ACA1`
   unchanged but add one new explicit alpha artifact and a distinct initial
   root/floor.
3. **Release metadata carries corpus acceptance.** Rejected provisionally:
   it merges release and naming authority.
4. **Keep local-only H4-4A.** Honest but does not provide a participant-alpha
   named journey.

## Selected direction

Choose option 1: `ACA2` has a separate fixed corpus component and independently
pinned corpus root. ADR-0041 records the versioned authority boundary. The
next work is a maintained reader/component verifier plus an Endpoint-owned
floor experiment; neither may reinterpret `ACA1` or grant Endpoint authority
to the disclosure catalog.

**Confidence:** medium. **Strongest argument against:** an `ACA2` reader may
duplicate control tooling for a single alpha-only corpus; option 2 may be less
complex if its extra root remains operationally legible.

## Disposition

Decided and implementation-linked. `ACA2` has its fixed fourth corpus
component; `inspection.VerifyACA2Corpus`, `alpha.PersistentFloor`,
`endpoint.AcceptAlphaCorpusControl`, and `ardents-control inspect-alpha-corpus`
verify and retain only explicitly supplied bytes under independent roots. Their
targeted behavior tests cover forged/cross-Network input, changed bytes,
restart, stale serial, same-serial conflict, active-writer recovery exclusion,
and expiry/not-before refusal without floor advancement.

The participant procedure is now maintained but not yet promoted: an
enrollment-v3 bundle adds exact manifested `corpus.pub` and
platform-specific `ardents-control-<platform>` companions, and
`ardents-control accept-alpha-corpus` first verifies that exact enrollment,
all accepted ACA1 Release/Network/Compatibility evidence, then ACA2/corpus,
before it writes only the separately named corpus floor. A Linux separate
process test built the exact enrolled artifact, accepted one ACA2/corpus pair,
repeated the operation across both inspection and corpus floors, then accepted
the next serial and rejected an attempted rollback while retaining the
successor Target. The replacement test passed in Linux Docker on 2026-08-26 in
2.750 seconds. No URL, downloader, or Endpoint startup authority was added.

The separate-process C-2 tracer now opens that persistent floor and resolves
the alpha Service Link through `endpoint.ResolveAcceptedAlpha`; its User no
longer accepts corpus bytes or the corpus authority from the Publisher
publication envelope. On the selected Linux profile, its harness builds one
exact enrolled v3 bundle, invokes the separately manifested
`ardents-control accept-alpha-corpus` process over its ACA1 and ACA2 evidence,
then starts the C-2 User against the resulting floor. That joined Docker
process test passed on 2026-08-26 in 7.540 seconds. It proves this one local
process composition only; it is neither a source-provenance, capacity,
multi-host, nor public-alpha claim. Non-Linux C-2 compatibility tests retain
the same persistent-floor consumer boundary but do not qualify the control
command procedure.

Still outside this decision: concrete published alpha artifact provenance and a real
participant distribution/replacement source for the explicitly supplied ACA2
and corpus bytes. The participant procedure and immutable cohort-notice
template are now recorded in `docs/product/closed-alpha-name-corpus.md` and
`docs/product/closed-alpha-name-corpus-notice-template.md`; they deliberately
have no live URL or source identity until the Product Owner publishes one. Its future verified
bundle must also manifest the invoked `ardents-control` command, and that
command must run from the exact enrolled manifest entry: an arbitrary
separately downloaded executable is not part of the acceptance trust path. No
missing source is silently fetched and no catalog key becomes a Namespace
authority.

**Measurement:** on 2026-08-26, `git ls-remote --tags origin` against the
declared `dianabuilds/ardents-network` GitHub remote returned no tag refs.
Therefore no published Git tag can currently be cited as the cohort's concrete
release/source notice. This says nothing about a private or draft release; it
does not make the repository, a future tag, or GitHub itself an authority.
