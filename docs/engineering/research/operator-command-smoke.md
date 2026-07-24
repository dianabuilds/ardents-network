# Operator Command Smoke Research Packet

## Decision

- Decision owner: Operator Interface / CLI
- Date: 2026-07-25
- Baseline commit:
  `main@180decc1b03f94a6115b59a4046b4795308ec235`
- Research class: R1 bounded investigation
- Recommendation: implement one command-contract foundation and four
  procedure-level smoke slices; retain environment qualification as R3

The Operator CLI has 68 reachable leaf commands across offline custody,
identity administration, Node/network/diagnostic inspection, workload/content
operations and interactive surfaces. Online commands consistently use the
supported generated Operator Interface through the shared authenticated
`internal/cli/client`; no product command calls a domain implementation
directly.

The command surface is implemented and broadly reachable, but it is not
operable as one documented contract:

- group help is intercepted before domain help and therefore hides every
  online leaf list;
- only a minority of leaf procedures has CLI-level integration/E2E smoke;
- JSON is deterministic in implementation but its per-command schema and
  stream shape are not catalogued as a CLI contract;
- RPC success always exits zero even when a mutation response explicitly says
  it was not accepted or was rejected;
- the same generated Operator client is correctly shared, but there is no
  machine check from command path to RPC procedure, access action, output shape
  and smoke owner.

Current maturity is:

| Dimension | Status | Basis |
|---|---|---|
| Implemented | yes | all 68 leaf parsers and their offline/RPC operations exist |
| Reachable | partial | leaf execution is reachable; supported help cannot enumerate online leaves |
| Operable | partial | shared errors/session/SSH exist, but smoke and outcome contracts are incomplete |
| Qualified | no | tagged Linux scenarios exist but did not run in this Windows research; no complete release matrix |

## User Outcome

An Operator can discover every supported command from the root `--help`, run
the documented procedure locally or through SSH stream-local forwarding, and
rely on:

- the exact administrative action and scope required;
- one documented human and JSON result shape;
- an exit status that distinguishes usage, transport/API failure and a
  domain-level rejected mutation;
- a deterministic smoke that drives the same command and Operator Interface as
  the real CLI;
- a trace from command to claim, RPC, access action and evidence.

## In Scope

- every leaf exposed by the current `ardentsctl` parser;
- offline Principal/device/Delegation custody commands;
- online Identity, Node, Network, Workload, Content, Transfer, Retention,
  Diagnostics and Configuration commands;
- `shell`, `tui` and `version`;
- human output, JSON output, error and exit semantics;
- direct protected Unix and SSH stream-local Operator transport;
- current unit, integration and tagged E2E smoke;
- a fail-closed command contract catalogue and grouped acceptance slices.

## Out Of Scope

- changing Operator RPC messages or authorization actions;
- remote administration by plaintext TCP or `ssh -W`;
- Application Interface or Application SDK features;
- adding one issue per command;
- redesigning the TUI;
- deployment, native installation or release qualification;
- changing domain operation/recovery semantics;
- cross-language Operator clients.

## Current Supported Interface

`ardentsctl` is the supported Operator presentation adapter. Online commands
resolve a protected Operator transport, authenticate an Operator Principal,
and call one generated `ardents.v1` service through
`internal/cli/client.Service` (`internal/cli/client/client.go:24-79`).

Direct mode uses the protected Operator Unix socket. Remote mode uses OpenSSH
stream-local forwarding to the same remote socket. The transport never falls
back to plaintext TCP or `ssh -W`
(`docs/operations/operator-access-contract.md:29-43`).

Offline Principal/device/Delegation custody commands use protected local
files. `version` is local. `shell` reuses the same live client/session and
dispatches ordinary command paths. `tui` uses the same command context and
Operator client.

The action owner for protected non-Identity RPCs is the closed server catalogue
in `internal/localapi/auth/access_catalog.go:21-69`. Identity procedure classes
and actions are closed in `internal/localapi/identity/catalog.go:18-31`.

## Current Reachable User Journey

The supported procedure is:

```text
discover command
  -> resolve context and exact Node Principal
  -> create direct Unix or SSH stream-local transport
  -> load finite device Credential
  -> Begin/Complete Operator Session
  -> invoke exact Operator RPC
  -> server canonicalizes resource and admits exact action/grant
  -> render human or JSON result
  -> close client and end Session
```

Inside `shell`/`tui`, the live client reuses the same bound Session. A unary
call retries exactly once only after `Unauthenticated`. `PermissionDenied`,
validation, conflict, capacity and ordinary transport errors do not select
another Credential.

### Help reachability defect

The root dispatcher calls `renderGroupIfRequested` before a domain command is
constructed (`internal/cli/run.go:63-125`). Therefore:

```text
the Node command group with its help argument
```

prints only:

```text
Usage: <operator-cli> [global flags] node <subcommand>
node lifecycle, runtime status and events
```

The detailed module usage
`node <start|stop|status|runtime|features|events>` exists at
`internal/cli/node/command.go:47-49` but is unreachable. The same masking
affects `network`, `workload`, `data`, `diagnostics`, `config`, `identity` and
`tui`. `shell help` is not intercepted and instead requires full Operator
context resolution before it can print help. Only nested offline Identity help
currently reaches its detailed leaf list without a Node.

## Current Implementation

### Shared transport, session and failures

