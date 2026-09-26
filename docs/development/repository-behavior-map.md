# Repository behavior trace map

Status: **working implementation trace**, not a second product contract or a
qualification verdict. Current committed path baseline: `53f02e64`; individual
traces name the earlier source revision they inspected where applicable.
ADR-0092's generic Endpoint deletion is committed; other adjacent Endpoint
changes still require reconciliation. The current requirement is
read from [scope](../product/scope.md),
[protected workload](../product/protected-service-workload.md), and the
[threat model](../security/threat-model.md); this map asks where the selected
behavior actually enters code, which Module owns its effects, and what evidence
still needs examination.

`Entry located` means the named production entrypoint exists. It does **not**
mean the complete normal, refused, interrupted, and restart paths have been
traced or qualified. Each row needs a caller/effect/cleanup/test trace before
it can inform a package move or deletion.

| Behavior to trace | Current contract owner | Production entry and first Modules | Trace state |
| --- | --- | --- | --- |
| Accept an independently pinned artifact and enrollment input before reporting Portable readiness | [Enrollment verification](../technical/enrollment-verification.md), [release/update/custody](../technical/release-update-custody.md) | `cmd/ardents/endpoint.go` -> `internal/endpoint/portable`, `internal/enrollment`, `internal/release`, `internal/endpoint/replacement` | Portable first-run, refusal, restart and resource source trace below; external before-execution verification and protected text launch remain separate |
| Accept current Network State and refuse stale/conflicting profile authority | [Network/Route/Node](../technical/network-route-node.md) | `cmd/ardents/offline.go`, `cmd/ardents-node/source_runtime.go` -> `internal/network/state`, `internal/network/source` | Offline Epoch, closed-profile and finite Source-wave/restart traces below; complete floor and installed two-host evidence pending |
| Create separate Service and admission authority material and issue bounded public credentials | [Release/update/custody](../technical/release-update-custody.md) | `cmd/ardents-custody/command.go` -> `internal/custody`, `internal/service/instance`, `internal/route/credential` | Service Credential and admission permission source traces below; full Vault rollback matrix pending |
| Create/reopen a host-local Service Instance and accept the exact Authority response | [Endpoint/Service](../technical/endpoint-service-runtime.md) | `cmd/ardents/service_instance.go` -> `internal/service/instance`, `internal/service/publication` | Command, durable transition, binding and restart source trace below; installed custody handoff pending |
| Start each State-selected Node duty and drain its accepted work | [Network/Route/Node](../technical/network-route-node.md) | `cmd/ardents-node/node_mode.go` -> `internal/node`, `internal/route`, `internal/route/replay` | State-to-Node handoff, forwarding, issuer and five-duty resource matrix traced below; remaining receiver failure and late-close paths pending |
| Serve closed admission token acquisition and receiving spend | [Private admission](../technical/private-admission.md) | `cmd/ardents-node/issuer_initialize.go`, `internal/node/closed_issuer_listener.go` -> `internal/route/credential`, `internal/route/replay` | Issuer lifetime and permission-to-spend traces below; full combined failure/late cleanup matrix pending |
| Start the confined Publisher/Reader Endpoint and admit its local capabilities | [Endpoint/Service](../technical/endpoint-service-runtime.md), [application confinement](../technical/application-confinement.md) | `cmd/ardents/endpoint_headless.go`, `cmd/ardents/endpoint_text_linux.go` -> `internal/endpoint`, `internal/application/broker` | Installed command path traced below; its launch does not compose the enrollment/Release route; active Endpoint edits require reconciliation |
| Publish an immutable text snapshot only after worker and Introduction readiness | [Protected workload](../product/protected-service-workload.md), [Endpoint/Service](../technical/endpoint-service-runtime.md) | `cmd/ardents-text/main.go`, `internal/endpoint/text_descriptor_publication.go`, `internal/endpoint/text_introduction_registration.go` -> `internal/application/textdocument`, `internal/service/publication`, `internal/service/reachability` | Worker, registration, Descriptor ACK, withdrawal and context close source trace below; installed Publisher-to-Reader acceptance pending |
| Interpret a Reader's explicit Target Link and resolve one private Descriptor | [Protected workload](../product/protected-service-workload.md), [private reachability](../technical/private-reachability.md) | `internal/endpoint/text_resolution.go` -> `internal/service/targetlink`, `internal/service/reachability`, `internal/route/terminal` | Reader Target Link, current authority and Descriptor-history path traced below; complete refusal/freshness matrix pending |
| Establish Introduction and JOIN over the selected Carrier | [Protected Route protocol](../technical/protected-route-protocol.md), [Network/Route/Node](../technical/network-route-node.md) | `internal/endpoint/text_introduction_exchange.go`, `internal/endpoint/text_join_service.go` -> `internal/route/closed_introduction_client.go`, `internal/route/closed_join_client.go` | Initial JOIN-to-Service resource transfer and close order traced below; all recipient timeout/refusal cases and replacement-close result remain pending |
| Authenticate the Service and exchange one bounded request/response with terminal outcome | [Protected workload](../product/protected-service-workload.md), [Endpoint/Service](../technical/endpoint-service-runtime.md) | `internal/endpoint/text_service_binding.go` -> `internal/service/connection`, `internal/application/interfacev2/connection`, `internal/application/textdocument` | Initial authentication, native stream, recovery, presentation and replacement-close trace below; full adversarial matrix pending |
| Withdraw publication, stop admission, cancel and join accepted work, then retain cleanup failure | [Endpoint/Service](../technical/endpoint-service-runtime.md), [Network/Route/Node](../technical/network-route-node.md) | `internal/endpoint/text_context_shutdown.go`, `internal/node/lifecycle.go`, `internal/service/connection/stream_lifecycle.go` | Endpoint local-server/context order, Publisher withdrawal, Node receiver matrix and Service stream cleanup traced below; nested cross-Module close/result graph pending |
| Replace/recover the installed Endpoint without bypassing Release or enrollment | [Release/update/custody](../technical/release-update-custody.md) | `cmd/ardents/endpoint_replace.go` -> `internal/endpoint/replacement` | Release decision, transaction checkpoints, explicit rollback and read-only recovery traced below; protected system-unit integration pending |
| Project bounded diagnostics without authority or network effects | [Command surface](command-surface.md) | `cmd/ardents/offline.go` -> `internal/diagnostics/timeline` | Node/Endpoint event producers and read-only timeline projection traced below; attempt-level Route history is still absent |
| Run installed qualification using the real Endpoint and Node paths | [Testing model](testing.md), [protected workload](../product/protected-service-workload.md) | `cmd/ardents-qualification/main.go`, `internal/qualification`, `tests/qualification` | Fixed stream runner's plan, artifact, participant join and evidence-terminal path traced below; ordinary installed text command profile is separate and remains an acceptance gate |
| Refuse retired commands, plans, wire forms, and persisted identities without fallback | [Command surface](command-surface.md), affected current technical owner | `cmd/ardents/offline.go`, `cmd/ardents/endpoint_headless.go`, `cmd/ardents-node/node_config.go`; retained decoders in their owning packages | Old Node duty refusal traced below; other compatibility entries pending |
| Select and retain adjacent closed Entry members | [Network/Route/Node](../technical/network-route-node.md) | `internal/endpoint/text_participant_linux.go` -> `textEntrySets` -> `entry.OpenClosedSets`, then `text_interior_set.go` -> `Members`/`CurrentMember` | Current root, refusal, restart and close path traced below; old Invite command is a separate retained writer pending retirement decision |

For each row, the next pass records: normal and refused call path; authority and
input validation order; mutable/durable state; accepted child resources; stop,
join, and cleanup error propagation; direct and installed test evidence; and
the exact current document that should change with implementation. The
[C0 component reconstruction](c0-component-reconstruction.md) is a proposed
Module composition against these traces, not a replacement for them.

## Source trace: text Application code versus process authority

This trace follows `cmd/ardents-text/{main,publisher,reader}.go`,
`internal/application/textdocument/{snapshot_file_linux,snapshot_exchange_linux,worker_process_linux,worker,worker_attachment_initialization,reader_connection,publisher_connections,exchange,presentation}.go`,
and the selected [workload](../product/protected-service-workload.md) and
[confinement](../technical/application-confinement.md) owners at the current
working tree. It separates Go package cohesion from process authority.

1. Trusted `ardents-text publish` calls `ReadSnapshotFile`: it walks the
   absolute path without following symlinks, reads at most 4 MiB twice,
   compares bytes and file identity, and rejects invalid UTF-8. The command
   clears its copy after `administration.RequestSnapshot`; no path is sent to
   Endpoint or the worker. Endpoint later copies the admitted bytes into a
   `Snapshot` and keeps Instance, publication and worker admission authority.
2. Endpoint calls `InitializeWorker` only on an accepted local attachment. Its
   nonce and snapshot digest bind readiness to one job but do not themselves
   prove isolation. The worker entrypoint `RunInheritedWorker` checks the
   inherited AF_UNIX stdio socket, null stderr and surviving descriptors before
   it calls `RunWorker`. The system-manager unit/cgroup proof remains Endpoint's
   separate prerequisite before any Grant; a Go package split would not
   supply that proof.
3. `ServeWorkerConnections` receives already admitted Service streams and
   owns those it takes from the channel, with 256 open and 64 active bounds.
   Its close path joins the attachment, stream I/O and worker-read goroutine;
   Endpoint still owns current-job checks and cgroup retirement.
   `ReadWorkerConnection` similarly joins local attachment and Service stream
   I/O and returns bytes only after the worker RESULT protocol; Endpoint still
   checks the current job and worker cgroup before presentation.
4. Trusted `ardents-text read` admits a typed Target Link through AAI3, then
   `textdocument.Read` sends the fixed 512-byte request and requires complete
   bounded UTF-8, directional EOF and authenticated clean terminal outcome.
   `WritePlainText` escapes terminal and bidi controls after that result. The
   command and fixed worker entrypoints share one executable, so artifact
   identity alone does not distinguish their authority: the installed process
   role, inherited descriptors and effective confinement do.

The shared fixed grammar, frame/credit rules and bounded document value give
`textdocument` one cohesive implementation responsibility. Its trusted import,
UI, Endpoint adapter and confined worker are distinct runtime roles and
resource owners. Moving files into new packages solely to mirror those roles
would increase exported seams without enforcing the process boundary. The
remaining acceptance proof is the installed journey that joins the selected
system unit, worker cgroup, Endpoint admission and actual Publisher/Reader
terminal outcomes; isolated package tests do not establish that join.

## Source trace: first enrollment versus protected text launch

This trace follows `cmd/ardents/endpoint.go`,
`cmd/ardents/endpoint_headless.go`, `internal/enrollment`,
`internal/endpoint/portable`, and the installed command test at `e48d4c3c`.
The selected readiness profile requires independently authenticated bytes
before first execution and a real installed Publisher/Reader journey.

1. The [Portable operator procedure](../reference/portable-enrollment.md)
   checks the independently delivered `SHA256SUMS` digest and exact inventory
   with shell tools **before executing** `ardents`. A running binary cannot
   authenticate its own first execution. The current technical verifier owns
   the in-process recheck, not this initial trust transfer.
2. `endpoint enroll`/`enroll-installed` enters `runEnrolledEndpoint`. It emits
   `starting`, opens the per-user Portable profile, claims its process lease,
   and binds a generic `probe`/`ready` Unix socket. On a first unbound program
   it then verifies the pinned enrollment, evaluates Release, prepares and
   commits the exact running program in the replacement ledger, and only then
   emits `endpoint-lifecycle ready`. A failed pin yields `incompatible` and no
   ready event, but the local profile was already opened and is closed on
   return; `Verify` itself does not mutate the Release floor.
3. On a routine restart, an exact `replacement.StateCurrent` program takes the
   durable selected-program path before parsing the old bundle/enrollment
   input. It emits `endpoint-replacement current`, then Portable `ready` and
   holds the profile until stop. The Linux process test explicitly removes the
   first bundle and verifies this restart; routine startup cannot be described
   as re-running the original enrollment check.
   The Installed first-run/rebind branch calls general `enrollment.Verify` and
   therefore accepts descriptor v1/v2/v3; the selected Portable first-run
   branch calls `VerifyHeadless`, whose Node/Custody inventory requires v3.
   The package process test actually upgrades through a v1 descriptor under
   `endpoint enroll-installed` (F-47). This older Installed lane still reports
   only Portable readiness.
