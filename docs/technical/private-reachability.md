# Private Target reachability

Status: **current generation-3 private Descriptor and receiving-Store contract
under ADR-0081; installed qualification remains pending.** Node Resolution
calls `Store.PublishPrivate` and `LookupPrivate` on the selected protected
Route. ADR-0091 retired the unwired generation-2 OHTTP Relay/Gateway/Client
adapter. The old-format Store decoder and conflict floors remain pending a
persisted-root decision; the short generation-2 note below records that
obligation without defining a second live lookup route. ADR-0036 and ADR-0037
record its accepted origin.

The current [protected protocol](protected-route-protocol.md#terminal-payloads-and-private-reachability)
preserves proof/currentness and conflict floors with a generation-3 private
Descriptor. The retained generation-2 evidence below does not define the
current transport or exposed join fields.

## Private Descriptor recipient

The generation-3 codec binds the complete existing Publication to the current
State profile digest, a positive revision, one Introduction Node, random slot,
independent recipient HPKE public key and a whole-second validity interval of
at most 600 seconds within the Credential. Its Instance signature is the exact
transcript in the [protected protocol](protected-route-protocol.md#terminal-payloads-and-private-reachability).
The proof contains no data Rendezvous, reusable join or origin address.
`VerifyPrivate` requires the independently selected Target, Network and current
profile; the legacy `Verify` cannot accept this format without those checks.

The State-selected resolution Node receives publication and lookup through a
State-authorized Node Carrier and a fresh confidential terminal role channel.
It requires an actual class-1 admission before one operation, verifies the
Introduction assignment against current State, and invokes the real Store.
Every RESULT occupies 16,384 bytes. Descriptor commit does not prove that its
claimed slot is registered or that a Publisher is ready; registration,
publication acknowledgement and readiness must still be joined by the owning
Endpoint lifecycle.

The Endpoint Administration context owns a separate Introduction-domain Entry/
Interior prefix alongside its Source prefix. Its real issuer client supplies
class-2 forwarding tokens and the class-3 registration token; neither the worker
nor the Introduction Node selects these recipients. Registration owns a fresh
random slot, explicit revision and bounded expiry. WITHDRAW joins the terminal
result and channel cleanup; context loss cancels and joins registration, pending
withdrawal and both prefixes. A failed exact issuance batch remains retryable
without new blinding or allocation; a foreign pending batch cannot be replaced.
The same context now consumes the accepted non-exporting Instance binding and
real REGISTER receipt to commit the existing public Publication proof. Its
Instance root creates an independent volatile X25519 recipient only after the
Instance is durably consumed. Revisions strictly increase in that live Instance;
a restart cannot revive its signer or reset the floor into another live key.
The context signs the exact private Descriptor and sends it through its Source
prefix with real Control issuance and journal admission. The resolution Node
commits its actual Store before acknowledgement. An exact retry retains the
same signed bytes and key; explicit replacement uses a new registration,
recipient and higher revision within the same Publication generation.
Registration completion erases its recipient and joins that erasure on Close.
Context loss additionally withdraws its local Publication and Instance after
joining work. Cleanup ownership precedes a cancellable publication handover;
failed withdrawal retains the binding and original error and closes admission.
Another context cannot take over the selected Instance.
The context schedules refresh from the original registration creation time,
retains a fresh slot/key and strictly higher revision, and switches accepting
readiness only after the exact Descriptor receives a verified acknowledgement.
While publication waits on the network, the context reserves its Instance
without holding the Endpoint publication mutex. Legacy publication operations
cannot take that reservation. The prior published registration remains usable
until that switch, bounded by its original signed expiry. The registration retains the time of its first verified publication acknowledgement;
an exact retry cannot replace this local transition receipt. The switch shortens
its remaining acceptance to at most 60 seconds; an exact retry cannot extend
the cutoff. A live Connection recovery resolves the same Target again and can
replace only its retained Introduction recipient with a monotonic revision
bound to the same Publication and profile; this changes no logical Connection
authority or work deadline. Unacknowledged replacements cannot accept capsules. Withdrawal and
context loss cancel and join the scheduler, registrations and recipient erasure.

These paths have real Node-network coverage on both Carriers, including old
capsule delivery while a replacement acknowledgement is delayed and refusal of
a new capsule until that acknowledgement. Accelerated scheduling does not prove
elapsed 300-second refresh or 60-second overlap trials. The complete installed
command/readiness transaction and system qualification remain required;
Descriptor acknowledgement alone is not Service Connection readiness.

The Endpoint text context can fetch a Descriptor through its retained admitted
source prefix. It derives the sole resolution recipient from live State, obtains
a real receiver-bound Control token through the issuer, records its attempt in
the context's journal and uses a fresh terminal TLS channel. The returned proof
is checked against the independently selected Target, Network and current
profile. Cancellation joins the operation before context shutdown completes.
Before returning a proof, the live context retains its publication generation,
publication digest, terminal Credential expiry, Descriptor revision and exact
Descriptor hash. It rejects lower generations/revisions and retains separate
publication and revision conflicts. A higher revision can repair only a revision
conflict; a new generation must not overlap any observed conflicting Credential.
The context stores at most 128 Target floors, without eviction or shared private
history. A full context refuses a new Target before issuance or admission;
existing Targets may still advance. Worker loss retains the floors; context
retirement clears them. Expiry never revives a predecessor. No Descriptor bytes
are cached for offline use, and these volatile context facts do not replace the
Store's durable floors or prove restart/migration qualification. The complete
command's protected read remains required; a lookup is not completed Connection
admission. The lookup-only network tests use explicit Publication and
Introduction-slot fixtures.
`PublishPrivate` and `LookupPrivate` retain the existing publication-generation,
no-overlap and same-generation Publication-conflict rules. Within one exact
Publication a higher revision replaces the current revision, including when its
expiry is shorter; a differing proof at the same revision persists as
unavailable. Only a strictly higher revision can repair that revision conflict.
For conflicting Publications at one generation, the Store retains the complete
signed Descriptor whose Credential has the latest observed `NotAfter`, together
with the terminal publication-conflict flag. Further valid conflicts can extend
this expiry floor even after lookup becomes unavailable; a shorter conflict
cannot reduce it. Reopening reconstructs the same signed floor, and a higher
generation must start at or after that expiry. This preserves the existing
record format; previously discarded observations cannot be reconstructed from
an older root and still require explicit adoption evidence.
Expired records retain their floors, never exposing a predecessor. The Store
retains at most 128 Targets, refuses additional Targets before writing, and
continues to permit updates of existing Targets without evicting their floors.

Private stored records use version 2 with separate publication/revision
conflict flags; existing v1/v2 Descriptor records retain stored version 1.
There is no implicit legacy/private format adoption. Before acknowledgement,
the Store syncs the record and containing directory. Initialization also
syncs directory links and creates the marker only after the records directory
is durable. A marked root with missing records, or an unmarked root with
retained records, refuses reopening. Any failed record persistence makes the
current owner unavailable until closed and reopened; it retains its exclusive
lease in the meantime. This does not claim that a storage device survives
failures beyond its filesystem's sync guarantees.

## Recipient-only capsule composition

The reader context now consumes its actual lookup and retained conflict floors
to prepare a capsule for one current qualified job. It selects the data-join
duty from authenticated State through its admitted Source prefix; neither the
Descriptor nor Application supplies a Rendezvous address. The selected Node,
key and known family must differ from the local adjacent peers, resolution,
Introduction and issuer duties. Join secret,
handshake context and delivery nonce are independently generated. The selected
HPKE suite seals the exact fixed plaintext, and its hash supplies the existing
Attachment exporter context.

Before any Publisher dial, the Administration context checks its channel-owned
registration, current private Descriptor and Publication, and opens the capsule
through the Instance's current non-exporting recipient. It independently checks
Target, publication, revision, profile, data-join duty, local job and Work Safety
bounds before constructing the same logical Service context. It rechecks caller,
job and the original capsule expiry after binding, before accepting the nonce.
Reader preparation inherits the job lifetime, interrupts its network flights on
retirement, and joins caller cancellation before handing over capsule bytes.
Opening attempts are limited to four in any second; accepted delivery nonces remain until the
original registration expiry plus 60 seconds, bounded to 2,640 entries per
context. Worker loss does not erase those entries; context retirement does.
The Source submits the capsule over a fresh class-1 Introduction Control channel.
Introduction generates a channel-local request nonce and sends the unchanged
sealed capsule through the Publisher's actual class-3 registration. That owner
bounds queued and active deliveries to 16, limits starts to four per second,
and reserves operation/RESULT/CLOSE plus withdrawal against its original 1 MiB.
Publisher acknowledges after independent capsule acceptance and joins the
terminal child CLOSE before handing over the binding. Both Endpoint exchanges
are job-scoped operations that context retirement joins; failed Source cleanup
is retained and disables new local admission.

Withdrawal disables the receiving slot and closes its completion signal; it
retains the inactive entry to prevent slot reuse. That entry includes the
registration request, rate and byte accounting, and the closed connection
object. Expired entries are reaped when a later registration is reserved, or
released when the receiver itself is discarded. Withdrawal alone is therefore
not a claim of immediate metadata or connection-object erasure. Receiver-state
observations must include these inactive entries as well as pending deliveries.
The fixed format, network submission/delivery and this composition are tested
with real issuer tokens, both Carriers, Instance HPKE and Service TLS. Data
JOIN and the complete installed command
remain required integration. Delivery acknowledgement is not a joined
Attachment or Service readiness.

## Retained generation-2 Store evidence

[ADR-0036](../adr/0036-target-private-reachability-v1.md) and
[ADR-0037](../adr/0037-private-reachability-entry-carrier.md) record the
former OHTTP lookup and Descriptor authority. [ADR-0091](../adr/0091-retire-uncomposed-legacy-artifacts.md)
retired its unwired Client/Relay/Gateway adapter and GatewayProfile codec.
The former Endpoint-to-Initiator-to-Gateway operation is not a maintained C0
lookup route; its detailed implementation remains in Git history.

The current Store still reopens stored-record version 1 and authenticates its
signed Descriptor v1/v2 bytes. These records retain per-Target Credential
generation, Publication digest, expiry and conflict floors. They share the
128-Target root with private v3 records and can prevent a same-Target v3
publication; the current private lookup does not return them. The uncalled
legacy `Issue` and `Store.Publish` writers are retirement candidates, while
the decoder and floor comparison remain until an authenticated adoption or
explicit refusal/new-Target policy preserves the existing root's authority.
Neither historical bytes nor this temporary reader authorize a second runtime
version. The exact restart consequence and removal gate are tracked in the
[architecture finding](../development/repository-reconstruction-findings.md#f-32-legacy-descriptor-issuance-and-persisted-decoding-have-different-fates).

## Non-claims

This is not Namespace resolution, DNS, a public directory, an ordinary
descriptor server, a Publisher-origin hiding guarantee against colluding Node
operators, protection against timing/volume correlation, or a guarantee that
the resolved Service stays available. It does not add browser isolation,
content safety, application authorization, replication, offline delivery, or
generic Internet proxying.
