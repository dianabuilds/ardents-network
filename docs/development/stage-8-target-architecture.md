# Stage 8 target architecture

Status: **S8.3 accepted by the Product Owner on 2026-08-22.** This is the target
ownership and migration-design authority derived from the accepted S8.1
[preservation ledger](stage-8-preservation-ledger.md), the S8.1
[decision-authority register](stage-8-decision-authority-register.md), and
the source-bound S8.0 inventories. It is not a current package map, a promise
that every target Module will be implemented, or authority to create a package
before its wave has a real caller, behavior, tests, and package-map row.

## Design constraints

- Preserve the Product Core and its honest H3 limitations exactly as S8.1
  records them. Product behavior, authority, persistence, wire, configuration,
  command, IPC, and evidence compatibility change only through a wave-owned
  observer and format decision.
- One Module owns each state transition, physical writer, process lifetime, and
  terminal cleanup. A caller may own an input port but never a second writer or
  lifecycle authority.
- `cmd` packages remain thin composition/adaptation roots. `endpoint` is the
  primary local Endpoint composition root; `node` is the separately runnable
  Contributor composition owner. Neither gains authority over foreign durable
  roots.
- A target interface appears only with real behavior. The factual current
  `package-map.md` remains the import authority until its owning migration wave
  cuts callers to a new Module.
- An unresolved decision-authority item blocks only the dependent Module or
  format wave; it does not authorize a speculative abstraction or a weaker
  fallback.

## Target ownership map

| Target Module | Owns | Migration source and boundary | Entry condition |
|---|---|---|---|
| `internal/endpoint` | Local process composition, readiness, signal/drain order, terminal cleanup, and one terminal result. | Replace command/`serviceendpoint` choreography; owns no domain state. | M10 selects the Broker/Isolation process boundary. |
| `internal/application/broker` | Volatile Application/Admin Principal, Grant, one-use capability, revocation, and drain tree; its only current isolation result is explicit `generic/unqualified`. | Replaces split `applicationipc`/`serviceconn` admission logic. | R-085 generic/unqualified profile; qualified principal adapters remain future work. |
| Future qualified platform adapter | A selected platform Adapter may supply a qualified isolation observation; no package exists merely to name that future claim. | Replaces the generic Broker observation only through a new qualified profile decision. | Platform threat evidence and ADR remain required. |
| `service/connection` | One live authenticated byte stream, replay/cutover state, bounded buffers, and terminal outcome. | Deepen `serviceconn`; remove operation/evidence unions and static plan authority. | R-076/ADR-0024 bind it to `ardents-interactive-route-v1`; it owns recovery, not Route selection. |
| `service/publication` | Instance generation, private material, admissions, unpublish/drain/erase, and crash-atomic publication. | Extract from connection/endpoint choreography. | Publication observer and format rule from DA-10. |
| `naming/namespace` | Authority, Lease, Claim, Recovery, admission, durable generation, and bounded materialization. | Consolidate six Namespace-state owners beneath the cohesive canonical `naming` vocabulary package as nested modules with explicit one-way imports; no shared generic store is presumed. | DA-03, DA-04, DA-05, and DA-07 as applicable. |
| `naming/resolution` | Private resolution/control exchange, gateway binding, replay state, and observer-safe counters. | Deepen `nameresolution` over opaque Namespace/State views; no plaintext fallback. | DA-03, DA-04, and DA-07. |
| `network/state` | Authenticated current/pending View, time floor, source distribution, durable publication, and source-server lifetime. | Absorb epoch/framing/store orchestration and remove concrete Source reversal. | DA-02 and DA-05. |
| `network/source` | State-owned acquisition port and bounded transport observations, never accepted state. | Retain one direct-origin Adapter only while selected. | Source protocol/compatibility decision under DA-05/DA-10. |
| `network/duty` | Durable local role-domain generations, watermark, expiry, and conflicts. | Replace `localroles`; State/Node/Route/Entry consume opaque duty facts. | D02 cutover and restart rule. |
| `resource` | Linux cgroup-v2/rlimit process profile, measurements, reservations, hysteresis, pressure state, and finite releases; explicit unsupported-platform refusal elsewhere. | Retain/deepen current resource owner; native metrics are concrete platform Adapters. | R-062 H1: Linux only until a native Adapter has measured acceptance. |
| `entry` | Durable Invite/replay/contact history, replacement, and finite acquisition attempts. | Replace `bridge` state/callback ownership. | R-076/ADR-0024 select adjacent TCP/TLS; R-077/ADR-0025 select its State-referenced Invite. Entry never selects a complete Route. |
| `route` | One Route selection and volatile role/attachment lifetime, capacity, cutover, and cleanup. | Absorb `routeplan`; consumes opaque View/Duty/Resource/Entry facts. | R-076/ADR-0024 selects the native Profile. |
| `node` | One Contributor duty admission, quarantine, listener/probe, protect/drain/withdraw, and joined cleanup. | Fold `node/probe`; no State-root or assignment authority. | D02 and selected Resource/platform contract. |
| `release` | Verified metadata result, roots/floors/archive, lease, and opaque update authorization. | Own the release verifier and keep floor persistence private. | DA-01 before D06 mutation. |
| `update` | Staging, predecessor/rollback, technical-tracer activation/self-test, journal, recovery, and cleanup. | Deepen `updatetransaction`; consumes unforgeable Release authorization and owns no Custody state. | R-064 limits M2 to one offline H3 tracer; a supported lifecycle reopens DA-09. |
| `internal/custody` | Vault/Recovery Bundle, unlock/export/restore/reconcile, revocation, and signing watermark. | New Module; secrets never enter Release/Update/diagnostics. The M12 slice owns canonical envelope admission, independent encrypted Vault-record create/verify, distinct-password Bundle export/test restore to a new destination, encrypted authority-locked quarantine restore into an empty Vault, an exact local Authority floor that advances only after encrypted record publication, and one sealed Namespace transition signature from an active Name record; it returns no root material. | ADR-0021; DA-08 and DA-09 remain required for platform and full lifecycle qualification. |

