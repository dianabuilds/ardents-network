# Stage 8 target architecture

Status: **S8.3 proposal for Product Owner acceptance.** This is the target
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
| `endpoint` | Local process composition, readiness, signal/drain order, terminal cleanup, and one terminal result. | Replace command/`serviceendpoint` choreography; owns no domain state. | M10 selects the Broker/Isolation process boundary. |
| `application/broker` | Volatile Application/Admin/Custody Principal, Grant, session, revocation, and drain tree. | Replaces split `applicationipc`/`serviceconn` admission logic. | DA-08 platform/process profile. |
| `application/isolation` | One admitted native sandbox/process lifetime and its terminal observation; no product truth. | New concrete platform Adapter, never a boolean capability bag. | DA-08 and platform evidence. |
| `service/connection` | One live authenticated byte stream, replay/cutover state, bounded buffers, and terminal outcome. | Deepen `serviceconn`; remove operation/evidence unions and static plan authority. | DA-06 decides retained Route protocol representation. |
| `service/publication` | Instance generation, private material, admissions, unpublish/drain/erase, and crash-atomic publication. | Extract from connection/endpoint choreography. | Publication observer and format rule from DA-10. |
| `naming/namespace` | Authority, Lease, Claim, Recovery, admission, durable generation, and bounded materialization. | Consolidate seven naming packages; no shared generic store is presumed. | DA-03, DA-04, DA-05, and DA-07 as applicable. |
| `naming/resolution` | Private resolution/control exchange, gateway binding, replay state, and observer-safe counters. | Deepen `nameresolution` over opaque Namespace/State views; no plaintext fallback. | DA-03, DA-04, and DA-07. |
| `network/state` | Authenticated current/pending View, time floor, source distribution, durable publication, and source-server lifetime. | Absorb epoch/framing/store orchestration and remove concrete Source reversal. | DA-02 and DA-05. |
| `network/source` | State-owned acquisition port and bounded transport observations, never accepted state. | Retain one direct-origin Adapter only while selected. | Source protocol/compatibility decision under DA-05/DA-10. |
| `network/duty` | Durable local role-domain generations, watermark, expiry, and conflicts. | Replace `localroles`; State/Node/Route/Entry consume opaque duty facts. | D02 cutover and restart rule. |
| `resource` | Process profile, measurements, reservations, hysteresis, pressure state, and finite releases. | Retain/deepen current resource owner; native metrics are concrete platform Adapters. | Accepted resource envelope and affected-platform scope. |
| `entry` | Durable Invite/replay/contact history, replacement, and finite acquisition attempts. | Replace `bridge` state/callback ownership. | DA-06 for retained Carrier/Route mechanism. |
| `route` | One Route selection and volatile role/attachment lifetime, capacity, cutover, and cleanup. | Absorb `routeplan`; consumes opaque View/Duty/Resource/Entry facts. | DA-06. |
| `route/webtunnel` | Pinned candidate child, local front connection, temporary root, join, and cleanup. | Conditional concrete Route/Entry Adapter replacing `camouflage`. | DA-06; otherwise delete rather than rename. |
| `node` | One Contributor duty admission, quarantine, listener/probe, protect/drain/withdraw, and joined cleanup. | Fold `node/probe`; no State-root or assignment authority. | D02 and selected Resource/platform contract. |
| `release` | Verified metadata result, roots/floors/archive, lease, and opaque update authorization. | Deepen `releasedecision`; hide staging/store exports. | DA-01 before D06 mutation. |
| `update` | Staging, predecessor/rollback, activation, self-test, journal, recovery, and cleanup. | Deepen `updatetransaction`; consumes unforgeable Release authorization. | DA-01 and DA-09 before D07 mutation. |
| `custody` | Vault/Recovery Bundle, unlock/export/restore/reconcile, revocation, and signing watermark. | New Module; secrets never enter Release/Update/diagnostics. | DA-08 and DA-09; no implementation by implication. |

## Intended dependency and trust direction

The following arrows describe allowed target knowledge, not packages to create
early. A Module may depend only on the smallest consumer-owned port needed for
the stated responsibility.

```text
cmd/ardents -> endpoint
cmd/ardents-node -> node, endpoint composition inputs
endpoint -> application/broker, service/publication, service/connection, route
application/broker -> application/isolation (platform adapter only)
service/connection -> route
route -> entry, network/duty, resource, network/state views
entry -> route/webtunnel (only if DA-06 retains it)
node -> network/duty, resource, network/state views, route views
network/state -> network/source (caller-owned acquisition port)
naming/resolution -> naming/namespace views, network/state views
release -> update authorization consumer
custody -> application/broker/isolation ports; never release or update state
```

Forbidden target direction includes product Modules to `internal/lab`, test
harnesses, `experiments`, or `scripts`; `route/webtunnel` to Route policy;
`network/source` to accepted State; `service/connection` to Namespace; and
Custody to Endpoint/Release/Update state. Platform and external dependencies
remain concrete Adapters at their consumer boundary.

## Commands and evidence

The retained end state has thin `ardents` and `ardents-node` commands. A third
bootstrap/update command exists only if DA-09 accepts a supported lifecycle.
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
| D07 update transaction and D08 custody envelope | No mutation until real activation/recovery or custody lifecycle is selected. | DA-09 (and DA-08 for platform custody). |
| W01-W04 peer-visible, cryptographic, Route, naming, and WebTunnel bytes | Retain semantic contract only; choose `read/migrate`, `break`, or `delete` per observer. | DA-05 through DA-07 and DA-10. |
| Q01-Q03 evidence/test/document records | Keep provenance separately from current product truth; only accepted claim evidence becomes Qualification. | S8.2 profile policy and M14 retirement ledger. |

No target wave may add a forwarding writer, indefinite legacy decoder, or
unbounded compatibility mode. The S8.4 plan records a concrete mode, observer,
cutover, rollback/forward-repair behavior, and deletion condition for every row
it mutates.

## Acceptance and stop rules

This proposal becomes the S8.3 design authority only when the Product Owner
accepts it. DA-01 through DA-10 remain explicit stop conditions for their
dependent waves. On acceptance, S8.4 may instantiate the M0-M14 migration and
retirement ledger; code moves begin only in an accepted S8.5 wave.
