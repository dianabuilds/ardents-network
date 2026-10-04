# Endpoint and Service runtime

## Selected protected installation handoff

The following bounded command contract selects ADR-0119's installation
boundary. The initial stopped provisioning, read-only integrity and installed
startup consumers are implemented but not yet qualified on an admitted installed
host. Successor update/recovery now have production callers and component
controls; first-provision recovery now has a bounded production caller, but
the full interruption matrix remains unfinished, and there is no
supported full installation receipt yet. Commands remain thin adapters under
`ardents endpoint`; the owning implementation issue must register any new
package before adding one.

`internal/endpoint/runtimeplan` owns the existing bounded headless-v2 and Source-v1 local
declarations and their role/path/permission and public-identity validation.
The current headless and Source refresh commands read bounded bytes and call these same decoders;
installation consumes that grammar rather than defining another copy. Decoding
opens no State or mutable root, contacts no manager and grants no Release,
holder or runtime authority. Source credential loading, runtime composition and installation transitions
remain separate owners; this parser extraction does not implement provisioning.

`internal/endpoint/installation` composes the two fresh Release evaluations from
one frozen set of enrolled metadata and the same local/reference facts. It
checks complete resource bytes and coherent authenticated target identities and
Targets floors before returning their opaque proofs. The ordinary enrolled
command consumes only the executable proof for its existing replacement owner;
its readiness remains general enrollment, not protected installation. Failure
of the second evaluation retains any already committed Release floors and
cannot return a partial accepting pair. Provisioning, immutable selection and
actual manager identity binding are separate from this authentication.

