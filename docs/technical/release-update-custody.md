# Release, Update, and Authority Custody

Status: **current maintained technical contract.** The bounded Custody
ownership transfer is complete in this scope. This document does not claim a
supported installer, automatic unlock, platform qualification, or a complete
operator journey.

The closed successor's [migration contract](../development/privacy-qualification.md#integration-and-migration-contract)
and [installed worker boundary](application-confinement.md) are selected under
ADR-0081. They define the future complete inventory/adoption change; this page
continues to describe the currently implemented ownership and authority floors.

## Custody ownership

[ADR-0120](../adr/0120-authorize-operator-prepared-qualification-release.md)
authorizes one operator-prepared isolated qualification release. Codex owns
its technical preparation and the Product Owner accepts the completed public
bootstrap receipt before installed execution. This does not add a Release
signing interface to `internal/domain/release`, revive retired seed ceremonies, or
establish independent initial distribution. Participant enrollment retains
its independent first-pin contract; existing roots and floors are preserved.
The [qualification release operation](../operations/qualification-release.md)
owns preparation, protected key files, bounded signing, retained refusals and
the completed public bootstrap receipt. It changes no runtime verifier policy.

`internal/domain/custody` owns the bounded encrypted-record, Admission and
Service operations consumed directly by `cmd/ardents-custody`. Pure
`internal/domain/publication` owns Service v3 request/response bytes,
unsigned fixed-purpose Credential transcript and exact public verification.
Neither Custody nor its command imports live Publication/Instance for parsing
or delegates signing through the predecessor backend. Each uses independent development
roots; format compatibility grants no predecessor adoption or floor reset. Its
canonical `ardents-authority-envelope-v1` Vault and Recovery Bundle formats use
the accepted fixed Argon2id/AES-GCM profile from ADR-0021. Callers receive only
bounded public receipts, never a password, derived key, root, or generic
signing capability.

### Service Credential issuance

The Service host generates its Ed25519 Instance key and gives
Custody one public request. The Custodian separately receives and types that
request's exact lowercase SHA-256 digest, unlocks the Authority only through
its interactive terminal, and returns one public signed response plus the
deterministic encrypted successor record. Neither the request nor the response
exports Authority material.

