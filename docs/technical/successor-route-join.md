# Isolated Rendezvous JOIN transport

The new Route JOIN uses only new Network, Admission and Hosting, under the
[migration contract](../development/route-migration-contract.md) and
[protected protocol](protected-route-protocol.md). It yields protected framed
transport. Instance authentication, Publication readiness, capsule acceptance
and logical Service Connection remain separate responsibilities.

An original Responder's observer fully reobserves its exact Source before
returning. A physical pair uses that complete chain once, with original Source
caller checks before and after and an exact parent-pointer comparison. Source-only
JOIN still observes its own Source directly. Intent matching compares immutable
JOIN commitments before one full acquisition observation; comparisons themselves
perform no I/O or authority-granting effect.

## Selection and exact lifetimes

Route selection owns one installation Entry root and independent role Interior
borrowers. Closing a physical prefix returns its own reservation; closing the
Route context joins every original physical generation and then returns the role
borrowers. Installation composition closes Entry selection afterward. Hosting
uses one existing provider-period root and the same budget handle for both roles;
each prefix receives its own work/termination reservation. Hosting still owns
its durable accounting and transaction leases.

`selection.Rendezvous` chooses uniformly among eligible current data-join duties
after known Node/key/family exclusions. The initial choice and at most one
alternative are fixed together. The original validity, time floor and known
exclusions survive failed use and physical replacement. `DutyForLeg` verifies
the new actual leg against those retained choices; conflict or expiry refuses
without redraw. Only the initiating Source selects. A Publisher verifies the
exact incoming public duty rather than choosing a substitute.

`join.JoinContext` owns one explicit local Route role/configuration, its
role selections, retained Rendezvous and physical generations. Construction
grants no readiness. `Open` publishes a `JoinOpening` only after genuine admitted
Source and, for Publisher, original-Source-bound Responder are ready.
The opening retains those exact identities: its Recipient, Join and Close
cannot resolve or close a later generation. Closing a physical opening preserves
context selection; console EOF, cancellation or its original deadline closes
the context. This local console context grants no Execution or Service authority.

Responder setup registers with the original live Source before I/O. Source
sealing cancels pending setups and denies new acquisitions. Final setup
publication and JOIN stream handoff share Source-then-Responder locks with
synchronous sealing. Original caller cancellation is checked directly after
lock waits and observations, without relying on delayed cancellation callbacks.
Stocked tokens do not bypass these checks.

The actual `prefix.JoinBorrow` retains both exact physical identities, their
original currentness, retirement signals and terminal-channel opener. JOIN
acquisition retains its caller, one attempt, stream and physical result; it
does not read Prefix fields or resolve a new parent. Its joined notification
captures the exact original Context generation and runs only after both borrows
return, outside the parent locks. The opaque acquisition binds its original
handle identity: copying its fields cannot admit another JOIN, publish a stream,
cancel its caller or return its original Prefix claims. A joined stream checks
that same original acquisition and its immutable admitted lifetime before a
Service can borrow it; these checks grant no Service authority.
The joined owner also retains its actual admitted side, a local digest of the
pair secret, handshake context and both original deadlines. Source and Responder
binding checks compare those immutable facts with the exact capsule before
Service TLS. A live opposite-side stream or a different intent cannot substitute
for the original acquisition. These local comparisons add no wire field and
do not expose the pair secret.
Acquisition, client exchange and joined stream
use portable mechanisms and execute the same local lifetime controls on Windows
and Linux. Context composition still uses the concrete native selection owner;
its durable-root dependency and Context tests remain Linux-specific. Holder
acquisition, attempt and stream live directly in `join`; `receiver` owns the
listener and dispatch. The intermediate operation package is removed.
Prefix is the actual physical-generation owner. Its pair-lock tests
remain there; acquisition tests retain caller and opening-join ordering without
accessing Prefix private fields.

At each client exchange boundary, acquisition-local checks fence the complete
physical observation. Prefix checks each original leg and exact recipient on
the same fresh view; scoped claims also check their original logical owners.
JOIN then reobserves the original owners after that recipient validation.
No effect occurs between the leg and recipient checks, and the outer acquisition fence does not duplicate those
full observations. Each later exchange still obtains fresh observations; local
identity, cancellation and seal checks supply no accepting authority.
Publication's initial authentication checks its accepting candidate and exact
Responder before obtaining the local acquisition, then observes the complete
retained delivery immediately before the first JOIN exchange. Acquiring that
local claim opens no channel and supplies no Service authentication.

