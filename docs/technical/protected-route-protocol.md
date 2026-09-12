# Protected forwarding protocol

Status: **closed successor design contract**; authenticated generation 3,
profile name `ardents-route-v3`. It is the one construction for the
[selected workload](../product/protected-service-workload.md).
It does not change the current generation-2 implementation until migration.

## Carrier, cryptography and information flow

Retain the two transport families selected by
[ADR-0048](../adr/0048-maintain-tcp-and-quic-carriers.md): TCP/TLS and QUIC v1
using quic-go v0.62.0. ADR-0081 selects successor Carrier profiles
`ardents-carrier-tcp-tls-v2` and `ardents-carrier-quic-v2` with ALPN
`ardents-route-v3`. They carry the same protected lane and inner TLS
composition. The old v1 identifiers retain their generation-2 meaning; a new
ALPN/binding must not silently change that signed identity. Retain the QUIC
profile's single ordered bidirectional stream, disabled datagrams, 1200-byte
initial-packet configuration and bounded cleanup. ARDP multiplexes inside
that stream; QUIC streams do not become a second lane authority. The authenticated receiving Node Record selects exactly one
Carrier for an attempt; no race, negotiation or failure-triggered fallback
selects another. Replacement requires a separately authorized fresh Attachment
under the same protection generation. Qualify both Carriers.

Adjacent Node Carriers retain exact current Node-key authentication.
Endpoint-to-Entry and inner Endpoint-to-role TLS use authenticated server keys
without stable Endpoint client certificates. The issuer authenticates an
issuance holder only inside its confidential operation. Disable tickets,
resumption, 0-RTT, session caches and peer-supplied certificate roots.

Each literal successor endpoint has one v3 listener for both direct role TLS
and Node Carrier TLS; a second listener, endpoint or ALPN is not implied. The
listener requests but does not require a client certificate. After TLS and
before an ARDP frame, exactly one current State-authorized Node certificate
selects the outer Node-Carrier state; no client certificate selects only the
direct Endpoint-to-role state. An unexpected, malformed, multiple, expired or
non-current certificate is unavailable and closes before ARDP work. Direct
role state refuses a certificate and Node-Carrier state never falls through to
direct role handling. The shared finite TLS-handshake reservation applies
before this classification; source identity or a rejected certificate cannot
increase it. The shared listener starts each bounded TLS/first-stream
acceptance interval when that connection arrives, not when the listening duty
begins waiting. Its maximum remains ten seconds, and caller cancellation still
bounds the handshake. Listener idle time consumes no peer handshake allowance;
no HELLO, admission, parent or authority deadline is extended.
Use the exact Node/role key from authenticated State, not Web PKI or DNS.

Select Go 1.26.8 for the successor build; earlier component evidence identifies
its actual Go 1.26.6 environment. Fix TLS key exchange to X25519MLKEM768 and
X25519, in that order, for
this closed profile. Both are part of one configured TLS profile; selection
cannot cross an authentication or privacy generation. Go's TLS 1.3 cipher
selection remains its maintained implementation. No post-quantum anonymity or
security claim follows. Service recipient HPKE is the existing X25519,
HKDF-SHA256, AES-128-GCM suite with independently generated short-lived keys.
Do not implement these primitives.

The five data positions remain User Entry, User Interior, Rendezvous,
Service Interior and Service Entry. A separate Introduction path carries no
Application Data. Each Endpoint selects its own leg; a Node sees only its
adjacent peer and the next hop it must open, not a supplied complete path.

Preserve the current Role Domain and Entry/Interior Set contracts. In the
closed profile, duty subroles are disjoint for the assignment lifetime:
endpoint adjacency, interior forwarding, Introduction delivery, data joining,
resolution and issuance cannot be combined on the same Node to bridge a
forbidden observation. Known-family exclusions apply as well. A role change
requires expiry/drain of every old assignment, not a new advertised key alone.

## Selection and readiness

A signed profile fixes eligible duty sets, exact Node addresses/keys, common
admission configuration, finite validity and the profile digest.
A profile cannot be accepted without the existing State/enrollment/time
checks. A Node's advertised IP is a dial input only after this verification.
No hostname, arbitrary IP, URL, route list or replacement trust root comes
from an Application, Descriptor, Introduction capsule or relay.

### Authenticated closed profile

The profile is public configuration authenticated by the same independently
pinned closed State authority, under a distinct signing domain. A distributor
has no signing or acceptance authority. Verify the existing current Epoch first,
then verify the profile against that exact Epoch and its authenticated authority;
an Application, issuer or Node certificate cannot supply this trust root.

All integers are unsigned big-endian. The unsigned body is `ARDCPR03`[8],
version u16=3, Network[32], State-generation[32], Epoch u64, Epoch-digest[32],
not-before u64, not-after u64, issuer-Node[32], issuance-authority-key[32],
Node-count u16, that many Node entries, key-count u16 and that many token-key
entries. Append one Ed25519 signature[64] over the domain
`ardents-closed-profile-v3\0` followed by that body. Profile digest is SHA-256
of the complete signed record. The closed State verifier owns acceptance and
returns an immutable verified profile; credential code consumes its narrow
issuer/key/permission projection, never an unchecked raw-profile callback.

`ardents-control prepare-closed-profile` renders only this canonical unsigned
body from a bounded public plan. `sign-closed-profile` rereads that plan, uses
one owner-only PKCS#8 Ed25519 State-authority file, and writes a new profile
artifact without printing the key or profile bytes. It cannot sign another
grammar and never creates a replacement authority. `inspect-closed-profile` is
read-only and checks the exact context and signature. A missing or malformed
signer is unavailable; durable State acceptance remains separate.

