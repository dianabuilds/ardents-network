---
id: R-073
title: What signed Name Record envelope remains compatible with the retained fixed current-proof profile?
status: accepted
owner: Product Owner and Codex
started: 2026-08-23
reviewed: 2026-08-23
---

# R-073 — Record and current-proof envelope

## Decision this unlocks

F031 cannot seal Namespace constructors while `SignRecord` accepts a 16 MiB
payload but an installed Record must later fit the retained 4,096-byte current
proof. The new compatibility bound must reject an unresolvable value before
signing, without changing R-067's Ed25519 transcript or fixed proof profile.

## Current contract

R-067 retains Record signing and fixed private-resolution/current-proof bytes.
R-066 retains only the 127-record technical tracer, not product scale. A
signed Record is embedded in a materialization leaf, which in turn appears in
the threshold-attested proof; the maximum attestation authority set is 16.

## Hypotheses

- **H1:** a measured signed-Record limit, below the worst-case retained proof
  envelope, lets Namespace reject incompatible Record input before signing.
- **H2:** retain the 16 MiB Record payload ceiling and let Store/Lookup reject
  an otherwise signed Record after installation.
- **H0:** the retained 4,096-byte proof cannot carry any useful signed Record
  under the accepted 127-record/16-authority technical tracer.

## Evaluation criteria

The experiment must use 127 Records, all 16 valid threshold signatures, a
worst-case materialization membership path, and a canonical V4 Record with
variable signed content. It must report the largest exact fit and demonstrate
that the next byte fails the fixed proof bound. The final limit needs explicit
headroom rather than treating a zero-margin maximum as a product capacity.

## Evidence plan

`experiments/r-073-record-proof-envelope/` builds the complete tracer corpus,
binary-searches the variable canonical Record field, signs the complete
threshold statement with 16 accepted authorities, and records the exact
signed-Record/proof sizes. It uses a fresh temporary Namespace store per
candidate and removes it after the run.

## Failure scenarios

- a Record signs successfully but cannot be returned in the fixed proof;
- fewer than the maximum threshold signatures hide the real envelope;
- a shallow membership path hides the maximum corpus proof overhead; and
- a new limit silently changes the signature domain or accepts an oversized
  legacy Record as resolution-safe.

## Findings

- **Measurement (2026-08-23):** the reproducible experiment installed 127
  Records, used all 16 valid materialization signatures, selected the longest
  membership path, and found that a 1,996-byte signed Record produces an exact
  4,096-byte proof. Adding one byte failed the proof bound.
- **Inference:** 1,996 bytes is a zero-margin implementation maximum, not a
  safe compatibility rule. A 1,920-byte signed container leaves 76 bytes in
  this complete retained tracer shape and corresponds to a 1,846-byte canonical
  Record payload after the 74-byte selected framing/signature overhead.
- **Inspection:** the former 16 MiB limit could sign an input that durable
  materialization accepted but `Lookup` could not return through the fixed
  proof envelope.

## Recommendation

Accept H1. Limit a newly signed/verified Record to a 1,846-byte canonical
payload and a 1,920-byte signed container. Keep the existing Record schema,
Ed25519 transcript domain, outer framing, and 4,096-byte proof unchanged.
Reject an oversized value in `SignRecord` and `VerifyRecord` before it can
become pending or current state. This is a technical tracer compatibility bound
only; it neither selects product capacity nor widens the 127-record envelope.

The strongest counterargument is that a future legitimate Record could need
more than 1,846 bytes. That requires a new compatible proof/profile design and
measurement, rather than creating an already signed but unresolvable value.

## Disposition

**Accepted H1 on 2026-08-23 under the Product Owner's standing Stage 8
authority.** No ADR is needed: R-067's retained signature, framing, and proof
profiles remain unchanged, while the former 16 MiB implementation ceiling had
no named observer. Retain the experiment and the early-rejection unit test;
F031's final sealed constructors must apply this same compatibility table.
