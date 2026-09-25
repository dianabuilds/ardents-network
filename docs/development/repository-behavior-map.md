# Repository behavior trace map

Status: **working implementation trace**, not a second product contract or a
qualification verdict. Baseline: `codex/architecture-refactor` at `50026274`,
with concurrent Endpoint edits still in progress. The current requirement is
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
| Accept current Network State and refuse stale/conflicting profile authority | [Network/Route/Node](../technical/network-route-node.md) | `cmd/ardents/offline.go`, `cmd/ardents-node/source_runtime.go` -> `internal/network/state`, `internal/network/source` | Offline Epoch and closed-profile trace below; Source-wave and full floor trace pending |
| Create separate Service and admission authority material and issue bounded public credentials | [Release/update/custody](../technical/release-update-custody.md) | `cmd/ardents-custody/command.go` -> `internal/custody`, `internal/service/instance`, `internal/route/credential` | Service Credential and admission permission source traces below; full Vault rollback matrix pending |
| Create/reopen a host-local Service Instance and accept the exact Authority response | [Endpoint/Service](../technical/endpoint-service-runtime.md) | `cmd/ardents/service_instance.go` -> `internal/service/instance`, `internal/service/publication` | Command, durable transition, binding and restart source trace below; installed custody handoff pending |
| Start each State-selected Node duty and drain its accepted work | [Network/Route/Node](../technical/network-route-node.md) | `cmd/ardents-node/node_mode.go` -> `internal/node`, `internal/route`, `internal/route/replay` | Entry located; duty-by-duty resource/close trace pending |
| Serve closed admission token acquisition and receiving spend | [Private admission](../technical/private-admission.md) | `cmd/ardents-node/issuer_initialize.go`, `internal/node/closed_issuer_listener.go` -> `internal/route/credential`, `internal/route/replay` | Issuer lifetime and permission-to-spend traces below; full combined failure/late cleanup matrix pending |
| Start the confined Publisher/Reader Endpoint and admit its local capabilities | [Endpoint/Service](../technical/endpoint-service-runtime.md), [application confinement](../technical/application-confinement.md) | `cmd/ardents/endpoint_headless.go`, `cmd/ardents/endpoint_text_linux.go` -> `internal/endpoint`, `internal/application/broker` | Installed command path traced below; its launch does not compose the enrollment/Release route; active Endpoint edits require reconciliation |
| Publish an immutable text snapshot only after worker and Introduction readiness | [Protected workload](../product/protected-service-workload.md), [Endpoint/Service](../technical/endpoint-service-runtime.md) | `cmd/ardents-text/main.go`, `internal/endpoint/text_descriptor_publication.go`, `internal/endpoint/text_introduction_registration.go` -> `internal/application/textdocument`, `internal/service/publication`, `internal/service/reachability` | Entries located; readiness/commit/withdraw trace pending |
| Interpret a Reader's explicit Target Link and resolve one private Descriptor | [Protected workload](../product/protected-service-workload.md), [private reachability](../technical/private-reachability.md) | `internal/endpoint/text_resolution.go` -> `internal/service/targetlink`, `internal/service/reachability`, `internal/route/terminal` | Entry located; exact refusal and freshness trace pending |
| Establish Introduction and JOIN over the selected Carrier | [Protected Route protocol](../technical/protected-route-protocol.md), [Network/Route/Node](../technical/network-route-node.md) | `internal/endpoint/text_introduction_exchange.go`, `internal/endpoint/text_join_service.go` -> `internal/route/closed_introduction_client.go`, `internal/route/closed_join_client.go` | Entries located; physical/logical owner and timeout trace pending |
| Authenticate the Service and exchange one bounded request/response with terminal outcome | [Protected workload](../product/protected-service-workload.md), [Endpoint/Service](../technical/endpoint-service-runtime.md) | `internal/endpoint/text_service_binding.go` -> `internal/service/connection`, `internal/application/interfacev2/connection`, `internal/application/textdocument` | Initial authentication, native stream, recovery, presentation and replacement-close trace below; full adversarial matrix pending |
| Withdraw publication, stop admission, cancel and join accepted work, then retain cleanup failure | [Endpoint/Service](../technical/endpoint-service-runtime.md), [Network/Route/Node](../technical/network-route-node.md) | `internal/endpoint/text_context_shutdown.go`, `internal/node/lifecycle.go`, `internal/service/connection/stream_lifecycle.go` | Entries located; cross-Module close graph pending |
| Replace/recover the installed Endpoint without bypassing Release or enrollment | [Release/update/custody](../technical/release-update-custody.md) | `cmd/ardents/endpoint_replace.go` -> `internal/endpoint/replacement` | Entry located; exact interrupted states pending |
| Project bounded diagnostics without authority or network effects | [Command surface](command-surface.md) | `cmd/ardents/offline.go` -> `internal/diagnostics/timeline` | Entry located; event producers and lost-error paths pending |
| Run installed qualification using the real Endpoint and Node paths | [Testing model](testing.md), [protected workload](../product/protected-service-workload.md) | `cmd/ardents-qualification/main.go`, `internal/qualification`, `tests/qualification` | Entries located; selected profiles and artifact identity pending |
| Refuse retired commands, plans, wire forms, and persisted identities without fallback | [Command surface](command-surface.md), affected current technical owner | `cmd/ardents/offline.go`, `cmd/ardents/endpoint_headless.go`, `cmd/ardents-node/node_config.go`; retained decoders in their owning packages | Entry examples located; exhaustive compatibility matrix pending |

