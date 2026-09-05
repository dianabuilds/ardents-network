---
id: R-144
title: How can an enrolled Headless Endpoint receive authenticated bootstrap and Source authorization without manual plans?
status: decided; governed by ADR-0071
owner: Product Owner and Codex
started: 2026-09-05
---

# R-144 — Headless bootstrap delivery

## Decision this unlocks

Determine whether C0-05 can give an enrolled Headless Endpoint one supported
operator path to current authenticated State, Entry, and the locally authorized
Direct-Origin Source inputs required to obtain them. The result must select a
bounded delivery contract or retain the issue blocked. It does not authorize
implementation, a public bootstrap service, new network/Route wire, a new
Authority, a secret in an ordinary artifact, or automatic Source issuance.

## Current contract

The C0 scope requires a technical operator to publish one Service Instance and
let a second Endpoint open an explicit Target Link without manually editing
JSON, extracting fixture keys, knowing package names, or guessing command
order. C0-05 makes the same bounded Target-Link-to-Service-Connection result
its acceptance criterion.

`cmd/ardents` currently decodes `ardents-headless-runtime-v1` and its optional
`ardents-source-plan-v1` from local operator files. The former names State,
Entry, local-role, time-observation, socket, and Application-principal inputs.
The latter names public State/Source facts and local client-certificate/key and
root paths. Enrollment-v3 verifies the exact program/control inventory, but
does not provide State/Entry bootstrap or a per-Endpoint Source client
authorization. `network.ac1` is alpha-control evidence, not a Source runtime
configuration.

ADR-0053 keeps the Network State root in separate local custody. ADR-0062
keeps purpose Transit Grant signing separate from that root. R-128 permits an
Endpoint-owned acquisition lifecycle but prohibits an operator Route plan,
hidden selector, or independent authority. R-143 keeps Source certificate
replacement local and controlled; it selects neither issuer nor automatic
credential delivery.

## Hypotheses

- **H1:** One exact per-Endpoint sealed enrollment companion can deliver only
  bounded, recipient-bound bootstrap and Source authorization after artifact
  verification, without selecting a Route or creating a standing service.
- **H2:** One authenticated, deliberately limited bootstrap responder can
  distribute only already-authorized inputs and preserve explicit unavailable
  behavior under partition without becoming a State, Route, or Service
  Authority.
- **H3:** Public enrollment bootstrap plus a separate finite Source-client
  enrollment operation can preserve separation of public facts and local
  authorization.
- **H4:** One fixed, enrollment-pinned Headless Entry-Set companion can carry
  only bounded signed State-referenced Entry Invites. It reuses the separately
  verified existing `network.ac1` Network Evidence for Network ID, authority
  set, threshold, Epoch, inputs, and materializations; it does not duplicate
  or select a State authority. It creates no Source client authorization and
  therefore permits only an explicitly expiry-bounded, non-refreshing C0
  start.
- **H0:** None can meet the selected C0 operator, trust, and maintenance
  limits; C0-05 must retain its current blocked implementation gap.

## Evaluation criteria

- The exact user outcome is a second enrolled Endpoint opening one explicit
  Target Link to one active published Instance and exchanging bounded opaque
  bytes through existing local Application interfaces.
- Protected material includes Source client private keys, Source authorization,
  State/Entry bootstrap facts before authentication, Endpoint-local protected
  roots, and any Target linkage. Adversaries include a substituted bundle or
  companion, malicious/bootstrap infrastructure, copied Endpoint state,
  expired or replayed material, and a colluding Source.
- Bootstrap must not sign State, select an Entry/Initiator/Route candidate,
  receive a Target, issue a Service Credential, or provide an alternate
  discovery/fallback path.
- A substituted, stale, malformed, expired, revoked, unauthorized, or missing
  result is a bounded visible unavailable outcome before Route work. DNS,
  direct, cache, same-looking name, and alternate Target fallback remain
  prohibited.
- Any Source client authorization is recipient-bound, finite, durable against
  replay/rollback, and recoverable or terminal after interruption/loss. A
  copied artifact cannot create a second accepted Endpoint identity.
- The selected route preserves Source CA, hostname, client authorization, and
  leaf-key pins; it does not make an external issuer, private key in a public
  bundle, or a generic local configuration channel acceptable.
- Latency, bandwidth, and retained state are bounded before the existing
  State/Entry/credential deadlines. Partition, responder failure, or local
  ambiguity yields unavailable rather than a hidden operator procedure.
- The procedure remains operable by one Product Owner and Codex, introduces no
  unreviewed dependency/license/distribution channel, and can be exercised by
  the maintained command seams without fixture commands or manually authored
  JSON.

## Evidence plan

### Primary sources

- C0 scope, threat model, C0-05 issue, R-128, R-129, R-143, and the current
  Network Route/Node, Endpoint/Service, enrollment, and Source owners,
  accessed 2026-09-05.
