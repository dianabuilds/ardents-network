# Horizon 3 Stage 5 implementation brief

Status: **accepted by the Product Owner on 2026-08-16; maintained Stage 5
development completed on 2026-08-19; final qualification deferred to S9.6**

Authoritative inputs: accepted ADR-0005, ADR-0009, ADR-0012, R-009, R-023,
R-028, R-032 through R-038, the product contract, threat model, operating
model, H3 technical design, package map, dependency register, and repository
rules. The Product Owner accepted R-037 option O1 and profile
`h3-s5-b1-v1` on 2026-08-16, then explicitly accepted this implementation
brief on 2026-08-16.

That explicit acceptance authorizes S5.1 through S5.5 in order. It
does not authorize public distribution, a release claim, a new Go module
dependency, an unpinned runtime download, or any broader Stage 5 scope.

## Entry gate and completion level

R-033, R-034, R-035, R-036, and R-037 are decided. R-036 selected standalone
WebTunnel `v0.0.6` at commit
`d729fde1f38357dcefa2a751eb4752e9ca78f910`; its client/server binaries and
goptlib closure remain external pinned supply, never files committed to Git.
The Stage 4 local development topology, real Route roles, exact-Target Service
Connection, recovery, resource behavior, and Application byte stream are the
unchanged base.

Stage 5 has one development completion level. S5.1-S5.5 must establish the
maintained product behavior, bounded negative/resource/lifecycle checks,
independent-verifier seam, and the frozen `h3-s5-b1-v1` qualification contract.
The complete reference/stronger, sustained, pressure, hostile, recovery, and
cleanup campaign is not a Stage 5 completion condition. It runs once against
the cleaned integrated H3 candidate in S9.6.

Neither level is Ubuntu/Windows product qualification, public Bridge supply,
real-censor validation, anonymity, invisibility, independence, or production
censorship resistance. Official deferred Stage 1-4 qualification gates remain
deferred and are not silently closed here.

## Outcome and unchanged seams

One client Endpoint imports one authenticated expiring local-file Bridge Invite.
The unchanged Route asks for one endpoint-adjacent Carrier Channel. In the
precommitted blocked condition, transport-neutral Bridge state changes once
from ordinary entry to Bridge entry and returns either an already-open opaque
Carrier Channel through the selected WebTunnel Adapter or one bounded terminal
failure. The existing Route, Service Connection, exact Target authentication,
recovery, Application Interface, and byte semantics do not change.

No Service Connection or Application value gains Bridge identity, Invite,
Entry Set, Adapter, candidate configuration, contact/retry, regime, topology,
or recovery controls. Route receives only an owned `net.Conn`-compatible
channel and a bounded internal terminal class. It never imports WebTunnel
configuration or selects an Adapter.

## Maintained Module ownership

The design deepens the existing Route acquisition seam, adds two deep product
Modules, and adds one thin Bridge command while extending the existing thin
Route command as the client composition root. It does not add a generic
transport, plugin, interface, or utility package.

### Existing `internal/route` first-Carrier seam

The client `route.Actor` gains one optional narrow function value that opens
the first raw endpoint-adjacent Carrier Channel for the supplied selected
Initiator endpoint, context, and absolute deadline. When absent, the existing
literal-IP TCP dial remains the ordinary adapter and all Stage 1-4 callers
behave exactly as before. For a Stage 5 client, `cmd/ardents-route` supplies the
second adapter by composing `internal/bridge` with `internal/camouflage` in the
same process; the original context and monotonic deadline therefore cross no
IPC or serialization seam.

Route still performs the existing TLS 1.3 authentication and Network/Epoch/Node
leg binding for the exact selected Initiator **after** receiving the raw
channel. The opener cannot claim authentication, change the selected Route, or
return a shorter path. `internal/route` and `internal/routeplan` import neither
Bridge nor Camouflage packages and receive no candidate configuration. The
function value is the complete seam; no exported Go `interface` is added.