A successful JOIN transfers stream and exact acquisition together. Retirement
seals all borrowers before waiting, joins them with original framing parents
still live for bounded terminal output, then retires the parents and returns
reservations. A Source also retains its published Responders and joins them
before its own physical parents. Idle readiness expires after 120 seconds:
the same joined retirement path completes before autonomous notification.
Repeated Close retains the original terminal result. Setup interruption joins
its callback before publication and preserves deadline-operation and Close
failures. Actual cleanup failure seals context admission and prevents reopen.
Failed Service Attachment retirement marks its exact original Joined child
before cancellation and Close. That child emits the existing refusal CLOSE(1)
under the same bounds; it cannot provide a decoded clean-CLOSE witness to the
peer's settled control tail. Copied children and late disposition changes
refuse. Ordinary successful retirement still uses CLOSE(0), and both paths
join original framing and resource borrows before returning their result.
The watcher atomically selects that disposition before physical output. A
failed-retirement marker arriving after selection refuses even when the original
acquisition has not yet closed; it cannot claim to change an emitted terminal.

## Receiving pair and framed stream

`AcquireScopedResponderJoin` consumes an exact opaque `ResponderHandle` from
the original retained role scope. Prefix's scoped pair claim carries that
generation through observations, recipient checks, opening and final handoff.
It never follows a later generation. `Joined.CheckResponderScope` compares the
opaque scope provenance; equal public leg facts cannot substitute another owner.
Closing the setup handle after transfer cannot retire the actual joined stream.
Scoped JOIN observations retain local logical-owner/generation checks before
and after the physical pair's complete fresh observation. Initial and replacement
physical generations keep the same immutable leg and original observer as their
scope, so these public facts are not read again through the logical wrapper.
Responder's local fence checks its exact Source handle under Source-before-Responder
scope locks: a logically closed Source refuses immediately while physical users
are still joining. No accepting observation is retained or shared across effects.
The ordinary direct Prefix acquisition remains for the separate Route consumer;
it cannot satisfy this scoped Service provenance check.

`join.Pairing` owns the receiving pair directly. The actual Receiver calls
`Serve` with its original admitted channel, allowance and currentness check.
`ReserveCapacity` holds frame memory and a child position from the same receiving
principal budget before spend. Receiving composition returns this opaque capacity
only after physical join, alongside the original Hosting return; JOIN owns no
Grant, listener or durable root. Physical failures are retained through the
Receiver's bound recording callback before pair completion.
Receiving composition supplies that exact admitted lower lane's interruption
to Pairing. Retirement stops its readers and selected writer through the lower
owner, including when framing already stopped; it does not attempt another
deadline mutation on the retired lane. Actual interruption and Close failures
still reach the original pair and Receiver result. Raw transports retain their
physical deadline interruption.

Relay deadline changes use Channel's `SetFrameDeadline` result: the complete
operation refusal remains with JOIN, while only a call to its own physical
connection supplies its physical failure. A closed nested framing lane can
refuse without another native deadline call. That refusal does not invent a
receiving cleanup failure or reassign the lower owner's retained failure.
Actual native deadline and partial-write failures remain with their original
owners through join; this distinction grants no successful exchange or recovery.
For already selected CREDIT, the relay uses Channel's `BoundControlOutput` so
the original one-second cleanup bound reaches its actual nested physical writer.
An ordinary payload deadline cannot interrupt that control and cannot substitute
for its owning cleanup bound. No extra frame, credit or lifetime is granted.

Failed unpublished setup retains the same separation. The original
`prefix.JoinChannel.CloseSetup` joins its setup callback, partial connection,
lower lane and control return once, retaining the complete result.
`SetupCleanupResult` publishes only that original physical retirement after all
those joins. Acquisition keeps the full refusal for its caller and supplies only
the joined physical result to Prefix cleanup. Missing completion remains a
failure; this receipt creates no JOIN, new attempt or recovery authority.

`pair.go` keeps capacity, matching, setup and the two-RESULT barrier together;
`relay.go` owns activated frame accounting, credit and terminal joining. Their
tests are grouped by these lifetimes in `pair_test.go` and `relay_test.go`,
including original attachment cancellation and late opposite refusals. All
these sources run unchanged on Windows and Linux: they need ordered I/O,
context, time and synchronization, with no native storage mechanism. Controls
that also assert the actual Receiver's retained failure stay with receiving
composition; its concrete Admission/root dependencies still select Linux.
Holder acquisitions and Context now belong directly to `join`;
their real Prefix borrowing interface and consumer cutover are complete.