- ADR-0053, ADR-0062, ADR-0065, and the current `cmd/ardents` headless/source
  loaders plus `internal/network/source` and `internal/network/state`,
  accessed 2026-09-05.
- Any evaluated sealed-delivery, enrollment, or mutual-TLS authorization
  mechanism's primary specification and security guidance, with access date,
  before selection.

### Experiment

Before a candidate is selected, use only external disposable diagnostics with
test keys to trace enrollment verification, bootstrap loading, Source TLS, and
the two C0-05 command seams. The experiment must prove that a substituted or
replayed input fails before State acceptance and Route work, and that no test
fixture command supplies Source credentials. Retain no private material in the
repository.

### Failure scenarios

- copied/replayed companion or Source client authorization starts another
  Endpoint;
- a malformed, expired, wrong-CA, wrong-pin, or substituted material reaches
  State acceptance, Source TLS, or Route work;
- bootstrap chooses a Route/Entry candidate, sees a Target, or becomes a
  generic configuration/discovery service;
- local durable rollback, interruption, or loss revives authorization;
- full expiry has no independently authenticated recovery and is hidden by a
  stale-good fallback;
- the procedure requires a password, Authority material, fixture key, or JSON
  editing by the operator.

## Findings

- **Current-code fact (2026-09-05):** `ardents-source-plan-v1` names local
  client certificate and private-key paths. `internal/network/source` rejects
  an absent or mismatched key pair, then its server requires both a
  CA-verified client certificate and an authorized client leaf-key pin. A
  public bundle, a copied key, or a later generic configuration file is not a
  valid Source authorization mechanism.
- **Current-code fact (2026-09-05):** `state.Accept` accepts a complete
  offline Epoch, inputs, and materialization through the same State validation
  used by Source acquisition. `entry.Import` separately validates the signed
  Invite against the accepted State view. Thus a bounded static delivery can
  retain State and Entry authority boundaries; the delivery byte is not itself
  State or Entry authority.
- **Current-code fact (2026-09-05):** the already enrollment-pinned
  `network.ac1` is a signed Network component. Its verified `NetworkEvidence`
  contains the Network ID, authority set, threshold, profile, Epoch, inputs,
  and materializations; the maintained alpha-control inspection verifies that
  evidence and accepts it through `state.Accept`. The current Endpoint does
  not consume that evidence, so it is not a runtime Source plan, but it is the
  existing selected authority-binding path H4 must reuse rather than duplicate.
- **Current-code fact (2026-09-05):** an Entry Invite is accepted only when
  its issuer, Network ID, Epoch, digest, profile, candidate domain, identity,
  family, record/domain-proof digests, assignment, signature, and time window
  agree with current authenticated State. The Entry owner has exactly two
  ordered active slots. A fixed companion can consequently contain at most one
  generation-one Invite per slot and cannot make a non-State Initiator or a
  complete Route valid.
- **Current-code fact (2026-09-05):** this validation is not recipient
  binding. The receiving Initiator's durable Admitter rejects a repeated
  attachment ID, but accepts the same valid Invite with a distinct attachment
  ID and client-key digest until its finite admission capacity is full. Its
  reopen behavior preserves that distinction. A copied shared Entry-Set
  companion can therefore create another accepted Entry use before expiry.
- **Current-code fact (2026-09-05):** enrollment verification returns pinned
  executable/control companions and Release inputs, while the enrolled
  Endpoint start only claims its portable local profile and Release floor. It
  creates no Endpoint recipient key, publishes no recipient public key, and
  has no enrollment-bound response channel. The only current HPKE recipient is
  a Service Instance key signed in a Service Credential for SealedIntroduction;
  it is not an Endpoint bootstrap identity and cannot be repurposed.
- **Current-code fact (2026-09-05):** ordinary extra v3 bundle files are
  mapped to Release metadata. The offline Release verifier rejects unreferenced
  metadata, so an unnamed bootstrap file cannot piggyback on the existing
  inventory. Conversely, v3 already derives and excludes exact Node and
  Custody companion names from Release inputs. A fixed Headless bootstrap
  companion could use the same bounded inventory pattern, but that changes the
  enrolled-headless artifact contract and requires an explicit decision before
  implementation.
- **Current-code fact (2026-09-05):** the maintained headless runtime has a
  deliberately static State mode when no Source plan is supplied, but still
  requires an accepted local State root, an imported Entry root, and a fresh
  local time-confidence file. The latter is currently an operator-owned
  regular-file seam, not an authenticated time-distribution decision. A
  bootstrap command may not silently claim that writing a marker authenticates
  time.
