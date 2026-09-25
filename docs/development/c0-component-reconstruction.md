# C0 component reconstruction

Status: **architecture working model** for rebuilding the maintained code
around the selected journey. This is not a new product contract, package
registry, delivery ledger, or authorization to replace persisted formats.
Initial file inventory: `codex/architecture-refactor` at `50026274`
(2026-09-25). Package graph and selected source traces were refreshed at
`e48d4c3c` (2026-09-26). Active Endpoint edits in this worktree are not a
settled API.

## Starting from the required result

The selected [C0 readiness profile](../product/scope.md#c0-closed-alpha-readiness-profile)
and [protected text workload](../product/protected-service-workload.md)
require one installed Ubuntu Publisher to expose an immutable text document,
and a second Endpoint to open an explicit Target Link and read it. The candidate
uses both TCP/TLS and QUIC Carriers. Stop/restart must retain required durable
state and return an honest terminal outcome.

The journey has four phases:

1. Verify the artifact and enrollment, establish current Network State and
   separately held authority, permission, and Service Instance material.
2. Admit the Publisher, confine its worker, prepare its network prefixes,
   register Introduction, publish the signed Descriptor, and return a Target
   Link only after readiness.
3. Admit a Reader, validate its Target Link, resolve the current Descriptor,
   establish Introduction and JOIN, authenticate the Service, exchange one
   bounded text request/response, and verify the terminal result.
4. Revoke or withdraw new work, cancel and join accepted work in dependency
   order, close physical transports, and report cleanup failure when present.

Every component below is required by one of those phases or by an accepted
authority, safety, or persistence obligation. A component is an ownership
boundary; one component need not equal one Go package or process.

## Minimum component set

| Component | Owns and returns | Current implementation to assess |
| --- | --- | --- |
| Artifact and enrollment trust | Authenticate first-execution executable/static inputs, retain the current-program record across restart, and gate actual protected-Service readiness on that identity under the selected system-manager unit/account. | `internal/enrollment`, `internal/release`, `internal/endpoint/replacement`, `internal/endpoint/portable`, enrollment and headless adapters under `cmd/ardents`. The per-user enrollment and installed protected-text system-unit lanes are not yet composed (F-25/F-27). |
| Network State and time | Publish an authenticated current view and refuse stale, conflicting, or absent authority. Supply immutable projections; own the State root. | `internal/network/state`, `internal/network/source`, clock observation. |
| Service authority and Instance | Keep Authority signing material separate; issue the public Credential; own the host-local Instance key and generation. | `internal/custody`, `internal/service/instance`, `cmd/ardents-custody`. |
| Permission and token issuance | Issue bounded permission and tokens under separate authority; retain issuer roots and exact retry state. | `internal/route/credential`, Endpoint permission/transit owners, Node issuer duty. These are several trust owners, not one package. |
| Receiving spend | Verify admission at the selected recipient, spend once durably, enforce class and resource limits. | `internal/route/replay`, Route admission channels, Node role-specific spend roots. |
| Carrier | Open/listen on the State-selected TCP/TLS or QUIC profile and own physical connection retirement. Expose authenticated peer/transport evidence. | Carrier implementations and listeners currently inside `internal/route`. |
| Protected wire and channels | Encode/decode bounded frames and operations, multiplex lanes, enforce credit/deadlines, and terminate channels. | `internal/route/ardp`, `internal/route/terminal`, and channel owners in `internal/route`. |
| Client path | Select and retain the exact Source/Introduction/Responder handles from current State, present tokens, perform resolution and JOIN, and join its Route transport. | Endpoint's prefix owners together with client operations in `internal/route`. |
| Node duties | Start one State-authorized receiver, accept its work, retain listener/host/session/spend lifetimes, react to pressure, drain, and return terminal cleanup. | `internal/node` plus Route receiving operations. Issuer, forwarding, resolution, Introduction, and Data Join have distinct lifetimes. |
| Service publication and discovery | Own Instance publication, private Descriptor proof/revision, and exact Target Link interpretation. | `internal/service/publication`, `internal/service/reachability`, `internal/service/targetlink`, Endpoint publication coordination. |
| Service Connection | Authenticate the Service Instance and own one ordered logical byte stream, attachment replacement, continuity, and final authenticated terminal state. | `internal/service/connection`; Endpoint supplies authenticated attachments and local lifetime. |
| Endpoint and local admission | Admit Client/Publisher capabilities, bind the current State, permission, publication, route, worker and Application Interface to one local lifetime, and revoke/join children. | `internal/endpoint`, `internal/application/broker`. |
| Confined Application and local interface | Verify worker confinement before network effects; exchange one bounded text request/response through separately authorized local Connection and Administration surfaces. | `internal/application/interfacev2/connection`, `interfacev1/administration`, `textdocument`, Endpoint installed-worker code, `cmd/ardents-text`. |
| Resource and diagnostic evidence | Measure actual limits, choose protect/drain, emit bounded safe causes and retain cleanup failures. Each owner supplies its own event facts. | `internal/resource`, Node/Endpoint event owners, `internal/diagnostics/timeline`. |

Qualification tools, fixtures, and architecture gates verify this system; they
are not production data-path owners. Compatibility refusals for retired plans
and retained persisted floors are obligations at existing command/storage
boundaries, not second accepting runtime implementations.

## Necessary runtime versus retained surroundings

The component table is the **selected C0 runtime composition**, not a demand
to import every currently maintained package into one process. The package
graph contains 57 `cmd`/`internal` packages. Four alpha-bundle commands can
reach 50 packages by static imports; that count includes non-C0 command
branches and shared packages, so it is an upper bound on compile reachability,
not a count of required C0 Modules. `ardents-text` is a separate installed
Application process; the two qualification commands are verification tools.

| Surrounding code | Target relationship to the C0 composition | Current decision limit |
| --- | --- | --- |
| Retained operator and compatibility work: Portable user-unit enrollment/replacement, Invite Entry, Contributor retirement, local Name encoding, historical floors and refusals | Keep exact command/storage owners and acceptance evidence, but do not make them alternate C0 paths or pull their authority into the text Service. The selected protected worker boundary requires a system-managed Endpoint account and root-owned units (F-27). | Remove only after an explicit retirement or migration decision for each persisted or operator obligation. |
| Installed qualification commands, stream-test Application, architecture gates, and test fixtures | Exercise the real C0 path and report evidence; keep their plans/verdicts outside product runtime ownership. | A test-only callback is not proof of installed behavior. |
| Canonical Namespace and private Resolution without a selected production composition | Keep isolated from C0 runtime imports; trace exact retained evidence and the ADR-0090 decision before choosing future integration or retirement. | ADR-0090 explicitly retains Resolution pending an exact-consumer decision. |
| Historical Alpha private OHTTP resolver | Candidate to retire from maintained source after the ADR-0088 floor/parser/refusal obligations are shown independent of it. | Its seven production files have no production importer; the package map still states historical retention and needs reconciliation. |
| Question-scoped R-149/R-152 experiments and research drafts | Evidence and open design, not C0 runtime foundations. | Existing research links must remain usable if a spike is retired. |

This separation makes removal measurable: a source file leaves the maintained
tree only after its live consumer, persisted/wire identity, refusal oracle,
test evidence, and documentation owner are accounted for. It also prevents a
large uncomposed research package from influencing the necessary C0 package
graph merely because it exists in the same repository.

## Current reduction decisions and limits

| Material | Source-backed disposition for this study |
| --- | --- |
| Five duplicate Network State fixture files in `tests/e2e/node` | Consolidate their two caller sites onto `tests/epochfixture/network`; the 356 copied lines differ only in package declarations. Keep the distinct older network-source and State unit-test oracles. |
| Seven production and two test files in `internal/naming/alpha/private` | Source trace shows the historical OHTTP Client/Relay/Gateway has no production caller and owns neither the ADR-0088 floor reader nor the supplied-bytes inspection or exact refusal. Retire this package, its package-map row, and its 22 deadcode allowances as one bounded change; keep `internal/naming/alpha` and external refusal/inspection evidence. |
| Fifteen production files in `internal/naming/resolution` | Not a C0 runtime component, but ADR-0090 explicitly retains this Module pending an exact-consumer decision. Do not infer deletion from command unreachability. |
| Sixteen production files in `internal/service/instance` | Retain as one deep host-key/root Module. Initialization, exact response acceptance, durable phase, non-exporting binding and restart refusal share one authority invariant; the traced command has a real non-test call path. |
| Uncomposed OHTTP facade in `internal/service/reachability` | Candidate to retire separately from the shared Store and old-format decoder. The installed v3 caller uses private publication/lookup, while 35 Reachability symbols are deadcode-allowlisted. Keep persisted generation/conflict floors and legacy record reads until their migration obligation is resolved. |
| Portable profile scaffold (`ConfigHome/grants`, `StateHome/vault`, `StateHome/diagnostics`, `CacheHome`, test-only `portable.Run`) | No non-test Go consumer uses these created directories or the `Run` facade. Review external operator assumptions, then contract the profile around the actual first-enrollment and current-program owners without deleting existing state (F-26). |
| Offline Control/Custody imports of `route/credential` | Preserve the public issuer-profile verification and signed permission grammar, but separate their offline contract from the OHTTP Transit client and Route-facing issuer listener. The present package import brings CIRCL, OHTTP and QUIC into both command closures even though their exact callers perform offline work (F-28). Recheck the closures after a real package boundary change; keep Node/Endpoint network dependencies. |
| Proposed `network-core-transition.md` and wire appendix | Keep as unaccepted design while decisions are open; promote selected facts to current owners and retire superseded execution chronology after reconciling issue state and links. Do not implement candidate wire grammar by document proximity. |

This ledger intentionally mixes removals with explicit retentions. It avoids
using raw file count as a proxy for necessary work and will expand only when a
caller/resource/contract trace supports a disposition.

## Transition dependencies visible so far

This is a dependency order for review, not a second issue ledger or a new C0
implementation slice. Each step must be compared with the active network and
Endpoint owners before execution; accepted ADRs, current contracts and the
single C0 work-in-progress rule continue to control scope.

| Order | Bounded outcome | Evidence required before the next step |
| --- | --- | --- |
| 1. Reconcile the source baseline | Freeze one reviewed architecture-worktree revision, classify the other agent's completed Endpoint and network changes by touched owner, and update this map only where behavior actually changed. | Exact HEAD/diff, no lost staged or untracked changes, current issue owner, and affected current technical owner. |
| 2. Repair observed outcome gaps | Keep State root/role close failures in the terminal result, stop emitting an accepted Source-wave event for `--resume`, and give Issuer roots a later close owner after bounded drain timeout. | Focused combined-error, resume/actual-wave, and delayed-child/late-close tests; unchanged authority and wire behavior. These are behavior-preserving corrections to existing contracts, not package moves. |
| 3. Remove verified duplication and retire only decided dead paths | Consolidate the five exact Node E2E fixture copies; resolve the historical Alpha private OHTTP package against ADR-0088, and the retained Resolution Module against ADR-0090. Promote current document facts before retiring old transition chronology. | Caller and compatibility/refusal evidence, package-map/deadcode changes where applicable, and unchanged black-box fixture verification. An open ADR decision blocks only its own deletion candidate. |
| 4. Deepen the real network owners | Use the [Node plan](node-architecture-refactoring.md) and observed Route/Credential/Terminal edges to make duty, Carrier, wire, spend, and logical Connection ownership navigable. Extract a package only with a real caller, small Interface, tests and no parent import cycle. | One coherent passing slice at a time, exact stop/join/error owner, targeted Linux/Windows build as affected, `make quick-check` at the required boundary. |
| 5. Integrate Endpoint and qualify the combined journey | Reconcile completed Endpoint refactoring with Node/Route; design the root-owned artifact/Release-to-system-unit handoff, keeping the exact service account/MainPID and worker-binding invariant (F-25/F-27); then exercise installed Ubuntu Publisher/Reader with both TCP/TLS and QUIC and final repository gates. | One installed path proves the trust transfer, restart, system-manager identity and actual Service readiness together; no test-only reachability, preserved enrollment/Custody/State roots, real Application terminal outcome, `make check`, and separately recorded installed evidence. |

Further source traces may reorder or split these steps. In particular, a
network bug already owned by the other active task stays there; this study
does not duplicate its implementation merely because it appears in a map.

## One read through the proposed boundaries

```text
Reader Application -> local Connection Interface -> Reader Endpoint
    -> current State + local permission + Target Link / Descriptor
    -> client path -> protected channels -> selected Carrier
    -> forwarding / Introduction / resolution / JOIN Node duties
    -> Publisher Endpoint -> authenticated Service Connection
    -> confined Publisher Application
```

The Publisher setup prepares its Instance, worker, prefixes and Introduction
registration, then commits its Descriptor and returns the Target Link. During a
read, Endpoint first admits the local operation; the client path obtains the
selected recipient and physical attachment; Service Connection authenticates
the final Instance and owns ordered Application bytes. A network JOIN result
alone is not a completed Service read.

## Import direction and the first exact seams

The observed Linux package graph has `endpoint -> route`, `node -> route`,
`route/credential -> route`, and `route/terminal -> service/reachability`.
Endpoint imports 24 first-party packages and Node imports ten. These are
compile-time edges, not evidence that the imported owner is needed on every
runtime path. The target should first assign each concrete decision and
resource to an owner, then let the top-level process composition call that
owner. It should not turn a 74-file Route root into a generic transport API.

| Current edge or mixed responsibility | Source-level reason | Target decision before moving files |
| --- | --- | --- |
| `route/terminal -> service/reachability` | Descriptor operation encoders use `MaximumPrivateDescriptorSize` from Reachability; the wire package otherwise does not verify a Service proof or operate a Reachability store. | Give the 15,000-byte constraint one explicit contract owner and keep the wire encoder's size refusal synchronized with the signed Descriptor format. A copied constant without a shared contract check is insufficient. |
| `route/credential -> route` | Five production files use parent Route Grant verification, Carrier/listener, bootstrap, admission-channel, and role types. The traced current path has Credential's issuer root/reservation ledger, Endpoint's volatile holder/stock plus durable attempt journal, and Replay's receiving spend ledger under different owners. | Preserve those three authority/lifetime boundaries. Separate the Credential token/ledger core from its Route-facing listener and admission adapter only after assigning the accepted-child join and late issuer-root cleanup owner; a broad Carrier interface would preserve the dependency knot. |
| Offline Control/Custody `-> route/credential` | Control calls only the signed public issuer-profile decoder; Custody calls only permission/request grammar. The mixed package also contains OHTTP and imports Route, so static dependency closure includes QUIC in these offline commands. | Give the signed offline grammar a cohesive owner with a non-test caller and exact validation tests. Keep the networked issuance/Transit adapters separate without changing wire bytes or moving admission authority into Route. |
| `endpoint -> route` and `node -> route` | Both processes borrow transport/channel operations, while Endpoint also coordinates publication and Node owns receiving duties. | Keep process admission, lifecycle, and terminal error ownership at Endpoint/Node. The shared Route boundary should expose only the operations and leases each caller actually consumes. |
| `service/connection` has no first-party imports | It owns authenticated logical byte ordering and attachment lifetime without importing Endpoint or Route. | Preserve this deep boundary; adapt physical attachment at its caller, not inside Service Connection. |
| Replacement Service Attachment close | Native `Attachment` accepts a `func()` close callback; Endpoint's replacement callback discards the joined Route close result, while its initial transport separately caches that result. | Keep Service Connection independent of Route, but make the replacement close result observable by Endpoint after native join. A package move cannot fix the lost result; preserve exactly-once retirement and terminal ordering. |

The target import rule is therefore directional: command adapters compose
Endpoint, Node, State, Service, and trust owners; Endpoint/Node borrow the
smallest Route operations required for their exact duty; wire and Carrier
mechanics do not own Service publication, Network authority, or process
lifetime. This is a constraint for the source-level design, not a package
creation list. The Endpoint agent's accepted boundary and the concurrent
network changes must be read before finalizing exact exported Interfaces.

## State and close ownership

| Resource or decision | Owner at the boundary | Terminal obligation |
| --- | --- | --- |
| Current Network State | State runtime; Node and Endpoint borrow checked views. | Close the State root after borrowers stop; successor/loss refuses new effects. |
| Local grant and participant job | Endpoint/Broker and the exact participant context. | Revoke new admission, cancel all children, then join operations and worker. |
| Instance key and publication generation | Instance and Publication owners, borrowed by Endpoint. | Withdraw publication, join leases, erase/close the Instance binding in the required order. |
| Source/Introduction/Responder prefix | Exact client-path owner. | Retire admission and join child channels before releasing the parent transport. |
| Node listener and accepted sessions | The selected Node duty. | Stop acceptance, join accepted producers/readers, then release spend and host resources; retain cleanup error. |
| Physical Carrier | Carrier lease/connection owner. | Close once; report physical close failure to its borrowing duty or path. |
| Logical Service Connection | Service Connection owner with Endpoint's attachment opener. | Verify terminal outcome and join physical attachment cleanup before reporting completion. |
| Confined worker | Endpoint's installed-worker owner. | Stop and join its process tree/cgroup before releasing its job authority. |

These are desired single-owner relationships, checked against the current
[Node/Route contract](../technical/network-route-node.md) and
[Endpoint/Service contract](../technical/endpoint-service-runtime.md). The
[first per-file ownership pass](c0-component-inventory.csv) is recorded. The
shutdown graph and production call graph still need a separate source-level
pass before changing package boundaries.

## First source inventory

The inventory covers all **386 tracked, non-test Go files** under
`internal/node`, `internal/route`, `internal/endpoint`, and
`internal/service` at `50026274`: Node 45, Route 130, Endpoint 144, Service
67. It includes platform files and package documentation. It excludes commands,
other `internal` packages, tests, and uncommitted work from the active Endpoint
agent. No file is omitted or assigned twice. The CSV assigns a proposed primary
component to each file; a mixed file is explicitly marked for a boundary or
split review. This is an ownership hypothesis, not a deletion list or a
package-move instruction.

| Treatment | Files | Meaning |
| --- | ---: | --- |
| `retain` | 159 | The existing package or primitive appears to have a coherent owner; preserve its contract while composing the new shape. |
| `deepen` | 93 | Local ownership is visible but buried in a broad package; make the owner and lifetime explicit before deciding on a new package. |
| `move-candidate` | 52 | The file's current package mixes different component responsibilities; decide the destination with the caller and close graph first. |
| `boundary-review` | 71 | The file bridges components or its exact owner is unclear from the current API. |
| `split-candidate` | 4 | One file contains behavior for more than one proposed owner. |
| `compatibility-review` | 2 | A codec appears to have no maintained production caller; confirm the accepted compatibility evidence before disposition. |
| `retirement-review` | 5 | An older Reachability/OHTTP path has no external production caller found in this pass; verify dependencies and accepted obligations before removal. |

The strongest positive boundary is already in `service/connection`,
`service/instance`, `route/replay`, and Endpoint's small durable packages.
`route/credential` has a cohesive token and ledger core, but five production
files import the parent `route` package for a listener, admission channel,
bootstrap controller, role constants, or Grant verification. The largest
unresolved boundary is the Route root package:
its 74 production files mix Carrier, protected wire, client Source/JOIN,
receiver admission, and compatibility evidence. The Endpoint root has 107
production files with participant lifetime, publication, worker, and client
path mixed together; the active Endpoint refactor is changing that exact area,
so this inventory must be rebased against its accepted result.

Several source checks changed the classification:

- `reachability.OpenStore` is called by Node resolution, so Reachability's
  `store*.go` files are active storage, not old debris. `portable` and
  `replacement` are called by `cmd/ardents`; they remain in the artifact and
  enrollment path.
- `node/closed_forwarding_admission.go` holds both forwarding reservation and
  a role-token verifier used by control duties. `node/closed_hosting.go` holds
  the shared host period, control admission, and pressure response. Moving
  either whole file by its prefix would put shared authority under one duty.
- `route/closed_node_open.go` combines the Node OPEN grammar with an accessor
  on the bridge lane. `route/terminal/descriptor.go` imports
  `service/reachability` for the private Descriptor bound. These are concrete
  cases where a desired package move needs an API/dependency decision first.
- Admission is a process-spanning operation, not one storage owner:
  Custody's encrypted allocation journal, Credential's issuer reservation
  ledger, Endpoint's potential-presentation journal and Replay's receiving
  spend ledger have separate retry, restart and close rules. The installed
  text command test invokes real Custody permission issuance for both roles;
  these boundaries are current C0 behavior, not candidate file names.
- The current Go import graph has `node -> route`, `endpoint -> route`,
  `route/credential -> route`, and `route/terminal -> service/reachability ->
  service/publication`. The issuer listener in `route/credential` therefore
  cannot simply be pulled into a Route-root caller, and a new low-level wire
  package must not inherit all of Reachability's OHTTP and store dependencies
  just for its Descriptor size limit.
- The v1 `LegBinding` codec and older Reachability Gateway/Relay/Client path
  have no external production caller found in the targeted symbol pass. They
  are review candidates only; this pass did not establish that their accepted
  compatibility or test obligations can be retired.

The table is intentionally file-level, not a claim that every Go file needs a
different module. Before a move, inspect its non-test callers, imports,
mutable state, resource ownership, and terminal cleanup. Then choose one
interface and import direction for the component, updating `package-map.md`
only when the package is actually introduced or renamed.

## Where current packages obscure the model

1. `internal/route` currently combines Carrier opening/listening, ARDP and
   terminal grammar, admitted channels, client Source/JOIN operations, and
   receiver-side primitives. The word *Route* does not identify which of those
   owns physical transport, protected wire, or a client path.
2. `internal/node` composes a process and five receiver duties in one package.
   Forwarding also retains outgoing Carrier sessions and links; Route owns
   some of their physical and admitted channel behavior. Their stop/join
   relationship is real and must be explicit in the new shape.
3. `internal/endpoint` composes local capability admission, worker, token stock,
   publication, Route prefixes, authenticated Service attachment and Application
   exchange. Its context coordinates necessary cross-owner ordering, while
   private owner implementations remain difficult to navigate in one directory.
4. `internal/service` already has distinct Instance, Publication, Reachability,
   Target Link and Connection packages. The distinction between Service's
   logical Connection and Route/Carrier physical connections needs to appear
   in interfaces and naming at their handoff. The Route terminal Descriptor
   codec currently imports `service/reachability` for a size bound, a concrete
   dependency to resolve when placing wire grammar.
5. `closed_*` and `text_*` filenames encode the implementation campaign across
   many responsibilities. A package or owner name should supply that context
   after the actual boundary is moved; a bulk rename alone cannot establish it.

## Reconstruction method and next decisions

1. Use the [first source inventory](c0-component-inventory.csv) to audit the
   production callers and owned resources of every `move-candidate`,
   `boundary-review`, and `split-candidate` file. Split files with two unrelated
   owners by behavior before moving directories. Reconcile Endpoint entries
   after its current implementation slice is complete.
2. Draw the package dependency graph for the target components. The first
   decision is the boundary between Carrier, protected wire/channel, client
   path, and Node duty. Specify the minimum input/result and who closes every
   returned resource; then settle the Service Connection attachment handoff.
3. For each existing owner choose **retain**, **move/deepen**, or **replace**.
   Retain tested codecs, durable floors and security checks where their owner
   is already sound. Replace code only when a small boundary cannot be made
   coherent without exporting private state or duplicating authority checks.
4. Recompose one Publisher and one Reader journey on the chosen boundaries in
   the architecture worktree, one complete implementation slice at a time.
   Each new Go package gets implementation, `doc.go`, behavior tests, a
   production caller and package-map entry together. Existing accepted wire,
   persisted, authority and cleanup contracts remain the acceptance tests.
5. Integrate completed network fixes at bounded checkpoints. Verify focused
   owners during each slice; run the full repository gate and installed Ubuntu
   TCP/TLS and QUIC journey on the combined result before integration.

The existing [Node refactoring plan](node-architecture-refactoring.md) and
[Endpoint ownership map](endpoint-architecture-refactoring.md) are inputs to
steps 1-2. Their proposed packages are not fixed until the cross-system
dependency and close-ownership map is complete.
