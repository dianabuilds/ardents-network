# Isolated Rendezvous JOIN transport

The new Route JOIN uses only new Network, Admission and Hosting, under the
[migration contract](../development/route-migration-contract.md) and
[protected protocol](protected-route-protocol.md). It yields protected framed
transport. Instance authentication, Publication readiness, capsule acceptance
and logical Service Connection remain separate responsibilities.

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
return, outside the parent locks. Acquisition, client exchange and joined stream
use portable mechanisms and execute the same local lifetime controls on Windows
and Linux. Context composition still uses the concrete native selection owner;
its durable-root dependency and Context tests remain Linux-specific. Holder
acquisition, attempt and stream live directly in `join`; `receiver` owns the
listener and dispatch. The intermediate operation package is removed.
Prefix is the actual physical-generation owner. Its pair-lock tests
remain there; acquisition tests retain caller and opening-join ordering without
accessing Prefix private fields.

A successful JOIN transfers stream and exact acquisition together. Retirement
seals all borrowers before waiting, joins them with original framing parents
still live for bounded terminal output, then retires the parents and returns
reservations. A Source also retains its published Responders and joins them
before its own physical parents. Idle readiness expires after 120 seconds:
the same joined retirement path completes before autonomous notification.
Repeated Close retains the original terminal result. Setup interruption joins
its callback before publication and preserves deadline-operation and Close
failures. Actual cleanup failure seals context admission and prevents reopen.

## Receiving pair and framed stream

`join.Pairing` owns the receiving pair directly. The actual Receiver calls
`Serve` with its original admitted channel, allowance and currentness check.
`ReserveCapacity` holds frame memory and a child position from the same receiving
principal budget before spend. Receiving composition returns this opaque capacity
only after physical join, alongside the original Hosting return; JOIN owns no
Grant, listener or durable root. Physical failures are retained through the
Receiver's bound recording callback before pair completion.

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

Unpaired setup ends at the earlier original request bound or ten seconds from
reservation. Paired data uses the original admitted parent/State/duty bounds,
64 KiB receive credit, BYTES at most 16 KiB and bounded queues. Setup, control,
headers, data and terminal traffic count toward the original allowance. CREDIT
returns only after consumption. Later ADMIT on dedicated JOIN is refused; it
has no refill reservation/spend path. Forwarding-parent refill remains a
separate full-Route obligation.

EOF closes one direction. Verified CLOSE stops new effects and joins both
directions. A receiving CLOSE interrupts payload and input immediately; a
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