The current `ardents-service-instance-request-v3` public request binds the
Network, Instance public key, validity interval, and request commitment. The
signed Service Instance Credential v3 delegates that Instance with its Target,
Authority, Network, generation, validity interval, and capabilities. Neither
format contains the retired Introduction recipient field. [ADR-0102](https://github.com/dianabuilds/ardents-network/blob/c58268d72f1eaeae173144e76d32d50986de2a77/docs/archive/adr/0102-stop-legacy-introduction-key-emission.md)
retired that key emission; [ADR-0117](../adr/0117-reset-closed-service-formats-to-v3.md)
selects the current v3 format identities. Service Credential is the glossary's
short form for the same public credential.

The private Introduction capsule instead uses a separate, fresh volatile
X25519 recipient owned by the live Instance binding. Its lifecycle belongs to
[Endpoint and Service runtime](endpoint-service-runtime.md); it is not a
Custody issuance input or a field in the signed Credential.

### Closed admission allocation

`create-admission-authority` creates a distinct encrypted Ed25519 authority
record for one closed Network. `issue-admission-permission` accepts only the
exact holder-signed `ARDPAR01` request after the operator re-enters its
independently transferred SHA-256 commitment. It returns a public digest
receipt and writes the signed 228-byte permission only to the specified
owner-only, same-directory no-replace output. Private delivery checks that the
original output is owned by the effective user and that group/other access is
absent before writing the staged Permission and when admitting an exact retry.
The Linux mechanism accepts ext4 and tmpfs, whose POSIX ACL mask follows the
mode bits; tmpfs is volatile and supplies no persistence claim. Since Linux
uses the same filesystem magic for ext2, ext3 and ext4, ext4 is identified by
the held object’s mount ID and the kernel mount table, never by magic or a
path lookup alone. If the mount ID or trusted procfs table is unavailable,
private delivery refuses. Other Linux filesystems, macOS and Windows return a typed unavailable
result before the command opens the Vault or creates a staging file. The API
rechecks the original file at delivery; command preflight is not a reservation.
macOS ACLs are evaluated separately from POSIX mode bits, and Windows may
inherit a DACL from its parent, so `0600` alone does not establish the required
access boundary ([Apple](https://developer.apple.com/library/archive/documentation/Security/Conceptual/AuthenticationAndAuthorizationGuide/Permissions/Permissions.html),
[Microsoft](https://learn.microsoft.com/en-us/windows/win32/fileio/file-security-and-access-rights)).
The boundary addresses other unprivileged local users; it does not protect
against the same UID, privileged processes, compromised kernels, or prove
crash/power-loss durability. Public Service Credential response delivery keeps
its separate access contract.
The [Admission allocation owner](../../internal/domain/admission/allocation/doc.go)
validates the current-hour request and journal transition. Custody retains the
key, signing and serialized encrypted persistence. The Vault record advances its
encrypted allocation successor, exact readback and monotonic floor before acknowledgement; neither command accepts
a private key, permission body, or password in argv, environment, or public
output. This authority cannot sign State, Name, Service, or Release inputs.

One supported Service Credential has a maximum 24-hour lifetime and a terminal
horizon of 48 hours from issuance. The successor ledger also requires a new
validity interval not to overlap its predecessor for the same Target. An exact
retry of the same request is idempotent; it is not a reusable authorization for
a future request. New issuance rechecks current validity after unlock and after
password-derived encryption, immediately before the first successor write.
Each new Service successor authenticates its exact canonical public request
in the encrypted `service_journal` field. Floors and a deterministic filename
alone cannot prove exact retry: copied authentic ciphertext for another request
refuses even if generation and terminal bound match. Generic predecessor evidence
without this journal gains no signing adoption. The outer envelope/KDF and public
v3 grammar remain unchanged.
An elapsed Service request or admission hour cannot begin a new durable
issuance. Once a successor is persisted, its monotonic floor completion
remains recoverable; this does not renew its terminal validity. Admission exact
retry still requires its original current hour and never receives the Service
post-expiry exception. A committed
exact Service retry may return the same response after expiry. Custody has no automatic renewal route, password file,
argument, environment, or configuration bypass. The service's published-root
restart limitation remains an Endpoint implementation limit, rather than a
claim that every future product lifecycle must work this way.

The former Name Authority signing route is retired by ADR-0105 together with
the whole Namespace subsystem. Custody no longer declares
`sign-namespace-transition`, `prepare-namespace-submission`, or
`activate-recovered-authority` operations, their transition/submission
operation fields, or receipt proof/submission fields, and it imports no
Namespace package. Existing Name Authority vault records remain inspectable,
verifiable, exportable, and purgeable because the `AuthorityName` kind stays
recognized in the generic record grammar.

## Recovery lifecycle

```text
active encrypted Vault record
  -> explicit Bundle export and isolated test restore
  -> restore into separate encrypted authority-locked quarantine record
```

The former activation step required an opaque witness originating only from
an already verified current Namespace materialization, strictly higher than
the recovered state. ADR-0105 retired both the witness producer and the
`activate-recovered-authority` operation, so a restored quarantine record is
terminal: it remains export-only and can never sign. No activation route
creates a runtime Instance Key or Local Grant.

Follow `Vault.Execute` in `vault.go` for the original operation and native
lease, `authority_record.go` for creation, verification and active/quarantine
lookup, and `vault_record.go` for record names, the shared storage budget and
durable ciphertext I/O. A new record is published without replacement, the
directory is synced by the platform's existing mechanism, and the exact
canonical encrypted bytes are read back before the caller can advance its
floor or return a receipt. Admission still owns journal replacement and
Recovery Bundle export still owns its test restore.

The Vault keeps one original operation and native lease through terminal
input, crypto and durable work. Close synchronously denies new effects, cancels
and physically joins that user before secret erasure/lease return completes;
late success retains the original cancellation and cleanup failure. Erasure
covers owned byte buffers; these checks do not establish whole-process memory
scrubbing of Go/crypto runtime allocations. Its state
mutex does not cover foreign I/O/join. Only a fresh independently marked root
can create Admission or Service signing authority. Existing predecessor Service/Name records
and quarantine remain inspectable/verifiable/exportable/purgeable evidence.
Birth admits only the empty native Vault layout; unrelated root files,
directories or symlinked record directories cannot gain signing provenance.
Purge removes the selected active Admission ledger too, while retaining floors.
A missing, malformed or skipped journal/floor refuses; one authenticated exact
pending successor may finish its floor under a new original operation.

## Custody disposition

The maintained Custody Module owns the bounded encrypted
Vault/Bundle/signing contract. It deliberately does not select a supported
Windows or Ubuntu storage profile, crash/power-loss or permissions
qualification, a qualified Application isolation profile, or a complete
Custody operator lifecycle. Those are future product/platform work and need a
new scope decision and the applicable ADR analysis; they are not deferred
implementation defects in the current Module.

## Explicit limits

- The retired `ardents name control` command refuses before reading its former
  operation input. ADR-0105 also deleted the custody-side Namespace
  preparation and signing machinery, so no signed control wire exists
  anywhere in the working tree.
- Endpoint replacement has no Vault/root input. The current replacement-owner
  test proves byte-for-byte preservation of an encrypted Authority Vault and a
  persisted Release-floor root across successful replacement, stop refusal,
  and explicit rollback; this is non-mutation evidence only, not crash,
  power-loss, or platform qualification.
- The retained `custody_notice` key is not live custody state. It is the exact
  H3 evidence text in the frozen `ardents-release-decision-v1`,
  `ardents-update-result-v1`, and update-manifest V0 formats. Go projections
  call it `EvidenceNotice`; the old byte name remains only in the unobserved
  V0 provenance fixtures. The V2 fixture cutover omits the field; an independent C4
  verifier retains the exact V0 vectors, and no V2 compatibility writer or
  Vault-status interpretation is permitted.
- Windows/Ubuntu crash, permissions, and power-loss qualification are future
  product/platform work, not current support claims.
- R-044 threshold recovery historically replaced the effective Name Authority
  in Namespace. Since ADR-0105 no Namespace exists and no Name replacement
  route remains; Name Authority records persist only as encrypted
  export-only evidence, and the former active-Vault demotion concern is
  moot. Broker Grant revocation remains a separate local-admission
  transition.
- Supported lifecycle/installer work is future product scope.

## Evidence

- [ADR-0021](../adr/0021-use-password-derived-authority-custody.md)
- `internal/domain/custody/record_refusal_test.go` and `bundle_publication_test.go` (retained envelope/refusal and Bundle publication oracles)
- `internal/domain/custody` behavior, independent envelope and native failure tests
- `tests/e2e/service/admission_custody_process_test.go` (canonical built terminal consumer)
- `make custody-check` and the ordinary checked deterministic/process/race inventories

## Release and Update ownership

[ADR-0123](../adr/0123-bind-installed-infrastructure-generations.md) accepts
distinct infrastructure program and invocation-generation targets. Infrastructure
admission must consume two fresh coherent opaque authorizations from one frozen
input; Release retains signature, policy, expiry and floor authority. This
contract has distinct Source, Forwarding, JOIN and Descriptor pair owners in
`release/infrastructure`. Native installation and each role lifecycle retain
separate owners and acceptance scopes; existing Endpoint targets remain unchanged.
The original Source, Forwarding, JOIN and Descriptor ceremonies publish five
fresh private files through the protected keyset owner. The command renders the
ordered public inventory after native publication and close. Signing separately
admits the original key directory, purpose and independently verified targets.
Descriptor initial admission requires its own independently pinned running Node
program and entirely unborn Release floors. Its candidate reader requires complete
retained floors and never reconstructs first-pin provenance. Fixed Descriptor
initial signing consumes original protected purpose/key/resources and independently
checks both exact targets before publishing metadata; public plan or diagnostic
self-verification supplies no installed or independent-custody qualification. Initial admission
requires empty floors; complete retained history admits only the candidate path.
`StaticMetadataFiles` projects only the three referenced URLs from fixed public
metadata files, retaining the normal narrower metadata bounds and all signature
and root-rotation checks. `CheckAuthorizations` reobserves original Release
ownership, caller cancellation and unchanged floors at the pair's handoff;
physical Close and uncertainty remain Release-owned refusals.

[ADR-0119](../adr/0119-bind-protected-endpoint-generation-to-release.md)
selects a protected Endpoint generation target in the same signed metadata set
as the executable. The installation owner consumes two fresh coherent opaque
authorizations, verifies their shared Targets floors and release/local facts,
and binds all resource bytes through the authenticated generation descriptor.
Endpoint compares the two fresh Decisions for source revision, build input
commitment, attestation policy, qualification, build state, protocol phase,
build and protocol classifications, both build safety deadlines and the
protocol transition deadline. Installed binding v1 retains its original
projection. Platform pre-start repeats the full comparison on fresh proofs.
The initial pin never authorizes successors; changed resources require a newer
generation even when executable bytes are unchanged.

`internal/domain/release` is the sole owner of release trust roots and non-decreasing
release floors. It verifies the selected TUF-compatible metadata profile,
consecutive root rotation, exact target identity, protocol/build state, and
the captured reference time. Its output is a bounded public Decision and
opaque authorization. It does not download artifacts, run a repository,
maintain an ambient cache, sign metadata, select a mirror, or expose its floor
storage.

On Windows the history owner accepts NTFS volumes with persistent ACLs. A new
root is born with an owner-only protected DACL; an existing root is checked
without changing its security descriptor. The original parent and root handles
remain held without delete sharing through the verifier lifetime. Before
reading or recovering a retained object, the owner checks its original handle,
owner, DACL, object kind and link/reparse properties; the lease is checked again
when acquired. An unavailable or broader protection refuses before recovery and
never recreates retained trust. Windows access checks may bypass directory
traversal, so the file and child-directory checks are necessary in addition to
the root check ([Microsoft](https://learn.microsoft.com/en-us/windows/win32/secauthz/privilege-constants)).
This boundary does not protect against the same account, privileged processes,
pre-existing privileged handles, kernel compromise or establish crash/power-loss
qualification.

The verifier admits at most 64 entries in its retained generation directory,
including staging residue. Under the original exclusive lease it refuses a
new generation before staging would exceed that same read bound.
`ErrHistoryCapacity` is the retained cause of that refusal. The current
pointer, trusted roots and floors remain readable. An exact no-update can use
the existing authenticated history.

The isolated new-domain composition uses `internal/domain/release` with
its own history and the genuine `ardents-next release verify-initial` consumer.
It consumes new Enrollment's authenticated facts and frozen bytes, not the
predecessor verifier or installed roots. The new owner serializes evaluations,
checks original cancellation at publication/handoff and joins admitted work
before returning its native lease. New Installation additionally consumes two
fresh exact-target authorizations from this owner. Its successor byte consumer
uses `OpenRetained`, which requires complete persisted floors for all four
top-level roles and cannot create cold trust. These pair checks grant no
installed predecessor continuity or runtime readiness. New Installation separately
owns initial stopped provisioning, successor replacement and bounded fresh-proof
recovery through genuine command consumers. Bounded accepting non-root startup
has an actual-manager Root archival/ACK and Source/permission receipt. Separate
actual process cuts exercise fresh-proof successor recovery before reload,
after reload before phase7 creation, and after durable phase7 before Start.
Original fixed resources, immutable generations, journal members and Release
floors are preserved; the recovered Endpoint reaches permission-pending. These
external trials retain the deliberately killed Root's failure and do not qualify
the installed Service. The
complete interruption matrix and installed Service acceptance remain unproven; these consumers do
not borrow the predecessor runtime or Release history.
Portable `internal/domain/release/endpoint` binds the exact program and protected
generation using fresh coherent Release proofs and original Enrollment
provenance. It also owns verification of the running program against an
independently pinned bundle and joined Release history. `internal/domain/endpoint/launch` consumes those proofs to stage the
generation and independently checks installed binding and startup. Native
observations supply physical facts while Release retains trust floors.
Portable metadata/target/safety rules remain
distinct from native history mechanisms. The new-domain
[design](../../internal/domain/release/README.md) preserves the accepted
metadata and durable identities; source presence and component checks do not
establish complete Release acceptance, installed replacement or qualification.
Existing predecessor Release roots remain independent retained evidence.
The current verifier grants no automatic installed-history adoption.

ADR-0050 through ADR-0052 historically assigned the first closed-alpha release
seeds and fixed RC1/RC2 assembly to a separate Product Owner boundary. ADR-0059
retired assembly after the completed ceremonies, and ADR-0067 retires the
remaining seed initializer/inspector and its private envelope as maintained
interfaces. Their exact records and receipts are provenance only and no future
release accepts them automatically. The Release verifier has no signing or
Custody interface.

A fresh executable cannot use its own Release Decision code to authenticate
itself before first execution. `RootBytes` and the first executable therefore
have caller-owned provenance established by the selected Distribution Profile;
the Release Module fails closed on their content but does not invent that
initial provenance. Once one trusted executable has committed the root and
floors, this external enrollment input authorizes no successor: later
executable authorization remains exclusively a Release Decision followed by
the Update boundary.

For the historical first closed-alpha cohort, the selected Distribution Profile was one
**Alpha Enrollment Pin** sent through an already authenticated Product Owner
contact independently of GitHub and the download path. It binds one exact
cohort, release, platform, and manifest SHA-256. The participant compares that
digest before parsing the manifest and uses its exact inventory to verify the
executable, descriptor, initial root, complete metadata, and declared static
companions before execution. On first run the Endpoint must submit those same
bytes and its own executable to Release Decision and durably establish the
accepted root/floors before reporting network readiness. The pin is neither a
Release Module input nor successor authority; it authorizes only this first
bundle and makes no public or independent release-control claim.

The per-user foreground replacement command surface is retired (#617 S3.5).
`ardents endpoint replace/rollback/replacement-self-test/replacement-recovery`
return typed refusals before input or root effects. Existing per-user roots,
retained programs, journals and floors remain on disk as evidence. The
maintained forward-only lifecycle uses `ardents-next platform stage-endpoint
<request> <previous-launch>` with freshly authenticated exact artifacts and
retained continuity evidence.

The former generic `internal/update` transaction had no production caller and
is retired. Its distinct schema-copy and adapter choreography are not part of
the selected Endpoint replacement contract. Current replacement behavior and
crash boundaries are verified at the owning Module; platform packaging,
automatic update, Windows replacement, system registration, and public release
qualification remain unselected.

## Cross-module invariants

- Release floors and trusted roots never decrease.
- Endpoint replacement cannot authorize itself or interpret Authority/Custody
  state.
- Custody never supplies secrets to Release or Endpoint replacement.
- A failed or interrupted replacement returns an explicit bounded outcome; it
  must not silently accept a candidate or erase the retained predecessor.
- The retired V0 evidence field remains only in historical provenance; no
  maintained writer recreates the retired generic transaction.

## Verification

Focused Release, Custody, and Endpoint replacement behavior tests exercise the
frozen public Release vector, metadata rejection, root and floor progression,
interruption and recovery, residue refusal, Vault export and restore, and
reconciliation. The retired Endpoint entrypoints retain pre-effect refusal
tests. Run the normal repository gate
during development and the full check before integration. Platform crash,
power-loss, permissions, and supported lifecycle qualification are not
satisfied by these tests.

## Release proof handoff

The pure evaluation pipeline produces authenticated decision facts. Only the
original `Verifier.Evaluate` handoff issues its private Authorization after
checking original caller/owner state and binds it to that live verifier and
caller. Evaluation facts alone cannot be passed as a reusable installation or
runtime permission. Delivery retains its independent effect/cleanup admission.