Each Node entry is Node-ID[32], SHA-256(exact signed Node Record)[32],
Role-Domain u8, subrole u8 and duty-generation u64. Sort by Node-ID and reject
duplicates. Domain values are Initiator=1, Rendezvous=2, Responder=3,
Introduction=4. Subroles are adjacent=1, interior=2, Introduction delivery=3,
data join=4, resolution=5 and issuance=6. Adjacent/interior use one of the three
adjacent Domains Initiator (1), Responder (3) and Introduction (4); Rendezvous (2) is never adjacent. Delivery uses Introduction; data join/resolution/issuance use
Rendezvous. The issuer entry is the sole issuance subrole. Current Node Records
supply the exact address, Ed25519 key, family and selected Carrier; their digest,
identity, assignment and duty must match. No address/key is duplicated in this
profile. Subroles refine existing Role Domains; they do not create new ones.
Node Record schema 2 retains its canonical byte layout, but this generation
requires one of the two successor Carrier identifiers above. Schema 1 and the
old v1 profile values remain generation-2 compatibility inputs only. State
must reject them for a generation-3 duty before any dial.

Token-key entries are window-start u64, class u8, DER-length u16 and the exact
RSA-PSS SPKI bytes from the [admission contract](private-admission.md).
Sort by window-start then class and reject duplicate pairs or reused keys.
There are at most 32 Node entries and 18 token-key entries: three classes for
each aligned hour of a profile's at-most-six-hour validity. No individual holder
gets a different configuration. Each token window lies inside the signed
profile and Epoch validity. The complete public State/profile inventory remains
inside the qualification owner's 64 KiB cap.

Exactly one profile digest is accepted per current Epoch. Persist its digest
beside State's existing generation/epoch floors before exposing it. Two valid
digests for that Epoch are a durable conflict; do not choose by arrival order.
A successor Epoch may bind a new profile only after normal State acceptance.
Expiry, withdrawal or conflicting authority prevents new work; existing leases
obey their original terminal bounds. This new persisted projection must be
included in migration and crash tests; it cannot reset State's existing floors.

For each activated adjacent Role Domain, select two eligible Entry Set members
once per installation from current State using a CSPRNG; hold for 6 hours or
the earlier assignment expiry. Choose one active member. Failure may use only
the already selected second member within the operation's retry bound; it
cannot sample a third or rotate the set. Choose a two-member Interior Set per
Isolation Context or Service role for 30 minutes, bounded by duty validity.
The Endpoint fixes the initial member and permitted alternative before dialing.

Select a Rendezvous uniformly from current eligible data-join duties after
excluding all known conflicting Node/family observations. Retain that public
Node choice only within the same local Isolation Context for at most 30 minutes
or earlier duty expiry. Select at most one eligible alternative at the same
time; failure cannot sample an expanding set. This retains a Node choice, not
a join, admission, terminal TLS session or cross-context identifier. An Introduction
request cannot force Publisher Entry/Interior resampling. The closed duty
partition makes a Rendezvous candidate ineligible to be a Publisher Entry.
A fresh Connection always has a new join secret and terminal channel.
A Node may host other Connections; completed Rendezvous state is never reused.

One target-free Entry/Interior prefix may remain prepared per admitted local
context for 120 seconds after actual work. A prefix belongs to one outbound
Isolation Context or one Publisher role. Fresh terminal TLS lanes separate
Name resolution, reachability, Introduction, issuance and data joining within
that owner. Reusing this outer prefix does not reuse a Gateway session or mix
private caches. It may host concurrent child lanes for the same operation.
Source bootstrap and Publisher Introduction/data Role Domains remain separate.

The source owner serializes terminal OPEN allocation with emission order and
retains a separate reader, credit and deadline for each child. The retained
Entry/Interior framing reapplies its original parent deadline to its own CREDIT;
a completed child's payload deadline cannot poison later parent consumption.
Waiting for the physical writer or payload credit remains bounded by the exact
payload deadline, including concurrent deadline updates. Updating a deadline
still interrupts an already active physical frame. Cancellation removes
unemitted work without sending a CLOSE for an unknown lane. An already emitted
CREDIT may finish within the earlier of its original write deadline and the
same one-second cleanup bound; later deadline updates cannot extend that bound.
A received complete next-peer CLOSE retires queued forwarding work independently
of downstream delivery, while physical write failures remain terminal errors.
A local child or parent deadline also retires reverse receive authority. Late
frames for that owner consume no new queue reservation and cannot invalidate
other owners sharing the physical Node Carrier. The shared reader checks that
retirement before queueing and again after a physical queue-full refusal, since
expiry may occur while waiting for the queue lock. A live owner's exhausted
queue or invalid frame remains a failure; expiry grants no further work.
If a queued child CLOSE has not begun emission when its parent fails, cleanup
joins physical retirement without replacing the original traffic error with a
new cleanup error; a failed physical retirement remains a cleanup failure.
If an emitted child cannot send its required terminal CLOSE within the cleanup bound, the source
retires and joins the physical prefix; it cannot report that child as cleaned up
while retaining a usable parent.
A counted source lifetime worker expires idle readiness after 120 seconds.
An admitted child's completion starts the next idle interval; rejected or
unemitted children cannot refresh it. Live child work still obeys the original
parent deadline. Autonomous retirement joins the source and nested readers
through the same terminal path as explicit Close before notifying Endpoint.
Endpoint observes that completed retirement before new private work; it keeps
the context's source members, allocation and exact pending-batch binding.
Retirement never creates another bootstrap allowance or starts an idle refill.
An idle Publisher checks an incoming capsule's independently selected current
Rendezvous and control-family separation without opening a Source. Only an
accepted capsule may prepare Source/Responder forwarding. Source readiness and
ordinary token issuance share a cancellable context-local operation reservation;
Descriptor acknowledgement and Service streams do not hold it. Admission,
registration and publication revalidate their own authority after waiting for
actual issuance instead of treating another valid issuer operation as lost
publication authority. Existing pending batches and finite allocations remain
unchanged. A cancelled waiter cannot release the active operation's reservation.
During explicit reader resolution or scheduled publication, Endpoint ensures
an unspent forwarding token is available per retained Source receiver for the
next open, obtaining only missing tokens through the current admitted Source and
existing allocation. Observing a joined Source precedes the decision to reopen;
no idle task replenishes stock or resets the two-batch bootstrap limit.