## Intended dependency and trust direction

The following arrows describe allowed target knowledge, not packages to create
early. A Module may depend only on the smallest consumer-owned port needed for
the stated responsibility.

```text
cmd/ardents -> internal/endpoint
cmd/ardents-custody -> internal/custody
cmd/ardents-node -> node, internal/endpoint composition inputs
internal/endpoint -> internal/application/broker, service/publication, service/connection, route
service/connection -> route
route -> entry, network/duty, resource, network/state views
entry -> selected adjacent TCP/TLS carrier
node -> network/duty, resource, network/state views, route views
network/state -> network/source (caller-owned acquisition port)
naming/resolution -> naming/namespace views, network/state views
release -> update authorization consumer
custody -> future Application/Broker isolation ports; never release or update state
```

Forbidden target direction includes product Modules to `internal/lab`, test
harnesses, `experiments`, or `scripts`; an Entry Carrier to Route policy;
`network/source` to accepted State; `service/connection` to Namespace; and
Custody to Endpoint/Release/Update state. Platform and external dependencies
remain concrete Adapters at their consumer boundary.

## Commands and evidence

The retained end state has thin `ardents` and `ardents-node` commands. A third
bootstrap/update command exists only if DA-09 accepts a supported lifecycle;
R-064 does not do so.
Current name, bridge, route, service, release, publish-app, stream-app, and
laboratory commands are not compatibility promises. M13 classifies actual
automation observers before removing or replacing them.

Laboratory packages, historical verifier commands, and their Docker/tool inputs
never enter the product graph. M14 retains only a named historical-reproduction
or accepted claim-Qualification obligation with an owner, source identity, and
retirement condition; all other runners and fixtures are deleted.

## Format and compatibility authority