Each side authenticates fresh exact-recipient terminal TLS through its original
prefix and presents genuine class-2 stock. Admission durably marks holder
presentation and receiving spend; Hosting capacity and Route frame/child
capacity precede irreversible spend. Route does not issue or refund rights.

JOIN is one canonical 4096-byte operation 5 on local odd lane 1, without OPEN,
after lane-zero HELLO/ADMIT/ACCEPT. The receiving pair retains at most two
opposite sides with the same fresh secret, context and profile. Conflicting,
duplicate or third arrivals cannot replace a side. Each side has its own local
nonce and fixed 16384-byte empty RESULT. Both actual successful RESULT writes
must finish before either data reader starts.

Unpaired setup ends at the earlier original request bound or thirty seconds from
reservation ([ADR-0121](../adr/0121-bound-installed-connection-setup.md)). Paired data uses the original admitted parent/State/duty bounds,
64 KiB receive credit, BYTES at most 16 KiB and bounded queues. Setup, control,
headers, data and terminal traffic count toward the original allowance. CREDIT
returns only after consumption. Later ADMIT on dedicated JOIN is refused; it
has no refill reservation/spend path. Forwarding-parent refill remains a
separate full-Route obligation.

EOF closes one direction. Verified CLOSE stops new effects and joins both
directions. A receiving CLOSE interrupts payload and input immediately; a
first peer CLOSE received during local lane retirement also cancels any local
CLOSE that has not entered physical output. Its original status is retained;
later terminals cannot overwrite it or make local retirement a clean EOF.
A started physical terminal still retains its actual output failure. A
selected CREDIT may finish within the earlier original deadline and one second.
No new read starts after that write joins a sealed pair. A timed-out or failed
physical CREDIT remains failure and cannot produce later successful terminals.
Sealing before the opposite pump reserves its next header is ordinary local
retirement: no input has started, so it must not suppress the pair's terminal
frames. This private accounting outcome is distinct from currentness loss or
sealing after an actual payload header has been consumed. Payload failures,
malformed input and raw EOF still prohibit clean terminal output. Portable pipe controls
gate the pre-header observation to exercise this ordering without timing delays,
and retain opposite currentness loss and peer refusal separately.
An input CREDIT can also be fully validated before opposite CLOSE, then reach
its recipient's output accounting after the pair seals. Only the private
pair-lock refusal before physical output discharges that unemitted control.
It grants no new credit or payload effect after retirement. A delayed original
check failure, cancellation, or a started physical write is not that refusal.
The causal relay control gates this pre-output observation while an actual
opposite CLOSE is consumed and checks both terminal frames and zero CREDIT
output; its negative cases retain the original failure without terminal output.
An already started CREDIT also joins its bounded body when sealing occurs
between header accounting, physical reading and dispatch. The complete frame
must still be canonical, within its original allowance and earned receive
window; original caller and currentness are rechecked after body I/O. A valid
control observed after seal is discarded without increasing credit or starting
output. Zero/overflow controls, payload, cancellation and currentness loss
remain failures. Portable tests gate both actual header and body reads and
assert unchanged credit and zero output accounting on the retired recipient.
The client waits for the authenticated inner terminal before closing its lower
parent. The original Prefix terminal retains that lower lane and its control
capacity return; Joined holds the terminal rather than separate raw parent and
release fields. It asks for lower retirement only after the inner framing
reader/writer and terminal wait join. The real nested-pipe close tests execute
unchanged on Windows and Linux, including verified CLOSE, raw EOF, cleanup
timeout and the earlier original deadline. Previously accepted bytes survive raw EOF within their original bounds,
then end with unexpected closure when no inner CLOSE was verified.

## Command consumer

The Linux `ardents-next admission holder --config PATH` Route plan supports
Domain 1 Source or Domain 3 Publisher. Publisher additionally supplies an
independent `source_interior_root`; Entry, Interior, Source Interior, Hosting,
holder and Network roots are distinct and unnested.

- `join-prefix-open` lazily creates the console's fixed Route context and opens
  one genuine physical generation. A failed physical open does not manufacture
  a replacement context.