### `internal/bridge` — transport-neutral Bridge state

This new package owns the complete R-035 state machine behind one small
Interface:

1. open and revalidate one exclusive state root against current authenticated
   network/domain/conflict facts;
2. atomically import one bounded Invite through one supplied candidate-envelope
   validation function;
3. execute one ordinary-to-Bridge attempt through one supplied contact-opening
   function; and
4. close, persist, erase expired secrets, and report bounded observations.

The package uses concrete request/result values and narrow function values at
the two seams. It does not export a Go `interface` type merely for tests. Its
Interface hides slots, replacement, regime, exposure, contact order, retry,
deadline clipping, crash recovery, generation files, and cleanup bookkeeping.
Callers cannot request an ordinal, reset a ledger, select a member, or mutate a
stored record.

`internal/bridge` may import `internal/network/state`, `internal/resource`, and
the standard library. It must not import `internal/camouflage`, `internal/route`,
`internal/serviceconn`, any lab package, or candidate code. Candidate bytes
remain signed, opaque, and length-bounded. The package persists only the opaque
envelope and its validation commitment; every open/restart revalidates it
through the supplied validator before new work.

### `internal/localroles` — endpoint-local duty truth

This new package owns one Endpoint-local, owner-only role-state root used by
every maintained Direct Source, Node, Route-selection, and Bridge command. Its
bounded read-through Interface atomically replaces one producer's complete duty
set only when it is conflict-free against other non-Initiator producers,
removes a producer after terminal cleanup, and answers identity/family conflict
queries from the latest durable generation. Plans name only this root;
they cannot assert a conflict-free result or supply editable identity/family
exceptions.

Each retained duty binds producer ID, authenticated Node identity, canonical
family digest, exact duty class and lifecycle state, and finite `not_after`.
There are at most `32` duties and `16` producers in `64 KiB`. Expired duties are
ignored and removed on the next successful write. A crashed producer therefore
fails conservatively until its precommitted bound; restart cannot reset the
generation. Ordinary Initiator Entry membership is explicitly nonconflicting,
while Direct Source Exposure, Interior, Rendezvous, Responder, Introduction,
Destination Resolution, prepared/quarantined non-Initiator duty, and live
non-Initiator Route state conflict.

The maintained source client/server, Node lifecycle, and authenticated client
Route selection update this owner before relevant work and clear only after
terminal cleanup. Bridge import holds the same local-role root generation lease,
reads it for every validation, and fails closed if the root is absent, stale,
unreadable, over-bound, or cannot be locked. The package imports the standard
library plus the already reviewed, Windows-only `golang.org/x/sys/windows` ACL
surface and exposes no generic role registry or network surface.

### `internal/camouflage` — selected Adapter implementation

This new package owns the accepted R-036 Adapter seam and only:

- pure validation of the WebTunnel candidate envelope into one immutable
  concrete configuration;
- client subprocess startup, PT v1 readiness, one SOCKS5 request, and return of
  one opaque owned Carrier Channel;
- the Bridge-side standard-library TLS/HTTP front and pinned WebTunnel server
  subprocess, forwarding only the endpoint-adjacent leg; and
- idempotent cancel, shutdown, evidence hashing, state removal, and residual
  verification inside the accepted `5 s` startup and `6 s` cleanup bounds.

It exposes concrete validate/open/serve operations, not a multi-candidate
registry or exported plugin Interface. Candidate-specific arguments and PT
transcripts remain inside this Module. It never selects a Bridge, Role Domain,
regime, retry, deadline, Route, Target, or fallback.

`internal/camouflage` imports only the standard library. The production Module
does not import experiment code or a Go WebTunnel library. It executes the
absolute manifest-bound external client/server binaries with the exact R-036
sanitized environment. DNS, ambient proxy variables, runtime fetch, cgo,
`unsafe`, implicit `init`, and first-party cryptographic primitives remain
absent.