For each row, the next pass records: normal and refused call path; authority and
input validation order; mutable/durable state; accepted child resources; stop,
join, and cleanup error propagation; direct and installed test evidence; and
the exact current document that should change with implementation. The
[C0 component reconstruction](c0-component-reconstruction.md) is a proposed
Module composition against these traces, not a replacement for them.

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
4. Portable `ready` proves the local profile and probe attachment. The
   `portable` package owns no State, Route, text worker, or Service Connection.
   `endpoint headless` is another command path: it parses the v2 runtime plan
   and invokes `runTextHeadlessRuntime`, with no call to enrollment, Release,
   replacement, or Portable in those files.
5. The installed `text-command-network` test creates an `ardents-endpoint`
   system unit whose `ExecStart` runs `endpoint headless` directly. It executes
   both Carriers and the real Custody/Node/text commands, but does not invoke
   `endpoint enroll` or `enroll-installed`. Its profile requires independently
   hashed candidate binaries and unit bytes, which is valuable fixture
   provenance; it is not the same first-execution enrollment/Release journey.
   The selected confinement contract requires this exact system-manager
   MainPID under the Endpoint service account before worker admission. The
   `endpoint user-unit` and `installed-user-unit` commands instead render
   per-user units for the generic enrollment/profile route; they cannot by
   themselves establish that system-manager worker-binding condition.

The two paths have separate readiness and lifecycle owners. The installed
text command evidence proves its bounded network/Application behavior, while
the enrollment process tests prove their own artifact and restart behavior.
Neither test, by itself, proves one joined path from authenticated first
execution through protected Service readiness. The target composition needs
an explicit trust transfer or one integrated launch owner; retain the
independent pre-execution verification and selected current-program restart
semantics. Qualification must run the combined path rather than infer it from
two separate green tests.

## Source trace: host Service Instance request and Authority acceptance

This trace follows `cmd/ardents/service_instance.go` and
`internal/service/instance/{root,lifecycle,state,storage,response}.go` at
`e48d4c3c`; the selected contract is in the Endpoint/Service technical owner.

1. `service-instance initialize` accepts a bounded canonical plan with an
   absolute existing owner root, Network ID, exact time window, and a separate
   public request destination. The command checks cancellation before effects.
   `instance.Initialize` validates root ownership/marker, acquires its
   exclusive process lock, and either reopens an exact matching generation or
   generates fresh Instance and Introduction keys. It atomically persists the
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

## Source trace: installed text Endpoint startup and shutdown

This trace was read from `cmd/ardents/endpoint_text_linux.go` and
`internal/endpoint/text_participant_linux.go` at `e48d4c3c`. It covers the
local process boundary, not the later publish/read network exchange.

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

The next trace must check whether accepted socket handlers, worker jobs,
publication refresh, and Route operations have joined before each parent's
`Close` returns. The defer order alone does not prove that invariant.

## Source trace: Publisher publication handover

This trace follows `cmd/ardents-text/publisher.go`,
`internal/endpoint/text_administration_linux.go`,
`text_publisher_start_linux.go`, and `text_descriptor_publication.go` at
`e48d4c3c`. It is a code-path trace, not installed qualification.

1. The trusted text command imports a bounded immutable snapshot from an
   explicit absolute file path, then sends only that snapshot through the
   separately authorized Administration socket with a 30-second deadline.
2. `textAdministration.PublishSnapshot` consumes a current Administration
   capability, checks snapshot length/UTF-8, and grants exactly one pending
   startup. Concurrent publication or close cannot accept a second run.
   `Withdraw` can cancel and join that pending startup before any late handover.