- `internal/cli/client` composes all generated Operator service clients once.
- the session interceptor caches by exact audience, single-flights login and
  retries one `Unauthenticated`;
- transport configuration accepts protected Unix and SSH stream-local only;
- `output.Renderer.Failure` produces a common human or JSON error object and
  returns exit 1 (`internal/cli/output/renderer.go:147-207`);
- parser/configuration errors return exit 2;
- successful handler return currently means exit 0.

### Human and JSON projection

Most online commands render a concise human projection and emit the complete
protobuf response with `EmitUnpopulated` in JSON mode. Identity commands emit
custom `encoding/json` DTOs with snake_case fields. Event and watch modes emit
JSON Lines. `shell` explicitly rejects JSON; TUI is interactive.

These shapes are deterministic in the current implementation. They are not
enumerated or versioned as CLI contracts, and no machine check prevents a
handler from silently switching projection styles.

### Exit outcome gap

Online mutation handlers return zero whenever the RPC returned a response.
They do not consistently inspect `OperationStatus.accepted`, configuration
reload outcome, or an equivalent domain result. For example,
`config reload` returns zero for `rejected_invalid`, and Node/workload
mutations print a possibly non-accepted status and still return zero
(`internal/cli/configuration/command.go:62-82`,
`internal/cli/node/command.go:51-80`,
`internal/cli/workload/command.go:97-164`).

This is safe for interactive inspection only if the Operator reads the body.
It is misleading for the README-supported `--output json` automation path.

## Command Catalogue

Legend:

- Output: `P` = protobuf JSON plus named human projection; `C` = CLI-owned
  custom JSON DTO plus named human projection; `JL` = JSON Lines/human stream;
  `H` = human interactive only; `L` = local deterministic human/JSON.
- Smoke: `U` = CLI unit/Unix integration; `I` = tagged CLI integration;
  `E` = tagged terminal/process E2E; `-` = no CLI-level smoke. Domain/API tests
  may still exist.
- SSH: `yes` = same command supports SSH stream-local forwarding; `n/a` =
  offline/local-only.
- Status: `R/O` = reachable/operable on the current commit. `yes/partial`
  means executable but missing command-specific smoke or stable outcome
  evidence.
- Docs: `H` = production help/usage strings; `R` = README Operator CLI;
  `O` = Operator runbook/access/configuration contracts; `I` = Principal CLI
  contracts.

Every protected action below is the exact action registered by the server, not
an inference from command naming.

### Node lifecycle

| Command | Procedure / domain action | Client/module | Required grant | Human / JSON | Exit | Smoke | SSH | R/O | Docs |
|---|---|---|---|---|---|---|---|---|---|
| `node start` | `NodeService.StartNode` / start Node | `cli/node` -> shared `NodeServiceClient` | `node.start`, Node | `node start`, status / P | 0 on RPC response; 1 API; 2 usage | E | yes | yes/partial: accepted=false still 0 | H,R |
| `node stop` | `StopNode` / stop Node | same | `node.stop`, Node | `node stop`, status / P | same | E | yes | yes/partial | H |
| `node status` | `GetNodeStatus` / inspect lifecycle truth | same | `node.status`, Node | lifecycle/readiness/features / P | 0/1/2 | E | yes | yes/yes | H,R,O |
| `node runtime` | `GetNodeRuntime` / inspect boot, identity and health | same | `node.runtime`, Node | runtime summary / P | 0/1/2 | - | yes | yes/partial | H |
| `node features` | `GetNodeFeatures` / inspect service surface | same | `node.features`, Node | version/services / P | 0/1/2 | - | yes | yes/partial | H |
| `node events --watch [--limit N]` | `StreamNodeEvents` / observe Node events | same | `node.events`, Node | events / JL | 0 clean/limit/cancel; 1 stream; 2 flags | E | yes | yes/yes | H |

### Network and Discovery

| Command | Procedure / domain action | Client/module | Required grant | Human / JSON | Exit | Smoke | SSH | R/O | Docs |
|---|---|---|---|---|---|---|---|---|---|
| `network status` | `GetNetworkStatus` / inspect participation | `cli/network` -> `NetworkServiceClient` | `transport.network_status`, Node | reachability/profile/privacy / P | 0/1/2 | I,E | yes | yes/yes | H,R,O |
| `network discovery` | `GetDiscoveryStatus` / inspect discovery store | same | `discovery.status`, Node | record counts/state / P | 0/1/2 | I,E | yes | yes/yes | H |
| `network presence` | `GetLocalPresence` / inspect local publication | same | `discovery.local_presence`, Node | publication/action-required / P | 0/1/2 | - | yes | yes/partial | H |
| `network peers` | `ListPeers` / inspect peer truth | same | `discovery.peers`, Node | peer list / P | 0/1/2 | I | yes | yes/partial: no E2E | H |
| `network routes [--service TYPE]` | `ListRouteCandidates` / inspect route eligibility | same | `transport.route_candidates`, Node | route candidates/reasons / P | 0/1/2 | I,E | yes | yes/yes | H |
| `network resolve record --id ID` | `ResolveRecord` / resolve exact record | same | `discovery.resolve_record`, exact discovery record | selected/candidates/trust / P | 0/1/2 | - | yes | yes/partial | H |
| `network resolve service --type TYPE` | `ResolveService` / administrative service resolution | same | `discovery.resolve_service`, exact service | selected/candidates/trust / P | 0/1/2 | - | yes | yes/partial | H |
| `network records list` | `ListRecords` / inspect catalogue | same | `discovery.list_records`, collection | signed record list / P | 0/1/2 | - | yes | yes/partial | H |
| `network records import --file FILE` | `ImportRecord` / import signed record | same | `discovery.import`, exact record | imported record/status / P | 0 on RPC response; 1 API/input; 2 flags | - | yes | yes/partial | H |

