---
id: R-170
title: One authenticated protected Endpoint artifact
status: decided
owner: Codex with Product Owner
started: 2026-09-30
reviewed: 2026-09-30
---

# R-170 — One authenticated protected Endpoint artifact

## Decision this unlocks

Define the exact enrolled inventory and compatibility boundary needed by the
supported protected Endpoint installer in issue #368. The Product Owner chose
one authenticated artifact and one independently delivered pin on 2026-09-30.
That direction does not accept a new grammar or implement/qualify installation.
The supplemental independently pinned worker bundle is rejected: maintaining
two trust inputs and their version combinations adds ongoing operational work.

## Current contract

The [product scope](../../product/scope.md),
[threat model](../../security/threat-model.md),
[enrollment owner](../../technical/enrollment-verification.md),
[release owner](../../technical/release-update-custody.md),
[confinement owner](../../technical/application-confinement.md) and
[runtime owner](../../technical/endpoint-service-runtime.md) remain binding.
[ADR-0112](../../adr/0112-pin-network-enrollment-v3-sole-descriptor.md)
accepts only Network enrollment-v3 and refuses recognized v1/v2 before
companion checks. General verification and headless verification differ in
inventory requirements, never accepted descriptor versions.

An independent pin authenticates exact manifest bytes before parsing. It is
not a signing key, successor authorization or permission to execute. Release
Decision and replacement authorization retain their own authority. Mutable
roots, local principals and installation-specific plans are separate inputs.
No Authority private keys or default test permissions belong in the artifact.

## Hypotheses

- **H1:** A bounded exact companion inventory can extend v3 without descriptor
  changes; older v3 bundles keep their existing enrollment meaning and are
  explicitly refused by the new protected-installation consumer when incomplete.
- **H2:** Exact protected inventory or authority requires descriptor fields that
  cannot be expressed under v3; a coordinated new Network grammar and explicit
  retirement decision are required before implementation.
- **H0:** Neither option closes first-execution trust, Release binding and
  installation authority without a second trust chain or weaker launch path.

## Evaluation criteria

Before any experiment, require:

1. One independently authenticated SHA256SUMS identifies all program and static
   resource bytes before privileged execution. Reject self-authentication by
   downloaded installer code, extra/missing files and digest substitution.
2. Every maintained enrollment caller has an explicit version/inventory
   outcome. Preserve typed retired-version refusal and manifest-pin precedence;
   do not silently treat an old v3 bundle as a complete protected installation.
3. Keep finite existing bounds (32 files, 64 MiB per file) unless separately
   justified. No new runtime dependency or cryptographic primitive is needed.
4. Each installed destination, owner, mode and templated input has one producer.
   A pinned unit template is not the digest of its installation-specific output.
   Plans cannot mint Release, Network, Custody or holder authority.
5. Partial generation, substituted file/ancestor, wrong executable/unit/PID,
   interrupted replacement and restart retain refusal and durable floors.
   Filesystem publication and system-manager transitions are separate operations.
6. The same public commands support two separate Ubuntu system managers,
   principals and roots, both selected Carriers and honest restart outcomes.
   Component fixtures do not prove that installed outcome. No latency or
   availability guarantee follows from packaging; no new accepted platform.
7. One human Product Owner and Codex can maintain the producer/verifier,
   independent pin delivery, update and recovery procedure. Reuse maintained
   components and licenses; any newly distributed static component needs an
   explicit provenance/license inventory before acceptance.

## Evidence plan

### Primary sources

Repository source inspected on 2026-09-30 at main-workspace commit `126761e9`:
`internal/enrollment/{enrollment,inventory,descriptor,companion}.go`,
`internal/endpoint/worker/artifact_linux.go`, the current owners linked above,
and ADR-0112. Follow actual packaging callers before selecting a schema.
Repository source is primary evidence for present behavior, not proof of a
supported installed workflow or independent security review.

### Experiment

First produce a source-derived matrix of all inventory producers and consumers.
Then, only after exact candidate rules are recorded, use a disposable bounded
probe outside maintained runtime to compare old-v3, complete proposed bundle,
partial worker resources, duplicate/unknown entries, swapped platform executable,
wrong manifest pin and recognized retired descriptors. Record immutable source,
actual bytes, commands and original failures outside Git. A parser/probe pass
is component evidence. Installed two-Endpoint qualification remains separate.
The first existing-verifier probe is reproduced under
[`experiments/r-170-enrollment-inventory`](../../../experiments/r-170-enrollment-inventory/README.md).
It does not implement either candidate rule.

### Failure scenarios

Downloaded provisioner used before independent verification; independently
valid but mismatched program/worker generations; old v3 bundle missing worker;
extra file; wrong role template; local root-generated checksum mistaken for
Release provenance; service-account writable plans; substituted ancestor;
partial publication; failed manager reload; wrong MainPID/executable; interrupted
replacement; restarted invocation with unavailable Service credential continuity.