Node-to-Node Carriers established by real work may be retained for 120 seconds
after their last child closes. Key the local pool only by exact current
Network/profile, two public Node identities and selected Carrier; at most one
Carrier per directed peer pair and 32 per Node. Do not pre-dial unvisited peers
or refill an idle pool. A fresh child gets a fresh inner role TLS channel,
HELLO, receiver admission and hop-local lane ID. The Node-authenticated outer
multiplexing allocates only bounded pre-admission handshake lanes; it grants
no Endpoint admission or right to bypass an inner receiver. Changes to the
verified HELLO binding or State/duty invalidate reuse. Pool queues and children count against
the same whole-Node limits. Last-child cancellation does not erase another
child, but withdrawal/expiry joins all children before closing the Carrier.
This is finite reuse of useful-work setup, with no reserved Service Connection.
The pool and its session borrowers share one physical close operation and retain
its original result; concurrent withdrawal joins that same close.
The outer HELLO lifetime is bounded by the current signed profile and receiving
State/duty bounds, independently of the first child's ten-second handshake.
The initial HELLO/ACCEPT exchange and sending a new OPEN use a separate
at-most-ten-second I/O deadline within the current authority. An ordinary
OPEN declares its maximum child lifetime within the parent; the receiver
independently limits pending TLS, HELLO and admission to ten seconds. Only the
actual receiving admission may extend that child's I/O beyond the pending
interval: class 2 for forwarding, or class 1 for the issuer or resolution
duty's one-operation Control channel. Each remains bounded by its own lease. The receipt is matched
to the exact inner TLS object, HELLO and exporter before the outer child accepts
that lifetime.
HELLO alone cannot extend a child. Restricted issuer-bootstrap OPEN and its
entire child remain limited to ten seconds.

A role writer that exhausts its child's outbound credit waits for the actual
peer CREDIT within its current write deadline. It holds no shared Carrier
writer while waiting. Each emitted BYTES frame fits both the available credit
and the 16 KiB frame bound; consumption is never refunded after a failed
physical write. Incoming credit, a changed deadline and child close wake that
lane's waiter. Temporary backpressure alone does not retire a live channel.

A child's changed write deadline updates only that child's active physical
frame. A queued writer reads the current deadline after obtaining write
serialization. If that deadline has already expired, the writer returns the
timeout before attempting physical output and preserves sibling lanes. Once
physical output starts, a failed frame still retires the Carrier, including a
partially emitted CREDIT. Incoming child CLOSE can retire its state while a payload
writer is blocked. A terminal CLOSE has its own one-second cleanup write
bound after payload expiry; it grants no further payload credit or authority.
Failure of a partially written physical frame invalidates the Carrier framing
boundary and joins its children; ordinary child expiry does not itself close
a healthy shared Carrier. If the lower authenticated parent has already sent
CLOSE(0), an upper CLOSE that never began physical output is discharged by
joined parent retirement. The retained lower framing's monotonic payload-attempt
count must remain unchanged across the complete encoded write, with no active
lower writer at either observation. Independently successful CREDIT does not
count as payload output, but any failed physical write, including CREDIT,
permanently vetoes this clean witness. This evidence spans nested TLS records; zero
returned bytes or EOF alone is insufficient. Preserve the parent's cause and
any retirement failure. Peer refusal, raw EOF and any started physical frame
remain failures.

For a joined stream, a late CREDIT may become unnecessary after the exact outer
lane receives CLOSE(0). Discard only an unemitted CREDIT: the outer lane's
monotonic physical-attempt count must be unchanged across the entire encoded
write. An already counted outer write may finish between observations; the
final observation requires no active writer, no local close, no parent failure,
and no earlier failed physical write. Keep the reader and its bounded reservation alive to consume
already accepted inner records and validate their terminal. Raw EOF, refusal,
partial output and missing inner completion cannot become document success.
For a wholly unemitted local JOIN CLOSE, the outer-lane cleanup witness counts all non-CREDIT physical attempts across the entire encoded write. A concurrent CREDIT may exist at the initial observation, but the final observation requires no active write, an unchanged count, actual CLOSE(0), no local close or parent failure, and no earlier failed physical write of any kind. Only then may joined parent retirement discharge that CLOSE. Refusal, partial output and failed CREDIT remain failures; cleanup grants no payload success.
A registered Publisher may maintain one Introduction prefix and one data-role
prefix as finite publication readiness. These count as publication background
work; no unvisited Service is kept ready by an idle User. Publication refresh
may renew this readiness; an idle User's expired prefix never self-refills.

