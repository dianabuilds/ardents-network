# Package map

This map registers Go packages, including domain implementation, shared
mechanisms, platform adapters, composition, and engineering checks.
[The domain map](domain-map.md) owns semantic responsibility.

## Package classes

Every registered package records its class from
[ADR-0133](../adr/0133-group-product-domains-under-internal-domain.md) as the
first sentence of its description (`Class: <class>.`). The classes are:
**domain** — a product domain that owns the rules and lifecycle of one Ardents
concept (the `internal/domain/<name>` migration target under ADR-0133);
**supporting mechanism** — a shared technical leaf that owns no product
authority, root or schema of its own; **platform adapter** — a bounded
host-operation port at a platform boundary, including
`internal/platform/processguard`; **application contract** — an application-layer
contract implementation under `internal/application`; **engineering check** —
architecture gates, verification harnesses and test infrastructure;
**composition** — an executable adapter or a dedicated command package that wires owners for that executable.
Classes are readability inventory only: they add no import permissions and
change no gate semantics.

## File-specific project imports

The package tables register project imports at package granularity.
The successor and Route migration isolation checks further restrict these
permissions. For the edges below, the architecture test checks every existing
Go file, including tests and files for other platforms, against the combined
rules. This table documents permissions even when an allowed file currently
has no import. Existing isolation checks remain authoritative.

Some `ardents-next` dependencies have command-wide permissions subject to the
existing exceptions. The table records the narrower file-specific command
edges. For example, `endpoint/funding` uses the command-wide permission, while
`node/command` has one allowed file.

| Importing package | Imported package | Allowed Go files |
| --- | --- | --- |
| internal/application/administration | internal/application/internal/socketfile | `server.go`, `server_retirement_test.go` |
| internal/application/connection | internal/application/internal/socketfile | `server_linux.go` |
| internal/diagnostics/command | internal/diagnostics/logstore | `monitor.go`, `monitor_test.go` |
| cmd/ardents-next | internal/domain/admission/console | `admission_console.go` |
| cmd/ardents-next | internal/domain/admission/operator | `admission.go`, `command.go`, `issuance.go` |
| cmd/ardents-next | internal/application/administration | `document_command.go`, `endpoint_administration_client.go` |
| cmd/ardents-next | internal/application/connection | `document_command.go`, `stream_command.go` |
| cmd/ardents-next | internal/application/document | `document_command.go` |
| cmd/ardents-next | internal/diagnostics/telemetry | `hosting.go`, `main.go` |
| cmd/ardents-next | internal/diagnostics/timeline | `diagnostic_timeline.go` |
| cmd/ardents-next | internal/domain/endpoint/command | `command.go` |
| cmd/ardents-next | internal/domain/enrollment | `enrollment.go` |
| cmd/ardents-next | internal/domain/execution/runtime | `admission_holder.go`, `execution_holder.go` |
| cmd/ardents-next | internal/domain/network/closedprofile | `offline_profile_inspection.go` |
| cmd/ardents-next | internal/domain/network/source/command | `source_command.go` |
| cmd/ardents-next | internal/domain/network/state/command | `network.go` |
| cmd/ardents-next | internal/domain/node/receive | `command.go` |
| cmd/ardents-next | internal/domain/node/command | `node.go` |
| cmd/ardents-next | internal/domain/node/provisioning | `network.go` |
| cmd/ardents-next | internal/domain/node/provisioning/infrastructure | `infrastructure_operator.go` |
| cmd/ardents-next | internal/domain/node/provisioning/runtimeplan | `node.go` |
| cmd/ardents-next | internal/platform/artifacts | `admission_console.go`, `inputs.go`, `publication.go` |
| cmd/ardents-next | internal/platform/launch | `command.go` |
| cmd/ardents-next | internal/domain/publication/durable | `publication.go` |
| cmd/ardents-next | internal/domain/publication/instance | `publication.go` |
| cmd/ardents-next | internal/domain/release/endpoint | `release.go` |
| cmd/ardents-next | internal/domain/release/infrastructure | `infrastructure_operator.go` |
| cmd/ardents-next | internal/domain/release/infrastructure/launch | `node.go`, `source_command.go` |
| cmd/ardents-next | internal/domain/release/infrastructure/signing | `infrastructure_operator.go` |
| internal/domain/release/infrastructure/signing | internal/domain/release/infrastructure/signing/internal/keyset | `release_key_command_linux.go`, `release_key_receipt_unix.go`, `release_key_receipt_unix_test.go` |
| internal/domain/release/infrastructure/signing/internal/keyset | internal/platform/artifacts | `prepare_linux.go` |
| internal/domain/admission/issuance | internal/platform/artifacts | `artifact_export_unix.go`, `result_declaration.go` |
| internal/domain/connection | internal/application/connection | `stream.go`, `stream_retirement.go`, `stream_test.go` |
| internal/domain/connection | internal/domain/route/transport | `initiator_authentication.go` |
| internal/domain/custody/command | internal/domain/admission | `admission_permission_client_linux_test.go` |
| internal/domain/custody/command | internal/domain/custody | `admission_authority.go`, `admission_permission_client_linux_test.go`, `command.go`, `command_test.go`, `purge_record.go`, `recovery_bundle.go`, `service_authority.go`, `terminal_secret.go`, `verify_record.go` |
| internal/domain/custody/command | internal/domain/publication | `command_test.go` |
| internal/domain/endpoint | internal/domain/network/duty | `history_preparation_linux.go`, `history_preparation_linux_test.go` |
| internal/domain/endpoint | internal/domain/network/source/servingdeclaration | `source_trust_preparation_linux.go` |
| internal/domain/endpoint | internal/platform/artifacts | `source_trust_native_linux_test.go`, `source_trust_preparation_linux.go` |
| internal/domain/endpoint | internal/platform/processguard | `source_trust_preparation_linux.go` |
| internal/domain/endpoint | internal/domain/route/selection | `connection_installed_linux_test.go`, `document_command_installed_linux_test.go`, `history_preparation_linux.go`, `history_preparation_linux_test.go` |
| internal/domain/endpoint/funding | internal/domain/admission/console | `receiver_conversation.go` |
| internal/domain/endpoint/session | internal/domain/admission/console | `local_conversation.go` |
| internal/domain/endpoint/funding | tests/epochfixture/network | `intent_network_test.go` |
| internal/domain/endpoint/funding | tests/networkfixture | `intent_network_test.go` |
| internal/domain/endpoint/launch | internal/platform/artifacts | `platform_delivery_linux.go`, `platform_pin_linux_test.go`, `platform_staging_linux.go` |
| internal/domain/endpoint/plan | internal/pathcheck | `platform_launch.go` |
| internal/domain/endpoint/publisher | internal/application/connection | `byte_stream_unix.go` |
| internal/domain/endpoint/unit | internal/pathcheck | `platform_rendering.go` |
| internal/domain/execution/runtime | internal/application/connection | `hostile_snapshot_installed_linux_test.go`, `document_publisher_handoff.go`, `document_publisher_linux.go`, `document_reader_handoff.go`, `document_reader_linux.go`, `stream_contract.go`, `stream_contract_test.go` |
| internal/domain/execution/runtime | internal/application/streamqualification | `stream_qualification_installed_linux_test.go`, `stream_qualification_linux.go`, `stream_qualification_other.go` |
| internal/domain/execution/worker | internal/application/streamqualification | `stream_activation_installed_linux_test.go` |
| internal/domain/network/closedprofile | internal/domain/admission/issuerprofile | `profile.go` |
| internal/domain/network/duty | internal/domain/network | `participation_policy.go` |
| internal/domain/network/epoch | internal/domain/network | `assignment.go`, `candidate_evaluation.go` |
| internal/domain/network/state | internal/domain/network | `acquisition_history.go`, `clock_observation.go`, `closed_profile_accept.go`, `closed_profile_role_join_test.go`, `closed_runtime_view_test.go`, `epoch_history.go`, `membership.go`, `runtime_observation.go`, `runtime_witness.go` |
| internal/domain/network/state | internal/platform/cgroup | `descriptor_repair_linux.go`, `forwarding_repair_linux.go`, `introduction_repair_linux.go`, `join_repair_linux.go`, `source_repair_linux.go` |
| internal/domain/network/state | internal/platform/process | `source_repair_linux.go`, `source_state_files_linux.go` |
| internal/domain/network/state | internal/platform/systemd | `descriptor_repair_linux.go`, `forwarding_repair_linux.go`, `introduction_repair_linux.go`, `join_repair_linux.go`, `node_duty_repair_linux_test.go`, `source_repair_linux.go`, `source_repair_linux_test.go` |
| internal/domain/network/state | internal/platform/systemd/observation | `descriptor_repair_linux.go`, `forwarding_repair_linux.go`, `introduction_repair_linux.go`, `join_repair_linux.go`, `node_duty_repair_linux.go`, `node_duty_repair_linux_test.go`, `source_repair_linux.go`, `source_repair_linux_test.go` |
| internal/domain/network/state | internal/domain/reachability/store | `descriptor_repair_linux.go` |
| internal/domain/node/receive | internal/domain/publication | `publication_registration_linux_test.go` |
| internal/domain/node/receive | internal/domain/publication/durable | `publication_registration_linux_test.go` |
| internal/domain/node/receive | internal/domain/publication/instance | `publication_registration_linux_test.go` |
| internal/domain/publication/instance | internal/domain/route/transport | `service_attachment.go` |
| internal/domain/publication/runtime | internal/domain/connection/wire | `publisher_streams.go` |
| internal/domain/release/endpoint | tests/releasefixture | `authorization_integration_test.go`, `execution_artifact_test.go` |
| internal/domain/release/infrastructure | internal/platform/artifacts | `command_linux.go`, `preparation_linux.go`, `preparation_native_linux_test.go` |
| internal/domain/route/introduction | internal/domain/admission/receiving | `receiving_history_other.go`, `receiving_history_unix.go` |
| internal/domain/route/prefix | internal/domain/network/state | `prefix.go` |
| internal/domain/route/prefix | internal/domain/reachability | `descriptor_exchange.go` |
| internal/domain/route/receiver | internal/domain/reachability | `descriptor_unix.go` |
| internal/domain/route/receiver | internal/domain/reachability/store | `receiver_unix.go` |

## Installed role declaration and protection policy

R01 now has ZERO canonical installed exceptions. The transitional Source,
Forwarding, JOIN and Descriptor orchestration packages and their canonical
`cmd/ardents-node` installed consumers are all retired (#604 FJD stage D,
ADR-0125); the platform-led lifecycles own installed delivery through
`cmd/ardents-next`. New Route belongs only in `internal/domain/route`,
composed only by `cmd/ardents-next`; no old-command role consumer or five-import
exception remains. See the [Route migration contract](route-migration-contract.md).