### Thin command composition

The new `cmd/ardents-bridge` has two explicit modes:

- `import <plan>` reads one bounded Invite file and invokes the Bridge owner;
- `serve <plan>` runs one Bridge-role TLS/WebTunnel front and connects only to
  the manifest-bound next Initiator leg.

`serve` composes the existing `internal/node` lifecycle with Camouflage: current
authenticated Network State and the exact Bridge-domain assignment must be
ready before the front starts; revocation, conflicting duty, PROTECT/DRAIN,
Work Safety, and terminal cleanup remain Node-owned inputs. Camouflage receives
only the already-authorized server configuration and next-leg target. It cannot
grant itself a Node role or keep serving after the Node lifecycle stops it.

The existing `cmd/ardents-route run <role-plan>` accepts one optional
`--entry-plan <bridge-entry-plan>` only for a client actor. The command opens
the Bridge owner, composes the candidate validator/opener, and supplies the
narrow raw-Carrier function to Route. It receives the Owner/policy transition
over one inherited local control pipe. The exact frame binds schema, attempt
ID, trigger class, policy ID, monotonic offset, and manifest commitment.
Silence, socket error, traffic failure, Adapter failure, or Route failure cannot
synthesize that frame. Both commands emit bounded JSON observations but never
`pass`.

`cmd/ardents-route` and `cmd/ardents-bridge` are the maintained non-test callers
of the two new packages. `internal/route`, `internal/routeplan`,
`internal/serviceconn`, `internal/serviceendpoint`, and the Application
Interface receive no Bridge or candidate imports. Existing Route command
behavior remains byte-for-byte compatible when `--entry-plan` is absent.

The implementation change that first adds these directories also adds each
`doc.go`, behavior tests, exact package-map imports, command ownership, and the
real command caller. No empty or placeholder package is permitted.

## Frozen laboratory encodings

These encodings are H3 fixture formats, not public wire protocols. All integer
fields are unsigned big-endian unless explicitly signed. Every decoder rejects
short, overlong, trailing, duplicate-by-construction, non-canonical, or unknown
input before allocation or state mutation.

### Bridge Invite file `ardents-h3-bi1`

The file is at most `4096` bytes and contains exactly:

1. ASCII magic `ardents-h3-bi1`;
2. one `uint16` signed-body length;
3. the exact signed body; and
4. one `64-byte` Ed25519 signature, followed by EOF.

The signed body contains, in this exact order:

1. `uint16 schema_version = 1`;
2. `32-byte network_id`;
3. `uint64 epoch_number` and `32-byte epoch_digest`;
4. one `uint8` length plus `1..63` ASCII route-profile bytes;
5. `uint8 role_domain = 1` for Initiator;
6. `32-byte bridge_identity`, `32-byte family_id`, and
   `32-byte node_record_digest`;
7. one `uint16` length plus `1..512` exact domain-proof bytes;
8. signed `int64 assignment_not_after`, `not_before`, and `not_after` Unix
   seconds;
9. `uint8 slot_generation`, `uint8 slot`, and `uint8 replaces_present`;
10. exactly `32` replacement-ID bytes only when `replaces_present = 1`;
11. one `uint16` length plus `1..1024` opaque candidate-envelope bytes; and
12. `32-byte issuer_key_id`, followed by signed-body EOF.

For this fixture, `family_id` is SHA-256 of the authenticated canonical family
bytes, `node_record_digest` is SHA-256 of the authenticated canonical Node
Record, `domain_proof` is that record's canonical current-Epoch
materialization, `assignment_not_after` is the authenticated Epoch assignment
bound, and `issuer_key_id` is SHA-256 of the Node Record's Ed25519 public key.
The importer derives all five values from current authenticated Network State;
the operator plan cannot assert them.

