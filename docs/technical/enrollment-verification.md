# Closed-alpha enrollment verification

Status: **current maintained technical contract.** This document defines the
bounded first-artifact verifier in `internal/enrollment`. It is not a
release procedure, download guide, installer, updater, or qualification
profile.

## Interface and ownership

`Verify(Request)` accepts one local Bundle Root, current executable path,
independently delivered Alpha Enrollment Pin, declared local environment,
network, target path, architecture, and reference time. The only supported
Installed-profile variation supplies one explicit package-owned executable in
`ArtifactPath`; all remaining enrolled static files remain below Bundle Root.

It returns `Verified`: exact bytes for Release Decision plus separately scoped
alpha-control and corpus companions; `VerifyHeadless` additionally requires
the manifest-pinned Node and Authority Custody companions. It never executes
or installs any byte, writes a Release or control floor, downloads from a
source, or grants authority to a Release, State, Namespace, Route, or Endpoint.

Browser enrollment-v4 is not part of the maintained verifier or artifact
surface. Its former inputs and results remain only in the non-executable
compatibility evidence tree.

The Endpoint adapter also retains a parser for the complete historical
`ardents-alpha-enrollment-input-v1` document when an already generated Portable
user unit invokes its original three-argument command. This is an execution
compatibility bridge, not an enrollment format: it derives the same verifier
request, including the manifest digest, and new C0 enrollment and newly rendered
units receive the bundle root and independently delivered pin as separate
arguments.

## Acceptance sequence

1. Reject an incomplete request and resolve Bundle Root to an absolute path.
2. For an Installed profile, prove that Bundle Root is a direct package-owned
   static directory.
3. Read `SHA256SUMS` as a bounded owned regular file and compare its SHA-256
   against the independent Pin **before parsing any manifest content**.
4. Parse the canonical sorted manifest and `RELEASE` descriptor; reject an
   unknown, duplicate, missing, non-regular, symlink, oversized, or unowned
   static entry.
5. Read every declared byte, compare every digest, and prove the descriptor
agrees with the caller's cohort/release/platform/environment/network/target
facts.
6. Prove the running executable is the identical bundled artifact or the one
   declared package-owned artifact, then construct Release Decision inputs.
7. Project disclosed companions outside Release metadata: catalog and its
   roots, optional `corpus.pub`, the v3 control executable, and the separately
   required headless Node/Custody pair when `VerifyHeadless` is used.

`ExecutableArtifactName` is the sole package-owned constructor for enrolled
command identities. It appends the declared platform to the command and also
appends the native executable suffix: accepted Windows `ardents-control`, Node,
Custody, and Endpoint command artifacts end in `.exe`; non-Windows names do
not. The accepted Network enrollment-v1--v3 descriptor parsing, bundle
construction/testing, and running-companion verification use that same
identity rather than reconstructing it locally.

The verifier permits at most 32 inventory entries, each no larger than 64 MiB.
Names are direct file names only. Manifest and descriptor require canonical
newline-terminated forms; v1--v3 descriptors have fixed field ordering and
fixed companion identities.

## Failure behavior

Any mismatch returns an error without execution or state mutation. In
particular, a changed manifest fails before descriptor parsing; an undeclared
file or a bundled copy of an external Installed artifact fails inventory
validation; a different current executable fails exact-file identity and byte
comparison; and an undeclared or malformed companion never crosses into
Release metadata.

`VerifyRunningCompanion` is narrower: after `Verify` has already returned the
manifest bytes, it proves that the current process is exactly one named
companion from that inventory. It neither re-verifies the bundle nor executes
the companion.

`VerifyHeadless` preserves the accepted enrollment-v3 `RELEASE` grammar but
requires the exact manifest to contain the canonical platform-named
`ardents-node` and `ardents-custody` companions. Those bytes are returned
outside Release metadata, alongside the already separate control artifact.
Ordinary `Verify` continues to accept the narrower ADR-0042 v3 inventory for
its historical corpus-control use; a partial Node/Custody pair fails closed.

## Verification owner

`internal/enrollment` behavior tests cover pin-before-parse,
inventory rejection, executable substitution, v2/v3 companion separation,
package-owned artifact binding, Windows v3 control-manifest acceptance through
the running-companion contract, and a current companion process. Callers in
`cmd/ardents` and `cmd/ardents-control` exercise this narrow Network enrollment
interface. Repository gates provide integration evidence;
historical RC2 enrollment evidence does not qualify a future baseline.