4. Portable `ready` proves the local profile and probe attachment. The
   `portable` package owns no State, Route, text worker, or Service Connection.
   `endpoint headless` is another command path: it parses the v2 runtime plan
   and invokes `runTextHeadlessRuntime`, with no call to enrollment, Release,
   replacement, or Portable in those files.
5. The installed `text-command-network` test creates an `ardents-endpoint`
   system unit whose `ExecStart` runs `endpoint headless` directly. The same
   test builds the v2 runtime JSON from its own State, permission, and socket
   fixtures. It executes both Carriers and the real Custody/Node/text commands,
   but does not invoke
   `endpoint enroll` or `enroll-installed`. Its profile requires independently
   hashed candidate binaries and unit bytes, which is valuable fixture
   provenance; it is not the same first-execution enrollment/Release journey.
   The selected confinement contract requires this exact system-manager
   MainPID under the Endpoint service account before worker admission. The
   `endpoint user-unit` and `installed-user-unit` commands instead render
   per-user units for the generic enrollment/profile route; they cannot by
   themselves establish that system-manager worker-binding condition.
   The product command only decodes the supplied v2 plan; the repository's
   production command and packaging sources provide no protected-text plan
   composer or system-unit renderer. The system-unit installer for stream
   qualification serves another profile. The selected C0 scope requires an
   operator journey without manual JSON editing or fixture-key extraction.

The two paths have separate readiness and lifecycle owners. The installed
text command evidence proves its bounded network/Application behavior, while
the enrollment process tests prove their own artifact and restart behavior.
Neither test, by itself, proves one joined path from authenticated first
execution through protected Service readiness. The target composition needs
an explicit trust transfer or one integrated launch owner; retain the
independent pre-execution verification and selected current-program restart
semantics. It must also supply the exact protected plan and system unit through
a supported operator route. Qualification must run that combined path rather
than infer it from two separate green tests or a test-created plan.

## Source trace: host Service Instance request and Authority acceptance

This trace follows `cmd/ardents/service_instance.go` and
`internal/service/instance/{root,lifecycle,state,storage,response}.go` at
`e48d4c3c`; the selected contract is in the Endpoint/Service technical owner.

1. `service-instance initialize` accepts a bounded canonical plan with an
   absolute existing owner root, Network ID, exact time window, and a separate
   public request destination. The command checks cancellation before effects.
   `instance.Initialize` validates root ownership/marker, acquires its
   exclusive process lock, and either reopens an exact matching generation or
   generates a fresh Instance key pair; ADR-0102 stopped Introduction key
   emission with the Credential v3 contract. It atomically persists the
   `pending` state; a changed Network/time request is refused rather than
   replacing it.
2. The command copies the public request, closes the root and joins the close
   error, then writes that request to a new public file with exclusive create,
   sync, and exact-byte existing-file handling. If writing the public file
   fails, the initialized root remains. A retry at the same destination works
   only when that file is absent or already has the complete exact bytes; a
   partial file conflicts, so the operator can name a new destination in an
   otherwise matching plan. The command never exports private keys. Its JSON
   result includes the request and digest only.
3. `service-instance accept` reads at most 1024 response bytes, checks
   cancellation, opens the exact root, and calls `Root.Accept`. An exact
   already-accepted response is idempotent. A malformed response terminally
   rejects a pending generation; a different response conflicts with an
   accepted one and erases its durable secret material. The response must bind
   the exact public request and separately issued Credential. Each transition
   is durably replaced under the root lock before returning. The command joins
   `Accept` and root-close errors and prints a public state/generation result.
4. Runtime may call `Root.Credential` without opening signing authority.
   `OpenBinding` first compares the accepted Credential generation with the
   actual publication floor and refuses a consumed predecessor. The opaque
   `Binding` signs without returning key bytes. After a publication commits,
   `CommitPublished` removes durable secrets while the live process retains
   its binding; a restart cannot revive that published generation. Withdrawal
   terminally erases the live binding. `Root.Close` erases in-memory keys and
   releases the exclusive process lock.

Direct tests cover exact initialization retry, accepted response repeat,
malformed/conflicting response, ambiguous root refusal, non-exporting binding,
and successor requirement after publication. The source trace does not yet
prove the separately operated Custody response handoff or the installed
Publisher journey. This is a cohesive deep module: splitting its root,
durable state, and binding by file count would divide one authority invariant.
The phase-less `root-v1` rederivation branch is retired with ADR-0102: the
`root-v2` decoder refuses pre-v3 bytes with the typed `ErrLegacyRoot` before
any field decode, and no compatibility branch remains (F-42 closed). If startup fails
after taking the exclusive lock, `openPrepared`, `Initialize`, or `Open` can
discard its release result (F-72). Neither fact creates a second current
private Introduction, but both must be handled in any old-root transition.

## Source trace: separate Service Authority issuance

This trace follows `cmd/ardents-custody/service_authority.go`, its stable
response writer, and `internal/custody/{vault_operation,service_authority,
service_credential}.go` at `e48d4c3c`. It covers the selected interactive
Service Credential ceremony, not Admission Authority or general Namespace
operations.

1. `create-service-authority` takes only public environment, Network and root
   commitments. The Vault generates its own Ed25519 authority key, retains it
   in an encrypted record under the Vault's operation lock, and returns only
   the public authority/Target and record identity. It does not consume a
   Service Instance private key.
2. The independent Service host transfers its public Instance request and
   lowercase SHA-256 commitment. `issue-service-credential` reads bounded
   request bytes and requires the operator to enter the commitment separately
   through the interactive input before opening the Vault. A substituted
   request fails before password input or Vault mutation. The command passes
   the exact public Authority binding, request and commitment to one bounded
   `Vault.Execute` operation.
3. Custody parses the Instance request, enforces a 24-hour maximum Credential
   lifetime, a 48-hour issuance horizon and no overlap with the previous
   Target generation. It unlocks the exact encrypted Service Authority record,
   derives a deterministic successor ID from the request commitment, signs
   only the new public Credential, ensures the encrypted successor record, and
   advances the durable Authority floor before returning a response. An exact
   retry compares the existing successor/floor rather than signing a second
   unrelated generation. The Vault operation lock is released and the Vault
   closed before the command publishes the response.
4. The response writer stages complete bytes in the destination directory,
   flushes them, then links a new public file without replacement. An already
   present destination is accepted only if its complete bytes match exactly.
   Failure to publish the public response does not roll back the advanced
   Authority floor; the deterministic exact retry is the recovery path.
   `service-instance accept` then consumes that response under the host's
   separate root lock and exact request commitment.

Direct tests cover substituted request refusal before secret input, bounded
validity before password input, exact successor retry, public-only receipts,
and Instance acceptance. The complete two-process installed handoff and
operator failure recovery still need qualification evidence. Both packages
have distinct authority: a common generic file writer or signing interface
would not simplify that trust boundary by itself.

## Source trace: offline State and closed-profile acceptance

This trace follows `cmd/ardents/offline.go` and
`internal/network/state/{open,offline_accept,closed_profile_accept,
closed_profile_store,lifecycle}.go` at `e48d4c3c`. The current Node/Route
technical owner defines the shared current/pending/conflict invariant.

1. `accept-offline` parses one explicit root, Network, authority threshold,
   verification time, selected profile and complete Epoch/input/materialization
   files. The closed Route profile also requires a pinned signer already in
   the State authority set. `state.Open` acquires the durable root, recovers
   current and distribution state, then `Accept` verifies the complete
   decision before committing a new generation and trusted-time floor.
2. `Accept` refuses a concurrent Source refresh and any already durable
   conflict. The State owner alone decides whether a verified candidate is
   admissible as current or pending. A different digest for an already pending
   Epoch durably records conflict and refuses; normal success commits before
   returning a snapshot. Tests cover recovery, wrong materialization,
   successor chains, pending activation and conflict across reopen.
3. The command can then optionally read and submit one signed `ARDCPR03`.
   `AcceptClosedProfile` requires an already accepted matching closed Epoch,
   its pinned signer, exact Node-record joins and bounded time. An exact
   profile retry returns the same projection. A second valid digest durably
   records conflict rather than replacing the first. The command's two stages
   are **not atomic together**: if profile acceptance fails, the Epoch remains
   committed and the command returns an error without its success event.
   There is also a separate `accept-closed-profile` command for an existing
   State root; installed Node E2E tests use both routes.
4. `CurrentClosedProfile` and `CurrentClosedRoute` recheck the stored profile
   under State's read lock with resource, clock, owner, epoch, candidate and
   conflict constraints. A signed file on disk alone gives no runtime use
   after State successor, expiry, clock uncertainty or owner failure.
   `Close` cancels and joins background work, releases the durable root and
   local Source role, and reports terminal errors. Its result handling has
   the cleanup-error gap recorded in the findings ledger.
5. The Node-record join currently checks identity, record generation, digest
   and closed Carrier, while State separately materializes the Epoch's string
   `Assignment`. The profile's numeric Role Domain is not compared with that
   assignment in `matchesClosedProfileCandidates` or Node's receiver
   projection. The technical contract requires the match; F-45 records the
   exact discrepancy and the focused refusal evidence still needed.
6. `parseEpoch` accepts AREP v1/v2/v3, and the shared `verifyEpoch` matches
   profile without restricting its envelope version. Closed Carrier admission
   filters old Node Records, but `AcceptClosedProfile` does not constrain the
   Epoch schema. The installed closed-State fixture uses v3; this does not
   prove that v1/v2 are refused. On restart, `loadGenerationChain` uses the
   same decision verifier for persisted predecessors and current State.
   `Open` assigns that decision before starting its Source owner; control-floor
   recovery can restore it as current, and pending recovery can later feed
   Source-wave activation (F-50). The one-version rule therefore needs gates
   for offline and Source-wave intake **and** a defined current/pending restart
   outcome; retaining a historical parser alone does not make old State inert.
   `candidate.go:verifySourceBundle` additionally reuses an exact-byte current
   or pending decision after checking the requested materialization, before
   reaching the general candidate verifier. An intake gate only in
   `verifyDecision` would miss those recovered-decision branches. In contrast,
   `storage.go:loadGeneration` must still authenticate historical predecessors
   while deciding whether the recovered current/pending generation may be
   exposed. F-50 records the separate gates and evidence matrix.

This path supports a real State owner and a separate profile acceptance
operation; file-count reduction must preserve its immutable projections and
durable conflict truth. The Source-wave trace follows next; exact floor
recovery and installed two-host behavior remain separate trace work.

## Source trace: finite Source wave and restart

This trace follows `cmd/ardents/source_plan.go`,
`internal/network/state/{refresh,selection,selection_commit,control_state,
scheduler}.go`, and `internal/network/source` at `e48d4c3c`.

1. `refresh-sources` reads one bounded two-Source plan and opens the existing
   State root. Normal mode calls `Refresh`; `--once` stops after that wave;
   `--resume` calls `Current` and enters `Wait` if automatic refresh is
   configured. The two flags cannot be combined. State's root, source plan,
   current decision and local Source role remain owned by State, not by the
   transport helper.
2. `Refresh` first checks resource and trusted-time evidence, durable backoff,
   source identity collisions, and its single active-wave flag. It persists
   one 15-second wave identity/order, then starts bounded `LATEST` attempts
   against both authenticated Sources. A `BY_DIGEST` fetch is permitted only
   for the exact digest returned by a failed `LATEST` response. Each bundle
   is decoded and verified against the current authenticated chain; source
   identities and Node roles cannot be substituted by the peer.
3. State waits for both attempted outcomes before selection. Two valid,
   conflicting digests at one Epoch number durably record conflict. One valid
   decision may advance State, but the snapshot records incomplete `LATEST`
   evidence rather than claiming two-Source completeness. No valid decision
   records finite failure and durable retry backoff. A verified future Epoch
   is staged as pending; after restart, a new complete Source wave can
   activate that exact digest only when trusted completion time admits it.