The signature input is ASCII
`ardents-h3-bridge-invite-signature-v1` plus one NUL byte plus the exact signed
body. The Invite ID is SHA-256 over ASCII
`ardents-h3-bridge-invite-id-v1`, one NUL byte, and the same body. Tests freeze
one positive golden file and one mutation at every field and length boundary.

### WebTunnel envelope `ardents-h3-wt1`

The opaque candidate envelope is at most `1024` bytes and contains exactly:

1. ASCII magic `ardents-h3-wt1`;
2. `uint8 version = 1`;
3. `uint8` length plus exact ASCII profile `webtunnel-v0.0.6`;
4. four raw global-unicast IPv4 octets and one nonzero `uint16` TCP port;
5. `uint16` length plus a `1..512` byte canonical HTTPS path beginning with
   `/` but not equal to `/`, with no query or fragment;
6. `uint8` length plus a `1..253` byte lowercase ASCII TLS server name with
   valid nonempty DNS labels and no trailing dot; and
7. one `32-byte` SHA-256 certificate-chain pin, followed by EOF.

Loopback, unspecified, private, link-local, multicast, IPv6, textual IP input,
userinfo, query, fragment, empty/root path, invalid TLS label, zero pin, another
profile, and trailing data return `adapter-config-invalid`. The TLS name is
used only for SNI/Host/certificate verification and is never resolved.

## Durable state and finite clocks

The Bridge state root is outside the repository, owner-only, and at most
`256 KiB` across current, previous, and temporary generations. Atomic commit is
write-new, file sync, atomic rename, and directory sync. A single exclusive
lock prevents two owners. Restart verifies generation monotonicity and all
authenticated facts before exposing state.

The sole attempt uses the accepted `64 s` absolute deadline, four starts in
slot-0 initial/retry then slot-1 initial/retry order, `15 s` contact bounds,
exact `1 s` post-cleanup spacing, `5 s` Adapter startup, and `6 s` Adapter
cleanup. Every child bound is clipped by Invite, Epoch, assignment, Work Safety,
cancellation, DRAIN, and parent deadline. Recovery additionally retains the
original R-032 `15 s` terminal deadline and its exact clipping cell. No wall
clock or restart resets a monotonic evidence offset.

The package retains at most four member records, four contact records, one
attempt, and one regime record. Secret endpoint/configuration bytes are erased
after terminal Epoch cleanup; bounded IDs, generation, exposure, and terminal
reason remain only as required to reject replay.

## Runtime supply and privileges

Before S5.2 code is integrated, `docs/development/dependencies.md` records the
external WebTunnel client/server and goptlib source identities, licenses,
advisory policy, binary hashes, offline build procedure, supported platform,
and removal/update owner. This is an external-process dependency entry; it does
not add a root `go.mod` requirement.

Source archives, build roots, Go module caches, binaries, TLS keys, Invites,
candidate state, captures, and generated evidence remain outside Git. The
runtime image verifies the accepted binary SHA-256 values before candidate
startup and performs no network download. Candidate children run as an
unprivileged UID with no capabilities, no cgo/`unsafe` exception, no ambient
proxy/DNS configuration, bounded cgroup/process/FD/socket/state limits, and a
read-only image plus one owned state directory.

Any source, binary, module, license, image, or command hash mismatch is
`invalid`; it is never repaired by an in-campaign download or version change.

## Vertical TDD slices

Every slice begins with a failing behavior test at the declared Interface,
implements the smallest vertical path, runs `make quick-check`, and preserves
all prior behavior. Tests use real files, sockets, subprocesses, deadlines, and
public Module Interfaces; no production test hook, exported fake, or state
inspection surface is added.

### S5.1 — Invite import and Bridge owner

- add `internal/bridge`, the pure WebTunnel-envelope validator in
  `internal/camouflage`, `internal/localroles`, and `cmd/ardents-bridge import`
  together so the
  transport-neutral Bridge can validate its opaque envelope through the
  accepted Adapter seam;