- `rendezvous` with `choice` 0 or 1 returns the initiating context's retained
  public Node/duty and original bound. Publisher refuses this operation.
- `join-open` takes `join` with `choice`, `node`, `generation`, fresh 32-byte
  `secret` and `context`, and whole-second `deadline`/`setup_deadline`. Route
  verifies the current retained/incoming duty and both original handles before
  presentation and final handoff.
- `join-close` joins the retained stream; `prefix-close` joins that exact
  physical generation. Another `join-prefix-open` uses the same context.
- Console close/EOF joins the Route context before holder and Network roots.
  Failed Close handles and context failures remain retained through repeated
  commands and terminal process exit.

The bounded console exposes no raw Application byte operation or Service
success. Unsupported platforms refuse before opening Route roots. Private pair
intent stays local; diagnostics contain fixed operation/phase/outcome only.

## Reproducing evidence

`Joined.CommitOriginal` observes the genuine retained parents before entering
their Source-before-Responder barriers, then takes the original Acquisition
mutex for a bounded local handoff. It compares the same published stream,
callers, admitted child deadline and original parent seals. The consumer may
compare its own local authentication/authority state there; it must perform no
foreign observation, I/O or join. Connection uses this barrier for initial
record-actor ownership transfer, without obtaining another JOIN or extending
its admitted lifetime.

The original Acquisition and Joined child expose a read-only `CleanupResult`
only after actor completion, borrower return and original completion notification.
`Close` still retains EOF/refusal and every physical error. The separate receipt
contains actual owned physical write/close failures, not an unsuccessful exchange
or missing peer Terminal. Connection and Instance use it for original Execution
cleanup accounting; a failed Attachment cannot manufacture another Job, while
actual cleanup failure still denies effects. `InterruptIO` interrupts this exact
child and performs no new physical deadline operation once framing is already
stopped. Neither interface supplies successful JOIN, Service or recovery authority.

The exact lower framing lane separately retains `CleanupResult` after its Close
has joined the original writer. `JoinChannel.ParentCleanupResult` additionally
requires the original control hold to return. JOIN retains the full parent Close
outcome separately from this physical receipt. A refusal before physical output
does not fabricate an upper write failure; a started write or actual close or
resource-return failure remains with its original owner. A lower owner's earlier
failure is not reassigned to an upper Close that emitted no new failing output.

`Joined.DecodedPeerClose` projects opaque evidence from the exact dedicated
child's canonical decoder after clean CLOSE(0). It identifies one original JOIN
and grants no Service authentication, active channel, recovery or successful
cleanup. Local closure, directional/raw EOF and refusal cannot mint that fact;
copied or foreign streams cannot consume it as their own. Connection separately
checks complete authenticated Terminal state, absence of additional plaintext
and original logical authority before using it for bounded control-tail
retirement. Physical cleanup still joins and retains its actual result.

Reader's `Joined.CheckSourceCaller` requires the original scoped Source borrow,
matching caller, Network/profile and absolute bound. It checks that provenance
and local seals before and after the full original acquisition observation.
Physical generations retain that Source scope's exact leg and authentic
observer; this operation observes those facts once and grants no detached or
cached accepting result. A foreign scope, replacement borrower or copied stream
cannot reuse the match.

`make route-check` runs the complete new Network/Admission/Hosting/Route and
command race profile plus architecture isolation/profile checks. Both selected
Carriers, local durable filesystem, counters, loopback and race compiler are
required; missing prerequisites invalidate execution. Repository quick/full
gates remain separate pre-integration requirements.

Genuine command scenarios use signed current Network, independently signed
permissions and blind token batches, separate stock/spend roots and real Hosting.
They exercise encrypted 96 KiB traffic, compiled paired consumers, stocked
original-Source loss, cancellation after real presentation, unchanged choice
across physical reopen, exact old-handle refusal and failed-close retention
through EOF. Ephemeral mutually pinned fixture TLS proves the stated opaque
transport/crypto boundary, never Service Instance authentication.

Transport-local real-pipe controls cover the RESULT barrier, retained input,
wrong/conflicting arrivals, deadlines, selected CREDIT cleanup, partial terminal
failure, cancellation/join/release, callback failure and exact role-lock handoff.
These mechanical fixtures supply no successful neighboring authority. Source
identities, commands, preserved failures and results belong in the selected
GitHub issue. None of this establishes complete Route, opaque recipient ACK,
Publication, installed qualification or privacy qualification.