| Surface | Target treatment | Required decision before writer/reader mutation |
|---|---|---|
| A01-A03 local Application/Admin bytes and terminal result | Preserve semantic authority separation and one classified terminal result; replace socket/frame syntax only with an observer rule. | Endpoint/Broker/Publication design and DA-10 caller inventory. |
| A04-A07 commands, plans, machine results, and stream modes | Retain only real operator/Application observers; plans never become authority and direct stream modes leave shipped product. | M13 command/configuration inventory and DA-10. |
| D01 Network State and D02 duty roots | One writer, atomic cutover/restart semantics, and no reset or dual authority. | DA-02/D02 and DA-05. |
| D03 Entry and D04 naming roots | Migrate only with replay, monotonicity, recovery, tamper, and proof rules preserved. | DA-03 through DA-07 as applicable. |
| D05 publication/Instance hand-off | One generation/publisher owner with explicit drain/cutover. | Publication observer inventory and DA-10. |
| D06 Release floors | Security-forward-only: never decrease trusted roots or floors. | DA-01. |
| D07 update transaction and D08 custody envelope | D07 may retain only the R-064 bounded offline technical tracer; no supported activation/recovery mutation. D08 has no mutation until a custody lifecycle is selected. | R-064 for M2; DA-09 (and DA-08 for platform custody) reopens for any product lifecycle. |
| W01-W04 peer-visible, cryptographic, Route, naming, and WebTunnel bytes | Retain semantic contract only; choose `read/migrate`, `break`, or `delete` per observer. | DA-05 through DA-07 and DA-10. |
| Q01-Q03 evidence/test/document records | Keep provenance separately from current product truth; only accepted claim evidence becomes Qualification. | S8.2 profile policy and M14 retirement ledger. |

No target wave may add a forwarding writer, indefinite legacy decoder, or
unbounded compatibility mode. The S8.4 plan records a concrete mode, observer,
cutover, rollback/forward-repair behavior, and deletion condition for every row
it mutates.

The source-controlled limits of the DA-10 caller search are recorded in the
[compatibility-observer inventory](stage-8-compatibility-observer-inventory.md).
Its absence-of-evidence result is not a license to break an unrecorded external
consumer.

## Complete current-code disposition

The following groups cover every current Go package under `cmd/`, `internal/`,
`tests/e2e/`, and `tests/live/`. A grouping is only a shared ownership outcome;
S8.4 still records per-wave paths and deletions. Empty directory placeholders
are not Go packages and do not represent a retained test surface.

