# Endpoint and Service runtime

## Selected protected installation handoff

[ADR-0125](../adr/0125-separate-software-delivery-and-infrastructure-deployment.md)
separates platform software delivery from Endpoint runtime. The transitional
`ardents-next installation` CLI, guarded startup exchange and transition
recovery have retired. Existing roots, histories and Release floors are
retained. The platform path refuses transitional or partial records in the
selected installation root.

The per-user Portable Endpoint command surface is retired under the same ADR
(#617 S3.5): `ardents endpoint enroll/enroll-installed/headless/user-unit/
installed-user-unit/replace/rollback/replacement-self-test/replacement-recovery`
return typed refusals before input or root effects, and their legacy command
implementations are removed. The canonical protected text Endpoint launch is the
platform-led system-unit composition in `ardents-next`. An explicit
`platform initialize-trust <launch> <independent-enrollment-pin>` authenticates
the initial protected Endpoint bundle through Enrollment and establishes both
target floors through Release. The operation uses the actual enrolled executable
and the declared Release root. With that exact history already committed,
`platform stage-endpoint` seals the generation for the platform-provided
account and vacant mutable roots. It rejects later Release metadata, preexisting
installation roots and nonempty mutable roots. The full original generation and
selection are synced before the service account can traverse the installation
root. The stage installs no Worker resources or systemd units.

For signed replacement, `platform stage-endpoint <request> <previous-launch>`
uses the existing Release history and prepares a different immutable
installation root. The platform caller retains the original unit reference
through shutdown and staging. When a Worker is active, signal only the main
Endpoint process with
`systemctl kill --kill-who=main --signal=SIGTERM ardents-endpoint.service`.
Wait for the original process to exit successfully so Execution can join the
Worker and its borrowers. Then stop both fixed activation sockets. A direct
systemd Stop transaction can stop a dependent Worker before Endpoint has
retired its own use and leave a failed Endpoint result. Staging verifies the
original successful execution, both stopped activation sockets and the empty
kernel scope inventory before authenticating the candidate against the
retained Release floors. A lost original exit observation, transitional records, queued or
running work, or an existing candidate root refuse. The account, Network ID,
mutable root paths and original directory identities are retained. Only after
the new generation is sealed does the platform operator install the new fixed
resources and unit and ask systemd to start it. Initial pin and domain history
creation are first-delivery actions. A failed stage may leave an inaccessible
partial candidate root or advanced Release floors. The command never resets
floors or overwrites the partial root.

Platform unit rendering and pre-start bind the executable path to the digest of the enrolled
protected Endpoint descriptor. The platform unit's ExecStartPre `platform check` transfers accepted current-program identity before
runtime start (F-25), `endpoint construct-plan` renders authenticated preparation
plans (F-27), `endpoint prepare-history` initializes the first histories, and
`endpoint start-installed` runs the participant under the system unit. The
successor Endpoint lifecycle is forward-only; rollback has no successor
translation. The read-only `ardents endpoint enrollment-check` diagnostic and the
`ardents endpoint withdraw` Administration Interface client remain. Existing
per-user roots and floors remain retained evidence. Their contents grant no
authority for automatic migration.


The fixed unit's privileged pre-start check binds the verified Release to
the exact executable and platform unit. Runtime independently retains the
selected generation and original process, account, manager and activation
sockets. Full content observation runs at operation and admission boundaries.
Inner effect brackets verify original file identity, account, process and
manager, and a refusal remains terminal for the original invocation.

Private Startup manager observation reads Version and both complete rounds of
fixed Endpoint/activation properties on one fresh bounded system-bus connection.
Each round submits its six fixed read-only queries together under the original
three-second query bound and joins every reply before the next round. A failed
round cancels and joins its outstanding calls and retains their errors.
It retains original unique manager identity across Startup checks and compares
that owner before/after each fresh read. Nested message lengths and duplicate
dictionary keys are checked before library decoding, and original wire variant
signatures survive typed projection. Cancellation and return join the physical
connection. File/inode/access/inventory, process and manager-property checks
remain at their original boundaries; detached facts grant no accepting cache.
Before first runtime, `ardents-next endpoint construct-plan NETWORK_OWNER
SOURCE_PLAN DECLARATION OUTPUT_DIR` renders the canonical `headless.json` and
`source.json` preparation plans without manual plan editing. The network owner
input reuses the established local Network owner pin grammar; its pins must
open the referenced already accepted retained State and yield a current
closed-profile observation, so a wrong network identity, authority set,
threshold or profile authority refuses before any byte is rendered. The plan's
network trust facts originate only from those authenticated pins. The bounded
operator declaration supplies local storage roots, sockets, execution
principals, permissions and role; the distributed Source plan artifact supplies
the client credential paths, both source members and the refresh grammar.
Construction reuses the same offline declaration projections preparation and
runtime consume, including the Source credential files, and refuses overlapping
preparation roots or roots overlapping the accepted reference State. Output
files are exclusively created and synchronized; existing bytes are never
adopted or overwritten. Construction opens no preparation root and creates no
State, permission, Stock, Service or runtime authority; its result is
preparation input, not an accepting receipt.

`ardents-next endpoint prepare-history HEADLESS_PLAN BUNDLE
SIGNED_PROFILE` performs finite offline preparation under the actual runtime
principal. The four declared State, participation, token and Entry roots must be
existing vacant private 0700 directories. State's exclusive fresh claim remains
held while State authenticates and durably accepts the supplied public Epoch
bundle and closed profile under the declaration's independently selected pins
and clock observation. Participation then initializes its empty first history;
Route selection initializes empty Entry and separately bound role Interior
histories without drawing members. Stock initializes separate Reader and, for dual role, Publisher potential-spend
histories from genuine current Network observations. No permission is requested
or imported. No Source transport, worker, Startup or Service is opened. The
command joins every original owner before reporting preparation. A failed or
partial preparation preserves its files and refuses another initial attempt;
it cannot clean, adopt or reset them. This operation is not an installer or
recovery controller. Hosting and Service Instance preparation remain separate.

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

The Reader's Connection composition requests missing Stock for its already
selected recipients through that same acquired Source before constructing the
capsule and starting its short setup deadline. Command composition retains
issuance policy and the original whole-operation deadline. Existing Forward
Stock requires no additional issuer Control reserve; missing Forward is funded
through the existing class-specific exchanges. Preparation neither presents a
recipient token nor replaces the later HELLO and live-authority checks.

Endpoint context composition serializes issuance admission with its retained
Source-operation reservation, but one private issuance operation owns each
admitted attempt's network context, cancellation, issuer presentation and
terminal completion. Context or Endpoint revocation cancels and joins that
operation before permission material is released. A delayed completion cannot
publish usable stock after revocation; ordinary retry retention and the
recovery-only canceled-batch discard remain distinct and consume the same
existing reservation and bootstrap allowance.

The command's shared on-demand token flow serializes terminal-role Stock
preparations through joined issuance and each final token presentation. Waiting
callers retain their original cancellation and deadlines. An issuer exchange
uses its already funded Control directly within that preparation; it cannot
recursively acquire the same preparation or obtain missing issuer Control.

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
| internal/application/connection | Carry one typed Target-Link request and the fixed protected text exchange under AAI3; refuse reserved Name requests and join terminal/cancellation cleanup. It is not a generic binary Application interface. | State, Entry, Target, Route, worker authority, confinement, Service keys, retries, fallback, and Network diagnostics. |
| internal/application/administration | Carry one separately authorized `publish` or `withdraw` request and its closed success/unavailable result under the same interface version and vectors. | Connection bytes, publication inputs, Credential/key material, State, Route, Target, and Network diagnostics. |
| internal/endpoint | Compose the platform-led protected text participant over the maintained AAI3 and Administration Interfaces, with joined shutdown of its role owners. | Execution session admission, authenticated State/Entry/Target projection, TLS carrier setup, publication acquisition, and Connection invocation. |
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
[ADR-0092](https://github.com/dianabuilds/ardents-network/blob/c58268d72f1eaeae173144e76d32d50986de2a77/docs/archive/adr/0092-retire-generic-publisher-transit-chain.md).
The `ardents endpoint publish` command refuses at dispatch before dialing its
Administration socket and directs the operator to `ardents-text publish` with
an explicit document file. The local bodyless protocol refusal remains intact.
Snapshot Administration has one absolute 45-second publication deadline from
request acceptance, including input and response. Header, body and EOF input
remain bounded to the original 15 seconds. The Publisher receives the remaining
absolute deadline; no phase renews it. `RequestSnapshot` bounds its caller from
before dialing to 45 seconds, preserving any earlier caller deadline or
cancellation. Server shutdown cancels and joins its operation; the half-close
protocol does not transmit the caller's context. Other Administration operations,
bootstrap and role bounds remain unchanged.
`Withdraw` cancels and joins the retained publication. The Connection Interface cannot invoke either operation.

Before attachment handoff, the AAI3 server closes any Stream returned after
setup cancellation or failure, including a deadline-reset failure. It joins
that cleanup error with the local setup error and retains the first non-nil
Stream cleanup failure across client turnover for its joined `Server.Close`
result. Raw errors stay local; bounded refusal and cancellation precedence
remain unchanged. Endpoint `readResult.Close` cancels and joins its Service/
worker work before returning its retained result.

AAI3 Reader setup and its single exchange share one absolute 60-second bound
from socket acceptance. Request input remains limited to 15 seconds from that
same acceptance; completing input does not renew the operation deadline. Setup
and stream reads/writes retain the original bound. The trusted text Reader
bounds its caller from before dialing through the exchange to 60 seconds;
earlier cancellation still applies. Bootstrap remains bounded to 10 seconds,
and new recovery remains forbidden in the final five seconds of the original
Work Safety interval. No additional batch, retry or authority cache is granted.

Under [ADR-0121](../adr/0121-bound-installed-connection-setup.md), each
Attachment selects one capsule setup deadline at most thirty seconds ahead,
clamped to the original caller, registration, Work Safety and recovery bounds.
Introduction and the private recipient enforce that same absolute deadline.
Unpaired JOIN also ends no later than thirty seconds from its original
reservation. Pairing cannot renew setup or extend the Reader operation.
Reader's initial authentication reobserves that original setup caller after
each joined proof/Continuity read. Setup expiry remains a deadline failure even
when its interruption callback closes the Attachment with physical cancellation;
the actual input error is retained alongside it. Complete malformed records
remain integrity failures, while incomplete input alone supplies no integrity
evidence. Buffered complete bytes cannot survive an expired setup caller.
Publication construction obtains missing Forward Stock for its already selected
Introduction and Responder legs in one admitted batch through the original
Source. Repeated recipients require distinct tokens. This spends the same
finite allowance earlier and never opens a Responder before capsule validation
and nonce commitment; each later HELLO still durably presents its own token.
Reader and Publisher Source construction also includes the signed inventory
Descriptor Store recipient in its existing second bootstrap Control batch.
The restricted bootstrap channel still reaches only the issuer. Actual Store
lookup/publication uses fresh admitted role transport and rechecks the selected
recipient; early Stock grants neither a Descriptor nor a successful operation.
For an already funded recipient, presentation invokes Stock's full `Take`
directly, including while a different missing-token preparation owns issuance.
Only its explicit pre-consumption stock-absence refusal may
enter missing-token preparation; authority, cancellation, signature and journal
failures cannot trigger issuance. Missing-token paths remain serialized through
joined issuance and final presentation. If an opening's first token is missing,
its batch retains the ordered deficit for both selected forwarding recipients.
Every returned token is still freshly checked and
durably marked, with no cached permission or refunded right.

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
The retired `endpoint headless` selector refuses before reading its former
plan or opening runtime roots. The process regression checks that refusal and
preserves the plan and State bytes. The platform-led installed command uses
its own selected declaration and runtime. It preserves the required worker
qualification, State and Permission currentness, finite allocation and joined
shutdown. Caller-supplied Targets or context identifiers confer no authority.

Reader-only component checks do not establish the two-Endpoint installed journey.
The new canonical `ardents-next endpoint start-installed` continuation exposes
the separate AAI3 and Administration Interfaces after original startup, signed
State and finite Stock handover. Reader-only continuation opens no Publisher
owner or Administration listener. Dual-role continuation constructs the real
snapshot Publisher only on an authorized local request; bodyless Publish refuses.
After the joined Store ACK, Publication joins the registration pair's setup-only
caller cancellation before its final fresh checks and accepting commit. A caller
cancellation that already won still refuses; completion of a successful local
request cannot cancel the retained pair. Its original Publisher/Execution parent,
registration expiry and withdrawal remain independent lifetime bounds.
Link and Withdraw consume fresh local Administration admission from the original
Execution generation, in addition to actual Publication checks. Withdraw
interrupts and joins pending setup before retiring a committed owner; it does
not extend or replace that setup. Unexpected accepted-stream consumer termination
stops participant supervision and retains the original failure.

One participant retains actual Entry history at `entry_state_root/entry` and
the already initialized Hosting budget at `entry_state_root/hosting`. Its Reader,
Introduction, Publisher Source and Responder selections use separate
`reader-interior`, `introduction-interior`, `publisher-source-interior` and
`responder-interior` siblings. Runtime never initializes or resets the budget.
Per-operation composition returns its own physical borrows and role selections;
listeners and pending/active consumers join before the participant closes shared
roots. These production consumers still require source-matched native acceptance
with two independently admitted managers; compilation and local admission tests
supply neither startup qualification nor Service success.
An installed continuation refusal emits only its fixed owning stage and
`unavailable` to local diagnostics after joined cleanup. It includes no original
paths, authority bytes, Target, error strings or accepting readiness. The
unchanged nonzero process result remains the refusal.

The genuine Reader and Publisher Source composition obtains missing cold
Entry/Interior Forward stock through the original finite issuer-only Bootstrap.
The second batch funds that profile's issuer Control token. It joins Bootstrap
before opening fresh admitted Source channels with the same selected leg and
shared provider history; it does not upgrade a bootstrap child. One original
Stock preparation excludes unrelated issuance during those batches/opening.
Already funded Source stock consumes no additional bootstrap batch. Exhausted
cold bootstrap refuses rather than renewing permission or rebinding another Job.
Subsequent requested Control, Forward and Registration presentations obtain a
missing token through that same admitted Source's real issuer exchange. Issuer
Control must already be funded and cannot recursively issue itself. Original
Operation/Startup/Network checks, exact challenge/window, whole-result deposit,
irreversible allocation/presentation and physical reservation joins still apply.
These mechanisms require source-matched installed Publisher/Reader exchange and
concurrency/recovery evidence; Route/Stock component checks do not qualify it.

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
Missing confirmation cannot yield a successful bounded outcome. The stream
also joins an original Terminal receipt write before classifying a
concurrent carrier EOF. A peer confirmation can arrive before that write returns;
only its successful local result completes the receipt, and an actual write
failure remains a failure even after the peer confirmation.
The protected text-Service owner now installs a Route-backed replacement source before
Application bytes are exposed. It retains the verified Introduction recipient,
selected Source or Responder prefix, original job, Candidate View, logical
tuple and Work Safety bounds. Every proposed generation uses a fresh capsule,
JOIN secret, handshake context, delivery and request nonces, token presentation,
Service TLS exporter and Attachment context. Client and Publisher recheck their
current job, State, publication or registration, and local recovery deadline
before transfer and after asynchronous opening. A successful proposal transfers
the Route lifetime from its bounded opening attempt to the Service Connection,
so normal attempt cancellation cannot retire the accepted Attachment.

Each peer anchors its recovery cutoff to its own authenticated progress; these
independently derived deadlines need not be ordered. Publisher preparation and
commit retain the original local cutoff while the private opening, its caller
and Route retain the unchanged sealed request/JOIN deadline. A later peer
deadline grants no extension of the earlier local cutoff; it is not itself an
integrity failure while both original bounds remain live.

Trusted worker composition also supplies one checked directional byte contract
before the job can receive a Service stream. The ordinary text composition
retains the existing 512-byte reader request and 4 MiB plus 13-byte framing
response bounds; the fixed qualification composition retains 64 MiB in each
direction. Reader and Publisher derive their opposite directions from that one
contract. The native Service stream therefore neither imports text-document
policy nor inspects qualification job input, and recovery continues the same
logical byte counters instead of resetting either bound.

The new Connection stages received bytes in a bounded private buffer until its
final original-owner fences succeed. Caller-buffer delivery then occurs under
the same local handoff locks, before publishing a clean outcome. A failed final
fence returns zero bytes, leaves caller memory unchanged and retains its cause;
authenticated EOF remains the exact `io.EOF` sentinel on a successful handoff.
Read planning does not consume the Application cursor. Write planning does not
append bytes to the replay ledger or increase the returned acceptance count;
both transitions occur only after those final fences under the same locks.

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

The isolated new `publication/instance` owner generates its own Ed25519 key,
stable canonical public request and exact external signed-response transition.
`publication instance-initialize <root> <network> <not-before> <not-after>` and
`instance-request <root>` return the public request as JSON base64;
`instance-accept <root> <response-file>` consumes untrusted raw response bytes.
Exact accepted retries preserve the same public Credential digest. Foreign or
changed responses terminally conflict; malformed pending responses reject.
Terminal records redact private material. Ambiguous post-rename sync or original
caller loss stops further receipts and clears the open owner's private bytes;
reopen independently reconciles the persisted response state. Private custody
uses original native Linux root/lease identities and real directory barriers;
other platforms refuse before key or filesystem creation. This mechanism does
not broaden ordinary Ubuntu24/systemd255 qualification. It grants no generic
signer, Authority custody or accepting readiness. Its separate private binding
reconciles a concrete original Publication durable owner; a supplied scalar
floor cannot create it. Consumption advances that reserved generation first,
then persists consumed redaction, retaining the key only in the original
volatile binding. A spent floor redacts a still-accepted root before returning
successor-required. The signing purpose accepts one genuine original REGISTER
receipt and retains identical public record bytes for exact retry; it never
accepts an arbitrary digest or acknowledgement. These mechanisms have native
and genuine both-Carrier registration tests; public preparation commands do not
drive the complete qualified Publisher/Descriptor ACK lifetime.

The same binding persists its exact signed public record through its original
durable reservation before returning that public proof. The owner requires the
already advanced floor and exact Credential digest, synchronizes the staging
link, record/staging directory, renamed generation and current pointer/root,
and preserves original record/pointer inodes on exact retry. Ambiguous effects
deny further receipts until reopen. Only an exact floor-bound empty or fully
verified staging generation can retire as unavailable residue; malformed or
foreign inventory refuses. Binding close retires the pointer before the record,
with both directory barriers, while the original reservation retains the lease.
These public persistence facts grant no Descriptor Store ACK or readiness.

Reachability now constructs the canonical public Descriptor transcript; the
Instance's purpose-specific producer supplies the signature using its retained
key and exact real registration receipt. Independent volatile X25519 recipients
require consumed authority and strictly increasing registration revisions, with
at most two retained owners. Exact Descriptor retries retain the same key,
validity and signed bytes. Registration currentness is rechecked outside the
private-material lock after key creation and before final signed-proof return.
This signing mechanism exposes neither private bytes nor a generic signer and
does not select an accepting pair or attest a receiving Store acknowledgement.

The Instance recipient additionally opens only the fixed v3 capsule through the
selected standard HPKE suite and a retained original Registration receipt.
Header slot/revision and original expiry, the exact profile, Network, Target and
Publication digest remain bound; no legacy bytes are accepted. Its opaque
opening handle supplies bounded candidate facts and the plaintext digest while
the original caller/recipient/registration/expiry remain usable. It retains the
private recipient through downstream checks; Close seals acquisition, joins
users outside the root lock, erases private material and only then returns the
two-recipient capacity. Root/binding close joins those same users before releasing
the original durable reservation. This operation is not downstream acceptance,
replay commit or an authenticated Connection. Rate/replay admission and actual
incoming delivery remain with their separate lifecycle and composition owners.

The bounded new Connection recipient binding derives the immutable logical
context from independently verified Publication fields and the exact candidate
tuple. Its Attachment context binds the complete canonical plaintext digest;
a supplied mismatching digest refuses. Work Safety fields cannot extend the
original local/Network terminal bound or Credential validity, and incompatible
fields refuse instead of silently shortening them. Initial handoff retains the
original qualified operation and private-opening cancellation. Publication
resolves the claimed Rendezvous through its original Source observation,
excluding issuer, resolution and Introduction identities, keys and known
families, then rechecks that same retained duty before handing out the binding.
This establishes neither accepted replay nor real Responder readiness, incoming
RESULT/CLOSE, Instance TLS or authenticated recovery.

The new Instance owner additionally retains one immutable logical Service lease
under its original Publication lifetime. First Attachment setup returns that
lease together with its purpose-specific TLS borrow. Initial admission matches the exact original JOIN intent,
Work Safety end and setup deadline before a fresh JOIN observation and Instance
admission; these immutable comparisons need no second pre-admission observation.
The TLS borrow's identity check freshly observes its original logical lease and
matches the supplied logical/Attachment tuple, so initial authentication uses that
one complete check for the two handles constructed together. Initial authentication
buffers only the exact fixed two-record
flight, with the wire owner's byte bound: it cannot read into following
Application data. Canonical parsing and the separate original-owner checks still
surround each record and precede proof signing and accepted handoff. Physical retirement retains
the logical lease, original Work Safety context and prior physical cleanup
failure, including an interval without any Attachment. It keeps the actual JOIN while
TLS cancellation interrupts its borrowing transport, joins TLS users before
physical JOIN release, and erases exporter material before returning the borrow.
Publication retirement joins these borrowers before Instance key erasure and
retains their cleanup failures. Key use and transport effects require
the original qualified Publisher Operation whose context created that
Publication lifetime. Instance checks that operation synchronously before and
after certificate signing, CertificateVerify and physical TLS I/O, and again at
Attachment handoff; cancellation propagation alone is not its authority check.
Copied Operation handles cannot cancel or complete the original cleanup borrow.
The ordinary Reader launch gives Text sole ownership of its initialized
protocol reader while awaiting one stream. Setup-only launches retain their
separate EOF observer. Worker output before handoff refuses without opening a
Service. The original Reader operation checks each protocol effect and final
attachment retirement. Its bounded output stays private in a one-use result
until the same Operation, Service I/O and original worker descendants have
joined and the original Job's completed-current observation still holds.
Cancellation or a replacement Job cannot authorize presentation of those bytes.
Text prepares its private pending result after local authenticated completion,
retaining the original Connection's bounded control tail and protocol reader. The
composition joins the original Source before closing that operation. Successful
operation retirement releases and joins the Text reader before retiring the
Job; worker descendant join and completed-current checking precede one-use byte
presentation. The same ordering applies after Attachment replacement; it does
not create a new Text request or operation.

Publisher Text completion likewise retains the original Connection borrow until
its joined notification or original cancellation. Sending Application Done
does not fully close a Stream with pending authenticated Terminal controls.
The same qualified worker cleanup remains retained; refusal and cancellation
interrupt the wait and close/join the original Stream without new authority.
Its full Close result remains with that original Stream; the worker's cleanup
accounting consumes the same joined Stream's separate physical receipt. A
logical tail refusal is not a physical cleanup failure, while all actual late
write/close failures remain retained across every Service borrower.

Route Acquisition and Joined, and the purpose-scoped Instance Attachment, expose
separate read-only cleanup results only after their original physical return
and completion notification. Their full Close results still retain failed
exchange/EOF and actual physical errors. Connection passes actual joined
write/close failures to the original operation's cleanup accounting; a transport
loss alone does not retire that Job or grant replacement success. No nil receipt
supplies accepting authority, and a real physical failure remains terminal for
local admission.
Publication's original Registration and Introduction terminal likewise expose
joined physical results separately from their full work/withdrawal outcome.
Publisher joins its Connection/private users, returns the Source/Responder
borrows with those physical results, then joins the original Prefixes and
Instance binding. Command composition consumes the original Publisher receipt
and retains actual custody/selection/Hosting close failures separately; it must
not project the full canceled Publication result into Execution cleanup.
An idle stream-acquisition stop is identified only by its original Publisher,
before any delivery is acquired. The command's serving loop may finish on that
exact result; ordinary cancellation, a foreign owner or an aggregate containing
another failure does not supply it. Owning Withdraw/Close still joins all work
and retains its complete withdrawal and physical errors. Caller loss and a
failure after delivery acquisition remain failures of that operation.
This execution seam requires a genuine Connection/Route consumer and installed
worker evidence; portable compilation and Text fixtures do not provide either.
Before TLS setup, Reader requires the exact Source JOIN and Publisher requires
the exact Source-bound Responder JOIN. Each compares the capsule's pair-secret
digest, handshake context, Work Safety end and setup deadline with immutable
facts retained by the actual admitted child. This is a Route provenance check;
it does not replace Instance authentication or accepting Publication checks.
Its private certificate signer accepts only
the TLS1.3 server CertificateVerify purpose; InstanceProof consumes a one-use
exact-TLS request receipt after checked initial Continuity. The coalesced
response is one actual TLS write of that proof and zero-offset Continuity;
only a complete write with final owner observations releases the setup bound
into the original Work Safety lifetime. This Attachment mechanism does not
establish the complete Reader/Publisher consumer or authenticated replacement
and terminal-control operations using the logical lease. Those operation
lifetimes remain Connection obligations; a completed capsule handoff cannot
replace them.
Both authenticated role owners keep a logical cancellation context separate
from their physical TLS Attachment. Reader's original Work Safety deadline and
Publisher's original Instance lease remain the logical bounds. Final owner
retirement synchronously cancels logical authority before physical joins;
retiring Reader's physical context alone cannot renew or cancel that original
logical deadline. Local byte effects check logical authority, while actual TLS
I/O additionally checks the exact Attachment. Transport failure still fails the
current stream until genuine authenticated replacement is implemented; context
separation grants neither replacement authority nor an accepting fallback.
The Publisher replacement preparation uses that same original logical lease,
immutable tuple and continuity secret with a fresh genuine Responder JOIN and
TLS exporter. Instance retains only the original-key commitment and separates
the actual response write from setup transfer. Connection validates peer byte
ranges without mutating ledgers, retains its finite proposal ticket through
rejected physical join, and treats Continuity integrity violations as terminal.
It reserves that ticket before acquiring the fresh Responder JOIN, matching
all immutable private-request fields and its exact JOIN configuration before
admission. The acquisition and established JOIN retain the original logical
caller; the proposal context only interrupts setup. Failed JOIN also consumes
the proposal, whose retirement waits actual acquisition and setup callback join.
An observed invalid wire prefix, body length or complete record is likewise
terminal; its typed grammar error is distinct from physical EOF, an incomplete
frame or another original I/O failure. This distinction permits no inference
about the cause of transport loss and supplies no recovery authorization.
Publisher's concrete recovery Route retains an original Source scope and an opaque
Responder scope before record actors start. The scope keeps the original caller,
selected role-3 leg, authentic observer and absolute bound; a retired physical
Prefix does not itself revoke that logical scope. Each JOIN acquires one exact
physical generation, which cannot follow a later replacement. Connection reserves
its finite proposal before that acquisition obtains fresh genuine Hosting and
Stock resources. The old generation joins before another opens; actual cleanup
failure remains retained and denies a new opening. A generation's quiet claim
seals that generation without stopping every logical Connection in Publication.
No failed TLS transport is reused. Physical loss seals those actors and
does not itself revoke the original accepting Publication. Responder opening
acquires one exact Source generation, and physical JOIN transfer checks that
generation plus both original logical scopes. Qualified command funding obtains
only an existing current-source handle for issuer exchanges; it cannot open a
replacement recursively while dependent presentation is in flight. Descriptor
publication likewise retains one exact acquisition through its final Store-ACK
commit. Fresh Instance/Source observations run before Publisher state locks;
local handoffs recheck the original Instance lifetime without native I/O.
Physical loss
starts the original finite episode without waiting for a proposal. Replacement
preparation freezes and joins both original actors before advertising retained
offsets. Its final commit independently reobserves native, Route and logical
authority before a local Source/Responder transition; Instance rechecks the
original private recipient and cutoff at that transition without native I/O.
All three ledgers advance together, retaining counters and queued bytes; the
new writer immediately drains the retained suffix. Joined setup callbacks no
longer interrupt accepted transfer, but the original setup caller still must
be live at commit. This mechanism requires genuine replacement capsule dispatch
and Reader recovery before ordinary Application admission; it establishes no
complete recovered stream on its own.
The new ordered receive ledger retains accepted unread bytes and recent replay
comparison bytes in one 256KiB buffer. A matching replay has no second local
read effect; contradictory overlap, a forward gap or an unverifiable discarded
prefix refuses before fresh-byte admission. Full unread capacity applies
backpressure without evicting bytes. Authenticated replacement retains both
the received and consumed cursors and the original directional byte limit.
Each Attachment reader additionally owns 512 bytes of private framing scratch,
with no background reads. Buffered records remain unaccepted until each one's
original-owner commit check; the receive/replay ledger bound is unchanged.
Partial successor bytes still forbid the empty-TLS-tail exception, and an
original read failure cannot be hidden by enough buffered bytes. The reader
clears its scratch before reporting completion; it never follows a replacement.
This mechanism still requires the real stream consumer and its authority,
Terminal-control and physical-join ordering.
New Reader construction additionally retains a fresh local ConnectionContext
of the original qualified ordinary Reader operation; a setup-only invocation
cannot acquire that authority. Only its salted v3
commitment enters the capsule; local context and salt do not enter network
fields. One initial construction consumes that operation's context claim
before lookup or physical Source ownership transfer. The Source must have been
opened by that same original operation context, not a descendant setup context
or another operation with matching limits. A refused second construction does
not close the Source retained by the first construction.
The Reader requires exact Instance TLS, signed challenge response and the
sealed zero-offset Publisher Continuity before transferring setup into Work
Safety. Both role authentications provide one-use initial state to the logical
send/receive/Terminal owner with an immutable checked directional byte contract.
The initial stream handoff observes original Instance/Route authority before
locking consumer state. It then uses the exact JOIN's Source-before-Responder
and Acquisition barriers, followed by the Connection state mutex and the
Instance-local lease gate for Publisher. `ServiceAttachment.CheckStreamCommit`
requires the same authenticated generation-one Attachment, retained logical
lease, original live Execution operation and Publication reservation; its local
reservation check performs no filesystem observation. Reader compares its
original sealed binding and Source owner. Both roles recheck the immutable
byte contract before consuming initial state and starting record actors. These
local gates follow fresh full observations; they cannot replace those
observations, authenticate another Attachment or grant a new generation.
The logical Stream applies the same ordering to byte admission, record
acceptance/completion, Terminal settlement and recovery preparation: full
original-owner observations outside consumer state locks, then bounded local
effects under the retained Source/Responder and Stream barriers.
Reader Attachment checks match its exact JOIN's Source scope, original caller,
Network/profile and absolute bounds before and after one full physical
observation. That generation retains the scope's same leg and authentic
observer; no detached match or local seal supplies accepting authority.
Instance's
`ServiceLease.CheckCommitIdentity` compares the same live logical lease and
Publication reservation without native I/O, including a physical Attachment
gap. Original Credential/Work Safety bounds and Execution operation remain
requirements at the final gate. Record completion also compares its exact
actors/generation after I/O; a stale completion cannot update the replacement.
Source and Responder also retain the State-produced original Network witness.
Their local commit checks compare its exact profile and synchronous retirement
projection, so a delayed successful observation cannot deliver bytes after
that State has retired. This refusal-only check supplements full fresh Network
observations and performs no foreign I/O under Route or Connection state locks.
Physical replacement retains that same witness; it cannot adopt a new State.
The durable reservation gate compares an immutable local original-pointer and
retained-failure projection without acquiring its storage mutex. Reservation
retirement seals only that original pointer; root retirement seals all local
commits before waiting for native effects and retained reservations to join.
This projection supplies no floor, filesystem proof or accepting authority;
the preceding full original-root observation remains required.
The logical lease's full identity observation retains an original Binding
observer before leaving Instance state. It reobserves native Instance custody
and the independently owned durable reservation without that state mutex,
then compares the same live logical lease locally. Binding retirement denies
new observers before waiting, and joins them before erasing private material
or returning the original reservation. Publication lifetime creation and public
Credential access use the same observation/commit split; neither a detached
Credential nor a local gate replaces current native custody.
Physical Service entry and post-I/O recheck use that full logical observation
outside Instance state, followed by the local exact Attachment/recipient gate.
TLS CertificateVerify signing additionally retains its actual Service user;
private signing and its final local admission comparison share Instance state,
while native custody and reservation checks precede them outside that state.
First Instance consumption retains one original native observer across the
separately owned floor burn and its own redacted state write. Concurrent retries
join that same operation/result; they cannot select a second writer. Instance
state is released during the durable owner's write. Revocation before the private
write refuses that effect without refunding a completed floor burn; retirement
joins the original operation before reservation return. A copied/foreign Binding
cannot cancel its lifetime, erase its key or return its original reservation.
Original Binding acquisition likewise reserves one local opening before leaving
Instance state for durable reconciliation. Root retirement denies late transfer
and waits for that opening's rejected reservation cleanup before returning native
custody. The returned reservation must still match the unchanged accepted public
Credential; a completed foreign read cannot bind a changed or retired Instance.
Public record persistence retains an original Binding observer across its
already separate durable write and fresh final observations. Recipient creation,
Descriptor signing and capsule opening likewise observe original Binding custody
outside Instance state before their local effect gates. Opening.Request performs
that full observation before locking Opening state, then rechecks its own caller,
expiry and private recipient locally; canceled/closed openings cannot expose a
late candidate while native observation is pending. No local gate supplies a
registration receipt, Descriptor ACK or accepting Publication.
Backpressure waits retain a notification captured under the Stream mutex but
hold no Route or consumer state lock while waiting. Cancellation releases them
without another byte/control effect. Final replacement checks retain the same
local Instance reservation gate in addition to the exact private recipient.
The Publisher factory consumes the actual completed delivery through its same
Source-bound Responder acquisition. Qualified local stream delivery uses the
original snapshot invocation's sole Text reader; no new worker/Grant replaces
it. Command composition, authenticated replacement and genuine native two-role
acceptance remain necessary; these owner APIs establish neither that full
journey nor permission for a weaker ordinary accepting path.
Reader's initial factory owns one actual Source acquisition from lookup through
Introduction submission, JOIN and Instance authentication. It re-verifies
signed Descriptor bytes against the independently parsed canonical Target Link
and exact original Network/profile. The same original physical Source checks
scope and exact Introduction duty; its retained Rendezvous choice cannot be
redrawn or outlived by supplied Work Safety bounds. Failure seals setup and
joins rejected TLS/JOIN before physical Source retirement. Context-private
lookup history and shared selection remain caller-owned; Source Close never
clears them. Reader TLS requires this original Source owner. This initial
factory does not supply authenticated Attachment replacement or complete
ordinary Application admission.
Logical lease Close seals acquisition and cancels before joining its current
TLS borrowers, then returns the retained Instance borrow. Publication joins
those logical leases even when their last physical Attachment already retired;
Instance erasure follows that join. Copied logical or physical handles cannot
cancel or retire the original. No lease grants a new Grant, Instance, recovery
attempt or Application stream.
After complete authenticated Terminal receipt and confirmation, a settled
control tail may retire on TLS EOF/truncation only with the exact JOIN's opaque
evidence of an already decoded clean Route CLOSE(0). The canonical dedicated
child decoder produces that evidence; zero, copied and foreign child/stream
handles cannot manufacture it. Local Close, directional EOF, peer refusal and
raw physical EOF supply no evidence. No additional Service plaintext may have
entered the failing record read, and the retained logical authority must remain
current. Partial records and joined authority/integrity errors still fail.
After Attachment replacement, the original Application settlement alone cannot
permit this retirement: all Terminal receipts and confirmations must be complete
on the current generation before its decoded clean CLOSE can end the tail.
Retirement emits no additional Application EOF or bytes and still joins the
Attachment/writer, retaining every actual cleanup failure. This bounded
retirement rule supplies neither an authenticated replacement nor a complete
ordinary recovered Service path.

The logical stream retains one physical record owner with separate completion
signals for its exact reader and serialized writer. Both use the same immutable
TLS/Instance Attachment snapshot; record parsing and Terminal-tail witnesses
cannot follow a later owner pointer. Retirement joins both original actors,
and a foreign or stale actor cannot update current byte/control ledgers.
Each actor publishes its own retained result before its completion signal;
joining reads those results only after both original signals. Recovery keeps
the retired attempt's work result separate from the replacement's logical
outcome and from the lower owners' physical cleanup receipts. Final retirement
retains the current actors' raw work results separately from the first logical
failure and all actual physical cleanup failures. Socket errors caused by
intentional interruption do not invent another cleanup or Application failure.
Nested framing deadline updates likewise retain a lower refusal as work, while
the exact lower owner retains any actual physical deadline failure. An upper
writer waiting for a lower refusal cannot invent another physical effect merely
because it also tries to shorten that retired lower lane's deadline.
JOIN relay likewise separates its original deadline refusal from the lower
framing owner's physical failure; a local nested refusal cannot supply another
failed cleanup effect to Execution.
An unpublished JOIN setup also retains its complete refusal separately from
the original joined physical setup receipt. That refusal cannot itself poison
Prefix cleanup; real callback/socket/lower-lane failures still retain their
original owners and deny subsequent dependent effects.
Framing deadline effects retain one exact selected frame and run outside its
state mutex. A separate physical deadline turn serializes lower calls; if local
interruption shortens the bound during I/O, that winning bound is applied before
the turn returns. Old-frame effects cannot update a replacement writer. Lane
and parent retirement join original deadline users as well as record writers
before publishing physical cleanup or returning their retained resources.
The independent deadline borrow remains mandatory after its original writer
has returned; a late deadline failure belongs to that same joined result.
Each Data and Terminal write ticket also retains its original ledger owner and its own
single-use identity. Copied or foreign tickets refuse before old-generation
completion is classified. An actual original writer may complete after
replacement only to return its retained physical result; it cannot settle the
replacement's control obligations.
These physical lifetimes still require the coordinated recovery controller,
authenticated generation commit and immediate replay before ordinary handoff.

The first acknowledged replacement also synchronously shortens the original
recipient's local accepting window. New openings and retained candidate reads
check that exact bound independently of the physical retirement scheduler.
Repeated restrictions can only shorten it; the original signed Descriptor and
registration expiry stay unchanged. Genuine both-Carrier registration controls
keep those original facts valid while checking refusal after the local cutoff.
This component evidence does not prove the elapsed sixty-second overlap run.

The new `publication/runtime` owner has a genuine non-test consumer in the
qualified snapshot holder console. Its optional `holder.publication` names
independent Instance and Publication roots; Domain4 `holder.route` additionally
names separate Source and Responder Interior roots (`source_interior_root` and
`responder_interior_root`). All declared roots must remain independent. The consumer opens one installation
Entry selection and shared Hosting budget, borrows independent Source and
Introduction selections, and opens both actual prefixes under the original
operation. It prepares a distinct role-3 selection and finite reservation
without dialing. The actual Responder opens only through that original Source
after private capsule binding and nonce commitment; original caller cancellation
interrupts opening and is joined before transfer to the Publisher lifetime.
At that same accepted nonce boundary, command composition prepares only the
incoming Rendezvous Forward Stock through an exact existing Source handle,
concurrently with the already funded Responder opening. `PrepareJoin` cannot
resolve another Source, open a JOIN or extend the caller. A sibling failure
cancels the common preparation, and both original operations join before the
delivery can return or retain success. Actual JOIN still obtains fresh recipient
authority and durably presents its own token after Introduction completes.
Unused preparation and failed opening retain their once-owned reservation until
cleanup. Private initial handoff retains the exact delivery nonce, capsule-bound
deadline, Connection binding and original opening. Successful RESULT is attempted
only after those checks and actual Responder preparation. The private preparation
returns no accepting state: delivery retention owns the single complete fresh
Binding/Rendezvous observation after acquisition and before its local commit
and successful RESULT. Binding resolves its retained public Rendezvous first,
then fully observes the accepting pair/private Candidate and checks the original
operation before returning. This read-only observation has no intermediate
private effect or accepting handoff, and retains no cached authority.
Matching child CLOSE
and final original observations precede retained handoff. This implementation
has an isolated qualified snapshot-console incoming scenario on both Carriers:
an independent actual Source submits a sealed capsule using the receiving
Store's independently verified proof; the actual Publisher prepares its
Source-bound Responder and completes successful RESULT/matching CLOSE.
Immediate identical replay receives the exact typed refusal with physical join.
Two simultaneous submissions use separate actual role channels and finite
tokens; exactly one succeeds and one receives the same joined refusal.
A freshly sealed valid capsule with the same delivery nonce also refuses after
the original eight-second capsule deadline. Removing only accepted-history
commitment makes that later replay accept while immediate replay still refuses;
the control distinguishes retained acceptance from a pending reservation.
A separate native case holds actual receiving CLOSE after the genuine Publisher
RESULT on an acknowledged predecessor, until the original capsule expires.
The original Registration retains its emitted-successful-RESULT failure through
owning withdrawal and repeated Close. Fresh valid bytes carrying the same nonce
refuse through the independently acknowledged current recipient; a new nonce
succeeds there under the same original Publisher and Administration context.
Two separately admitted Sources retain their own finite User allocations and
original bootstrap bounds. Failed Source results remain failures through physical
join; no spent Stock, caller or Publisher operation is renewed or refunded.
These observations establish no JOIN, Service TLS or Application bytes, full
original-registration-expiry+60s retention. The separate worker-loss case first
completes genuine delivery, then physically joins the original Publisher and
worker and observes retained accepted history under the exact context. A failed
context terminal report also retains that history; revocation grants no erasure.
After original users join, this native oracle compares retained replay expiry
with the independently verified original Store Descriptor expiry+60s. The
private accounting oracle separately checks refusal immediately before that
horizon and release at it; no capsule or replacement deadline supplies the bound.
`publication-open` retains custody but exposes no accepting Link;
`publication-publish`, `publication-refresh`, `publication-link` and
`publication-withdraw` consume that same live owner. A completed preparation
or setup-only worker cannot select this path.

The owner retains a single opaque Instance publication lifetime. Registration,
canonical signing and the exact Source Store exchange precede current-pair
selection; final original operation, caller, binding, receipt and Descriptor
checks follow actual synced ACK and physical exchange join. Pending proofs
cannot supply a Link. The real-clock scheduler uses REGISTER creation+300s,
preserves old current while a replacement awaits ACK, and caps a joined
predecessor at first switch+60s/original expiry. Acquisition stops before
pending cancellation, owning withdrawal and physical joins; the Instance key
and durable reservation remain retained until that original lifetime returns.
Publication seals each handed-out original Registration before canceling its
owned flight contexts. Registration handoff shares that stop barrier; a late
REGISTER owner is sealed and retained for physical join. This ordering does
not discharge a canceled original caller or erase a prior physical failure.
The Responder's physical caller retains the original Publisher caller and
absolute deadline through bounded physical Close. Local Publication stop
interrupts private users without canceling that physical caller before join;
original caller, Execution or Instance loss still interrupts both lifetimes.
Physical cancellation follows prefix join on an ordinary local withdrawal.
After admission and registration users join, Publication detaches and joins
its Instance-loss callback before intentionally retiring its own Instance
lifetime. That retirement joins Service TLS borrowers before returning Prefix
claims; it must not manufacture an external authority loss while gracefully
closing those Prefixes. An already-started external-loss callback still joins
its original interruption, and original caller/Execution loss remains active.
The qualified operation exposes an opaque identity for its original admitted
Administration Session. Publication matches that identity to the exact live
operation at effects and handoffs. Session revocation and joined retirement
are separate from worker/Job retirement; a joined Job cannot complete or replace
its still-retained context. This identifies the lifetime required by context
history, without implementing replay acceptance or authenticating a Connection.
The selected native execution-route profile exercises the production snapshot
console through genuine REGISTER and synced receiving Store ACK on both
Carriers. It independently checks the committed Descriptor, unchanged exact
retry, fresh replacement slot/key, third-recipient refusal, withdrawal, physical
worker/receiver join, retained Store reopen and consumed Instance restart
refusal. Authority approval and offline blind issuance are explicit fixture
inputs; neither supplies the qualified operation or ACK. A separate genuine
snapshot operation invokes the same production holder composition while a
receiving authority observation waits after the actual public record changes.
Initial Link refuses until the real Store exchange completes; during replacement
the prior Link remains available. Independent signed-record verification and
matching Store bytes follow the actual ACK and physical exchange join on both
Carriers. This Link evidence establishes no private capsule selection.
The separate delayed-ACK withdrawal case closes the original current and pending
registration owners before releasing the receiving observation. Link refuses
before and after release, while the actual Store retains the signed replacement
record. Registration cancellation seals its stop barrier before interrupting
the idle reader; previously observed failures and physical Close or WITHDRAW
errors remain retained. This is withdrawal coverage, not worker or Network loss.
This isolated Ubuntu24/systemd255 component does not establish accepting
Installation startup, the full replay-history/loss matrix or successful Service
Connection. The separate unchanged-clock `publication-timing` profile checks
actual creation+300s automatic refresh after a twelve-second initial ACK delay,
pending refusal and old-pair acceptance, genuine replacement ACK, exact retry,
third-recipient refusal and delivery before/after the bounded switch+60s cutoff
on both Carriers. Independent call/commit brackets bound creation and switching;
no substituted clock or forced Refresh supplies the timing result. Its
[profile owner](../../tests/qualification/publication-timing/README.md) retains
the selected native actors, fifteen-minute test bound and finite permissions.
Separate original-worker-loss and signed-profile-conflict cases exercise held
Store responses on both Carriers. Worker loss retains its complete use failure
separately from the original joined cleanup receipt. Signed conflict leaves the
caller live and denies accepting Link before and after the exchange joins;
the Store keeps its actual signed record. Full repeated Close preserves failed
Network and native peer outcomes. The signed-conflict fixture's receiving side
independently verifies the same signed Epoch/profile in a separate State root
and stays current after Publisher conflict and original exchange join. Replacing
it with shared State causally fails the currentness observation on both Carriers.
The held-record oracle still does not prove successful late ACK decoding at
Source after Publisher-only authority loss. Signing and clock infrastructure
remain under one fixture controller.
The separate post-decode case pauses the genuine Source currentness observation
after a matching successful Store RESULT, then commits Publisher-only signed
conflict. Failed final processing retains the original acknowledged-failure
cause, denies Link and joins physical work while receiving State stays current.
This verifies late ACK processing; decoding occurs before authority loss and
does not establish successful ACK arrival/decoding after that loss.
Retained TLS peer-read provenance changes diagnostic categorization only;
raw/local/mixed failures cannot acquire it or become successful completion.
A shorter explicit refresh cannot
prove the scheduling and overlap timing boundary.

The isolated new `publication/durable` owner prepares and reopens its own v3
root through `ardents-next publication prepare-root <root> <target> <network>`.
It holds an exclusive native lease, checks finite exact layout and canonical
uint64 floor, and verifies retained signed records against the selected Target
and Network at their signed NotBefore. Reopen resynchronizes retained floor,
record and pointer barriers; an exact unavailable orphan is retired without
lowering the floor. Foreign residue, lost floors and inconsistent records refuse.
The command returns `publication-root-prepared` and a detached floor observation;
it grants no Instance signing authority or accepting readiness. Predecessor roots
remain independent. Native directory-flush refusal remains failure.

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

New Execution has a selected-snapshot launch seam consumed by
`execution route-holder` with its explicit `snapshot` input. Stable local file
import belongs to Text; the original qualified Administration invocation owns
INIT/READY, one exact operation and joined cleanup. Its sole post-READY reader
uses Text's bounded protocol instead of the setup consumer's byte/EOF observer.
No Connection stream is fabricated. The installed lifecycle profile includes
this snapshot operation, and must run on the selected actual native stand before
it can establish installed execution evidence. This seam alone supplies no
Credential, registration, Descriptor Store acknowledgement or accepting Link.

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
[ADR-0088](https://github.com/dianabuilds/ardents-network/blob/c58268d72f1eaeae173144e76d32d50986de2a77/docs/archive/adr/0088-retire-alpha-service-links-and-corpus-intake.md),
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
[ADR-0091](https://github.com/dianabuilds/ardents-network/blob/c58268d72f1eaeae173144e76d32d50986de2a77/docs/archive/adr/0091-retire-uncomposed-legacy-artifacts.md) retired the
in-tree Firefox compatibility source; it survives only in Git history in
accordance with [ADR-0061](https://github.com/dianabuilds/ardents-network/blob/c58268d72f1eaeae173144e76d32d50986de2a77/docs/archive/adr/0061-retain-firefox-entry-as-compatibility-evidence.md)
and [ADR-0069](../adr/0069-retire-active-browser-implementation.md).

## Verification and related decisions

- Go tests for Broker, Endpoint, Publication, and Service Connection exercise
  the Module Interfaces and failure paths.
- Protected text AAI3 Connection and Administration behavior tests exercise
  framing, typed refusal/outcome, cancellation, join, and exact socket cleanup
  through their public Interfaces. AAI3 rejects the removed AAI2 request before
  Application owner I/O. Architecture tests forbid a second Endpoint-local
  transport owner and enforce the command dependency graphs.
- [ADR-0024](https://github.com/dianabuilds/ardents-network/blob/c58268d72f1eaeae173144e76d32d50986de2a77/docs/archive/adr/0024-native-interactive-route-foundation.md) selects the
  native Route foundation; [ADR-0075](https://github.com/dianabuilds/ardents-network/blob/c58268d72f1eaeae173144e76d32d50986de2a77/docs/archive/adr/0075-service-connection-v2-terminal-receipt.md)
  preserves the surviving Terminal receipt, confirmation and control-tail
  clauses; byte identities are selected by
  [ADR-0117](../adr/0117-reset-closed-service-formats-to-v3.md).
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

Installation's `request` Module owns canonical local declarations and independent
original root-owned request-file reobservation. Its detached `Document` data and
opaque `Origin` do not authorize Release, filesystem transactions, generation
selection or startup. The existing Installation request facade remains the
command/transaction consumer; original Root effect admission checks that same
provenance. General trusted-path observations for lease, staging and inspection
remain distinct from request-file provenance.


The new installed participant lifetime is owned by `internal/endpoint`.
Its `Run` opens and retains the original Installation Startup, decodes only its
bound declarations, prepares the local Execution provenance, refreshes genuine
State and performs exact permission-file handover before opening shared funding
parents and IPC. CLI `endpoint start-installed` supplies only the original root,
public progress rendering and process-exit projection. Progress supplies no
readiness or authority. IPC and Publication borrowers join before shared parents,
Stock, State, local Execution and Startup close; partial construction follows the
same reverse ownership order and retains original failures. Permission and
history bytes, role declarations and qualified-worker requirements are unchanged.