- connect existing Direct Source, Node, and authenticated Route-selection
  owners to the same bounded local-role root before Bridge import can succeed;
- freeze both golden encodings and strict bounds;
- exercise valid/idempotent import, every invalid field, expiry, wrong
  network/Epoch/profile/domain, role/family conflict, two slots, one
  replacement, replay, atomic crash points, restart, and secret erasure;
- prove failed import changes zero bytes of durable logical state; and
- update package map and command tests in the same commit.

Exit: the maintained command can import and reopen one valid fixture, all
R-035 state negatives pass, and no process/network action occurs during import.

### S5.2 — WebTunnel Adapter carrier

- deepen `internal/camouflage` with its runtime carrier, add
  `cmd/ardents-bridge serve`, and add the optional client entry-plan
  composition in `cmd/ardents-route`;
- register and verify offline supply before process startup;
- test pure config validation, sanitized environment, PT v1 transcript,
  SOCKS5 grant/refusal, malformed control, stdout/stderr quarantine, startup,
  cancellation, repeated close, SIGTERM/SIGKILL escalation, and residue;
- run one Linux container client/server useful-work cell through the real pinned
  WebTunnel binaries and standard-library TLS front; and
- prove zero candidate DNS and exact numeric dial target with positive controls.

Exit: one contact yields one opaque channel or one bounded Adapter result, with
no hidden retry/fallback and cleanup within `6 s`.

### S5.3 — Same Route and Target through blocked entry

- compose the Bridge opener into the existing client Route entry without new
  Bridge/candidate imports in Route, Service Connection, or Application
  Modules; Route must authenticate the same selected Initiator over the returned
  raw channel before useful work;
- run C0 ordinary control and C1/C2 WebTunnel success using the same Stage 4
  Route, exact Target, Service Connection, request/response, and canaries;
- add C3 exhaustion, complete C4 ordinal sequence, C5 uninformed probes, and C6
  informed-probe limitation with the accepted sample floors;
- prove the one authenticated transition, retained exposure, deadline clipping,
  recovery-parent terminal cell, and explicit Connection Result; and
- observe E, Bridge front/server, and next-leg egress with no DNS, proxy,
  ordinary/direct/alternate/shorter/weaker fallback.

Exit: short deterministic maintained E2E tests carry the accepted bytes on the
same Route/Target or fail exactly; all forbidden paths remain zero.

### S5.4 — Hostile state, evidence, and independent verifier

- add `internal/lab/blockedentry` and thin `cmd/blocked-entry-lab` for immutable
  topology, fault injection, collection, cleanup, and canonical evidence;
- add separately built `internal/lab/blockedverify` and
  `cmd/blocked-entry-verify-lab`; the verifier imports neither harness nor
  candidate packages and never receives a candidate-authored verdict;
- implement all nine R-037 hostile groups, private canary corpus, exact
  candidate-`fail` versus harness-`invalid` precedence, observer positive
  controls, residual inventory, and verifier replay/tamper tests; and
- keep raw secrets/captures outside Git while publishable evidence retains only
  safe commitments and bounded aggregate observations.

Exit: command E2E proves `pass|fail|invalid` recomputation, mutation coverage,
fail-closed missing/ambiguous evidence, and zero owned residue.

### S5.5 — Bounded resource evidence and qualification handoff

- implement bounded behavior tests for capacity admission/refusal, sustained
  cadence and active-window aggregation, pressure transitions, process
  lifecycle, cleanup, immutable file handoff, and campaign-monotonic clocks;
- keep the harness/verifier split fail-closed and prove `pass|fail|invalid`,
  mutation, replay, missing-evidence, and residue behavior at their declared
  command and Module Interfaces;
- freeze the complete `h3-s5-b1-v1` evidence suite: one `564`-cell candidate
  campaign plus six independently verified five-episode evidence-integrity
  campaigns, resource/evidence schema,
  stand reservation inputs, the required runtime-attestation shape,
  supply-lock requirements, and independent-verdict rules without
  weakening any R-037 threshold;
