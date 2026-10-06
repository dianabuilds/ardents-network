# Closed-alpha enrollment verification

Status: **current maintained technical contract.** This document defines the
bounded first-artifact verifier in `internal/enrollment`. It is not a
release procedure, download guide, installer, updater, or qualification
profile.

## Interface and ownership

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

After this manifest-only projection, `internal/endpoint/installation` now
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
the manifest-pinned Node and Authority Custody companions. It never executes
or installs any byte, writes a Release or control floor, downloads from a
source, or grants authority to a Release, State, Namespace, Route, or Endpoint.

The sole accepted Network enrollment descriptor grammar is
`ardents-closed-alpha-enrollment-v3` (ADR-0112). A recognized v1 or v2 schema
is refused with the typed exported `ErrLegacyEnrollmentDescriptor`; existing
legacy bundle bytes stay on disk unchanged and no converter or compatibility
reader exists.

Browser enrollment-v4 is not part of the maintained verifier or artifact
surface, and an unknown schema keeps its generic invalid refusal rather than
the typed retirement classification. Its former inputs and results remain only
in the non-executable compatibility evidence tree.

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
   roots, the pinned `corpus.pub` authority, the platform-named control
   executable, and the separately required headless Node/Custody pair when
   `VerifyHeadless` is used.

`ExecutableArtifactName` is the sole package-owned constructor for enrolled
command identities. It appends the declared platform to the command and also
appends the native executable suffix: accepted Windows `ardents-control`, Node,
Custody, and Endpoint command artifacts end in `.exe`; non-Windows names do
not. The sole accepted Network enrollment-v3 descriptor parsing and bundle
construction/testing use that same identity rather than reconstructing it
locally.

The verifier permits at most 32 inventory entries, each no larger than 64 MiB.
Names are direct file names only. Manifest and descriptor require canonical
newline-terminated forms; the v3 descriptor has one fixed field ordering and
fixed companion identities.

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

`VerifyHeadless` preserves the accepted enrollment-v3 `RELEASE` grammar but
requires the exact manifest to contain the canonical platform-named
`ardents-node` and `ardents-custody` companions. Those bytes are returned
outside Release metadata, alongside the already separate control artifact.
The distinction from ordinary `Verify` is companion inventory scope, not
version (ADR-0112): `Verify` accepts the same sole v3 grammar and the ADR-0042
inventory without the headless pair; a partial Node/Custody pair fails closed
on both entry points.

## Verification owner

### Isolated new owner

`internal/successor/enrollment` owns a separately implemented initial portable
verification flow consumed by `ardents-next enrollment verify` and
`verify-headless`, each taking only Bundle Root and independent manifest pin.
The command obtains its own original executable path from the OS. This is a
read-only initial verification consumer, not a new supported distribution or
installed runtime. The predecessor commands and roots remain independent.

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

### Predecessor owner

`internal/enrollment` behavior tests cover pin-before-parse,
inventory rejection, executable substitution, the typed refusal of retired
v1/v2 descriptors and the generic refusal of unknown schemas, v3 companion
separation including the partial-pair failure, package-owned artifact binding,
Windows v3 manifest acceptance through the running-artifact identity gate in
a re-executed enrolled child process. Callers in
`cmd/ardents` and `cmd/ardents-control` exercise this narrow Network enrollment
interface. Repository gates provide integration evidence;
historical RC2 enrollment evidence does not qualify a future baseline.