A forwarding-channel admission reserves its own aggregate byte/time budget
and permits at most 256 simultaneous work lanes and two reserved control lanes
within that reserve. Each
child has its own next-recipient admission. An additional child cannot multiply
its parent's allowance. Replenishment changes only the remaining byte reserve,
never the peer, purpose, context or original terminal deadline.

## Bounded lane framing

Every TLS channel terminating at an Ardents role carries the same lane grammar.
Header: ASCII `ARDP`[4], generation u16=3, kind u8, flags u8=0,
lane u32, body-length u32, followed by exactly body-length bytes.
All integers are unsigned big-endian; there is no varint, optional field,
compression, trailing data or extension negotiation. Header length is 16.
Reject an unsupported generation/kind/flags/length before allocation.

Lane zero is reserved for one channel admission handshake. Endpoint-initiated
child lanes use monotonically increasing odd IDs; role-initiated lanes use
even IDs only for already authorized publication delivery. IDs are local to
one TLS channel and never reused there. Exhaustion closes the channel.
No lane ID or nonce is copied into a different hop's identifier namespace.

| Kind | Body and valid use |
|---|---|
| 1 HELLO | Network[32], State-generation[32], State-digest[32], profile-digest[32], recipient-Node[32], recipient-duty-generation u64, purpose u8, fresh channel-nonce[32], absolute-deadline u64. Exactly 209 bytes; lane zero, once after TLS. |
| 2 ADMIT | class u8, token[354]. Exactly 355 bytes; lane zero for initial admission or an existing lane for finite replenishment. |
| 3 BOOTSTRAP | operation u8: public evidence=1 or issuer=2. Exactly one byte; lane zero only, under the finite bootstrap contract. |
| 4 OPEN | next-Node[32], next-duty-generation u64, next-purpose u8, deadline u64. Exactly 49 bytes on an Endpoint-role channel. On an authenticated Node Carrier append mandatory restriction u8 (0=no additional restriction, 1=issuer-bootstrap only), exactly 50 bytes. The authenticated channel state fixes the grammar; fresh child after the corresponding parent admission or bounded Node allocation below. |
| 5 ACCEPT | status u8 (0 accepted, 1 unavailable, 2 exhausted, 3 stale/incompatible, 4 withdrawn), credit u32. Exactly 5 bytes; credit zero on refusal. No detailed path-conflict oracle. |
| 6 BYTES | 1 through 16,384 bytes on an accepted lane, within current credit and parent budgets. A pending OPEN permits at most 4,096 total bytes of the next TLS handshake under the separately reserved handshake allowance; no Application Data is enabled. Until the terminal role these bytes are opaque to the forwarding role. |
| 7 CREDIT | u32 additional byte credit, from 1 through 65,536, only for a live accepted lane. Reject overflow or credit above its fixed receive-window maximum. |
| 8 EOF | Empty body; closes only the sender's lane direction after preceding BYTES. |
| 9 CLOSE | One terminal status byte using ACCEPT's classes plus cancelled=5 and timeout=6; closes the lane and all its descendants. No child creation afterward. |
| 10 OPERATION | terminal-operation u8, request-nonce[32], payload. Total body exactly 4,096 or 16,384 bytes as selected below. Allowed only at the exact current recipient role; private payload sizes are fixed as below. |
| 11 RESULT | matching request-nonce[32], status u8, result-length u32, result bytes and zero padding to the operation's fixed bound. |
| 12 KEEPALIVE | Empty body, lane zero; answered once with the same kind only on a role-designated initiator/responder direction to prevent echo loops. |

Purposes are issuer=1, Name=2, reachability=3, Introduction registration=4,
Introduction submission=5, data join=6 and forwarding=7. Unknown purposes
are rejected. For generation 3, the following is the normative purpose-to-duty
assignment table. It checks only that a recipient assignment is eligible for a
purpose; it is additional to the Role-Domain/known-family and role-transition
rules, the exact accepted profile/Node Record/duty/digest match, admission and
every resource budget. A matching subrole never independently authorizes an
arbitrary route.

| Purpose | Permitted recipient assignment |
|---|---|
| issuer | Rendezvous / issuance |
| Name | Rendezvous / resolution |
| reachability | Rendezvous / resolution |
| Introduction registration | Introduction / Introduction delivery |
| Introduction submission | Introduction / Introduction delivery |
| data join | Rendezvous / data join |
| forwarding | one of Initiator, Rendezvous or Responder / endpoint adjacency or interior forwarding |

The table is an interpretation rule for the existing signed Node-entry
Role-Domain and subrole fields; it adds no field and does not change the
`ARDCPR03` profile grammar. The HELLO is itself protected by the recipient TLS channel.
It binds the full channel to current verified public facts; it contains no
Name, Target, Persona, permission ID or Isolation Context identifier.
On Endpoint-to-role channels, before ADMIT/BOOTSTRAP permit only HELLO
and termination. Authenticated outer Node Carriers have the separate bounded
handshake state below; Node identity cannot satisfy an Endpoint's ADMIT.

OPEN and the bounded initial TLS handshake may be sent in one flight; they do
not wait for an extra OPEN acknowledgement. The forwarding role validates the
parent authority, next recipient and handshake reservation before dialing or
forwarding those bytes. The next Node can process its authenticated Carrier
binding and the incoming TLS handshake in one flight. This is bounded handshake
forwarding, not TLS early Application Data. It is needed by the latency model.