- **Current-code fact (2026-09-05):** H4 alone cannot satisfy the complete
  C0-05 operator journey. The supported Service host still supplies
  `ardents-service-instance-initialize-v1` and
  `ardents-headless-runtime-v1` JSON plans with roots, times, and local
  socket/principal facts. The existing Custody command correctly remains an
  interactive, independently request-hash-confirmed ceremony; it must not be
  replaced by a password-bearing wrapper. A selected C0 route therefore also
  needs one command-owned local-plan initializer for both User and Publisher
  roles, or C0-05 remains incomplete even if its State/Entry inputs are
  delivered safely.
- **Current-contract inference (2026-09-05):** H1 and H2 do not avoid the
  same missing primitive. A sealed per-Endpoint companion needs a recipient
  public key before it can be constructed; an online responder needs an
  authenticated requester identity before it can decide which bounded bytes it
  may return. The enrollment manifest/pin authenticates an artifact, not a
  fresh local Endpoint instance, while the current Source client certificate is
  downstream authorization and cannot bootstrap itself. Neither the Service
  Instance Introduction key nor a Source TLS key may be reused for that first
  binding. Both candidates therefore require one explicitly designed Endpoint
  recipient enrollment lifecycle before they differ in delivery topology.
- **Falsification:** H4 fails the predeclared copied-artifact criterion. Its
  shared Entry Invite is a valid capability for more than one distinct
  attachment/client-key tuple, so a copied companion can create another
  accepted Endpoint use. Reducing its expiry, putting it in a manifest-pinned
  file, or relying on the two local Entry slots does not make it
  recipient-bound. H4 is rejected.

Before the Product Owner decision below, no delivery option was selected:
H1--H3 required a recipient enrollment or online bootstrap authority that the
current contract did not define. H4 is rejected by its copied-capability
behavior, before its companion grammar or local initializer can be selected.

## Options

1. **Selected:** exact per-Endpoint sealed enrollment companion.
2. Authenticated bounded bootstrap responder.
3. Public bootstrap plus a separate finite Source-client enrollment operation.
4. **Rejected:** fixed enrollment-pinned Entry-Set companion plus existing
   verified Network Evidence. A copied Invite remains usable for a distinct
   attachment/client-key tuple.
5. Retain explicit local operator plans (known to fail C0-05 acceptance).

## Decision boundary for H1 and H2

Before comparing sealed delivery with a responder, define the shared missing
recipient-enrollment primitive: local key creation without private-key export;
an authenticated first binding of that public key to exactly one Endpoint
owner; finite, versioned authorization; durable replay/rollback floor; and
explicit loss, expiry, withdrawal, and replacement behavior. It must neither
grant Source access by itself nor become State, Route, Target, Custody, or
Release authority.

With that primitive specified, H1 can be selected only if its sealed companion
has a bounded issuer, recipient, bytes, validity, replay record, and delivery
event and can be recovered without silently making the artifact channel an
online registrar. H2 can be selected only if its responder authenticates that
same bound recipient, has no Route/Target input, returns no extra discovery
data, and produces `unavailable` rather than a hidden operator recovery path.
H3 still needs the same recipient lifecycle and additionally a separately
bounded Source-client authorization decision. Thus H2 is not a shortcut around
first binding, and H1 is not an artifact-only implementation detail.

The current C0 command-role contract makes their operational cost different.
The enrollment verifier is deliberately static: it neither writes local
Endpoint identity, downloads, nor grants authority. H1 consequently needs an
explicit pre-delivery enrollment/companion issuance event outside that verifier
and a bounded way to place the resulting exact bytes with the intended
Endpoint. H2 needs a new authenticated Node or bootstrap duty: the current
`ardents-node` role is limited to Source, Transit issuance, and already
selected Node duties, and its State profile declares no bootstrap responder.
Neither change can be smuggled into the existing four command roles. H1 has
the smaller standing online footprint but still requires a first-bind issuer;
H2 has a direct recovery channel but adds a continuously operated selected
duty. This comparison is not a selection.

## Recommendation

Do not select H4. The Product Owner selected H1 on 2026-09-05: an offline
recipient-enrollment issuer returns one exact sealed response after explicit
approval; it does not add a standing bootstrap responder. The selected time
trust path is a separate offline challenge-bound time witness. Its trust key is
purpose-separated from enrollment; the response is bound to a fresh Endpoint
challenge; acceptance accounts for delivery delay through one bounded local
monotonic request window; and a restart without that monotonic continuity
requires a fresh request and response. This is a consequential new trust and
operator boundary, so [ADR-0071](../../adr/0071-recipient-bound-offline-headless-enrollment.md)
was accepted by the Product Owner on 2026-09-05. No implementation may begin
until its required follow-on owner contracts are accepted.