### Workloads and hosted services

| Command | Procedure / domain action | Client/module | Required grant | Human / JSON | Exit | Smoke | SSH | R/O | Docs |
|---|---|---|---|---|---|---|---|---|---|
| `workload list` | `ListWorkloads` / inventory workloads | `cli/workload` -> `WorkloadServiceClient` | `workload.list`, collection | workload summaries / P | 0/1/2 | - | yes | yes/partial | H,R |
| `workload get ID` | `GetWorkloadStatus` / inspect workload | same | `workload.status`, exact workload | desired/observed/recovery / P | 0/1/2 | - | yes | yes/partial | H |
| `workload register --file FILE` | `RegisterWorkload` / register specification | same | `workload.register`, exact workload | status/workload / P | 0 on RPC response; 1 API/input; 2 flags | E | yes | yes/partial: accepted=false still 0 | H |
| `workload start ID` | `StartWorkload` / start workload | same | `workload.start`, exact workload | status/workload / P | same | - | yes | yes/partial | H |
| `workload stop ID` | `StopWorkload` / stop workload | same | `workload.stop`, exact workload | status/workload / P | same | I | yes | yes/partial: no E2E command assertion for stop outcome | H |
| `workload restart ID` | `RestartWorkload` / restart workload | same | `workload.restart`, exact workload | status/workload / P | same | - | yes | yes/partial | H |
| `workload services` | `ListHostedServices` / inventory hosted services | same | `workload.hosted_services`, collection | hosted-service summaries / P | 0/1/2 | I | yes | yes/partial: no E2E | H |
| `workload service ID` | `GetHostedService` / inspect readiness/backing | same | `workload.hosted_service`, exact service | readiness/endpoints / P | 0/1/2 | I,E | yes | yes/yes | H |
| `workload publication ID` | `GetServicePublicationStatus` / inspect publication | same | `workload.service_publication`, exact service | publication/withdrawal / P | 0/1/2 | I,E | yes | yes/yes | H |

### Content, retention and transfer

| Command | Procedure / domain action | Client/module | Required grant | Human / JSON | Exit | Smoke | SSH | R/O | Docs |
|---|---|---|---|---|---|---|---|---|---|
| `data inventory` | `GetDataInventory` / inspect content totals | `cli/content` -> `ContentServiceClient` | `data.inventory`, inventory | counts / P | 0/1/2 | I,E | yes | yes/yes | H,R |
| `data objects list` | `ListObjects` / list owned Objects | same | `data.list_objects`, collection | ID/type/owner / P | 0/1/2 | - | yes | yes/partial | H |
| `data objects get ID` | `GetObject` / get exact Object | same | `data.get_object`, exact Object including owner | ID/type/owner / P | 0/1/2 | - | yes | yes/partial | H |
| `data objects publish --file FILE` | `PublishObject` / publish Object | same | `data.publish_object`, exact Object | published ID/type / P | 0 on RPC response; 1 API/input; 2 flags | - | yes | yes/partial | H |
| `data blobs list` | `ListBlobs` / list Blobs | same | `data.list_blobs`, collection | reference/state/retention / P | 0/1/2 | - | yes | yes/partial | H |
| `data blobs get ID` | `GetBlob` / inspect Blob | same | `data.get_blob`, exact Blob | reference/state/retention / P | 0/1/2 | - | yes | yes/partial | H |
| `data blobs publish --file FILE` | `PublishBlob` / publish Blob metadata/payload reference | same | `data.publish_blob`, exact prospective Blob | Blob summary / P | 0 on RPC response; 1 API/input; 2 flags | - | yes | yes/partial | H |
| `data blobs fetch ID` | `TransferService.FetchBlob` / start private fetch | `cli/content` -> `TransferServiceClient` | `data.fetch_blob`, exact Blob | Blob summary / P | 0 on RPC response; 1 API; 2 usage | I,E | yes | yes/partial: async accepted outcome not classified | H |
| `data blobs sources ID` | `ListBlobSources` / inspect eligible sources | same | `data.blob_sources`, exact Blob | source/service/usability / P | 0/1/2 | I | yes | yes/partial: no E2E assertion after failure | H |
| `data blobs retain --id ID --expires-at TIME` | `RetentionService.RetainBlob` / set retention | `cli/content` -> `RetentionServiceClient` | `data.retain_blob`, exact Blob | Blob summary / P | 0 on RPC response; 1 API/input; 2 flags | - | yes | yes/partial | H |
| `data blobs pin ID` | `PinBlob` / pin Blob | same | `data.pin_blob`, exact Blob | Blob summary / P | 0 on RPC response; 1 API; 2 usage | - | yes | yes/partial | H |
| `data blobs drop ID` | `DropBlob` / drop local retention | same | `data.drop_blob`, exact Blob | Blob summary / P | 0 on RPC response; 1 API; 2 usage | - | yes | yes/partial | H |
| `data manifests list` | `ListManifests` / list Manifests | `cli/content` -> `ContentServiceClient` | `data.list_manifests`, collection | ID/kind/owner / P | 0/1/2 | - | yes | yes/partial | H |
| `data manifests get ID` | `GetManifest` / get exact Manifest | same | `data.get_manifest`, exact Manifest including owner | ID/kind/owner / P | 0/1/2 | - | yes | yes/partial | H |
| `data manifests publish --file FILE` | `PublishManifest` / publish Manifest | same | `data.publish_manifest`, exact Manifest | ID/kind / P | 0 on RPC response; 1 API/input; 2 flags | - | yes | yes/partial | H |
| `data transfers list` | `ListTransfers` / inspect transfer inventory | `cli/content` -> `TransferServiceClient` | `data.list_transfers`, collection | transfer summaries / P | 0/1/2 | I,E | yes | yes/yes | H |
| `data transfers get ID` | `GetTransfer` / inspect transfer recovery truth | same | `data.get_transfer`, exact transfer | state/resource/reason / P | 0/1/2 | I,E | yes | yes/yes | H |

