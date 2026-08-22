# Stage 8 preservation and compatibility ledger

Status: **S8.1 in progress.** On 2026-08-22 the Product Owner selected
`continue` for the accepted Product Core and H3 candidate. This temporary
ledger applies the resulting preservation disposition to the clean Stage 8
entry `1cf7100da3ada32ba53abb51201aaf7b6183a3da`. It is not a target package
map, a format change, a support promise, or a Qualification result. It is
removed at S8.6 after its surviving facts have one canonical owner.

## Accepted scope and honest limitations

`continue` preserves the Product Core: a local Application uses an opaque,
ordered and protected Service Connection to a location-independent Target; a
Route protects the stated role-local information-flow boundary; failures are
explicit and never fall back to direct or weaker transport; and a known exact
unlisted Service Name remains distinct from discovery, authorization, and the
Target Link path.

It also preserves the H3 candidate as a project-controlled, bounded Closed
Test Network. That is not a Public Beta, public anonymous network,
decentralized network, supported installed product, or a release of a route
privacy claim. In particular, a passing local test, test-network campaign, or
historical laboratory receipt does not establish independent operation,
Application isolation, production custody, public naming, update, platform, or
Route Qualification. The conditions and limits in the accepted product scope,
vision, journeys, and threat model remain mandatory.

No current command name, package, internal Go Interface, plan schema, JSON
shape, socket path, lab topology, transport, cryptographic representation, or
test file gains preservation merely from this decision. Those are classified
below by their observable contract and actual observer.

## Contract disposition

The table completes the prepared G0 family disposition. `Preserve` retains the
stated semantic obligation. `Migrate` retains it through an explicit
compatibility/cutover plan. `Replace` preserves no representation. `Decide
first` means the named authority and evidence must resolve the mechanism before
S8.3 fixes an Interface or format; it does not permit implementation in the
meantime.

| G0 family | S8.1 disposition | Binding consequence |
|---|---|---|
| P01 exact-target protected Service Connection | Preserve | A target owner may replace Route, handshake, buffering, or IPC only while keeping authenticated current Instance, protected ordered bytes, explicit terminal outcome, and no Application-operation replay. |
| P02 opaque data and local Application Interface | Preserve | Ardents remains a carrier, not an Application runtime, SDK mandate, identity model, or payload store. |
| P03 separate Connection, Administration, and Custody authority | Preserve | A target architecture must keep this privilege lattice; no less-privileged caller receives Custody. |
| P04 location-independent Target and Instance lifecycle | Preserve | Routine migration uses a fresh non-exported Instance Key; Authority loss/compromise replaces the Target. |
| P05 unlisted Service Name | Preserve | A known Name resolves to a Target through distinct authority; it is neither discovery nor a direct-network fallback. |
| P06 Interactive Route knowledge boundary | Preserve with its stated limitations | Current topology and transport are not preserved. One ordinary role-local Node must not obtain a forbidden binding; collusion and Broad Traffic Observation remain limitations, and no qualification claim is implied. |
| P07 Stage 1--7 mechanisms/evidence | Replace or remove by the concrete rows below | Historical implementation effort creates no Product Core obligation. |
| S01 fail-closed authentication/freshness/profile behavior | Preserve | A detected violation, unavailable state, or unsupported profile never becomes retryable success or silent downgrade. |
| S02 distinct least-privileged authorities | Preserve | Service/Name roots, Instance Keys, Node identities, Local Grants, release authority, and Application Principals remain distinct wherever their retained contract requires them. |
| S03 role-local disclosure boundary | Preserve with its stated limitations | A replacement topology needs equivalent or explicitly superseding claim/evidence; no one-Node anonymity claim is earned here. |
| S04 Isolation Context and Network-Isolated Application boundary | Preserve as a claim condition, not as present behavior | Generic adapters carry the weaker honest claim. A claim-bearing native profile needs an accepted platform design and evidence before implementation or advertisement. |
| S05 finite observable resource/lifecycle behavior | Preserve where the retained product/claim exercises it | A new owner must retain bounded work, cancellation, pressure, cleanup, and observability rather than moving them behind an unbounded helper. |
| S06 complete honest security/privacy statements | Preserve | Promotion may shorten duplicated prose but cannot remove protected information, adversary, conditions, measurement, or limitation. |
| L01 monotonic lifecycle and recovery | Preserve | Restart, update, rollback, revocation, expiry, drain, shutdown, and removal may change owner only with explicit non-resurrection behavior. |
| L02 retained observable encodings | Migrate | Each concrete encoding receives a G0-M0--M4 mode, observer, reader/writer/cutover rule, and deletion condition in S8.3/S8.4. |
| L03 Application Interface and result semantics | Preserve | Transport framing, paths, and command syntax may change; exactly one classified terminal result remains required. |
| L04 Qualification identity/evidence integrity | Preserve | A changed candidate cannot inherit earlier evidence; claim-bearing evidence needs its identity, retention, and independent recomputation rule. |
| L05 supported host/install/privilege claims | Narrow to H3 research hosts | No shipped host/install/update/isolation claim exists at S8.1. Any later supported profile needs a Product Owner scope decision, operating contract, and platform evidence. |
| I01 Go/root module/first-party monorepo | Preserve | ADR-0009/0010 remain binding. |
| I02/I03 present package graph, internal and exported Go shapes | Replace freely subject to retained observers | S8.3 may replace them after recording callers and formats; no exported symbol is an external promise without evidence. |
| I04 decision-bound technologies | Decide first | Existing ADRs constrain their accepted scope; a withdrawn/open choice needs its required research and ADR before a retained protocol, cryptographic, storage, or transport decision. |
| I05 test files/fixtures/helpers | Replace or remove by risk role | Preserve the owned behavior/fault/claim evidence, not historical test seams. |
| I06 laboratory/temporary Adapter/compatibility machinery | Promote, retain as provenance, or remove by the concrete rows below | No laboratory code enters the runtime product graph by default. |
| I07 stage documents and duplicate prose | Remove after promotion | Accepted ADR/research remains provenance; current facts move to the canonical product, technical, operations, or reference owner. |
| I08 dependencies/external tools | Decide per real retained caller | A dependency is retained only with operational/security fit and a non-speculative caller; protocol/evidence-bound changes need compatibility authority. |

