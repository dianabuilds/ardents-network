# Stage 8 current-system surface inventory

Status: **S8.0 factual inventory at source entry
`1cf7100da3ada32ba53abb51201aaf7b6183a3da`.** This temporary inventory makes
the G0 prepared matrix current by binding it to the clean entry, the S8.0 delta
reviews, and current package-map/architecture evidence. A class is an inventory
fact, not the S8.1 Product Owner disposition.

## Application, operator, and process surfaces

| ID | Concrete entry surface and current owner | Observer/trust crossing | Class |
|---|---|---|---|
| A01 | Raw local Application bytes: `applicationipc` plus Endpoint Application socket. | External Application -> Endpoint. | Migrate |
| A02 | `ASRS` v1 terminal frame and `.result`/raw-tail compatibility in `serviceendpoint`. | Application -> Endpoint process. | Migrate |
| A03 | `publish\n` / `published\n` Administration socket used by `ardents-publish-app`. | Publication operator -> Endpoint. | Migrate |
| A04 | Candidate commands: `ardents`, `ardents-name`, `ardents-node`, `ardents-bridge`, `ardents-route`, `ardents-service`, `ardents-release`. | Operator/automation -> composition roots. | Decide first |
| A05 | Versioned source/node/bridge/route/name/service plan and input JSON. | Adjacent process/test/operator -> composition. | Migrate |
| A06 | JSON events/results: Network/Node/Route, Name, Release/Update, stream tracer. | Automation/evidence collector -> command boundary. | Migrate |
| A07 | `ARDENTS_STREAM_*` and direct stream modes. | Development/live workload driver. | Replace freely |

The current process roots are the nine non-laboratory commands listed in the
[current-system report](stage-8-current-system-report.md); `ardents-stream-app`
and `ardents-publish-app` are external tracers, and six `*-lab` commands are
historical/evidence runners. `cmd/ardents-service` has no package tests. The
current command syntax, plan schemas, socket paths, direct TCP mode, workload
limits, and stage composition are not implicit product compatibility promises.

## Durable roots and hand-off formats

| ID | Current root/format and owner | Observer/adversary | Class |
|---|---|---|---|
| D01 | `.ardents-network-state-v1`, Network generations, distribution/pointers/lock in `network/store`. | Restart, Source/Node, tamper actor. | Migrate |
| D02 | `.ardents-local-roles-v1`, JSON generations, current/watermark/lock in `localroles`. | Route/Node/Bridge/restart. | Migrate |
| D03 | `.ardents-bridge-state-v1`, replay/attempt generations and watermark in `bridge`. | Bridge/restart/hostile Invite peer. | Migrate |
| D04 | Naming snapshots, leaves/proofs, `ardents-naming-state-snapshot-v2` in `namestore`. | Resolver/Gateway/restart/tamper actor. | Migrate |
| D05 | Publication file, Instance lifecycle generation, Administration/Application/Route sockets in `serviceendpoint`/`serviceconn`. | Service operator/Endpoint/Route. | Migrate |
| D06 | `.ardents-release-decision-v1`, verified roots/floors/current/lease in `releasedecision`. | Release verifier/operator/rollback. | Migrate, security-forward only |
| D07 | `.ardents-update-transaction-v1`, `ARDUPD01`, immutable generations, staging, activation and journal in `updatetransaction`. | Bootstrap/update/restart/rollback. | Migrate |
| D08 | Accepted Authority Vault/Recovery Bundle envelope (ADR-0021); no Custody Module exists at entry. | Authority custodian/offline attacker/restore. | Decide first |

Each D-row has a distinct writer and trust model today. A shared filesystem
helper is not assumed: its possible use is an S8.3 technical-Adapter decision.
The D06 migration cannot lower floors; D07 is only migrated from classified
quiescent/recovered state; D08 cannot be silently activated or reset.

## Peer-visible and cryptographic representations

| ID | Current representation | Observer/adversary | Class |
|---|---|---|---|
| W01 | Network Epoch/record bytes, Source TLS exchange, role selection, distribution commitments, Node probe framing. | Source/Node/Endpoint/malicious peer. | Decide first |
| W02 | Route introduction/acknowledgement, sealed introduction v2, Service Connection frames, TLS labels/domain tags. | Endpoints/role-local Node/active attacker. | Decide first |
| W03 | Name Record v3, signed/control/recovery/claim transcripts, Namespace proof and OHTTP Gateway exchange. | Name authority/resolver/epoch authority/replay attacker. | Decide first |
| W04 | Bridge Invite domains/transition, WebTunnel candidate commitment/identity. | Bridge owner/invited peer/blocker. | Decide first |

These rows are protocol/claim surfaces rather than implementation types. A
retained change needs its accepted research/ADR, explicit mixed-version or
retirement behavior, and Qualification impact; no refactor may silently change
one.

## Testing, evidence, and documentation surfaces

| ID | Current surface | Observer | Class |
|---|---|---|---|
| Q01 | Six lab commands and Stage 5/6 manifests, observations, verdicts, and independent verifier schemas. | Product Owner/auditor/verifier. | Decide first |
| Q02 | Unit, `tests/e2e`, `tests/live`, fixtures/goldens/test exports. | Maintainer/test runner. | Replace freely subject to traceability |
| Q03 | Stage briefs/plans, ADR/research provenance, technical design, maps, and behavior prose. | Maintainer/operator/auditor. | Remove after promotion |

At entry there are 140 Markdown documents under `docs/`: 22 ADR, 60
development, 5 product, 50 research, and 1 security document plus the root
indexes/records counted by the current-system report. The document categories
are an inventory rather than a quality target. Product/scope/threat documents
are current authorities; ADR and research records are provenance; completed
stage material is transitional until its unique current facts are promoted.

## Accepted surfaces absent from entry source

| Accepted contract | Entry fact | Required treatment |
|---|---|---|
| Launcher-born Application Principal and native isolation | No Broker, Isolation, or Grant owner/package exists. | Add concrete process, IPC, cleanup, and platform evidence before claiming it. |
| Native install/repair/uninstall and versioned activation | No supported installed-path composition exists. | Inventory registration, privilege, activation, rollback, residue, and purge before support. |
| Authority Vault/Recovery Bundle implementation | ADR-0021 exists but no Custody owner/package exists. | Add secret-input, export/restore/reconciliation/resource/platform rows before promotion. |

## Inventory conclusion

Every entry surface is now located by owner, observer/trust crossing, and
inventory class. The Stage 7 delta does not add an unrecorded command,
dependency, package, durable root, or wire surface; its substantive Update
change is covered by D07 and the Release/Update delta review. The outstanding
S8.0 evidence is test-portfolio measurement/disposition, not an unlocated
runtime surface. S8.1 decides which `Decide first` and `Migrate` rows survive.
