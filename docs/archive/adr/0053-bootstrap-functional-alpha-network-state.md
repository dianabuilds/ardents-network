---
status: accepted
date: 2026-08-27
supersedes: none
---

# ADR-0053 — Bootstrap functional-alpha Network State with a separate 1-of-1 authority

## Context

ADR-0052 requires one real verifier-accepted Network State decision before the
fixed H4-alpha-1 inputs can be signed. Production State code verifies canonical
Epochs, while existing writers and keys are test-owned. Release custody cannot
gain another role without changing its selected encrypted record, and the
H4-6A Network key is disclosure-only. The active project has one Product Owner
and makes no independent-control, persistent-capacity, or availability claim.

## Decision

The `internal/network/state` Module owns one additional deep interface,
`InitializeAlphaGenesis`. It creates exactly one H4-alpha-1 genesis and one
separate encrypted Epoch seed record. It generates the Network identifier,
assignment seed, and Ed25519 Epoch key internally; fixes epoch `1`, threshold
`1`, `ardents-interactive-route-v1`, the four sorted Route role domains, and an
empty candidate/input view. It fixes validity to 30 days from the locally
observed whole-second initialization time; the later release request must
remain inside that interval.

The operation atomically writes one previously absent fixed child directory
under an existing owner-only local root. That child contains exactly one
encrypted record and one public request-fragment file, both outside the
repository. It verifies the constructed bytes through the maintained State
acceptance path before final publication. Its receipt
reports only the envelope digest, Network identifier, authority public key,
Epoch digest, profile, threshold, validity, canonical Epoch bytes, and empty
inputs/materializations.

The seed uses the accepted Argon2id/AES-256-GCM local custody profile with a
fresh salt and nonce. The sole terminal Adapter accepts one fixed absolute root
path and reads a new passphrase plus confirmation only through a trusted local
dialog or no-echo terminal. No interface returns a private seed, password,
derived key, or arbitrary signing capability.

The empty view is the actual initial alpha topology: no persistent Node is
claimed. H4-2 retains its separately labelled temporary qualification State and
two-host evidence. A non-empty successor, persistent Node identity, rotation,
recovery, threshold or hardware custody requires a later decision and bounded
operation.

## Consequences

- Release, disclosure, Network Epoch, Namespace, Node, and qualification keys
  remain distinct roles.
- The static H4-6A input can be real and reproducible without promoting a test
  fixture or fictitious operator.
- The release can prove authenticated enrollment and selected Carrier behavior,
  but cannot report Target Connect readiness, capacity, availability,
  independent operation, or Public Beta governance from this State.
- Losing the passphrase makes this provisional State authority unrecoverable;
  the finite State and release validity bound that failure.

## Compliance

- [ADR-0004](0004-authenticated-epochs-and-separated-control-roots.md)
- [ADR-0038](0038-alpha-control-disclosure-reader-v1.md)
- [ADR-0050](0050-separate-local-release-seed-custody.md)
- [ADR-0052](0052-build-fixed-alpha-static-inputs.md)
- [R-121](../research/records/r-121-functional-alpha-state-authority.md)