### Diagnostics and configuration

| Command | Procedure / domain action | Client/module | Required grant | Human / JSON | Exit | Smoke | SSH | R/O | Docs |
|---|---|---|---|---|---|---|---|---|---|
| `diagnostics snapshot` | `GetDiagnostics` / inspect combined diagnostics | `cli/diagnostics` -> `DiagnosticsServiceClient` | `diagnostics.snapshot`, diagnostics | health/event/pending counts / P | 0/1/2 | - | yes | yes/partial | H |
| `diagnostics health` | `GetHealthSummary` / inspect readiness causes | same | `diagnostics.health_summary`, diagnostics | health/degraded domains/action / P | 0/1/2 | I | yes | yes/partial: no terminal E2E | H |
| `diagnostics pending` | `GetPendingOperations` / inspect incomplete operations | same | `diagnostics.pending_operations`, collection | state/recovery action / P | 0/1/2 | E | yes | yes/yes | H,O |
| `diagnostics explain --scope S [--resource-id ID]` | `ExplainFailure` / explain exact diagnostic subject | same | `diagnostics.explain_failure`, exact subject | reason/impact/recovery / P | 0/1/2 | I,E | yes | yes/yes | H,O |
| `diagnostics events [--limit N --cursor C]` | `ListRecentEvents` / inspect recent events | same | `diagnostics.recent_events`, collection | events/cursor / P | 0/1/2 | I | yes | yes/partial: no E2E | H |
| `config show` | `GetEffectiveConfiguration` / inspect redacted effective config | `cli/configuration` -> `ConfigurationServiceClient` | `config.effective`, configuration | generations/fingerprint/redacted config / P | 0/1/2 | I | yes | yes/partial: no E2E | H,R,O |
| `config reload` | `ReloadConfiguration` / validate and atomically reload | same | `config.reload`, configuration | outcome/generations/restart paths / P | currently 0 for applied, restart-required and rejected outcomes; 1 API; 2 usage | I | yes | yes/partial: rejection exit ambiguous | H,O |

### Principal custody, enrollment, access and sessions

| Command | Procedure / domain action | Client/module | Required grant | Human / JSON | Exit | Smoke | SSH | R/O | Docs |
|---|---|---|---|---|---|---|---|---|---|
| `identity principal create` | offline root creation | `cli/identity` protected files | none | public root metadata / C | 0/1/2 | U | n/a | yes/yes | H,I |
| `identity principal import` | offline validated no-replace import | same | none | public root metadata / C | 0/1/2 | U | n/a | yes/yes | H,I |
| `identity principal show` | offline public root inspection | same | none | public root metadata / C | 0/1/2 | U | n/a | yes/yes | H,I |
| `identity device create` | offline finite Key Credential creation | same | root signer file | public device/Credential metadata / C | 0/1/2 | U | n/a | yes/yes | H,I |
| `identity device show` | offline public device inspection | same | none | public device metadata / C | 0/1/2 | U | n/a | yes/yes | H,I |
| `identity device revoke` | `IdentityService.RevokeDevice` / revoke exact Credential device | `cli/identity` -> protected `IdentityServiceClient` | `identity.device.revoke`, exact Principal/device | confirmed mutation / C | 0 on RPC response; 1 denial/API; 2 usage | U | yes | yes/partial: no process smoke | H,I,O |
| `identity enroll` | public first enrollment or protected later enrollment | `cli/identity` -> public/protected Identity client | Bootstrap Ticket for first; `identity.principal.enroll` for later | enrollment metadata / C | 0 committed and ticket cleaned; 1 incl. committed-cleanup failure; 2 usage | U,E via APP-001 bootstrap | yes | yes/partial: later-enrollment process smoke absent | H,I,O |
| `identity grant list` | `ListAccessGrants` / inspect subject grants | protected Identity client | `identity.grant.list`, exact subject | canonical grant list / C | 0/1/2 | U | yes | yes/partial: no process smoke | H,I,O |
| `identity grant issue` | `IssueAccessGrant` / issue finite exact actions | same | `identity.grant.issue`, exact subject | reviewed mutation/request ID / C | 0 RPC success; 1 denial/API; 2 usage | U | yes | yes/partial: no process smoke | H,I,O |
| `identity grant revoke` | `RevokeAccessGrant` / revoke exact grant | same | `identity.grant.revoke`, exact grant | reviewed mutation/request ID / C | same | U | yes | yes/partial: no process smoke | H,I,O |
| `identity delegation issue` | offline one-hop Delegation signing | `cli/identity` protected artifacts | delegator device Credential and current Node context; no RPC action | reviewed consent/protected output / C | 0/1/2 | U | context may be SSH but operation is local | yes/yes | H,I |
| `identity delegation revoke` | offline Delegation revocation signing | same | delegator device Credential; no RPC action | reviewed revocation/protected output / C | 0/1/2 | U | n/a | yes/yes | H,I |
| `identity delegation import-revocation` | public `ImportDelegationRevocation` / persist verified revocation | public Identity client | no Session grant; verified signed artifact bound to Node | import metadata / C | 0/1/2 | U | yes | yes/partial: no process smoke | H,I |
| `identity application-ticket issue` | protected `IssueApplicationEnrollmentTicket` / authorize initial actions | protected Identity client | `identity.principal.enroll`, exact prospective Principal | Principal/Node/actions/expiry/path, secret excluded / C | 0 file created; 1 API/write; 2 usage | U,E APP-001 | yes | yes/yes for issuance; handoff cleanup gap is in installation packet | H,I |
| `identity login` | public Begin/Complete / create bound Session | shared session manager | enrolled finite device Credential; no action grant for handshake | public cache-key metadata / C | 0 authenticated; 1 auth/transport; 2 usage | U + Unix integration | yes | yes/yes | H,I,O |
| `identity status` | local Session cache inspection | session manager | none | authenticated/not-authenticated public metadata / C | 0/1 output; 2 usage | U | yes within live shell/TUI; one-shot CLI normally starts empty | yes/partial: one-shot result can surprise | H,I |
| `identity logout` | `EndSession` for cached sessions, then zero local secret | session manager | presented bound Session | not-authenticated metadata / C | 0 best-effort server logout; 2 usage | U + Unix integration | yes | yes/partial: server EndSession failures are intentionally ignored | H,I,O |