The selected direction must still define recipient identity lifecycle, first
binding, replay/rollback/loss recovery, expiry/revocation, isolation from
State/Route/Target/Custody authority, and one-team operational burden before
implementation. Only after that governing work can C0-05 derive local
User/Publisher runtime facts and Service Instance request paths rather than
asking an operator to author the current JSON plans; it must retain the
separate interactive Custody request-hash confirmation. The strongest risk is
mistaking a file or connection bootstrap convenience for a harmless
implementation detail when it actually grants a new authority.

## Prepared decision proposal — recipient enrollment before delivery

Status: **design for Product Owner review, 2026-09-05; not an accepted
contract or implementation authorization.** This proposal develops the same
R-144 question. It creates no second active research question or task ledger.
The [C0-05 issue](https://github.com/dianabuilds/ardents-network/issues/4)
remains the authority for delivery status. The proposal does not claim that
the issue's acceptance criteria have passed.

### Recommended product decision

**Recommendation:** define a recipient-enrollment primitive and use H1 for
its first delivery path: an explicit offline approval followed by one exact
sealed response. A running Endpoint proves possession again at the receiving
Entry boundary. Keep H2 as an alternative if repeated manual delivery is
unacceptable. Both options need the same first-binding primitive; a responder
is not a replacement for it.

H1 is preferable for the actual one-operator team because enrollment need not
add an always-running listener, public registration API, or recovery service.
Its cost is real: a new issuer trust role, a protected issuance journal,
explicit file exchange, versioned Entry admission, and expiry handling. It is
not an artifact-verifier enhancement or an ordinary implementation of C0-05.

The proposed role is an **offline Endpoint-enrollment issuer**. It can approve
one exact public-key request under one finite enrollment authorization and
assemble an authenticated response. It cannot sign Network State, choose a
Route, mint an Entry Invite or Source certificate by virtue of enrollment
authority, issue a Service Credential, or authorize a Release. These remain
separate approvals by their existing respective authorities.

Propose a separate operator-only command/artifact for this role, provisionally
`ardents-enrollment`. It is outside the current four-command contract; its
addition needs an explicit scope/ADR decision and artifact ownership. Do not
hide it in `VerifyHeadless`, turn `ardents-control` into a signer, give the
running `ardents-node` new ambient issuance rights, or reuse Service Custody.
The new command must not imply a new network Node role or a standing service.

### Evidence and confidence

All sources below were accessed on 2026-09-05. These are design inputs, not an
Ardents security proof or a selection of another protocol.

- **Sourced fact:** [Entry admission](../../../internal/entry/admission.go)
  indexes replay by attachment ID. The existing
  [reopen test](../../../internal/entry/admission_test.go) accepts the same
  Invite under a distinct attachment ID. Neither the Invite grammar nor its
  signature contains a recipient key. This proposal reviewed those sources;
  it did not rerun or modify the test.
- **Sourced fact:** [Entry verification](../../../internal/entry/verification.go)
  authenticates the Invite with its State-selected Initiator candidate key.
  An enrollment issuer cannot replace that signature with its own signature.
- **Sourced fact:** [RFC 9449, sections 2 and 11.1](https://www.rfc-editor.org/rfc/rfc9449.html#section-11.1)
  separates binding an authorization to a public key from checking a fresh
  proof and its replay history. This is a relevant pattern; OAuth, JWT, and
  DPoP are not proposed Ardents dependencies or wire formats.
- **Sourced fact:** [RFC 9180, sections 9.7.3–9.7.4](https://www.rfc-editor.org/rfc/rfc9180.html#section-9.7.3)
  does not give HPKE general replay protection or forward secrecy against
  recipient-key compromise. Sealing delivery therefore cannot substitute for
  recipient-bound Entry admission or durable lifecycle state.
- **Sourced fact:** [RFC 8446, sections 7.5 and 8](https://www.rfc-editor.org/rfc/rfc8446.html#section-7.5)
  defines TLS exporters and distinguishes application replay from transport
  protection. The proposal uses the normal exporter after a completed
  handshake and permits no early-data enrollment/admission.
- **Sourced fact:** the [Roughtime protocol](https://roughtime.googlesource.com/roughtime/+/HEAD/PROTOCOL.md)
  illustrates an authenticated time interval bound to a requester nonce.
  This supports the freshness question below; it selects neither Roughtime
  nor a time service for Ardents.
- **Inference:** the smallest defensible fix binds authorization at issuance
  and verifies possession at use. Encrypting the current bearer Invite leaves
  it transferable after decryption. Globally consuming an Invite only once
  would instead change legitimate attachment/reconnection behavior and permit
  a first-presenter theft race.

Confidence is high in the defect and required boundaries, moderate in H1's
operational fit, and unmeasured in its complete lifecycle. No experiment,
independent review, platform qualification, or implementation is claimed.

### Keys and the precise copying claim

Propose the following independently generated keys, held in owner-private
state outside the artifact bundle. Names here describe proposed purposes,
not new accepted glossary terms or package names.

| Key | Lifetime and holder | Permitted use |
|---|---|---|
| Endpoint enrollment signing key | One local enrollment lineage for one Network/cohort; survives an ordinary Endpoint restart | Sign enrollment requests and explicit successor requests; never a Person, Device, Service, or transport identity |
| Delivery recipient key | One pending response; erased after durable acceptance or terminal abandonment | Open the sealed response; never authorize Entry or Source access |
| Entry recipient signing key | One Entry slot/Invite lineage; independently generated for each slot | Prove possession to that Invite's exact Initiator; never sent to Source or other Route positions |
| Source client TLS key | One finite Source authorization | Prove possession through the existing CA-verified, leaf-key-pinned client TLS boundary |
| Attachment TLS key | One attempt, as today | Bind one fresh carrier attempt; never reused as enrollment identity |

The request binds the public purpose keys and proves possession of signing
keys; a Source request uses its ordinary proof of possession. The recipient
encryption key is authenticated by the signed request. Service Instance,
Introduction, Authority, State, Release, and Transit issuer keys are excluded.
No private Endpoint or Source key is issued, exported, transferred, or included
in a response. Local signing must expose only fixed-purpose operations, never
a general signing oracle to an Application.

**Proposed security claim:** an adversary holding a copied public artifact,
request, sealed response, or even the decrypted public authorizations cannot
create an admitted use under another recipient key, provided the Endpoint's
private keys, relevant issuer/admission roots, and authenticated first-binding
channel remain uncompromised. Measure this at both the Source and Initiator,
including after process restart, before Route attachment allocation.

This does not prove that a software key exists on exactly one physical machine.
A full copy of protected keys, or control of their signer, can impersonate that
same enrollment. A local file lock does not prevent cross-host cloning.
External generation floors can reject superseded credentials but cannot
distinguish two holders of the same still-current private key. This is an
explicit clarification requiring Product Owner acceptance of R-144's copying
criterion, not a silent relaxation. If full protected-state cloning must be
prevented, neither this H1 nor H2 is sufficient without a separately researched
hardware or exclusive online authorization boundary.

### First binding and response transaction

1. Verify the artifact before first execution using the existing independently
   delivered Alpha Enrollment Pin. Enrollment authority is authenticated
   separately through the already approved Product Owner contact, with its
   exact Network/cohort, public key, and finite purpose. The artifact pin alone
   grants no enrollment authority and no Endpoint admission.
2. The verified Endpoint creates one protected local root, purpose keys, a
   random request ID, and one canonical public request. Persist it before
   export. Repeating preparation returns the same request; it does not create
   another identity or silently replace keys.
3. The operator authenticates the request digest with the intended Endpoint
   Owner through the approved independent contact. A request signature proves
   possession, not entitlement. No shared bearer invitation, unauthenticated
   email address, or first network connection can establish first binding.
4. Under one exclusive issuer root, bind the independently approved finite
   authorization to the exact request digest and recipient-key commitments
   **before** releasing any response. The same authorization with another
   digest is a conflict, even if the attacker submits it first. An exact retry
   returns the recorded outcome. The authorization must already identify the
   approved digest; "first request wins" is forbidden.
5. Obtain separately authorized Entry and Source public outputs, using
   purpose-specific subrequests that disclose only their recipient key and
   necessary Network/slot/validity facts. The Entry issuer uses its own
   State-selected candidate authority; Source authorization requires the
   configured Source client CA and explicit authorized leaf-key policy.
   Neither accepts an enrollment certificate as sufficient authority. Their
   issuance/installation acknowledgements must be durable before the combined
   response is called ready. The coordinator holds no such private signer.
6. Commit the exact response bytes/digest to the issuer journal before delivery.
   Authenticate the envelope with the separately pinned enrollment key and
   seal it for the request's delivery key. Bind format, purpose, Network,
   cohort, request digest, recipient commitments, enrollment generation,
   predecessor, validity, and the complete payload digest. Once committed or
   exposed, those response bytes never change on recovery. A failure before
   that commit cannot be reported as delivery.
7. Endpoint validates bounded framing, issuer binding, request and generation,
   authenticated envelope, and decryption before accepting payload contents.
   It validates State, Entry, and Source outputs at their own owners, stages
   one recoverable transaction, and becomes ready only after every required
   owner has durably accepted. A missing part remains pending/unavailable;
   rollback of already advanced authority floors is never compensation.

After acceptance, retain the authenticated response digest and public owner
receipts so that an exact repeat returns the existing result without needing
the erased delivery key. This idempotent result never restores expired
readiness. Retain the delivery key during incomplete acceptance; if it is lost
before completion, report terminal delivery loss rather than generating a new
key for the old ciphertext.

The reply is a separate typed delivery object outside the immutable v3 bundle.
It cannot be appended as unreferenced Release metadata or copied into another
Endpoint root to enroll it. Its contents are limited to already authenticated
Network Evidence, recipient-bound Invites, public Source client certificate
and bounded Source configuration, and delivery receipts. State anchors derive
from the existing verified Network Evidence rather than a second authority
list. No Target, route plan, Node private key, arbitrary path, command, URL
override, or executable is accepted from this channel.

The issuer's work is offline and operator initiated. Its journal retains exact
approval, generation, component receipts, and response digest; it does not
retain a Person directory, Target history, or live Origin observations.
Source and Entry issuance need explicit maintained command seams with their
own custody/approval contracts. Those do not exist merely because lower-level
signing functions or Source plan loaders exist today.

### Entry admission is the enforcement point

Propose a versioned successor to the Invite and EntryBinding contracts in
ADR-0025/ADR-0027. Preserve the current State reference and two-slot replacement
lineage. The Invite's **issuer-signed body** additionally commits to its Entry
recipient public key and finite authorization generation. An unsigned wrapper
around a v1 Invite is not a recipient-bound capability.

For every attachment, first complete the existing State-pinned TLS handshake
with a fresh client attempt key. The Entry recipient signs one canonical,
domain-separated proof binding the exact Invite digest, recipient generation,
Network/Epoch/digest, receiving Initiator identity, attachment ID, actual TLS
client-key digest, and normal TLS exporter context. The proof is valid only
on that channel; the exporter must not use early keying material. No Target
or complete Route enters this proof.

The Initiator checks current duty/State, Invite signature and recipient,
generation/revocation, finite deadline, channel binding, possession, and replay
under the owning admission transaction. It durably consumes the attachment
before allocating a Route attachment or reporting success. Rechecking State
must remain atomic with admission as in the current owner. Invalid possession
must not bind or spend a victim's entitlement. Pending handshakes and rejected
proofs still consume bounded admission resources.

A new attachment from the same authorized recipient is allowed under existing
capacity/deadline rules and requires a new channel proof. Repeating an
attachment is rejected. Thus the old "distinct attachment accepted" behavior
is not blindly deleted: it remains a positive case for the correct key, paired
with a negative case for another key and with recorded-proof replay tests.

The selected candidate must require the new admission version through
authenticated compatibility/State facts on both sides. A peer-proposed version,
missing proof, or old bearer Invite cannot trigger a v1 fallback. The later ADR
must name exact format identities and the treatment of existing v1 evidence;
existing bytes must not silently change meaning or be removed as cleanup.

### Durable lifecycle, recovery, and withdrawal

| Event | Required transition |
|---|---|
| Fresh initialization | Create one pending request; it grants no network authority |
| Issuer first bind | Commit authorization-to-request once; exact repeats reconcile, different recipients conflict |
| Interrupted component issuance | Reconcile the same subrequests and durable receipts; withhold the final reply until all parts agree |
| Interrupted local acceptance | Resume the same response into already advanced owner roots; never reset floors or expose partial readiness |
| Ordinary Endpoint restart | Reopen the same enrollment and Entry keys, generations, and protected roots; revalidate current safety before new work |
| Expired or revoked enrollment/credential | Deny new affected work and terminate it at the applicable safety bound; no grace period from copying or reimporting |
| Explicit replacement | Bind one successor to its predecessor and a new approved request; prove old admission is disabled or expired before declaring replacement complete |
| Lost key/root or irreconcilable journal | Terminalize that enrollment; revoke its authorizations and independently approve a new request; no recovery from a public bundle |
| Lost issuer/admission root | Issuance/admission unavailable; do not initialize an empty replacement ledger under the same authority |

Issuer, Endpoint, Source policy, and receiving Initiator have distinct durable
floors. Persist generation plus digest, reject lower generations and
same-generation divergence, and retain revocation tombstones through every
credential's possible acceptance horizon. A response generation does not reset
the existing Entry slot/replacement budget. Journals are finite and fail with
resource exhaustion rather than evicting still-security-relevant records.

Source withdrawal must remove the exact client authorization and join affected
sessions, including any resumable TLS state. Current Source files have no hot
reload: the proposal must supply an explicit policy-apply and bounded
stop/restart path, with completion receipts. Entry withdrawal similarly stops
new use and joins dependent work. The offline issuer records a withdrawal as
pending until the relevant receiving owners acknowledge it. Under partition,
the honest bound is the remaining credential/safety lifetime, not immediate
global revocation. Short lifetime limits exposure; it is not replay protection.

Restoring a complete old copy of every trusted durable root is not detectable
from those copies alone. Recovery therefore cannot claim rollback protection
solely from an on-disk checksum or generation in the restored directory.

### Freshness is an explicit unresolved selection within this proposal

H1 must not manufacture confidence by touching the current time-observation
file. A signed old Epoch or an enrollment response's `issued_at` proves no
current time by itself; Source TLS cannot authenticate the time needed to
validate its own certificate without an independent starting condition.

**Proposed requirement:** before time-dependent acceptance, obtain an
independently authenticated bounded time observation tied to a fresh local
challenge. Preserve its uncertainty, monotonic elapsed time, and durable floor;
evaluate not-before against the earliest possible time and expiry against the
latest possible time. Delayed responses consume the remaining safety budget.
After restart without a trustworthy elapsed-time bound, require a new
observation; do not extend any credential by refreshing time.

**Decision still required:** select the actual observation provider/procedure,
its trust anchor, error bound, loss behavior, and maintained command seam. A
bounded offline time witness is compatible with H1's absence of a standing
bootstrap responder, but is an additional explicit time trust role; assigning
it to the enrollment signer would silently expand that signer. An online
authenticated time service also requires its own operational/exposure
contract. Neither is selected here. Until this selection is made, the design
can address recipient copying but cannot claim a complete C0-05 startup or
restart journey. This is part of the decision packet, not a hidden later fix.

### Operator journey without authored plans

Proposed operation names below are an interface sketch, not runnable commands
or additions to the current command reference.

1. **Verify artifact**, using the existing pre-execution instruction and pin.
2. **Endpoint prepare**, producing one public request and its digest. The
   command derives owner-private roots, separate local principals/sockets,
   Network binding, and finite defaults. The operator supplies only local
   purpose and output location, not State, Entry, peers, keys, or JSON.
3. **Issuer approve and deliver**, independently confirming that request,
   obtaining the separately approved components, and exporting the exact
   response. One Product Owner may perform the distinct roles sequentially;
   no additional staff or continuous operator availability is assumed.
4. **Endpoint accept**, consuming that response plus the independently
   authenticated trust/freshness inputs. Persist only a command-owned local
   profile and return a bounded readiness or actionable failure result.
5. **Publisher prepare**, creating the Service Instance request through its
   owner with derived local paths. Retain the existing interactive
   `ardents-custody issue-service-credential` ceremony and its independently
   typed request hash. **Publisher accept/start/publish** then consumes the
   public response and prints the explicit Target Link.
6. **Second Endpoint prepare/accept/start**, with its own keys and separately
   approved response. **Open Target Link** exchanges bounded opaque bytes
   through the existing Application Connection Interface.
7. **Stop/restart/status**, retaining all domain floors and reporting whether
   enrollment, time, State, Entry, Source, or Service credentials prevent use.

Both User and Publisher use the same enrollment/profile lifecycle. Generated
local configuration is private implementation state, not an editable operator
contract; the response never supplies local paths or Application principals.
No background wrapper refreshes the time marker, answers Custody prompts, or
launches a test fixture. The maintained enrollment runbook gains this one
sequence only with the implementing change.

The existing published Service Instance restart limitation remains separate:
enrollment-key persistence does not restore the redacted published Instance
key or enable overlapping successor Credentials. A restart may correctly
report successor-required; this proposal cannot claim seamless Publisher
restart or resolution of the other research records' limits.

Failure output must distinguish rejected artifact/enrollment, recipient
mismatch, replay/conflict, revoked/expired authorization, uncertain time,
unavailable State/Source/Route, local resource exhaustion, interruption, and
Service lifecycle failure. Exact new reason strings belong to the accepted
command grammar later. None may become success, stale-good fallback, automatic
key regeneration, or a request to edit JSON.

### Proposed bounds and acceptance evidence

The following numbers are **unmeasured C0 design assumptions for review**,
not changes to current limits: one pending enrollment transaction per Endpoint;
two Entry slots; at most 64 live enrollment records per offline issuer; a
64 KiB request; a 1 MiB response; a 10-minute approval/delivery window; and
at most one hour of enrollment/Entry/Source authorization, shortened by every
applicable State, assignment, Release, certificate, and Work Safety deadline.
Existing stricter owner/parser limits still apply. No automatic delivery retry
loop or network discovery is introduced. Reconsider these bounds if the exact
Network Evidence does not fit or the one-operator walkthrough cannot finish.

Required falsification cases, declared before future implementation/evidence:

| Case | Required oracle |
|---|---|
| Copy artifact, response, or decrypted Invite to another fresh Endpoint; change attachment ID and TLS key | No accepted Source/Entry use or allocated Route attachment without the bound private key |
| Race a substituted request before the authorized request | No first-presenter binding; only the independently approved exact digest can be issued |
| Replay proof on another TLS channel, Initiator, Network, Epoch, recipient generation, or attachment | Rejection before allocation; current duty and exporter context checked |
| Same recipient reconnects legitimately | Fresh key/channel proof admitted within existing capacity and lifetime; duplicate attachment still rejected |
| Crash at every issuer bind/sign/write/delivery and Endpoint owner-commit boundary | Exact reconciliation or explicit terminal unavailability; no second identity, changed response, or partial ready state |
| Replay after issuer, Endpoint, Source, or Initiator reopen; race acceptance and withdrawal | Durable floors/replay survive; one terminal result and joined cleanup |
| Lost, rolled-back, corrupt, or divergent roots; parallel opens | No silent empty-root recovery, floor reset, or second writer |
| Expiry, time rollback, delayed observation, loss of monotonic continuity, partitioned revocation | Bounded unavailable/terminal outcome; no new work beyond the earliest applicable deadline |
| Wrong CA, hostname, client/server leaf pin, certificate/key, State signature, or recipient commitment | Rejection by the proper owner; enrollment does not override it |
| Oversized input, unknown field/version, full journal, repeated invalid proof | Bounded work/memory and explicit failure; no security-record eviction |
| Old bearer format offered or proof omitted | Incompatible/rejected; no negotiated or automatic v1 fallback |
| Two fresh Ubuntu Endpoints, real supplied artifacts, User and Publisher instructions | One authenticated Target-Link connection and bounded byte exchange without authored JSON, fixture keys, or hidden steps |

The last case must exercise the issuer and component-authorization commands as
well as the participant path. Test-only issuers cannot qualify that journey.
Future code requires owner behavior/race/parser tests, the selected process and
Ubuntu profiles, `make quick-check` while developing, and `make check` before
integration. No tests or security gates are weakened to accept this proposal.

### Ownership and decisions before implementation

Keep the static `internal/enrollment` verifier pure. A proposed separate
recipient-enrollment owner holds keys, request/response acceptance, and its
finite journal; issuer approval/issuance is a distinct trust owner.
`internal/entry` owns Invite/recipient/replay rules, `internal/route` the
versioned channel proof, `internal/node` the receiving duty transaction,
`internal/network/source` its TLS authorization, and `internal/network/state`
authenticated State/time acceptance. Endpoint composes these owners; it does
not absorb their durable state. Exact package/import/command boundaries need
the package-map decision before any new package is created.

Use existing reviewed cryptographic implementations, including standard Go
signatures/TLS and the already registered HPKE implementation if sealing is
retained. Reusing that dependency does not authorize a new HPKE suite or the
Service Instance recipient. Review the selected fixed suite/API, key erasure,
license, distribution, and artifact closure for this purpose before coding;
no dependency or primitive is added by this document.

The enrollment issuer learns the mapping between request, Source key, and
Entry keys; Source sees client identity/origin; an Initiator can link uses of
its recipient key during that Invite lineage. Separate purpose keys reduce
unnecessary cross-role disclosure but do not hide this mapping from colluding
project operators. H1 selects no anonymity, unlinkability, independent
operation, or public enrollment claim. Finite journal retention and protected
diagnostics must be included in its accepted privacy statement.

The Product Owner decision must cover, together:

- the precise copied-material protection and full-key-compromise limitation;
- the offline enrollment authority, authenticated request approval, separate
  operator artifact, and purpose-isolated Entry/Source issuance operations;
- H1's manual finite response delivery and explicit loss/expiry/re-enrollment;
- the recipient-bound Entry format and compatibility transition;
- the actual time-observation trust/procedure and measurable budget; and
- the derived User/Publisher command journey and retained Service limits.

Acceptance would be recorded in consequential ADRs and promoted to the current
product/security, enrollment, Network/Route/Node, Endpoint, command, package,
artifact, and operational owners. It would then permit selecting a separate
implementation issue under the one-issue C0 WIP limit. It would not itself
close C0-05, create a task, or integrate code.

The strongest objection to H1 is the compound offline ceremony: purpose-
separated issuance, finite time evidence, and manual redelivery may be too
burdensome even for one technical operator. If that is unacceptable, choose H2
only with an explicit availability, custody, rate-limit, recovery, and
direct-source exposure contract. H2 still cannot mint State/Entry/Source
authority or prove uniqueness of a cloned software key. Keeping both options
unselected remains a valid outcome until these concrete costs are accepted.

## Disposition

Open and selected on 2026-09-05. No ADR, implementation, dependency, wire, or
current owner contract has changed. The external proposal at
`C:/Users/vitek/AppData/Local/Temp/ardents-r-144-headless-bootstrap-proposal-2026-09-05.md`
is planning evidence only and remains outside the repository.

The Product Owner selected the H1 direction and its separate offline
challenge-bound time witness on 2026-09-05, and accepted ADR-0071. The
prepared recipient-enrollment proposal remains design evidence. No command,
format, package, dependency, or implementation issue is accepted by this
record; no code or test was changed or executed for the design work.