## Concrete boundary and compatibility disposition

The S8.0 [surface inventory](stage-8-current-system-surface-inventory.md)
locates the source owners. This ledger supplies the product disposition. A
`Migrate` row does not promise a public compatibility window: S8.3 discovers
real external observers and selects G0-M0 (coordinated switch), G0-M1
(side-by-side state conversion), G0-M2 (bounded external Adapter), G0-M3
(protocol phase), or G0-M4 (retire/export).

| Rows | S8.1 disposition | Required S8.3 authority before mutation |
|---|---|---|
| A01 raw local Application bytes; A02 terminal result | Migrate P01/P02/L03 semantics; replace socket and `ASRS` representation only with an observer-based support rule. | Endpoint/Application Interface design; external-Application caller inventory. |
| A03 publication Administration socket | Migrate P03 privilege separation; text request and helper command are replaceable. | Service Publication/Endpoint design and operator caller inventory. |
| A04 product-candidate commands | Replace current command topology; retain only the product/operator journeys chosen for thin commands in M13. | S8.3 command/process map and discovered automation consumers. |
| A05 plan/input JSON; A06 machine results/diagnostics | Migrate only outcomes, ordering/bounds, redaction, and real consumers; plans never become authority. | Per-owner format ledger and compatibility mode. |
| A07 stream environment/direct modes | Remove from shipped product; retain a test Adapter only while a named risk test requires it. | M10/M13 test replacement and caller search. |
| D01 Network State; D02 Local roles; D03 Bridge; D04 naming state; D05 publication/Instance hand-offs | Migrate with one writer and explicit G0-M1 recovery/cutover; preserve the listed monotonic, tamper, replay, expiry, and key-secrecy invariants. | Their S8.3 state-owner/format rows. |
| D06 Release floors | Migrate security-forward only; preserved floors/trusted roots must never decrease. | Release owner and forward-repair/reader rule. |
| D07 Update transaction | Retain as an H3 technical security/recovery input, not a shipped H3 product command; migrate only after release authority, real activation, and classified recovery are designed. | Release/Update design and resolved root-rotation transaction semantics. |
| D08 Vault/Recovery Bundle | Preserve P03/S02 custody semantics and ADR-0021 provenance; no current implementation is promoted. | Custody design, secret-input/export/restore/reconciliation lifecycle, and platform evidence. |
| W01 Network record/source/node bytes; W02 Route/Service Connection bytes | Preserve P01/P06/S01/S03 semantics; decide protocol representation before implementation. | Accepted protocol research/ADR, mixed-version/downgrade/retirement rule, and Qualification impact. |
| W03 Name/claim/recovery/OHTTP bytes | Preserve P05/S01/S02 semantics; do not treat current algorithm/profile as a product promise. | Applicable naming research/ADR and old-record/verifier retirement rule. |
| W04 Bridge/WebTunnel bytes | Keep only as an H3 experiment/technical input; no public censorship-resistance promise and no automatic product transport retention. | Product Owner decision after transport evidence; otherwise G0-M4 removal. |
| Q02 tests/e2e/live | Replace by retained requirement, seam, oracle, fault/adversary, platform/format, and independence role. | S8.2 profile/manifest policy. |
| Q03 stage docs and current-behavior prose | Promote unique current facts, then remove. | S8.2 documentation policy and M14 retirement ledger. |