### Interactive and local surfaces

| Command | Procedure / domain action | Client/module | Required grant | Human / JSON | Exit | Smoke | SSH | R/O | Docs |
|---|---|---|---|---|---|---|---|---|---|
| `shell` | reuse live client and dispatch ordinary commands | `cli/tui.Shell` | union of invoked command grants | interactive H; JSON rejected | 0 EOF/exit; child code; 1 I/O; 2 args | U | yes | yes/partial: help wrongly requires resolved Operator context and quoting is whitespace-only | H,R |
| `tui` | inspect five sections; Node start/stop actions | `cli/tui` -> same command context | grants for selected queries/mutations | interactive H only | 0 normal; 1 runtime; 2 args | U | yes | yes/partial: no integration/E2E | H,R |
| `version` | print build identity locally | root CLI/buildinfo | none | build identity / L | 0 or 1 output | U | n/a | yes/yes | H,R |

## Existing Deterministic Evidence

Executed on
`180decc1b03f94a6115b59a4046b4795308ec235`:

```text
go test ./internal/cli/... -count=1
```

Result: passed. Packages without command-level unit tests are explicitly
reported as `[no test files]`: `internal/cli/content`,
`internal/cli/network`, `internal/cli/node` and
`internal/cli/workload`.

The test slice verifies shared transport, SSH arguments/readiness/cleanup,
session refresh/logout, output failure handling, Identity custody and
administration, configuration resolution, watch behavior, TUI navigation and
version.

Executed:

```text
go run ./tests/tooling/testcatalog -tags "integration e2e" ./tests/...
```

Result: passed with 142 entries. It identifies:

- three terminal E2E procedures: Node lifecycle, Network lifecycle, and
  workload/data/diagnostic readiness;
- five CLI surface integration procedures: Network, Diagnostics, hosted
  services, transfer, and stale-source projection;
- one Application process E2E that also drives Operator Identity commands.

Safe local help smoke confirmed the group masking described above. No daemon,
Docker or external environment was used.

## Historical Evidence

`docs/engineering/evidence/stabilization-baseline-75471a6.md` records a passing
predecessor local/static baseline and explicitly records unavailable
Docker/Linux E2E. It is not substituted for current-head CLI process evidence.

The historical audit is not used as a command backlog. The current shared
session client and CLI/SDK parity remediation in
`docs/engineering/current-remediation-ledger.md` is background evidence only;
current source and tests determine this packet.

## Missing Or Unreachable Behavior

### Unreachable help claims

Detailed online group usage exists but is hidden by root dispatch. Nested
`resolve`, `records`, `objects`, `blobs`, `manifests` and `transfers` lack
complete reachable help and flag descriptions. `shell help` unnecessarily
requires a valid Node context.

### Commands without CLI smoke evidence

The following procedures have no CLI-level unit, integration or E2E invocation:

- Node runtime and features;
- Network presence, both resolve forms, records list/import;
- workload list/get/start/restart;
- Object and Manifest list/get/publish;
- Blob list/get/publish/retain/pin/drop;
- diagnostics snapshot;
- all online Identity administration at process level except bootstrap,
  Application ticket issuance and login used by APP-001;
- TUI with a real Operator surface.