- implement every scheduled worker and the complete capacity, recovery,
  telemetry, hostile-binding, and raw-to-verdict reduction path so a frozen
  campaign can run without an unimplemented product-cell branch; the collector
  that derives runtime CPU-set/cgroup/network-namespace attestation from the
  allocated stand belongs to S9.6; and
- hand only execution of the full reference/stronger, ten-minute sustained,
  P0-P4, C0-C6, hostile, recovery, and cleanup campaign to S9.6, where it
  qualifies the cleaned integrated candidate rather than an earlier Stage 5
  snapshot.

Exit: bounded maintained tests and repository gates pass, every final worker
and reducer is implemented and fail-closed, and the only deferred inputs are
actual campaign execution, runtime allocation attestation, plus identities that
can be known only after the S9.6 stand is frozen. No Stage 5 development result may be described as a
qualification pass.

## Test and review gates

While implementing:

- targeted unit and command E2E tests at each Module Interface;
- Linux-only process, signal, socket, DNS-observer, and residual tests with an
  exact documented command/environment;
- `git diff --check` and `make quick-check` for each slice;
- dependency/source/license/advisory review before S5.2 integration;
- `make check` before every scoped integration commit; and
- final two-axis Standards/Spec review against this brief and the accepted
  records, with no unresolved high-confidence finding.

The full Docker campaign is not a Stage 5 test. S9.6 must run it against the
post-cleanup frozen H3 candidate with its exact external evidence identity.
Generated evidence is never staged.

## Stop and redesign conditions

Stop before the next slice if implementation needs any of:

- another Bridge package, generic transport registry, public plugin Interface,
  first-party camouflage protocol, custom cryptographic primitive, or root Go
  dependency not separately reviewed;
- Route/Service/Application knowledge of WebTunnel or Bridge state;
- public DNS, proxy, direct/ordinary/shorter/alternate fallback, a second
  Adapter, automatic family cycling, or a fifth contact;
- unbounded Invite/state/process/socket/queue/timer/evidence/cleanup work;
- a deadline, exposure, generation, or attempt reset;
- Bridge use in another Role Domain or conflicting family duty;
- test-only production control, candidate-authored verdict, secret publication,
  or evidence that cannot distinguish `fail` from `invalid`; or
- a privacy, availability, independence, or censorship claim beyond R-037.

## Definition of done

Stage 5 maintained development is complete only when:

- this brief was explicitly accepted before code;
- S5.1-S5.5 pass in order with scoped commits and package-map truth;
- offline/file import, finite state, restart, replacement, expiry, transition,
  contact order, deadlines, and exposure are fail-closed;
- selected WebTunnel carries the same exact-Target Service Connection and
  Application bytes through the unchanged Route;
- every negative produces the accepted result with zero forbidden fallback;
- Adapter/front/server and all helpers are included in resource/traffic/DNS/
  cleanup accounting;
- bounded capacity, sustained, pressure, hostile, recovery, evidence, and
  cleanup behavior is exercised at the maintained public seams;
- the independent verifier recomputes development-fixture verdicts, while the
  deferred final branch rejects incomplete evidence and all generated
  secret/evidence/build artifacts remain outside Git;
- bounded cleanup tests find zero owned resources;
- `git diff --check`, relevant tests, `make quick-check`, and `make check` pass;
  and
- final Standards/Spec review is clean, the Product Owner records `advance`, and
  the report states only maintained H3 development evidence, never a
  qualification or production censorship-resistance claim.

The complete R-037 matrix, long sustained cells, stand-specific supply freeze,
process-derived runtime allocation attestation, independent final verdict, and
external cleanup remain mandatory S9.6 inputs.
Deferral changes scheduling, not their thresholds or the whole-H3 completion
gate.
