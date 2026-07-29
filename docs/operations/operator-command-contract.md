# Operator Command Contract

Status: OCS-01 implementation contract. This is not release qualification or a
production-readiness claim.

## Authoritative Metadata

`internal/cli/catalog` is the single production-owned metadata catalogue for
the Operator CLI. It contains exactly 87 leaf commands. The catalogue describes
stable command ID and path, complete help syntax, exact Operator procedure
requirements with their actions, resource kinds and mutation classes, output
family, SSH stream-local support and evidence owner. It does not dispatch
commands; the existing domain command packages continue to own parsing and
execution.

Protected procedure values are generated Connect procedure constants. Contract
tests join every protected entry to the exact server-owned rules in
`internal/localapi/auth` or `internal/localapi/identity`. The same tests reject
unknown procedures, sibling actions and resource/mutation mismatches. Parser
contract tests probe every catalogue entry through its production parser.
Runtime dispatch rejects a command path which is not present in the closed
catalogue before resolving a Principal context or constructing a client.
Protected aggregate commands declare every constituent procedure separately;
`topology status` therefore carries the exact runtime, Network status, Node
features and bounded session-termination requirements instead of masquerading
as a local-only command. The first three are the complete protected product
observation budget; `EndSession` is a session-lifecycle cleanup call.
`topology recover` separately declares authenticated Node clock observations,
Authority inspection, conditional exact restore verification and bounded
session termination. It never presents those calls as repository
administration.

Offline Principal/device/Delegation custody commands use stable
`offline.identity.*` procedure identifiers and declare neither an RPC/action
nor SSH. Local and interactive surfaces use `local.*` and `interactive.*`
identifiers. SSH support means only the existing OpenSSH stream-local forward
to the protected Operator Unix socket; it does not describe TCP or a general
remote transport.

## Help Contract

Root, group and nested help are projections of the closed catalogue. Help is
selected before context resolution, signer loading and client construction, so
the following forms require no Node, socket, SSH endpoint or network:

```text
ardentsctl --help
ardentsctl node help
ardentsctl network help
ardentsctl network resolve help
ardentsctl network records help
ardentsctl data help
ardentsctl data objects help
ardentsctl data blobs help
ardentsctl data manifests help
ardentsctl data transfers help
ardentsctl identity help
ardentsctl identity principal help
ardentsctl identity device help
ardentsctl identity grant help
ardentsctl identity delegation help
ardentsctl identity application-ticket help
ardentsctl shell help
ardentsctl tui help
```

Unknown help prefixes fail with usage exit code 2. Group and nested output
lists full leaf syntax, including required positional arguments and flags.

## Output Families

The catalogue freezes the existing successful payload families; it introduces
no envelope:

| Family | Identifier | Contract |
|---|---|---|
| protobuf JSON | `proto-json-v1` | deterministic `protojson` with `EmitUnpopulated`; existing protobuf field names and payload remain authoritative |
| CLI-owned JSON | `cli-json-v1` | existing `encoding/json` projections for Identity and local build identity |
| JSON Lines | `json-lines-v1` | one JSON document per Node event or watch update/notice |
| interactive human | `human-only` | shell and TUI reject JSON before context, signer or transport work |

Network status, diagnostics health and transfer list/get declare
`json-lines-v1` as their conditional watch output while retaining
`proto-json-v1` for one-shot JSON. API failures continue to use the common JSON
error object on stderr and exit 1.

## Fail-Closed Checks

The catalogue validator and contract tests reject an empty catalogue,
duplicate IDs or paths, missing/unreachable leaves, unknown output/access
values, missing evidence/help metadata, invalid offline transport claims,
incomplete protected metadata, procedures absent from the generated Operator
surface, and server action/resource/mutation divergence. Human-only commands
cannot be classified as protected JSON-capable commands.

## OCS-02–OCS-05 Handoff

OCS-01 provides metadata and contract gates only. The following procedure-level
smoke slices remain separate:

| Slice | Catalogue IDs now available | Count | Remaining evidence |
|---|---|---:|---|
| OCS-02 | `node.*`, `network.*`, `diagnostics.*` | 20 | implemented by the tagged terminal Node/restart, Network/Discovery and exact-admission scenarios; release qualification remains separate |
| OCS-03 | `workload.*` | 9 | implemented by the tagged terminal workload lifecycle with local process execution; Docker qualification remains R3 |
| OCS-04 | `data.*` | 17 | implemented by the tagged terminal content/retention/private-transfer lifecycle; release multi-node qualification remains R3 |
| OCS-05 | `identity.device.revoke`, `identity.enroll`, `identity.grant.*`, `identity.delegation.import-revocation`, `identity.application-ticket.issue`, `identity.login`, `identity.status`, `identity.logout` | 10 | implemented by the tagged Principal administration/session procedure plus focused retry/logout contracts |

The remaining 12 entries are owned by OCS-01 contract evidence: two
configuration commands, seven offline custody commands, shell, TUI and version.
OCS-03–OCS-05 process smoke remains separate.

## OCS-02 Procedure Evidence

The `ocs-02` tagged terminal scenarios exercise all 20 Node, Network and
Diagnostics catalogue entries through the real CLI parser, Principal session,
generated Operator client and protected server admission:

- Node lifecycle covers start, stop, status, runtime, features and JSON Lines
  events, including restart-retained pending recovery truth.
- Network/Discovery covers status, discovery, presence, peers, routes, records
  list/import, both record kinds and service resolution.