On a successor Node Carrier, mutual TLS verifies both exact current Node keys
and their State-authorized duties. The dialer sends HELLO on lane zero with
purpose=forwarding, the receiving Node/duty and the exact current State/profile
binding; the receiver verifies every field before ACCEPT. This outer
`HELLO=forwarding` is its own Carrier state, not a lookup of the inner OPEN's
recipient assignment: it may establish a bounded Carrier to an issuer or
Introduction duty even though those duties are not forwarding destinations.
It authorizes only bounded creation of inner channels. This HELLO and the
TLS-authenticated public peer identities replace the old LegBinding record;
no generation-2 LegBinding bytes or Endpoint context label cross this Carrier.
The receiver cannot substitute a peer or return a different profile in ACCEPT.

[ADR-0082](../adr/0082-bind-bootstrap-restriction-to-node-child.md) amends the
outer OPEN with a mandatory child-local restriction byte. An honest forwarding
Node derives it from the actual incoming channel reservation: issuer bootstrap
sets 1, ordinary admission sets 0. Value 0 grants nothing at the recipient. A
restricted child is bound before TLS allocation, permits only forwarding or
issuer purpose under the exact bootstrap route, and refuses private ADMIT even
with a valid token. Every further child retains the restriction. The Endpoint's
49-byte role OPEN cannot supply, remove or replace it. No frame or successful
issuance relabels an existing child; private work uses fresh children after real
admission at every required hop. Mixed restricted/ordinary children share the
same exact Node Carrier pool without sharing authority.

The former 49-byte Node-outer OPEN is explicitly retired, not decoded as an
implicit zero. Old and revised Node peers fail closed before child effects;
there is no fallback or changed generation 2 meaning. Adoption of this amendment
requires the exact revised artifact and fresh signed profile/duty bindings,
quiescence and preserved floors under the existing migration contract. Prior
receipts do not qualify the amended bytes. The Endpoint-role OPEN remains 49
bytes and rejects an injected restriction byte.

Only this authenticated outer channel may use OPEN to allocate an inner TLS
handshake lane without an Endpoint token on the outer channel. Such an OPEN
must name the receiving Node itself, its exact duty and a purpose permitted by
the assignment table. A different Node, duty or digest is unavailable before
dial, reuse of a ready Carrier or forwarding any bytes. It never authorizes
that receiver to dial another Node. The fresh inner role TLS HELLO must name
the same receiving Node/duty and the exact purpose assigned by that OPEN; a
mismatch is unavailable before admission or bytes. The Name row does not make
Names available: terminal Name handling remains reserved and refuses this
generation's requests as specified below.
Its nonce/IDs are local; its initial bytes use the same 4,096-byte handshake
allowance and whole-Node caps, with a deadline no later than ten seconds or
the earlier parent/authority deadline. HELLO, OPEN and initial handshake bytes
may be pipelined after mutual TLS; all receiver checks still precede effects.
A fresh inner role channel must independently pass HELLO and ADMIT or bounded
BOOTSTRAP before it receives further credit or authorizes any forwarding.

An Endpoint has no Node client certificate. Its server-authenticated direct
channel is accepted only by the endpoint-adjacent duty; non-adjacent public
listeners require the authenticated Node Carrier above. Inner Endpoint TLS
arrives inside an authorized Carrier child. On an admitted inner forwarding
channel, OPEN names the actual next eligible Node and opens or reuses that
Node's Carrier. The authenticated channel state fixes which OPEN semantics
apply; there is no peer-supplied switch. Do not reuse a generation-2 session
or automatically fall back to it.

A forwarding role opens only the exact next public Node/purpose allowed by
the profile. The next authenticated Node Carrier provides a bounded incoming
lane for a fresh Endpoint-to-role TLS handshake. It supplies no origin field.
The recipient enforces handshake and pre-admission budgets before allowing
another hop. The same code owns User and Publisher forwarding; purpose never
enables arbitrary Internet dialing.

Use TLS exporter label `EXPORTER-ardents-channel-v3` and
SHA-256(exact HELLO body) as exporter context, returning 32 bytes.
Admission and local lane state retain this binding. Do not publish it, put it
in a token challenge, or compare exporters across roles.

## Flow control and scheduling

Each live lane starts with 64 KiB receive credit and at most 64 KiB queued
ciphertext per direction. CREDIT is granted only after bytes leave that queue
for the actual consumer. Prefix queues total at most 4 MiB; complete Node
queues remain within the admission owner's 64 MiB ceiling.
A slow lane cannot block reading another lane's bounded control/termination.

Schedule one at-most-16-KiB frame per ready lane in round-robin order. Reserve
a separate 16 KiB/channel control queue and service it before data, with
control-rate admission to prevent priority flooding. Coalesce already
available bytes for at most 1 ms, never wait to manufacture traffic.
A peer exceeding credit, frame bounds or the channel's ownership closes that
channel; accepted Service bytes are never silently dropped.

A child cannot extend its parent's deadline or byte reserve. Child cancellation
joins its reader/writer/next-Carrier cleanup. EOF preserves the reverse
direction; terminal CLOSE prevents more data and joins both directions.
No parser error can create a child, perform a dial or consume an unlimited
signature-verification budget. Retry cleanup does not erase a prior failure.

## Complete connection sequence

1. A trusted local owner authorizes the job; the selected worker is confined
   before it receives an attachment or remote Application effect.
2. Endpoint obtains current public facts and finite token stock through the
   bootstrap/issuance contract. Retire bootstrap channels and establish genuinely
   admitted channels with fresh children before private use.