4. The automatic scheduler treats an overlapping active wave, finite
   unavailability and clock uncertainty as non-terminal conditions. Other
   terminal refresh errors remain visible through `Current` and `Wait`.
   `State.Close` cancels and joins the scheduler/Source server before releasing
   root and local-role ownership. The `--resume` command's event mismatch is
   recorded separately in the findings ledger.

Tests inspected here cover two authenticated Sources, collision/conflict,
durable backoff, one-valid partial evidence, uncertain clock before contact,
pending Epoch recovery/activation, and an automatic tick overlapping an
active wave. The installed two-host profile and exact resource/close failures
still need their own evidence pass.

### Source package boundary and server lifetime

`internal/network/source` is a cohesive transport and bounded grammar owner,
not a second State authority. `source.New` accepts either no acquisition half
or exactly two complete, distinct Source declarations; it copies certificates
and trust material, and requires State's verification clock for configured TLS.
State builds this plan in `config_validation.go`, calls `Fetch` from `refresh.go`,
and starts `Serve` from `server.go`. State owns the durable root, candidate
verification, Source-wave selection and server goroutine lifetime.

`Fetch` makes one bounded mutual-TLS request using the declared key pin. The
request is exactly 77 bytes; the response has a 45-byte header and at most
1 MiB of payload. `Serve` owns its listener, limits concurrent handlers to
eight, closes on cancellation and joins accepted handlers before returning.
Tests in this package check frozen request bytes, refusal of a non-OK response
with object data, certificate copying, TLS clock/pin behavior, public plan
observation and handler write-error return. They do not prove an installed
two-host result.

One visibility limit remains: `handleConnection` returns deadline and response
write errors, but the `Serve` goroutine discards that result because a single
peer failure must not terminate the shared listener. State receives active
handler counts and whole-server termination, not a bounded per-connection
failure category (F-65).

## Source trace: authenticated State duty handoff to Node

`cmd/ardents-node/node_mode.go` opens the State root, refreshes it when no
current generation exists, and supplies `store.CurrentNodeDuty` through
`node.Config.Current`. `state.NodeDutyView` holds a copied broad Snapshot but
exposes only duty getters; `node.currentFacts` immediately copies 38 getters
into its own `dutyFacts`, rejecting candidate count above 64, authority count
above 16 and empty copied authority identities/keys. This leaves State's
source, pending and persistent internals outside Node. The reverse projection
(`dutyFacts` satisfying `DutyView`) is test scope since ADR-0101: production
supplies `state.NodeDutyView` through the command callback, and behavior
tests supply the snapshot directly without a Network State runtime.

Node assesses that local copy before opening a duty, reads it again after
quarantine, and polls it while live. A changed selected generation, Network,
Epoch, digest, Node or assignment withdraws the running duty through
`sameDuty`.
Current closed forwarding, Resolution and issuer peer checks read candidate
facts from the copy. The copied old Epoch authority list has no subsequent
production read in Node; State's three extra Transit issuance projection
methods have no production caller. Their persisted signed inputs need their
own compatibility disposition before storage changes (F-07).

The target handoff is one State-owned copied duty projection returned by the
existing command callback, with bounded candidates and no source/pending
fields. Node retains validation and its exact recheck/withdraw lifecycle;
this source trace does not authorize exposing `state.Snapshot` or removing
the current candidate facts. Direct State projection tests and Node lifecycle
fixtures cover currentness and admission; the value-shape refactor needs those
oracles preserved, not a new package.

## Source trace: installed text Endpoint startup and shutdown

This trace was read from `cmd/ardents/endpoint_text_linux.go`,
`internal/endpoint/text_participant_linux.go`, the two local Application
servers, and the text adapters at `9835e225`. It covers the local process
boundary, not the later publish/read network exchange.

1. `runTextHeadlessRuntime` requires a pollable inherited event output and
   opens an owned handle for it. It derives Network configuration from the
   selected plan and calls `RunTextParticipant` with explicit durable roots,
   local socket addresses, principals, permission files, clock, and a bounded
   event writer. A failed event write becomes an error rather than silent
   success.
2. `useTextParticipant` opens and optionally refreshes Network State, requires
   a current closed Route profile, opens the Service Instance, verifies that
   its Credential names the selected Network, and creates an Endpoint/Broker
   generation. It opens the publication floor and matching Instance binding,
   then makes the token journal and Entry sets available.
3. `runTextInterfaces` admits separate Connection and Administration
   principals. Each receives its own `textContext` and provisioned permission.
   Both permissions must remain current before either socket is exposed. It
   then opens the Reader Connection adapter and Publisher Administration
   adapter, listens on their separate local sockets, emits `ready`, and waits
   for cancellation or an observation failure. `ready` therefore establishes
   local interface readiness, not that a text Descriptor has been published.
4. The deferred close order is Administration listener, Connection listener,
   Publisher adapter, Reader adapter, both contexts, Endpoint, Instance root,
   then Network State. Each close error is joined into the returned outcome.
   The command finally closes the owned event output and reports a bounded
   startup/running failure class when it can still write the event.

The local server close boundary has now been checked in source:
`interfacev1/administration.server.Close` and
`interfacev2/connection.server.Close` cancel, close their listeners and
clients, then wait for each accepted handler before returning. The Publisher
and Reader adapters subsequently call their context's `Close`; that method
releases admission and waits for `closeAfterAuthorization`, whose child join
and publication retirement finish before its `done` signal. Thus the outer
participant cannot release its Endpoint/Instance/State parents while an
accepted local handler still borrows those adapters. This is a source-level
ordering claim, not a proof that every nested worker, refresh and Route
cleanup result is bounded or surfaced correctly; those deeper lifetimes and
the installed process result remain separate acceptance evidence.

The launch artifact path has a separate source gap (F-25/F-27):
`endpoint headless` consumes a caller-supplied v2 plan; it does not call the
Enrollment/Release or current-program owner. The alpha bundle inventories four
headless command binaries and static enrollment inputs, while the installed
text-command candidate separately builds `ardents-text` and worker fixtures.
Endpoint's `loadInstalledWorkerArtifact` checks the local root-owned six-file
worker manifest and rechecks it around launch, but no maintained installer
ties that manifest, the accepted program and the active protected system unit
to one candidate. These local checks and the test's independent hashes do not
form the missing operator handoff by themselves.

The entrance was rechecked at `53f02e64` against
`cmd/ardents/{endpoint,endpoint_headless,endpoint_text_plan,operator_input}.go`
and the installed command test (F-67). Dispatch opens the plan path using
`os.Open`; the bounded decoder checks JSON shape, current v2 marker, retired
fields and distinct canonical referenced paths, but not the plan file's
owner, symlink status or release identity. A supplied Source plan is compared
to repeated State trust anchors; a static plan supplies them directly. The
installed test writes `runtime.json` itself and changes ownership to the
service account before starting its temporary system unit. This proves
consumer behavior, not the provenance of plan bytes or their binding to the
active MainPID. A production installer must supply that missing proof before
`RunTextParticipant` opens Network State and Service Instance. The targeted
test reading does not claim a complete installed restart or tamper matrix.

## Source trace: Publisher readiness, publication, and withdrawal

This trace follows `cmd/ardents-text/publisher.go` and the Linux Endpoint
Administration, worker startup, registration, Descriptor, withdrawal, and
context-retirement owners at `9835e225`. It is a source trace, not installed
qualification or evidence of a completed Reader exchange.

1. `runTextInterfaces` opens Connection and Administration sockets and emits
   `ready` before any `PublishSnapshot`. The event proves local command
   admission, not a registered or reachable Publisher Descriptor. The trusted
   text command later imports a bounded immutable snapshot from an explicit
   absolute path and sends it over the authorized Administration socket.
2. `PublishSnapshot` checks the Administration capability, snapshot length and
   UTF-8, and reserves one startup. Concurrent Withdraw or Close cancels and
   joins the pending startup before any late handover. `startTextPublisher`
   launches and qualifies the installed worker before registration or
   Descriptor effects.
3. The worker opens Source, Introduction, and Responder prefixes. It obtains
   the State-selected Introduction recipient, bounds a fresh registration to
   its duty and ten minutes, and presents a real class-3 token. The Route
   channel holds its slot, revision, and ACK receipt; registration alone does
   not make the Descriptor visible.
4. `publishTextDescriptor` requires the same live permission, Instance binding,
   Source, and registration. It acquires or durably commits Publication,
   creates a private recipient and signed Descriptor, sends the exact bytes to
   the selected resolution receiver, requires status 0, then rechecks owner
   and authority after the network wait. Only that acknowledged transition
   makes the local pair published and starts refresh. Exact retry reuses the
   signed bytes; an acknowledged successor retains its predecessor for at
   most 60 seconds. `PublishSnapshot` returns after ACK and transfers the run
   to Administration, without proving Reader reachability.
5. `Withdraw` first stops new local admission, then joins refresh, withdraws
   and closes the Route registration and private recipient, and joins the
   worker. Already accepted streams retain their bounds with a five-second
   maximum for orderly withdrawal; failure cancels the run. Repeated calls
   cannot extend the deadline. `Close` aborts and joins startup/run without
   reporting successful withdrawal.
6. Context loss cancels every child before joining. Openings finish before
   prefixes close; registration/refresh producers finish before channels
   close; issuance and resolution finish before exchanges; the Job joins
   last. The Context then unpublishes the durable Service record and withdraws
   the Instance binding. A failed unpublish retains the binding and error.

On the Node side, `closedResolutionServer.resolve` returns status 0 for a
publication only after exact private-proof and current-Introduction checks and
`reachability.Store.PublishPrivate` returns `StoreAccepted` or
`StoreAlreadyCurrent`. A new accepted record passes through `store.write`:
temporary file write and sync, rename to the Target record, then directory
sync, before the in-memory map advances. A write failure returns an error,
terminalizes that Store owner, and cannot produce status 0. The Node rechecks
its duty before writing the response; Route verifies the response nonce and
publication payload shape. A lost response leaves an uncertain caller result,
but an exact retry may receive `StoreAlreadyCurrent` for the same durable
bytes. Status 0 does not promise later reachability after profile/State loss,
expiry, withdrawal, or a different Reader authority.

Direct tests cover pre-ACK refusal and late ACK loss
(`text_publication_initial_ack_test.go`), startup/withdrawal concurrency
(`text_administration_linux_test.go`), cleanup retention
(`text_publication_cleanup_test.go`), and admitted/stalled read drain
(`text_publisher_withdraw_linux_test.go`). The installed worker network test
exercises Administration withdrawal. `closed_resolution_network_test.go`
checks admitted publication and lookup on both Carriers; Reachability's
`private_store_test.go` and `private_store_failure_test.go` check durable
reopen, exact retry, conflict floors, and failed-write refusal. The combined
installed Publisher-to-Reader journey remains unproven by these separate
tests. Revalidate this path after the active Endpoint slice changes it.

## Source trace: closed forwarding Node resource ownership

This trace follows `cmd/ardents-node/node_mode.go`,
`internal/node/lifecycle.go`, `duty_server.go`,
`closed_forwarding_listener.go`, `closed_forwarding_shutdown.go`, and
`closed_forwarding_receiving.go` at `e48d4c3c`.

1. The command opens Network State and supplies `CurrentNodeDuty` to
   `node.Run`. It closes State and the optional clock observation after the
   Node run returns, joining their close errors with the run outcome.
2. `node.Run` opens the shared Hosting period before admitting a duty. It
   retains the local duty state during preparation/quarantine, rechecks the
   exact State assignment, then `startDuty` selects the closed receiver purpose.
   The forwarding path opens the host, one bound replay spend ledger with
   limits/bootstrap controller, an outgoing Carrier pool, and a shared Carrier
   listener in that order. Each failed open joins cleanup of earlier resources.
3. The live server owns the listener, accepted producer goroutines, pool,
   receiving resources, host, and outgoing session readers. Withdrawal calls
   `Stop` to prevent acceptance and cancel the server context, then enters
   DRAINING and calls bounded `Drain`. State loss, listener exit, resource
   pressure, and evidence failure enter the same stop/drain cleanup path.
4. `finishShutdown` waits for server workers, then joins outgoing session
   readers, closes the host, closes the replay ledger, and publishes the joined
   cleanup result. `Drain` returns that result after `drained` closes. If its
   deadline expires, the command reports an unproven cleanup; the server's
   sole owner continues retaining roots until the join finishes.
