# Closed-alpha enrollment verification

Status: **current maintained technical contract.** This document defines the
bounded first-artifact verification contract. The maintained
`endpoint enrollment-check` command boundary consumes
`internal/successor/enrollment`. The current v3 inventory and refusal rules
apply to the exact enrolled artifacts.

## Interface and ownership

[ADR-0123](../adr/0123-bind-installed-infrastructure-generations.md) accepts a
separate closed infrastructure initial inventory and actual-running-program
verifier. Its exact grammar belongs to the [infrastructure contract](network-route-node.md#proposed-exact-artifact-and-installation-binding).
The isolated new owner implements this separate Source verifier through
`ardents-next infrastructure authenticate-initial`. It does not extend the
Endpoint/v3 inventory or turn a candidate into independent initial provenance.
`infrastructure-enrollment.json` is pin-checked before parsing and binds exactly
the seven public files. The snapshot retains original loader cancellation after
all physical input descriptors join; Release/admission recheck it at handoff.
Actual Linux mapped-executable inode and bytes must equal the enrolled program;
same-looking paths or a copied public receipt cannot supply that result.
This Source extension and its canonical byte oracles compile only on the selected
Linux profile with their real consumer. Existing Endpoint/v3 grammar and tests
remain portable; unsupported commands refuse before opening any artifact/history.

The new owner's `ValidateInfrastructureInventory` checks a canonical Source,
Forwarding or JOIN inventory against all seven bounded supplied byte images. Its copied
declarations and fresh digest map supply no initial pin, native file custody,
running-program proof, Candidate, Bundle or installation permission. A caller
may compare the same image's digests with other public commitments within that
one validation; independently pinned loading and executable verification remain
separate obligations.

[ADR-0119](../adr/0119-bind-protected-endpoint-generation-to-release.md)
selects the protected distribution successor: one initial pin and bundle,
unchanged Network v3 descriptor, and a complete protected resource group with
`protected-endpoint.json` projected outside Release metadata. General/headless
acceptance retains its existing scope; protected readiness requires the new
complete-inventory boundary. Partial group presence must refuse. This selected
verifier now checks the complete group's canonical descriptor and resource
digests and projects those bytes separately. This is manifest authentication,
not the required fresh Release generation authorization or an installed
qualification receipt; the installation consumer remains unfinished.

After this manifest-only projection, `cmd/ardents/protected_enrollment.go`
composes fresh program and generation Release proofs from the same frozen
metadata/local/reference inputs. The enrolled command uses this composition
when the complete protected group is present, and passes only the program proof
to executable replacement. Its existing general readiness is not protected
readiness. Immutable generation installation and actual system-manager binding
remain separate unfinished obligations.

The descriptor is compact UTF-8 JSON followed by one LF, with the five fields
in the order stated by ADR-0119 and `files` keys in lexical order. Re-encoding
must reproduce the exact input; unknown/duplicate fields, alternate whitespace
or ordering refuse. The maximum is 16 KiB, version is a positive int64, platform
is `linux-amd64`, and release identity matches the enrolled release. Exactly
nine lowercase SHA-256 digests bind the actual program/resource bytes. The
whole protected companion group is optional for existing general v3 bundles;
any partial group refuses in both `Verify` and `VerifyHeadless`.

`Verify(Request)` accepts one local Bundle Root, current executable path,
independently delivered Alpha Enrollment Pin, declared local environment,
network, target path, architecture, and reference time. The only supported
Installed-profile variation supplies one explicit package-owned executable in
`ArtifactPath`; all remaining enrolled static files remain below Bundle Root.

It returns `Verified`: exact bytes for Release Decision plus separately scoped
alpha-control, corpus and optional protected-generation companions;
`ProtectedDescriptor` and `ProtectedFiles` grant no installation authority and
are excluded from `Inputs.Files`. `VerifyHeadless` additionally requires
the manifest-pinned headless companion inventory for the descriptor schema.
It never executes
or installs any byte, writes a Release or control floor, downloads from a
source, or grants authority to a Release, State, Namespace, Route, or Endpoint.

The accepted Network enrollment schemas are
`ardents-closed-alpha-enrollment-v3` (ADR-0112) and
`ardents-closed-alpha-enrollment-v4` (ADR-0128). Recognized v1 and v2 schemas
are refused with the typed exported `ErrLegacyEnrollmentDescriptor`.
Browser enrollment remains outside this verifier. An unknown schema returns
a generic inventory error.

The historical `ardents-alpha-enrollment-input-v1` persisted document is
retired (ADR-0127, #647). Its former consumers — the three-argument
`endpoint enrollment-check <alpha-enrollment.json>` local diagnosis and
`ardents-control inspect-bundle` — return typed before-effect refusals, the
v1-document parser is removed, and the per-user `endpoint enroll` /
`endpoint user-unit` start path that once invoked it was already retired
(#617 S3.5). Existing documents stay on disk byte-for-byte with no reader,
converter, or deleter. The four-argument bundle-root+pin form is the sole
`enrollment-check` input; new C0 enrollment and platform delivery receive the
bundle root and independently delivered pin as separate arguments.

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
   roots, the pinned `corpus.pub` authority, the platform-named control
   executable, and the headless companions required by the pinned schema.

`ExecutableArtifactName` is the sole package-owned constructor for enrolled
command identities. It appends the declared platform to the command and also
appends the native executable suffix: accepted Windows `ardents-control`, Node,
Custody, and Endpoint command artifacts end in `.exe`; non-Windows names do
not. Enrollment verification and bundle construction use the same identity.

The verifier permits at most 32 inventory entries, each no larger than 64 MiB.
Names are direct file names only. Manifest and descriptor require canonical
newline-terminated forms; both schemas share the same fixed field ordering.

## Failure behavior

Any mismatch returns an error without execution or state mutation. In
particular, a changed manifest fails before descriptor parsing; an undeclared
file or a bundled copy of an external Installed artifact fails inventory
validation; a different current executable fails exact-file identity and byte
comparison; a recognized retired v1/v2 descriptor returns the typed
`ErrLegacyEnrollmentDescriptor` after the pin and descriptor-digest checks and
before any companion inventory or executable identity work; and an undeclared
or malformed companion never crosses into Release metadata.

The former `VerifyRunningCompanion` companion proof was retired by ADR-0113;
`Verify`'s own running-artifact gate (`exactExecutable`) remains the single
exact-file identity check, and the Windows v3 re-execution contract proves it
against the enrolled endpoint artifact.

A v3 headless bundle requires the platform-named Node and Custody pair.
Ordinary `Verify` also accepts the original v3 inventory with neither
companion. Both scopes reject a partial pair. A v4 bundle requires the
platform-named Custody artifact and rejects the legacy Node companion in
either scope. Companion bytes remain outside Release metadata.

## Verification owner

### Isolated new owner

`internal/successor/enrollment` owns a separately implemented initial portable
verification flow consumed by `ardents-next enrollment verify` and
`verify-headless` and by the read-only `ardents endpoint enrollment-check`
bundle-root+pin diagnostic (ADR-0127), each taking only Bundle Root and
independent manifest pin. Each command obtains its own original executable
path from the OS. These are read-only initial verification consumers, not a
new supported distribution or installed runtime; the enrollment-check
diagnostic renders only authenticated initial facts and grants no readiness.
The predecessor commands and roots remain independent.

The new owner authenticates pin before parsing, reads each file once into an
owned snapshot with actual byte bounds, limits directory enumeration, verifies
original file identity before/after I/O and at final handoff, and refuses
observed original cancellation without a partial result. Unix ownership and
no-follow/nonblocking open are separate native mechanisms; Windows portable
verification attests regular-file identity and bytes, not Unix ownership or
installed ACL qualification. Common grammar and behavior tests are portable.

The result has private construction and returns copied authenticated initial
facts/bytes. No Release inputs, metadata URLs, permission or durable floor is
created here. Future new Release composition must consume these exact bytes
and establish its own authorization. General/headless companion and protected
group rules remain those above, with no first-pin successor authority. The
[new owner design](../../internal/successor/enrollment/README.md) records the
stages and refusal obligations; the selected GitHub issue owns verification
and acceptance status.

The real new Release consumer now uses `Bundle.MetadataNames` for the frozen
metadata/static inventory classification. Enrollment retains its existing
companion grammar, including platform suffixes and the protected resources;
composition owns fixed metadata URLs and Release inputs. This read-only
projection grants no signature authority and preserves all companion bytes for
separately owned consumers. `ReadCandidate` separately loads an opaque,
self-consistent immutable `Candidate` with the same bounded physical inventory
checks. Its declarations and observed manifest checksum are untrusted; it has
no independent-pin or running-program provenance and cannot become a `Bundle`.
The [Installation design](../../internal/successor/installation/README.md)
has a genuine consumer establishing two fresh Release authorizations against
complete retained Release history. Snapshot loading itself establishes neither
proof. Installed predecessor binding, forward generation continuity and actual
transition remain separate unimplemented obligations before successor effects.

The alpha-bundle packaging producers use the new owner's public
`ExecutableArtifactName` and existing `ValidateProtectedGeneration`. Both the
bundle verifier and packaging use the same artifact-name constructor, including
Windows `.exe` identities; the helper formats declared names without selecting
platform support or attesting artifact provenance. The two Go script entry points
retain their existing argv, stdout and exit-status contract and static-byte
refusal boundary. The protected
checker never rewrites the descriptor whose exact bytes Release must bind.

The maintained Enrollment tests cover pin-before-parse, inventory rejection,
executable substitution, retired descriptor refusal, and separation of v3 and v4
companion bytes from Release metadata. The headless artifact lane exercises
these checks at the maintained owner. Historical receipts remain bound to
their original artifacts.