## Commands, laboratories, and deferred campaigns

| Family | Product/claim disposition | Code/evidence disposition |
|---|---|---|
| `ardents`, `ardents-name`, `ardents-node`, `ardents-route`, `ardents-service` | Their retained Product Core/H3 journeys continue; none of their present names, flags, plans, or one-command-per-stage shapes is a compatibility promise. | Replace with the M13 thin operator surface after S8.3 selects composition. |
| `ardents-bridge` | Bridge/camouflage remains only a bounded H3 experiment, not a public entry promise. | Retire or replace as M7 determines; no external command support is assumed. |
| `ardents-release` | Security-forward release/update constraints remain technical inputs; production release/update is outside H3 shipped scope. | Replace/retire the command topology after M1/M2; do not infer an installer or updater product promise. |
| `ardents-publish-app`, `ardents-stream-app` | Not product UI. They currently expose publication/Application tracer seams. | Keep only while a named test/example Adapter needs them; then remove with its replacement test. |
| Carrier Lab and Named Unlisted Site lab commands/evidence | Historical provenance, not current runtime product or live qualification. | Retain immutable record/evidence access; remove code runners at M14 unless a specific accepted reproduction obligation still names them. |
| Blocked-entry and Stage 6 lab/verifier commands/evidence | H3 historical/claim evidence inputs only; they make no production or public claim. | Keep a reproducer only while linked to an accepted claim/source identity; otherwise retain the record and remove the code at M14. |
| `internal/lab/*`, live test cells, external images/binaries | No product-runtime role. | S8.2 must assign one evidence profile and prerequisite receipt to each retained item; unowned code/fixtures are removed at M14. |

## Absent accepted surfaces and decision authority

| Surface | S8.1 disposition | Authority needed before S8.3/M-wave work |
|---|---|---|
| Application Broker and native Isolation | Preserve the identity/confinement contract as an eventual claim condition; no current H3 behavior or platform support is asserted. | Product Owner must select a qualified operating profile; accepted platform/Adapter design and evidence are required before M10. |
| Install/repair/uninstall, activation, update | Preserve the honest lifecycle requirement, but H3 has no shipped install/update support. | Product Owner scope decision plus release/update/operations design before M1/M2/M13. |
| Custody implementation | Preserve the accepted custody separation and ADR-0021 format authority; no package is assumed to exist. | Custody design and Product Owner acceptance after its dependent authorities stabilize (M12). |
| Public naming, permissionless admission, independent-operation or public-route claims | Outside current H3 product scope. | A future-horizon Product Owner promotion, research/ADR, and external evidence; no S8 wave may add them as an implied backlog. |

## S8.1 acceptance status

The product disposition, retained scope, honest limitations, concrete
preservation classes, and laboratory/deferred-campaign treatment are now
recorded. The companion
[decision-authority register](stage-8-decision-authority-register.md) names
every `Decide first` route that can constrain S8.3. S8.1 is ready for Product
Owner acceptance; no Module migration is authorized until that acceptance is
explicitly recorded.