5. The Route seam is already specific: `ClosedSharedCarrierListener` exposes
   `Accept(ctx, timeout)` and `Close`, and transfers ownership of each returned
   `ClosedSharedCarrier.Connection` to Node. Route selects TCP/TLS or QUIC and
   classifies direct-role versus authenticated outer-Node TLS before ARDP.
   A failed peer handshake is tagged so Node can continue accepting; a broken
   listener exits the serve loop. Node's `Stop` closes the listener once and
   `Drain` includes that close result. An admitted connection is closed by its
   Node handler after its inner work joins; the handler currently discards
   `SetDeadline` and physical `Close` errors. This is a separate per-connection
   outcome from the listener and durable-root cleanup result. The direct
   tests cited below do not establish its terminal-error policy.

The direct tests exercise real TCP authentication and idle Carrier drain
(`closed_forwarding_shutdown_test.go`), a delayed outgoing reader and retained
root lease across a Drain timeout (`closed_forwarding_reader_shutdown_test.go`),
and failed receiving-resource initialization with joined cleanup errors
(`closed_forwarding_receiving_test.go`). These test sources were inspected;
this study has not rerun them against the current tree.

This establishes the forwarding ownership order and an existing narrow
listener Interface. The recipient matrix below compares the other duty
constructors and drain paths; tests for timeout, concurrent producer/reader
exit, and the required per-connection close-error policy remain necessary
before proposing a package move.

## Source trace: closed issuer Node and token-listener lifetime

This trace follows `internal/node/lifecycle.go`, `closed_issuer_listener.go`,
`closed_outer_issuer.go`, `closed_outer_lifetime.go`, and
`internal/route/credential/closed_token_listener.go` at `e48d4c3c`.

1. Node admits one current State assignment, retains the local role and shared
   Hosting period, then `startClosedIssuer` validates local issuer roots,
   certificate, finite connection/drain limits, and the current closed State
   profile. It derives the selected listen address from State and the bounded
   local override; local configuration cannot supply the receiver identity.
2. Startup opens the credential issuer root, rechecks the exact issuer
   receiver, opens a separately bound replay spend ledger, creates Route duty
   limits, then opens the selected TCP/TLS or QUIC shared Carrier listener.
   Each later open failure joins closure of resources already acquired.
3. Credential's `ClosedTokenListener` owns the accept loop, connection limit,
   direct bootstrap, Node-bootstrap handoff, worker wait group, and listener
   stop. Its Node callback rechecks current State/peer authority, establishes
   the outer and inner protected lanes, and passes an admitted ordinary child
   to the issuer with a replay-backed admission channel. The shared
   `serveClosedOuter` joins all inner handlers before closing the outer lane.
4. On withdrawal, `node.Run` calls `Stop` then bounded `Drain`. The listener
   closes admission, cancels accepted operations, and waits for accept and
   exchange workers. Only after a successful join does Node close the spend
   ledger and issuer root. A drain timeout returns an unproven-cleanup error
   while those durable owners remain unreleased; the current code has no
   observed later in-process release path for that timed-out issuer instance.

The inspected tests cover successor-profile withdrawal, wrong duty/digest,
Node-authenticated bootstrap, admitted issuance, and direct TCP/TLS plus QUIC
bootstrap. The separate permission-to-spend trace below follows one-use
receiver spending and issuer restart. Late cleanup after a drain timeout and
whether an issuer-root `Close` failure remains visible through the process
result are still required before moving the issuer listener across the
Node/Route/Credential boundary.

## Source trace: admission permission through token spend

This trace follows `cmd/ardents-custody/admission_authority.go`,
`internal/custody/admission_authority.go`, Endpoint's `text_permission*`,
`text_issuance*` and `text_token_transfer.go`,
`internal/route/credential/closed_token_{issuer,issuer_ledger,admitted}.go`,
`internal/route/closed_admission_channel.go`, and
`internal/route/replay/spend_ledger.go` at `e48d4c3c`. Its current technical
owner is [private admission](../technical/private-admission.md).

1. The live Publisher or Reader `textContext` checks its verified worker grant
   and current closed State profile. It creates one holder key and signed
   request for the current hour, role and finite maxima. The holder key and
   pending request remain with that Context. The public request file and its
   SHA-256 are an offline handover, not admission authority; same-hour changes
   to its allocation or profile cannot silently replace the retained request.
2. `ardents-custody issue-admission-permission` reads the request and requires
   its digest through a separately entered commitment **before** opening the
   Vault. Custody verifies the holder proof, exact admission Authority binding,
   current hour, role quotas and retained allocation journal. A repeated
   permission ID must carry the identical request digest. For a new allocation,
   it writes the encrypted Vault successor and advances the floor before
   returning signed permission bytes. The command publishes those private
   bytes through an owner-only staged file and prints only their digest.
3. Endpoint reopens the response file and verifies the signed Permission against
   the exact retained request, holder, State profile and hour. For issuance it
   derives each recipient duty from current State, holds at most one live batch
   per Context, and reserves its own class quota. An ambiguous network result
   retains the original opaque blinding state and request ID for a same-process
   retry; cancellation may burn the allocation. Successful finalization
   verifies every token before it enters volatile, Context-owned stock.
4. Before Route receives a selected token, Endpoint removes it from stock and
   durably records the potential presentation in its separate token-attempt
   root. A journal failure closes local admission. The receiving admission
   channel binds HELLO to the actual TLS exporter, verifies the token and
   current recipient, reserves capacity, and durably spends the token in its
   exclusive replay ledger before returning a finite work lease. Duplicate or
   ambiguous spends refuse work; a failed handoff releases only that attempt's
   capacity. Issuer reservations, Endpoint attempt receipts and receiver spends
   are three distinct ledgers with distinct owners and restart meanings.

The command-level test in `tests/e2e/node/closed_text_command_installed_linux_test.go`
starts the ordinary Endpoint under systemd, uses real `ardents-custody` commands
for both role permissions, and runs TCP/TLS and QUIC scenarios. Its Custody
helper also proves a substituted request digest fails before Vault unlock.
Focused tests cover exact issuer retry/restart and old-ledger promotion,
Endpoint presentation/cancellation burns, receiver duplicate/recovery and
failed writes, and admission capacity release. These test sources were
inspected; this study has not rerun their selected profiles on the current
worktree. The combined installed result remains a qualification question,
and the issuer's late cleanup gap above remains open.

### Permission-file handover and Endpoint token-attempt root

`internal/endpoint/permissionfile` owns only the Linux file transport for one
public request and one separately approved 228-byte response. `Open` pins an
absolute canonical path through an owner-private directory. A request is
created without following links, synced with its directory, and an exact
retry is allowed; a different existing request is refused. Response reading
checks regular-file ownership, mode, link count, size and identity before and
after reading. `text_permission_files_linux.go` and
`text_permission_provision_linux.go` own the bounded wait, request digest,
live Context check, signed response verification and all `Path.Close`
results. The package does not grant permission or retain a participant.
Its direct tests cover exact retry, conflicting request, absent/complete
response, and symlink, hardlink, FIFO, shared mode and oversized refusals.

`internal/endpoint/tokenjournal` owns a different durable root. The protected
participant opens it before exposing local transports; Endpoint retains the
handle and closes it with its Entry root. `takeTextTokenLocked` first removes
a token from volatile stock, then calls `Mark` before Route receives bytes.
The journal stores only a token hash and public attempt binding, under an
exclusive lease with one network, replay and clock floors. It rejects duplicate
hashes after restart, bounds records to 131072, prunes only beyond the hour
plus one-minute margin, and persists the deletion time floor. Ambiguous append
or replacement poisons the owner; `Close` preserves that failure. Endpoint
terminalizes local admission if `Mark` fails. Direct tests cover restart
replay, foreign/partial/clock-invalid roots, file replacement, pruning and
compaction floor. Neither this journal nor the permission file is an issuer
reservation or receiver spend ledger; those have separate authority and
lifetimes in the admission trace above.

## Node receiver resource matrix

### Resolution duty: durable commit and reply boundary

This trace follows `internal/node/closed_resolution_listener.go`,
`closed_resolution_operation.go`, and
`internal/service/reachability/{private_store,store}.go` at `9835e225`.

1. `startClosedResolution` derives the receiver from current State, then
   opens the bound replay spend ledger, exclusive Reachability Store root,
   duty limits and selected shared TCP/TLS or QUIC listener. A later startup
   failure joins close results for the resources already opened. The local
   profile reserves roots and limits; it does not choose receiver identity.
2. The server admits only authenticated outer Node Carriers. Its worker
   obtains a fresh inner role TLS channel, verifies the Reachability HELLO,
   spends the Control token through the admission channel, enforces the byte
   and deadline lease, and decodes one terminal Descriptor request. A direct
   role Carrier or exhausted capacity is closed without an operation.
3. Publication verifies the signed private Descriptor and current
   Introduction slot before `Store.PublishPrivate`. The Store writes and
   synchronizes the accepted record before updating its in-memory map; a
   failed write terminalizes that Store instance. Node returns status 0 only
   for `StoreAccepted` or `StoreAlreadyCurrent`, status 3 for stale or
   conflicting evidence, and status 1 for other failure. Lookup reads only
   a current unconflicted proof under the selected profile.
4. Before sending the terminal result, Node rechecks context, exact State
   duty and lease deadline. A change after durable commit can suppress the
   ACK, leaving the Publisher uncertain although the Store has committed.
   An exact retry can then receive `StoreAlreadyCurrent`; replacing the
   Descriptor as though the first attempt failed would violate the revision
   contract. The Endpoint's exact-byte retry and verified ACK are separate
   responsibilities from the Store commit.
5. `Stop` cancels acceptance and closes the listener once. The server's own
   goroutine waits for all accepted workers, then closes Store and spend
   roots and publishes their joined close result. `Drain` may time out before
   that result, but the goroutine retains both roots until the workers join.
   Physical connection-close errors in rejected and accepted handlers are
   currently discarded; they are not part of the joined root-close result.
6. On restart, `OpenStore.restore` authenticates both old stored-record v1 and
   private v2 entries into one Target map. An old entry is unavailable through
   `LookupPrivate`, but its Target and Credential floor remain. A private v3
   publication for the same Target reaches `compareStored` and fails the
   format-adoption check; old entries also count toward the 128-Target bound
   regardless of expiry. There is no Store adoption operation (F-32).

The inspected direct network test covers publish, lookup, duplicate token,
revision conflict, successor, invalid proof and foreign Introduction over
both Carriers. The Store failure test proves that a failed conflict write
terminalizes that Store instance. These tests do not by themselves prove the
old-record-to-private same-Target restart case, the post-commit/pre-ACK
State-change retry, or the per-connection close-error
policy; this study has not rerun them against the current worktree.

The following constructor/close paths were inspected at `e48d4c3c` in the
five `closed_*_listener.go` owners and `closed_forwarding_shutdown.go`.
"Later close" means a server goroutine continues the close attempt after
`Drain` times out; it does not mean the caller receives its eventual result.

| Duty | Distinct durable or retained resources | Who closes them after accepted workers join | Accepted-connection close result | Later close after caller timeout |
| --- | --- | --- | --- | --- |
| Forwarding | Replay receiving ledger, Hosting period, outgoing Carrier pool and session readers | `closedForwardingServer.finishShutdown` after producers, readers and pool join | Discarded for accepted and refused children; outgoing-pool close is retained separately | Yes, from its own completion goroutine |
| Issuer | Credential issuer key root and replay spend ledger | `node.startClosedIssuer` adapter after `ClosedTokenListener.Drain` succeeds | Discarded by Credential's direct and shared child handlers | No observed owner; listener joins later but holds neither root-close callback |
| Introduction | Replay spend ledger and its Introduction slot floor | `closedIntroductionServer.run` after accepted workers join | Discarded for accepted and refused children | Yes, from its own run goroutine |
| Resolution | Replay spend ledger and Reachability store | `closedResolutionServer.run` after accepted workers join | Discarded for accepted and refused children | Yes, from its own run goroutine |
| Data JOIN | Replay spend ledger, Hosting period, JOIN pairing owner and monitor | `closedDataJoinServer.run` after accepted workers join | Non-benign errors joined into `drainErr` | Yes, from its own run goroutine |