Server handler, resource-target and domain tests exist for many of them. Those
tests prove implementation, not command reachability or output.

### JSON contract gaps

- protobuf JSON, custom Identity JSON and JSON Lines are three different
  families with no command-owned schema/version declaration;
- watch JSON interleaves response messages with CLI-owned retry notices;
- stream JSON is a sequence rather than a single document;
- shell/TUI do not support JSON but root help presents `--output json` as a
  global option without per-command capability;
- no validator asserts that every JSON command produces valid JSON on success
  and the shared error object on failure;
- raw protobuf additions are wire-compatible but can change automation-visible
  output because `EmitUnpopulated` emits every new field.

### Duplicate and divergent paths

- `node status`, `node runtime`, `diagnostics snapshot` and
  `diagnostics health` intentionally overlap but have no procedure guidance
  telling an Operator when each is authoritative.
- `workload service` and `workload publication` expose different lifecycle
  truth and are correctly separate, but documentation does not state that
  publication success is not workload readiness.
- shell/TUI dispatch the same command modules, which is desirable reuse, not a
  duplicate implementation.
- CLI uses the supported Operator Interface; public Go SDK uses the separate
  Application Interface. Their domain sets intentionally differ. There is no
  supported public Operator Go SDK, so parity must not be inferred.

### Misleading success outcomes

- Node/workload/content/config mutations use exit 0 for any successfully
  decoded RPC response, including `accepted=false`, `rejected_invalid`,
  `rejected_immutable`, `rolled_back`, `rollback_failed`, or an asynchronous
  response that did not reach the requested terminal state.
- `identity logout` reports not authenticated even if best-effort server
  `EndSession` failed; local secret disposal is real, but remote invalidation
  is not confirmed.
- `identity status` in a normal one-shot process reports only that invocation's
  empty cache, not durable Node authentication state.

None of these outcomes exposes an unprotected route. The risk is automation or
human interpretation, not authorization bypass.

## Actors, Assets And Trust Assumptions

### Actors

- Operator Principal with finite Node-specific Access Grants;
- CLI process as a presentation adapter, never a Principal;
- Node Principal and protected Operator Interface;
- OpenSSH client/server for remote stream-local transport;
- local OS account protecting signer and socket access.

### Assets

- root and device signer bundles;
- finite Key Credential and memory-only Operator Session;
- Access Grants, revocations and administrative request IDs;
- protected socket path and SSH host-key policy;
- workload specifications, signed discovery records and content metadata read
  from files;
- human/JSON output used for operational decisions and automation.

### Trust assumptions

- Operator pins the target Node Principal and protected transport;
- OpenSSH host-key verification and local socket permissions are correctly
  administered;
- server access catalogues remain closed and canonicalize resources before
  dispatch;
- CLI input files are bounded and strict protobuf/artifact inputs;
- stdout/stderr and exit status are part of the automation boundary and must
  not overstate mutation success.

## Proposed Module Boundary And External Interface

Keep the external command names, flags, Operator RPCs and access actions.

Make the supported CLI contract a closed, CLI-owned catalogue:

```go
type OutputShape string

const (
    OutputProtoJSON  OutputShape = "proto-json-v1"
    OutputCLIJSON    OutputShape = "cli-json-v1"
    OutputJSONLines  OutputShape = "json-lines-v1"
    OutputHumanOnly  OutputShape = "human-only"
)

type CommandSpec struct {
    Path          []string
    Procedure     string
    Action        string
    ResourceKind  string
    Mutating      bool
    Output        OutputShape
    SSH           bool
    EvidenceOwner string
}
```

The catalogue is production metadata, not a second dispatcher. It generates
reachable help and is checked against the actual command/RPC/action registry.
Offline commands use stable pseudo-procedures such as
`offline.identity.principal.create`; interactive surfaces declare
`human-only`.

Externally:

- `<group> help` and nested help enumerate the real leaf commands without
  resolving Node context;
- existing success JSON shapes remain compatible and are declared by family;
- JSON Lines are documented for events/watch;
- exit 0 means the requested synchronous mutation was accepted or the query
  completed; a stable rejected mutation returns exit 1 with its structured
  response still on stdout and a bounded error summary on stderr; usage stays
  exit 2.

Asynchronous acceptance is not terminal completion. The human/JSON body must
retain operation state and recovery instructions.

## Proposed Internal Seam

Add a small `internal/cli/catalog` package plus a tooling validator:

- each leaf command contributes exactly one `CommandSpec`;
- help reads only this closed registry;
- validator joins `Procedure` to
  `internal/localapi/auth.RuleForProcedure` or the Identity procedure
  catalogue and fails on action/mutation mismatch;
- validator joins `EvidenceOwner` to tagged testcatalog metadata;
- test harness executes help and safe parser/output fixtures for every spec;
- mutation renderers share a `RenderOutcome` helper that classifies accepted,
  rejected, restart-required and asynchronous states without moving domain
  policy into CLI.

The seam does not expose generated clients or internal domain types to users.
It also avoids turning the command catalogue into a new Operator SDK.

## Dependencies

### In-process

- root and domain CLI parsers/renderers;
- shared Operator client/session/transport;
- generated `ardents.v1` clients;
- server access and Identity procedure catalogues;
- testcatalog metadata and CLI tooling tests.

### Local-substitutable