3. Parse the authorized Target Link and verify its Network binding locally.
   Verify a fresh/scoped Descriptor through a confidential terminal lane.
   Canonical Name resolution is a later composition with a real authenticated
   Namespace producer; the closed job returns unavailable for Name input before
   any network effect. It does not use an alpha alias or simulated close proof.
4. In parallel, open fresh terminal channels for data Rendezvous and
   Introduction submission over the context-owned prefix. After durably marking
   their independent tokens and reserving the join locally, send the one JOIN
   and one capsule concurrently. Do not wait for a Rendezvous acknowledgement
   before capsule delivery. Each receiving role validates its own admission
   before effects; a refusal cancels the complete bounded attempt. A malicious
   Rendezvous could withhold despite an acknowledgement, so no security claim
   treats that acknowledgement as authority to create Publisher work.
5. The Introduction role forwards the recipient-confidential capsule through
   the Publisher's registered Introduction channel. It never sees the data
   join secret, chosen Rendezvous, handshake binding or Publisher origin.
6. Publisher validates the capsule, current publication and independently
   verified Rendezvous eligibility, then opens the data leg through its
   separate Responder-domain prefix. The same token rules apply.
7. Rendezvous pairs exactly two matching live joins. Each endpoint then
   authenticates the existing end-to-end Service TLS and Connection binding.
   Only now may the Application exchange its request/response. Completion also
   includes the retained directional Terminal receipt and peer confirmation;
   they cannot be excluded from the useful-response clock.
8. Completion, cancellation, withdrawal, expiry or failure follows one joined
   terminal lifecycle. Bounded Connection recovery may acquire a fresh
   Attachment; it cannot repeat the Application request or switch generation.

## Terminal payloads and private reachability

A private operation's fixed size counts its complete OPERATION body:
operation byte, request nonce, canonical payload and zero padding. The 16-byte
lane header and TLS records are additional accounted bytes. Descriptor requests
are exactly 4,096 bytes by this rule.
Their replies are exactly 16,384 bytes including the RESULT fields.
The selected closed Descriptor proof must fit that bound; reject an
oversize result rather than return an incomplete proof or direct URL.
Retain current proof, Target, currentness, conflict-floor and cache validators.
Success, absent, stale and refused responses use the same public size.
The Name purpose is reserved and refused in this closed generation until a
separately accepted canonical Namespace composition supplies its authority.
It cannot redirect to a Target Link, local alias or an unprotected resolver.

A successor Descriptor retains the existing exact Target/Instance/Credential
and publication-generation proofs. Add revision ordering within the same
publication generation: a larger valid revision replaces the current one,
a same-revision conflict persists as unavailable, and a lower revision cannot
return. A refresh cannot bypass the existing no-overlap Instance-generation
rule. Store and local cache keep both the publication and revision floors.
Ordinary reachability publication and lookup use the one State-selected
resolution duty; that duty learns the queried Target but not Endpoint origin. Its replacement reachability fields are:
revision u64, Introduction-Node[32], slot[32], recipient-HPKE-key[32],
not-before u64, not-after u64 and Instance Ed25519 signature[64].
Sign the domain `ardents-private-reachability-v3\0` plus Network[32],
profile-digest[32], SHA-256(existing public publication proof) and those fields
in order excluding the signature. It has no data Rendezvous or reusable join.
Revision strictly increases within an Instance; retain its conflict floor.

The maintained v3 Descriptor envelope is version u16=3, Network[32], Target[32],
Authority[32], Publication-digest[32], profile-digest[32], the revision/recipient/
validity fields above, publication-length u16, exact Publication bytes, then
Instance-signature[64]. Integers are big-endian; no trailing bytes are accepted.
The Publication digest must equal SHA-256 of those complete Publication bytes.
The private verifier requires the current expected profile as well as Target
and Network. Lookup and publication OPERATIONs are carried on lane zero inside
their dedicated, already admitted terminal TLS channel; the receiving purpose
and exact body decoder remain mandatory after generic frame validation.
Terminal operation IDs are issue=1, Descriptor lookup=2, register=3,
submit capsule=4, join=5, publish Descriptor=6 and withdraw slot=7. No other
operation is accepted. One operation uses one admitted terminal channel;
publication registration alone keeps its bounded delivery channel open.
Issuance carries exactly the permission/request encoding in the admission
owner, padded to 16,384 bytes; bootstrap permits at most two such batches for
one permission, within its aggregate limit. Descriptor lookup carries Target[32]
and zero padding to its fixed request size. Publish carries u16 descriptor
length, the exact signed successor Descriptor and zeros to 16,384 bytes; the
existing reachability Store verifies authority and durable conflict floors
before returning success. Private control recipients never accept an unchecked
storage callback. A complete Descriptor is at most 15,000 bytes before padding.