The Resolution Store also has an opening failure path: after acquiring its
exclusive lease, `reachability.OpenStore` discards the lease-release error if
retained-record restoration fails. The normal `Store.Close` returns that
error. F-71 identifies the missing combined startup outcome; it matters when
an old Descriptor root is refused or migrated under the one-version policy.

All five roles use the Node lifecycle's common `probeServer` handle, although
only the private probe is a probe. This is a naming/interface problem in the
composition layer; it does not make their durable resources interchangeable.
`Done` reports accept-loop completion before the final child/root join; only
`Drain` can report the later cleanup result. F-61 records the inconsistent
accepted-connection close accounting that a listener/outer seam must resolve.

## Source trace: Reader Target Link to safe text presentation

### Local Grant and AAI3 transport boundary

`internal/application/broker` owns volatile Local Grant admission, not
Endpoint identity or installed isolation. It issues a 15-second one-use
capability for an exact Principal and `connection` or `administration`
surface. Activation consumes the capability and returns an opaque cancelable
`ActiveSession`; the capability expiry is not the active lease deadline.
Connection and Administration have independent 64-slot and six-slot
admission loads across pending capabilities and active leases. Revocation,
Broker close and permitted finite Drain cancel
matching active leases; `Release` is exactly once. Broker restart retains no
capability or session. Its direct tests cover binding/replay, independent
budgets, expiry, revoke, finite drain and a non-extendable deadline. The
Broker reports only `generic/unqualified` isolation.

`internal/application/interfacev2/connection` owns the separate local AAI3
request and stream. The portable client sends one bounded typed destination;
the Linux server refuses the reserved Name tag and the old AAI2 magic before
calling its Endpoint `Interface`. A successful setup transfers a Unix socket
to one stream whose writes are split into at-most-16-KiB frames; `CloseInput`
orders directional EOF after writes while response Read remains available.
One bounded terminal `Outcome` is required; EOF alone is not semantic success.
Server close cancels clients, joins handlers, removes its exact socket path
and retains the first Application cleanup failure. Client close interrupts
its transport and joins output and receive workers while preserving a verified
remote terminal outcome that arrived first. This module supplies no Job,
worker attestation, Route or Service authority; Endpoint owns the `Open`
implementation and Broker lease. The inspected request, setup-refusal and
server-cleanup tests cover typed refusal and terminal cleanup; the remaining
AAI3 lifecycle test files are listed in the file map but not yet traced.

The package `doc.go` still says AAI2 has an independently retained
compatibility owner. Current source has no `interfacev1/connection` package,
and product/Endpoint owners record its removal; only AAI3's pre-effect
refusal of old magic remains (F-66).

The separate `internal/application/interfacev1/administration` is a current
contract, not the removed AAI2 Connection. Its one authorized local socket
accepts exact `publish` and `withdraw` operations with fixed success/failure
lines. `SnapshotPublisher` optionally accepts one immutable UTF-8 body up to
4 MiB; an owner without that interface refuses it and never falls back to
bodyless Publish. A one-slot snapshot transition prevents concurrent
snapshot allocation while withdrawal remains callable. The optional
`PublishedLinkProvider` returns only bounded printable text from the current
publication; the caller supplies no Target or authority. Endpoint implements
the actual publication/withdrawal authorization, while `cmd/ardents-text`
uses snapshot and Link clients and `cmd/ardents` uses Publish/Withdraw.
The server owns its exact Unix socket, accepted handlers and close results;
close cancels operations and joins their handlers. The client uses a bounded
deadline, directional request EOF and cancellation that interrupts a blocked
response. Direct tests cover fixed v1 conformance vectors, malformed/surplus
refusal before owner effects, snapshot limits/no fallback, concurrent
withdrawal, Link projection, and shutdown retirement. The two maintained
local protocols therefore have different version numbers because they are
different interfaces; neither negotiates the other's grammar.

The `internal/service/targetlink` boundary has one current canonical grammar:
`ardents-target:v1:` followed by unpadded base64url for an exact 65-byte
algorithm/Network/Target payload. `Encode` refuses zero identifiers; `Decode`
refuses padding, surplus or truncated bytes, another prefix or algorithm, and
noncanonical spelling. The package neither resolves a Descriptor nor owns a
connection. Publisher publication exposes this link only after its current
Credential still binds the same Network and Target. Reader Endpoint first
refuses the retired Alpha prefix, then decodes the link and compares its
Network to the local Endpoint before any Resolution work. Package tests cover
frozen spelling and invalid forms; Endpoint tests cover foreign-network and
retired-Alpha refusal. This is one active Target Link version, not a v1/v2
negotiation point.

This trace follows `cmd/ardents-text/reader.go`, the Linux
`internal/endpoint/text_connection_linux.go`, `text_introduction_prepare.go`,
`text_resolution.go`, `text_join_service.go`, `text_service_reader_linux.go`,
and `internal/application/textdocument/reader_connection.go` at `e48d4c3c`.

1. The trusted `ardents-text read` command reads one bounded Target Link from
   human input, validates its AAI3 request encoding, and opens one local
   Connection with a 30-second UI deadline. It owns and joins input/output
   closure on cancellation. It does not select a Network receiver or retry a
   different local Connection.
2. The Reader `textConnection` parses the Target Link, excludes an old Alpha
   destination before effects, allows one pending read, admits a separate
   Connection capability and Job, launches a confined worker, then prepares
   Introduction. Resolution uses the current Source prefix, issued stock and
   durable token-attempt journal; a verified Descriptor enters the bounded
   history only after the same current authority is rechecked.
   The `descriptorhistory.History` package verifies the raw private proof
   again under the Context lock before recording a Target. Its volatile map
   holds at most 128 Targets, including expired and conflicting floors, until
   that Context retires. A lower publication generation or revision is
   refused; an equal-generation different publication retains a conflict and
   the longest authority interval. An equal-revision different Descriptor
   retains a revision conflict, which a higher revision can repair without
   changing publication authority. A returned proof cannot alias the retained
   hashes. The Context owns live authority and lock ordering; the package has
   no durable root, network call or authority to publish. Its direct tests
   cover rollback, conflicting branches, non-overlapping successor and proof
   mutation. Endpoint's tests cover per-Context isolation and retirement.
3. Introduction preparation checks the exact target, signed Publication,
   current State/profile and Job. It selects a live Data JOIN recipient and
   seals a recipient-only capsule with fresh nonce/secret material. The
   capsule is not a joined Service Attachment. After stock is ready, Endpoint
   runs actual submission and JOIN concurrently, cancels the other flight on
   failure, joins both results, and transfers the joined Route transport only
   while the owner and caller remain live.
4. Service binding authenticates the real Instance/Publication and the
   protected Connection context. Only after that does local AAI3 `Open`
   return a stream. The Reader worker moves one bounded request/response over
   the Service Connection and requires its clean terminal state. Endpoint
   closes the Service stream, finishes the worker operation, joins the worker,
   and rechecks that the result belongs to the current Job before releasing
   text bytes to the trusted UI.
5. The local `textReadResult` owns the pending slot, capability lease, caller
   cancellation bridge, worker, Service stream, and pipe goroutine. Its
   `Close` cancels and waits for that goroutine; errors become a bounded AAI3
   terminal outcome. The UI writes plain text only after `textdocument.Read`
   accepts the complete local stream and its terminal outcome.

Direct tests include UI cancellation and no-partial-presentation cases in
`cmd/ardents-text/reader_linux_test.go`, real resolution stock in
`text_resolution_network_test.go`, Source replacement in
`text_join_acquisition_test.go`, and a joined network document transfer in
`text_join_service_network_test.go`. Installed-worker coverage exists in
`text_worker_network_installed_linux_test.go`. These are source-inspected test
owners, not a fresh passing installed qualification result. Reconcile this
trace after the active Endpoint changes are integrated.

## Source trace: authenticated Service bytes and Attachment replacement

This trace follows Endpoint's `text_service_binding.go`,
`protected_service_{attachment,tls}.go`, `text_service_stream.go`,
`text_join_service.go`, native `service/connection/{initial_authentication,
stream_contract,stream_lifecycle,stream_recovery}.go`, and
`application/textdocument/{reader_connection,presentation}.go` at
`e48d4c3c`. The Endpoint/Service technical owner defines the selected boundary.

1. Endpoint binds the verified Publication, Target, current State digest,
   Instance key, Job, Work Safety bounds and a private per-Connection nonce
   into an immutable protected context. It checks the Publisher's live
   Publication lease independently. A JOIN transport alone creates no Service
   authority or Application stream.
2. Endpoint authenticates the selected Instance over the protected inner TLS
   Attachment. Native `NewAuthenticatedStream` joins an Instance proof and
   Continuity exchange before either side can send Application bytes. Native
   Stream owns ordered byte offsets, bounded queues, directional Terminal
   receipts and finite recovery under the same logical context; its opener
   may supply a fresh physical Attachment but cannot change Target or Job.
3. The text Reader worker sends one fixed request and receives a bounded
   response. `ReadWorkerConnection` requires Service EOF and a clean native
   terminal before accepting the worker's complete RESULT; Endpoint then
   joins Service stream and worker, rechecks the current Job, and releases
   bytes. The trusted command escapes control/bidi characters only after
   the complete result. No partial network or worker output becomes a
   successful UI presentation.