## Findings

- **Sourced fact:** `Pin` identifies cohort/release/platform and exact manifest
  SHA-256. Verification checks this pin before parsing manifest content.
- **Sourced fact:** The manifest is sorted, newline terminated, bounded to 32
  direct file names; `exactInventory` rejects unknown/missing directory entries.
  The Installed profile permits only its one named external executable.
- **Sourced fact:** The v3 descriptor has fixed ordered fields; it accepts no
  additional descriptor fields. Node/Custody companions already distinguish
  headless inventory from ordinary verification without a different schema.
- **Sourced fact:** The local worker manifest authenticates six installed file
  digests and root-controlled filesystem identity. It does not establish that
  those digests belong to an independently accepted enrolled Release.
- **Inference:** Existing inventory-scope separation makes H1 plausible, but
  source inspection has not established how every producer, Release operation
  and installation consumer would project added static resources. H1 is unproven.

### Producer and consumer matrix

| Boundary | Present behavior | Required decision |
| --- | --- | --- |
| `Makefile` headless-build | Builds the four commands in the headless profile; outputs live outside Git. | Add the text command only to the selected protected distribution, with exact platform identity. |
| `packaging/alpha-bundle/build.sh` | Copies four binaries plus 14 static files, rejects other static-root entries, emits one sorted manifest and deterministic archive. | Name all added static files and their platform restrictions; 18 existing entries leave 14 entries under the current cap. |
| `enrollment.Verify` | Authenticates every manifest entry; checks exact directory membership; projects recognized companions separately and passes remaining files to Release metadata. | Added worker/resources need an explicit projection. Merely adding them to the manifest does not give them a non-metadata meaning. |
| `enrollment.VerifyHeadless` | Calls the same verifier, additionally requires the Node/Custody pair. | Protected installation needs a distinct mandatory complete inventory boundary; old headless readiness does not prove it. |
| `cmd/ardents` enrollment-check | Both maintained forms use general verification. | Preserve read-only diagnostic meaning and explicit version/refusal precedence. |
| `cmd/ardents` first-run enrollment | Uses headless verification. | Decide whether this caller remains its existing runtime scope or is retired in favor of protected installation. No implicit launch switch. |
| `cmd/ardents` enroll-installed | General verification with one package-owned external program; renders the retained per-user path. | Its retirement is tied to the protected system-unit successor under ADR-0112; no additional external inventory exception is implied. |
| `alphacontrol/inspection` | General verification before read-only inspection. | Preserve control/Release projection and refuse incomplete protected inventory wherever that new scope is requested. |
| `packaging/ubuntu-deb/build.sh` | Packages one Endpoint executable and static enrollment files, creates a launcher; no protected text inventory producer. | Decide its exact retirement/replacement outcome. Do not claim existing deb packaging supplies the protected system launch. |
| `worker.LoadArtifact(Text)` | Root-installed local manifest checks text executable, four worker units/sockets and stop rule. | Bind these exact installed digests to the authenticated bundle; local root ownership alone is insufficient. |

**Sourced fact:** Additional independently pinned names are not inherently
rejected by the general verifier: names outside its recognized projection enter
`release.Inputs.Files` through `release.MetadataURL(name)`. Directory membership
exactness rejects files omitted from the manifest, not every unfamiliar name
that is itself declared. Therefore H1 requires a deliberate projection and
complete-inventory rule, rather than packaging-only changes. This distinction
corrects the overly broad shorthand "unknown inventory"; it is not evidence of
an exploitable Release bypass.

**Sourced fact:** The text resources already have source owners under
`packaging/text-worker/`: reader/publisher service templates, their sockets,
the stop rule and `ardents-text.conf`. The current worker manifest covers six
files, so the tmpfiles configuration and the Endpoint system unit also need
explicit installation ownership; they cannot be presumed covered by those six
digests. The stream-qualification installer/unit is a separate qualification
producer and is not a supported text installer.

**Inference:** Fourteen static files plus four existing binaries plus the text
binary, four worker units, stop rule, tmpfiles configuration and one Endpoint
unit template total 26 manifest entries, leaving six under the present cap.
This is a candidate inventory count, not an accepted exact list or a measured
artifact size. It assumes the installation binding is generated locally and
does not add a separately downloaded authority input.

### Existing verifier falsification result

**Measurement:** The documented Windows Go overlay probe at `126761e9` exited
zero in 0.640 s. Old v3 headless without worker and a re-pinned partial worker
inventory both verified. Eight complete synthetic resources were authenticated
but all projected into Release metadata. Changed manifest with predecessor pin,
undeclared extra file and worker digest substitution each refused. The PASS
confirms these existing behaviors; the desired protected consumer was absent.
Synthetic fixture bytes and test-time repinning supply neither valid Release
Decision nor independent first-execution trust. No Linux installation, process
containment, real worker or both-Carrier outcome was measured.