- Unix socket transport;
- SSH helper process and stream-local socket;
- signer/input files;
- in-process test Operator server with deterministic protobuf responses.

### Remote But Owned

- Node daemon behind the protected Operator socket;
- OpenSSH endpoint used only to reach that owned socket.

### True External

- OpenSSH host-key and account administration;
- Docker/Linux/native runner for tagged process qualification;
- Docker runtime for real workload lifecycle smoke.

## Alternatives

### A. Keep Markdown as the command map

Rejected as the sole source. It can explain procedures but cannot fail when a
new command/RPC/action is added or omitted.

### B. Generate help by parsing flag sets at runtime

Plausible for syntax only, but insufficient. It cannot declare access action,
resource kind, output family, SSH support or evidence owner.

### C. Closed `CommandSpec` registry joined to server action catalogues

Recommended. It is small, keeps external commands unchanged and makes help,
authorization traceability and smoke ownership fail closed.

### D. Replace all JSON with one new CLI envelope

Rejected for R1. It would break existing automation and duplicate versioned
Operator protobuf fields. Preserve current families and document/test them.

### E. Keep exit 0 for every decoded response

Rejected for synchronous rejected mutations because README explicitly presents
JSON as an automation surface. Preserve response bodies, but make exit status
reflect acceptance. Asynchronous accepted operations remain exit 0 with
non-terminal state visible.

## Failure, Retry, Restart And Recovery Behavior

| Condition | Required CLI behavior |
|---|---|
| unknown command/flag or missing input | reachable help on stderr, exit 2, no client/domain call |
| output writer failure | exit 1 even if the domain call succeeded; never silently claim success |
| authentication failure | one session refresh only after `Unauthenticated`; second failure exits 1 |
| permission denial | no login fallback or mutation retry; structured denial, exit 1 |
| ambiguous administrative mutation transport failure | Identity administration keeps existing one retry with same request ID; exposes request ID for reconciliation |
| ordinary domain mutation transport failure | no automatic replay unless operation contract already owns idempotency; exit 1 |
| synchronous response rejects mutation | render full response, explicit human rejection/recovery, exit 1 |
| asynchronous mutation accepted | render operation ID/status/recovery, exit 0; Operator follows status/pending procedure |
| watch transient failure | bounded retry notices in JSON Lines/human; exhausted budget exits 1 |
| stream ends cleanly or reaches limit | exit 0; stream failure exits 1 |
| SSH helper fails/exits | clean local socket/process, redact command material, exit 1 |
| daemon restarts | Session refresh once; status/recovery commands re-read durable truth |
| shell child command fails | shell returns child nonzero rather than swallowing it |
| logout server call fails | local secret is still zeroed; output must say server invalidation was unconfirmed instead of unconditional success |

## Security, Privacy And Abuse Analysis

- Catalogue metadata contains no secrets, identifiers or dynamic resources.
- Help must be reachable without loading signers, opening sockets or resolving
  a Node.
- Stable errors must never print Credentials, Session secrets, ticket content,
  signer paths or sensitive input-file content.
- Exact action/resource joins prevent documentation from suggesting a broader
  grant than the server requires.
- SSH support means stream-local forwarding only; a catalogue entry cannot
  enable a new transport.
- JSON output must preserve secret redaction from protobuf/CLI projections.
- Watch/event labels and catalogue identifiers remain bounded; dynamic
  Principal/resource values are not metric labels.
- A smoke may use a local substitute only if it still passes through the real
  CLI parser, shared client, generated Operator RPC and server admission seam.

## Observability And Operator Actions

The command catalogue should expose a generated human reference and a
machine-readable test projection containing command ID, procedure, action,
resource kind, output family and evidence owner. It must not expose live grants
or secrets.

Operators should be directed to:

- `node status` for concise lifecycle/readiness;
- `node runtime` for boot/Identity/composite runtime detail;
- `diagnostics health` for current degradation causes;
- `diagnostics pending` for incomplete operations and recovery;
- `diagnostics explain` for one exact subject;
- `workload service` for backing/readiness and
  `workload publication` for advertised/withdrawn truth;
- `data transfers get` for asynchronous fetch progress.

Command failure metrics use stable command ID, output mode and outcome class
only. They do not include argv, file paths, Principal IDs or resource IDs.

## Acceptance Matrix

| Level | Required evidence |
|---|---|
| Catalogue unit | exactly 68 current leaf specs; unique path and ID; known output/transport/evidence values; empty catalogue fails |
| Action join | every protected spec maps to exactly one generated RPC and exact server action/mutation class; unknown/sibling action fails |
| Help contract | root, group and nested help enumerate the same specs without context, signer, socket or network; no detailed usage remains unreachable |
| Parser contract | every spec accepts its minimal safe fixture or returns documented missing-input exit 2; unknown leaf fails closed |
| JSON success | every JSON-capable command produces one valid declared-family document/stream with deterministic field naming; human-only commands reject JSON before connection |
| JSON failure | all API failures produce the common error object on stderr and exit 1; no success body is fabricated |
| Outcome/exit | synchronous rejected mutations return 1 with full response; accepted async responses return 0 and expose operation truth; usage is 2 |
| SSH | each online spec is exercised through the shared stream-local transport contract; offline specs never open a tunnel |
| Node/Network/Diagnostics process slice | lifecycle, runtime, features, events, presence, records/resolve and diagnostic procedures use real CLI and Operator RPC |
| Workload process slice | register/list/get/start/stop/restart plus service/publication truth and rejected outcome use real CLI; Docker-dependent rows are tagged for canonical runner |
| Content/Transfer process slice | Object/Blob/Manifest lifecycle, retention and transfer progress use real CLI and exact grants |
| Identity process slice | later enrollment, grant list/issue/revoke, device revoke, Delegation import, login/status/logout use real CLI with redacted output |
| Qualification | all required Linux/Docker/native/release gates pass on one exact clean commit; local unit/integration success never sets `Q=yes` |