| Current source | Target disposition | Wave and condition |
|---|---|---|
| `cmd/ardents` | Replace with thin Endpoint command. | M10/M13 after Endpoint composition exists. |
| `cmd/ardents-custody` | Retain the separate custody-process adapter. It performs public envelope inspection and active-record verification through a no-echo interactive terminal secret boundary, returning only bounded public verification facts. Bundle, restore, reconciliation, and signing operation routes remain admitted only with their complete M12 custody lifecycle. | M12/M13 under ADR-0021 and DA-08/DA-09. |
| `cmd/ardents-node` | Replace with thin Node/Endpoint composition command. | M11/M13. |
| `cmd/ardents-name` | Remove current command shape; retain only selected naming/resolution operator journey. | M5/M6/M13, subject to DA-03/04/07/10. |
| `cmd/ardents-bridge`, `cmd/ardents-route` | Remove current tracer shapes; retain selected Entry/Route operator journey only. | M7/M8/M13, subject to DA-06/10. |
| `cmd/ardents-service`, `cmd/ardents-publish-app`, `cmd/ardents-stream-app` | Remove tracer commands; retain a real Endpoint/Application operator surface only if DA-10 names its observer. | M9/M10/M13. |
| `cmd/ardents-release` | Retire as an H3 product command. R-087/ADR-0029 allow it only until the owned V1→V2 Update-root conversion and C4 V0 verifier are complete; no C2 operator observer remains. | M1/M2/M13, subject to DA-01/R-064/R-087 and DA-10. |
| `tests/e2e/service/fixturecommand/publish-app`, `tests/e2e/service/fixturecommand/stream-app` | Retain only as explicit non-shipped process-profile fixtures for the separately granted publication socket and opaque Application stream. They are built by the Endpoint recovery process test, never installed or promoted as operator UI. | M13 C0 completed; delete with the named e2e evidence when a replacement test owns those boundaries. |
| `cmd/blocked-entry-verify-lab`, `cmd/carrier-lab`, `cmd/named-site-lab`, `cmd/stage6-evidence-lab`, `cmd/stage6-verify-lab` | Historical reproduction with a named retained obligation, or delete the runner while retaining immutable provenance. | M14, subject to DA-11; R-080 already retires the Stage-5 evidence generator. |
| `internal/endpoint` | Retain the M10 target owner for Application/admin process composition, raw opaque Application bytes, exactly one classified terminal result, readiness, and cleanup. The former `internal/applicationipc` and `internal/serviceendpoint` paths are deleted. | M10 under R-085; old raw-tail and timing-selected result delivery are C0 retired in favour of Endpoint's one explicit v1 local contract. |
| `internal/custody` | Retain the M12 owner for canonical envelope admission, encrypted Vault records, explicit bounded secret use, and public header inspection. It neither releases root material nor allows Release/Update/Endpoint to mutate custody state. | M12 has Bundle export/test restore, quarantine/reconciliation, custody-derived Name control signing, and R-044 effective Authority replacement. Its old V0 `custody_notice` is an explicitly bounded C2 tracer field. R-086 must decide local Vault demotion after a replacement; platform qualification remains open. |
| `internal/service/publication` | Retain the M9 target owner for one exclusive C1 Instance publication generation, floor, volatile signer, and drain lifecycle. It has no local admission, IPC, connection/recovery, or legacy H3 reader authority. | M9 under R-084; Endpoint is its direct role-local composition caller. |
| `internal/service/connection` | Retain the M9 target owner for closed ADR-0028 endpoint records, immutable context, logical stream/recovery lifecycle, and native terminal outcome. It accepts only opaque already-authenticated Attachments; no H3 record reader may be added. | M9 under R-083/ADR-0028. |
| `internal/bridge` | Deleted after transferring the required durable Invite/replay/replacement responsibility to `entry`. | M7 complete under R-076/R-080. |
| `internal/entry` | Retain the M7 target: signed State-referenced Invite v1, bounded durable Entry/replay set, adjacent candidate lookup, and candidate-opener lifecycle. | M7 under R-076/R-077/R-079. |
| `internal/camouflage` | Deleted: R-076/ADR-0024 retire the H3 WebTunnel adapter from the maintained Profile. | M7 complete under R-080. |
| `internal/localroles` | Transfer durable duty state to `network/duty` without generation reset. | M4. |
| `internal/network/duty` | Own the retained durable Endpoint-local Role Domain duty generations, watermark, expiry, and conflict truth. | M4 D02 C1 cutover; preserve the existing root format and one writer. |
| `internal/naming` | Retain the cohesive canonical Service Name V1 parser and encoder as the parent Namespace vocabulary package. | M5 retains it with its exact R-041 responsibility; no generic naming utility surface. |
| `internal/naming/namespace` | Compose Namespace admission, Authority, Record/Lease, Claim, Recovery, and Epoch modules under one canonical vocabulary root; retain only opaque views and C4 compatibility at the root. | M5, subject to DA-03/04/05/07; delete each former source package as its ownership moves into its nested Namespace module. |
| `internal/naming/namespace/admission` | Own bounded anonymous-work challenge/proof, replay, expiry, capacity, and in-flight refusal facts. | M5 under the root composition; no lower Namespace package imports Authority or Epoch. |
| `internal/naming/namespace/record` | Own canonical Record/Lease lifecycle, signatures, lineage, and destination binding as one state machine. | M5 under the root composition; Lease is not a separate package. |
| `internal/naming/namespace/claim` | Own root-claim commitment/reveal and authenticated winning-claim materialization. | M5; may consume Admission and Record, never Authority or Epoch. |
| `internal/naming/namespace/recovery` | Own Recovery Policy, quorum proof verification, and sealed authorization facts. | M5; may consume only canonical vocabulary. |
| `internal/naming/namespace/epoch` | Own durable current/pending Namespace materialization, its exact-successor pending journal/cursor, attestation, and proof verification. | M5; consumes Record and Claim, never Authority. |
| `internal/naming/namespace/authority` | Own canonical private control submission and authorized transition orchestration; writes only validated successors through Epoch's pending port. | M5; the sole upper Namespace orchestrator over Admission, Record, Claim, Recovery, and Epoch. |
| `internal/naming/resolution` | Own private resolution/control over opaque Namespace/State views. | M6, subject to DA-03/04/07. |
| `internal/network/epoch`, `internal/network/epoch/assignment`, `internal/network/epoch/merkle`, `internal/network/framing`, `internal/network/store`, `internal/network/state` | Consolidate authenticated acceptance, current/pending state, and durable publication under `network/state`. | M3, subject to DA-02/05. |
| `internal/network/source` | Retain as State-owned acquisition port and selected direct-origin Adapter only. | M3, subject to DA-05/10. |
| `internal/node`, `internal/node/probe` | Transfer Node lifecycle and private probe to `node`. | M11. |
| `internal/resource` | Retain/deepen as the sole shared resource coordinator; Linux profiles only and fail closed elsewhere. | M4, R-062 H1 accepted. |
| `internal/route`, `internal/routeplan` | Consolidate route selection, attachment lifetime, process cleanup, and the R-078 closed v1 wire under `route`; no legacy reader survives cutover. | M8, subject to DA-06. |
| `internal/release` | Own release trust/root/floor verification behind `Open`, `Evaluate`, and `Close`. | M1, subject to DA-01. |
| `internal/update` | Own the bounded offline transaction/recovery tracer; do not add a supported activator, installer, or Custody writer. Its C1 V1→V2 root conversion removes the V0 EvidenceNotice from runtime while preserving only C4 vectors. | M2/M13, subject to DA-01/R-064/R-087. |
| `internal/planfile` | Replace with command/owner-local bounded input decoders; do not retain a generic plan abstraction. | M3/M8/M9/M11/M13 as each consumer moves. |
| `internal/streamworkload` | Retain only a named test/Qualification workload; otherwise delete. | M9/M14. |
| `internal/architecture` | Retain factual graph/policy gate; remove historical receipts as their truth moves to current owners. | M0/M14. |
| `internal/lab/blockedverify`, `internal/lab/carrier`, `internal/lab/directcontrol`, `internal/lab/modulecache`, `internal/lab/namedsite`, `internal/lab/nativecircuit`, `internal/lab/preflight`, `internal/lab/routecomparison`, `internal/lab/runlayout`, `internal/lab/sourceidentity`, `internal/lab/stage6evidence`, `internal/lab/stage6verify`, `internal/lab/tooling` | Never promote into product runtime. Retain only a named reproducer/verifier with source identity and retirement condition, otherwise delete code/assets at closure. R-080 already removed the Stage-5 evidence generator. | M14, subject to DA-11. |
| `tests/e2e/network-source`, `tests/e2e/node`, `tests/e2e/service` | Replace tests through the target Module/process seam, retaining only independently observable process facts. | M3/M9/M11. |
| native live Route suite | Register only after M8/M11 select the peer-facing Route and measured Node operating profile; it is not current Qualification. | R-081, M8-M11/M14, subject to DA-06/08 and claim acceptance. |

## Acceptance and stop rules

This accepted map is the S8.3 design authority. DA-01 through DA-10 remain
explicit stop conditions for their dependent waves. S8.4 may instantiate the
M0-M14 migration and retirement ledger; code moves begin only in an accepted
S8.5 wave.
