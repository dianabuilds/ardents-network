# ADR 0009: Immutable and verifiable release materials

- Status: Accepted
- Date: 2026-07-24
- Decision owners: Release, CI, Supply Chain

## Context

Release builds previously selected Go, PowerShell, and Debian images by mutable
tags. The two reproducibility builds ran sequentially on one runner and shared
named Go and BuildKit caches. The local SLSA statement identified the Git commit
and output subjects, but did not bind the source tree bytes, toolchain image, or
runtime and metadata base images. Its verifier checked labels and subjects but
did not compare build materials with a trusted policy.

A repeated build in one mutable environment can reproduce the same compromised
output. Checksums around a provenance document also cannot detect a substituted
material when an attacker can recompute the outer checksum.

## Decision

`scripts/release/materials.json` is the checked-in release-material policy. It
contains the canonical source repository and every builder, toolchain, runtime,
metadata, test-runner, and smoke-verifier image as a platform-specific
`sha256` reference. Release Dockerfiles and workflow containers may use only
references present in that policy.

The source material digest is SHA-256 over the canonical `git archive` tar of
the exact release commit. The generated SLSA v1 statement records that source
descriptor plus one descriptor for every policy image, including its role and
kind. The builder identity is the same immutable Go image used for the native
bundle.

The release verifier independently derives or receives the expected source
archive digest and loads the trusted policy from the exact expected Git commit.
The self-check inside an extracted build snapshot uses the policy in that same
digest-bound snapshot. The provenance material set must match the selected
policy exactly. Recomputing `SHA256SUMS` after changing a source, toolchain, or
base descriptor therefore does not make the release valid.

The two release builds run as a CI matrix on separate clean hosted runners.
Each native build uses anonymous disposable Go caches; image builds disable
layer reuse and release Dockerfiles do not use persistent cache mounts. The
comparison job downloads both outputs, compares their manifests, and verifies
the selected candidate before publication.

## Consequences

- Updating a release image requires an explicit policy and Dockerfile change.
- Registry tag movement cannot alter a release input.
- Release verification requires the repository policy for the release commit;
  the unsigned statement remains metadata, not an independent signature.
- Clean rebuilds cost more network and compilation time in exchange for
  independent evidence.
- Go modules remain application dependencies represented by `go.sum` and the
  CycloneDX SBOM; they are not mislabeled as resolved materials with empty
  digests.