Register carries slot[32], revision u64 and expiry u64, padded to 4,096 bytes.
A class-3 admission creates one random vacant slot bound to that exact terminal
TLS channel. It does not prove Service Authority: the independently signed
Descriptor supplies that authority to readers. A duplicate slot is refused.
A refresh always creates a new random slot and recipient key, then publishes
a strictly higher Descriptor revision before reporting readiness. Losing the
registration channel closes its delivery state; reconnect cannot reclaim it.
Withdraw carries slot[32] and revision u64 on its owning registration channel.
Delivery is an even recipient-initiated child carrying exactly one fixed-size
capsule. Bound its acknowledgement and lifetime by the original registration;
no application payload or remote-selected callback address is accepted.
Within that already admitted registration, Introduction sends one OPERATION
on a fresh strictly increasing even lane ID, Publisher returns exactly one
RESULT on that ID, and Introduction terminates the child with CLOSE carrying
the same outcome. This does not send OPEN or create a new next-hop authority.
The OPERATION request nonce is freshly generated for this registration channel;
the submitter's nonce is used only for its own RESULT. The sealed capsule,
including its separate delivery nonce, is unchanged. Lane allocation and frame
emission share one writer so concurrent submissions cannot reorder IDs.
At most 16 deliveries, including those waiting for that writer, may be pending.
Reserve each operation, fixed RESULT and CLOSE against the original class-3
byte allowance before dispatch; retain owning-withdrawal capacity separately.
Missing or invalid acknowledgement makes that delivery unavailable; if an
expired or failed write cannot complete the child protocol, retire its
registration channel without allowing slot reclamation.

Submit carries the following outer capsule, padded to 4,096 bytes; join carries
the JOIN body below, padded to 4,096 bytes. All ordinary operation outcomes have
16,384-byte RESULT plaintexts. Parse the selected operation's exact inner length
and zero padding; no request may concatenate a second operation. A payload
length prefix, where needed, is u16 and part of that operation's fixed envelope.
Thus payload plus padding has 4,063 or 16,351 bytes; never add another fixed-size
block after the operation/nonce. RESULT's 16,384-byte bound includes its nonce,
status and length fields. The complete Descriptor ceiling above fits both.
The OPERATION request-nonce is channel-local and never copied to the next hop.

The outer Introduction payload is slot[32], revision u64, expiry u64,
delivery-nonce[32], HPKE encapsulation[32], ciphertext-length u16 and
ciphertext. The complete capsule is carried inside the fixed 4,096-byte operation payload.
HPKE associated data is the generation/profile domain plus that header before
encapsulation. The recipient-only plaintext is Network[32], Target[32],
publication-digest[32], revision u64, Rendezvous-Node[32],
Rendezvous-duty-generation u64, join-secret[32], handshake-context[32],
profile-digest[32], Connection-nonce[32], Attachment-generation u64 and
deadline u64, initiator-binding[32], WorkSafetyNotAfter u64,
WorkSafetyMaximum u64 and NoNewRecoveryAfter u64. Connection-nonce is fresh CSPRNG material for the logical
Connection and is carried only inside this capsule/Service encryption.
The canonical plaintext is exactly 344 bytes and the selected AEAD ciphertext
is exactly 360 bytes, including its tag. The complete outer capsule is 474
bytes; its 4,096-byte OPERATION therefore has 3,589 zero padding bytes after
the operation byte, channel request nonce and capsule. HPKE info is the exact
ASCII domain `ardents-introduction-capsule-v3\0` followed by profile-digest[32].
Associated data is that complete info followed by the exact 80-byte outer
header (slot, revision, expiry, delivery nonce), excluding encapsulation. The
recipient-only deadline equals the outer expiry and cannot exceed the signed
registration or Work Safety deadline. The channel request nonce is independent
of the capsule delivery nonce; changing either does not alter the other's
identifier namespace. These fixed bytes have no legacy format fallback.
Handshake-context is independent fresh 32-byte CSPRNG material per Attachment;
no stable logical Connection label reaches Rendezvous. No candidate-provided
IP is honored.

Rendezvous JOIN carries join-secret[32], side u8 (User=1, Publisher=2),
handshake-context[32] and deadline u64. Both independently authenticated
terminal lanes must match the secret/context/profile and opposite side.
At most two joins exist; third or duplicate-side joins are refused and do not
replace an accepted side. Expire unpaired state after 10 seconds.
The secret is 256 random bits. Each recipient independently generates its
admission nonce; the join secret is never an issuer input or a hop label.


### Rendezvous JOIN data-lane transition

[ADR-0083](../adr/0083-activate-joined-rendezvous-data-lane.md) selects the following
transition for the closed candidate. Each side opens its own fresh role TLS to
the State-selected Rendezvous through its Source or Responder prefix. Lane 0
carries existing HELLO, class-2 ADMIT and ACCEPT with the channel-local nonce
and TLS exporter binding. Admission alone enables no Service data.

On this dedicated one-JOIN channel, the Endpoint allocates local odd lane 1
and sends exactly one OPERATION with operation=5, a fresh request nonce and the
canonical JOIN body/padding above. It sends no OPEN. Other child IDs and a
second JOIN on the channel refuse. Rendezvous retains at most two live sides
with matching secret, handshake context and profile and opposite side values.
Duplicate/third sides or context mismatch refuse without replacing a retained
side. Unpaired state expires at the earlier original JOIN deadline or ten
seconds from reservation.

Only after pairing may Rendezvous emit the fixed 16,384-byte RESULT on local
lane 1 with that channel's matching request nonce, accepted status and empty
result payload. Refusal has the same fixed size and activates nothing. The
pairing owner maps local lanes; it never copies operation nonces or lane IDs
between the two TLS channels.

A verified accepted RESULT activates lane 1 with the existing 64-KiB receive
credit in each direction, within the original parent reserve. Carry Service TLS
ciphertext exclusively in existing BYTES/CREDIT/EOF/CLOSE frames on this lane;
return credit only after bounded queue consumption. No raw Service bytes are
written directly on role TLS. JOIN bounds setup; paired data keeps its original
HELLO/class-2 lifetime and byte reserve, already bounded by parent, State, duty
and Work Safety. Pairing never extends a lease or truncates a permitted data
workload to the capsule's ten-second setup deadline.