- Diagnostics covers snapshot, health, pending, explain and recent events.
- A restricted grant fixture admits `network.status` and rejects its
  `network.discovery` sibling with the structured permission error.

Human and protobuf JSON projections are asserted in the procedure. A valid,
older signed discovery record returns its full rejected response while the CLI
returns exit 1 in both modes. The shared Session interceptor test separately
proves that an `Unauthenticated` response refreshes exactly once and that
`PermissionDenied` is not retried. The restart procedure re-reads durable
pending truth with a newly constructed one-shot CLI client; Session-cache
continuity belongs to persistent shell/TUI clients rather than separate CLI
processes.

The Operator test socket remains Unix stream-local transport. Its disposable
path is allocated directly under the system temporary directory so it fits the
platform `sockaddr_un` limit on Windows as well as Linux, and cleanup removes it
after each fixture. This evidence is an implementation smoke, not release
qualification or a production-readiness claim.

## OCS-03 Procedure Evidence

The `ocs-03` tagged terminal scenario exercises all nine workload catalogue
entries with catalogue-derived exact grants through CLI parsing, the generated
Operator client and protected admission. It registers and inventories
workloads, starts/stops/restarts the local process executor, and queries the
hosted-service and publication projections in human and protobuf JSON modes.

Hosted-service readiness and network publication are asserted independently:
the same running workload exposes a ready `LocalOnly` service whose publication
remains false, alongside a ready and published `NetworkPublished` service.
Both distinct projections are checked in human and protobuf JSON modes.

The current workload server maps domain mutation failures to structured
Operator API errors rather than returning `accepted=false`. The process smoke
therefore proves a missing-workload failure exits 1 without fabricated stdout,
while the workload renderer contract test injects the generated
`WorkloadCommandResponse` rejection shape and proves its complete human/JSON
response is preserved with exit 1. This records both supported failure forms
without claiming that the server currently emits a synchronous rejected
response.

The scenario is tagged `environment=local` and uses the repository's real local
process executor. It does not simulate Docker and does not qualify Docker
execution. Docker-dependent workload evidence remains an R3 runner concern.

## OCS-04 Procedure Evidence

The `ocs-04` tagged terminal scenario exercises all 17 `data.*` catalogue
entries with catalogue-derived actions through CLI parsing, generated Content,
Retention and Transfer clients, and protected server admission. Data authority
uses signed exact-resource grants for the owner-bound Object/Manifest/Blob and
the ownerless transfer/collection/inventory tuples resolved at admission.

File-backed protobuf JSON inputs publish a Blob payload, an owned Object and an
owned Manifest. Their list/get procedures, inventory, retain/pin/drop
transitions and an ungranted sibling-Object denial are asserted in human and
protobuf JSON modes. Exact grants bind the authenticated Principal to each
owner-required content tuple; the server assigns the same authenticated owner
to the Object and Manifest inputs, which omit owner.

Two real local Node runtimes exercise private Blob exchange. `data blobs fetch`
reports the fetched Blob's `available-local` state and does not label that
response as a completed transfer. Terminal completion/state truth is read
separately from `data transfers list/get`, preserving the existing wire
payloads and directing automation to the transfer procedures. Because the
current `FetchBlob` RPC blocks until the private exchange completes, this
scenario proves terminal transfer state rather than claiming observation of an
in-flight progress transition.

This is local multi-runtime evidence inside one tagged test process, not
release qualification.
Canonical cross-runner/network qualification remains R3; no production
readiness claim follows from the local pass.

## OCS-05 Procedure Evidence

The `ocs-05` tagged terminal scenario exercises the ten Identity procedure IDs
listed above through the real root parser, shared Principal client, generated
Identity RPCs and server-owned admission catalogue. Later enrollment, grant
list/issue/revoke, device revoke, Application ticket issuance and an invalid
administrative target use catalogue-derived exact actions. Every
administrative mutation that accepts an idempotency key asserts its explicit
request ID in the success or structured failure output.

Protected signer paths and artifact bytes are absent from enrollment,
Delegation and import output. Application ticket and Delegation material is
written only to the requested protected output files; the intentionally public
`protected_output` handoff path remains visible while the file content does
not. The existing focused Identity retry matrix proves one replay only for an
ambiguous transport failure with the same protobuf request/request ID, and no
replay for permission denial or local failure.

Grant revocation is reconciled through the subsequent protected grant list, and
the revoked device is rejected by a new public authentication exchange. The
tagged process proves Delegation revocation import and its stable revocation ID;
`TestDelegationRevocationIsIdempotentPermanentAcrossRestartAndConcurrentAdmit`
in the Identity access package separately proves that the imported artifact
immediately and durably rejects delegated admission.

One-shot `identity login/status/logout` preserves the existing CLI-owned JSON
payloads. A tagged shell invocation uses one live client to prove authenticated
status, confirmed logout and process-local cleanup without exposing Session
secrets. If `EndSession` fails, the Session manager still zeroes and removes
local secrets; logout now returns exit 1 with the common error object stating
that local cleanup completed while server invalidation is unconfirmed.

All online Identity entries continue to use the shared Operator transport.
The transport contract tests assert OpenSSH stream-local forwarding to the
remote Operator Unix socket, readiness/cleanup and redacted failures; no
general TCP/HTTP remote transport is introduced. This local tagged scenario
does not claim an external SSH-host qualification run.
