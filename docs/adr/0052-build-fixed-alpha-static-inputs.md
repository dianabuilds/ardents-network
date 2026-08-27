---
status: accepted
date: 2026-08-27
supersedes: none
---

# ADR-0052 — Build only fixed closed-alpha static inputs from local custody

## Context

ADR-0050 creates the one-person local encrypted release-seed record, and
ADR-0051 can recover its public receipt without exporting a secret. R-120 must
now create the exact H4-1/H4-6A static input directory from that record without
turning the local workstation into a general signing service. The first profile
is recorded in `docs/product/horizon-4/08b-alpha-1-release-profile.md`, but
its actual Network State and two-builder release facts remain gates.

## Decision

`internal/release/custody` will own one additional deep Module interface:
`BuildAlphaInputs`. It accepts one versioned, non-secret fixed-alpha request,
the already selected encrypted record root, the exact Endpoint and control
artifact bytes, and one previously absent external output directory. It may
write only these fixed direct regular files:

```
1.root.json       1.targets.json      1.snapshot.json      timestamp.json
catalog.ac1       catalog.pub
release.ac1       release.pub
network.ac1       network.pub
compatibility.ac1 compatibility.pub
corpus.pub        RELEASE
```

The public request may declare only the profile's cohort/release identity,
validity/emergency bounds, exact `linux-amd64` target identity, two named
builder attestations, and complete offline Network State decision. It cannot
select a signer role, arbitrary bytes/message, target path, root threshold,
metadata version, source URL, mirror, publication destination, or key. The
operation derives keys internally from the decrypted record, fixes all TUF
top-level roles at 3-of-5, fixes ACA1 to the three independently rooted
Release/Network/Compatibility components, and derives every public companion
from those same seeds.

The exported operation is bound to the recorded `ardents-h4-alpha-1-v1`
profile: its selected encrypted-envelope digest, Endpoint/control digests, and
Endpoint source revision are fixed policy, not caller choices. A recorded
reference time drives deterministic verifier evidence, while the actual local
invocation time must still precede metadata expiry and the no-new-work safety
bound.

Before external publication it validates the request's Network State through
the maintained State acceptance path, evaluates the constructed TUF set through
the maintained Release verifier for the exact artifact, and verifies every
constructed ACA1/ACS1 component against those outcomes. It stages outside the
requested output root and publishes the complete directory atomically only
after all checks pass. Its receipt contains only request, input, output, and
preflight digests/identities; it never contains a secret or first-contact
authority.

The sole future terminal adapter is
`ardents-release-custody assemble --root ... --request ... --endpoint ...
--control ... --output ...`. It obtains the record passphrase through the
existing trusted local adapter. It has no network, upload, shell-command,
interactive topology, or arbitrary signing route.

## Consequences

- One small interface contains role ordering, canonical metadata construction,
  cross-component binding, verifier preflight, secret zeroing, and atomic
  output, rather than distributing those security decisions among shell steps.
- The operation creates candidate inputs only. It does not assemble the bundle,
  publish it, contact a participant, execute an artifact, or make a release
  accepted.
- A real invocation remains blocked by the public profile facts named in its
  "Required static inputs" section. Synthetic test inputs may prove module
  behavior but can never become release material.
- Threshold and independent-control claims remain false: the initial operation
  is one custodian holding five local TUF keys. Rotation, hardware custody,
  multi-device custody, or another release profile requires a later ADR.

## Compliance

- [ADR-0015](0015-separate-release-decision-from-local-activation.md)
- [ADR-0038](0038-alpha-control-disclosure-reader-v1.md)
- [ADR-0050](0050-separate-local-release-seed-custody.md)
- [ADR-0051](0051-confirm-local-release-seed-public-receipt.md)
- [R-120](../research/records/r-120-bounded-alpha-input-signing.md)
