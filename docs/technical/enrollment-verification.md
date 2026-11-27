# Closed-alpha enrollment verification

Status: **current maintained technical contract.** This document defines the
bounded first-artifact verification contract. The maintained
`ardents-next enrollment verify` consumes `internal/domain/enrollment`. The
independent pin authenticates the exact v6 inventory.

## Interface and ownership

[ADR-0123](../adr/0123-bind-installed-infrastructure-generations.md) accepts a
separate closed infrastructure initial inventory and actual-running-program
verifier. Its exact grammar belongs to the [infrastructure contract](network-route-node.md#proposed-exact-artifact-and-installation-binding).
The isolated new owner implements this separate Source verifier through
`ardents-next infrastructure authenticate-initial`. It does not extend the
Endpoint inventory or turn a candidate into independent initial provenance.
`infrastructure-enrollment.json` is pin-checked before parsing and binds exactly
the seven public files. The snapshot retains original loader cancellation after
all physical input descriptors join; Release/admission recheck it at handoff.
Actual Linux mapped-executable inode and bytes must equal the enrolled program;
same-looking paths or a copied public receipt cannot supply that result.
Native infrastructure admission is restricted to Linux amd64. Unsupported
commands refuse before opening any artifact or history.

`internal/domain/enrollment/infrastructureinventory` owns the portable
canonical grammar, fixed role companions and byte bounds. Offline Release
preparation constructs the public inventory from the seven bounded byte images.
Native Enrollment parses that same grammar after the independent pin check for
an initial Bundle. The portable byte oracles independently specify Source,
Forwarding, JOIN and Descriptor encodings.

`infrastructureinventory.Validate` checks the canonical document against all
seven bounded supplied byte images. Its copied declarations and fresh digest
map supply no initial pin, native file custody,
running-program proof, Candidate, Bundle or installation permission. A caller
may compare the same image's digests with other public commitments within that
one validation; independently pinned loading and executable verification remain
separate obligations.

[ADR-0119](../adr/0119-bind-protected-endpoint-generation-to-release.md)
binds the complete protected resource group to the same Release set as the
Endpoint program. Enrollment checks the protected descriptor and resource
digests and projects their frozen bytes separately. Release independently
authorizes the signed program and protected generation. Endpoint launch uses
those exact proofs to stage and observe the installed generation.

The descriptor is compact UTF-8 JSON followed by one LF, with the five fields
in the order stated by ADR-0119 and `files` keys in lexical order. Re-encoding
must reproduce the exact input; unknown/duplicate fields, alternate whitespace
or ordering refuse. The maximum is 16 KiB, version is a positive int64, platform
is `linux-amd64`, and release identity matches the enrolled release. Exactly
nine lowercase SHA-256 digests bind the actual program/resource bytes. The
protected group is optional in v6. Enrollment rejects a partial group.

`Verify(Request)` accepts one local Bundle Root, current executable path,
independently delivered Alpha Enrollment Pin, declared local environment,
network, target path, architecture, and reference time. The only supported
Installed-profile variation supplies one explicit package-owned executable in
`ArtifactPath`; all remaining enrolled static files remain below Bundle Root.

It returns a `Bundle` that retains the exact bytes for Release evaluation
and the complete protected group when selected. Release input projection excludes the static inventory.

[ADR-0131](../adr/0131-distribute-authority-custody-separately.md) defines the
sole descriptor, `ardents-closed-alpha-enrollment-v6`. It names the cohort,
release, platform, environment, network, target path, Endpoint artifact and
trusted root in that order after the schema. The inventory requires the
canonical platform-named Endpoint executable, a canonical versioned trusted root,
timestamp metadata and one direct snapshot and targets metadata file.
Additional root metadata uses canonical versioned names. Release authenticates
all metadata and owns threshold, expiry, rotation and retained trust policy.
Prior descriptor versions return an inventory error. A previously installed
v5 original remains authenticated by its own sealed binding and the retained
Release history. The v6 command stages a strictly newer signed successor only
after observing the original platform unit and Worker scopes cleanly stopped.
The successor uses the original Release history and the exact mutable root
identities. The archive's initial pin cannot authorize replacement.

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
7. Project the authenticated Endpoint bytes and the optional
   protected group separately from versioned Release metadata.

`ExecutableArtifactName` supplies platform-named executable identities.
On Windows these names include `.exe`. The verifier permits at most 32
inventory entries of at most 64 MiB each and admits only direct file names.
The manifest and descriptor are canonical and newline-terminated.

## Failure behavior

Enrollment refuses a changed manifest before interpreting it. It rejects a
changed descriptor, a missing or undeclared file, an incomplete protected group
and a different running executable. Older descriptors refuse after verification
of the pin and descriptor digest and before reading companion files. Refusal
provides no authenticated initial facts or state mutation.

## Verification owner

### Isolated new owner

`internal/domain/enrollment` owns portable initial verification.
`ardents-next enrollment verify` takes the Bundle Root and the independent
manifest pin. The command obtains the original executable path from the OS
and reports the verified inventory and whether it includes protected resources.

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
and establish its own authorization. The same companion and protected-group rules apply to independent candidate loading. The
[new owner design](../../internal/domain/enrollment/README.md) records the
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
The [Installation contract](release-update-custody.md#release-and-update-ownership)
has a genuine consumer establishing two fresh Release authorizations against
complete retained Release history. Snapshot loading itself establishes neither
proof. Installed predecessor binding, forward generation continuity and actual
transition remain separate unimplemented obligations before successor effects.

The alpha-bundle packaging producers use the new owner's public
`ExecutableArtifactName` and existing `ValidateProtectedGeneration`. Both the
bundle verifier and packaging use the same artifact-name constructor, including
Windows `.exe` identities; the helper formats declared names without selecting
platform support or attesting artifact provenance. Packaging calls
`ardents-next enrollment artifact-name` and
`ardents-next enrollment check-protected-generation` from reviewed source. The
protected checker reads the descriptor whose exact bytes Release must bind.

The maintained Enrollment tests cover pin-before-parse, inventory rejection,
executable substitution, retired descriptor refusal, and separation of v6
static inventory from Release metadata. The headless artifact lane exercises
these checks at the maintained owner. Historical receipts remain bound to
their original artifacts.