3. `startTextPublisher` launches and qualifies the installed worker before
   registration or Descriptor effects. The worker's publication start opens
   Source, Introduction, and Responder prefixes, registers the Introduction
   slot, and invokes `publishTextDescriptor`.
4. Descriptor publication requires the same live permission, Instance
   binding, current Source and registration. It issues or reuses exact signed
   private proof bytes, sends them to the selected resolution receiver,
   requires status 0, rechecks the owner and authority after the network wait,
   and only then commits the acknowledged publication and starts refresh.
5. A `textPublisherRun` receives the retained worker/publication lifetime. If
   handover was canceled or the Administration owner ended, the new run is
   closed and no successful local result is returned. Later withdrawal and
   shutdown join the run; `Published` does not claim a Reader has completed a
   Service exchange.

The next source pass must follow `textPublisherRun.Withdraw`, registration
withdrawal, the resolution Store's durable acknowledgement, and the tests for
cancel/acknowledgement races. The active Endpoint agent may change this trace;
revalidate it at the integration checkpoint.

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

The direct tests exercise real TCP authentication and idle Carrier drain
(`closed_forwarding_shutdown_test.go`), a delayed outgoing reader and retained
root lease across a Drain timeout (`closed_forwarding_reader_shutdown_test.go`),
and failed receiving-resource initialization with joined cleanup errors
(`closed_forwarding_receiving_test.go`). These test sources were inspected;
this study has not rerun them against the current tree.

This establishes the forwarding ownership order. The recipient matrix below
compares the other duty constructors and drain paths; tests for timeout,
concurrent producer/reader exit, and close-error retention remain necessary
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

## Node receiver resource matrix

The following constructor/close paths were inspected at `e48d4c3c` in the
five `closed_*_listener.go` owners and `closed_forwarding_shutdown.go`.
"Later close" means a server goroutine continues the close attempt after
`Drain` times out; it does not mean the caller receives its eventual result.

| Duty | Distinct durable or retained resources | Who closes them after accepted workers join | Later close after caller timeout |
| --- | --- | --- | --- |
| Forwarding | Replay receiving ledger, Hosting period, outgoing Carrier pool and session readers | `closedForwardingServer.finishShutdown` after producers, readers and pool join | Yes, from its own completion goroutine |
| Issuer | Credential issuer key root and replay spend ledger | `node.startClosedIssuer` adapter after `ClosedTokenListener.Drain` succeeds | No observed owner; listener joins later but holds neither root-close callback |
| Introduction | Replay spend ledger and its Introduction slot floor | `closedIntroductionServer.run` after accepted workers join | Yes, from its own run goroutine |
| Resolution | Replay spend ledger and Reachability store | `closedResolutionServer.run` after accepted workers join | Yes, from its own run goroutine |
| Data JOIN | Replay spend ledger, Hosting period, JOIN pairing owner and monitor | `closedDataJoinServer.run` after accepted workers join | Yes, from its own run goroutine |

All five roles use the Node lifecycle's common `probeServer` handle, although
only the private probe is a probe. This is a naming/interface problem in the
composition layer; it does not make their durable resources interchangeable.

## Source trace: Reader Target Link to safe text presentation

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
   terminal exchange, retires a recovery tail when present, joins the run
   goroutine and its physical cleanup, and returns the retained cleanup
   result. Initial Attachment cleanup has an Endpoint wrapper that caches its
   Route close error. Replacement Attachment cleanup currently crosses a
   `func()` native callback and drops the Route close result; see
   [F-23](repository-reconstruction-findings.md#f-23-replacement-service-attachment-loses-its-route-close-result).

Source-inspected tests cover Instance proof, lost terminal receipt, offset
rollback, bounded recovery, real TLS/document exchange, and recovery over
both Carriers. They do not cover a distinct replacement Route close failure
propagating to the final Endpoint outcome. This trace is not a fresh passing
installed qualification result, and the Endpoint agent's unfinished change
must be reconciled before treating these file boundaries as settled.

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
   one ordered tabular stream, and retains no raw input. It rejects corrupt
   recognized records without echoing their bytes; unknown schemas are
   ignored. The command reference gives a live `journalctl -f -o json` pipe.

`internal/diagnostics/timeline/project_test.go` checks cross-owner ordering,
redaction, malformed categories, and cancellation; the command adapter has a
stdin projection test. This is a functioning operator navigation path, not an
attempt-level Route trace or a complete retained event store.