Accepted JOIN receive bytes remain readable before a subsequent transport EOF,
within their original bounded reservation and lifetime. Without a verified inner
CLOSE, draining those bytes still ends with an unexpected-closure error; transport
EOF alone does not establish successful JOIN or Service completion. Cancellation,
expiry, or consumption releases the retained bytes through joined cleanup.
EOF closes one direction. CLOSE retires the lane and joins both directions and
their readers/writers. Count JOIN, RESULT, frame headers, data, credit and terminal
traffic against original admission and aggregate bounds. Reserve before
allocation or effects; exhaustion refuses. Source JOIN and Introduction
submission run concurrently after their independent tokens are available and
durably marked. Source does not await JOIN acknowledgement before delivering
the capsule. Publisher independently validates the capsule and opens its
Responder prefix before JOIN. Existing end-to-end Service authentication must
complete before Application data is exposed.
Introduction keys and slots live for 600 seconds, with refresh at 300 seconds.
The predecessor accepts already bounded capsules for at most 60 seconds after
a replacement, never past its original signed expiry. Each delivery nonce has
a replay entry until that expiry plus 60 seconds. Reject replay, wrong
publication/revision, malformed capsule or invalid role before Publisher dial.

The current Service Connection wire records, ordered offsets, EOF,
acknowledgement and bounded continuity remain authoritative. The successor
changes only its context construction and Attachment composition. The Initiator
retains its local Isolation Context and destination authorization locally.
Compute initiator-binding as SHA-256(domain `ardents-initiator-binding-v3\0`,
fresh secret salt[32], existing local ConnectionContext[32]). The salt never
leaves that Endpoint. This per-Connection opaque commitment is not an identity
or authority granted by the Publisher; it prevents publishing a local context
identifier and binds recovery to the original local authorization.

Compute the shared immutable logical context as SHA-256 of the concatenation
of domain `ardents-service-context-v3\0`, Network[32], Target[32],
Instance-public-key[32], Instance-generation u64, publication-digest[32],
profile-digest[32], Connection-nonce[32], initiator-binding[32], and the three
u64 Work Safety bounds in the order above. Both endpoints obtain the Instance
and publication fields from independently verified publication facts. The
Publisher checks the capsule's Network/Target/publication and bounds against
those facts and its own live authorization before accepting; requester fields
cannot extend any local or public authority. Refuse incompatible bounds rather
than silently calculating a different context. Keep the complete accepted tuple
unchanged across recovery. A later capsule must match it and pass retained
continuity authentication; the nonce alone never locates authorized work.

For generation 3, coalesce initial Service authentication and continuity into
one authenticated request/response flight: after Service TLS, the client sends
InstanceChallenge followed by its initial Continuity; the Publisher verifies
the exact challenge/context and Continuity, then returns InstanceProof followed
by its Continuity. Client verifies both before exposing Application bytes.
The Publisher also enables Application work only after its receiving checks;
no early Application Data is introduced. Retain every existing signature, MAC,
role, offset, exporter and nonce check. The Connection owner returns an opaque
verified initial-state result to its stream lifecycle, never a caller-provided
established flag. Current generation 2 retains its old sequential composition
until migration; this generation-3 schedule changes no record encoding or
cryptographic primitive. The model's one Service-authentication RTT includes
this initial Continuity exchange. Later recovery keeps its retained continuity
authentication and original authority.

Separately compute the fresh Attachment context as SHA-256(domain
`ardents-attachment-context-v3\0`, logical-context[32],
SHA-256(capsule plaintext)[32], Attachment-generation u64). Use that context
for this Attachment's TLS exporter. Retain the logical context at the Service
Connection Interface and bind its authentication to the fresh exporter
commitment. The first accepted Attachment establishes the existing continuity
secret; later Attachment exporters cannot replace that secret or its original
work-safety authority. Recovery must authenticate under the retained continuity
contract before committing a replacement. Hashing a new join secret into the
immutable logical context would break recovery and is explicitly forbidden.
No Target or recovery secret is exposed in Carrier TLS metadata.

## State, diagnostics and failure

Persist only current owners' authority/conflict/withdrawal floors and the
necessary admission/replay journals. Keep channel keys, prefix contexts,
unused tokens, join secrets and Application contents volatile.
Introduction recipient keys do not derive from the Instance signing key.
A restart cannot resurrect a prior Connection or an expired publication.

Ordinary logs contain operation class, typed terminal reason and aggregate
counts only: no Name, Target, peer path, permission/token/nonce, capsule,
source address or unique Connection label. Keep local diagnostic counters
in memory for 24 hours, then discard; persistent floor/journal data has its
explicit purpose/expiry. Do not export it as telemetry or crash attachments.

The admission, publication and [qualification](../development/privacy-qualification.md)
owners supply exact lifetime and resource ceilings. A valid peer that withholds
work can force finite unavailability. No timeout detects all traffic-analysis
attacks, authorizes weaker protection or supplies a new endpoint identity.

The forwarding Node owns its retained Carrier readers through the same joined
lifecycle as accepted handlers. Cancellation interrupts outgoing reads, writes
and pending HELLOs before waiting for that tree. Only completed joining releases
the receiving spend root; a bounded Drain timeout reports failure while the
terminal owner retains the root. Physical-close and invalidation failures are
retained through repeated Drain. Closing a descriptor alone is not evidence
that its reader or final registry cleanup has completed.