4. `textServiceStream.Close` half-closes local input, allows a bounded native
   terminal exchange, retires a recovery tail when present, and joins the run
   goroutine. It returns cleanup results retained by Endpoint, but the
   ordinary native `Done()` closes before `stream.close()` completes; that
   signal alone does not prove physical retirement has finished. The tail
   path closes `Done()` after retirement. Initial Attachment cleanup has an
   Endpoint wrapper that caches its
   Route close error. Replacement Attachment cleanup currently crosses a
   `func()` native callback and drops the Route close result; see
   [F-23](repository-reconstruction-findings.md#f-23-replacement-service-attachment-loses-its-route-close-result).

Source-inspected tests cover Instance proof, lost terminal receipt, offset
rollback, bounded recovery, real TLS/document exchange, and recovery over
both Carriers. They do not cover a distinct replacement Route close failure
propagating to the final Endpoint outcome. This trace is not a fresh passing
installed qualification result, and the Endpoint agent's unfinished change
must be reconciled before treating these file boundaries as settled.

## Source trace: retired Node duty input

`cmd/ardents-node/node_config.go:readNodePlan` decodes the operator plan,
checks its schema and required fields, then calls `oldNodeDutyReservation`.
Each former `rendezvous`, `initiator`, `introduction`, `responder`, or
`transit_issuer` reservation returns `errOldNodeDutyRetired` before key-file
reads, State-root opening, or Node startup. The direct
`node_duty_retirement_test.go` covers all five and a mixed former/closed plan,
asserting the State and local-role roots were not created. A later branch in
the parser still names interactive Route v2 but is dominated by this refusal
(F-46). The current closed reservations choose `route.ClosedRouteProfile`.
This trace covers command input refusal, not every lower-level State/Node
compatibility reader or every old wire form.

## Source trace: local runtime diagnostic timeline

This trace follows `internal/node/event_writer.go`,
`cmd/ardents-node/node_event_output.go` and `source_runtime.go`,
`internal/endpoint/text_participant_observation_linux.go`,
`cmd/ardents/endpoint_text_linux.go`, and
`internal/diagnostics/timeline/project.go` at `e48d4c3c`.

1. Node supplies a bounded event writer to `node.Run`. The writer obeys the
   caller deadline, writes one JSON line, and optionally replaces two latest
   diagnostic snapshots. Lifecycle/resource output failure returns to the
   Node lifecycle rather than silently asserting readiness.
2. Source emits its ready event after opening current State. A wait or close
   failure emits a fixed `source-failed` category and joins event-write failure
   with the underlying result. Pre-admission failures intentionally have no
   ready-to-terminal event sequence.
3. Endpoint serializes event emission and stamps the occurrence time. Its
   background publication/Connection failure callbacks use a bounded output
   context; the first failed write interrupts the participant wait. The
   command writes the schema to a pollable owned output and reports a bounded
   startup/running category if that output is still usable.
4. `ardents diagnostics timeline` reads app JSON lines or authorized journal
   JSON from standard input, projects only the three recognized schemas into
   one tabular stream in input order, and retains no raw input. The displayed
   event time does not reorder records or prove cross-process causality (F-63).
   It rejects corrupt
   recognized records without echoing their bytes; unknown schemas are
   ignored. The command reference gives a live `journalctl -f -o json` pipe.

`internal/diagnostics/timeline/project_test.go` checks cross-owner projection
of already ordered input, redaction, malformed categories, and cancellation;
the command adapter has a
stdin projection test. This is a functioning operator navigation path, not an
attempt-level Route trace or a complete retained event store.

## Source trace: ACA1 transition reader and ACA2 corpus inspection

This trace follows `cmd/ardents-control/main.go`,
`internal/alphacontrol/inspection/inspect.go` and `aca2_corpus.go`,
`internal/alphacontrol/reader.go`, and the two catalog decoders at the mapped
HEAD `8cd1575f`.

1. `inspect-bundle` and `inspect-transitions` verify an enrollment input,
   derive independently pinned disclosure and three component keys, then
   open an ACA1 reader root. The reader owns its lock and catalog floor; an
   accepted catalog commits the floor before inspection returns. The command
   closes that reader, while Release and Network checks use their own
   inspection roots. The callback may durably advance Release and Network
   floors; all three close results are currently discarded (F-64).
2. `inspect-alpha-corpus` accepts explicit ACA2 catalog and corpus bytes plus
   supplied keys and decision time. It verifies the four-component catalog
   and corpus without opening the ACA1 root or committing a corpus floor.
3. Both routes report inspection evidence only. Neither calls an Endpoint acceptance
   operation. `accept-alpha-corpus` is a fixed retired-command refusal in
   `cmd/ardents-control/main.go`.

The two input grammars and different floor lifetimes are a real maintenance
split (F-49). Replacing ACA1 with ACA2 requires a decision about the current
transition report and retained ACA1 reader roots; switching a parser alone
would lose its anti-rollback state.

## Source trace: retained Namespace without a C0 opener

The current `docs/technical/naming.md` describes a local chain from
Authority transition through Custody signing, Namespace pending state and
Epoch materialization. In production source, `internal/naming/resolution`
formerly imported Namespace interfaces; ADR-0100 removed that package with
its tests and the OHTTP dependency closure, and Custody
implements two Namespace-specific `Vault.Execute` cases. A non-test search
over the command adapters finds no import of Namespace, no
construction of those Custody operation kinds, and no call to `epoch.Open`.
`ardents name resolve` and `name control` refuse before effects under
ADR-0090; `name encode` uses the separate canonical encoder. Consequently,
none of these commands opens a Namespace root, starts a Gateway/Resolver,
commits a pending successor, or verifies a live proof for C0. The 54-file
Namespace closure is retained technical behavior and a persisted-evidence
obligation (F-51), not a hidden C0 Service Name path; its durable roots
await the separate PO disposition decision.

## Source trace: old Transit Grant spend inside the current local-role root

At the current architecture worktree, a non-test `cmd`/`internal` call search
finds no caller of `route.VerifyTransitGrant` or
`network/duty.(*store).SpendTransitGrant`. The old Route verifier still reads
its Grant v1 body through `route/wire_encoding.go:wireReader`, but it does not
admit a current Node connection. Node `local_roles.go` and State
`local_roles.go` still open the `network/duty` root; the Endpoint qualification
preflight also opens it. Thus the root is a current resource owner even though
the Grant-spend operation is uncalled.

The root's strict version-1 JSON generation includes `TransitGrantSpends`,
and `loadGeneration` validates that field before returning an owned root.
`Replace` carries forward only unexpired spends and commits a new hashed
generation under a watermark; `Open` alone does not prune them. Retained
current/predecessor generations can therefore contain historical spend
records. This is a persisted-schema and rollback question, not evidence of a
second supported Grant admission path (F-53). Any schema cleanup needs an
explicit disposition for existing roots and their generation chain while
preserving the still-current duty conflict records.

## Source trace: JOIN transport transfer into Service Connection

The selected closed wire has two cohesive, authority-free grammar packages.
`internal/route/ardp` encodes only generation-3 frames: a 16-byte header,
body bound of 16 KiB, exact 209-byte HELLO, one-byte bootstrap selector and
five-byte ACCEPT with zero credit on refusal. `ReadFrame` checks the header
and declared bound before allocating a body. `ValidFrame` checks kind and
basic lane/body shape, but cannot decide whether a 49-byte Endpoint-role OPEN
or 50-byte Node-outer OPEN is authorized: the authenticated channel in Route
chooses the contextual decoder. `route.DecodeClosedNodeOpen` refuses the
retired 49-byte Node form. Node and Route own State, admission, lane credit,
Carrier and cleanup; decoding a frame grants none of those. Its direct test
checks fixed v3 framing, rejects generation 2 and over-limit length before
body read, and checks invalid kinds and lane forms.

`internal/route/capsule` owns one 4096-byte Introduction submission, its
80-byte visible header, 360-byte ciphertext and exact 344-byte recipient-only
plaintext. It binds the profile and complete visible header through the
selected HPKE transcript; `Seal` returns the encrypted capsule and a digest
of the plaintext. `Open` delegates private-key use to an Instance-owned
`Recipient` interface and rechecks profile, revision, expiry and plaintext
bounds. Route/Node carry the opaque operation, while Endpoint verifies live
publication, registration, replay and Rendezvous eligibility. Its direct
test checks literal schema offsets and transcript independently, mutated
header/ciphertext/profile refusal, expiry, fixed padding and surplus refusal.
These packages are wire/crypto boundaries, not alternate Route composition
or a second accepted version.

This trace follows Endpoint `text_join_service.go`,
`text_introduction_exchange_lifetime.go`, `text_service_stream.go`,
`protected_service_attachment.go`, Route `closed_join_client_stream.go`,
and native `service/connection/stream_contract.go` at `53f02e64`.

1. Endpoint reserves an Introduction exchange before network I/O. Its
   opening lifetime stays cancellable by the caller until transfer; the
   source or responder acquisition is retained under the Context lock. JOIN
   and, for Reader, capsule submission run concurrently. The caller consumes
   both results, cancels the other flight after a failure, and closes any
   returned Route stream plus the acquisition if transfer did not occur.
2. On success, Endpoint detaches the setup caller, rechecks Job and authority,
   then retains the exchange and Route stream. `textJoinedTransport` owns the
   stream, its prefix acquisition and the finish callback. Its `Close` joins
   `ClosedJoinedStream.Close`, cancels the retained exchange, records its
   terminal result and releases the acquisition exactly once. Route's Close
   itself joins lane shutdown, a bounded peer-close wait, channel close and
   its worker result. A JOIN transport alone has no Service authority.
3. `openTextServiceStreamWithRecovery` immediately wraps that transport in
   `protectedServiceTransport` and owns it on every early return. Only after
   Service TLS, Instance authentication and native Stream construction does
   it transfer the Attachment to the running Service Connection. Its final
   cleanup joins the owned Application half, physical transport, recovery
   reservation and Publisher lease before `textServiceStream.finished` closes.
4. The initial wrapper retains the physical close result. A replacement
   transport bypasses that wrapper and crosses native Attachment's `func()`
   close callback; the callback discards its close result (F-23). Also,
   ordinary native `Done()` currently closes before physical `stream.close`,
   whereas the terminal tail closes it afterwards. These are exact result and
   completion-barrier gaps at the otherwise explicit ownership transfer.

The source proves the normal and opening-failure transfer order, not every
recipient timeout or installed shutdown outcome. Preserve the distinct
setup-caller, Job and retained-cleanup lifetimes when moving files between
Endpoint and Route. The active Endpoint task owns any code correction.

### Publisher Introduction registration and delivery lifetime

`route/closed_introduction_client.go` and
`route/closed_introduction_delivery.go` own one admitted registration channel;
`endpoint/text_introduction_registration.go` owns its use by one Publisher
Context. This is a source trace of those files at `53f02e64`, not a claim that
the complete installed Publisher path has passed.

1. Endpoint reserves a registration flight against its current Introduction
   prefix, checks Publication-token stock, and issues a fresh slot. Route
   bounds expiry by the selected peer and prefix, opens a Source lane, performs
   Role TLS and HELLO/ADMIT, and accepts a registration only after a matching
   zero-status RESULT. Before handover, Route's deferred `closeTransport`
   joins the lane and any Source cleanup. After handover, its reader owns that
   cleanup and closes `Done` only after it finishes.
2. Endpoint rechecks flight, prefix, Administration lease and publication
   state before installing the registration. A lost flight closes the Route
   channel and joins its cleanup failure into the Context terminal result.
   Registration alone does not publish a Descriptor; a later verified
   Descriptor ACK establishes readiness and predecessor overlap.
3. Route buffers at most 16 pending capsule deliveries, validates their slot,
   revision, expiry, even increasing lane, rate and total byte budget. A
   caller claims a delivery through `TakeDelivery`; `Complete` writes one
   RESULT and waits for the matching terminal CLOSE. Cancellation interrupts
   a blocked write. A registration ending closes pending delivery `done`
   signals with its outcome and cleanup result.
4. `Withdraw` reserves one withdrawal, writes its operation with a deadline,
   waits for the authenticated RESULT and EOF, then Route closes the lane.
   Endpoint stops refresh, cancels the registration and joins its close result
   and any predecessor recipient before returning. Route `EndReason` exposes
   only a fixed local category after `Done`; it does not expose raw peer data.
   Route `Close` joins reader and transport cleanup but does not return the
   already classified protocol `outcome` separately.

The inspected Route tests cover cancelled withdrawal and RESULT writes,
foreign/reused/over-budget delivery refusal, retained budget accounting and
fixed end-reason categories. They do not by themselves prove installed
Descriptor ACK, recipient process lifetime, or the complete refusal/timeout
matrix across Endpoint and Node. Keep the channel and Context close owners
separate if the package boundary moves.

## Source trace: current closed Entry set versus retained Invite import

This trace follows `cmd/ardents/entry_import.go`, `internal/entry/{open,
import,attempt,closed_set_store,closed_sets}.go`, Endpoint
`text_source_state.go` and `text_interior_set.go` at `53f02e64`.

1. The dispatchable `ardents entry import` command reads an operator plan and
   signed Invite, opens the older Entry root through `entry.Open`, calls
   `owner.Import`, and closes the owner before emitting a receipt. `Open`
   claims an exclusive root lease, recovers interrupted contacts/attempts,
   revalidates retained Invites against current State and persists changes.
   `Close` cancels acquisition, waits for it, joins accepted attachments and
   their cleanup outcomes, settles the attempt, then releases the root lease.
   `entry recipient` reads the corresponding recipient public key. This
   command still writes an older durable format, but it does not feed the
   selected protected participant.
2. The current Endpoint opens `entry.OpenClosedSets` under `textMu` using its
   separate closed Entry root and a callback to live State-selected adjacent
   members. The root marker rejects an Invite root. First creation commits
   state before returning; an interrupted or malformed claim refuses. On
   restart, the store validates retained network identity, generation and
   watermark and retains its prior selections.
3. `ClosedSets.Members` selects and durably commits exactly two members for a
   Role Domain before use. `CurrentMember` rechecks one chosen slot against
   live State; it cannot draw a replacement or advance an expired set. Endpoint
   closes the Entry-set lease with the token journal after closing text
   contexts, joining their close errors. Tests cover restart, no refill,
   concurrent activation, conflicting state and legacy-root refusal.
4. The old Route `OpenEntryAttachment` was removed by ADR-0093, and ADR-0095
   retired the then-uncalled attachment execution machinery: `owner.Acquire`,
   `owner.Contact`, the guarded carrier, the cleanup leases, and the
   attempt-journal writers. The durable attempt/contact journal schema stays
   decodable; `Open` terminalizes a legacy journal as interrupted. The command
   still uses the private `validateInvite` through `owner.Import` and reopen.
   ADR-0096 then rejected the closed-alpha candidate surface: `entry.Issue`,
   exported `entry.Verify`, its `Authorization` result, and the
   reservation/`Insufficient` policy are removed; only `validateInvite`
   remains, reading retained Invite records.

The two roots are not version negotiation in one C0 journey. The target has
one closed Entry selection path. The uncalled v2 attachment machinery is
retired (ADR-0095); retirement of the Invite writer still needs a decision
for existing roots and the accepted operator contract, including the retained
attempt/contact journal schema; preserving a bounded historical reader or
typed refusal during that transition does not authorize old admission
execution (F-08).

## Source trace: Release decision root before Endpoint enrollment/replacement

This trace follows `internal/release/{store,evaluate,store_persist}.go`,
`cmd/ardents/endpoint.go`, `cmd/ardents/endpoint_replace.go` and the
`internal/enrollment` input projection at `53f02e64`.

1. The enrollment verifier supplies pinned Release inputs. The command opens
   one exclusive `release.Verifier` under `StateHome/floors/release-decision`,
   calls `Evaluate`, then closes and checks the root-release result before it
   accepts authorization for initial replacement. A close failure blocks the
   command. The replacement command similarly loads its explicit bundle and
   evaluates the same durable root before preparing an operation.
2. `Open` validates the claimed root and retained floors before evaluation.
   `Evaluate` authenticates the offline metadata and target identity, checks
   build/protocol outcomes, and commits successor floors only for accepted or
   no-update data that advances them. Verified root-chain publication can
   precede that final decision; rejection retains roots already verified in
   order, without publishing executable metadata floors. `Decision` exposes
   accepted authorization only after these checks.
3. This Release root belongs to the enrollment/replacement trust path, not
   the separate protected `endpoint headless` startup. A successful Release
   decision does not establish the installed Publisher/Reader journey or
   transfer its state to the protected text participant. The combined
   installation handoff remains an explicit missing composition step (F-25).

## Source trace: Contributor retirement without a start path

This trace follows `cmd/ardents-node/{contributor_mode,systemd_linux}.go`
and `internal/contributor/{contract,control,recovery,diagnostics,
installation_record}.go` at `53f02e64`.

1. The command parser recognizes `apply` and `restart` but returns their
   retirement error before creating a host environment, opening a root or
   calling systemd. The dispatchable routes are `diagnose`, `drain`,
   `withdraw` and confirmed `remove`. The command opens a Profile without
   effects, then calls `Control` and emits a bounded report.
2. `Control` admits only those four actions, claims an exclusive root lease,
   authenticates the installation record and reconciles an interrupted
   update before performing the requested action. The recovery path verifies
   either the committed current generation or its exact predecessor, may
   Stop an active predecessor, and never invokes Start/Restart/Enable. An
   ambiguous or foreign residue refuses; the lease-release error joins the
   result even on failure.
3. Diagnose verifies installed files, management executable, lifecycle event
   and systemd status. Drain Stops and waits at most 15 seconds for
   `WITHDRAWN`; Withdraw also Disables. Remove requires the exact deployment
   confirmation and inactive, disabled, `WITHDRAWN` status before deleting
   managed paths and reloading systemd. Tests cover interrupted recovery,
   no-start behavior, concurrent lease exclusion and removal preconditions.

This package is a live *retirement* owner, not an alternate C0 Node duty.
Removing it requires evidence that its accepted already-owned installation
and recovery obligations are closed; its presence alone does not justify
preserving an old runtime (F-56).

## Source trace: process pressure versus shared host-period accounting

This trace follows `internal/resource/{contract,hosting_ledger,
hosting_sample}.go`, `internal/network/state/resources.go` and
`internal/node/resource_pressure.go` at `53f02e64`.

1. State's owned governor samples its process `Guard` once per second. A
   measurement or resource-event write failure sets State's resource error,
   enters protect and cancels work. A drain observation does the same with a
   bounded emergency cause. Node uses a separate Guard for its own process;
   its duty combines that observation with the shared Hosting observation.
2. `Hosting` opens an existing, operator initialized provider period and
   initially samples it. `Sample` may reuse a recent committed observation;
   an expired sample elects one writer to charge interface-counter deltas.
   `Reserve` always transacts durably, committing both work and termination
   capacity before admitting effects. The reservation stays charged until
   its consumer joins work and calls `Release`; an ambiguous release is not
   retried as a second refund. `Close` releases the handle without clearing
   remaining durable reservations.
3. The resource package measures and classifies. State owns State withdrawal,
   Node owns the duty's protect/drain action and bounded event reason, and the
   command owns event output destinations. The diagnostic timeline projects
   those events after emission; it does not control pressure or own the root.
4. For class-1/3 control admissions, Node's verifier reserves from the shared
   Hosting handle. Resolution, Introduction and admitted Issuer handlers defer
   the Route admission's `Release` but discard its durable release result.
   Their server drain results therefore do not prove a successful Hosting
   refund. On a bounded drain timeout, `node.Run` closes that shared handle
   although a child may finish later and try to release through it. The
   ledger retains the reservation in that case; F-62 records the missing
   outcome owner and late-close ordering. Data JOIN's transferred claim and
   cleanup accumulator are a different path.

## Source trace: Portable Endpoint compatibility forms and replacement recovery

This trace follows `cmd/ardents/{endpoint,endpoint_user_unit,endpoint_replace}.go`,
`docs/reference/commands.md`, `tests/e2e/endpoint/{enrollment_process,
enrolled_runtime_process_unix,replacement_process_unix}_test.go` and
`internal/endpoint/replacement` at `53f02e64`.

1. The one-argument `endpoint enrollment-check` verifies a retained v1
   enrollment JSON input; the current two-argument form verifies a bundle
   against an independently delivered manifest pin. The one-argument
   `endpoint enroll` supplies its legacy verifier callback to the same
   `runEnrolledEndpoint` function as the pinned form. The v1 restart process
   test removes the original bundle before restarting an already selected
   executable through that old command shape. These are two accepted input
   shapes at a command boundary, not two Portable runtimes. The legacy
   check has a process test; this trace does not establish a fresh legacy
   first-install acceptance path.
2. `endpoint user-unit <old-input>` loads the retained JSON and calls the
   current renderer with its bundle root and manifest digest. Its output
   invokes `endpoint enroll <bundle-root> <manifest-sha256>`; rendering never
   writes, enables or starts a unit. The reference command contract retains
   the old forms for an existing unit's restart and explicit migration.
   The searched command and Endpoint process tests exercise the pinned
   renderer, but did not locate an exact legacy-renderer process test.
3. `endpoint replacement-recovery` calls `replacement.Recover` to report a
   durable classification and, if required, the exact retained predecessor
   path. It does not start, replace or roll back a program. `endpoint rollback`
   first verifies that its own executable is that predecessor, then opens a
   fresh Release decision for the supplied bundle and calls the replacement
   owner's rollback operation. The Unix process test runs the predecessor
   rollback after a failed candidate and checks the systemd calls and
   restored executable; module tests cover recovery classification and
   residue. An exact command-level recovery test was not found in this
   targeted search.

The supported C0 command shape can be singular while a bounded old-input
adapter remains for installed units. Its retirement needs evidence that those
units have migrated; it does not require preserving a second execution
implementation (F-57).

### Replacement transaction and interrupted-state ownership

At `53f02e64`, this trace follows `cmd/ardents/endpoint_replace.go`,
`replacement/{contract,operation}.go`, `operation_test.go`, and
`preactivation_retry_linux_test.go`. The current technical owner is
`release-update-custody.md`; its foreground retry table agrees with the
source's preactivation phases.

1. `endpoint replace` verifies that its own executable matches the committed
   current record **before** opening the supplied bundle. `LoadBundle` and
   `release.Open(.../floors/release-decision).Evaluate` authenticate the exact
   candidate and advance Release floors; the command closes that verifier
   before passing opaque authorization to `replacement.Replace`. It accepts
   `release-accepted` or, for an exact bounded retry, `no-update`. A failed
   Release close is returned, not treated as authorization.
2. Under one replacement-state lease, `Replace` rechecks the current executable
   bytes and record, prepares the exact candidate, journals `prepared`, retains
   the predecessor, journals `rollback-retained`, stages beside the program and
   journals `staged`. Only then does its caller-owned `Unit.Stop` run. After
   atomic activation it journals `activated`, runs the candidate's no-network
   self-test, commits the new record, journals `committed`, and calls
   `Unit.Start`. A failed stop leaves the original program; a failed self-test
   records `self-test-failed` and requires a new Release-authorized rollback.
   Start failure returns `committed-start-failed`, not success or implicit
   rollback.
3. `Recover` opens the state read-only. With a preactivation journal and the
   original executable it returns `keep-current`; after activation without
   committed current it returns `self-test-required`; a failed candidate test
   returns `rollback-authorization-required`; matching committed bytes return
   `committed-restart-permitted`. Unknown byte/journal combinations return
   `repair-required`. The four injected durable checkpoints in
   `TestReplaceInterruptionLeavesOnlyExplicitRecoveryPaths` assert these
   classifications and program bytes. Retry tests cover exact `no-update`
   authorization after stop or staging failure, reject a substituted record,
   and retain evidence under concurrent attempts. These are module/process
   boundary tests, not power-loss qualification.
4. `endpoint rollback` must run from the retained predecessor copy. It verifies
   that copy and obtains a **fresh** `release-accepted` decision for its exact
   bytes; `Rollback` requires the failed-self-test journal and retained digest.
   The old program is thus recovery evidence and a separately authorized
   candidate, not a second continuously supported Endpoint version.

The command adapter still controls `ardents-endpoint.service` with
`systemctl --user`. The selected protected worker requires a root-owned
system-managed unit and account (F-27); this transaction trace does not prove
that the existing replacement operation can stop, activate and restart that
protected unit without breaking enrollment, Release floors or confinement.
That integration remains an explicit C0 design and installed-verification gap.

## Source trace: offline closed-profile and Custody commands

This trace follows `cmd/ardents-control/{closed_profile,
closed_issuer_inspection}.go`, `cmd/ardents-custody/{inspect_envelope,
verify_record,recovery_bundle,purge_record}.go`, their direct owner calls and
the focused command tests at `53f02e64`.

1. Control `prepare-closed-profile` parses a bounded public plan and creates
   one new unsigned output file. `sign-closed-profile` reads an owner-only
   PKCS#8 signing key, signs the plan and creates a new signed output; the
   key copy is zeroed on return. Both destinations use create-exclusive mode
   and reopen comparison. `inspect-closed-profile` verifies a supplied
   signed profile against the plan, pinned authority and explicit time,
   emitting a bounded report without writing a profile. `inspect-closed-
   issuer-profile` validates a public Node export against independently
   supplied Node key and identities; it does not perform wall-clock
   acceptance or commit State. Command tests and the Node provisioning
   process test exercise the prepare/sign/inspect route and refusal cases.
2. Custody `inspect-envelope` reads only a public canonical envelope header
   and never opens a Vault. `verify-record` does open a Vault, obtains a
   password interactively, authenticates the exact binding and retained
   floor, and emits bounded facts without releasing root material. Although
   this verification does not change a record or floor, `custody.Open` can
   create the Vault root, records/quarantine directories and lock file on an
   absent path. It is therefore not a filesystem read-only operation.
3. Custody export writes a separately passworded Recovery Bundle and checks
   that it can be restored. Restore writes only a locked quarantine record,
   not activated Authority. Purge verifies the encrypted record and public
   binding, obtains explicit terminal confirmation, removes exactly that
   record and syncs its directory while retaining its Authority floor. Each
   command joins `Vault.Close` errors into the result. Command tests cover
   export/restore and purge; module tests cover the bound record verification,
   refusal and floor outcomes. These offline actions are distinct from the
   installed Endpoint/Node accepting runtime.

## Source trace: qualification execution and evidence verification routes

This trace follows `cmd/ardents-qualification/{main,preflight_linux,
completed_evidence_linux,paired_evidence_linux,network_manifest_linux,
net14v_verdict_linux,failed_net14v_linux}.go` at `53f02e64`.

1. The Linux `run <plan>` path checks the installed Endpoint artifact,
   decodes and hashes the local plan, records the binary/build and environment
   identity, then invokes `endpoint.RunStreamQualification` for selected
   participants. The artifact check requires the fixed root-installed binary,
   plan and system unit with exact hashes and verifies `/proc/self/exe` is that
   installed binary before participant work. The plan refuses mixed Reader and
   Publisher owners: one Publisher or all four Readers share a period, seed,
   profile and condition. Participants run concurrently; one failure cancels
   the group, but `run` consumes every participant result before emitting one
   result per participant and the final journal terminal. An output failure
   remains in the final error. `preflight` checks the same artifact and plan,
   then calls `endpoint.PreflightStreamQualification` for each selection; this
   is a readiness check, not a workload verdict.
2. `verify-run` validates a completed runner JSONL file and emits its digest
   and byte count. `verify-pair` combines two runner outputs with a network
   manifest, relay and Node evidence, inventory identity and cleanup evidence;
   it refuses failed participant criteria, mismatched candidate/unit
   identities and incomplete owner or topology facts. `verify-network-manifest`
   validates one bounded manifest, including exact Carrier spelling, two
   directional paths and the expected isolated relay topology, and prints
   its digest and structural counts.
3. `verify-net14v` compares accepted baseline and recovery paired verdicts,
   manifest bindings, recovery fault schedule, topology, bytes and bitrate
   criteria. `verify-failed-net14v` instead consumes failed relay evidence
   and requires an incomplete or failed episode to be accounted for within
   its bounded criteria. Both print their criteria and return an error on a
   failed verdict. They are evidence consumers, not alternate Endpoint or
   Route versions. The journal records a SHA-256/count terminal after the
   joined participant results; `readCompletedEvidence` refuses truncation,
   tampering, a missing/failed terminal, duplicate or absent participant
   results, and a record after terminal. The direct plan tests cover mixed
   owners and missing Reader inventory; journal tests cover short writes,
   incomplete evidence and an early artifact refusal through `run`. Other
   verifier tests exercise their evidence helpers. This source trace locates
   the effects and refusal conditions; it does not claim that installed
   P5/P7/P8 or NET-14V evidence has passed on this worktree. In particular,
   `make check` does not invoke `text-command-network-check`; that separately
   selected installed ordinary-command journey is required for the C0
   Publisher/Reader verdict and cannot be inferred from this stream runner.

## Source trace: physical Carrier transfer across Route, Endpoint and Node

This trace follows `route/{closed_role_carrier_client_linux,
closed_bootstrap_exchange,closed_source_prefix,closed_source_channels,
closed_carrier_pool,closed_shared_carrier}.go` and
`node/{closed_forwarding_link,closed_forwarding_carrier,
closed_forwarding_shutdown,closed_forwarding_listener}.go` at `53f02e64`.

1. A direct-role `OpenClosedRoleCarrier` returns one authenticated TCP/TLS or
   QUIC `net.Conn`; it does not select a recipient. The one-shot bootstrap
   exchange retains its own `closedRoleRetirement`, interrupts the physical
   connection before joining nested child streams, and adds a close failure
   to its returned error. A retained Source/Introduction/Responder prefix
   instead transfers that connection to `ClosedSourcePrefix`. On opening
   failure it joins `prefix.Close`; on normal close, channel workers join
   before physical retirement, child close and callback completion. Its
   `Close` returns `ErrClosedSourceCleanup` with the physical cause. Endpoint
   owns the returned prefix handle and its final result.
2. Node's forwarding duty selects and revalidates the exact State peer before
   asking `ClosedCarrierPool.AcquireContext` to dial a Node Carrier. The pool
   serializes an exact pair's opening, retains only Carriers marked used,
   and wraps the physical Carrier in an exactly-once retirement result. Each
   link borrows a lease; a session owns one reader and writer per retained
   Carrier. The link's `close` joins its reverse copier and returns lease
   release error. Failure and invalidation close the physical Carrier;
   `closedForwardingSessions` retains reader cleanup errors.
3. The shared listener returns a classified authenticated `net.Conn` whose
   close belongs to the accepted Node or Credential handler. Listener
   `Close` stops acceptance; it does not join those handlers. The forwarding
   server cancels and closes its outgoing pool, joins accepted producers and
   session readers, then closes receiving resources and Hosting in its own
   `finishShutdown` goroutine. Its bounded `Drain` may time out while that
   owner continues cleanup. The current Issuer adapter differs: on a drain
   timeout its key and spend roots lack a later close owner (F-17).

The non-test caller check covers all ten Carrier move candidates: Node
forwarding calls `OpenClosedNodeCarrier` and owns `ClosedCarrierPool`; the five
Node duties call `ListenClosedSharedCarrier` and the Issuer passes its shared
listener to Credential. Credential's direct path calls
`ListenClosedRoleCarrier`. Route's
bootstrap exchange and retained prefixes call `OpenClosedRoleCarrier`.
The direct TCP role path uses `OpenClosedRoleTLS`; QUIC uses the same role TLS
state check in its handshake. `ClosedRoleTLSExporter` is used by Route and
Node admission and returns a function type declared in
`closed_admission_channel.go`. `exactPeer`, literal endpoint validation, the
Route ALPN and role TLS helpers are a shared security closure. A package move
must assign that closure without making a new Carrier package import Route
back; moving only the ten filename candidates would create a cycle or split
the peer-authentication rule.

Carrier extraction must preserve these three result destinations. A single
physical package may supply all three mechanics, but it cannot own the
Endpoint prefix, Node accepted-handler join, State selection, or issuer root.
The pool's open-failure cleanup is recorded in its eventual terminal result;
some immediate forwarding error paths discard a lease release error while
returning the opening failure. That distinction needs a focused final-result
check during the boundary slice, rather than claiming every per-attempt error
contains every physical close failure.

## Source trace: Route client prefix operation ownership

At `53f02e64`, the non-test caller chain for the 25 Route-root client-path
files has three distinct lifetimes:

1. Endpoint's `text_issuance_operation.go` calls
   `ExchangeClosedBootstrap`, which prepares and rechecks current State and
   closes its one-shot Entry/Interior transport before returning.
   `text_introduction_recipient.go` calls
   `InspectClosedDataJoinRecipient`; that inspection rechecks State without
   opening a channel. Both use the shared `ClosedBootstrapState` projection.
2. `text_source_prefix.go` opens a Source prefix; `text_publisher_prefix.go`
   opens separate Introduction or Responder prefixes. Each returned
   `ClosedSourcePrefix` owns one physical parent, child stream and
   `closedSourceChannels` owner. Its channel, lane, credit, queue, lifetime
   and write-witness files implement one parent/child resource closure.
   `closedRoleChildStream` and terminal-write priority are used only by
   outgoing bootstrap and prefix paths; they follow this same owner.
   Endpoint retains and ultimately closes each prefix; a new operation does
   not independently own that parent.
3. `text_source_lifecycle.go` borrows the prefix for issuer, Descriptor,
   submission, recipient and replenishment operations;
   `text_introduction_prefix_lifecycle.go` and
   `text_responder_prefix_lifecycle.go` borrow their own prefixes. Route's
   control and operation methods recheck the selected State and return
   bounded results or child owners. Introduction delivery stays under its
   retained registration, while terminal-recipient selection opens no
   channel. Endpoint owns publication, token attempt and pairing decisions.

This caller and resource closure supports a client-path operation boundary,
but not a bulk move of every `closed_source_*` file into an independent
package. A move must preserve the parent close result, child join, per-effect
State recheck and the one-shot-versus-retained distinction. Existing
`closed_source_channels`, stop, nested-retirement and replenishment tests
exercise those mechanics; the combined Endpoint journey remains separate
acceptance evidence.

## Source trace: receiving wire and JOIN resource owners

At `53f02e64`, the 13 Route files still at first-source-pass in the receiving
wire cluster have these non-test callers and transfer rules:

| Route closure | Caller and returned owner | Resource boundary |
| --- | --- | --- |
| `closed_outer_handshake.go`, `closed_outer_bridge.go`, `closed_outer_bridge_shutdown.go`, `closed_outer_admission.go`, `closed_outer_write.go` | Node issuer, forwarding, resolution, Introduction and Data JOIN construct `ClosedOuterHandshake`; `node/closed_outer_lifetime.go` constructs the bridge and gives each accepted lane to a duty handler. | Route validates and multiplexes virtual lanes. Node owns the accepted physical `net.Conn`, interrupts it, closes the bridge and joins all handler goroutines. Bridge close alone neither joins handlers nor releases Node roots. |
| `closed_forwarding_channel.go`, `closed_forwarding_reverse.go`, `closed_duty_control_queue.go`, `closed_node_open.go` | Node forwarding constructs the admitted channel, supplies authorization/replenishment callbacks and encodes its adjacent OPEN; Route checks OPEN, reverse frames, credit and control budget. | Route owns per-lane framing and admission accounting. Node retains outgoing Carrier leases, receiving spend, Hosting and the duty shutdown result. The OPEN file also holds a bridge-lane restriction accessor; split by responsibility within Route if its placement changes. |
| `closed_join_pairing.go`, `closed_join_accept.go`, `closed_join_replenishment.go`, `closed_join_stream.go` | Node Data JOIN creates one `ClosedJoinPairs`, then calls `AcceptStream` for admitted connections. Route verifies exporter binding, reserves one pair, owns its queue/timer, and transfers a matched side into `ClosedJoinSide.Serve`. | On refused transfer, Node retains the connection/admission. On accepted work, Route owns the pair and stream protocol; Node waits for accepted workers, closes pairs, then closes its spend and Hosting roots. A timeout cannot release those roots ahead of a child. |

The receiving-wire package boundary is therefore below Node's duty lifecycle.
It can expose a bounded lane/pair result while keeping physical accept, spend
root, host pressure and terminal cleanup with Node. These files cannot be
moved as one `closed_*` block with Carrier or Endpoint's retained client
prefixes; their accepted-child and close owners differ.

## Source trace: remaining mixed owner files

The last ten focused boundary/split rows at `53f02e64` have concrete callers
and remain conditional on the adjacent Endpoint implementation slice:

| Files | Non-test caller and present owner | Boundary decision |
| --- | --- | --- |
| `endpoint/application_half_close.go`, `text_service_publisher_linux.go`, `text_service_route_recovery.go` | `text_service_stream` constructs the half-close Application pipe; `text_publisher_start_linux` runs the worker operation; `text_join_service` supplies the recovery opener to native Service Connection. | The pipe and worker belong to Endpoint's local Application bridge; recovery revalidates the immutable Service binding before acquiring a fresh Route transport. Reconcile the adjacent Endpoint changes before changing their interfaces. |
| `endpoint/text_source_prefix.go`, `text_source_state.go` | `textContext.openTextPrefix` reserves one stock-to-Route opening; `endpoint.textTokenJournal` lazily opens its durable journal in the same file. `closedTextRoleMembers` joins current State and local conflict facts, while `textEntrySets` opens the separate durable Entry root and `closeTextSourceRoots` joins root closes. | Split local file responsibilities between projection/opening and durable-root ownership without transferring Endpoint's lock or root lifetime to Route. |
| `node/closed_forwarding_admission.go` | Forwarding passes its token verifier and replenisher to Route's admitted channel. It reserves Hosting capacity before token spend and returns a release callback; a failed replenishment spend joins the release result. | Node owns host-period reservation, State/current-profile checks and the Replay root; Route performs bounded lane admission and the spend operation. Do not move this callback into a generic wire owner. |
| `route/closed_role_tls.go`, `closed_role_tls_client_linux.go`, `closed_terminal_recipient.go` | Direct TCP role Carrier and QUIC handshake share the exact role TLS state rule; the retained Source/Responder path calls terminal-recipient selection before operation I/O. | Keep role TLS with the Carrier authentication closure. Recipient selection rechecks State and opens no channel, so it follows the client path rather than receiving wire. |
| `service/publication/publication.go` | Endpoint opens the exclusive Publication root and borrows live leases for Descriptor, Introduction and Service attachment work. | Publication withdraws before draining leases, then erases signer and releases its own root; its `Open/AcquireAt/Floor/Unpublish/Close` facade stays with that authority. The `Publish` path is in other files of the same owner. |

These are caller/resource traces, not approval to move a file or a claim that
the adjacent Endpoint branch has been integrated. The first two rows require
a post-integration diff before their final file placement is frozen.