`start-installed` rechecks root-owned selection, all generation and fixed bytes,
actual mutable root identities, this process's UID/GID/executable/arguments and
unified cgroup, and the system manager's typed unit/Service properties. MainPID,
InvocationID, exact ExecStartEx and protection properties must match before it
returns the bound v2 plan to the ordinary participant composition. It opens no
Release floor store and never recreates a fresh proof from stored target facts.
The authenticated Endpoint unit requires both activation sockets before start;
the socket units are PartOf the Endpoint so its stop also retires their listeners.
Before returning the plan, startup observes both live socket units through typed
manager properties and checks their fixed fragments, PartOf, listening state,
Endpoint ownership, mode0600 and removal-on-stop contract.
Effective stop/activation ordering still needs admitted installed qualification.
During successor start a root-private `start-guard.json` retains the exact intent
independently of cursor archival. Before manager start, root binds a private
ephemeral Unix completion socket under the protected installation root. The
Root-private socket birth record binds its device/inode and group to the exact
guarded intent before manager start. Recovery validates this record before
stop; cleanup removes only that recorded socket. An unrecorded or substituted
socket refuses even with matching Root ownership and mode, retaining the guard.
Endpoint verifies its root peer and waits before participant composition;
root verifies the connecting MainPID/UID and the exact InvocationID, generation
and binding digests. Root sends completion only after the start observation and
archive directory syncs succeed. Cursor absence alone grants no transition
admission. EOF, unavailable owner, substituted identity or the existing startup
deadline refuses. The guard retains explicit recovery provenance if root dies;
recovery still requires fresh floor-compatible proofs and actual stop/join.
After acknowledgement, guard/socket cleanup failure is a post-acceptance error:
the public command retains `installed-started-recovery-required` with nonzero
exit status instead of describing the invocation as never admitted. That receipt
does not establish Service readiness. Socket visibility under confinement and
the complete interrupted-cleanup matrix remain admitted qualification work.
The ExecStartEx flag name and typed command representation follow the
[systemd v255 implementation](https://raw.githubusercontent.com/systemd/systemd/v255/src/core/dbus-execute.c)
and its [flag mapping](https://raw.githubusercontent.com/systemd/systemd/v255/src/shared/exec-util.c),
checked 2026-10-01; component observations do not substitute for actual manager properties.

The initial `endpoint provision` caller now composes platform/root admission,
the independent manifest pin and both fresh Release proofs, explicit account
creation, private mutable directories, immutable candidate staging, fixed
resource copies, selection and manager reload. It returns only
`installed-stopped` after checking the actual loaded Endpoint/socket units are
inactive with their exact fragments and no drop-ins or worker instances. It
never starts a unit or issues permissions. Initial installation requires absent
managed writable directories and sockets; existing state is not adopted or
cleared. Existing parent directories remain root-controlled, and Source
credentials and public permission responses still require their explicit owners.
Root-only preparation and generation phase records retain original errors.
After preparation, provisioning records an initial transition intent before
generation writes. Installed startup refuses either a pending intent or a
pending failure before manager/runtime admission. Initial recovery requires
fresh authority from the established Release floor store, the owned generation
directory and complete fixed-file creation records (device/inode, intended digest,
mode and group). It can repair authenticated prefixes on those owned objects,
reload and return `installed-recovered-stopped`; explicit startup remains separate.
An interruption before complete fixed-file birth records requires repair rather
than adoption. Each fixed file and selection records its exclusively created inode
durably before writing contents. The full actual interruption matrix remains
unqualified.
Component filesystem and manager-property controls do not qualify a positive
Ubuntu/systemd/cgroup installation or the two-Endpoint Carrier journey.

The read-only `installation-check` consumer now checks a canonical root-owned
selection and local binding, exact generation/resource/plan bytes, the real
dedicated account, mutable root device/inode/access and actual fixed worker
resources. It requires a matching Ubuntu22.04/systemd249 or
Ubuntu24.04/systemd255 amd64 profile with cgroup v2. Mixed OS/manager pairs
and newer unadmitted managers refuse. On manager249, absent `ExitType` and
`RestartMode` reflect unavailable selectable policies; parent lifetime still
requires `RemainAfterExit=false`, `Restart=no` and the exact main process.
On manager255 both observed policies remain mandatory. Every installed process
check observes the actual manager version independently. This compatibility
admission does not qualify either installed host profile.
Its result is only `local-integrity-verified`: it opens no Release floor store,
evaluates no fresh Release proof and grants neither runtime readiness nor an
active MainPID/InvocationID receipt. The actual installed
positive journey remains unqualified; byte-backed fixtures prove only component
validation, not filesystem ownership or installed containment.

The local request schema is `ardents-endpoint-installation-request-v1`, compact
UTF-8 JSON plus LF, with fields in this order: `schema`, `bundle_root`, optional
`manifest_sha256`, `installation_root`, `release_floor_root`, `reference_time`,
`headless`, `source`. Nested declaration encoding uses the current shared
grammar owners; duplicate/unknown fields and alternate encodings refuse.
Reference time is canonical UTC RFC3339Nano. Initial provisioning requires the
independent pin; its absence never authorizes a first installation. Inline
Source must match the headless Network, signer map, threshold, role root and
clock observation file, with nonzero refresh. The input headless Source-plan
path is empty; its rendered output selects only that generation's `source.json`.
The two declared Direct Source operator families must be present and distinct;
the installation request refuses a duplicate before creating an Endpoint account
or selecting a generation. Distinct names pass only the syntactic check: known
common operational control makes them one Operator Family for installed
qualification. A one-operator artifact receipt can authorize exact-byte
inspection and isolated negative checks, but cannot supply missing Source or
Route family eligibility. Hidden common control among apparently distinct
families remains an honest limitation even when the installation proceeds.
Every declared path is absolute/canonical and remains outside the bundle,
immutable installation and Release floor roots. No private material is copied.

The local selection binds descriptor and binding digests. The binding records
both public target facts (never opaque authorizations), exact fourteen staged
file digests, fixed account/unit names and numeric UID/GID, and explicit mutable
root paths/device/inodes. `binding.json` is excluded from its own digest map;
the selection separately commits its bytes. These local facts cannot produce
`release.Authorization`, lower floors or authorize a transition.

| Command | Input and owner effect |
| --- | --- |
| `endpoint provision <request-file>` | Explicit root operation after independent first-artifact verification. Authenticate executable/generation through Release, validate local declarations, stage and select one complete stopped installation. No implicit start. |
| `endpoint installation-check <installation-root>` | Read-only bounded local integrity/selected-generation observation; does not create authority, repair files or claim readiness. |
| `endpoint start-installed <installation-root>` | Fixed system-unit ExecStart consumer under ardents-endpoint account. Verify binding, program bytes and observed unit/MainPID/InvocationID before participant effects, then consume the bound v2 plan. No generic privileged launcher. |
| `endpoint upgrade-installed <request-file>` | Explicit root operation with fresh coherent Release proofs and strictly newer generation; stop/join predecessor, stage fixed resources and select successor. Starts only after all validation and selection succeed. |
| `endpoint recover-installed <installation-root>` | Explicit root recovery from exact owned journal and generations; revalidate current Release authority and never lower floors. Refuse repair-required when no authorized complete generation is available. |

The provision/upgrade request is canonical bounded JSON (maximum 64 KiB), with
only schema, bundle root, independent first-install pin where applicable,
installation root, Release floor root, reference time, and the existing
headless/Source operator declarations. Its schema belongs to the installation
owner, not the wire protocol. Reject duplicate/unknown keys, symlinks,
noncanonical or nonabsolute paths, overlapping immutable/mutable roots, and
Reader requests containing Publisher/Instance/administration inputs. Root may
declare local identities and paths; it cannot manufacture Network, Custody,
Instance or holder permission authority. Exact field projection reuses current
v2 owners rather than a second grammar for their content.

Immutable generations are root-owned direct directories, identified by the
authenticated generation descriptor digest. No service-account write or
symlinked ancestor is allowed. A bounded local binding contains both verified
target facts, program/resource and rendered v2 plan/Source/unit digests, fixed
account/unit names and explicit mutable root identities. Files readable by
the service account are mode0640 with root ownership and its group; program
and installed worker retain their required execution/read-only modes.
The selection is root-owned, readable by the Endpoint group and never writable
by it; the mutation journal is root-only. No stored serialized
Release authorization may be replayed as a fresh proof.

Use the fixed `ardents-endpoint.service` and current worker unit/socket names.
The Endpoint unit template is authenticated before rendering its one selected
generation's absolute program/installation-root arguments. This consumer
rechecks the rendered output as well as actual manager observations, not merely
the template digest. Mutable State/Entry/token/Publication/Instance roots are
separate, never copied into generation bytes or cleared on replacement.
Explicit root provisioning owns service-account creation and directory access;
it accepts no undeclared existing account or conflicting unit silently.

Stage all bytes and fsync files/directories before changing fixed resources.
Worker's existing direct-path inventory prohibits replacing its root by a
symlink to a generation. While the Endpoint and worker scopes are stopped and
joined, install the exact fixed-path copies, observe their digests, publish
selection, reload the system manager, then permit start. Every step records
its owned journal phase and original error. No filesystem rename makes the
manager transition atomic. A failed transition leaves the unit inactive and
the exact prior/successor bytes available for explicit recovery; foreign
paths, mixed resources or ambiguous journal state refuse before effects.

`upgrade-installed` refuses an initial manifest pin and loads a self-consistent
candidate through the enrollment inventory owner without claiming enrollment
authenticity. Both targets must receive fresh Release proofs from the already
established complete floor store; a new local trust root, missing floors,
conflicting same-version floor digest, changed Release environment or Network,
equal generation release or changed durable roots refuses. The root-only
successor intent binds previous selection, candidate selection and public
candidate binding facts. Generation staging records its newly created physical
directory identity before writing artifacts. Actual predecessor MainPID and
InvocationID are rechecked after pinning its original cgroup; both activation
sockets are stopped with the fixed service. Stop success alone is insufficient:
all original pins, stopped fixed units, loaded worker inventory and remaining
kernel scopes are checked before direct fixed-resource writes.

Each successor replacement durably records its original direct inode, mode,
group and complete old/new digests before truncation. A visible existing record
does not prove that an earlier sync succeeded: replacement re-synchronizes that
private record and its directory before modifying the resource on a later
attempt. A refusal to synchronize that record or its directory leaves the
resource and retained record unchanged. A resource-directory sync refusal after
writing the candidate still returns an error and retains the replacement record;
visible candidate bytes alone do not establish successful durable completion.
An explicit retry re-establishes journal durability and completes the replacement
on the same recorded inode.
Initial fixed-file recovery likewise re-synchronizes its private birth record
and record directory before repairing the recorded inode. A refusal at that
boundary retains the current resource prefix and the original record.
`recover-installed`
requires an exact root-only successor intent, fresh floor-compatible proofs at
the recovery time, the owned generation directory and exact replacement
records. It can complete authorized prefixes inside that generation and torn
fixed copies on their recorded inodes; another inode or foreign bytes refuses.
Recorded target observations do not recreate a proof. Recovery reuses the
original bound plans and public observations only after checking current proofs
against their exact target bytes and identities. The first transition failure
is retained and archived with the completed intent. Missing initial preparation
or generation ownership evidence returns `repair-required`; initial recovery's
bounded stopped path is described above. The complete first-provision and
successor failure-injection matrix remains acceptance work. The receipts `installed-started` and
`installed-recovered-started` observe the fixed unit, not Service continuity or
the complete two-Endpoint journey. Component tests exercise these controls;
actual admitted manager/namespace/seccomp/empty-scope receipts remain required.

All root installation subprocesses use fixed absolute programs and only
`PATH=/usr/bin:/bin`, `LANG=C`, `LC_ALL=C`; caller-supplied system bus, unit
lookup and loader environment overrides are not inherited.

Retaining predecessor bytes is not permission to activate an older Release.
Recovery must finish a valid selected candidate or obtain a fresh floor-compatible
explicit rollback authorization; otherwise report repair-required and stay
inactive. Restart observes a new InvocationID and the same immutable selection,
retaining durable floors. Successful process restart does not imply publication
continuity: the current credential/recipient refusal remains an honest outcome.

Implementation acceptance requires causal pre-effect refusals for missing or
mismatched proofs, partial resource groups, digest/ancestor substitution,
service-account edits, conflicting accounts/units, wrong executable/MainPID,
failed writes/reload/start/join and each interrupted journal phase. The final
installed oracle uses two separate admitted Ubuntu system managers, distinct
Endpoint principals/roots, public provisioning and permission commands, both
Carriers, publish/link/read/Descriptor refresh/withdraw/refusal and restart.
Actual containment observations and post-close empty scopes are recorded
separately. Fixture-produced JSON/units, ordinary Docker and one dual-role
Endpoint remain component/diagnostic evidence.

[ADR-0119](../adr/0119-bind-protected-endpoint-generation-to-release.md)
selects the installation binding for the protected successor. Before participant
effects, its consumer must verify the accepted generation, immutable plan/unit
outputs and actual system-manager unit/account/executable/MainPID/InvocationID.
Restart retains durable floors and does not repair Service credential continuity
by resetting roots. Existing runtime checks alone do not implement this binding
or qualify the complete installed two-Endpoint scenario.

Status: **current maintained technical contract.** This document describes the
local Endpoint, generic Broker, Service publication, and Service Connection
Modules that exist in the repository. It does not select a supported desktop
profile, a qualified Application Isolation profile, a public Service protocol,
or a complete Route/Node qualification.

ADR-0117 selects and implements a coordinated fresh-root Service v3 format
reset. Historical v1/v2 Service bytes authorize no compatibility reader or
mixed-version deployment. The Connection record profile is
`ardents-service-connection-profile-v3`, independent of `ardents-route-v3`.
Changing the Target derivation changes the Target and Link even for a reused
Authority key. Old bytes are refused without automatic mutation.

Although its directory is under `internal/application`, the Broker is
Network-owned because the maintained headless Endpoint uses it for local-grant
admission and session lifecycle. The sibling `administration` and `connection`
packages have separate owners and preserve their respective v1 and v2 local
protocol identities without owning Endpoint behavior or Browser presentation.

The selected closed successor's [workload](../product/protected-service-workload.md),
[confinement](application-confinement.md) and [protocol](protected-route-protocol.md)
own its Endpoint composition under ADR-0081. Installed-host qualification
remains separate; generic callers do not acquire a qualified-launch receipt.

The successor text Publisher context independently owns its Introduction and
Responder prefixes and its Introduction registration. It consumes the existing accepted Instance binding to commit
an Instance-signed private Descriptor through the actual resolution duty. The
Instance owns a fresh volatile recipient and its monotonic revision floor;
registration loss erases the recipient, and context loss joins operations before
withdrawing Publication and Instance. A committed Publication retains cleanup
ownership even when cancellation prevents Lease handover. Failed withdrawal
retains its binding and original error until cleanup completes. This tested
composition consumes real recipient-confidential capsule delivery before opening
its separate Responder-domain forwarding prefix. A private Introduction
lifecycle alone reserves and publishes its opening, exposes an exact read-only
handle to registration/refresh/refill callers, invalidates that handle before
retirement, and joins opening and Route cleanup without closing the borrowed
Source. A separate private Responder lifecycle owns its exact live handle,
opening, idle retirement and context-stop cleanup without closing the sibling
Introduction or borrowed Source. Both Publisher prefixes obtain
genuine tokens through Source and share admission rules while retaining
separate selections and transports. Known Node/key/family overlaps across live
domains or subroles are excluded before selection and issuance; losing a member
cannot resample a retained set. A final handover rechecks the exact live
Responder owner. Worker loss preserves a surviving context's allocation, while
context loss joins both prefixes. Its refresh scheduler retains the old
published registration while the replacement Descriptor awaits acknowledgement;
the new registration cannot accept a capsule until that acknowledgement is
verified against its still-live context, Instance, channel and profile. One
private Publication-pair lifecycle owns current/predecessor registration
visibility and the withdrawal drain barrier. It commits the acknowledged local
pair in one transition; cancellation after remote ACK leaves it non-accepting
while the existing durable Publication owner retains cleanup. Withdrawal makes
the barrier visible before drain, so a late ACK cannot revive the pair. Network
publication does not hold the shared Publisher mutex: a checked context
reservation retains exclusive Instance ownership against legacy publication
operations. Only the first successful switch bounds predecessor overlap to
60 seconds or its earlier signed expiry; exact retries retain that cutoff.
An established Connection does not extend that overlap: each recovery resolves
the Target again and may advance only to a monotonic Descriptor revision under
the same immutable Publication and profile before sealing its fresh capsule.
Scheduler timing is tested with accelerated events, not a wall-clock lifetime
qualification. These module paths do not establish complete command exposure
or installed-worker qualification.

The closed text-Service composition retains a bounded job-owned exchange through
JOIN and the authenticated Service transport's final cleanup. It prepares the
independent Rendezvous class-2 and Introduction class-1 stocks before concurrent
Source JOIN and capsule submission; each exact HELLO still requires its own
durable token transfer. Publisher JOIN consumes its independently accepted capsule
and current Responder prefix. The worker receives no raw JOIN stream: the existing
Service TLS and native Instance authentication precede Application I/O.
Each initial or recovery Connection JOIN acquires an operation-local wrapper for
the exact current Source handle before stock preparation. Publisher JOIN likewise
acquires the exact current Responder handle and its exact retained Source issuer.
Token presentation and the final transport transfer recheck that same
acquisition under the context lock. Failure joins any returned Route stream
before releasing only that
acquisition; success transfers the acquisition to the joined transport, whose
close joins Route cleanup and then releases it. A replacement Source therefore
cannot be used by a late old JOIN, and a replacement Responder cannot be exposed
through an older opening or JOIN acquisition.

Endpoint context composition serializes issuance admission with its retained
Source-operation reservation, but one private issuance operation owns each
admitted attempt's network context, cancellation, issuer presentation and
terminal completion. Context or Endpoint revocation cancels and joins that
operation before permission material is released. A delayed completion cannot
publish usable stock after revocation; ordinary retry retention and the
recovery-only canceled-batch discard remain distinct and consume the same
existing reservation and bootstrap allowance.

One private Source lifecycle owns the stock-to-opening reservation, the exact
published Route prefix and its cancellation through joined retirement. It alone
mutates those states. Existing Descriptor, JOIN, issuance and qualification
consumers receive a read-only handle for the exact published opening; they can
perform their existing Route operations and test that identity under the context
lock, but cannot close or replace the prefix. Retirement invalidates the handle
before closing its Route owner. A late completion from an obsolete opening
cleans and joins only its own result, cannot publish over a replacement, and
cannot renew stock or the two-batch bootstrap allowance. The context lock and
Source-operation serialization remain the admission boundary; Publisher
Introduction and Responder lifecycles keep separate exact handles, openings and
stop/join cleanup. Context shutdown and their callers no longer mutate either
prefix's fields.

Descriptor lookup and publication share one private resolution-flight owner
under the Context lock. Admission retains one operation-local acquisition of
the exact current Source handle; a second flight cannot replace it. The owner
joins caller cancellation, releases that acquisition once and closes its exact
completion barrier. Recipient selection, stock preparation, token presentation
and a returned proof remain bound to the acquired Source. Cancellation, Source
retirement or replacement makes it non-current, so a late response cannot
commit a Descriptor floor or publication acknowledgement or attach to later
Source work. Context shutdown cancels the flight before its ordered join;
Permission, Publisher/Instance and Descriptor authority remain root checks.

A clean JOIN peer CLOSE may precede consumption of the final authenticated
Service record. The client retains those bounded received bytes and their original
queue reservation until consumed, explicitly closed, cancelled, or expired under
the original data lifetime. Only a clean transport EOF permits that drain; later
protocol failures remain failures. Role TLS EOF alone is not successful outer
completion: joined retirement waits for the peer's successful outer terminal,
retains refusal/transport errors, and joins cleanup before releasing ownership.

## Ownership

The local runtime has separate Modules and Interfaces:

| Module | Interface responsibility | Implementation hidden from callers |
|---|---|---|
| internal/application/broker | Admit and consume one short-lived Local Grant capability for either connection or administration; revoke, drain, and close pending capabilities and active Connection leases; report generic/unqualified. | Capability generation, replay removal, expiry, commitments, admission-load accounting, and grant invalidation. |
| internal/application/connection | Carry one typed Target-Link request and the fixed protected text exchange under AAI3; refuse reserved Name requests and join terminal/cancellation cleanup. It is not a generic binary Application interface. | State, Entry, Target, Route, worker authority, confinement, Service keys, retries, fallback, and Network diagnostics. |
| internal/application/administration | Carry one separately authorized `publish` or `withdraw` request and its closed success/unavailable result under the same interface version and vectors. | Connection bytes, publication inputs, Credential/key material, State, Route, Target, and Network diagnostics. |
| internal/endpoint | Compose the selected protected text participant and implement its AAI3 Connection Adapter plus the separate Administration Interface. `RunClosedParticipant` opens authenticated participant owners, delegates local transports to the Application Modules, and joins shutdown. | Broker consumption, authenticated State/Entry/Target projection, TLS carrier setup, publication acquisition, and Connection invocation. |
| internal/service/publication | Open, publish, acquire, unpublish, and close one exclusive Service Instance generation. | Crash-atomic public record/floor persistence, volatile Instance signer, live-reference accounting, drain, and private-material erasure. |
| internal/service/connection | Carry one logical authenticated Service Connection across fresh Route Attachments, preserve directional Application EOF through its existing authenticated Terminal record, and return one terminal outcome. | Exact Instance challenge/proof, continuity MAC, ordered data/acknowledgement offsets, replay handling, recovery deadline, and attachment cleanup. |

The caller-facing Endpoint seam is role-specific: a Publisher start request
cannot include Route, Credential, signer, or Application facts, and an outbound
connection cannot supply a Publisher binding. This keeps publication ownership, local admission, Route
attachment, and logical-stream recovery out of one mutable request bag.

The selected protected text Connection Interface adds one narrower consumer
operation over that composition. Its typed AAI3 caller supplies one explicit
Target Link and one fixed text request; Endpoint retains the local Connection
principal, authenticated State, Entry, Target authentication, Route inputs,
the closed token and durable attempt journal, worker qualification, and Broker
admission input. After Target-Link parsing and Network binding, Endpoint
activates and consumes the Connection capability before it reads current State, touches Entry
or private reachability, acquires a closed token, opens Route, or
sends Introduction. Only the fixed text exchange and bounded terminal class
cross the Interface. This does not preserve the generic AAI2 binary workload.
Administration remains separately authorized. The selected text Publisher
imports a bounded snapshot and invokes `PublishSnapshot`; its bodyless
`Publish` refuses because the generic `StartPublisher` transaction was
retired by
[ADR-0092](../adr/0092-retire-generic-publisher-transit-chain.md).
The `ardents endpoint publish` command refuses at dispatch before dialing its
Administration socket and directs the operator to `ardents-text publish` with
an explicit document file. The local bodyless protocol refusal remains intact.
`Withdraw` cancels and joins the retained publication. The Connection Interface cannot invoke either operation.

Before attachment handoff, the AAI3 server closes any Stream returned after
setup cancellation or failure, including a deadline-reset failure. It joins
that cleanup error with the local setup error and retains the first non-nil
Stream cleanup failure across client turnover for its joined `Server.Close`
result. Raw errors stay local; bounded refusal and cancellation precedence
remain unchanged. Endpoint `readResult.Close` cancels and joins its Service/
worker work before returning its retained result.

The fixed text reader has these setup outcomes:

| Observed condition | AAI3 caller result | Command presentation |
|---|---|---|
| The caller cancels its `Dial` context or reaches its deadline while setup is pending | Raw `context.Canceled` or `context.DeadlineExceeded`; the local caller context takes precedence | The existing cancellation or timeout diagnostic |
| Server-side setup returns a delivered `LocalCancellation` or `LocalTimeout` refusal | The same bounded class in `connection.SetupRefusalError` | The existing cancellation or timeout diagnostic |
| Endpoint refuses the typed Target Link with a bounded outcome, such as the retired alpha destination | The same outcome class in `connection.SetupRefusalError` | A fixed safe diagnostic for a recognized class; otherwise generic unavailable |
| Endpoint setup fails without a bounded refusal | `ServiceUnavailable` with the fixed safe reason | The existing generic unavailable diagnostic |

This table applies only to the selected Target-Link text reader. It does not
extend the Interface to Name, a generic Application, or a stream-terminal
result. For an already delivered setup refusal, the trusted client presents
`LocalFailure` as a local connection failure, `IndeterminateFailure` as an
unknown connection outcome, and the existing capacity class as capacity unavailable.
`ServiceUnavailable`, unknown classes and a refused `CleanClose` keep the
generic unavailable diagnostic. The caller context takes precedence over a
delivered refusal. The client never presents the refusal reason, Target Link,
or Endpoint failure detail. This presentation does not create missing Endpoint
classifications for authority, currentness, generation or stream termination.

The generic `route.Route` User composition described by
[ADR-0070](../adr/0070-own-volatile-user-route-orchestration.md) was retired
with its Open/Attach owner. The selected text Reader keeps the active Broker
lease in Endpoint, verifies the current protected State and Target, and uses
its closed Source, private Descriptor lookup, Introduction and JOIN owners.
Endpoint owns the durable closed-token attempt journal; State selects the
issuer and Route recipients. Service Connection receives only a verified
Attachment and immutable publication evidence. No generic User Route or
Transit Grant fallback is selected for C0.

## Local admission

The Broker has one volatile generation. A Grant is bound to one opaque local
Principal and one of the closed surfaces connection or administration. Admit
creates a fresh one-use capability. Receipt-only Administration operations
consume their capability before work and receive only its bounded receipt.
The protected participant additionally activates a retained Administration
Context lease before publication work. Connection activation also consumes its
capability and returns an opaque active-session lease whose cancelable context
is the ancestor of all Network work for that operation.
The one-use capability expires after its finite admission window; successful
activation does not transfer that pending TTL into the active Connection.
The lease exposes neither the capability nor authority facts, counts against
the Connection Grant's finite budget of 64 sessions, and is released exactly
once after the terminal outcome. Administration has a separate finite budget
of six pending capabilities plus active Administration Context leases combined;
it cannot consume the Connection floor. Receipt-only consumption and retained
Context activation are different uses of the same surface, not two budgets.
Endpoint also retains an independent bounded Context cleanup reservation after
lease revocation, until its descendants report joined cleanup.

Exact revoke and Broker or Endpoint close immediately cancel matching active
Connection and Administration Context sessions as well as invalidating
unconsumed capabilities. Drain
refuses new admission and is allowed only when that exact Grant carried
`PermitDrain` and the caller supplies a finite deadline. The first active-lease
drain deadline may only be shortened by later calls; it cannot be extended.
A missing or otherwise unprovable finite bound is denied or causes immediate
cancellation.

The Broker mechanism's isolation observation remains generic/unqualified. Its
receipt or lease makes no statement about sandboxing, hostile same-user
applications, process-tree confinement, supported host platforms, or
Application-level Endpoint Location Privacy. The separately verified installed
text-worker launch establishes its local binding under the selected
[confinement profile](application-confinement.md); qualification still requires
that profile's actual installed evidence. Broker admission alone cannot supply
that proof or a wider privacy claim.

## Publication and connection lifecycle

    Administration Grant
      -> participant-owned Endpoint runtime with an opened host Instance binding
      -> register the authenticated State-selected Introduction slot
      -> Publication.PublishAfterReadiness one higher Instance generation
      -> immutable public record + volatile signer
      -> the participant-owned Connection boundary activates a session
      -> session authorization precedes State/Entry/issuer/Route work
      -> exact-Instance TLS challenge/proof + Service Connection v3
      -> zero or more replacement Attachments under immutable recovery facts
      -> one terminal outcome and exactly-once session release
      -> withdraw/supersede stops acquisitions, drains references, erases private material

The Service Connection v3 record grammar uses its own fixed
`ardents-service-connection-profile-v3`; the selected protected Route keeps
its separate `ardents-route-v3` identity. There is no H3 reader,
record-profile negotiation, direct fallback, Publication private key, or
Application IPC authorization. The parser bound of 16 KiB per Data record is
an allocation limit, not a product throughput promise.

The Connection owner implements the successor's coalesced initial
InstanceChallenge/Continuity request and InstanceProof/Continuity response.
NewAuthenticatedStream verifies both proofs against the independently supplied
publication identity and authenticated TLS Attachment before returning a stream.
The Publisher verifies initial Continuity before invoking its opaque Instance
signer. The private initial-state receipt binds the exact Attachment, has zero
Application offsets, and is consumed once by the stream lifecycle without a
second Continuity flight. Cancellation joins the owned transport close and
prevents later receipt consumption. NewStream retains the preceding sequential
composition.

The Endpoint text-Service binding retains the exact verified worker job and
independently checks the signed publication against its live State profile
and local authority bounds. Its Initiator creates a fresh Connection nonce and
salted commitment to the private local context; the local context and salt do
not leave Endpoint. Both roles derive the selected immutable logical context,
while each Attachment derives its separate exporter context from the capsule
digest and generation. TLS 1.3 permits only the selected X25519MLKEM768 and
X25519 groups. Publisher TLS and Instance proof use the currently acquired
opaque publication lease.

The private Job lifecycle owns its random invocation nonce, verified worker
Grant handoff, retirement and first joined cleanup result. The text Context
retains the exact admission reservation until that Job reports joined cleanup;
it does not edit handoff fields. A late handoff is closed against its old Job
and cannot supply a Grant or completion to a replacement.

An owned Service binding refuses a canceled Context with that Context's
cancellation cause, including the interval before cancellation reaches its Job
and after joined Job cleanup. It does not synthesize a separate Job-retirement
failure from that propagation order. Independent Job retirement under a live
Context and a foreign Job remain distinct refusals; physical cleanup errors
retain their existing joined owners.

Context shutdown uses one explicit stop/join dependency table. Stop runs while
the Context mutex is held and revokes every child before any wait; join runs
after releasing that mutex. Extracted lifecycle owners detach and retire their
own state, while Context-owned maps and flights remain with the Context:

| Ordered phase | Owners or state | Required dependency |
| --- | --- | --- |
| Stop | refresh, Publication pair, Registration opening, Introduction/Responder/Source prefixes, issuance, resolution, withdrawal, exchanges and Job | Every admission/effect path observes revoke before the first join. |
| Join openings | Source, Introduction and Responder openings; Registration opening | No Route prefix is closed while its opening can still publish it. |
| Join producers | refresh and the Publication pair | No scheduler or registration producer remains before registrations close. |
| Close prefixes and join Context flights | Introduction, Responder and Source prefixes; issuance, resolution, withdrawal and exchanges | Route and Context-owned operations finish in their established dependency order. |
| Join Job, then release root | Job cleanup; durable Publication retirement; Context reservation | The first cleanup error and finite admission reservation survive until the last child is terminal. |

No generic callback registry participates in this order. Repeated Context
Close waits for and returns the one stored joined result.

The initial text-Service stream invokes that real TLS/native Connection path
and retains its opaque job binding. The reader holds its worker operation
through authentication and document exchange. Publisher accepts only streams
belonging to its exact job and context before transferring bytes to its one
worker; a foreign or retired binding is closed and joined. Cancellation joins
accepted stream forwarding and native I/O before releasing the worker operation
and installed cleanup. A successful document remains conditional on current
context ownership after cleanup.

The installed worker/network profile exercises the initial-attachment composition
with the real installed launcher, Introduction/Route producers, Service authentication
and confined worker protocols for empty, 64-KiB and 4-MiB documents on both Carriers.
Its State, Authority/Instance provisioning and registration scheduling remain explicit
fixtures; this does not qualify the complete protected journey or hostile host.
The four-Reader retained qualification shares one local Introduction delivery
slot. A Reader takes it immediately before submitting its capsule; completion
or refusal releases it, and the next submission waits at least 300 ms. This
paces acknowledged deliveries across the cohort despite variable preparation
and Route timing while preserving the Publisher's four-openings-per-second cap.

The Endpoint's text Publisher network producer retains one qualified worker
across independent reads and owns Introduction receipt, JOIN and authenticated
Service-stream handover. It uses an unbuffered handover; cancellation joins the
producer, bridge and worker. A refused malformed, unauthenticated or rate-limited
capsule can leave the snapshot available only when the refusal acknowledgement
succeeds and no cancellation or cleanup failure accompanies it.
The Publisher startup owner qualifies the worker before opening its Source,
Introduction, and Responder prefixes, creates the initial registration, and
waits for verified Descriptor publication before returning its Link. The
returned owner retains the network producer under the job lifetime, independently of the completed
startup request. Startup failure joins worker/context cleanup. Its Close is an
abort. Its separate withdrawal operation stops new Introduction acceptance before
network withdrawal and joins scheduled refresh before withdrawing the final
registration. Previously admitted reads retain their original lifetimes with an
additional five-second drain bound. Repeated withdrawal cannot extend that bound.
Scheduled publication refresh retries only a Source membership recheck whose
one-second local-role transaction ended because a concurrent producer still
held the lease. The failed opening publishes no token or registration; every
retry repeats current authority, State, selection and admission checks after a
fixed local delay. Other Source, Route, journal and cleanup failures remain
terminal, and retry never extends the registration, permission or Route expiry.
One private refresh lifecycle owns the single scheduler identity, coalesced
wake-up, cancellation and joined terminal result. A verified Descriptor ACK may
start it once or wake that same scheduler; context shutdown and withdrawal stop
and join the same result before any later network attempt can begin.
Producer drain preserves cancellation and cleanup failures, and context cleanup
waits for withdrawal ownership to finish. This composition still requires full
network lifecycle qualification and ordinary command adoption.
For a new authorized text context, the permission provisioning owner first uses
the installed launcher with an empty initialization and joins that preparation
worker before exporting any request. It opens no Service stream and publishes
no document. Cancellation or failed qualification cannot export a request;
a previously verified context still undergoes the current authority checks.
The text permission provisioning owner exports the exact public request and
reports its digest before waiting for an actual Custody response file. The
observer cannot grant authority. Waiting ends with caller/context cancellation
or the original request's hour boundary; malformed, partial or mismatched
responses fail without being retried into success. Only a verified import can
complete provisioning. The protected participant supplies trusted owner-only paths and consumes this owner before exposing its commands.
The text Administration owner retains that separately authorized context and
its original local Principal. Each snapshot publication and withdrawal consumes
a fresh Administration capability. Snapshot publication launches the installed
worker and returns success only after the real Descriptor acknowledgement;
bodyless publication cannot invent a document. Withdrawal can cancel and join
pending startup, while a committed publication delegates to the finite drain.
Shutdown aborts the context and joins startup and the retained run. The installed
network profile now exercises snapshot and withdrawal through the real local
Administration transport; that updated profile still requires execution on the
qualified host. Local refusal tests do not establish installed publication.
An explicit `ardents-text link` request consumes fresh Administration authority
and projects the committed publication's canonical Target Link through that
private local transport. Projection requires the current retained worker,
acknowledged live registration and current publication Credential with the same
Network and Target. Pending, withdrawn, expired or disconnected publications
return unavailable. The request cannot publish, retry or refresh, and the Link
is not a promise of future availability. The command joins cancellation of its
bounded output; ordinary runtime diagnostics do not contain the destination.
The text Connection owner consumes a fresh local Reader lease, launches the
installed worker, and completes Service authentication before returning its
AAI3 stream. It then validates the fixed local request, forwards the single
Service exchange through the confined worker, and joins worker retirement
before projecting its bounded RESULT through the local document grammar. This
projection creates no second remote request. Caller or original-context loss
interrupts both setup and local result I/O; owner shutdown joins the pending
read. One retained Reader context admits one read through result completion.
Local network tests exercise this result projection with explicit qualification
fixtures. The installed profile uses the actual AAI3 owner and launcher, but
that revised profile still requires execution on its qualified host.
The protected `RunClosedParticipant` composition opens the accepted closed State,
Entry sets and the `internal/admission/attempts` durable potential-spend owner.
The default composition also opens the existing Instance binding and provisions
both retained text contexts before opening the AAI3 Connection and snapshot
Administration transports. An explicit v2 `role: reader` selects only the
Connection context. It requires its own State/Source/time/local-role/Entry/token
inputs, Broker and Connection principal, Application socket and Reader Permission.
It rejects nonempty Instance, Publication, Administration or Publisher Permission
configuration and opens none of those owners, grants or transports. An omitted
role preserves the existing dual-role contract; any other role refuses before
runtime effects. The internal composition carries this choice as `ReaderOnly`.
Both compositions retain the same required worker qualification, State and
Permission currentness, finite allocation and joined shutdown. No protection
mode, wire identity or permission authority is added.
`endpoint headless` selects this composition through an explicit v2 plan;
missing permissions or mixed legacy fields fail without selecting another
runtime. The decoder refuses persisted v1 plans under
[v1 startup retirement](#v1-startup-retirement) before runtime dispatch. The
protected composition still requires installed command
and full network lifecycle
qualification. No caller-supplied Target, permission file or local context
identifier may bypass these owners.

Reader-only component checks do not establish the two-Endpoint installed journey.
The fixed worker boundary verifies the MainPID of `ardents-endpoint.service`;
two Endpoints therefore require separate system-manager installations, each with
its own roots and principals and the unchanged fixed unit/account/worker checks.
Fixture-written plans do not supply the supported authenticated installation
handoff or a complete Ubuntu systemd/cgroup qualification.

The participant serializes local lifecycle output. Each event records UTC occurrence time before output delivery; the local JSON-line adapter uses `schema`, `kind`, and `at` for correlation with Node lifecycle events while retaining the existing bounded, role-specific fields. A background failure event uses a
bounded observer context; if delivery fails, the participant ends the generation,
joins its owners, and returns the output failure instead of silently discarding it.
After the event output is acquired and the participant has joined its owners,
an uncanceled fatal return emits
`headless-runtime-failed` with only `startup` or `running` as its failure category.
It does not serialize the returned error; stderr retains that detail for local
investigation. A failed event output is not retried through the same output.

The coalesced authenticated stream requires its directional Terminal receipt
and peer confirmation even when only the initial Attachment is available.
Missing confirmation cannot yield a successful bounded outcome. The protected
text-Service owner now installs a Route-backed replacement source before
Application bytes are exposed. It retains the verified Introduction recipient,
selected Source or Responder prefix, original job, Candidate View, logical
tuple and Work Safety bounds. Every proposed generation uses a fresh capsule,
JOIN secret, handshake context, delivery and request nonces, token presentation,
Service TLS exporter and Attachment context. Client and Publisher recheck their
current job, State, publication or registration, and local recovery deadline
before transfer and after asynchronous opening. A successful proposal transfers
the Route lifetime from its bounded opening attempt to the Service Connection,
so normal attempt cancellation cannot retire the accepted Attachment.

Trusted worker composition also supplies one checked directional byte contract
before the job can receive a Service stream. The ordinary text composition
retains the existing 512-byte reader request and 4 MiB plus 13-byte framing
response bounds; the fixed qualification composition retains 64 MiB in each
direction. Reader and Publisher derive their opposite directions from that one
contract. The native Service stream therefore neither imports text-document
policy nor inspects qualification job input, and recovery continues the same
logical byte counters instead of resetting either bound.

This is maintained component integration, not installed-host P7 or NET-14
qualification. A path that supplies no Attachment opener retains its existing
orderly half-close behavior and cannot recover.

After a replacement Attachment commits, the Connection replays any accepted but
unacknowledged Data suffix without waiting for a further local Application read,
EOF, or Terminal. That replay remains ordered with later Application bytes and
is joined or interrupted by the Connection's existing terminal cleanup.

Publication startup joins any ownership/preparation or restore refusal with a
failed exclusive-root release, returning no owner. Retained root evidence and
floors are not reset as part of failure cleanup.

Publication persists public proof and its non-decreasing generation floor but
never persists a live Instance private key. The supported generation floor
comes only from the current Publication root's floor file. Its persisted encoding
is canonical nonzero decimal uint64 followed by LF, including 20-digit values;
malformed and overflowing encodings refuse without changing the floor. An empty
owned root
starts at zero; a root retaining a generation or current pointer without its
floor refuses recovery. The former separate plain-decimal generation file has
no Target, Authority, or Network binding. Its bytes remain untouched: the
maintained runtime neither reads nor migrates it. The maintained Publisher
participant receives one opened host Instance binding as an opaque signer. For current
private Introduction, it creates a volatile `PrivateRecipient` with a bounded
revision and expiry; the recipient opens only the authenticated private
capsule, without an Interface returning private bytes or an exportable HPKE
key. The SealedIntroduction v1 grammar is retired by ADR-0094; the current
private Introduction path is only the v3 capsule recipient. ADR-0102 supersedes ADR-0034: the Service Instance root emits
only its ed25519 Instance key, the accepted Credential v3 no longer binds a
legacy introduction recipient, and the private v3 capsule uses its separate
volatile recipient. Old Instance roots fail the v3 marker check without state
decoding and require re-initialization under a new root.
The maintained closed participant requires `service_instance_root` and
opens its Instance binding only after reconciling the accepted public
Credential with the durable publication floor. It reads the current accepted
closed Route profile and Snapshot from State, checks the matching generation
and issuer/recipient Node Records, and uses Endpoint's closed token journal
for admitted work. Missing, conflicting, expired, or mismatched State/profile
blocks that work; the Application cannot choose peers, roles, keys, or Route
facts.
AcquireAt yields an opaque Lease; the Lease can sign for its generation without
exposing the signer. Withdrawal, supersession, expiry, or close first prevent
new acquisition, then wait for bounded references before erasing private
material. If the caller cancels while that drain waits, the Publication retains
the withdrawn generation's cleanup ownership; a later publish, withdrawal, or
close joins the same drain before it can release the root lease or expose a
successor generation.

Publication persistence flushes each created ancestor link, the marker and
root layout before opening. A higher floor is file-synced, renamed, and its
parent directory synced before readiness. The immutable record is file-synced,
its staging directory synced, renamed, and the generations directory synced
before the file-synced current pointer is renamed and the root synced. Pointer
withdrawal precedes generation removal; both parent directory barriers must
complete before retirement ownership is released. Sync errors remain errors,
including after a visible rename or removal. An ambiguous publish blocks further
publication and floor receipts on that open owner until reopen reconciles disk
state; it never exposes a live signer. Reopen retains the floor and cannot
revive persisted private material. Platforms/filesystems that reject a required
directory flush refuse. Injected phase tests cover ordering and refusal; they do
not qualify actual power-loss behavior or provide a power-loss receipt.

After Publication reaches root-lease release, it retains that terminal cleanup
result for repeated and concurrent Close callers. Earlier failed withdrawal or
generation drain remains retryable under the existing lifecycle; a retained
release error never recreates a signer or resets the publication floor.

## Service credential and publication limit

The Instance key is generated by the Service host and bound into its public
Service Credential. The inner Service TLS challenge/proof authenticates that
exact Instance; it is not a general mandatory-mTLS product layer. In
particular, an X.509 certificate elsewhere in the deployment has no authority
to create, replace, or renew a Service Credential.

The currently supported Custody issuance is interactive and produces one
public signed response for an independently approved public request. A
Credential lasts at most 24 hours, may end no later than 48 hours from
issuance, and cannot overlap a predecessor for the same Target. There is no
automatic Credential renewal route. The C0 successor rule is an implementation
limit, not a permanent product requirement.

After `CommitPublished`, the Instance root deliberately no longer retains the
private material needed to reopen that published generation. A real
participant restart therefore fails at `Credential()` before it could call
`OpenBinding()`; a successor cannot begin before the predecessor's terminal
`NotAfter`. Deleting a root or publication floor, or changing Target, is not a
recovery procedure for that Service. A later, separately selected design may
change this limitation, but the maintained runtime has no such recovery path.

## Endpoint process contract

### v1 startup retirement

The selected successor behavior for an `ardents-headless-runtime-v1` input is
one bounded refusal before State, Entry, Route, Application sockets, network
operations, or durable roots are opened. The refusal leaves every existing
root and floor byte unchanged. It does not synthesize or convert a Grant, key,
permission, protected plan, or other authority. Malformed or incomplete v2
input is refused by the v2 path and never falls back to v1. Existing retained
bytes require a separately selected reader, recovery, or migration contract;
startup retirement supplies none. The command decoder now enforces this before
validating or opening any plan-owned path. The unreachable `RunParticipant`
composition and its exclusive configuration/event wiring have been removed.
That startup removal did not retire Administration, which remains selected.
The separately callable AAI2 Connection was removed only after its own caller
retirement and AAI3 version-refusal evidence.

## Generic Connection command retirement

| Option | Authorizing consumer and finite workload | Product consequence | Decision |
|---|---|---|---|
| Preserve through a generic AAI3 caller | None exists. The selected AAI3 caller is the fixed protected text reader, not an arbitrary byte application. | Would widen the trusted Interface and confinement contract and make an unsupported generic Application a product surface. | Rejected. |
| Retire generic `endpoint open` | No successor consumer is required; the command is closed at its adapter before effects. | Removes the file-to-file binary CLI contract while preserving protected text, Target Links, Administration, and shared native stream semantics. | Selected. |

The generic `ardents endpoint open <application-socket> <target-link>
<input-file> <output-file>` command is selected for retirement. No current
product journey or maintained Application requires its arbitrary binary
file-to-file workload, and no real generic AAI3 Endpoint caller exists. The
AAI3 caller belongs to the protected text composition: it carries a typed
Target-Link request, launches a qualified fixed worker, and enforces that
workload's bounds. Treating it as a generic replacement would widen that
Interface and its confinement claim without an authorizing consumer.

The command's binary input, concurrent binary output, explicit input
half-close, terminal-class rendering, and output-file commit are therefore
retired as a caller contract, not translated. The bounded enforcement point is
the command adapter: a recognized exact `endpoint open` invocation returns a
deterministic non-success result before validating or opening either file,
creating an output, dialing the Application socket, or causing Endpoint,
Route, or Network work. It selects no alternate command, text request, Target,
or migration path.

This decision preserves four separate facts. Target Links remain the maintained
destination input; future protected Service Names require a new scoped design
under ADR-0113. The
fixed text AAI3 Interface remains selected. Service Administration remains a
separately authorized Interface. The shared native Service Connection retains
its directional half-close semantics for selected callers. None of those facts
is a caller for AAI2, and qualification-only or conformance fixtures cannot
supply one. After both accepting callers closed, the AAI2 codec/server/client,
vectors, and exclusive Endpoint adapter were removed. AAI3 now rejects a
complete AAI2 request before calling its Application owner.

The command adapter now implements that refusal with the stable diagnostic
`endpoint open is retired`. The former accepting file client and its
success/cancellation fixtures are absent. A command-level regression uses both
missing and existing files plus an available local socket to prove the refusal
precedes file validation or mutation and IPC connection. The separately
callable Administration client and its behavior test remain unchanged.

The removed AAI2 grammar accepted a non-empty Target Link, opaque frames and a
typed terminal outcome. Those bytes have no maintained decoder, server, client,
Endpoint adapter, persisted-state reader, or compatibility promise. The
Service Connection v3 is a separate network identity and is unaffected by
AAI2 retirement. `internal/application/administration` separately owns
only `publish` and `withdraw`; it cannot carry Connection data or silently turn
a failure into another success state. No Browser client is selected in the
maintained product.

The Administration client owns its Unix socket from successful dial through the
closed `publish` or `withdraw` response. Caller cancellation immediately
interrupts that owned request I/O and returns the caller's cancellation or
deadline error; a completed response wins only when its cancellation callback
has already been stopped. This aborts local waiting, not a server operation
already accepted by the peer: the client never invents an outcome, retry, or
rollback for Publish or Withdraw.

The Administration `ardents-application-interface-v1` identity remains its own
contract and is not an AAI2 Connection fallback. The historical Alpha corpus
triple has no accepting runtime adapter: v1 plans are refused before path
validation or owner startup, and v2 rejects those fields. Existing retained
bytes require a separately selected reader, recovery, or migration contract;
this removal creates none.

### Alpha destination retirement

Under
[ADR-0088](../adr/0088-retire-alpha-service-links-and-corpus-intake.md),
the Alpha Service Link transition is an explicit final refusal, not a grace
period. Fresh `accept-alpha-corpus` intake now returns its stable retirement
refusal before parsing arguments or opening, creating, or changing either named
floor. The former accepting adapter and its floor-mutation authority are
absent. Under
[ADR-0110](../adr/0110-retire-aca2-corpus-inspection.md), the independent
`inspect-alpha-corpus` diagnostic is retired the same way: the exact route
refuses before parsing arguments or opening any file, root, or floor, and its
ACA2 production verifier is removed; it never conferred Endpoint authority. Every accepting Alpha destination adapter
is absent. The maintained Target-Link seam recognizes the exact historical
`ardents-alpha://` prefix only to return `alpha service link is retired`; it
does so before Target-Link decoding, a corpus-floor read, resolver call,
Network or Route work, dial, fallback, or conversion.

The transition neither converts an Alpha Link or old Target nor resets or
deletes existing corpus floor files.
[ADR-0113](../adr/0113-retire-retained-alpha-compatibility-surface.md) then
deleted the retained read-only corpus parser and persistent-floor reader
outright: those floor bytes are byte-for-byte inert evidence — their serial,
digest, signed withdrawal, rollback, and conflict facts remain on disk, but
no maintained code can read, convert, or delete them, and the absence of any
read path is itself the incompatibility. They do not authorize continued
Alpha resolution, and no successor reader, migration, or grace contract
exists.

Endpoint is a composition Module, not a second durable domain owner. It owns
no Namespace, Network State, Release, Update, Custody, or Route-selection
state. Route Attachments are already authenticated opaque carriers; Namespace
and State facts arrive only in the typed inputs required for Connection
binding.

`Stream.CloseInput` remains an orderly directional operation in the selected
AAI3 transport and native Service Connection, distinct from `Stream.Close`.
The local transport preserves it through Endpoint as the authenticated native
Terminal record. Conversely, only a verified matching remote Terminal gives
the local Application reader EOF. Either transition leaves the opposite
direction available for a response, is safe to repeat, and rejects later
writes in its closed input direction. Cancellation, malformed local input,
carrier loss, and full close remain abort paths, and neither local nor native
EOF is semantic success without the one typed terminal outcome.

A locally written Terminal is a directional completion obligation, not proof
of peer receipt. Under the closed Service Connection v3 grammar selected by
[ADR-0117](../adr/0117-reset-closed-service-formats-to-v3.md), marker `1`
is a Terminal receipt: it names the same generation and offset and is sent only
after the peer verifies that Terminal. Marker `2` confirms the peer observed
that receipt; the receiving endpoint retains recovery ownership until it has
that matching confirmation. If a replacement Attachment completes Continuity
before either control record, Service Connection owns replay: it first replays
any unacknowledged Data at the carried offsets, then emits the same Terminal on
the new generation. The peer still presents only its first verified Terminal as
Application EOF. This preserves one logical half-close without reissuing an
Application operation or inventing EOF on a timer. A headless path that did not
supply an Attachment opener retains its existing orderly half-close and does
not enable recovery.

The endpoint that sends marker `2` cannot learn from a finite record whether
the peer received that final confirmation. After a successful `RunBounded`
outcome, a recovery-enabled Service Connection therefore retains a bounded
in-memory post-close tail until the operation Context, authenticated Work
Safety deadline, or no-new-recovery boundary ends it. That tail owns only the
already-settled Terminal and receipt-control replay; it accepts no Application
Data and never presents another Application EOF. Thus the caller's completed
Application operation is released while a returned peer can still recover the
missing final control proof. The tail adds neither durable recovery state nor a
headless recovery path. The Endpoint may publish that completed Application
outcome before the tail ends, but explicit text-stream close cancels and joins
the native tail, its current Attachment and the owning Introduction exchange
before releasing the stream owner. TLS may map the admitted Route child's
authenticated `CLOSE(0)` to transport truncation because Service TLS has no
second close-notify exchange. The exact child exposes that already decoded
clean retirement to the text-Service adapter; only that witness may end a
peer's already complete tail without recovery. Raw EOF, local close, refusal,
truncation without the witness and every other Carrier failure retain the
bounded recovery path.

Explicit publication withdrawal uses a fresh Service Administration capability
and returns `unpublished` only for the exact Target/generation after retained
connections drain; an established connection may therefore finish as `clean
service connection close` while no later publication acquisition is possible.
A fresh repeated withdrawal must return `service unavailable`; the publication
owner rejects acquisition as soon as unpublish begins, before retained leases
finish draining.
A non-EOF Publisher Application socket failure and an abrupt Publisher Endpoint
loss are `abrupt connection loss`, never `service unavailable` or clean close.
An Application may receive a byte prefix before an abrupt failure. The typed
terminal outcome remains authoritative for the Connection lifecycle; received
bytes do not prove that an Application operation completed. Interpretation of
HTTP status, response completeness, or semantic retry belongs to the external
Application. Endpoint never substitutes another Target or an Internet path.

The Endpoint contains no Browser presentation or Browser Entry state. The
former Browser implementation and qualification lanes are retired, and
[ADR-0091](../adr/0091-retire-uncomposed-legacy-artifacts.md) retired the
in-tree Firefox compatibility source; it survives only in Git history in
accordance with [ADR-0061](../adr/0061-retain-firefox-entry-as-compatibility-evidence.md)
and [ADR-0069](../adr/0069-retire-active-browser-implementation.md).

## Verification and related decisions

- Go tests for Broker, Endpoint, Publication, and Service Connection exercise
  the Module Interfaces and failure paths.
- Protected text AAI3 Connection and Administration behavior tests exercise
  framing, typed refusal/outcome, cancellation, join, and exact socket cleanup
  through their public Interfaces. AAI3 rejects the removed AAI2 request before
  Application owner I/O. Architecture tests forbid a second Endpoint-local
  transport owner and enforce the command dependency graphs.
- [ADR-0024](../adr/0024-native-interactive-route-foundation.md) selects the
  native Route foundation; [ADR-0075](../adr/0075-service-connection-v2-terminal-receipt.md)
  selects the closed Service Connection grammar.
- The Broker is limited to its explicit generic/unqualified contract; it makes
  no platform-isolation or Application-level Endpoint Location Privacy claim.

## Local process debugging

The command adapter can explicitly enable the owner-private process Interface
with `ARDENTS_DEBUG_SOCKET` under the
[local diagnostic contract](../development/local-diagnostics.md). It supplies
runtime counters and finite profile/trace requests without modifying Route,
State authority, peer selection, Application privileges or worker confinement.
Empty input creates no diagnostic resources; configured admission failures
precede product dispatch and diagnostic cleanup is joined to command outcome.
The diagnostic interface reports process observations; existing owner events
remain the source of readiness and typed product failure.