The transitional `internal/platform/installation/descriptor` package (closed
request/plan preparation with five independent semantic roots, initial/start/
stop/replacement/recovery/transition orchestration and the OLD fixed unit
policy) is retired (#604 FJD stage D, ADR-0125). The platform-led Descriptor
lifecycle owns installation and runtime admission through the canonical platform
unit, `ardents-next node descriptor --retained-config` and `descriptor
repair-retained`; the retained repair mechanisms live in `platform/systemd`
(DescriptorRuntime), `platform/cgroup` (CheckDescriptorRecoveryQuiescent),
`network/state` and `reachability` (RepairRetainedStoreStage). Descriptor Release
signing is preserved and bound to `cmd/ardents-next` plus
`infrastructure.PlatformUnitTemplate`. The retired `ardents-node descriptor ...`
selectors are refusal-by-absence (the `ardents-node` binary is removed); the
transitional Control `prepare-template`/`prepare-request`/`prepare-plan`/
`prepare-hosting`/`prepare-state` routes were removed.
`ardents-next platform template` emits the canonical platform unit template.
Platform-led operator preparation is realized by the node domain's pure
runtime-plan declaration codec (`internal/domain/node/plan/declaration.go`)
and `ardents-next node prepare-plan` (#673, ADR-0125); the shared byte grammar
supplies no universal role policy.

The transitional `internal/platform/installation/join` package (closed
request/plan preparation, initial/start/stop/replacement/recovery/transition
orchestration and the OLD fixed unit policy) is retired (#604 FJD stage D,
ADR-0125). The platform-led JOIN lifecycle owns installation and runtime
admission through the canonical platform unit, `ardents-next node join
--retained-config` and `join repair-retained`; the retained repair mechanisms
live in `platform/systemd` (JoinRuntime), `platform/cgroup`
(CheckJoinRecoveryQuiescent) and `network/state`. JOIN Release signing is
preserved and bound to `cmd/ardents-next` plus
`infrastructure.PlatformUnitTemplate`. The retired `ardents-node join ...`
selectors are refusal-by-absence (the `ardents-node` binary is removed); the
transitional Control `prepare-template`/`prepare-request`/`prepare-plan`/
`prepare-hosting`/`prepare-state` routes were removed.
`ardents-next platform template` emits the canonical platform unit template. Platform-led operator preparation is realized
by the node domain's pure
runtime-plan declaration codec (`internal/domain/node/plan/declaration.go`)
and `ardents-next node prepare-plan` (#673, ADR-0125).
Each role's semantic authority and original resource completion remain with its
existing owner; shared byte preparation does not supply a universal role policy.

Forwarding, JOIN and Descriptor share one fixed initial/forward Release
composition. The operator selects a role at dispatch. That role supplies its
exact plan schema, program/generation target pair, native resource filenames
and candidate verifier. Each operation retains its own reviewed plan, five-key
directory and original signed predecessor. The Source Timestamp purpose
belongs to the separate Source composition.

- `internal/domain/release/infrastructure/signing/node_release_arguments_linux.go` parses public options,
  checks the selected plan and retains its exclusive output. It uses the
  standard library.
- `internal/domain/release/infrastructure/signing/node_release_plan_linux.go` binds the fixed plan and the
  exact resources. It imports `internal/domain/release/infrastructure` and
  `internal/platform/systemd` for the static program and canonical unit
  template contract.
- `internal/domain/release/infrastructure/signing/node_release_linux.go` owns the joined, finite signing
  operation. It imports `internal/domain/release/infrastructure`, `internal/domain/release`,
  `github.com/sigstore/sigstore/pkg/signature` and
  `github.com/theupdateframework/go-tuf/v2/metadata`.
- `internal/domain/release/infrastructure/signing/node_release_predecessor_linux.go` retains the exact
  protected public files and independently frozen candidate, checks the
  original five-key/three-threshold Root and verifies the role's coherent
  signed predecessor before forward signing. It imports `internal/domain/enrollment`,
  `internal/domain/release/infrastructure`, `internal/domain/release` and
  `github.com/theupdateframework/go-tuf/v2/metadata`.

The three existing initial/forward native role tests independently exercise
their fixed purposes. The shared key-birth mechanism creates five distinct
fresh files in each ceremony's own output. The platform-led lifecycle owns
installed admission.

Forwarding, JOIN and Descriptor create each individual provisioned Node or
State key through the command-local `provisioning_key_linux.go`. The fixed
role commands select their respective Node/State file, output directory and
public receipt. Record, Epoch, profile and Release signing retain their
purpose-specific inputs and checks.

`release_key_command_linux.go` selects the fixed Source, Forwarding, JOIN or
Descriptor operation and its refusal. The private `signing/internal/keyset`
owner publishes an original five-file set through Platform Artifacts and joins
that custody before returning the exact public inventory. The command renders
it through `release_key_receipt_unix.go`. All four roles use the same typed
wrong-argument refusal and preserve their existing preparation diagnostics.

Provisioning's `networksigning/certificate_unix.go` owns the exact Node transport
leaf profile and its strict verifier for Forwarding, JOIN and Descriptor
(#718, audit E1-4). Each role command supplies its own prepared Record facts
and freshly born Node key; the shared codec is pure X.509 preparation and
self-verification, not Network membership, State trust or process admission.

The canonical lowercase hex-32 shape is the shared `internal/canonicaldigest`
leaf (Phase 4, #835); the command-local `public_identifier_unix.go` adds the
offline preparations' non-zero public identifier rule, and the portable
closed-profile reader keeps its own error text and zero-value policy (#718,
audit E1-4). Publication, Issuer identity and offline Network command inputs
and Admission Issuance declarations also use the shared parser. Role plans
keep their own schemas, purposes and validators.

The development Route command and the original Forwarding Node use
`internal/domain/node.RefillForwarding` for Admission and Hosting refill composition.
The command retains its own listener and reservation lifetime.

The command-local `infrastructure_inventory_linux.go` reads the exact seven
public files for Source, Forwarding, JOIN and Descriptor. It retains the
original directory and files through output and rechecks the names alongside
protected-input verification. Each role supplies its fixed filenames and
keeps its own resource validation and Enrollment projection.

The former `cmd/ardents-next/forwarding_installed_native_linux_test.go` OLD-path
installed QA test and its `forwarding_bootstrap_receipt_test.go` public receipt
reader retired with the transitional Forwarding orchestration (#604 FJD stage D,
ADR-0125). The platform-led Forwarding lifecycle owns installed qualification;
see `forwarding_platform_native_linux_test.go` and the retained repair mechanism.

| Package | Declaration | Responsibility and genuine consumer | Exact permitted imports |
|---|---|---|---|

The transitional `internal/platform/installation/forwarding` package (closed
request/plan preparation, initial/start/stop/replacement/recovery/transition
orchestration and the OLD fixed four-write-root unit policy) is retired (#604
FJD stage D, ADR-0125). The platform-led Forwarding lifecycle owns installation
and runtime admission through the canonical platform unit, `ardents-next node
forwarding --retained-config` and `forwarding repair-retained`; the retained
repair mechanisms live in `platform/systemd` (ForwardingRuntime),
`platform/cgroup` (CheckForwardingRecoveryQuiescent) and `network/state`.
Forwarding Release signing is preserved and bound to `cmd/ardents-next` plus
`infrastructure.PlatformUnitTemplate`. Platform-led operator preparation is
realized by the node domain's pure runtime-plan declaration codec
(`internal/domain/node/plan/declaration.go`) and `ardents-next node
prepare-plan` (#673, ADR-0125); it supplies no universal role policy.

`admission/spending.OpenRetained` uses the existing standard-library and native
Unix flock / Windows `golang.org/x/sys/windows` lease mechanisms without file
creation or incomplete-tail repair. `admission/receiving.OpenRetained` is its
non-test owner consumer; `node.ListenForwardingRetained` separately consumes it
with noncreating participation opening and the unchanged genuine duty/resource
checks. These existing packages add no imports or runtime dependency. Their
portable retained-root tests use real files/leases and no GC/process-exit release
substitute. This storage seam grants no installed restart or native qualification.

The fixed role `forwarding` uses `ardents-next infrastructure
prepare-runtime-descriptor` and `prepare-runtime-inventory`. Release
Infrastructure retains the protected ext4 inputs and exclusive durable output
through Platform Artifacts. The command decodes arguments and reports outcomes.
The former `prepare-template`, `prepare-request`, `prepare-plan`,
`prepare-hosting` and `prepare-state` producers retired with the transitional
Forwarding orchestration (#604 FJD stage D, ADR-0125).
`ardents-next platform template` emits the canonical platform unit template.
`infrastructure/artifact_identity.go` owns the fixed artifact names.
The shared descriptor grammar in `artifact.go` checks exact bytes, including
independently encoded Forwarding descriptor fixtures.
Enrollment's existing finite inventory mechanism admits the exact separate
Forwarding seven-file inventory; Source Installation still requires Source
facts, and no runtime dispatcher or Source Startup permission is generalized.
`infrastructure/forwarding_admission.go` consumes exactly the standard library,
Enrollment and Release for its distinct opaque ForwardingAuthorization. The
shared immutable pair implementation admits neither Source Startup nor native
effects. The separate signed candidate integration in
`internal/domain/release/infrastructure/forwarding_artifact_linux_test.go` consumes exactly those three
owners and `github.com/theupdateframework/go-tuf/v2/metadata`; its public signed
artifact oracle shares no live root or installed runtime. Source's unchanged
exact descriptor vector remains separately checked.
The common fixed Node Release argument producer
(`node_release_arguments_linux.go`) owns the single shared public-option
uniqueness grammar (`uniqueControlOptions`) consumed by every
Forwarding/JOIN/Descriptor record, profile, token-material, release-plan and
Epoch argument parser; the caller supplies its exact repeatable option set
(the Epoch producers pass domain/record-plan/record), and the option
predicate matches Go flag's own parseOne (#718, audit E1-3). It imports only
the standard library. No Source request, account, Startup or native
installation policy is broadened.

The fixed systemd recovery replacement opener matches the genuine expected
broker-instance/unique owner on its original connection before any RefUnit or
unit load. It has the same exact imports and physical Close owner as ordinary
replacement. Its real caller is the fresh recovery composer; an expectation
string does not grant Installation authority or replace the original-cut proof.

The transitional Source orchestration file-level import permissions (source
recovery/replacement/transition/cut/startup/stop/retirement owners in
`infrastructure`, `systemd/source_identity_linux.go` and the canonical
`cmd/ardents-node/source_installation_linux.go` consumer) are removed with the
orchestration (#604 stage 1.6, ADR-0125).

State's `source_inspection_linux.go` consumes exactly
`network/source/servingdeclaration`, `network/duty`, `network/state/durable`
(all under `internal`) and the standard library. The two native `source_inspection_linux.go` files under
`state/durable` and `duty` consume only the standard library. Their actual
non-test caller is State; no mutable root or lease escapes the owning
inspection. The genuine command consumer is `cmd/ardents-next/platform_linux.go`
through the original retained SourceInspection.

`internal/domain/release/infrastructure/signing/source_release_predecessor_linux.go` additionally imports
exactly `internal/domain/enrollment`, `internal/domain/release/infrastructure`,
`internal/domain/release`, `github.com/theupdateframework/go-tuf/v2/metadata`
and the standard library. It retains the finite public predecessor input and
checks the fixed offline forward-signing purpose; it grants no Installation or
runtime authority. `source_release_linux.go` remains the direct signing caller.
| Directory | Declaration | Responsibility and consumer | Permitted imports |
|---|---|---|---|
| `internal/domain/release/infrastructure/launch` | `package launch` | Class: domain. Owns portable fixed-role declaration, credential and writable-root projection. Native pre-start joins original artifacts, loaded systemd observation, the Enrollment and Release proofs and read-only Source State inspection. Retained recovery holds the exact original plan and manager through the State repair. | `internal/domain/admission/issuance`, `internal/canonicaldigest`, `internal/domain/enrollment`, `internal/domain/network/source/serving`, `internal/domain/network/source/servingdeclaration`, `internal/domain/network/state`, `internal/domain/node/plan`, `internal/pathcheck`, `internal/platform/artifacts`, `internal/platform/process`, `internal/platform/systemd`, `internal/domain/release`, `internal/domain/release/infrastructure`, standard library. Linux behavior test only: `internal/platform/processguard`. |
| `internal/domain/release/infrastructure` | `package infrastructure` | Class: domain. Owns native descriptor and inventory preparation and verifies canonical infrastructure artifacts. The offline preparation retains exact inputs through exclusive output and closure using Platform Artifacts. Runtime admission verifies coherent Release pairs from original Enrollment snapshots. Platform pre-start uses runtime authorization. Source distribution inspection owns the independent history path check, Enrollment and joined Release verifier. The `ardents-next infrastructure authenticate-*` adapter supplies parsed inputs and renders the receipt. | `internal/canonicaldigest`, `internal/domain/enrollment`, `internal/domain/enrollment/infrastructureinventory`, `internal/domain/release`, `internal/platform/artifacts` for protected input preparation and output, `internal/pathcheck`, standard library. Test-only `tests/releasefixture` in `authorization_lifetime_linux_test.go` and `forwarding_artifact_linux_test.go`, and `github.com/theupdateframework/go-tuf/v2/metadata` in `forwarding_artifact_linux_test.go`. The offline Release handoff in `platform_release_linux_test.go` uses `tests/releasefixture`, `tests/epochfixture/network`, `internal/domain/network/duty`, `internal/domain/network/state`, `internal/platform/systemd` and `github.com/theupdateframework/go-tuf/v2/metadata`. The Linux inventory refusal test uses `tests/releasefixture` for its independently signed baseline. The Source Release candidate and retained-history tests use `tests/releasefixture` and `github.com/theupdateframework/go-tuf/v2/metadata`. |

The infrastructure launch owner holds native pre-start and retained repair. The
command dispatches operations and renders their results. Exact additional import
permissions:

Explicit Source repair stays with Network State and participation. Portable
`state/source_repair.go` consumes only `network/duty`, `network/epoch` and
`network/state/durable`; `state/retained_history.go` consumes only `network/epoch`
and `network/state/durable`. Its Linux native adapter and detached policy tests
(`state/source_repair_linux.go`, `state/source_repair_linux_test.go`) additionally
consume exactly `platform/systemd`; only the implementation also consumes
`platform/cgroup` and `platform/process`. These are
independent physical observations, not Installation transaction authority.
The `launch/repair_linux.go` consumer holds the original declaration and manager
until State returns, then closes both. Node supplies the four fixed role
declarations. The independent
non-root Source cgroup recovery observation reuses the fixed physical mechanism;
existing root-only quiescence/retirement APIs keep their original restrictions.
`durable/retained_reader.go` and `duty/retained_history.go` use only the standard
library and have real State repair callers. The reader exposes no State writes;
the participation adapter owns exact producer inspection, preserved refusal
bytes and ordinary generation/watermark commit. No old Source runtime is used.
The process leaf `source_recovery_linux.go` uses only the standard library and
fixed actual service-principal procfs observations. State is its genuine caller;
`identity.go` owns strict UID-field parsing with portable behavior tests. This
read-only refusal check has no signal, adopting or original completion surface.
Recovery domain units use Unix filesystem semantics; the public repair command
is Linux-only. Windows retained participation repair explicitly refuses before
the ordinary opener's mutating DACL enforcement; no Windows repair is selected.

Explicit Introduction repair stays with Network State and participation.
Introduction is one of the four node-duty roles whose participation repair is
ONE parameterized mechanism within package state (W4, #711; audit AR-N-01/E1-5).
Portable `state/node_duty_repair.go` owns the shared `nodeDutyRepair`
open/checkHistory/close mechanism and the `nodeDutyMember` binding, consuming
only `network/duty`, `network/epoch` and `network/state/durable`; the role's
portable `state/introduction_repair.go` carries only its refusal sentinel and
`nodeDutyRepairRole` descriptor. The shared Linux skeleton
`state/node_duty_repair_linux.go` consumes `network/epoch` and
`platform/systemd/observation`; the role's Linux wrapper
`state/introduction_repair_linux.go` additionally consumes exactly
`platform/systemd`, `platform/systemd/observation` and `platform/cgroup`, supplying its original read-only
manager observe closure and per-unit quiescence. The detached cross-role policy
tests (`state/node_duty_repair_linux_test.go`) consume exactly
`platform/systemd/observation`. Unlike Source, Introduction shares the
`ardents-node` account with sibling Node role units, so quiescence is the per-unit
`cgroup/quiescence_linux.go` observation only — never an account-wide
process scan. The original Node producer identity is random, so
`duty/retained_history.go` `CheckNodeProducer` discovers the sole live `node-duty`
fence and binds it to the current generation's node identity; slot claims and spend
obligations are never repairable. The launch owner retains the original files and read-only manager through
the State repair attempt. The separate
`systemd/introduction_manager_linux.go` owns the fixed Introduction read-only
runtime connection with the exact `introductionUnitReply` vocabulary.
It grants neither installed admission nor domain/native join. No old Introduction runtime is used.

Explicit Forwarding, JOIN and Descriptor repair stay with Network State and
participation as the same shared node-duty mechanism, mirroring Introduction.
Their portable role files (`state/forwarding_repair.go`, `state/join_repair.go`,
`state/descriptor_repair.go`) carry only the role sentinel and descriptor; the
shared `state/node_duty_repair.go` mechanism consumes `network/duty`,
`network/epoch` and `network/state/durable` and owns the `nodeDutyMember`
binding that ties the discovered sole live `node-duty` fence to an accepted
Candidate of the current generation with the exact authenticated family digest.
The shared Linux skeleton `state/node_duty_repair_linux.go` consumes
`network/epoch` and `platform/systemd/observation`; the role wrappers
(`state/forwarding_repair_linux.go`, `state/join_repair_linux.go`,
`state/descriptor_repair_linux.go`) additionally consume exactly
`platform/systemd`, `platform/systemd/observation` and `platform/cgroup`; the Descriptor wrapper
additionally consumes `network/epoch` and `reachability` for the bounded
existing-only Store uncertain-stage recovery (`RepairRetainedStoreStage`) under
the same dead-unit proofs and retained leases, passed to the skeleton as the
role's optional Store-stage hook at the unchanged ordering point. The portable
behavior tests (`state/forwarding_repair_test.go`, `state/join_repair_test.go`,
`state/descriptor_repair_test.go`, `state/introduction_repair_test.go` and the
cross-role equivalence oracle `state/node_duty_repair_test.go`) consume only
`network/duty` and `network/state/durable`. All four roles share the
`ardents-node` account with sibling Node role units, so quiescence is the per-unit
`cgroup/quiescence_linux.go`
observation only — never an account-wide process scan. JOIN repair binds the
`/var/lib/ardents/roles/join/` root prefix,
the fixed `ardents-join.service` stopped policy and the exact
`node join --retained-config` declaration; each JOIN pair participant owns a
separate participation root and sole fence, so repair is per-invocation, never a
shared pair effect. Descriptor repair binds the
`/var/lib/ardents/roles/descriptor/` root prefix, the fixed
`ardents-descriptor.service` stopped policy, the exact
`node descriptor --retained-config` declaration and the five ordered writable
roots (network, participation, spend, hosting, descriptor/Store); its process
profile is `processguard.DescriptorProfile`. The Store lease is flock and dies
with the process, so the Store root carries no durable fence to remove; the
bounded repair instead validates every committed publication floor and removes
only an uncertain `.record-stage` under the exclusive Store lease, never
initializing a missing root, resetting/evicting a floor, promoting staging or
deleting foreign entries. Slot claims, spend obligations and publication floors
are never repairable. The JOIN and Descriptor declarations' process profiles are
`processguard.JoinProfile` and `processguard.DescriptorProfile`, not the
shared Node profile. The separate `systemd/join_manager_linux.go`
`JoinRuntime`/`OpenJoinRuntime` and `systemd/descriptor_manager_linux.go`
`DescriptorRuntime`/`OpenDescriptorRuntime` read-only connections are retained for
`join repair-retained` / `descriptor repair-retained` manager observation; they
load/refs/start/stop nothing. The transitional JoinControl and DescriptorControl
start-stop halves retired with the JOIN/Descriptor stage D orchestration removal
(#604 FJD stage D, ADR-0125); only the read-only JoinRuntime/DescriptorRuntime
remain. No old Forwarding, JOIN or Descriptor runtime is used.

| File | Permitted imports |
|---|---|
| File `internal/domain/network/state/source_repair_linux.go` | File `internal/platform/systemd`; `internal/platform/systemd/observation`; `internal/platform/cgroup`; `internal/platform/process` |
| File `internal/domain/network/state/source_repair_linux_test.go` | File `internal/platform/systemd/observation`; `internal/platform/systemd` for missing or unbound original handles |
| File `internal/domain/network/state/node_duty_repair_linux.go` | File `internal/domain/network/epoch`; `internal/platform/systemd/observation` (shared node-duty repair skeleton, W4 #711) |
| File `internal/domain/network/state/node_duty_repair_linux_test.go` | File `internal/platform/systemd/observation`; `internal/platform/systemd` for missing or unbound original handles |
| File `internal/domain/network/state/introduction_repair_linux.go` | File `internal/platform/systemd`; `internal/platform/systemd/observation`; `internal/platform/cgroup` |
| File `internal/domain/network/state/forwarding_repair_linux.go` | File `internal/platform/systemd`; `internal/platform/systemd/observation`; `internal/platform/cgroup` |
| File `internal/domain/network/state/join_repair_linux.go` | File `internal/platform/systemd`; `internal/platform/systemd/observation`; `internal/platform/cgroup` |
| File `internal/domain/network/state/descriptor_repair_linux.go` | File `internal/domain/network/epoch`; `internal/platform/systemd`; `internal/platform/systemd/observation`; `internal/platform/cgroup`; `internal/domain/reachability` (bounded existing-only Store stage recovery) |
| File `cmd/ardents-next/platform_linux.go` | `internal/domain/release/infrastructure/launch` for retained infrastructure launch. The native command also opens and closes the shared `internal/platform/artifacts` input owner. |
| File `cmd/ardents-next/release.go` | Files `internal/domain/enrollment` and `internal/domain/release` for initial and candidate Release inputs used by both software delivery paths |
| File `cmd/ardents-next/platform_endpoint_staging_linux.go` | File `internal/domain/enrollment`; `internal/domain/release/endpoint`; `internal/domain/endpoint/launch`; `internal/domain/release` for first-delivery floor verification and signed successor staging under the retained Release lease |
| File `internal/domain/release/infrastructure/platform_release_linux_test.go` | File `internal/domain/enrollment`; `internal/domain/release`; `internal/domain/release/infrastructure`; `internal/domain/network/state`; `internal/domain/network/duty`; `internal/platform/systemd`; `tests/epochfixture/network`; `tests/releasefixture`; `github.com/theupdateframework/go-tuf/v2/metadata` |

The Source CLI and its process fixtures consume `servingdeclaration` directly
in the exact files listed by the import guard. File custody stays with the
native adapters, and composition converts Source's embedded network
declaration only where it builds the existing State configuration.

`internal/domain/network/state/source_state_files_linux.go` owns Source State
preparation from the original public files through both retained histories.
It uses `network/source`, `network/source/servingdeclaration`,
`platform/artifacts`, `platform/process` and `processguard`.
State behavior tests exercise the original file and root custody against
independent signed `tests/epochfixture/network` fixtures.
The CLI adapts argument, refusal and receipt output.

`infrastructure/runtime_artifact_linux.go` retains a distinct
`RuntimeAuthorization` for the six fixed Source/Node artifacts; native
Installation transactions cannot consume it. Its exact imports are Enrollment,
Release and the standard library, with real consumers in `platform_linux.go`
and the existing offline public preparation command. `systemd/platform_linux.go`
uses only the standard library and the existing closed GetAll mechanism to
observe those six fixed service objects and independently check sibling-slot
absence on its own absence-only manager connection; it exposes no mutation or
arbitrary unit selection. Systemd observations and artifact authorization have separate owners. See the [platform launch contract](../technical/network-route-node.md#platform-managed-infrastructure-launch).

`internal/domain/release/infrastructure` owns portable canonical descriptor preparation
and verification for infrastructure artifacts. Its Linux admission binds the
program and descriptor to fresh Release proofs and the original Enrollment
inventory. `internal/platform/systemd` owns `PlatformUnitTemplate`. The
platform command and offline Control bind its exact bytes. The artifact
package consumes only `internal/domain/enrollment`, `internal/domain/release` and the
standard library.

Public artifact preparation uses the `ardents-next infrastructure` adapter,
with the fixed invocation selected by the operator. It consumes
`internal/domain/release/infrastructure` for portable descriptor encoding and
resource validation, and `internal/domain/enrollment` for the exact native inventory.
Original ext4 inputs, exclusive output and bounded receipts stay in command
composition. The former Control public preparers for Source, Forwarding, JOIN
and Descriptor are removed. Fixed Release signing and prerequisite verification
retain their purpose-specific Control commands.

Source's retained native leaves serve the platform-led repair path only.
`systemd.OpenSourceRuntime` has its own non-root read-only loaded-Source
GetUnit/property connection for `source repair-retained`; it starts/stops
nothing. `cgroup.CheckSourceRecoveryQuiescent` and
`process.CheckSourceRecoveryQuiescent` freshly check the fixed empty/absent
Source scope and actual service-principal procfs quiescence before repair
abandons a dead producer; they cannot infer historical join. The transitional
`process.SourceExpected`/`RetainSource*`/`InterruptSource`, `cgroup.SourceScope`,
root `systemd.SourceControl` start/stop owners and the
`source_start_records`/`source_stop`/`source_retirement_history` transcript
owners are removed with the orchestration (#604 stage 1.6, ADR-0125). A
detached dictionary or scope observation grants no Release, Startup, Source
serving or joined process; their presence grants no installed qualification or
bootstrap acceptance.

Portable State/durable `OpenRetained` have actual successor Source consumers
and share State's existing recovery/lifetime implementation without initialization
of missing retained history. No new package or runtime dependency is introduced.

The distinct Source inventory/codec compile only
on Linux, matching their genuine accepting consumer; non-Linux commands refuse
before effects. Existing Endpoint/v3 grammar and its portable controls stay
unchanged. `StaticMetadataFiles` is likewise Linux-only. The platform-selection
architecture check asserts the exact retained native Source file and Linux deterministic
profile membership, and separately requires portable retained State files on
Linux, Windows and macOS, without a deadcode allowance or a fictitious native caller.
The shared original-proof/floor check has a genuine existing portable consumer
in `cmd/ardents-next/release.go`, before its verification report and physical
Close; it does not provide a native infrastructure accepting path on Windows.

Node certificate preparation lives in Linux Provisioning. Its existing fixed
infrastructure roles and `network prepare-node-certificate` share the exact
transport leaf construction and verifier. The latter retains its independent
Record commitment and signing-time check before entering the common codec. `epoch/initial_record_verification.go`
adds `VerifyInitialClosedRecord`: exact canonical preparation, generation-one
ARNR-v2 signature and detached public facts, without State acceptance. It uses
only the standard library; its behavior tests do too. No package, private-key
Interface, runtime credential mechanism or dependency is added.

`internal/domain/endpoint/source_trust_preparation_linux.go` owns the offline Source
public preparation. It reuses the Endpoint declaration types, Source and
serving declaration owners, exact process profile and Platform Artifacts for
native input/output. `internal/domain/network/source/command/prepare_linux.go` adapts
arguments, timeout and receipts. Native tests use the Endpoint preparer for
failure controls and the compiled command for actual preparation. The process
test additionally consumes `network/source`, `network/state` and independent
`tests/epochfixture/network`.
Portable `network/source/public_trust.go` owns public X.509 observation, sharing runtime
declaration/collision/key-digest mechanisms without constructing a live Plan
for callers. P1-B batch 1 (#781, #691) dropped its accidental Linux constraint
after per-file purity verification: it composes only portable Source helpers and
adds only standard-library imports, with no native mechanism. Its portable
behavior test executes on the windows/macos lane; the genuine production
consumer remains the Linux `endpoint/source_trust_preparation_linux.go`, so the
observation is windows-production-dead under a reviewed deadcode allowance, and
other platforms retain runtime and explicit command refusal. The existing installed
Source composer retains one pure `installedNetworkAuthority` projection used by
both runtime and preparation; private loading remains runtime-only. Native
finite output uses `internal/platform/artifacts` through the Linux command
adapter, with final inventory following the last original-input I/O.
`network/source/connection_cancellation.go` is a standard-library physical
mechanism used by both Fetch and serving; it joins the exact original context
callback before their connection/key/user ownership can return. It adds no
domain callback, policy, new clock or accepting transport substitute.

Initial Network key preparation belongs to `internal/domain/node/provisioning`
(`initial_network_keys_linux.go`), which admits the closed plan, creates the
State key and distinct Node keys and returns the exact published public receipt.
The CLI projects it through `network_commands.go`. Provisioning tests
exercise expiry after native I/O and durable signing inventory publication on
ext4, including original Closed Profile commit and output contention, retained
key custody, and physical child interruption for Record, Profile and Certificate. The separate `networkpreparation` owner tests unsigned publication
through original file faults, partial writes and child interruption. The CLI retains the genuine
command-to-State chain and receipt probes.
Both offline generation and the
purpose-specific signers use Platform Artifacts' canonical Ed25519 PKCS#8 codec.
Private generation uses protected native inventory publication, and signing
retains its original public inventory and key-specific checks. Unsigned
preparation uses its separate caller-owned parent and exact file limit.
Provisioning clears owned secrets and private readback buffers. The pure
Network format owners receive public bytes only. Native key tests
use standard-library command composition and the existing registered preparation
chain harness; that harness may consume independently generated test keys while
its Admission fixture remains separate. It imports no new domain owner.

The Network command in `cmd/ardents-next/network.go` delegates detached artifact
preparation and signing to `internal/domain/node/provisioning`. The corresponding CLI
files parse arguments and project receipts and statuses. The exact declaration, size
limit, original input custody and finite publication live in Provisioning. The
portable preparation codec uses Epoch, Closed Profile and Platform Artifacts.
Linux-only files bind the original reviewed commitment, frozen validity interval,
protected key and purpose-specific Record, Epoch, Profile or certificate output.
The private key stays with the signing owner. The command checks the retained
interval around the public signing receipt.

`networksigning/declaration_unix.go` consumes Epoch, Closed Profile and
Platform Artifacts. `networksigning/profile_unix.go` uses Closed Profile.
`networksigning/certificate_operation_unix.go` uses Epoch and the shared certificate
codec. The native input and output files use Platform Artifacts. Owner tests
check exact signatures and certificates, while command tests retain the original
process, physical refusal and signed State chain controls. Unsupported platforms
refuse the native operation through its owner adapter.

Cohort 2b (#859, portability design audit #848) de-tags the universal and
Unix-generic codecs to Linux and macOS: the role declaration, public identifier
decoder, Node transport leaf codec, Node Record plan, offline Network signing
declaration/certificate/profile codecs, the public report writer and the
release-key receipt, plus the pure `artifacts/flag_options_unix.go` option
grammar and `closedprofile/signing_transcript_unix.go` verification leaf they
compose. The genuine Linux boundaries stay Linux with the same typed refusal
elsewhere: the ext4 storage floor and protected input/output composition
(`networkpreparation/prepare_linux.go`, `artifacts/output_linux.go`,
`artifacts/protected_input_linux.go`, `artifacts/protected_storage_linux.go`),
the Enrollment infrastructure inventory and its `/proc/self/exe` running-program
proof, and the systemd Release plans. A macOS storage floor and a
security-equivalent macOS process-identity capability remain separate Product
Owner decisions; `os.Executable` is not equivalent to `/proc/self/exe`.

`internal/domain/node/command/report.go` imports exactly
`internal/domain/node/runtime` and the standard library to encode its copied original
first-failure projection inside the established single terminal report. Node's
`runtime/failure.go` imports only `processguard` and the standard
library; it records local phase/cause/elapsed facts without foreign I/O, authority
or private identity. P1-B batch 2 (#787, #691) dropped its accidental Linux
constraint after per-file purity verification: it composes only the portable
processguard failure projection and standard-library imports, with no native
mechanism. Its portable behavior tests execute on the windows/macos lane; the
physical first-failure join test still composes the Linux forwarding lifetime and
uses the Linux/macOS build selection (`runtime/failure_join_unix_test.go`). The Linux-only
`admission/network_profile.go` public-observation projection was likewise de-tagged in
batch 2 (it composes only portable `admission`/`network` facts) with a new
portable behavior test. Process-resource failure facts retain original numeric
measurements separately from Node's physical cleanup causes.
`processguard/admission_diagnostics.go` owns the copied failure projection portably: P1-B
batch 1 (#781, #691) merged the former `failure_linux.go` `Failure` projection
into the already-portable fact representation after verifying it is pure
(`errors` only, no native mechanism). Its portable behavior test executes on the
windows/macos lane. Its genuine Node consumer `node/runtime/failure.go` is now
portable, but that projection's own command consumers remain Linux
(`internal/domain/network/source/command/serve_linux.go`, `internal/domain/node/command/report.go`), so
`Failure` and the Node projection stay windows-production-dead under reviewed
deadcode allowances. The native process/cgroup observation mechanism stays Linux
(`native_linux.go`) with its unchanged `!linux` typed refusal (`native_other.go`);
unsupported-platform commands refuse acquisition.

[ADR-0122](../adr/0122-select-tcp-only-closed-release.md) selects TCP/TLS alone
for the nearest closed release. The QUIC adapter and its import grants retire;
historical signed profile identifiers stay distinguishable but grant no runtime
execution or fallback. Route owns current Carrier admissibility, Network retains
canonical signed facts, and each retained/new runtime enforces its own boundary
without an old/new bridge. Historical both-Carrier descriptions are not an import
grant or current qualification of a removed mechanism.

The installed exchange scenario in `internal/domain/endpoint/endpoint_journey_native_linux_test.go`
consumes `internal/application/administration`, `internal/application/connection`
and `internal/application/document` for actual public client operations.
Its local controller uses `tests/e2e/endpoint/pair_messages_linux.go` to transfer
public signed Network inputs and actual permission requests and responses between
separately installed participants. It neither imports
private Instance/Publication runtime nor transfers a live domain owner.

The five runtime roles share joined invocation supervision in
`internal/domain/node/runtime`. The supervisor waits for physical completion
before releasing the borrowed Network authority, including after a retirement
timeout. The `cmd/ardents-next/node_roles.go` adapter delegates to `node/command`,
which decodes `node/plan` and starts the exact invocation through `node/runtime`.
The declaration grammar and outcome remain portable. Linux is the command's
supported runtime host. Linux and macOS select the native Go role composition
and Unix file custody, with process protection required before any accepting
role effect. Other hosts select the typed unavailable path.
The separate `route receive` development command dispatches to `node/receive`.
Both consumers use `node/admission` for the same narrow Admission/Hosting
handoffs. Exact imports are registered in the package table below and enforced
at their original call sites.

The platform-led JOIN retained evidence and repair (#604 FJD stages A+B+C,
ADR-0125) is exercised by `tests/e2e/node/retained_lifecycle_native_linux_test.go` (`node_process_native`; imports
`processguard`, `network/duty` and `route/transport`) and
`join_platform_native_linux_test.go` (`join_platform_native`; imports
`release/infrastructure`, `network/duty`, `processguard` and
`route/transport`). The lifecycle test proves retained relaunch and SIGKILL
crash refusal for one single-unit JOIN member; the installed gate proves the
real `ardents-join.service` platform-led lifecycle including the bounded
`node join repair-retained` against real failed-unit manager facts. The
transitional JOIN orchestration is not exercised or retired by these files;
that retirement is the separately authorized stage D.

The platform-led Descriptor retained evidence and repair (#604 FJD stages
A+B+C, ADR-0125) is exercised by `tests/e2e/node/retained_lifecycle_native_linux_test.go` (`node_process_native`; imports
`processguard`, `network/duty`, `route/transport` and `reachability` for the
genuine fresh Store-root provisioning and post-crash retained reopen) and
`tests/e2e/nodeinstalled/descriptor_platform_native_linux_test.go` (`descriptor_platform_native`;
imports `release/infrastructure`, `network/duty`, `processguard`
and `reachability` for the genuine Store floor commit and
post-repair retained reopen). The lifecycle test proves retained relaunch and
SIGKILL crash refusal for one single-unit Descriptor member (id 17, domain 2,
subrole 5); the installed gate proves the real `ardents-descriptor.service`
platform-led lifecycle including the bounded `node descriptor repair-retained`
against real failed-unit manager facts and per-unit cgroup quiescence, plus a
causal mid-Publish-crash Store uncertain-stage recovery leg (a genuine committed
floor is preserved byte-exact while only the staging is removed under the
exclusive Store lease). The readiness oracle is the authenticated listener, not
publication readiness; genuine publish/lookup remains
`TestNativeDescriptorPublishLookupAndReopenTCP`. The transitional Descriptor
orchestration is not exercised or retired by these files; that retirement is the
separately authorized stage D.

The native authority and operation tests in `node/runtime` open genuine State
and Node owners with signed inputs prepared outside the measured cgroup.
A scheduling barrier delays completed State observation and confers no
accepting authority. The native Forwarding refill test exercises the same
command profile and real client Carrier. The Descriptor role's Reachability
dependency remains scoped to `runtime/descriptor_unix.go` and its exact
behavior tests. The compiled CLI's process lifecycle scenarios stay with
`tests/e2e/node`.

The exact `node_completion_native_linux_test.go` additionally imports
`internal/platform/processguard`, `internal/domain/network/duty` and
`internal/domain/route/transport` to exercise the actual command's bounded
unavailable-completion result against a genuinely occupied producer-root lease.
The lease is a test fault, not a live shared-root design or a recovery right.

This is the factual naming and dependency registry for maintained Go code. It
lists packages that exist, not future intentions. A new entry is an architecture
change: name the responsibility first, add the real Implementation and tests,
then update this table in the same change. The normative growth and dependency
rules are in [repository-layout.md](repository-layout.md).

The checked installed component's
`internal/domain/endpoint/connection_response_loss_linux_test.go` additionally imports
exactly `connection`, `execution/runtime`, `publication/runtime`, `route/transport` and
`internal/application/connection`: it observes
original authenticated bytes and qualified operation delivery on the selected
Carrier profiles and checks the defined Application outcome after original
Reader cancellation. Its withdrawal oracle invokes the genuine original
Publisher and checks original joined cleanup without revoking either Job.
It constructs no private Instance, TLS/QUIC adapter or successful authority.
Within `publication/runtime`, only `publisher_streams.go` additionally imports
`connection/wire`, to compute the canonical full-tuple dispatch commitment.
The commitment grants no authentication; the original Stream checks the private
recipient and original-key Continuity before transfer. These exact allowances
grant no other command, Publication file or old owner the same dependencies.

Route Introduction's `Receipt`/`CheckReceipt` retain one original genuine
registration and its immutable public binding; `RegistrationFacts` are detached
data only. Prefix's `IntroductionChannel.ProfileDigest` supplies its original
authenticated profile. The `endpoint/session` client obtains the checked
receipt before returning registration facts to the command. These are seams in the existing
Introduction/Prefix packages, with unchanged import directions and no Descriptor
ACK, Publication readiness, private recipient or Execution authority.

The new command's portable `execution_holder.go` consumes Execution and its runtime;
`admission_holder.go` consumes only the runtime's sealed preparation/live checks.
The registered installed command test `execution_installed_linux_test.go` may
import `route/transport` for the two exact Carrier identities; concrete adapters
remain solely with the registered Route fixture and production Route owners.
Those exact files extend the command row's imports. The Document Application
grammar is independently owned: only runtime `document_init_linux.go` borrows
`document.InitializeWorker`. No old Endpoint, Broker, Source or Service
runtime owner is an Execution dependency or consumer.

The exact native command scenario `publication_installed_linux_test.go`
additionally imports `publication` for its public Link oracle,
`publication/instance` for independently approved host-generated input,
`reachability` for genuine Store/reopen and proof verification, and
`route/transport` for the selected Carrier identities. It drives the production
qualified console; it imports neither `publication/runtime` nor
`publication/durable`, and constructs no concrete Carrier adapter. These exact
test imports extend the command row without granting another test or production
file the same dependencies.

The exact `connection_authority_linux_test.go` scenario additionally imports
`connection`, `execution/runtime`, `route/transport` and
`internal/application/connection`. It observes original qualified operations
and real Application refusal after physical worker stop, independent local
Network loss or elapsed original safety bounds. It supplies no accepted
authority, stream records or successful Application result.

The exact `connection_reopen_linux_test.go` scenario additionally imports
`publication/durable`, `publication/instance`, `reachability` and
`route/transport`. After the original Connection and all physical owners join,
it reopens their actual roots to check retained Publication generation,
consumed Instance authority and exact signed Descriptor bytes. Carrier imports
select the two registered profiles; they grant no concrete adapter construction.
These imports apply only to this test file, without extending production imports.

The separately registered `publication_ack_installed_linux_test.go` imports
exactly `execution`, `execution/runtime`, `publication`, `reachability` and
`route/transport` in addition to the command's existing common dependencies.
It launches the genuine installed snapshot operation and invokes production
`publisher.Open`; direct handle calls observe Link concurrently with
the real Store exchange. It neither imports Publication runtime/durable nor
constructs Carrier adapters. The receiving authority callback only holds
observation after actual public record replacement; it supplies no ACK.

The signed Network fixture accepts the same signed Epoch/profile inputs into a
separate receiving State root with its own lease and clock observation.
The isolation test verifies that a conflict in one root leaves current authority
and member assignments in the other root intact.

The Linux `installed_endpoint_linux.go` adapter calls `endpoint.Run` and renders
only public progress and the final phase. The original participant owns Startup,
Execution preparation, State, permission-file exchange, Stock, shared funding
parents and IPC listeners. Its `declarations_linux.go` projects Installation's
bound inputs; offline Source preparation consumes those same public schemas and
pure Network configuration without acquiring a live participant. The native
`internal/permission/exchange_linux.go` retains write-once/exact retry and original parent checks.
Listeners and Service borrowers join before funding parents, Stock, State,
Execution and Startup close. Reader and Publisher retain their own operation
lifetimes. CLI parsing, rendering and process exit projection stay in
`cmd/ardents-next`; no other binary is a new consumer.
Those consumers do not establish two-manager installed acceptance.

[ADR-0117](../adr/0117-reset-closed-service-formats-to-v3.md) selects the
current Service v3 format set. It adds no package or import permission.

R-091 retired the closed laboratory namespace. Its provenance is retained in
Git history; no current package may import or start a lab Module.

| Directory | Go declaration | Responsibility | May import |
|---|---|---|---|
| `internal/platform/processguard` | `package processguard` | Class: platform adapter. Mandatory separate Forwarding, Issuer, Descriptor, Introduction, JOIN and Direct-Origin Source development process profiles, original Linux process/cgroup pins, actual placement and resource observations, elapsed CPU interval and retained fail-closed pressure/currentness decision. It owns no Node, State, participation, Admission, Hosting or Route authority. Runtime consumers: `internal/domain/node/runtime` and `internal/domain/network/source/serving`. The `ardents-next node inspect-process` diagnostic calls `internal/domain/node/command`. | standard library |
| `internal/durablefile` | `package durablefile` | Class: supporting mechanism. Shared durable filesystem operations and generation encodings for owner-only stores (R1, #764): the content-addressed generation-name grammar and SHA-256 naming, bounded reads, exclusive owner-only creation, staged same-directory replacement, generation pruning, the watermark floor codec, owned-root inspection, directory synchronization, the exclusive Unix root lease with its strict regular-file/single-link identity check and the Windows owner-only root DACL enforcement (#793): a protected DACL granting only the current process owner SID full control, inherited by future children and applied without inheritance to every existing regular file, refusing whenever enforcement fails. Each caller owns its root, marker, lock name and lifetime, record schema, authorization policy and refusal text. The shared lease identity check refuses a non-regular or multiply-linked lease file so a substituted inode can never become a store's exclusive fence. Real consumers: `internal/domain/route/selection`, `internal/domain/network/duty` (durable-file mechanism and Windows owner-only DACL enforcement), `internal/domain/network/state/durable` (durable-file mechanism) and `internal/domain/release` (generation-name grammar). `internal/domain/publication/instance` retains its separate joined-error, TOCTOU-hardened private custody and does not consume this mechanism. The Unix-family (Linux and macOS) inspection port (#850) owns the directory-relative open, the normalized inode change/modification times and the shared full-identity comparison consumed by `internal/application/document`, `internal/domain/endpoint/generation`, `internal/domain/endpoint/launch`, `internal/domain/endpoint/plan`, `internal/domain/node/runtime`, `internal/domain/node/provisioning` and `internal/platform/artifacts`; the two Stat_t time field spellings stay at that boundary. The directory-relative atomic rename (Cohort 1 N2, #853) extends the same port for the Introduction slot-history staged replacement consumed by `internal/domain/route/introduction`. It grants no installed restart or native qualification. | standard library; `golang.org/x/sys/windows` only in `owner_root_windows.go` and `owner_root_windows_test.go`; `golang.org/x/sys/unix` only in `relative_open_unix.go` and `relative_rename_unix.go` (Unix-family inspection port, #850; directory-relative rename, #853) |
| `internal/pathcheck` | `package pathcheck` | Class: supporting mechanism. Shared pure path-validation grammar for the maintained owners (G7, #772): strict predicates for absolute, already-clean host paths and lexically disjoint root declarations, plus a separate literal POSIX argument predicate shared by platform launch, plan and unit rendering. They perform no filesystem observation and never absolutize relative paths. It is a mechanism leaf, not a root or custody owner: it holds no root, marker, schema, authority or lifecycle of its own, and each consumer keeps its distinct refusal prose plus any additional root, NUL-rune or length bounds it genuinely needs. It replaces the formerly repeated inline absolute/clean checks whose variants drifted across owners. Deliberate boundaries stay separate: the POSIX installation declaration continues to validate its own wire grammar with the platform-independent path package, the Application zone keeps its purity, and configured-root resolution operations (which legitimately absolutize a relative root) are not this validation grammar. Network State configuration validation now refuses relative and non-clean paths instead of silently absolutizing them — the one deliberate behavior tightening, verified against every production caller, which already supplies absolute clean paths. Real consumers: `internal/domain/admission/attempts`, `internal/domain/admission/issuance`, `internal/domain/admission/issuer`, `internal/domain/admission/issuerprofile/signing`, `internal/domain/admission/quota`, `internal/domain/admission/spending`, `internal/domain/endpoint`, `internal/domain/endpoint/funding`, `internal/domain/endpoint/generation`, `internal/domain/endpoint/publisher`, `internal/domain/endpoint/session`, `internal/domain/enrollment`, `internal/domain/execution/worker`, `internal/domain/hosting`, `internal/domain/network/duty`, `internal/domain/network/state`, `internal/domain/network/state/durable`, `internal/domain/node/plan`, `internal/domain/node/receive`, `internal/domain/node/runtime`, `internal/platform/artifacts`, `internal/platform/cgroup`, `internal/platform/process`, `internal/domain/reachability`, `internal/domain/release`, `internal/domain/release/infrastructure` and `internal/domain/route/introduction`. It grants no installed restart or native qualification. | standard library |
| `internal/canonicaldigest` | `package canonicaldigest` | Class: supporting mechanism. Shared pure canonical hex-32 validation atom for the maintained owners (Phase 4, #835): one decode predicate admitting exactly 64 lowercase hexadecimal characters whose decoded 32 bytes re-encode to the identical string, plus its validation-only projection. It performs no hashing and no filesystem observation. It is a mechanism leaf, not a root or custody owner: it holds no root, marker, schema, authority or lifecycle of its own, and each consumer keeps its distinct refusal prose plus any additional zero-value, presence or length bounds it genuinely needs. It replaces the formerly repeated private `canonicalDigest` predicates, provisioning `decodeHex32` and inline decode/length/round-trip checks whose variants drifted across owners. Deliberate boundaries stay separate: variable-length canonical hex (Issuance and command admission inputs, closed network preparation text), the weaker no-round-trip Release floor and fixed-hex decoders, the 16-byte systemd InvocationID shape, and Custody authority validation are not this atom. Release builder-attestation validation joins the canonical atom because the same package's verification already requires exact equality with `hex.EncodeToString` of the authenticated digest — the one deliberate tightening, which removes no reachable acceptance. Real consumers: `internal/domain/enrollment`, `internal/domain/endpoint`, `internal/domain/endpoint/generation`, `internal/domain/endpoint/launch`, `internal/domain/endpoint/plan`, `internal/domain/endpoint/unit`, `internal/domain/execution/worker`, `internal/domain/hosting`, `internal/domain/network/state`, `internal/domain/node/provisioning`, `internal/platform/process`, `internal/domain/publication/instance`, `internal/domain/release`, `internal/domain/release/infrastructure`, `internal/domain/release/infrastructure/launch` and `cmd/ardents-next` (`source_inspection_linux.go`). It grants no installed restart or native qualification. | standard library |
| `internal/domain/node/plan` | `package plan` | Class: composition. Owns the five exact role declaration codecs, independent writable roots and retained repair selection. The runtime command, offline runtime-plan preparation and retained infrastructure repair consume the same pure declarations. | standard library, `internal/domain/hosting`, `internal/pathcheck`, `internal/platform/processguard`. |
| `internal/domain/node/admission` | `package admission` | Class: composition. Projects the observed public Network profile for Admission and transfers original Hosting reservations into Route for Forwarding refill and Issuer work. Called by Node runtime and development receive. Network authenticates and Admission decides spend. | standard library, `internal/domain/admission`, `internal/domain/admission/receiving`, `internal/domain/hosting`, `internal/domain/network`, `internal/domain/route/ardp`, `internal/domain/route/receiver`. File-specific imports are enforced by the architecture checks. |
| `internal/domain/node/receive` | `package receive` | Class: composition. Runs the independent development Route receiver through cmd/ardents-next. Composes original State, receiving Admission, Hosting, Route and exact optional Issuer, Descriptor or Introduction owners. Linux accepts the selected development path. Other platforms return the existing typed refusal. | standard library, `internal/domain/admission`, `internal/domain/admission/issuer`, `internal/domain/admission/quota`, `internal/domain/admission/receiving`, `internal/domain/hosting`, `internal/domain/network`, `internal/domain/network/state`, `internal/domain/node/admission`, `internal/domain/reachability`, `internal/domain/route/ardp`, `internal/domain/route/channel`, `internal/domain/route/introduction`, `internal/domain/route/receiver`, `internal/domain/route/role`, `internal/pathcheck`, `internal/platform/artifacts`. Test only: `internal/domain/admission/allocation`, `internal/domain/admission/stock`, `internal/domain/admission/token`, `internal/domain/endpoint/funding`, `internal/domain/endpoint/session`, `internal/domain/publication`, `internal/domain/publication/durable`, `internal/domain/publication/instance`, `internal/domain/route/capsule`, `internal/domain/route/join`, `internal/domain/route/prefix`, `internal/domain/route/selection`, `internal/domain/route/transport`, `internal/domain/route/transport/tls`, `tests/e2e/node/processfixture`, `tests/epochfixture/network`, `tests/networkfixture`. File-specific imports are enforced by the architecture checks.; `internal/domain/reachability/store` at the existing verified caller. |
| `internal/domain/node/runtime` | `package runtime` | Class: composition. Owns one Forwarding, Issuer, Descriptor, Introduction or JOIN invocation. Retains original producer, lifecycle and private TLS key until all physical borrowers join. Each role constructs its actual owners and preserves their separate policy and state. Native tests exercise real startup, authority loss, refusal, exchange and retirement. | standard library, `internal/domain/admission`, `internal/domain/admission/issuer`, `internal/domain/admission/quota`, `internal/domain/admission/receiving`, `internal/domain/hosting`, `internal/domain/network`, `internal/domain/network/duty`, `internal/domain/network/state`, `internal/domain/node/admission`, `internal/domain/node/plan`, `internal/domain/reachability`, `internal/domain/route/ardp`, `internal/domain/route/bootstrap`, `internal/domain/route/introduction`, `internal/domain/route/receiver`, `internal/domain/route/role`, `internal/durablefile`, `internal/pathcheck`, `internal/platform/processguard`. Test only: `internal/domain/admission/issuance`, `internal/domain/admission/spending`, `internal/domain/admission/stock`, `internal/domain/admission/token`, `internal/domain/endpoint/funding`, `internal/domain/endpoint/session`, `internal/domain/network/epoch`, `internal/domain/route/capsule`, `internal/domain/route/transport`, `internal/domain/route/transport/tls`, `tests/e2e/node/processfixture`, `tests/epochfixture/network`, `tests/networkfixture`. File-specific imports are enforced by the architecture checks.; `internal/domain/reachability/store` at the existing verified caller. |
| `internal/domain/node/command` | `package command` | Class: composition. Reads role input, retains original State around the selected role lifetime, writes command results and inspects the local process. Caller: ardents-next node. | `internal/domain/node/plan`, `internal/domain/node/runtime`, `internal/domain/network/state`, `internal/domain/admission/issuance`, `internal/domain/admission/issuer`, `internal/diagnostics/measurement`, `internal/diagnostics/runtimeevent` in role_command.go; `internal/domain/node/plan`, `internal/domain/node/runtime` in report and report tests; `internal/platform/artifacts` in role_input.go; `internal/platform/processguard` in process.go; standard library. |
| `internal/domain/execution` | `package execution` | Class: domain. Volatile exact Principal/surface capability generation, independent 64 Connection and 6 combined Administration admission limits; exact Session current/last Job and once-only lifetime and operation claims, synchronous revoke and first immutable cleanup result; a distinct supervisor retains physical cleanup capacity after revoked leases and permanently after failed cleanup. Real consumer: installed Execution preparation and live invocation runtime. Local rules grant no installed confinement, Service, Network or token authority. | standard library ; `internal/diagnostics/measurement`, `internal/diagnostics/runtimeevent` for the registered finite measurement producers/consumers |
| `internal/domain/execution/worker` | `package worker` | Class: domain. Fixed root-installed worker artifact and manager inventory, selected Ubuntu 22.04/24.04 systemd 249/255 ordinary Execution admission, credentialed attachment rejecting and closing received descriptors, original pinned cgroup observation and finite original-invocation stop/join. Failed Stop still observes original descendants while retaining failure; original helper threads survive through physical child join. Public Activate/ObserveInstance selects only the closed Text inventory; the trusted qualification entries ActivateStream/ObserveInstanceStream select the fixed Stream qualification inventory (identical confinement contract; qualification identity, owner slice and nested qualification cgroup root). No caller-visible inventory selector exists; the architecture gate restricts Stream callers to the trusted qualification launch path, and each inventory refuses the other's units/slices. The unsupported-platform adapter returns only failure and no activation/artifact/cleanup pin. Internal manager/file observations are private, and original cleanup reports its first definite failure to the bound launch owner before further descendant waiting. Real consumer: Execution preparation and live invocation runtime; the installed Stream activation test is the bounded qualification launch path until the accepted runner lands. Mechanisms grant no authority or readiness. The actual loaded socket is checked before activation and again at original invocation/READY observation through the pure fixed-unit contract. Installation admission remains separate. The attachment's AF_UNIX/SCM_RIGHTS transport is Unix-family (Linux and macOS, `attachment_unix.go`, #852); per-frame peer-credential authentication is a capability port at that boundary — Linux `SO_PEERCRED`/`SO_PASSCRED`/`SCM_CREDENTIALS` authenticates every frame (`peercred_linux.go`), macOS exposes only connect-time `LOCAL_PEERCRED` and refuses with an explicit typed unavailable outcome (`peercred_darwin.go`), and non-Unix platforms refuse the attachment (`attachment_refusal_other.go`). A missing required protection is never a weaker accepting path (ADR-0129). The launch orchestration is a portable skeleton (`activate.go`, #866) over capability symbols: the Linux adapter implements them through the genuine systemd/cgroup activation, artifact and original-invocation mechanism (`manager_linux.go` and the bounded `*_linux.go` set); the macOS adapter (`manager_darwin.go`) is an explicit typed refusal documenting the missing G1–G6 confinement and lifetime guarantees (staging, not a new confinement profile); the non-Unix adapter (`native_refusal_other.go`) refuses likewise. Both refuse at the first parent-service verification, before any artifact, activation or cleanup pin. `Instance.Cgroup` is the current Linux spelling of a cleanup-scope identity fact; the platform-neutral guarantee-facts factoring is recorded as a deferred shape review (#866) that keeps every Linux property check and the cgroup ENODEV removal proof byte-identical. | `internal/platform/systemd/observation` for detached typed values; `internal/domain/endpoint/unit` for the expected fixed socket contract; `internal/application/streamqualification` through `stream_activation_installed_linux_test.go` only; `internal/canonicaldigest`, `internal/pathcheck`, standard library; `golang.org/x/sys/unix` only in `peercred_darwin_test.go` (macOS peer-credential refusal evidence, #852) |
| `internal/domain/execution/runtime` | `package runtime` | Class: domain. Common portable composition consumes exact local Session, joins unsupported native refusal without a qualified handle or participant effects, serializes process-wide fixed activation inventory, claim and pin cleanup before Text INIT, independently reobserve original artifact/invocation after READY, create fresh private byte Grant and close/join it before returning separate completed-current preparation provenance. Failed ambiguous cleanup retains the inventory gate. Genuine consumers: `ardents-next execution prepare-permission`, `execution route-holder` and the independently admitted `endpoint start-installed` Source/permission bootstrap, retaining real Stock and signed Network/Route composition. Live Invocation lends one sealed operation, interrupts before borrower join, retains first physical failure synchronously and joins original worker cleanup before exact completed-current handoff. Bounded actual-manager installed Source/permission startup has a genuine command consumer. Qualified Reader and selected-snapshot Publisher retain Text's sole protocol reader and transfer one authenticated Connection stream under the original operation; pending Reader output remains private until Source, operation and original worker join with completed-current checking. A private worker protocol parameterizes the one fixed launch flow (gate/artifact/Job/Grant/lease/joined cleanup) over the closed Text inventory and the trusted Stream qualification inventory; no caller-visible selector exists. Trusted LaunchStreamQualification/RunStreamConnections admit the fixed Stream qualification worker (INIT/READY, installed reobservation) and run the fixed qualification bridge over the retained attachment inside the original invocation through an authority-rechecking borrow, one transfer attempt per invocation, joining the attachment and every supplied authenticated Service stream; no attachment is exported and ordinary Text one-use claims are unchanged. The architecture gate restricts the qualification seams to the trusted qualification composition. The complete installed NET-14 qualification runner remains separate. The portable byte-stream exchange (`byte_stream.go`) composes the Linux launch/protocol (`byte_stream_linux.go`) and the non-Linux `LaunchByteStreamPublisher`/`LaunchByteStreamReader` refusal stubs (`byte_stream_other.go`, #866), closing the compile gap flagged by the Cohort 2b endpoint boundary review (PR #860) without moving the endpoint boundary. The authority-rechecking borrow shared by the Reader and Stream qualification bridges is Unix-family (`worker_borrow_unix.go`, Linux and macOS, #866); the Windows refusal Attachment exposes no Write, so the borrow stays Unix-family until that refusal surface is completed. | `internal/domain/execution`, `internal/domain/execution/worker`; `internal/application/document` through `document_init_linux.go`, `document_publisher_linux.go`, its Linux behavior test, `document_reader_linux.go`, `byte_stream.go` and `byte_stream_linux.go` only; `internal/application/connection` through `document_publisher_linux.go`, `document_publisher_handoff.go`, `document_reader_linux.go` and `document_reader_handoff.go` only; `internal/application/streamqualification` through `stream_qualification_linux.go`, `stream_qualification_other.go` and `stream_qualification_installed_linux_test.go` only; standard library ; `internal/diagnostics/measurement`, `internal/diagnostics/runtimeevent` for the registered finite measurement producers/consumers |
| `internal/domain/enrollment` | `package enrollment` | Class: domain. Authenticate an independently pinned portable v6 Endpoint bundle, exact original program and optional complete protected group; retain private copied byte provenance. Root-contained bounded actual reads and inventory, original cancellation and justified native ownership mechanisms. Read-only `Bundle.MetadataNames` classifies metadata outside trusted-root/static companion inventory using the same grammar identities. Separate `ReadCandidate` retains an opaque untrusted snapshot without initial-pin or running-program provenance; its declarations and byte copies grant no successor authority. Both results use one private bounded inventory and projection implementation; the genuine fresh-pair consumer is `cmd/ardents-next/installation.go`. `ValidateProtectedGeneration` supplies the same canonical static-group grammar to new Installation inspection without pin provenance or filesystem/Release authority. Linux amd64 infrastructure custody authenticates the independent pin before canonical parse, retains the exact original files and verifies the executable through `/proc/self/exe` for the pinned Bundle. Its distinct Candidate carries no first-pin or running-program provenance. `ExecutableArtifactName` owns the same platform artifact identity used internally and by the two read-only packaging operations in `cmd/ardents-next/enrollment.go`. No Release inputs/URLs, signing, installation, execution, Network or old runtime authority. Actual consumers: `cmd/ardents-next/enrollment.go`, `cmd/ardents-next/release.go` and `cmd/ardents-next/installation.go`; the latter consumes authenticated facts, metadata projection and frozen bytes. | `internal/domain/enrollment/infrastructureinventory`, `internal/canonicaldigest`, `internal/pathcheck`, standard library ; `internal/diagnostics/measurement`, `internal/diagnostics/runtimeevent` for the registered finite measurement producers/consumers |
| `internal/domain/enrollment/infrastructureinventory` | `package infrastructureinventory` | Class: domain. Owns the portable seven-file canonical Infrastructure Inventory, including role companions, size bounds, declarations and digests. Native Enrollment parses the document after checking the pin for an initial Bundle. Release offline preparation uses the same companion bounds and constructs the document from original bytes. `Validate` retains the reviewed whole-image verification from the retired selected-generation consumers (see the deadcode allowlist). This package confers no pin, custody, Candidate, Bundle or running-program provenance. | `internal/canonicaldigest`, standard library. |
| `internal/domain/route` | `package route` | Class: domain. Own exact role-purpose eligibility, participant conflicts and retained Interior pair rules; grant no Network, token or physical authority. | standard library |
| `internal/domain/route/selection` | `package selection` | Class: domain. Own independent exclusive installation Entry and local Interior roots, canonical committed pairs, context-local Rendezvous choices and bounded alternative, original retention and time floors; consume coherent Network observations and retain exact duties without redraw on failure. Endpoint first preparation uses `InitializeInstallation` for empty Entry/role histories; funding uses `OpenRetainedInstallation` and retained role borrows, which cannot initialize lost history. Portable Entry-pair eligibility/draw/bounds, retained Leg checks and recipient rules compose with the durable selection owner and root lease/permission mechanisms; all are Unix-portable (Flock, O_NOFOLLOW, Stat_t.Uid) with no Linux-only dependency (ADR-0125 portable core). The Windows owner-only floor enforces the shared protected owner-only DACL through `internal/durablefile` (#793), because Go synthesizes Windows mode bits from FILE_ATTRIBUTE_READONLY rather than the DACL; the Unix floor keeps its read-only permission-bit and endpoint-UID checks unchanged. The closed Entry set and local Interior set share one parameterized generation-store mechanism (`generation_store.go`, R2 #764) over the shared `internal/durablefile` durable-file leaf; each keeps its own marker, schema, validator, root lease and refusal prose, and the Entry lease enforces the same strict regular-file/single-link identity as the duty and State durable owners (G1 #764). | `internal/domain/route`, `internal/domain/network`, `internal/durablefile`, standard library; `golang.org/x/sys/windows` only in `root_permissions_windows_test.go` |
| `internal/domain/route/ardp` | `package ardp` | Class: domain. Own canonical generation-three bounded frame/HELLO, OPEN recipient/envelope encoding JOIN and Descriptor lookup/publication operation/result grammar; reject impossible headers before body reads or allocation. Decoded OPEN fields grant no current duty, role or deadline authority; channel and operation state own permitted effects. | standard library |
| `internal/domain/route/transport` | `package transport` | Class: domain. Own portable ordered-stream/exporter and Node/role request/shared-listener contracts, bounded TLS record writes through original physical capacity projections, exact accepted Carrier/ALPN identities, literal endpoint/pre-effect bounds, shared certificate/TLS authentication, the one closed TLS 1.3 key-exchange group list that the Service TLS owners borrow, and direct/Node classification. Peer-handshake and physical-retirement categorization retain original causes and cannot hide an unrelated aggregate leaf. A separate unstarted-write marker retains failed output provenance only; raw/mixed closed errors grant neither that marker nor peer classification. Actual physical adapters consume these same requirements. No concrete adapter, channel framing, Network/Admission or durable-root authority. | standard library |
| `internal/domain/route/transport/tls` | `package tls` | Class: domain. Own actual literal TCP role/Node dial, inner role TLS client/server handshake over selected lanes, shared direct/Node TCP listener, finite handshake capacity, absolute bounds, deadline reset and physical Node socket interruption. Original native socket-write admission beneath TLS distinguishes wholly unstarted Node output from partial/started physical failure; neither refusal nor zero output grants success. Shared transport owns requests/authentication/classification; supplied raw connections remain the caller's ownership on refusal, while failed dials close the adapter's own socket. No path, token, durable-root or QUIC authority. | `internal/domain/route/transport`, standard library |
| `internal/domain/route/channel` | `package channel` | Class: domain. Own portable framing: sole reader, serialized writer, bounded principal memory, lanes, scheduling, credit/debits, parent exchanges and joined physical retirement with retained original failures. Projects only its own immutable frame overhead for nested TLS record sizing; the projection grants no live credit or authority. The selected writer publishes actual physical counts/failure before becoming inactive; nested output separately retains real lower payload progress and actual lower physical failures, so successful lower bytes followed by retirement remain failed upper work without inventing cleanup failure. Consumes selected transport and caller currentness; no concrete adapter, Network/Admission/Hosting decisions or durable root. Real prefix, receiver and JOIN callers consume this mechanism. Also owns `MinDeadline`, the one shared pure setup/cleanup bound for prefix, pairing and receiver: the earlier original deadline wins and equal deadlines keep the first argument's exact reading; it grants no deadline authority. | `internal/domain/route/ardp`, `internal/domain/route/transport`, standard library ; `internal/diagnostics/measurement`, `internal/diagnostics/runtimeevent` for the registered finite measurement producers/consumers |
| `internal/domain/route/bootstrap` | `package bootstrap` | Class: domain. Own one duty's finite issuer-bootstrap live claims, per-physical-adjacency cap, original 10-second bounds, shared queued-work budget and aggregate output credit. Receiving Node children reserve before TLS and transfer capacity return through the actual lane finish. No Network authority, Admission permission/quota/signing, token stock or ordinary channel upgrade. | `internal/domain/route/ardp`, `internal/domain/route/channel`, standard library |
| `internal/domain/route/issuer` | `package issuer` | Class: domain. Own portable single-terminal request/result exchange, nonce/outcome agreement and role TLS termination for actual Prefix and Receiver callers. No listener, selection, Stock completion, token spend/quota/signing, physical queue/reservation or root authority. | `internal/domain/admission`, `internal/domain/route/ardp`, standard library |
| `internal/domain/route/role` | `package role` | Class: domain. Own portable original-duty/profile reobservation, exact HELLO purpose/recipient/bounds, negotiated role exporter binding and holder HELLO/Stock-presentation/ADMIT/ACCEPT ordering with original-caller checks. Admission supplies the actual durable presentation; receiving owns Grant transfer and joined physical retirement. No intake authentication, replacement selection, spending, Hosting reservation or root. | `internal/domain/network`, `internal/domain/admission`, `internal/domain/route`, `internal/domain/route/ardp`, `internal/domain/route/channel`, `internal/domain/route/transport`, standard library |
| `internal/domain/route/join` | `package join` | Class: domain. Own exact Source/Responder acquisitions, holder attempt/Context and Joined stream over opaque Prefix contracts. Prefix owns generations; native selection owns roots. The separately receiving Rendezvous matching, two-RESULT barrier and relay accounting/credit live in the `join/pairing` subpackage, which Receiver consumes directly without the holder's Prefix/selection imports. No token spend, listener or Service authority. | `internal/domain/route/prefix`, `internal/domain/route`, `internal/domain/route/selection`, `internal/domain/route/ardp`, `internal/domain/route/channel`, `internal/domain/route/role`, `internal/domain/route/transport`, `internal/domain/network`, `internal/domain/admission`, standard library |
| `internal/domain/route/join/pairing` | `package pairing` | Class: domain. Own one exact receiving JOIN duty's Rendezvous matching: two complete RESULT writes before data, relay accounting/credit and joined physical retirement of both original sides. Capacity reserves one side's bounded frame memory from the receiving principal's shared budget before irreversible admission spend and releases it exactly once after Serve joins. Receiver supplies admitted channels and retains actual Grant/Hosting return until Serve completes; pairing neither verifies nor releases an Admission Grant. No holder acquisition/generation, durable root, selection, listener, token spend or Service authority. Real consumer: `internal/domain/route/receiver` through `receiver_unix.go` and `admission_retirement_unix.go`. | `internal/domain/route/ardp`, `internal/domain/route/channel`, `internal/domain/route/role`, standard library |
| `internal/domain/route/prefix` | `package prefix` | Class: domain. Own portable exact Entry/Interior physical generations, pending setup and sealed borrows, original Source/Responder pair commits, authenticated terminal openings, separate finite issuer-only bootstrap, exact-generation issuer and Descriptor exchanges, parent refill and joined resource return. Reachability owns proof/history completion after the original physical exchange joins. Actual Registration/JOIN consumers retain opaque claims; no listener, durable root, token spend or Hosting budget. | `internal/domain/network/state` for the opaque original RuntimeWitness refusal check; `internal/domain/route`, `internal/domain/route/selection`, `internal/domain/route/ardp`, `internal/domain/route/issuer`, `internal/domain/route/bootstrap`, `internal/domain/route/channel`, `internal/domain/route/role`, `internal/domain/route/transport`, `internal/domain/route/transport/tls`, `internal/domain/network`, `internal/domain/admission`; `internal/domain/reachability` for Descriptor exchange; standard library |
| `internal/domain/route/receiver` | `package receiver` | Class: domain. Own actual listener, exact-purpose dispatch to Introduction Registry, JOIN Pairing, genuine issuer composition and separately owned Descriptor Store, directed Node Carrier borrowing and forwarding children, receiving Admission composition and transferred reservations through joined physical retirement. Recheck the current Introduction assignment before Descriptor effects/output; Store owns proof/conflict/durability decisions. No holder operation/generation, token quota/spend policy or Hosting budget. | `internal/domain/route`, `internal/domain/route/transport`, `internal/domain/route/transport/tls`, `internal/domain/route/channel`, `internal/domain/route/bootstrap`, `internal/domain/route/role`, `internal/domain/route/ardp`, `internal/domain/route/issuer`, `internal/domain/route/introduction`, `internal/domain/route/join/pairing`, `internal/domain/network`, `internal/domain/admission`, `internal/domain/admission/receiving`; `internal/domain/reachability` for the receiving Descriptor Store; standard library; `internal/domain/reachability/store` at the existing verified caller. |
| `internal/domain/network` | `package network` | Class: domain. Own Candidate View eligibility, rejection precedence, collision exclusion, family totals and deterministic Role Domain assignment; own the accepted current/pending Epoch relation, conflict refusal, exact successor selection and completion-time validity; bind signed role assertions to exact authenticated Node Records and expose copied members with unambiguous identity/key lookup and bounded validity. Evaluate every authenticated observation before choosing a generation. Own separate local-participation restriction decisions, producer-scoped replacement, live Source retention, identity/family collisions and finite record/producer/exposure bounds; own finite Acquisition attempts, recovery, original deadlines/order, exposure history and backoff inside the State commit boundary; own profile history, trusted-time confidence, coherent AcceptedState observations with copied public issuer inventory and exact issuer/duty binding, and retained-duty binding. Proposals do not publish authority; the State application adapter verifies inputs and commits effects. | standard library |
| `internal/domain/network/canonicalcursor` | `package canonicalcursor` | Class: supporting mechanism. Network-domain pure bounded cursor for the canonical Network byte grammars (R5, #804): bounded reads over one caller-supplied canonical byte slice, big-endian uint16/uint32/uint64/int64 readers, the exact consumed-offset observation and the single "truncated canonical bytes" bounds refusal formerly repeated byte-identically by every consumer. Reads alias the caller's bytes and never copy. It is a Network-domain mechanism leaf, not a grammar or authority owner: every consumer is a network package, so the cursor lives inside `internal/domain/network` rather than as a top-level leaf (Phase 4, #834); it holds no root, marker, schema, authority or lifecycle of its own, and each consumer keeps its distinct magic, field order, validation bounds and refusal prose. Epoch's canonical text grammar (length byte, UTF-8, printing range) stays in `internal/domain/network/epoch` (`text.go`) over this cursor. Real consumers: `internal/domain/network/epoch` (Record, Epoch and materialization grammars), `internal/domain/network/closedprofile` (signed-profile body), `internal/domain/network/state/internal/distribution` (distribution journal) and `internal/domain/network/source` (response bundle framing). It grants no installed restart or native qualification. | standard library |
| `internal/domain/network/duty` | `package duty` | Class: domain. Coordinate the bounded, owner-only, durable installation-local participation generations, watermark, expiry, owner-held `direct-source/live` guards, and current conflict truth. Network supplies producer-scoped restriction replacement, conflict, expiry, classification and bounds; this adapter owns the root, canonical journal, commit and recovery lifecycle. A live Direct Source remains effective until its producer replaces or removes it; time-held exposure still expires, and another producer cannot prune the live guard. The root schema is version 2 without the spend field: ADR-0107 retired the historical receiving one-use Transit Grant spend ledger by a bounded in-place conversion, so a strictly validated version-1 generation loads as version 2 with its spend records dropped while conflict duties, generation continuity, and the watermark survive, and every committed generation is version 2. Short Source-write and conflict-read operations acquire the same exclusive lease with a cancellable one-second wait; retained owners keep non-waiting Open. Successful acquisition verifies the current generation, and a busy or invalid root remains unavailable. Independent Linux read-only Source and stopped-Node inspections retain original native identities and existing leases, with distinct exposure-versus-zero-producer policies and joined physical completion; neither initializes, repairs or prunes history. It retains no issuer root, private generation, public profile, budget, request reservation, or signing operation; it owns no profile/request grammar, Route, Target, Service, or participant identity. The durable-file mechanism (bounded read, staged replacement, pruning, watermark codec, root inspection, directory sync, the strict-identity Unix root lease and the Windows owner-only root DACL enforcement, #793) is the shared `internal/durablefile` leaf (R1, #764); duty keeps its own marker, lock, schema, generation immutability pre-check and refusal prose. | `internal/domain/network` for participation policy, `internal/durablefile`, `golang.org/x/sys/windows` only in `root_lease_windows.go` and `root_permissions_windows_test.go`, `internal/pathcheck`, standard library |
| `internal/domain/network/source` | `package source` | Class: domain. Own the finite Direct-Origin Source plan, credential binding, private TLS transport, network-scoped request digest and bounded response-bundle framing, material selector, ordering, and exposure identity. The pure local serving-declaration byte grammar lives in the `internal/domain/network/source/servingdeclaration` subpackage (AR-N-07a, #691); it returns declaration facts only, opens no runtime and grants no authority. `ObservePublicTrust` owns the offline public X.509 observation for trust preparation. The transitional `ObservePublicContact` installation-input observation is retired (#604 stage 1.6, ADR-0125). It does not verify Epoch authority or admit State. | `internal/domain/network/canonicalcursor` for bounded bundle reads, `internal/domain/network/epoch` for member bounds, standard library ; `internal/diagnostics/measurement`, `internal/diagnostics/runtimeevent` for the registered finite measurement producers/consumers |
| `internal/domain/network/source/inspection` | `package inspection` | Class: domain. Retain and reobserve original bounded local inputs and the protected client key through one authenticated Source contact. The Linux Source CLI decodes arguments and renders the unaccepted result. The contact composes the Linux-only State source preparation and stays Linux; every other platform receives the same explicit typed unavailable result (ADR-0129, #856). | `internal/domain/network/source`, `internal/domain/network/source/servingdeclaration`, `internal/domain/network/state`, `internal/platform/artifacts`, `internal/platform/processguard`, standard library. |
| `internal/domain/network/source/serving` | `package serving` | Class: domain. Owns Direct-Origin Source operational input custody and the original admitted process lifetime, through joined State, native observations and retained completion. Serving classifies bounded first-failure and joined cleanup owners. The command selects fresh or retained State and encodes those facts in the CLI report. The operation runs on the Unix family (Linux and macOS) on the cleared Cohort 0/1 seams; every other platform receives the same explicit typed refusal (ADR-0129, #856). | `internal/domain/network/source/servingdeclaration`, `internal/domain/network/state`, `internal/pathcheck`, `internal/platform/artifacts`, `internal/platform/processguard`, standard library. |
| `internal/domain/network/source/servingdeclaration` | `package servingdeclaration` | Class: domain. Own the pure closed Source serving-declaration byte grammar (`serving_declaration.go`): exact field names, duplicate/null refusal, one complete object, exact 32-byte array shapes and one finite bounded input, mirroring the original node-role declaration codec in `internal/domain/node` (`plan_declaration.go`, #673). It returns declaration facts only and opens no resource: no State acceptance, root creation, signing, runtime or process policy. Extracted from `internal/domain/network/source` as AR-N-07a (#691) with unchanged canonical declaration bytes. Real consumers: `cmd/ardents-next/source_input_linux.go`, `internal/domain/network/source/command/prepare_linux.go` (prepare-state), `internal/domain/network/source/command/inspect_linux.go` (inspect-latest) and State's read-only `source_inspection_linux.go`; the closed-grammar behavior oracle is `serving_declaration_linux_test.go`. | standard library |
| `internal/domain/network/epoch` | `package epoch` | Class: domain. Authenticate bounded Epoch/Record/View grammar, authority signatures, commitments and materializations from an explicit policy. Project decoded input facts and authentication results into Network's Candidate View policy; compare the returned disposition and summaries to signed commitments. Network is the sole production owner of eligibility, collisions, family totals and deterministic Role Domain assignment. Return one owned verified Decision; `Inspect` exposes only a bounded untrusted Header for exact-byte reuse and chain loading, and `InspectMaterializationIndex` reads an untrusted bounded prefix before the full proof check. Called by State. `ardents-next network` and the bounded `ardents-control` provisioning commands consume `PrepareInitialClosedRecord`/`PrepareInitialClosedEpoch` for unsigned fresh closed Node Records and initial Epoch commitments. Preparation shares Network evaluation and grants no signature or State acceptance (ADR-0120). | `internal/domain/network/canonicalcursor` for bounded grammar reads, `internal/domain/network` for candidate evaluation and assignment, `tests/epochfixture/assignment` and `tests/epochfixture/network` in behavior tests, standard library |
| `internal/domain/network/closedprofile` | `package closedprofile` | Class: domain. Own only the bounded ARDCPR03 signed-profile grammar and canonical unsigned preparation, validated fixed-purpose public signing transcript and verification against an explicit caller-supplied State context. It validates exact RSA-PSS SPKI through the Admission issuerprofile leaf. Return owned parsed public facts; State alone joins them to verified Epoch candidates, decides durable acceptance/conflict, and exposes runtime views. Unsigned preparation is an explicit subsequent provisioning contract; private signing remains outside Network. New State consumes verification. | `internal/domain/network/canonicalcursor` for bounded body reads, `internal/domain/admission/issuerprofile` for token SPKI decoding, standard library |
| `internal/domain/network/state` | `package state` | Class: domain. Own selected closed-profile intake, authenticated history recovery, finite acquisition and serving, ordered durable publication, clock observation and joined stop/close. `CurrentRuntimeWitness` additionally retains a synchronous original-owner refusal projection without granting authority; `CurrentRuntime` returns one opaque checked observation with copied public profile and exact membership; `AcceptClosedProfile` returns only a committed identity receipt. `Current` is non-authorizing intake/acquisition diagnostics with the local Source exclusion identity. Process admission is borrowed through `PermitWork`; no process-resource monitor, Node lifecycle projection, role/Purpose selection, private signing, token debit, reservation or Carrier execution belongs here. Separate Linux NodeInspection authenticates retained signed/time/security and accepted-profile/member history plus zero-producer participation, returning only copied binding facts without runtime rights. Product tests use genuine signed closed-v3 inputs and independent canonical builders. | `internal/domain/network/state/internal/distribution`, `internal/canonicaldigest`, `internal/domain/network/closedprofile`, `internal/domain/network/duty`, `internal/domain/network/epoch`, `internal/domain/network/source`, `internal/domain/network/source/servingdeclaration`, `internal/domain/network/state/durable`; `tests/epochfixture/network` in behavior tests; `internal/domain/network` for retained State policy; `internal/platform/systemd`, `internal/platform/cgroup` and `internal/platform/process` for native repair, `internal/platform/systemd/observation` for typed repair facts in the exact files listed above, `internal/domain/reachability/store` for bounded existing-only Descriptor Store uncertain-stage recovery; `internal/pathcheck`, standard library; `internal/platform/artifacts`, `internal/platform/process` and `internal/platform/processguard` for the finite adapter in `source_state_files_linux.go`. Its owner test also uses `internal/platform/artifacts`. ; `internal/diagnostics/measurement`, `internal/diagnostics/runtimeevent` for the registered finite measurement producers/consumers |
| `internal/domain/network/state/internal/distribution` | `package distribution` | Class: supporting mechanism. Validates and emits the existing ARDS1D4 distribution journal and computes the canonical record digest. State is its sole runtime consumer and retains authority, history admission and commit. | `internal/domain/network/canonicalcursor`, standard library. |
| `internal/domain/network/state/command` | `package command` | Class: composition. Accepts the local offline Network Epoch/profile declaration or retained inspection, opens the original State and joins it before its bounded receipt. The CLI in cmd/ardents-next/network.go consumes it. | `internal/domain/network`, `internal/domain/network/state`, `internal/platform/artifacts`, standard library; test-only `tests/epochfixture/network` for genuine independent signed Epoch bytes. |
| `internal/domain/network/state/durable` | `package durable` | Class: domain. Own the exclusive State-root lease, bounded opaque immutable Epoch/input generations, distribution journal, closed-profile bytes and physical state, atomic pointer replacement, fsync and recovery staging cleanup. Independent Linux Inspection retains existing read-only native history/lease without mutation or role policy; Source and Node semantic consumers verify its bytes separately. Accept exact framing limits from State; never verify Epoch authority, select Source results, decide profile conflicts, or publish a runtime View. The bounded read, directory sync, generation-name grammar and strict-identity Unix root lease open are the shared `internal/durablefile` leaf (R1, #764); State keeps its own marker, lock, wrapped lease release, pointer durability boundary and refusal prose. | `internal/durablefile`, `golang.org/x/sys/windows` on Windows, `internal/pathcheck`, standard library |
| `internal/domain/admission` | `package admission` | Class: domain. Own canonical Permission and holder request grammar, token classes and validation of externally supplied authority facts. Child packages own allocation, quota, issuance, stock and spending transitions; no Network authority. | `internal/domain/admission/issuerprofile`, standard library |
| `internal/domain/admission/console` | `package console` | Class: command mechanism. Owns bounded JSON-line conversations, inherited file descriptors, interruption and joined close. The existing CLI and Endpoint holder and receiver supply actual operations. Portable conversation, Unix descriptor duplication, Windows refusal before owner effects. | standard library |
| `internal/domain/admission/issuerprofile` | `package issuerprofile` | Class: domain. Own the canonical public issuer profile, complete class/hour inventory and RSA-PSS SPKI grammar; mint opaque purpose-bound signature requests and verified public profiles. PublicPins/Inspect authenticate independently selected public identities without clock acceptance; Verify additionally fixes the exact interval. The genuine public Control consumer reads only signed public bytes. No storage, private key, quota or State authority. | standard library |
| `cmd/ardents-next` | `package main` | Class: composition. Adapt holder DTOs/results through `endpoint/session`, which owns retained Prefix/JOIN/registration/history; dispatch independent receiving composition through `internal/domain/node/receive`; adapt selected Network epoch/profile intake and inspection through `network.go`; project the read-only diagnostic timeline through `diagnostic_timeline.go` (`internal/diagnostics/timeline`) and request local withdrawal through `endpoint_administration_client.go` (`internal/application/administration`); adapt Admission holder, issuer and receiver inputs to the owners of live Network observations with separate roots and no fallback; adapt standalone Admission holder, receiver, unsigned allocation and current-issuer operations, Admission ledger initialization, issuer key initialize/inspect/export and confirmed token initialize-results/issue and Hosting inputs to independent inspection and durable budget initialize/observe, Node identity import, issuer profile initialization/inspection and Admission binding preparation, finite classified JSON results and explicitly local bounded OTLP export. | standard library; `internal/canonicaldigest`, `internal/diagnostics/telemetry`, `internal/diagnostics/timeline`, `internal/application/administration`, `internal/domain/endpoint/funding`, `internal/domain/endpoint/publisher`, `internal/domain/network/state`, `internal/domain/network/state/command`, `internal/domain/endpoint/command`, `internal/domain/network/closedprofile`, `internal/domain/admission`, `internal/domain/admission/issuerprofile`, `internal/domain/hosting`, `internal/domain/admission/operator`, `internal/domain/admission/issuer`, `internal/domain/admission/quota`, `internal/domain/admission/receiving`, `internal/domain/admission/allocation`, `internal/domain/node/receive`, `internal/domain/enrollment`, `internal/domain/release/endpoint`, `internal/domain/execution/runtime`, `internal/domain/publication/durable`, `internal/domain/publication/instance`, `internal/application/document`, `internal/application/connection`, `internal/domain/release/infrastructure`, `internal/domain/endpoint/session`, `internal/platform/launch`, `internal/platform/artifacts`, `internal/domain/node/provisioning`, `internal/domain/release/infrastructure/signing`, `internal/domain/release/infrastructure/launch`, `internal/domain/network/source/command`, `internal/domain/node/command` ; `internal/diagnostics/runtimeevent` for the finite local observation catalogue and classification ; `internal/diagnostics/measurement` for the registered finite measurement producers/consumers; `internal/domain/node/provisioning/runtimeplan` for node prepare-plan; `internal/domain/node/provisioning/infrastructure` for fixed offline role provisioning.; `internal/domain/admission/console` for bounded inherited Admission console descriptors in `admission_console.go`. |
| `internal/domain/network/source/command` | `package command` | Class: domain. Portable Source CLI boundary with Linux implementations and typed platform refusal for serve, inspect-latest, prepare-trust and prepare-state. Retained repair uses the same serving report projection through the release launch owner. | `internal/domain/endpoint` and `internal/pathcheck` in `prepare_linux.go`; `internal/domain/network/state`, `internal/platform/artifacts`, `internal/domain/network/source/serving`, `internal/domain/network/source/inspection`, `internal/canonicaldigest`, standard library. ; test-only `internal/domain/network/state`, `internal/domain/network/duty`, `internal/domain/network/source`, `internal/domain/network/source/servingdeclaration`, `tests/epochfixture/network` and `tests/e2e/source` for real State, output and original input custody tests ; `tests/networkfixture` in `state_lifetime_linux_test.go` |
| `cmd/ardents-custody` | `package main` | Class: composition. Start the separate custody process with its signal context and original terminal streams. | `internal/domain/custody/command`, standard library ; `internal/diagnostics/measurement` for the registered finite measurement producers/consumers |
| `internal/domain/custody/command` | `package command` | Class: domain. Run the separate custody-process adapter. `create-service-authority` generates a Service Authority inside one encrypted active record; `issue-service-credential` requires the exact independently transferred host request digest before Vault/password access, advances only a bounded request, and renders its receipt only after same-directory staged no-replace publication or exact reopen of the public response. `create-admission-authority` creates the separate closed admission root; `issue-admission-permission` independently confirms the holder request digest, writes its 228-byte permission only to an owner-only no-replace destination, and emits only a digest receipt. `verify-record` consumes a password only through an interactive no-echo terminal; `export-recovery-bundle` and `restore-recovery-bundle` expose respectively a distinctly passworded test-restored Bundle and an authority-locked quarantine restore; `purge-record` deletes one exact verified encrypted record only after terminal confirmation while retaining its Authority floor. It never accepts Authority material, permission body, or a password through arguments, environment, configuration, or shared application stdin; it does not activate a recovered Authority. | `internal/domain/custody` for the command adapters and their tests; `internal/domain/admission` and `internal/domain/publication` in behavior tests; `github.com/aymanbagabas/go-pty` only in `terminal_secret_linux_test.go`; `golang.org/x/term`; `golang.org/x/sys/windows` only in `terminal_line_windows.go`; `golang.org/x/sys/unix` only in `terminal_line_linux.go`, standard library ; `internal/diagnostics/runtimeevent` for the finite local observation catalogue and classification |
| `internal/platform/artifacts` | `package artifacts` | Class: platform adapter. Read bounded standalone files through nonblocking native opens. Retain original Unix-family descriptors and custody of one bounded offline input and output lifetime; the observed-input and bounded-report seams are Unix-generic (Linux and macOS), while the ext4 storage floor, namespace output and protected-input policy stay Linux. Own exact path identity, root-controlled input and secret buffer retirement, directory reobservation, publication, exact private-file retries and checked readback, and the original configuration inputs and executable identity used by Endpoint and infrastructure delivery. Portable typed declaration decoding checks exact field spellings, duplicates and fixed-array lengths. The package also owns operator artifact codecs, canonical Ed25519 PKCS#8 encoding and admission, and bounded native report writes. Consumers choose the private key and signing policy. | `internal/durablefile`, `internal/pathcheck`, Standard library |
| `internal/domain/node/provisioning` | `package provisioning` | Class: composition. Dispatches offline Network preparation and signing. Creates one fresh State Ed25519 key and the exact finite declared number of distinct Node keys with exclusive protected output and public receipts. | standard library; `internal/canonicaldigest`; `internal/domain/node/provisioning/networkdraft`; `internal/domain/node/provisioning/networkpreparation`; `internal/domain/node/provisioning/networksigning`; `internal/pathcheck`; `internal/platform/artifacts`; test-only: `internal/domain/network/source`; `internal/domain/network/source/servingdeclaration`; `tests/epochfixture/network`; `tests/networkfixture`; file-specific imports are enforced by offline operator isolation. |
| `internal/domain/node/provisioning/infrastructure` | `package infrastructure` | Class: composition. Runs the fixed-role offline infrastructure provision command. Node role operations retain independent purpose, schema, native input/output and keys. Source uses the separate genesis path. Unsupported hosts refuse before effects. | standard library; `internal/canonicaldigest`; `internal/domain/admission/issuance`; `internal/domain/admission/issuerprofile`; `internal/domain/network`; `internal/domain/network/closedprofile`; `internal/domain/network/epoch`; `internal/domain/network/source`; `internal/domain/node/provisioning/networksigning`; `internal/domain/node/provisioning/sourcegenesis`; `internal/platform/artifacts`; test-only: `internal/domain/admission`; `internal/domain/admission/issuer`; `internal/domain/admission/quota`; `internal/domain/admission/receiving`; `internal/domain/admission/stock`; `internal/domain/admission/token`; `internal/domain/custody`; `internal/domain/network/state`; `tests/networkfixture`; file-specific imports are enforced by offline operator isolation. |
| `internal/domain/node/provisioning/networkpreparation` | `package networkpreparation` | Class: composition. Admits the original public declaration and publishes one unsigned Network artifact through the Linux ext4 floor. The Network artifact command consumes the joined result. | `internal/domain/node/provisioning/networkdraft`, `internal/platform/artifacts`, `internal/durablefile`, standard library. |
| `internal/domain/node/provisioning/networksigning` | `package networksigning` | Class: composition. Retains original protected inputs, a fixed-purpose Network signing key and exclusive output for offline Record, Epoch, profile and certificate signing. The Network command is its live consumer, and fixed-role certificate provisioning shares its exact certificate profile. | `internal/domain/node/provisioning/networkdraft`, `internal/domain/network/epoch`, `internal/domain/network/closedprofile`, `internal/platform/artifacts`, `internal/canonicaldigest`, standard library; tests only: `tests/networkfixture`, `tests/epochfixture/network`. |
| `internal/domain/node/provisioning/networkdraft` | `package networkdraft` | Class: supporting mechanism. Parses public Record, Epoch and Closed Profile declarations into canonical unsigned bytes and typed public facts. The original preparation and signing operations verify and use them without moving private keys or output custody. | `internal/canonicaldigest`, `internal/domain/network/epoch`, `internal/domain/network/closedprofile`, `internal/platform/artifacts` for declaration validation, standard library. |
| `internal/domain/node/provisioning/sourcegenesis` | `package sourcegenesis` | Class: composition. Offline Source genesis prepares the original plan, distinct keys and one signed Record/Epoch bundle. The Linux infrastructure provision command dispatches. | `internal/canonicaldigest`, `internal/domain/network/epoch`, `internal/domain/network/source`, `internal/platform/artifacts`, standard library. |
| `internal/domain/node/provisioning/runtimeplan` | `package runtimeplan` | Class: composition. Prepares one exact offline Node role declaration. Caller: ardents-next node prepare-plan. Node plan owns the declaration codecs. The command writes a new output file and opens no live authority. | `internal/canonicaldigest`, `internal/domain/admission/issuance`, `internal/domain/hosting`, `internal/domain/network/state`, `internal/domain/node/plan`, `internal/pathcheck`, `internal/platform/artifacts`, `internal/platform/processguard`, standard library. |
| `internal/domain/release/infrastructure/signing` | `package signing` | Class: domain. Produce original initial/forward infrastructure Release artifacts and the separate Source Timestamp operation. Each closed role binds the original program, generation, template and protected key set. | `internal/domain/enrollment`; `internal/platform/artifacts`; `internal/platform/systemd`; `internal/domain/release`; `internal/domain/release/infrastructure`; stdlib and recorded dependencies. Exact file imports are enforced by the offline operator isolation check.; `internal/domain/release/infrastructure/signing/internal/keyset` for key preparation and public receipt. |
| `internal/domain/release/infrastructure/signing/internal/keyset` | `package keyset` | Class: supporting mechanism. Holds protected original Linux publication of the exact five freshly generated Release key files through join and erasure. Returns detached public file inventory for the command receipt. | `internal/platform/artifacts` only for exclusive protected output. Standard library. |
| `tests/e2e/admissioncli` | `package admissioncli` with external `admissioncli_test` cases | Class: engineering check. Compiled offline Admission and Issuance commands, retained roots, console interruption, strict inputs, independent public pins and bounded telemetry. Launches the same `ardents-next` command through the shared build fixture. | `internal/domain/admission`, `internal/domain/admission/issuance`, `internal/domain/admission/issuerprofile`, `internal/domain/admission/issuerprofile/signing`, `internal/domain/admission/quota`, `internal/domain/admission/receiving`, `internal/domain/network/closedprofile`, `internal/domain/network/state`, `tests/e2e/node` for the independent signed Admission request, `tests/e2e/node/processfixture` for compiled process inputs and lifecycle. Standard library, CIRCL blind signature client and OTLP protobuf readers. |
| `tests/e2e/endpoint` | `package endpoint` with external `endpoint_test` cases | Class: engineering check. Native Endpoint qualification driver shared by installed lifecycle scenarios and Endpoint owner tests. Owns signed Release fixture inputs, dedicated UID/GID and immutable resources, staged platform delivery, distinct manager setup and joined guest cleanup. Also owns independent scope, worker, permission, proof and observation fixtures. | `internal/application/administration`, `internal/application/connection`, `internal/domain/admission`, `internal/domain/admission/allocation`, `internal/domain/connection`, `internal/domain/endpoint/plan`, `internal/domain/execution/runtime`, `internal/domain/hosting`, `internal/domain/network`, `internal/domain/network/source`, `internal/domain/network/state`, `internal/domain/publication`, `internal/domain/publication/durable`, `internal/domain/publication/instance`, `internal/domain/reachability`, `internal/domain/reachability/store`, `internal/domain/release/infrastructure/launch`, `internal/platform/cgroup`, `internal/platform/systemd`, `internal/platform/systemd/observation`, `tests/e2e/node/processfixture`, `tests/e2e/source`, `tests/epochfixture/network`, `tests/releasefixture`. Standard library. Existing registered external dependencies. |
| `tests/e2e/endpointcli` | `package endpointcli` with external `endpointcli_test` cases | Class: engineering check. Compiled Endpoint plan/history commands, read-only Enrollment and Release verification, before-effect refusal and Ubuntu package contents. Uses the same artifact environment precedence and compiled command as installed Endpoint scenarios. | `internal/domain/network/source`, `internal/domain/network/state`, `tests/e2e/node/processfixture`, `tests/e2e/source`, `tests/epochfixture/network`, `tests/networkfixture`, `tests/releasefixture`. Standard library. Existing Release verification fixture dependencies. |
| `tests/e2e/endpointinstalled` | `package endpointinstalled` with external `endpointinstalled_test` cases | Class: engineering check. Installed Endpoint pre-start, replacement, migration, NET-32 and Service checkpoints. The manager-pair test checks positive launch and causal refusal in two distinct admitted systemd managers. The participant journey separately checks exchange. | `internal/application/administration`, `internal/application/connection`, `internal/application/document`, `internal/domain/endpoint/plan`, `internal/domain/endpoint/unit`, `internal/domain/enrollment`, `internal/domain/release`, `internal/domain/release/infrastructure/launch`, `tests/e2e/endpoint`, `tests/e2e/node/processfixture`, `tests/e2e/source`, `tests/networkfixture`, `tests/releasefixture`. Standard library. Existing Release verification fixture dependencies. |
| `tests/e2e/node` | `package node` with external `node_test` cases | Class: engineering check. Compiled Node dispatch, prepared/retained State, Network, Hosting, and the standalone Admission cycle and its Hosting integration. Keeps the independent signed offline Admission request, plus process fixtures with other native-role consumers. The Source refresh checks two live TLS observations and refusal of stale authority after a signed successor. On macOS and Windows, unsupported operations refuse before reading private input. | `internal/domain/admission`, `internal/domain/admission/issuance`, `internal/domain/admission/issuer`, `internal/domain/admission/issuerprofile`, `internal/domain/admission/quota`, `internal/domain/admission/receiving`, `internal/domain/admission/stock`, `internal/domain/admission/token`, `internal/domain/endpoint`, `internal/domain/endpoint/funding`, `internal/domain/endpoint/session`, `internal/domain/enrollment`, `internal/domain/hosting`, `internal/domain/network`, `internal/domain/network/closedprofile`, `internal/domain/network/duty`, `internal/domain/network/epoch`, `internal/domain/network/source`, `internal/domain/network/source/serving`, `internal/domain/network/source/servingdeclaration`, `internal/domain/network/state`, `internal/domain/reachability`, `internal/domain/reachability/store`, `internal/domain/release`, `internal/domain/release/infrastructure`, `internal/platform/artifacts`, `internal/platform/processguard`, `internal/platform/systemd`, `tests/e2e/node/processfixture`, `tests/e2e/source`, `tests/epochfixture/network`, `tests/networkfixture`, `tests/releasefixture`. Standard library. Existing OTLP protobuf readers and Release fixture dependencies. |
| `tests/e2e/nodeinstalled` | `package nodeinstalled` with external `nodeinstalled_test` cases | Class: engineering check. Installed Source, Forwarding, JOIN, Descriptor and Introduction repair drivers. Uses original signed Network and Release fixtures, real host accounts and systemd units and authenticated TCP/TLS role operations and invocation-scoped joined completion. | `internal/domain/admission`, `internal/domain/admission/issuer`, `internal/domain/admission/stock`, `internal/domain/endpoint/funding`, `internal/domain/endpoint/session`, `internal/domain/hosting`, `internal/domain/network/duty`, `internal/domain/network/epoch`, `internal/domain/network/source`, `internal/domain/network/source/servingdeclaration`, `internal/domain/network/state`, `internal/domain/reachability`, `internal/domain/reachability/store`, `internal/domain/release/infrastructure`, `internal/domain/release/infrastructure/launch`, `internal/platform/processguard`, `internal/platform/systemd`, `tests/e2e/node/processfixture`, `tests/e2e/source`, `tests/epochfixture/network`, `tests/networkfixture`, `tests/releasefixture`. Standard library. |
| `tests/e2e/routeprocess` | `package routeprocess` with external `routeprocess_test` cases | Class: engineering check. Compiled Route Bootstrap, Prefix, Registration, replenishment, Descriptor and JOIN, transport refusals and retained spend. Uses signed Network State, actual Admission and Hosting, real receiver processes and the holder console. Exact lifetime and refusal cases call the genuine Endpoint session against the original receiving processes. | `internal/domain/admission`, `internal/domain/admission/quota`, `internal/domain/admission/stock`, `internal/domain/admission/token`, `internal/domain/endpoint/funding`, `internal/domain/endpoint/session`, `internal/domain/hosting`, `internal/domain/network`, `internal/domain/network/epoch`, `internal/domain/network/state`, `internal/domain/reachability`, `internal/domain/reachability/store`, `tests/e2e/node/processfixture`, `tests/epochfixture/network`, `tests/networkfixture`. Standard library. |
| `tests/descriptorfixture` | `package descriptorfixture` | Class: engineering fixture. Independently assembles signed public Descriptor v3 transcripts for the maintained Node process driver and Reachability owner oracles. Fixed vectors verify canonical fixture bytes. | standard library. |
| `tests/e2e/node/processfixture` | `package processfixture` | Class: engineering check. Node process test driver for the package-owned CLI build, pinned process execution, native host state, signed Network and Admission input, and joined cleanup. | `internal/domain/admission`, `internal/domain/admission/allocation`, `internal/domain/admission/issuance`, `internal/domain/admission/issuer`, `internal/domain/admission/quota`, `internal/domain/admission/spending`, `internal/domain/admission/stock`, `internal/domain/admission/token`, `internal/domain/endpoint/funding`, `internal/domain/hosting`, `internal/domain/network`, `internal/domain/network/duty`, `internal/domain/network/epoch`, `internal/domain/network/state`, `tests/epochfixture/network` and standard library. ; `internal/domain/admission/receiving` in `receiving_spend_linux.go` for replaying actual spends after joined reopen.; `tests/descriptorfixture` at the existing verified caller. |
| `tests/e2e/source` | `package source` with external `source_test` cases | Class: engineering check. Compiled Source command checks for authenticated inspection and before-effect host refusal. Native fixtures provide independent public TLS and root-controlled signing workspace inputs and the shared native initial key and certificate signing fixtures. | `internal/domain/network/source`, `internal/domain/network/source/servingdeclaration`, `internal/platform/artifacts`, standard library |
| `tests/e2e/service/fixturecommand/publish-app` | `package main` | Class: engineering check. E2E-only separately granted publication-operator fixture. It uses only the bounded local Service Administration socket and is built solely by the Endpoint recovery process test. | standard library |
| `tests/e2e/service/fixturecommand/stream-app` | `package main` | Class: engineering check. E2E-only paired direct opaque-stream workload used by retained qualification evidence. It owns deterministic bounded bytes and receipts but no Endpoint-local transport grammar. | standard library |
| `tests/epochfixture/assignment` | `package assignment` | Class: engineering check. Independently construct deterministic role-domain assignments for black-box Network State fixtures; never serve maintained product code. | standard library |
| `tests/epochfixture/merkle` | `package merkle` | Class: engineering check. Independently construct canonical Epoch commitment roots and proofs for black-box Network State fixtures; never serve maintained product code. | standard library |
| `tests/networkfixture` | `package networkfixture` | Class: engineering check. Prepare independent signed Network inputs, issued test-key inventory, clock observations and reopened State histories for owner and process tests; no runtime authority. | `internal/domain/admission/issuance`, `internal/domain/admission/issuer`, `internal/domain/admission/quota`, `internal/domain/network/epoch`, `internal/domain/network/state`, `tests/epochfixture/network`, standard library ; test-only `internal/domain/admission`, `internal/domain/admission/allocation`, and `internal/domain/admission/token` in `issuer_conflict_test.go` ; `internal/domain/network` in the signed Admission fixture; `internal/domain/network/closedprofile` only from provisioning_plans.go to decode independently generated profile fixtures. |
| `tests/releasefixture` | `package releasefixture` | Class: engineering check. Construct independent signed Release authority and inventory bytes for owner and process tests; no runtime authority. | `internal/domain/enrollment` and `internal/domain/release` only in the Linux infrastructure fixture, `github.com/sigstore/sigstore/pkg/signature`, `github.com/theupdateframework/go-tuf/v2/metadata`, standard library |
| `tests/epochfixture/network` | `package network` | Class: engineering check. Construct canonical signed Node Record, Epoch and materialization bytes for black-box Network State and successor provisioning tests. It supplies deterministic test input bytes only and cannot accept State, choose a runtime Route or serve maintained product code. | `tests/epochfixture/assignment`, `tests/epochfixture/merkle`, standard library |






| `internal/architecture` | `package architecture` | Class: engineering check. Test repository structure, ownership inventory, package/import boundaries, artifact-lane separation, extraction rehearsals, names, dependencies, formatting, and quality wiring. It owns the bounded AAI2 retirement closure guard whose exact receipt and scope are defined by [Testing](testing.md#reachability-audit). | standard library |
| `tests/diagnostics/heapdump` | `package heapdump` | Class: engineering check. Receipt-bound heap dump verification and parsing test harness; no runtime implementation. | Standard library. Heap dump role-map tests consume `internal/domain/admission`. |
| `internal/diagnostics/command` | `package main` | Class: engineering command. Linux entry point for the Docker diagnostic image and the `ardents-diagnostics` executable. Composes explicit capture, monitor, report, panel and evidence operations. The manifest and tool installation remain with `scripts/diagnostics`; log segment custody remains with the store. | standard library; `internal/diagnostics/logstore`, `internal/diagnostics/runtimeevent` |
| `internal/diagnostics/logstore` | `package logstore` | Class: supporting mechanism. The Linux monitor's original owner-private directory, writer lease and segment identities bound retained size, count and age. Continues the same monitor session's counters after joined Close and inventory reopening. The monitor retains subprocess and panel ownership, Console output and its own status snapshot. The package grants no domain authority or capture capability. | standard library |
| `internal/diagnostics/measurement` | `package measurement` | Class: supporting mechanism. Own explicitly activated, bounded capture-local aggregate recording and detached sample history. Domain producers perform no I/O and supply only finite runtimeevent instruments. Capture composition in both binaries joins its sampler after original work; only ardents-next can lend the existing bounded telemetry exporter. No domain authority, identity, root, SDK or network dependency. | standard library; `internal/diagnostics/runtimeevent` |
| `internal/diagnostics/runtimeevent` | `package runtimeevent` | Class: supporting mechanism. Own the shared finite OTLP/local operation catalogue and canonical command-report writer. Adapt current owner/command reports through a finite operation/phase/outcome catalogue, preserving absent event time and excluding private details. Decode detached facts from the three retained local event formats and journal envelopes, validate shared field grammar and Source reason rules, and supply the finite category/numeric-name catalogue to timeline and engineering projections. No runtime authority, exporter, independent runtime sampler or raw input retention. | standard library |
| `internal/diagnostics/telemetry` | `package telemetry` | Class: supporting mechanism. Own explicit loopback Collector admission and the bounded OTLP trace and metric exporter lifecycle for one command invocation. Admission, Hosting, Issuance, issuer-profile and issuance-results CLI adapters select registered operations and classify their own outcomes through the shared runtimeevent catalogue; this package owns SDK and export lifetime only. | Standard library; `go.opentelemetry.io/otel/attribute`, `go.opentelemetry.io/otel/metric`, `go.opentelemetry.io/otel/trace`, `go.opentelemetry.io/otel/sdk/{resource,trace,metric,metric/exemplar,metric/metricdata}`, `go.opentelemetry.io/otel/sdk/instrumentation` and the two selected OTLP HTTP exporters. `vocabulary_test.go` also uses the selected OTLP trace/metric protobuf readers and `google.golang.org/protobuf/proto` to verify actual collector packets. ; `internal/diagnostics/runtimeevent` for the finite local observation catalogue and classification |
| `internal/diagnostics/timeline` | `package timeline` | Class: supporting mechanism. Format retained local events and current command reports as a streaming operator timeline. The `ardents-next diagnostics timeline` command selects input/output; `runtimeevent` owns shared decoding and categories. No raw retention or authority root. | standard library; `internal/diagnostics/runtimeevent` |
| `internal/application/administration` | `package administration` | Class: application contract. Own the separately authorized Application Interface v1 Publish/Withdraw contract, lifecycle, closed grammar, failure response, conformance vectors, local server transport, and Application client. Its bounded snapshot operation dispatches only to an implementing publication owner; unsupported owners refuse without bodyless fallback, and one snapshot allocation/transition excludes another while withdrawal remains available. Its server serializes accepted-socket close and ownership retirement, joins handlers before returning shutdown, and retains actual close failures. Its Linux text-client published-Link query projects bounded printable destination text only from an implementing separately authorized owner; it carries no caller-supplied Target, Connection bytes, Credential, Service key, State, Route, peer, or Custody fact. | standard library; `internal/application/connection` only in `byte_stream_linux.go` for framed publication ; `internal/application/document`, `internal/application/connection`; `internal/application/internal/socketfile` for the original listener entry |
| `internal/domain/release` | `package release` | Class: domain. New isolated offline Release verifier with separately owned trust history, serialized original evaluations and joined Close. Keeps Root-only versus complete metadata floor publication distinct, rejects lost retained pointers, and returns private immutable authorization. Genuine `cmd/ardents-next/release.go` consumes pinned new Enrollment bytes; `cmd/ardents-next/installation.go` opens complete retained history with `OpenRetained` for fresh candidate evaluation, without creating cold trust; no old runtime, installation, signing or live readiness. Portable metadata/target/policy and floor rules use native lease/publication/flush adapters; other Unix platforms have no runtime qualification from compilation. The content-addressed generation-name grammar is the shared `internal/durablefile` leaf (R1, #764); Release keeps its separate TOCTOU-hardened bounded read, retained-file flush and joined-error native lease custody, which this mechanism does not flatten. | `internal/canonicaldigest`, `internal/durablefile`; `internal/pathcheck`, Standard library; `github.com/theupdateframework/go-tuf/v2/metadata` only in `metadata_authentication.go`, `target_identity.go`, `safety_policy.go`, `floor_encoding.go`; `github.com/theupdateframework/go-tuf/v2/metadata/trustedmetadata` only in `metadata_authentication.go`; `golang.org/x/sys/windows` only in `history_native_windows.go`; behavior signing fixtures only: `metadata` and `github.com/sigstore/sigstore/pkg/signature` in `rotation_test.go`, `metadata` in `generation_authorization_test.go` ; `internal/diagnostics/measurement`, `internal/diagnostics/runtimeevent` for the registered finite measurement producers/consumers |
| `internal/domain/release/endpoint` | `package endpoint` | Class: domain. Projects immutable Enrollment bytes as exact Release inputs, authenticates the coherent program and protected-generation pair, and checks the retained metadata needed for initial staging before effects. Real consumers are `ardents release verify-initial` and Endpoint launch. | `internal/domain/enrollment`, `internal/domain/release`, standard library; `tests/releasefixture` for independently signed inventory fixtures ; `internal/diagnostics/measurement`, `internal/diagnostics/runtimeevent` for the registered finite measurement producers/consumers |
| `internal/domain/endpoint/launch` | `package launch` | Class: domain. Composes Endpoint initial trust, initial and stopped-predecessor staging, platform unit rendering and privileged pre-start checks. Retains the original installed observations separately at startup. The platform manager owns process start and stop. Real consumers are Endpoint participant and the existing `ardents platform` entrypoints. | `internal/canonicaldigest`, `internal/domain/endpoint/generation`, `internal/domain/endpoint/plan`, `internal/domain/endpoint/unit`, `internal/domain/enrollment`, `internal/platform/artifacts` for protected delivery, staging and tests of installed input pins, `internal/platform/cgroup`, `internal/platform/process`, `internal/platform/systemd`, `internal/platform/systemd/observation`, `internal/domain/release`, `internal/domain/release/endpoint`, `internal/durablefile`, standard library |
| `internal/domain/endpoint/plan` | `package plan` | Class: domain. Validates bounded Endpoint declarations, platform launch identity, writable paths and read-only Source credentials on every host. Linux request-file custody retains byte, inode and access provenance. Real consumers are Endpoint launch and the platform CLI. | `internal/canonicaldigest`, `internal/durablefile`, `internal/pathcheck` for platform launch path declarations, standard library |
| `internal/domain/endpoint/unit` | `package unit` | Class: domain. Owns pure Endpoint unit rendering and typed systemd policy. The renderer and the loaded-unit verifier use the same privileged pre-start command arguments. Endpoint launch and Worker activation are real consumers. | `internal/canonicaldigest`, `internal/platform/systemd/observation`, `internal/pathcheck` for platform unit rendering, standard library |
| `internal/platform/systemd` | `package systemd` | Class: platform adapter. Renders the portable signed infrastructure template. Linux observes the fixed unit, pins the original manager and joins transport and cancellation. Read-only role connections refuse every job object; the absence-only connection refuses all unit objects. Platform launch, Endpoint launch and State repair consume the actual manager. | `internal/platform/systemd/observation`; `github.com/godbus/dbus/v5` only in the exact Linux manager files registered below; standard library |
| `internal/platform/systemd/observation` | `package observation` | Class: supporting mechanism. Detached typed property values and portable bounded JSON decoding and comparison. Endpoint unit policy and Worker checks use these facts, and State recovery compares the conditions for its own exact role. The native manager separately produces the observations. | standard library |
| `internal/platform/launch` | `package launch` | Class: platform adapter. Platform service template and native pre-start and delivery command composition; Enrollment and Release retain artifact authority. | `internal/domain/endpoint/launch`, `internal/platform/artifacts`, `internal/platform/systemd`, `internal/domain/release`, `internal/domain/release/infrastructure/launch`, standard library |
| `internal/platform/cgroup` | `package cgroup` | Class: platform adapter. Reads bounded kernel scopes and checks quiescence for Endpoint replacement and fixed-role State repair. | `internal/pathcheck`, Standard library |
| `internal/application/document/worker` | `package main` | Class: application contract. Adapt only the fixed installed reader and Publisher worker entrypoints. The artifact remains `ardents-text`. The trusted document commands in `cmd/ardents-next` own only local terminal/pipe presentation and the bounded Application and Administration clients. | `internal/application/document`, standard library |
| `tests/qualification/worker-lifecycle/stream-worker` | `package main` | Class: engineering check. Implement only the fixed reader or Publisher entrypoint of the separately pinned NET-14 stream-test Application. It accepts one fixed local initialization record, returns its matching readiness acknowledgement, and transfers only the fixed worker-frame bridge; no argument supplies a profile, target, address, executable, authority, Principal or Grant. | `internal/application/streamqualification`, standard library |
| `internal/application/connection` | `package connection` | Class: application contract. Own AAI3 document requests and AAI4 byte-stream requests, refused reserved Name destinations, bounded ordered streams, directional EOF, terminal outcomes and joined cancellation. The server admission, request decoder and setupCleanupError refusal classifier (`refusal`) compile with the protected Linux Endpoint; the pure typed refusal constructor (`Refuse`, `server_refusal.go`, #885), client, request encoder, shared data framing and outcome validation remain portable. This grammar does not attest a worker launch or create local authority. | standard library; `internal/application/internal/socketfile` for the original listener entry |
| `internal/application/internal/socketfile` | `package socketfile` | Class: supporting mechanism. Creates a local Unix listener and retains the original socket entry and parent. Administration and the Linux Connection server join their own accepted clients before asking it to retire the original entry. A replacement at the same name is preserved. | standard library |
| `internal/application/streamqualification` | `package streamqualification` | Class: application contract. Own the fixed `ARDTQP01` local initialization/readiness and bounded worker-frame bridge, authenticated Service-stream transfer and per-stream workload verdict for the separately pinned NET-14 stream-test worker. Its two retained 64/16 client and 256/64 Publisher workload shapes send only deterministic predeclared bytes after Endpoint OPEN/CREDIT and validate received bytes before replenishing exact credit. It defines no target, Route, address, executable, authority, Principal, Grant, or arbitrary caller-selected schedule. The initialization/readiness codec, worker-frame grammar, worker workload and Endpoint-side connection bridge are portable and compile and behavior-test in the portable deterministic inventory; the Endpoint-side `InitializeWorker`/`RunConnections` have only the Linux qualification composition (`internal/domain/execution/runtime`) as production consumer, and installed stream worker activation remains Linux-only. | `internal/application/connection`, standard library |
| `internal/application/document` | `package document` | Class: application contract. Own the trusted Unix-family no-follow import of an explicitly selected regular file, the portable immutable bounded UTF-8 snapshot and its fixed document responder, the shared document reader, one bounded trusted local Target Link exchange and independently escaped plain-text presentation, nonce-bound reader/Publisher worker exchange, finite stream credit and round-robin output. Presentation validates the whole body before writing and escapes C0/C1 and Unicode bidi controls, preserving LF and tab. Its reader bridge transfers exactly one admitted Service stream, validates the complete fixed request before forwarding, waits for authenticated terminal completion, validates bounded worker RESULT, and joins owned I/O before returning bytes. Its Publisher bridge receives at most 256 admitted Service streams, schedules up to 64 worker exchanges, and gives each active response a 64 KiB byte queue plus one at-most-16 KiB write. Credits follow consumed or discarded bytes; malformed requests and Service loss remain stream-scoped, while worker protocol violations close the job. Endpoint still owns job/Grant checks and cgroup cleanup. The fixed Linux entrypoint checks inherited local descriptors before its readiness exchange; it identity-verifies and closes any manager-passed resource-control descriptor of its own unified cgroup (systemd 255 `cpu.max` passing for resource-limited units) and refuses every other surviving descriptor, including own-cgroup migration/lifecycle surfaces; it cannot attest the installed unit or issue a Grant. The document grammar, snapshot exchange, worker INIT/READY and frame codecs, reader/Publisher bridges and worker multiplexer are portable and compile and behavior-test in the portable deterministic inventory; the no-follow file import (`snapshot_file_unix.go`) is Unix-family (Linux and macOS) through the shared `internal/durablefile` inspection port (#850) with an explicit typed unsupported-platform refusal on every other platform, and the installed inherited-descriptor entrypoint (`worker_process_linux.go`) remains Linux-only with its explicit typed unsupported-platform refusal, and the portable codecs' production consumers remain the selected Linux installed-worker, Execution and Endpoint compositions. | `internal/application/connection`, `internal/durablefile`, standard library |
| `internal/domain/publication` | `package publication` | Class: domain. Own canonical public Service v3 request/response grammar and unsigned fixed-purpose Credential and Publication transcripts, verify exact public Credential/Publication v3 signatures, Target/Network/validity and separate read versus publish delegation. Retain copied public proof bytes; no Authority issuance, private Instance signer/root, readiness or Route/Store ownership. | standard library |
| `internal/domain/publication/durable` | `package durable` | Class: domain. Independently owned exclusive v3 Publication root, synced ancestor/marker/layout preparation, exact consumed-generation reservation, immutable public record/current persistence and bounded canonical floor/public-record reconciliation. Record publication joins staging/generation/pointer barriers; exact retry preserves bytes/inodes, and original generation Close retires pointer before record without lowering the floor. Restored public evidence grants no private Instance or accepting Publisher. Real consumer: `cmd/ardents-next/publication.go` through `publication prepare-root`. The durable owner joins original root closure before returning the floor. Portable tests isolate required flush failures; the Unix-family (Linux and macOS) tests use the actual flock lease and directory barriers and the behavior suites run on all mandatory targets against each platform's native lease. The lease seam re-tag (#880, #691) moved the flock implementation from Linux-only to the Unix family byte-identically; the Windows handle-based lease and the refusal stub for every other host are unchanged, and the macOS fixture canonicalizes the temp root past the system `/var` symlink without weakening the ancestor refusal. | `internal/domain/publication`; `golang.org/x/sys/windows` only through `lease_windows.go`; standard library |
| `internal/domain/publication/instance` | `package instance` | Class: domain. Host-generated non-exporting Instance key, stable public approval request and verified external response through pure Publication and exclusive durable pending/accepted/rejected/conflicting lifecycle. Exact accepted retries preserve the response; terminal or ambiguous transitions deny further authority receipts and clear private material. Unix-family private custody (Linux and macOS) retains original root/lease identity; other platforms refuse before key/filesystem creation. Public grammars and sealed Credential verification remain portable. Real consumer: `cmd/ardents-next/publication.go` for public initialization, request and response operations. The Instance owner joins original root closure before returning each receipt. Original private binding now reconciles a concrete `publication/durable` owner, advances its exact reserved floor and persists consumed redaction before retaining volatile purpose-specific signing. Publication-record signing requires the original genuine Introduction registration receipt and persists the exact consumed record/current through its retained durable reservation. Canonical Descriptor signing consumes Reachability transcript construction and an independent volatile X25519 recipient with strictly increasing real registration revision and capacity two; public preparation commands do not establish that live signing consumer or readiness. Purpose-specific capsule opening retains the original recipient through an opaque user handle, checks exact registration/publication/profile binding and joins all users before erasure/capacity return. Candidate plaintext grants no downstream or replay acceptance. No generic signer or Authority custody. ServiceAttachment confines TLS1.3 CertificateVerify and one-use verified initial challenge proof to the original private opening and admitted JOIN; actual coalesced response write plus final live checks transfer setup into unchanged Work Safety, while the separate immutable ServiceLease retains original Instance authority across physical Attachment gaps and earlier cleanup failures. Logical retirement joins current TLS users before returning that borrow; Publication joins logical leases before key erasure. | `internal/canonicaldigest`, `internal/domain/publication`, `internal/domain/publication/durable`, `internal/domain/reachability`, `internal/domain/route/introduction`, `internal/domain/route/capsule`, `internal/domain/connection/wire`, `internal/domain/route/join`, `internal/domain/execution/runtime`; `internal/domain/route/transport` only in `service_attachment.go` for the closed TLS key-exchange groups; standard library |
| `internal/domain/publication/runtime` | `package runtime` | Class: domain. One original qualified snapshot Publisher, exclusive Instance lifetime borrow, genuine Introduction registration and actual Source Descriptor Store exchange. Own pending/current/bounded predecessor pairs, exact retry, strictly increasing revision, creation+300s real-clock refresh, joined withdrawal and terminal result. Canonical Link is visible only after actual synced Store ACK and final original Execution/Instance/registration/Descriptor checks. Real non-test consumer: `internal/domain/endpoint/publisher/opening_unix.go` through the qualified holder console; capsule opening, replay and native acceptance remain separate obligations. | `internal/domain/connection`, `internal/domain/execution/runtime`, `internal/domain/network`, `internal/domain/publication`, `internal/domain/publication/instance`, `internal/domain/reachability`, `internal/domain/route`, `internal/domain/route/introduction`, `internal/domain/route/prefix`, `internal/domain/route/capsule`, `internal/domain/route/join`; `internal/domain/connection/wire` for copied full-tuple dispatch commitments; standard library ; `internal/diagnostics/measurement`, `internal/diagnostics/runtimeevent` for the registered finite measurement producers/consumers |
| `internal/domain/connection` | `package connection` | Class: domain. The original Reader Source and Publisher recipient bind exact Execution, Publication and Route owners before authentication. `initiator_initial.go` acquires the initial Reader JOIN and hands the authenticated Attachment to Stream. Publisher authentication consumes the original Responder JOIN and non-exporting Instance lease. Stream owns the shared lock, exact record actors, recovery deadline, replacement commit and joined retirement. Initial and replacement Reader authentication use the same exact Instance TLS client and standard-library-only certificate verifier. Production consumers are `endpoint/reader` and `publication/runtime`. | Standard library; `internal/domain/execution/runtime`, `internal/domain/publication`, `internal/domain/publication/instance`, `internal/domain/route/capsule`, `internal/domain/route/join`, `internal/domain/connection/wire`, `internal/domain/connection/transfer`; `internal/domain/connection/recordio` in the two authenticators, `stream_records.go` and their registered response/Terminal tests. `internal/domain/network`, `internal/domain/reachability`, `internal/domain/route`, `internal/domain/route/prefix`, `internal/domain/route/selection` and `internal/domain/route/introduction` in the registered Reader Source, initial and replacement Route files. `publisher_route.go` retains the original Source/Responder via `internal/domain/route/prefix`. `internal/application/connection` and `internal/domain/route/transport` edges are fixed in the source-specific table above. `internal/diagnostics/measurement` and `internal/diagnostics/runtimeevent` use the registered producers. |
| `internal/domain/connection/recordio` | `package recordio` | Class: supporting mechanism. Coalesces the initial Reader write, bounds both initial read flights, and reads a Stream actor's physical records through fixed framing scratch. Reports original transport errors even when sufficient bytes accompanied the error and counts consumed plaintext for the Stream's Terminal-tail decision. Connection's Reader and Publisher authenticators and its Stream reader are real callers. The callers own deadlines, authority, TLS and joined lifetime. | Standard library and `internal/domain/connection/wire`. |
| `internal/domain/connection/transfer` | `package transfer` | Class: domain. Ordered byte and Terminal state of one authenticated Connection across physical replacement. Owns bounded replay, ownership of an original record write, exact peer offsets, preparation of the replacement ledgers, and the original recovery deadline and one-use proposal tickets. The Stream owns the lock, current authority, physical actor join, Attachment installation and original cleanup. | standard library, `internal/domain/connection/wire`, `internal/diagnostics/measurement` and `internal/diagnostics/runtimeevent` for the exact registered replay and receive queue producers and test. |
| `internal/domain/connection/wire` | `package wire` | Class: domain. Closed canonical Service Connection v3 records, immutable logical/Attachment transcripts, TLS exporter binding and one-use verified initial request receipt. Genuine non-test callers are Connection recipient binding and Instance Service key use. Pure checked values grant no local permission, accepting Publication or Route. No generic signer or owner callback. | Standard library only |
| `internal/domain/reachability` | `package reachability` | Class: domain. Verifies exact signed Descriptor v3 bytes for a selected Target and live profile. Local lookup history retains original attempts and monotonic facts. One pure conflict-floor rule serves the local history and the receiving Store. | standard library, `internal/domain/publication`. Independent signed vectors use `tests/descriptorfixture` in tests. |
| `internal/domain/reachability/store` | `package store` | Class: domain. Owns the receiving Descriptor Store root, native lease, durable conflict-floor commits and joined original retirement. Retained opens validate complete history. Linux inspection and bounded repair leave committed floors in place. Node and Route are real consumers. | standard library, `internal/domain/reachability`, `internal/pathcheck`, `internal/diagnostics/measurement`, `internal/diagnostics/runtimeevent`, `golang.org/x/sys/windows` only in `lease_windows.go`. Tests use `tests/descriptorfixture`. |
| `internal/domain/hosting` | `package hosting` | Class: domain. Shared durable provider-period accounting, portable work/termination reservations and bounded confirmed snapshots. Budget and storage check boot and interface continuity before charging and publish results after durable commit. The original Unix-family lease and Linux retained inspection keep their own physical lifetimes with typed refusal on other hosts. | standard library, `internal/canonicaldigest`, `internal/pathcheck`, `internal/platform/artifacts`, `internal/domain/hosting/counters`. Diagnostics use the existing finite `internal/diagnostics/measurement` and `internal/diagnostics/runtimeevent` producers. |
| `internal/domain/hosting/counters` | `package counters` | Class: platform adapter. Captures original boot identity and ordered per-interface byte totals. Linux uses procfs and sysfs. macOS uses full 64-bit IFMIB totals and checks the interface index again. The portable bounded reader and typed unsupported path have direct behavior coverage. Hosting Budget is the production consumer. | standard library; `golang.org/x/sys/unix` only from `read_darwin.go` for the reviewed IFMIB sysctl read. |
| `internal/domain/admission/issuance` | `package issuance` | Class: domain. Own canonical batch-facts declarations and profile-to-ledger binding preparation, immutable offline RSA material, unsigned inventory and its offline export, immutable signed issuer profiles and durable confirmed blind-token results; exclusive Unix leases where supported, no live authority. | `internal/domain/admission`, `internal/domain/admission/issuerprofile`, `internal/domain/admission/issuerprofile/signing`, `github.com/cloudflare/circl/blindsign/blindrsa`, `internal/domain/admission/quota`, `internal/pathcheck`, `internal/canonicaldigest`, standard library; `internal/platform/artifacts` for portable unique JSON grammar and Linux and macOS private output publication |
| `internal/domain/admission/operator` | `package operator` | Class: domain. Adapt offline quota initialization, issuance inventory/results and Issuer profile operations to bounded command input and receipts. Called by the Admission, Issuance and Node identity CLI dispatch. | `internal/domain/admission`, `internal/domain/admission/issuance`, `internal/domain/admission/issuer`, `internal/domain/admission/quota`, `internal/diagnostics/telemetry`, `internal/platform/artifacts`, standard library ; `internal/diagnostics/runtimeevent` for the finite local observation catalogue and classification |
| `internal/domain/admission/issuer` | `package issuer` | Class: domain. Compose offline and current-authority open/debit/sign/retain/close and pinned issuer profile preparation/inspection; retain the primary result and every acquired owner cleanup before finite command projection. No quota arithmetic or private signing implementation. | `internal/domain/admission`, `internal/domain/admission/issuance`, `internal/domain/admission/issuerprofile/signing`, `internal/domain/admission/quota`, `internal/pathcheck`, standard library ; `internal/domain/network`, `internal/canonicaldigest`, `internal/platform/artifacts` |
| `internal/domain/admission/issuerprofile/signing` | `package signing` | Class: domain. Import independently pinned Ed25519 Node key into exclusive immutable Linux storage; sign only opaque validated issuer-profile requests. No private-key or general signing API. | `internal/domain/admission/issuerprofile`, `internal/pathcheck`, standard library |
| `internal/domain/custody` | `package custody` | Class: domain. Encrypted purpose-specific Vault/Recovery Bundle, original serialized operation/cancel/join/erasure, Admission signing through genuine Allocation and durable successor/readback/floor. Service Authority creation and independently approved fixed-purpose Credential issuance use the pure public v3 contract; predecessor Service/retired Name records remain encrypted evidence; restored quarantine has no signing/activation API. No predecessor backend or live-root adoption. Credential and Permission delivery share flushed no-replace publication with exact retry and separate private Permission access checks. Real consumer is `cmd/ardents-custody`. | `internal/domain/admission`, `internal/domain/admission/allocation` and pure `internal/domain/publication`, standard library; `golang.org/x/crypto/argon2` only in `canonical_envelope.go`; `golang.org/x/sys/windows` only in `vault_lock_windows.go`, `vault_lock_windows_test.go` and `durability_windows.go` ; `internal/diagnostics/measurement`, `internal/diagnostics/runtimeevent` for the registered finite measurement producers/consumers |
| `internal/domain/admission/allocation` | `package allocation` | Class: domain. Permission allocation, role maxima and retained allocation decisions. | `internal/domain/admission`, standard library |
| `internal/domain/admission/attempts` | `package attempts` | Class: domain. Durable holder presentation history. | `internal/pathcheck`, standard library |
| `internal/domain/admission/quota` | `package quota` | Class: domain. Exclusive issuer quota journal, atomic debit, exact retry and opaque debit confirmation. | `internal/domain/admission`, `internal/domain/admission/issuerprofile`, `internal/pathcheck`, standard library |
| `internal/domain/admission/receiving` | `package receiving` | Class: domain. Receiver binding, redemption and finite token allowance; separate original-operation serialization and synchronous Close denial, with foreign observation/reservation and refused cleanup outside the state mutex and spend-lease return after user join. Accepted work owns transferred reservations. | `internal/domain/admission`, `internal/domain/admission/token`, `internal/domain/admission/spending`, standard library ; `internal/domain/network`, `internal/platform/artifacts` ; `tests/networkfixture`, `tests/epochfixture/network`, `internal/domain/network/state` in `registration_initialization_integration_test.go` for signed-State observation ; `internal/domain/admission/allocation`, `internal/domain/admission/issuer`, `internal/domain/admission/quota`, `internal/domain/hosting` in the signed Network reservation integration test |
| `internal/domain/admission/spending` | `package spending` | Class: domain. Receiver token spend history and distinct Linux existing-only read-only inspection with original numeric principal/lease and joined physical completion; no Introduction slot ownership or runtime authority from inspection. | `golang.org/x/sys/windows`, `internal/pathcheck`, standard library |
| `internal/domain/route/introduction` | `package introduction` | Class: domain. Own holder Registration request/reader/owning withdrawal and joined result over opaque Prefix channels, separately receiving Registry/live slots, canonical requests/results and durable non-reclaim History. No shared principal, root or lifetime, listener, token spend, Hosting budget or Publication readiness. The durable History root is the Unix-family (Linux and macOS) slot-history port (Cohort 1 N2, #853): every mutation resolves through the held directory descriptor via the shared `internal/durablefile` Openat/Renameat and full-identity inode comparison, and a kernel root-move watch (Linux inotify, macOS kqueue EVFILT_VNODE) retains original move/loss/restoration evidence; every other platform receives the same explicit typed refusal, never a weaker accepting path. | `internal/domain/admission/receiving` for the genuine one-use root creation fact and the typed platform refusal, `internal/domain/admission/spending`, `internal/durablefile` through the Unix-family slot-history port (`slot_history_unix.go`) and its custody test, `internal/domain/route`, `internal/domain/route/ardp`, `internal/domain/route/role`, `internal/domain/route/prefix`, `internal/domain/network`, `internal/domain/admission`, `internal/domain/route/capsule`, `internal/pathcheck`, standard library |
| `internal/domain/route/capsule` | `package capsule` | Class: domain. Closed v3 fixed envelope and recipient-only request grammar, exact domain/header transcript and X25519/HKDF-SHA256/AES-128-GCM public-key sealing. Immutable copied envelopes and candidate requests grant no authority, private key, replay acceptance, Connection binding or live Route. Actual consumers: `publication/instance` purpose-specific borrowed opening and native `ardents-next` Source preparation followed by its genuine admitted Submit exchange; independent codec/suite controls and genuine Registration composition exercise the same grammar. No new package-level authority or old runtime connection. | standard library |
| `internal/domain/admission/stock` | `package stock` | Class: domain. Holder permission, pending exact batch, stock and irreversible presentation. Application owns exchange execution and cancellation. | `internal/domain/admission`, `internal/domain/admission/allocation`, `internal/domain/admission/attempts`, `internal/domain/admission/token`, standard library; behavior tests only: `internal/domain/admission/issuer`, `internal/domain/admission/issuance`, `internal/domain/admission/receiving`, `internal/domain/admission/spending`, `internal/domain/admission/quota` ; `internal/diagnostics/measurement`, `internal/diagnostics/runtimeevent` for the registered finite measurement producers/consumers |
| `internal/domain/admission/token` | `package token` | Class: domain. Holder blind preparation and finalization, token challenges and verification. Consumes the batch grammar directly from Admission and the canonical public key parser from issuerprofile. | `internal/domain/admission`, `internal/domain/admission/issuerprofile`, `github.com/cloudflare/circl/blindsign/blindrsa`, standard library |
| `internal/platform/process` | `package process` | Class: platform adapter. Retains the actual Endpoint invocation facts, independently observes Source principal quiescence, checks the invocation credentials for Source State preparation and validates the fixed service account ID from retained platform input snapshots. Real consumers are Endpoint launch, Source State repair and platform pre-start. | `internal/canonicaldigest`, `internal/pathcheck`, Standard library |
| `internal/domain/endpoint/generation` | `package generation` | Class: domain. Owns Endpoint generation inventory, exclusive creation/sealing and independent retained snapshots. Real consumer is Endpoint launch. | `internal/canonicaldigest`, `internal/durablefile`, `internal/pathcheck`, Standard library |

The exact Service Custody process scenario `tests/e2e/service/service_custody_process_linux_test.go` imports only pure `internal/domain/publication` beyond the existing process mechanisms. It drives built `ardents-next` host request/acceptance and real no-echo `ardents-custody` Authority/issuance commands, with no Instance import or fixture signing authority. `internal/domain/custody/command/service_authority.go` now consumes `internal/domain/custody` directly; the former Service backend is not called.

The Direct-Origin Source adapter in `internal/domain/network/source/command/serve_linux.go` calls
`internal/domain/network/source/serving` for `serve` and `serve-retained` and projects
the original result and diagnostic into the existing JSON and exit status.
Its owner tests in `state_lifetime_linux_test.go` exercise the original State
and input custody, including held observations.
The Source serving owner tests native input custody directly and supervision
at its event boundary. The CLI
output tests exercise native pipes and bounded writes from
`internal/platform/artifacts`.
Serving retains State's opaque SourceCompletion through input, process, and
output cleanup and finishes it with the original causal result. The same
Linux artifact mechanism observes exact original file descriptors for Source,
Introduction, Forwarding, JOIN and Descriptor repair. Each repair keeps its
own policy, authority and durable effect boundary.
The explicit Source repair boundary is registered with the platform composition
above. Its native adapter accepts only a real original manager connection and
independent kernel quiescence under both retained leases; no caller-supplied
process-death callback grants authority. The command retains the public plan
without loading a private transport key and initializes no runtime or root.

The Source serving owner retains its files, synchronous denial, native-use
joins and terminal result. Source and platform
pre-start share original directory traversal, bounded original file reads and
retained file reobservation through `internal/platform/artifacts`. The file check reobserves original
directory access, the retained descriptor, linked identity and exact bytes.
Root-controlled signing remains a separate access check. Each command
retains its role-specific input policy and deadlines. State and Source own
their own effects and joins. Command cancellation can start the original
Close while native input observation is held.

`tests/e2e/source/tls_fixture_linux.go` owns independent public TLS fixture preparation
and standard-library imports only; it grants no native process proof.

`endpoint/launch` checks native ownership and access for the mutable roots
prepared by the operator. The retained manager references supply read-only
observations for Endpoint replacement and role repair.

The former `cmd/ardents-control/join_initial_stopped_native_linux_test.go`
OLD-path build-bound preparation QA, the JOIN allowance preparation
(`installation/join/hosting_prepare_linux.go`, `join_hosting_linux.go`,
`OpenJoinInstalledStopped`, `platform/cgroup/join_quiescent_linux.go`) and
the JOIN explicit Stop/same-generation Restart orchestration retired with the
transitional JOIN package (#604 FJD stage D, ADR-0125). The platform-led JOIN
lifecycle owns allowance, stop and restart through the canonical platform unit;
the retained per-unit repair quiescence observation is
`platform/cgroup/quiescence_linux.go`
(`CheckJoinRecoveryQuiescent`).

The alpha bundle packager invokes reviewed `cmd/ardents-next` source for
`enrollment artifact-name` and `enrollment check-protected-generation`.
`enrollment.go` imports only `internal/domain/enrollment` and the standard
library. It passes the static root and the two supplied executable paths to
Enrollment. Enrollment loads them as data using its existing protected resource
inventory. The document worker's source resources live in
`packaging/document-worker` under the existing signed resource names.

The standalone `scripts/sign-qualification-release.go` implements only the
ADR-0120 initial Endpoint Release operation. Its exact permitted imports are
`internal/domain/enrollment`, `internal/domain/release`,
`github.com/sigstore/sigstore/pkg/signature`,
`github.com/theupdateframework/go-tuf/v2/metadata`, `golang.org/x/sys/unix` and
the standard library. Enrollment validates the static protected resources;
Release verifies both actual signed targets in fresh diagnostic histories.
No old runtime, Network, Route, installation or generic signing authority is
admitted. This permission does not extend to adjacent signing scripts.

### Shared offline Node provisioning

Forwarding, JOIN and Descriptor select a fixed role at CLI dispatch. They
share the offline Node Record, Epoch and Closed Profile ceremonies while
each role keeps its own schema, purpose and receipt namespace. Private keys
and protected input/output roots belong to each invocation. The shared
producers do not admit Node or Route runtime work.

| Source under `internal/domain/node/provisioning` | Exact domain imports |
|---|---|
| `infrastructure/node_record_unix.go`, `infrastructure/node_provision_linux.go`, `infrastructure/epoch_plan_linux.go`, `networksigning/certificate_unix.go` | `internal/domain/network/epoch` |
| `infrastructure/epoch_signing_linux.go` | `internal/domain/network/epoch`, `internal/domain/network/source` |
| `infrastructure/profile_plan_linux.go` | `internal/domain/admission/issuerprofile`, `internal/domain/network`, `internal/domain/network/closedprofile`, `internal/domain/network/epoch`, `internal/domain/network/source` |
| `infrastructure/profile_signing_linux.go` | `internal/domain/network/closedprofile` |
| `infrastructure/token_material_linux.go` | `internal/domain/admission/issuance` |

### Systemd property and manager imports

Only `bus_linux.go`, `descriptor_manager_linux.go`, `doc.go`,
`forwarding_manager_linux.go`, `infrastructure_prestart_linux.go` and its test,
`introduction_manager_linux.go`, `join_manager_linux.go`,
`manager_properties_linux.go`, `platform_linux.go`, `source_manager_linux.go`,
`startup_observation_linux.go` and `startup_observation_native_linux_test.go`
consume the portable observation package inside the systemd adapter. Production
and test consumers outside the adapter are the State repair files listed above,
Endpoint unit's `contract.go` and `platform_completion_linux.go`, Endpoint
launch's `manager_binding_linux.go` and `manager_binding_platform_test.go`,
Worker's `manager_linux.go` and `service_linux.go`, and
`tests/e2e/endpoint/replacement_scopes_linux.go`.

The native manager can import `github.com/godbus/dbus/v5` only in
`descriptor_manager_linux.go`, `endpoint_reference_linux.go`,
`forwarding_manager_linux.go`, `introduction_manager_linux.go`,
`join_manager_linux.go`, `manager_properties_linux.go`,
`manager_reference_linux.go`, `node_slots_linux.go`,
`source_manager_linux.go`, `source_reference_native_linux_test.go` and
`startup_observation_linux.go`. Detached properties and native frame validation
have no bus client import.

### JOIN retained signing/provisioning import inventory

The transitional JOIN first-start/replacement/recovery/transition orchestration
and its canonical Node consumers retired (#604 FJD stage D, ADR-0125); the
platform-led JOIN lifecycle owns installed delivery. The retained offline JOIN
producers keep their own fixed purpose and original input/output custody; the
read-only repair runtime manager keeps its own godbus connection.

- `internal/domain/node/provisioning/networksigning/certificate_unix.go` (shared with Forwarding/Descriptor): `internal/domain/network/epoch`.
- `internal/platform/systemd/join_manager_linux.go`: `github.com/godbus/dbus/v5`.

The former JOIN replacement exact-import inventory
(`replacement_linux.go`/`replacement_candidate_linux.go`/
`replacement_inspections_linux.go`/`replacement_stopped_linux.go`,
`systemd/join_replacement_linux.go`) and the original-cut physical coordination
(`transition_scope_linux.go`, `recovery_replacement_linux.go`,
`systemd.JoinRecoveryReplacement`) retired with the transitional JOIN package;
no production consumer remains. The shared `state.OpenNodeInspection` /
`duty.OpenNodeInspection` mechanisms lost their last production consumer when the
Descriptor replacement orchestration retired (#604 FJD stage D, ADR-0125); they
are retained as reviewed mechanism code with genuine native coverage (see the
deadcode allowlist), and their shared custody/reader helpers stay production-live
through SourceInspection and the bounded repair.

| `internal/domain/endpoint/funding` | `package funding` | Class: domain. Application coordination of original Source stock preparation, issuance and presentation; shared Entry/Hosting parent lifetime. Its holder Route declaration preserves the existing JSON fields and converts them to the owner's plan without granting permission or changing validation. Reader, Publisher and bootstrap callers remain in `cmd/ardents-next`. Admission retains finite rights/debit and durable presentation; Network retains current authority; Route retains selection and physical join. P1-B batch 2 (#787, #691) de-tagged the whole composition (`source.go`, `preparation.go`, `prefix.go`, `issuance.go`, `parents.go`, `observation.go`, `reservation.go`) to portable pure Go after per-file purity verification: every composed symbol (hosting reservation accounting, stock/token/prefix/selection helpers) became portable in batch 1 or earlier, and no file adds a native mechanism. The genuine consumers remain Linux (session/endpoint/`cmd/ardents-next`), so the newly portable symbols are windows-production-dead under reviewed deadcode allowances; the portable `preparation_test.go` exercises the Stock preparation coordination on the windows/macos lane. | `internal/domain/admission`, `internal/domain/admission/stock`, `internal/domain/admission/token`, `internal/domain/hosting`, `internal/domain/network`, `internal/domain/network/state`, `internal/domain/route`, `internal/domain/route/ardp`, `internal/domain/route/bootstrap`, `internal/domain/route/prefix`, `internal/domain/route/role`, `internal/domain/route/selection`; `internal/pathcheck`, standard library. ; `internal/domain/admission/issuer`, `internal/domain/admission/receiving`, `internal/domain/admission/quota`, `internal/platform/artifacts` ; `tests/networkfixture` in `intent_network_test.go` ; `tests/epochfixture/network` in `intent_network_test.go`; `internal/domain/admission/console` for bounded Admission receiver conversation in `receiver_conversation.go`. |

| `internal/domain/endpoint/reader` | `package reader` | Class: domain. One original qualified Reader operation for the installed Endpoint, Source/history ownership, sole stream handoff, local document request/result projection, interruption and joined retirement. The installed participant constructs the Reader for the local Connection surface used by `ardents read`. Borrows participant authority/Stock/Execution/Parents; no CLI or publication decisions. P1-B batch 3 (#794, #691) de-tagged the pure local Text result projection (`result.go`: `newResultStream`/`resultStream`/`checkedOutput` over `io.Pipe`, composing only portable `application/connection.Outcome` and `application/document.NewSnapshot`) with its portable behavior test (`result_test.go`). The Reader coordinator, operation, Source opening and byte-stream adapter (`reader_unix.go`, `operation_unix.go`, `source_unix.go`, `byte_stream_unix.go`) are Unix-family (Linux and macOS, #885, #691). Cohort 2d (#877, #691) had kept them Linux because the coordinator composed the then-Linux-only `application/connection.Refuse` projection; #885 split the pure typed refusal constructor `Refuse` into the portable `server_refusal.go` (byte-identical; the Linux-only `refusal` setupCleanupError classifier stays with the protected Linux server admission), and per-file re-verification confirmed every remaining dependency is portable, Unix-family, or has a typed `!linux` worker-launch refusal (`LaunchReader`/`LaunchByteStreamReader`; `connection.StreamFailureReason` is portable). macOS now runs the genuine Reader coordinator composition; `beginRead` reaches the `!linux` typed worker refusal before any Source or root state, so no unconfined worker is spawned and Linux behavior is byte-identical. The de-tagged local Text result projection (`result.go`) stays windows-production-dead under a reviewed deadcode allowance (its consumer is now the Unix-family coordinator); `Refuse` is windows-production-dead but test-reachable through its portable behavior test (`server_refusal_test.go`). The installed participant stays Linux (`endpoint/launch` systemd delivery plus the `declarations_linux.go`/`permission_linux.go` projections) and is the Reader's sole production consumer, so the Reader composition adds no Windows production code and needs no Windows refusal stub. | `internal/application/connection`, `internal/application/document`, `internal/domain/admission`, `internal/domain/admission/stock`, `internal/domain/connection`, `internal/domain/endpoint/funding`, `internal/domain/execution/runtime`, `internal/domain/hosting`, `internal/domain/network`, `internal/domain/reachability`, `internal/domain/route/prefix`, `internal/domain/route/selection`; standard library. |

| `internal/domain/endpoint/publisher` | `package publisher` | Class: domain. Qualified snapshot publication application lifecycle: pending/active handoff, Source and role selections, Instance/history custody, stream delivery, withdrawal and joined cleanup. Real callers: installed participant and holder console in `cmd/ardents-next`. Domain Publication retains readiness/refresh/recipient policy. Root isolation uses the shared portable `pathcheck.DisjointRoots`. The portable document stream contract (`contract.go`: `DocumentContract`) composes `Operation.BindStreamContract` and `document.MaximumBytes`. The Publisher composition (`opening_unix.go`, `publisher_unix.go`, `snapshot_unix.go`, `byte_stream_unix.go`) is Unix-family (Linux and macOS, #883, #691): after the Instance custody (#867), reachability Store (#870), Hosting lease/measurement (#871/#873), worker launch capability (#875) and durable lease (#880/#882) seams merged, live Open composes only Unix-family selection/Hosting/Instance/durable roots with the portable domain Publisher and launches no native worker itself; the coordinator launches the native worker (`LaunchPublisher`/`LaunchByteStreamPublisher`) before live Open, so macOS receives the typed worker refusal before any root state; `LaunchSnapshotFile` imports the caller input read-only (`document.ReadSnapshotFile` creates no filesystem state or authority) before the same typed launch refusal; the byte-stream adapter composes only the portable `application/connection` contract, not the `Refuse` typed-refusal projection. macOS worker launch stays a typed refusal (`execution/runtime` `!linux` stubs); no unconfined worker is spawned and Linux behavior is byte-identical. The opening/snapshot typed refusals (`opening_other.go`, `snapshot_other.go`) narrow to Windows (`!linux && !darwin`), which lacks the ported Hosting measurement and Instance custody and would otherwise create selection root state before the deeper refusal; the Windows refusal-behavior tests (`opening_other_test.go`, `snapshot_other_test.go`), a macOS read-only-input refusal test (`snapshot_darwin_test.go`) and the architecture assertion pin that every unsupported host refuses before any state or authority. `DocumentContract` keeps its reviewed Windows production and native test deadcode allowances (its genuine consumers remain Unix-family or Linux-tagged). | `internal/application/document`, `internal/domain/admission`, `internal/domain/admission/stock`, `internal/domain/endpoint/funding`, `internal/domain/execution/runtime`, `internal/domain/hosting`, `internal/domain/network`, `internal/domain/publication/durable`, `internal/domain/publication/instance`, `internal/domain/publication/runtime`, `internal/domain/route/prefix`, `internal/domain/route/selection`; `internal/pathcheck`, standard library.; `internal/application/connection` for the byte-stream protocol |

## Descriptor retained signing/provisioning import inventory

The transitional Descriptor initial admission, stopped installation, first-start/
replacement/recovery/transition orchestration and its canonical Node consumers
retired (#604 FJD stage D, ADR-0125); the platform-led Descriptor lifecycle owns
installed delivery. The former `installation/descriptor` package, the
`cmd/ardents-node/descriptor_installed_linux.go` R01 five-import composition, the
`descriptor_installation_linux.go` transactions, the OLD-path installed QA tests
and the `prepare-descriptor-release-plan.py` public build-bound preparer are
removed. The retained offline Descriptor producers keep their own fixed purpose
and original input/output custody; the read-only repair runtime manager keeps its
own godbus connection. Descriptor Release signing is preserved and bound to the
static `ardents-next` program and `infrastructure.PlatformUnitTemplate`; the
former initial admission entry point and detached inventory accessors retired.

- `internal/domain/node/provisioning/networksigning/certificate_unix.go` (shared with Forwarding/JOIN): `internal/domain/network/epoch`.
- `internal/domain/release/infrastructure/signing/descriptor_release_native_linux_test.go`: `internal/domain/release/infrastructure`, `internal/platform/systemd`, `github.com/theupdateframework/go-tuf/v2/metadata`.
- `internal/domain/release/infrastructure/signing/descriptor_forward_release_native_linux_test.go`: `internal/domain/release/infrastructure`, `internal/platform/systemd`.
- `internal/domain/release/infrastructure/descriptor_admission.go`: `internal/domain/enrollment`, `internal/domain/release`.
- `internal/platform/systemd/descriptor_manager_linux.go`: `github.com/godbus/dbus/v5`.

`release_key_command_linux.go` selects the fixed Descriptor key ceremony and
reports its public inventory, while `signing/internal/keyset` retains the
original file publication.
Fixed initial/forward key/plan/signing commands share native key/input/output
codecs, never Source/JOIN signing purposes. Descriptor uses the fixed role
in `ardents-next infrastructure` for public artifact preparation. The former
`prepare-template`/`prepare-request`/`prepare-plan`/`prepare-hosting`/
`prepare-state` routes were removed.
`ardents-next platform template` emits the canonical platform unit template.

The former Descriptor replacement exact-import inventory
(`replacement_linux.go`/`replacement_candidate_linux.go`/
`replacement_inspections_linux.go`/`replacement_stopped_linux.go`,
`systemd/descriptor_replacement_linux.go`) and the original-cut physical
coordination (`transition_scope_linux.go`, `recovery_replacement_linux.go`,
`systemd.DescriptorRecoveryReplacement`) retired with the transitional Descriptor
package; no production consumer remains. The shared `state.OpenNodeInspection` /
`duty.OpenNodeInspection` / `reachability/store.OpenInspection` /
`spending.OpenInspection` / `hosting.OpenInspection` read-only inspection
mechanisms lose their last production consumer and are retained as reviewed
mechanism code with genuine native coverage (see the deadcode allowlist); their
shared custody/floor/reader helpers remain production-live through
SourceInspection, the live Store and the bounded repair.

`route/receiver/outer_authority_linux.go` owns fresh parent HELLO and original
adjacent-peer reobservation for JOIN and Descriptor. Its exact imports are
`internal/domain/network`, `internal/domain/route/ardp` and the standard
library. Its real caller is `receiver_unix.go` at every existing parent frame
boundary. `outer_authority_linux_test.go` additionally imports the existing
`internal/domain/route/role` contract for supplied-fact refusal controls;
actual signed intake and installed consumers retain their native evidence.
No parent observation performs Store or Registry effects or replaces their
separate role checks.

The transitional Descriptor complete stopped replacement recovery (original-cut
custody, `recovery_replacement_linux.go`, the pidfd/subreaper
`transition_scope_linux.go` and the read-only `systemd.DescriptorRecoveryReplacement`)
retired with the transitional Descriptor package (#604 FJD stage D, ADR-0125);
the platform-led lifecycle and systemd own replacement/recovery through the
canonical platform unit. No sibling role lifetime or permission is borrowed.

| Path | Package | Responsibility and real consumer | Permitted imports |
|---|---|---|---|
| `internal/domain/endpoint` | `package endpoint` | Class: domain. Original installed participant lifetime, bound Endpoint declaration projection, bounded preparation effects and installed participant composition. `ardents-next endpoint construct-plan` validates declarations and writes fresh canonical preparation plans from accepted retained State pins, bounded operator declarations and the distributed Source artifact, without runtime authority. `ardents-next endpoint prepare-history` explicitly authenticates initial signed State/profile and prepares vacant participation, Entry/Interior and role Stock; `start-installed` requires retained histories. Owns the offline Source trust preparation and reuses its detached declaration types. Private Linux permission-file handover stays with the original participant in `permission_handover_linux.go`. It publishes the exact request, checks original response custody and retires response before request. Admission owns the signed response size and Stock admits the response. IPC and Service users join before shared funding, Stock, State, local Execution and Startup. The installed command adapter owns its explicit operation identity, bounded progress and exit projection; domain policies stay with their respective owners. P1-B batch 3 (#794, #691) de-tagged the pure network presentation guard (`authority.go`: `presentationGuard`, composing only portable `stock.Presentation`, `network.RuntimeView` and `funding.NetworkFacts`); its genuine consumer `participant_linux.go` stays Linux, so it is windows-production-dead and platform-native test_dead under reviewed deadcode allowances. The plan constructor (`plan_construction_linux.go`) stays Linux because it composes the Linux `declarations_linux.go` projections (`NetworkConfig`/`installedLocalConfig`/`HeadlessPlan.Network`, native `syscall` identity/config). | `internal/application/administration`, `internal/application/connection`, `internal/domain/admission`, `internal/domain/admission/stock`, `internal/domain/endpoint/funding`, `internal/domain/endpoint/reader`, `internal/domain/endpoint/publisher`, `internal/domain/execution`, `internal/domain/execution/runtime`, `internal/domain/hosting`, `internal/domain/endpoint/launch`, `internal/domain/network`, `internal/domain/network/source`, `internal/domain/network/state`; `internal/domain/network/source/servingdeclaration` and `internal/platform/processguard` for Source trust preparation, `internal/platform/artifacts` for retained preparation inputs and Source trust; `internal/domain/network/duty` and `internal/domain/route/selection` for history preparation, with Route selection also used by the installed Connection and document tests; tests only: `internal/domain/admission/attempts` in `history_preparation_linux_test.go`, `tests/epochfixture/network` in `history_preparation_linux_test.go` and `plan_construction_linux_test.go`; `internal/application/connection` and `internal/application/document` tests only in `latency_native_linux_test.go` (the #650 cold/warm latency participant runs as the installed Endpoint MainPID, retains the original dual-role owner composition and measures the ordinary Reader composition directly, opening the public Connection request and reading the validated document response; it owns no Application server); `internal/canonicaldigest` for declaration digest validation; `internal/pathcheck`, standard library.; `source_trust_native_linux_test.go` uses `tests/e2e/source` for independent TLS inputs and `internal/platform/artifacts` to verify the protected test parent ; Installed qualification test imports `internal/domain/admission/issuer`, `internal/domain/admission/quota`, `internal/domain/admission/receiving`, `internal/domain/admission/token`, `internal/domain/connection`, `internal/domain/endpoint/session`, `internal/domain/execution/worker`, `internal/domain/network/epoch`, `internal/domain/publication`, `internal/domain/publication/instance`, `internal/domain/publication/runtime`, `internal/domain/reachability`, `internal/domain/route`, `internal/domain/route/ardp`, `internal/domain/route/capsule`, `internal/domain/route/introduction`, `internal/domain/route/prefix`, `internal/domain/route/receiver`, `internal/domain/route/role`, `internal/domain/route/transport`, `tests/e2e/endpoint`, `tests/e2e/node/processfixture`, `tests/networkfixture`. ; `internal/diagnostics/runtimeevent` for the finite local observation catalogue and classification ; `internal/diagnostics/measurement` for the registered finite measurement producers/consumers; `internal/domain/reachability/store` at the existing verified caller. |
| `internal/domain/endpoint/command` | `package command` | Class: composition. Dispatches original installed Endpoint and explicitly offline preparation, retains bounded file inputs and renders the command progress and exit. Native checks stay Linux and other targets receive an explicit unsupported-platform result. Caller: cmd/ardents-next/command.go. | `internal/domain/endpoint`, `internal/domain/network/source`, `internal/domain/network/state`, `internal/pathcheck`, `internal/platform/artifacts`, `internal/diagnostics/runtimeevent`, standard library. |

| Path | Package | Responsibility and real consumer | Permitted imports |
|---|---|---|---|
| `internal/domain/endpoint/session` | `package session` | Class: domain. Serial original holder scenario: attempt/intent, Prefix/JOIN, registration, Publication, lookup history and joined Stock closure. Separate local, preparation and live-operation constructors; each typed method checks original Execution permission. Real consumer: ardents-next holder console. Native Route construction retains selection and Hosting through physical join; no command DTO, JSON, output or process status. P1-B batch 2 (#787, #691) de-tagged the pure route reservation helpers (`reservation.go`: release/root-independence/deadline-min) to portable with a new portable behavior test; they compose only the batch-1 portable hosting reservation release and `pathcheck`. The `OpenPrefix`/`OpenBootstrap`/`OpenJoinContext` compositions (`prefix_unix.go`, `bootstrap_unix.go`, `join_unix.go`) are Unix-family (Linux and macOS): they compose only selection (Unix-family roots), Hosting (Unix-family lease plus Linux/macOS measurement) and the portable Route core, and launch no native worker, so macOS runs the genuine composition with no refusal. Their typed refusal (`route_other.go`) narrows to Windows, which has no ported Hosting measurement and would otherwise create selection root state before the deeper measurement refusal; a Windows refusal-behavior test (`route_other_test.go`) pins refusal before any root state or authority. Cohort 2d (#877, #691) shifted this boundary after the Instance custody, reachability Store, Hosting lease/measurement and worker launch capability seams merged, and re-pinned it in the architecture assertion. The Publisher composition moved to the Unix family after the durable lease re-tag (#880/#882, #883, #691; see its row); the Reader and participant compositions stay Linux (see their rows): they still compose the Linux-only application/connection.Refuse projection and endpoint/launch systemd delivery. | `internal/domain/admission`, `internal/domain/admission/stock`, `internal/domain/endpoint/funding`, `internal/domain/endpoint/publisher`, `internal/domain/execution/runtime`, `internal/domain/hosting`, `internal/domain/reachability`, `internal/domain/route/ardp`, `internal/domain/route/bootstrap`, `internal/domain/route/capsule`, `internal/domain/route/introduction`, `internal/domain/route/join`, `internal/domain/route/prefix`, `internal/domain/route/role`, `internal/domain/route/selection`; `internal/pathcheck`, `internal/domain/execution`, `internal/domain/network`, `internal/domain/network/state`, `internal/platform/artifacts`, `internal/domain/route/channel`, standard library ; signed issuance integration test only: `internal/domain/admission/issuer`, `internal/domain/admission/quota`, `internal/domain/admission/token`, `tests/e2e/node/processfixture`, `tests/networkfixture`; `internal/domain/admission/console` for bounded Admission holder conversation in `local_conversation.go`. |

Retained State/duty opens require their original lease and refuse missing,
symlink/reparse-point or hardlinked lease files before initializing any history.
State's portable retained opener changes no authority or native-runtime support.
Admission `attempts.OpenRetained` and `stock.OpenRetained` require existing
presentation history; the Reader and existing participant directories consume
them. Only a genuinely new participant directory can initialize Stock.

`internal/domain/release/endpoint/execution_artifact_test.go` directly verifies real signed
worker/unit inventory through `internal/domain/enrollment`,
`internal/domain/release` and the current `internal/domain/release/endpoint`
authorization pair, instead of invoking retired detached CLI authentication.
Only those three test imports are registered; no manager, startup, Route or
installed authority is supplied by this test.

Source offline preparation uses existing owners, without another deployment
package. `network/state.OpenFresh` delegates fresh root custody to
`network/state/durable.OpenFresh`; their exclusive new lease refuses existing
and partial attempts before normal preparation/recovery. `network/duty.Initialize`
creates only an empty first participation generation and closes its own original
lease. Its Unix O_EXCL / Windows CREATE_NEW mechanism is private to the existing
owner. `network/state.PrepareSourceStateFiles` retains the original native input and
vacant root descriptors and joins signed State acceptance and empty
participation initialization. No runtime authority or
transport is created; retained runtime openings remain noncreating.

The command shares Linux descriptor operations across Admission, document and stream
presentation in `cmd/ardents-next/command_io_linux.go`. Unsupported host
command refusals are projected by `cmd/ardents-next/unsupported_platform_other.go`.

The portable `cmd/ardents-next/stream_command.go` CLI adapter uses only
`internal/application/connection` and the standard library for its local
Application IPC and byte input/output. Endpoint retains admission authority.

The installed Reader's `operation_unix.go` and `byte_stream_unix.go` use
`internal/application/connection` for profile selection, directional limits
and the local terminal projection. The byte-stream adapter also uses the fixed
failure categories from `internal/application/document`.

Execution's `byte_stream.go` and `byte_stream_linux.go` use the fixed worker
protocol in `internal/application/document`. Its `stream_contract.go` and exact
limit test consume the participant limit from `internal/application/connection`.

The installed Publisher uses the fixed byte-stream port in
`internal/application/document` from `publisher_unix.go`, `opening_unix.go`
and `byte_stream_unix.go`. Only the last consumes Application Connection
limits and terminal classes from `internal/application/connection`.

The Linux Administration upgrade in `byte_stream_linux.go` reuses the local
framing in `internal/application/connection` after Publisher admission.


### Internal measurement edges

Only the following files admit the new capture/measurement edges. The exact
architecture policy also verifies the two SDK adapters and packet-oracle tests.

Imports of `internal/diagnostics/measurement` are permitted only from:

- `cmd/ardents-custody/main.go`
- `cmd/ardents-next/main.go`
- `internal/domain/admission/stock/measurement_unix_test.go`
- `internal/domain/admission/stock/permission.go`
- `internal/domain/admission/stock/stock.go`
- `internal/domain/connection/initiator_authentication.go`
- `internal/domain/connection/transfer/measurement_test.go`
- `internal/domain/connection/publisher_authentication.go`
- `internal/domain/connection/transfer/receive_ledger.go`
- `internal/domain/connection/transfer/send_ledger.go`
- `internal/domain/connection/stream_io.go`
- `internal/domain/custody/canonical_envelope.go`
- `internal/domain/custody/measurement_test.go`
- `internal/domain/endpoint/participant_linux.go`
- `internal/domain/enrollment/bundle.go`
- `internal/domain/enrollment/infrastructure_snapshot_linux.go`
- `internal/domain/execution/measurement_test.go`
- `internal/domain/execution/runtime/launch.go`
- `internal/domain/execution/supervisor.go`
- `internal/domain/hosting/budget.go`
- `internal/domain/hosting/measurement_unix_test.go`
- `internal/domain/network/source/transport.go`
- `internal/domain/network/state/refresh.go`
- `internal/domain/node/command/role_command.go`
- `internal/domain/publication/runtime/publisher.go`
- `internal/domain/reachability/store/store.go`
- `internal/domain/release/endpoint/initial_verification.go`
- `internal/domain/release/verification.go`
- `internal/domain/route/channel/child_capacity.go`
- `internal/domain/route/channel/frame_schedule.go`
- `internal/domain/route/channel/lane.go`
- `internal/domain/route/channel/measurement_test.go`
- `internal/domain/route/channel/queue_budget.go`
- `internal/domain/route/channel/session.go`

Imports of `internal/diagnostics/runtimeevent` are permitted only from:

- `internal/diagnostics/measurement/capture.go`
- `internal/diagnostics/measurement/capture_test.go`
- `internal/diagnostics/measurement/command.go`
- `internal/diagnostics/telemetry/measurements.go`
- `internal/diagnostics/telemetry/measurements_test.go`
- `internal/diagnostics/timeline/project.go`
- `internal/domain/admission/stock/measurement_unix_test.go`
- `internal/domain/admission/stock/permission.go`
- `internal/domain/admission/stock/stock.go`
- `internal/domain/connection/initiator_authentication.go`
- `internal/domain/connection/transfer/measurement_test.go`
- `internal/domain/connection/publisher_authentication.go`
- `internal/domain/connection/transfer/receive_ledger.go`
- `internal/domain/connection/transfer/send_ledger.go`
- `internal/domain/connection/stream_io.go`
- `internal/domain/custody/canonical_envelope.go`
- `internal/domain/custody/measurement_test.go`
- `internal/domain/endpoint/participant_linux.go`
- `internal/domain/enrollment/bundle.go`
- `internal/domain/enrollment/infrastructure_snapshot_linux.go`
- `internal/domain/execution/measurement_test.go`
- `internal/domain/execution/runtime/launch.go`
- `internal/domain/execution/supervisor.go`
- `internal/domain/hosting/budget.go`
- `internal/domain/hosting/measurement_unix_test.go`
- `internal/domain/network/source/transport.go`
- `internal/domain/network/state/refresh.go`
- `internal/domain/node/command/role_command.go`
- `internal/domain/publication/runtime/publisher.go`
- `internal/domain/reachability/store/store.go`
- `internal/domain/release/endpoint/initial_verification.go`
- `internal/domain/release/verification.go`
- `internal/domain/route/channel/frame_schedule.go`
- `internal/domain/route/channel/measurement_test.go`
- `internal/domain/route/channel/queue_budget.go`
- `internal/domain/route/channel/session.go`
- `internal/diagnostics/command/capture.go`
- `internal/diagnostics/command/report.go`
- `internal/diagnostics/command/view.go`