**Sourced fact:** `replacement.Record` binds only the one Release-authorized
program's target, length, digest, platform, environment, Network and release
identity/version. Its authorization compares `request.Artifact` with that
program Decision. Neither it nor `LoadBundle` authenticates a complete static
worker generation. The current Release owner explicitly limits the independent
enrollment pin to first installation; it cannot authorize successors.

**Inference:** Even a new protected v3 inventory projection is insufficient to
authorize worker/unit replacement. Reusing the initial pin as successor authority
would contradict the Release owner. A complete design must bind the protected
generation to Release-authenticated successor facts, before choosing H1 or H2.

### Proposed consumer outcomes — not accepted

| Input/boundary | Proposed result |
| --- | --- |
| Existing complete general/headless v3 | Existing enrollment scope only; never protected readiness. |
| Protected consumer, v3 without all eight resources | Explicit incomplete-protected-inventory refusal before local effects. |
| Any boundary, partially present protected resource group | Refuse the partial group; no downgrade to old inventory scope. |
| Complete protected v3 | Project resources outside Release metadata, then require Release binding and installation ownership separately. Authentication alone grants no execution. |
| Recognized v1/v2 | Keep typed refusal after independent manifest authentication and descriptor digest, before companion work. |
| Unknown descriptor or extra descriptor field | Keep invalid-descriptor refusal; no implicit version widening. |
| Changed protected generation after first install | Require fresh Release-authorized generation identity and joined predecessor; the enrollment pin alone is insufficient. |

This matrix is a candidate behavior contract. The exact resource names and
Release generation commitment remain open. A signed Release target for a
bounded generation descriptor (containing program/resource digests), or an
accepted commitment in the existing target's authenticated identity, are two
routes to evaluate. A local installer manifest cannot supply either authority.
Do not add a second independently delivered pin as an update workaround.

## Options

H1 keeps the existing descriptor and one trust input, but needs exact companion
identity and refusal semantics at every caller. H2 makes the new meaning
explicit, but requires a superseding accepted version decision and fresh bundle
delivery. Both must retain independent first-execution verification, actual
running-unit binding and explicit replacement. The two-pin option is rejected
by the Product Owner; it is not an implementation fallback.

## Recommendation

Recommend H1 with a separately Release-authenticated generation descriptor,
subject to the exact contract below. Keep one delivered bundle and one initial
pin, the existing v3 descriptor grammar and existing Release custom profile.
Add a second target in the same signed metadata set, not a second independent
trust input. Confidence is moderate from source/probe evidence; the strongest
counterargument is the extra authorization composition and recovery state that
two targets require. It must be verified rather than treated as already safe.

### Release binding comparison

**Sourced fact:** `release.customIdentity` rejects unknown custom fields. Its
`build_input_commitment` also binds both builder records to build inputs; it is
not an installation-generation authority. Reinterpreting that existing field
would silently change its meaning. `Authorization.AcceptedDecision` is a cloned
public observation, while the opaque authorization itself remains private.

| Option | Change and trade-off | Assessment |
| --- | --- | --- |
| New installation commitment in executable target custom identity | New mandatory field/profile semantics, Decision projection and authorization binding; all builders must attest the right commitment. | Possible, but changes the existing Release identity grammar. Do not overload build_input_commitment or add an ignored optional field. |
| Generation descriptor as another target in the same signed Targets set | Existing target length/digest verification authenticates canonical descriptor bytes; a protected installation owner consumes both opaque target authorizations and checks their coherence. | Recommended. No new signing root, independent pin, delegated role or Release custom-profile change. Installation composition is a new accepted contract, not generic executable replacement. |

### Concrete binding — accepted by Product Owner, 2026-09-30

The Product Owner explicitly accepted the contract below. Owner/ADR promotion
is the next required step; acceptance does not establish implemented behavior
or qualify installation. Earlier proposal wording records the evaluated design.

**Measurement:** The existing Release two-target overlay probe (Windows,
Go1.26.8, `126761e9`) passed in 0.587 s. Distinct target paths in the same
synthetic signed metadata returned release-accepted then no-update, both with
fresh opaque authorizations and equal floors. Changed second artifact and an
absent second target refused without authorization. This supports the shared
floor premise only: both fixture targets contain identical synthetic bytes;
the proposed descriptor and installation composition remain unverified.