## Open Questions

No open question changes the selected external command names, RPC procedures,
actions or JSON families.

The exact mapping from each domain response to accepted/rejected/asynchronous
is an implementation-owned closed table and must be reviewed against existing
protobuf semantics. If any response lacks enough information to classify the
outcome without a new field, that individual command is an R2 candidate and
must not guess success.

## Recommendation

Implement the closed command contract and reachable help first. Then add four
vertical smoke slices by Operator procedure, not by command or software layer.
Keep existing JSON payload compatibility and make rejection explicit in exit
semantics only where the current response is unambiguous.

Do not add a public Operator SDK, change wire messages, conflate Application
SDK parity, or claim release qualification.

## Vertically Sliced Issues And Dependency Order

### OCS-01 - Publish a fail-closed Operator command contract

- Parent: R1 Existing Product Truth / Operator command smoke
- User story: As an Operator, I can discover every supported command and know
  its action, transport, output family and smoke owner without connecting to a
  Node.
- What to build: closed 68-leaf `CommandSpec` registry, reachable root/group/
  nested help, action/procedure/evidence validator, generated reference
  projection and parser/JSON family contract tests.
- Acceptance criteria: catalogue/action/help/parser/JSON rows above pass;
  existing command names and successful JSON payloads remain compatible;
  `shell help` works offline.
- Blocked by: none.
- Research class: R1 resolved to implementation.
- Proposed status: `ready-for-agent`.

### OCS-02 - Prove Node, Network and Diagnostics operator procedures

- Parent: R1 Existing Product Truth / Operator command smoke
- User story: As an Operator, I can inspect and recover Node/network state
  through one documented terminal procedure with trustworthy exit outcomes.
- What to build: complete current terminal tracer bullet for Node runtime/
  features/events, Network presence/records/resolve, diagnostic
  snapshot/health/pending/explain/events, human/JSON assertions and
  accepted/rejected outcome classification.
- Acceptance criteria: corresponding process rows above pass through CLI,
  generated RPC and exact admission; no direct domain shortcut; restart truth
  remains visible.
- Blocked by: OCS-01.
- Research class: R1 resolved to implementation.
- Proposed status: `ready-for-agent`.

### OCS-03 - Prove workload and hosted-service operator procedures

- Parent: R1 Existing Product Truth / Operator command smoke
- User story: As an Operator, I can register, control and diagnose a workload
  and distinguish runtime readiness from service publication.
- What to build: one vertical workload lifecycle smoke covering list/get/
  register/start/stop/restart/service/publication, human/JSON output, exact
  grants and a non-accepted mutation.
- Acceptance criteria: real CLI and Operator API are used; publication and
  readiness remain distinct; rejected outcome is nonzero; Docker dependency is
  explicitly tagged and not simulated.
- Blocked by: OCS-01.
- Research class: R1 resolved to implementation with Docker evidence in R3.
- Proposed status: `ready-for-agent`.

### OCS-04 - Prove content retention and transfer operator procedures

- Parent: R1 Existing Product Truth / Operator command smoke
- User story: As an Operator, I can publish, inspect, retain and transfer
  content while automation receives stable output and progress truth.
- What to build: one procedure smoke covering Object/Blob/Manifest list/get/
  publish, Blob retain/pin/drop/fetch/sources, inventory and transfer list/get.
- Acceptance criteria: exact owner/resource grants are exercised; file inputs,
  human/JSON and failure exits are asserted; asynchronous fetch success is not
  presented as completed transfer.
- Blocked by: OCS-01.
- Research class: R1 resolved to implementation; multi-node qualification
  remains R3.
- Proposed status: `ready-for-agent`.

### OCS-05 - Prove Principal access administration as an Operator procedure

- Parent: R1 Existing Product Truth / Operator command smoke
- User story: As an Operator, I can enroll and revoke Principals, Credentials,
  grants and Delegations with redacted, reconcilable outcomes.
- What to build: process smoke for later Principal enrollment, grant list/
  issue/revoke, device revoke, Delegation revocation import, login/status/
  logout and their safe failure/retry behavior; keep offline custody unit
  evidence.
- Acceptance criteria: exact Identity actions and request IDs are asserted;
  SSH path uses stream-local forwarding; secret/path redaction holds; logout
  distinguishes local cleanup from unconfirmed server invalidation.
- Blocked by: OCS-01; may reuse AIJ-02 fixtures but is independently
  implementable.
- Research class: R1 resolved to implementation.
- Proposed status: `ready-for-agent`.

Dependency order:

```text
OCS-01 command contract/help
  +--> OCS-02 Node/Network/Diagnostics
  +--> OCS-03 Workload/Hosted service
  +--> OCS-04 Content/Transfer
  +--> OCS-05 Principal access

All completed slices -> existing R3 qualification program
```