**Measurement:** A separate different-byte control then authenticated a
4096-byte synthetic program and a 54-byte synthetic descriptor target in the
same signed fixture set (Windows/Go1.26.8, package0.349 s, test0.01 s, exit0).
The second fresh proof carried its own descriptor digest and the same floors.
Substituting program bytes for that target or changing one descriptor byte
returned release-invalid without proof and preserved the committed floors.
The fixture uses in-memory floors; descriptor grammar, real signing/delivery,
installation composition and system-manager behavior remain unqualified.

1. Keep Network enrollment-v3 RELEASE bytes and initial pin grammar. Add one
   flat `protected-endpoint.json` companion plus the eight proposed resources;
   total candidate inventory becomes **27**, under 32. It is projected outside
   Release metadata with the worker/static group. It is mandatory only at the
   protected installer/start/update boundary; partial presence refuses at all
   maintained enrollment boundaries. General diagnostics cannot report protected
   readiness merely because verification succeeded.
2. The proposed canonical descriptor has exactly `schema`, `platform`,
   `release_identity`, `release_version`, `files`. Proposed schema identity is
   `ardents-protected-endpoint-artifact-v1`. `files` maps nine exact flat names
   to lowercase SHA-256: the canonical Endpoint executable and the eight
   resources listed by the probe (using the declared supported platform for
   executables). No paths, mutable roots, permissions, plans, private keys,
   timestamps or local unit InvocationID occur in this descriptor. Unknown or
   duplicate fields/names, noncanonical bytes and incomplete inventory refuse.
   Cap this descriptor at 16 KiB. The proposal is Ubuntu linux-amd64 only.
3. Do not hash SHA256SUMS, RELEASE or signed metadata inside that descriptor:
   doing so would create a packaging/signing cycle. SHA256SUMS authenticates
   the descriptor on first delivery; a target length/digest in the same signed
   Targets set authenticates it for both first install and replacement. A
   proposed exact target path is `ardents/linux-amd64/protected-endpoint`.
   The ordinary Endpoint target remains the exact executable, not descriptor
   bytes. New target/profile distribution still needs owner acceptance.
4. Evaluate both targets from identical admitted metadata bytes, local
   environment and captured reference time. Require fresh opaque authorizations
   for both; compare Targets version/digest floors, release identity/version,
   platform, architecture, environment and Network. Descriptor facts must agree
   with both Decisions; its Endpoint digest must equal the executable Decision,
   and all nine actual bytes must equal its inventory. The second evaluation
   may legitimately be `no-update` after the first committed the shared floors;
   that is not permission to skip its validation or to reuse a stored proof.
5. The protected installation owner consumes generation bytes and both opaque
   authorizations. Existing executable replacement cannot substitute a
   generation proof for its executable authorization. Signed descriptor bytes
   are data; authentication never executes them or authorizes arbitrary paths.
   No current owner permits treating this composition as implemented today.
6. A local root-installed generation record binds the authenticated descriptor
   digest, both exact target facts, actual installed resource digests and
   installation-specific plan/unit output digests. It grants no authority by
   itself. Same executable with changed resources still requires a strictly
   newer accepted generation release; equal release/version with different
   generation digest refuses. Restart re-verifies selected generation and actual
   unit/executable; it does not request a new enrollment pin or reinterpret a
   historical pin as successor authority. Current State/Entry/token/Publication/
   Instance floors and the #369 continuity limitation remain binding.
7. Explicit replacement stages a complete predecessor/successor pair, joins the
   old fixed system unit, installs selected immutable resources and observes
   the successor's real unit/account/MainPID/InvocationID before ready. Retain
   both generations until successful validation; no mixed-generation readiness,
   cross-systemd atomicity claim or unapproved rollback of Release floors.
   Exact crash recovery and unit-output grammar belong in the next bounded
   installer brief after this binding decision is accepted.

The assembly uses project-controlled builder facts under the existing Release
profile. It does not claim two independent people, an available signing
service, external audit or a new maintained release publisher. The current
Release owner has no signing API; supplying authentic signed metadata is an
explicit operator prerequisite, separate from fixture-produced evidence.

### Required causal controls before implementation acceptance

Use the same signed fixture metadata to authenticate both targets. Then vary
one cause: descriptor bytes, one resource digest, executable mismatch, metadata
set, release identity/version, stale generation with unchanged executable,
unknown field, duplicate file, missing resource or second proof absent.
Require typed refusal before installation effects for each negative. Keep
original failures. These controls qualify the authorization composition only;
actual root installation, systemd/cgroups and the two-Endpoint journey remain
the full later acceptance boundary.

## Disposition

Decided by the Product Owner on 2026-09-30; promoted to ADR-0119 and the
enrollment, Release and Endpoint runtime owners. R-167/#262 remains paused.
No runtime package, installer or support claim is created.
The resulting accepted contract must be promoted to its current owners before
one implementation issue is selected. Retain the disposable overlay probe as
reproducible component provenance; it introduces no maintained implementation.
