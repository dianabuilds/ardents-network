# Package map

## Installed role declaration and protection policy

R01 now has ZERO canonical installed exceptions. The transitional Source,
Forwarding, JOIN and Descriptor orchestration packages and their canonical
`cmd/ardents-node` installed consumers are all retired (#604 FJD stage D,
ADR-0125); the platform-led lifecycles own installed delivery through
`cmd/ardents-next`. New Route belongs only in `internal/route`,
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
runtime-plan declaration codec (`internal/node/plan_declaration.go`)
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
runtime-plan declaration codec (`internal/node/plan_declaration.go`)
and `ardents-next node prepare-plan` (#673, ADR-0125).
Each role's semantic authority and original resource completion remain with its
existing owner; shared byte preparation does not supply a universal role policy.

Forwarding, JOIN and Descriptor share one fixed initial/forward Release
composition. The operator selects a role at dispatch. That role supplies its
exact plan schema, program/generation target pair, native resource filenames
and candidate verifier. Each operation retains its own reviewed plan, five-key
directory and original signed predecessor. The Source Timestamp purpose
belongs to the separate Source composition.

- `internal/release/infrastructure/signing/node_release_arguments_linux.go` parses public options,
  checks the selected plan and retains its exclusive output. It uses the
  standard library.
- `internal/release/infrastructure/signing/node_release_plan_linux.go` binds the fixed plan and the
  exact resources. It imports `internal/release/infrastructure` and
  `internal/platform/systemd` for the static program and canonical unit
  template contract.
- `internal/release/infrastructure/signing/node_release_linux.go` owns the joined, finite signing
  operation. It imports `internal/release/infrastructure`, `internal/release`,
  `github.com/sigstore/sigstore/pkg/signature` and
  `github.com/theupdateframework/go-tuf/v2/metadata`.
- `internal/release/infrastructure/signing/node_release_predecessor_linux.go` retains the exact
  protected public files and independently frozen candidate, checks the
  original five-key/three-threshold Root and verifies the role's coherent
  signed predecessor before forward signing. It imports `internal/enrollment`,
  `internal/release/infrastructure`, `internal/release` and
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

The command-local `release_key_receipt_unix.go` renders the public key
receipt for Source, Forwarding, JOIN and Descriptor. Each command fixes its
operation and completes its own key output before calling the formatter.
All four roles refuse a wrong argument count with the bare shared input
error; preparation failures keep their own role failure classification
(audit E1-6, #718).

Provisioning's `node_certificate_unix.go` owns the exact Node transport
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
`internal/node.RefillForwarding` for Admission and Hosting refill composition.
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
(`internal/node/plan_declaration.go`) and `ardents-next node
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
`infrastructure/forwarding_artifact.go` and its behavior tests use only the
standard library, with independently encoded Forwarding descriptor bytes.
Enrollment's existing finite inventory mechanism admits the exact separate
Forwarding seven-file inventory; Source Installation still requires Source
facts, and no runtime dispatcher or Source Startup permission is generalized.
`infrastructure/forwarding_admission.go` consumes exactly the standard library,
Enrollment and Release for its distinct opaque ForwardingAuthorization. The
shared immutable pair implementation admits neither Source Startup nor native
effects. The separate signed candidate integration in
`internal/release/infrastructure/forwarding_artifact_linux_test.go` consumes exactly those three
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

`internal/release/infrastructure/signing/source_release_predecessor_linux.go` additionally imports
exactly `internal/enrollment`, `internal/release/infrastructure`,
`internal/release`, `github.com/theupdateframework/go-tuf/v2/metadata`
and the standard library. It retains the finite public predecessor input and
checks the fixed offline forward-signing purpose; it grants no Installation or
runtime authority. `source_release_linux.go` remains the direct signing caller.
| Directory | Declaration | Responsibility and consumer | Permitted imports |
|---|---|---|---|
| `internal/release/infrastructure/launch` | `package launch` | Owns portable fixed-role declaration, credential and writable-root projection. Native pre-start joins original artifacts, loaded systemd observation, the Enrollment and Release proofs and read-only Source State inspection. Retained recovery holds the exact original plan and manager through the State repair. | `internal/admission/issuance`, `internal/canonicaldigest`, `internal/enrollment`, `internal/network/source/serving`, `internal/network/source/servingdeclaration`, `internal/network/state`, `internal/node`, `internal/pathgrammar`, `internal/platform/artifacts`, `internal/platform/process`, `internal/platform/systemd`, `internal/release`, `internal/release/infrastructure`, standard library. Linux behavior test only: `internal/processresource`. |
| `internal/release/infrastructure` | `package infrastructure` | Owns native descriptor and inventory preparation and verifies canonical infrastructure artifacts. The offline preparation retains exact inputs through exclusive output and closure using Platform Artifacts. Runtime admission verifies coherent Release pairs from original Enrollment snapshots. Platform pre-start uses runtime authorization. Source distribution inspection owns the independent history path check, Enrollment and joined Release verifier. The `ardents-next infrastructure authenticate-*` adapter supplies parsed inputs and renders the receipt. | `internal/canonicaldigest`, `internal/enrollment`, `internal/release`, `internal/platform/artifacts` through `preparation_linux.go` and `preparation_native_linux_test.go` only, `internal/pathgrammar`, standard library. Test-only `tests/releasefixture` in `authorization_lifetime_linux_test.go` and `forwarding_artifact_linux_test.go`, and `github.com/theupdateframework/go-tuf/v2/metadata` in `forwarding_artifact_linux_test.go`. The offline Release handoff in `platform_release_linux_test.go` uses `tests/releasefixture`, `tests/epochfixture/network`, `internal/network/duty`, `internal/network/state`, `internal/platform/systemd` and `github.com/theupdateframework/go-tuf/v2/metadata`. The Linux inventory refusal test uses `tests/releasefixture` for its independently signed baseline. The Source Release candidate and retained-history tests use `tests/releasefixture` and `github.com/theupdateframework/go-tuf/v2/metadata`. |

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
`platform/systemd`; the role's Linux wrapper
`state/introduction_repair_linux.go` additionally consumes exactly
`platform/systemd` and `platform/cgroup`, supplying its original read-only
manager observe closure and per-unit quiescence. The detached cross-role policy
tests (`state/node_duty_repair_linux_test.go`) consume exactly
`platform/systemd`. Unlike Source, Introduction shares the
`ardents-node` account with sibling Node role units, so quiescence is the per-unit
`cgroup/introduction_quiescence_linux.go` observation only — never an account-wide
process scan. The original Node producer identity is random, so
`duty/retained_history.go` `CheckNodeProducer` discovers the sole live `node-duty`
fence and binds it to the current generation's node identity; slot claims and spend
obligations are never repairable. The launch owner retains the original files and read-only manager through
the State repair attempt. The separate
`systemd/introduction_manager_linux.go` owns the fixed Introduction read-only
runtime and root control connections (distinct `introductionUnitReply` /
`introductionControlReply` vocabularies), granting no installed admission or
domain/native join. No old Introduction runtime is used.

Explicit Forwarding, JOIN and Descriptor repair stay with Network State and
participation as the same shared node-duty mechanism, mirroring Introduction.
Their portable role files (`state/forwarding_repair.go`, `state/join_repair.go`,
`state/descriptor_repair.go`) carry only the role sentinel and descriptor; the
shared `state/node_duty_repair.go` mechanism consumes `network/duty`,
`network/epoch` and `network/state/durable` and owns the `nodeDutyMember`
binding that ties the discovered sole live `node-duty` fence to an accepted
Candidate of the current generation with the exact authenticated family digest.
The shared Linux skeleton `state/node_duty_repair_linux.go` consumes
`network/epoch` and `platform/systemd`; the role wrappers
(`state/forwarding_repair_linux.go`, `state/join_repair_linux.go`,
`state/descriptor_repair_linux.go`) additionally consume exactly
`platform/systemd` and `platform/cgroup`; the Descriptor wrapper
additionally consumes `network/epoch` and `reachability` for the bounded
existing-only Store uncertain-stage recovery (`RepairRetainedStoreStage`) under
the same dead-unit proofs and retained leases, passed to the skeleton as the
role's optional Store-stage hook at the unchanged ordering point. The portable
behavior tests (`state/forwarding_repair_test.go`, `state/join_repair_test.go`,
`state/descriptor_repair_test.go`, `state/introduction_repair_test.go` and the
cross-role equivalence oracle `state/node_duty_repair_test.go`) consume only
`network/duty` and `network/state/durable`. All four roles share the
`ardents-node` account with sibling Node role units, so quiescence is the per-unit
`cgroup/forwarding_quiescence_linux.go` / `cgroup/join_quiescence_linux.go` /
`cgroup/descriptor_quiescence_linux.go` / `cgroup/introduction_quiescence_linux.go`
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
profile is `processresource.DescriptorProfile`. The Store lease is flock and dies
with the process, so the Store root carries no durable fence to remove; the
bounded repair instead validates every committed publication floor and removes
only an uncertain `.record-stage` under the exclusive Store lease, never
initializing a missing root, resetting/evicting a floor, promoting staging or
deleting foreign entries. Slot claims, spend obligations and publication floors
are never repairable. The JOIN and Descriptor declarations' process profiles are
`processresource.JoinProfile` and `processresource.DescriptorProfile`, not the
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
| File `internal/network/state/source_repair_linux.go` | File `internal/platform/systemd`; `internal/platform/cgroup`; `internal/platform/process` |
| File `internal/network/state/source_repair_linux_test.go` | File `internal/platform/systemd` |
| File `internal/network/state/node_duty_repair_linux.go` | File `internal/network/epoch`; `internal/platform/systemd` (shared node-duty repair skeleton, W4 #711) |
| File `internal/network/state/node_duty_repair_linux_test.go` | File `internal/platform/systemd` |
| File `internal/network/state/introduction_repair_linux.go` | File `internal/platform/systemd`; `internal/platform/cgroup` |
| File `internal/network/state/forwarding_repair_linux.go` | File `internal/platform/systemd`; `internal/platform/cgroup` |
| File `internal/network/state/join_repair_linux.go` | File `internal/platform/systemd`; `internal/platform/cgroup` |
| File `internal/network/state/descriptor_repair_linux.go` | File `internal/network/epoch`; `internal/platform/systemd`; `internal/platform/cgroup`; `internal/reachability` (bounded existing-only Store stage recovery) |
| File `cmd/ardents-next/platform_linux.go` | `internal/release/infrastructure/launch` for retained infrastructure launch. The native command also opens and closes the shared `internal/platform/artifacts` input owner. |
| File `cmd/ardents-next/release.go` | Files `internal/enrollment` and `internal/release` for initial and candidate Release inputs used by both software delivery paths |
| File `cmd/ardents-next/platform_endpoint_staging_linux.go` | File `internal/enrollment`; `internal/release/endpoint`; `internal/endpoint/launch`; `internal/release` for first-delivery floor verification and signed successor staging under the retained Release lease |
| File `internal/release/infrastructure/platform_release_linux_test.go` | File `internal/enrollment`; `internal/release`; `internal/release/infrastructure`; `internal/network/state`; `internal/network/duty`; `internal/platform/systemd`; `tests/epochfixture/network`; `tests/releasefixture`; `github.com/theupdateframework/go-tuf/v2/metadata` |

The Source CLI and its process fixtures consume `servingdeclaration` directly
in the exact files listed by the import guard. File custody stays with the
native adapters, and composition converts Source's embedded network
declaration only where it builds the existing State configuration.

`internal/network/state/source_state_files_linux.go` owns Source State
preparation from the original public files through both retained histories.
It uses `network/source`, `network/source/servingdeclaration`,
`platform/artifacts`, `platform/process` and `processresource`.
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

`internal/release/infrastructure` owns portable canonical descriptor preparation
and verification for infrastructure artifacts. Its Linux admission binds the
program and descriptor to fresh Release proofs and the original Enrollment
inventory. `internal/platform/systemd` owns `PlatformUnitTemplate`. The
platform command and offline Control bind its exact bytes. The artifact
package consumes only `internal/enrollment`, `internal/release` and the
standard library.

Public artifact preparation uses the `ardents-next infrastructure` adapter,
with the fixed invocation selected by the operator. It consumes
`internal/release/infrastructure` for portable descriptor encoding and
resource validation, and `internal/enrollment` for the exact native inventory.
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

`internal/endpoint/source_trust_preparation_linux.go` owns the offline Source
public preparation. It reuses the Endpoint declaration types, Source and
serving declaration owners, exact process profile and Platform Artifacts for
native input/output. `internal/network/source/command/prepare_linux.go` adapts
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

Initial Network key preparation belongs to `internal/node/provisioning`
(`initial_network_keys_linux.go`), which admits the closed plan, creates the
State key and distinct Node keys and returns the exact published public receipt.
The CLI projects it through `network_preparation.go`. Provisioning tests
exercise expiry after native I/O and durable signing inventory publication on
ext4, including original Closed Profile commit and output contention, retained
key custody, and physical child interruption for Record, Profile and Certificate. The same owner tests unsigned publication through original file faults,
partial writes and child interruption. The CLI retains the genuine
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
preparation and signing to `internal/node/provisioning`. The corresponding CLI
files parse arguments and project receipts and statuses. The exact declaration, size
limit, original input custody and finite publication live in Provisioning. The
portable preparation codec uses Epoch, Closed Profile and Platform Artifacts.
Linux-only files bind the original reviewed commitment, frozen validity interval,
protected key and purpose-specific Record, Epoch, Profile or certificate output.
The private key stays with the signing owner. The command checks the retained
interval around the public signing receipt.

`network_signing_declaration_unix.go` consumes Epoch, Closed Profile and
Platform Artifacts. `network_profile_signing_unix.go` uses Closed Profile.
`network_certificate_signing_unix.go` uses Epoch and the shared certificate
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
(`network_preparation_linux.go`, `artifacts/output_linux.go`,
`artifacts/protected_input_linux.go`, `artifacts/protected_storage_linux.go`),
the Enrollment infrastructure inventory and its `/proc/self/exe` running-program
proof, and the systemd Release plans. A macOS storage floor and a
security-equivalent macOS process-identity capability remain separate Product
Owner decisions; `os.Executable` is not equivalent to `/proc/self/exe`.

`internal/node/command/report.go` imports exactly
`internal/node` and the standard library to encode its copied original
first-failure projection inside the established single terminal report. Node's
`failure_projection.go` imports only `processresource` and the standard
library; it records local phase/cause/elapsed facts without foreign I/O, authority
or private identity. P1-B batch 2 (#787, #691) dropped its accidental Linux
constraint after per-file purity verification: it composes only the portable
processresource failure projection and standard-library imports, with no native
mechanism. Its portable behavior tests execute on the windows/macos lane; the
physical first-failure join test still composes the Linux forwarding lifetime and
stays Linux (`failure_projection_linux_test.go`). The Linux-only
`admission_projection.go` public-observation projection was likewise de-tagged in
batch 2 (it composes only portable `admission`/`network` facts) with a new
portable behavior test. Process-resource failure facts retain original numeric
measurements separately from Node's physical cleanup causes.
`processresource/failure.go` owns the copied failure projection portably: P1-B
batch 1 (#781, #691) merged the former `failure_linux.go` `Failure` projection
into the already-portable fact representation after verifying it is pure
(`errors` only, no native mechanism). Its portable behavior test executes on the
windows/macos lane. Its genuine Node consumer `node/failure_projection.go` is now
portable, but that projection's own command consumers remain Linux
(`internal/network/source/command/serve_linux.go`, `internal/node/command/report.go`), so
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

The installed exchange scenario in `internal/endpoint/endpoint_journey_native_linux_test.go`
consumes `internal/application/administration`, `internal/application/connection`
and `internal/application/document` for actual public client operations.
Its local controller uses `tests/e2e/endpoint/pair_messages_linux.go` to transfer
public signed Network inputs and actual permission requests and responses between
separately installed participants. It neither imports
private Instance/Publication runtime nor transfers a live domain owner.

The five runtime roles share joined invocation supervision in `internal/node`.
The supervisor waits for physical completion before releasing the borrowed
Network authority, including after a retirement timeout.
`cmd/ardents-next/node_roles.go` imports `network/state`, `node`
and `route/role` to acquire the original State, bind the constructor and render
the CLI result on supported hosts. The command passes the original State release
to the supervisor. `internal/node/runtime_linux.go` binds each typed declaration
to its native constructor. Its portable method set has the same Authority and
Issuer plan types on other hosts and refuses startup.

The original JOIN command in exactly `cmd/ardents-next/node_roles.go`
additionally imports `internal/node` and `internal/processresource`.
`node_join_native_linux_test.go` additionally imports `processresource` and
`route/transport` and `network/duty`;
`node_authority_native_linux_test.go` additionally imports `node`,
`network/duty` and `route/transport`; `node_completion_native_linux_test.go`
additionally imports `network/duty` and `route/transport`;
`node_operation_native_linux_test.go` additionally imports `node` and
`route/transport`. All these abbreviated paths are under `internal`.
Other imports remain the command row's exact existing owners. These native
controls delay genuine observations; they provide no accepting authority.

The platform-led JOIN retained evidence and repair (#604 FJD stages A+B+C,
ADR-0125) is exercised by `tests/e2e/node/retained_lifecycle_native_linux_test.go` (`node_process_native`; imports
`processresource`, `network/duty` and `route/transport`) and
`join_platform_native_linux_test.go` (`join_platform_native`; imports
`release/infrastructure`, `network/duty`, `processresource` and
`route/transport`). The lifecycle test proves retained relaunch and SIGKILL
crash refusal for one single-unit JOIN member; the installed gate proves the
real `ardents-join.service` platform-led lifecycle including the bounded
`node join repair-retained` against real failed-unit manager facts. The
transitional JOIN orchestration is not exercised or retired by these files;
that retirement is the separately authorized stage D.

The platform-led Descriptor retained evidence and repair (#604 FJD stages
A+B+C, ADR-0125) is exercised by `tests/e2e/node/retained_lifecycle_native_linux_test.go` (`node_process_native`; imports
`processresource`, `network/duty`, `route/transport` and `reachability` for the
genuine fresh Store-root provisioning and post-crash retained reopen) and
`tests/e2e/node/descriptor_platform_native_linux_test.go` (`descriptor_platform_native`;
imports `release/infrastructure`, `network/duty`, `processresource`
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

The original Introduction command in `cmd/ardents-next/node_roles.go`
additionally imports exactly `internal/node` and
`internal/processresource`. The exact `node_introduction_native_linux_test.go`
additionally imports `internal/processresource`, `internal/network/duty`,
`internal/route/capsule` and `internal/route/transport`.
`node_authority_native_linux_test.go` additionally imports exactly
`internal/node`, `internal/network/duty` and
`internal/route/transport`; `node_completion_native_linux_test.go`
additionally imports exactly `internal/network/duty` and
`internal/route/transport`. `node_operation_native_linux_test.go` additionally imports exactly
`internal/node` and `internal/route/transport`. These genuine native composition tests
supply no accepting State, History, process or recipient substitute.

The original Forwarding consumer in `cmd/ardents-next/node_roles.go`
additionally imports exactly `internal/node` and
`internal/processresource`; `internal/node/command/process.go` imports only the latter.

The original Descriptor consumer in `cmd/ardents-next/node_roles.go`
additionally imports exactly `internal/node` and
`internal/processresource`. Node's `descriptor_unix.go` additionally
imports exactly `internal/reachability` for the genuine original Store;
that import grants no other Node source a Reachability or proof-policy boundary.
These exact command files extend the command row, without granting these
dependencies to other composition files.

The exact `node_native_linux_test.go` scenario additionally imports
`internal/processresource`, `internal/network/duty`, `internal/route/transport` and
the existing independently encoded `tests/epochfixture/network` input owner.
It starts genuine canonical commands with actual native placement; these imports
grant no adapter construction or authority shortcut.

The exact `internal/node/refill_process_native_linux_test.go` scenario imports
`internal/processresource` and `internal/route/transport`
to select the same actual command profile and real client Carrier. Its other
imports use Admission, Hosting and signed process fixtures. It grants no accepting substitute for the Node runtime.

The exact `node_authority_native_linux_test.go` additionally imports
`internal/node`, `internal/network/duty` and
`internal/route/transport`. This native test composition opens genuine
State and Node owners with signed inputs prepared outside the measured cgroup.
Its scheduling barrier delays only a completed real State observation; it
cannot construct successful authority or replace the actual command consumer.

The exact `node_completion_native_linux_test.go` additionally imports
`internal/processresource`, `internal/network/duty` and
`internal/route/transport` to exercise the actual command's bounded
unavailable-completion result against a genuinely occupied producer-root lease.
The lease is a test fault, not a live shared-root design or a recovery right.

This is the factual naming and dependency registry for maintained Go code. It
lists packages that exist, not future intentions. A new entry is an architecture
change: name the responsibility first, add the real Implementation and tests,
then update this table in the same change. The normative growth and dependency
rules are in [repository-layout.md](repository-layout.md).

The checked installed component's
`internal/endpoint/connection_response_loss_linux_test.go` additionally imports
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
grammar is independently owned: only runtime `initialization_linux.go` borrows
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
`permission_linux.go` retains write-once/exact retry and original parent checks.
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
| `internal/processresource` | `package processresource` | Mandatory separate Forwarding, Issuer, Descriptor, Introduction and JOIN development process profiles, original Linux process/cgroup pins, actual placement and resource observations, elapsed CPU interval and retained fail-closed pressure/currentness decision. It owns no Node, State, participation, Admission, Hosting or Route authority. Real consumers: `node` original Forwarding/Issuer/Descriptor/Introduction/JOIN and `cmd/ardents-next/node_process.go`. | standard library |
| `internal/durablefile` | `package durablefile` | Shared pure durable-file mechanism for the owner-only generation stores (R1, #764): the content-addressed generation-name grammar and SHA-256 naming, bounded reads, exclusive owner-only creation, staged same-directory replacement, generation pruning, the watermark floor codec, owned-root inspection, directory synchronization, the exclusive Unix root lease with its strict regular-file/single-link identity check and the Windows owner-only root DACL enforcement (#793): a protected DACL granting only the current process owner SID full control, inherited by future children and applied without inheritance to every existing regular file, refusing whenever enforcement fails. It is a mechanism leaf, not a store owner: every helper takes caller paths, bytes, names and refusal prose only, and holds no lease, root, marker, schema, authority or lifecycle of its own. Each consumer keeps its distinct marker, lock name, record schema, root and refusal text; the shared lease identity check refuses a non-regular or multiply-linked lease file so a substituted inode can never become a store's exclusive fence. Real consumers: `internal/route/selection`, `internal/network/duty` (durable-file mechanism and Windows owner-only DACL enforcement), `internal/network/state/durable` (durable-file mechanism) and `internal/release` (generation-name grammar). `internal/publication/instance` retains its separate joined-error, TOCTOU-hardened private custody and does not consume this mechanism. The Unix-family (Linux and macOS) inspection port (#850) owns the directory-relative open, the normalized inode change/modification times and the shared full-identity comparison consumed by `internal/application/document`, `internal/endpoint/generation`, `internal/endpoint/launch`, `internal/endpoint/plan`, `internal/node`, `internal/node/provisioning` and `internal/platform/artifacts`; the two Stat_t time field spellings stay at that boundary. The directory-relative atomic rename (Cohort 1 N2, #853) extends the same port for the Introduction slot-history staged replacement consumed by `internal/route/introduction`. It grants no installed restart or native qualification. | standard library; `golang.org/x/sys/windows` only in `owner_root_windows.go` and `owner_root_windows_test.go`; `golang.org/x/sys/unix` only in `relative_open_unix.go` and `relative_rename_unix.go` (Unix-family inspection port, #850; directory-relative rename, #853) |
| `internal/pathgrammar` | `package pathgrammar` | Shared pure path-validation grammar for the maintained owners (G7, #772): strict predicates for absolute, already-clean host paths and lexically disjoint root declarations, plus a separate literal POSIX argument predicate shared by platform launch, plan and unit rendering. They perform no filesystem observation and never absolutize relative paths. It is a mechanism leaf, not a root or custody owner: it holds no root, marker, schema, authority or lifecycle of its own, and each consumer keeps its distinct refusal prose plus any additional root, NUL-rune or length bounds it genuinely needs. It replaces the formerly repeated inline absolute/clean checks whose variants drifted across owners. Deliberate boundaries stay separate: the POSIX installation declaration continues to validate its own wire grammar with the platform-independent path package, the Application zone keeps its purity, and configured-root resolution operations (which legitimately absolutize a relative root) are not this validation grammar. Network State configuration validation now refuses relative and non-clean paths instead of silently absolutizing them — the one deliberate behavior tightening, verified against every production caller, which already supplies absolute clean paths. Real consumers: `internal/admission/attempts`, `internal/admission/issuance`, `internal/admission/issuer`, `internal/admission/issuerprofile/signing`, `internal/admission/quota`, `internal/admission/spending`, `internal/endpoint`, `internal/endpoint/funding`, `internal/endpoint/generation`, `internal/endpoint/publisher`, `internal/endpoint/session`, `internal/enrollment`, `internal/execution/worker`, `internal/hosting`, `internal/network/duty`, `internal/network/state`, `internal/network/state/durable`, `internal/node`, `internal/platform/artifacts`, `internal/platform/cgroup`, `internal/platform/process`, `internal/reachability`, `internal/release`, `internal/release/infrastructure` and `internal/route/introduction`. It grants no installed restart or native qualification. | standard library |
| `internal/canonicaldigest` | `package canonicaldigest` | Shared pure canonical hex-32 validation atom for the maintained owners (Phase 4, #835): one decode predicate admitting exactly 64 lowercase hexadecimal characters whose decoded 32 bytes re-encode to the identical string, plus its validation-only projection. It performs no hashing and no filesystem observation. It is a mechanism leaf, not a root or custody owner: it holds no root, marker, schema, authority or lifecycle of its own, and each consumer keeps its distinct refusal prose plus any additional zero-value, presence or length bounds it genuinely needs. It replaces the formerly repeated private `canonicalDigest` predicates, provisioning `decodeHex32` and inline decode/length/round-trip checks whose variants drifted across owners. Deliberate boundaries stay separate: variable-length canonical hex (Issuance and command admission inputs, closed network preparation text), the weaker no-round-trip Release floor and fixed-hex decoders, the 16-byte systemd InvocationID shape, and Custody authority validation are not this atom. Release builder-attestation validation joins the canonical atom because the same package's verification already requires exact equality with `hex.EncodeToString` of the authenticated digest — the one deliberate tightening, which removes no reachable acceptance. Real consumers: `internal/enrollment`, `internal/endpoint`, `internal/endpoint/generation`, `internal/endpoint/launch`, `internal/endpoint/plan`, `internal/endpoint/unit`, `internal/execution/worker`, `internal/hosting`, `internal/network/state`, `internal/node/provisioning`, `internal/platform/process`, `internal/publication/instance`, `internal/release`, `internal/release/infrastructure`, `internal/release/infrastructure/launch` and `cmd/ardents-next` (`source_inspection_linux.go`). It grants no installed restart or native qualification. | standard library |
| `internal/node` | `package node` | One original Forwarding, Issuer, Descriptor, Introduction or JOIN runtime startup, admission/retirement latch, actual new receiving/Hosting/Route composition, original participation producer, purpose-specific runtime TLS key custody, physical join and retained completion. Introduction retains one prepared Route Registry/History through synchronous denial and physical join, without slot/storage policy. Descriptor composes a genuine separately owned Reachability Store, without proof/conflict/storage policy. Owns the original node-role runtime invocation declaration grammar (`plan_declaration.go`): a pure closed byte codec mirroring `network/source/servingdeclaration.ServingDeclaration`, shared by the runtime, the retained repair commands and `node prepare-plan`. This is configuration validation kept with the owner (ADR-0125 decision 6), returning declaration facts only; it is not effectful provisioning and supplies no universal role policy. No State acceptance, token/spend policy, provider-period policy, transport implementation, Source, effectful provisioning/signing, Service or unselected Node role. Real consumers include the runtime CLI composition in `cmd/ardents-next/node_roles.go`, Node provisioning and retained infrastructure repair in `internal/release/infrastructure/launch`. | `internal/durablefile`, `internal/admission`, `internal/admission/receiving`, `internal/hosting`, `internal/network`, `internal/network/duty`, `internal/processresource`, `internal/route/ardp`, `internal/route/receiver`, `internal/route/role`, `internal/pathgrammar`, standard library; `internal/admission/issuer`, `internal/admission/quota` and `internal/route/bootstrap` through `issuer_unix.go` only; `internal/reachability` through `descriptor_unix.go` only; `internal/route/introduction` through `introduction_unix.go` only; `internal/hosting` and standard library through `plan_declaration.go` (the pure declaration grammar) The native refill owner test also uses `internal/admission/issuer`, `internal/admission/quota`, `internal/admission/stock`, `internal/admission/token`, `internal/endpoint/funding`, `internal/network/state`, `internal/route/ardp`, `internal/route/transport`, `internal/route/transport/tls`, `tests/e2e/node/processfixture` and `tests/epochfixture/network`. The native Introduction exchange also uses `internal/endpoint/session`, `internal/network/epoch` and `internal/route/capsule`. The native operation workload drivers use the same Endpoint and Network owners. The native authority tests also use `internal/admission/issuance`, `internal/admission/spending`, `internal/reachability` and `internal/route/introduction`. ; `internal/platform/artifacts`, `internal/route/channel` ; `tests/networkfixture` in `authority_process_native_linux_test.go`, `introduction_exchange_native_linux_test.go`, `operation_workload_native_linux_test.go`, `refill_process_native_linux_test.go` ; `internal/route/selection` in `route_refusal_linux_test.go` to select the actual Forwarder before observing joined post-spend refusal and replay after reopen. ; `internal/publication`, `internal/publication/durable` and `internal/publication/instance` only in `publication_registration_linux_test.go` to exercise the original Registration receipt in Instance and Publication. ; `internal/route/prefix` in `route_submission_linux_test.go` opens the physical Introduction Prefix before transmitting and checking the refused Source submission. ; `internal/admission/allocation` in the JOIN fixtures for independent signed allocation and `internal/route/join` in `route_join_lifecycle_linux_test.go` for the original Source rendezvous binding. |
| `internal/node/command` | `package command` | Inspect the local process without owning Node role policy. | `internal/processresource`, standard library |
| `internal/execution` | `package execution` | Volatile exact Principal/surface capability generation, independent 64 Connection and 6 combined Administration admission limits; exact Session current/last Job and once-only lifetime and operation claims, synchronous revoke and first immutable cleanup result; a distinct supervisor retains physical cleanup capacity after revoked leases and permanently after failed cleanup. Real consumer: installed Execution preparation and live invocation runtime. Local rules grant no installed confinement, Service, Network or token authority. | standard library |
| `internal/execution/worker` | `package worker` | Fixed root-installed worker artifact and manager inventory, selected Ubuntu 22.04/24.04 systemd 249/255 ordinary Execution admission, credentialed attachment rejecting and closing received descriptors, original pinned cgroup observation and finite original-invocation stop/join. Failed Stop still observes original descendants while retaining failure; original helper threads survive through physical child join. Public Activate/ObserveInstance selects only the closed Text inventory; the trusted qualification entries ActivateStream/ObserveInstanceStream select the fixed Stream qualification inventory (identical confinement contract; qualification identity, owner slice and nested qualification cgroup root). No caller-visible inventory selector exists; the architecture gate restricts Stream callers to the trusted qualification launch path, and each inventory refuses the other's units/slices. The unsupported-platform adapter returns only failure and no activation/artifact/cleanup pin. Internal manager/file observations are private, and original cleanup reports its first definite failure to the bound launch owner before further descendant waiting. Real consumer: Execution preparation and live invocation runtime; the installed Stream activation test is the bounded qualification launch path until the accepted runner lands. Mechanisms grant no authority or readiness. The actual loaded socket is checked before activation and again at original invocation/READY observation through the pure fixed-unit contract. Installation admission remains separate. The attachment's AF_UNIX/SCM_RIGHTS transport is Unix-family (Linux and macOS, `attachment_unix.go`, #852); per-frame peer-credential authentication is a capability port at that boundary — Linux `SO_PEERCRED`/`SO_PASSCRED`/`SCM_CREDENTIALS` authenticates every frame (`peercred_linux.go`), macOS exposes only connect-time `LOCAL_PEERCRED` and refuses with an explicit typed unavailable outcome (`peercred_darwin.go`), and non-Unix platforms refuse the attachment (`attachment_refusal_other.go`). A missing required protection is never a weaker accepting path (ADR-0129); the systemd/cgroup activation, artifact and original-invocation mechanisms remain genuine Linux. | `internal/platform/systemd` for detached typed values; `internal/endpoint/unit` for the expected fixed socket contract; `internal/application/streamqualification` through `stream_activation_installed_linux_test.go` only; `internal/canonicaldigest`, `internal/pathgrammar`, standard library; `golang.org/x/sys/unix` only in `peercred_darwin_test.go` (macOS peer-credential refusal evidence, #852) |
| `internal/execution/runtime` | `package runtime` | Common portable composition consumes exact local Session, joins unsupported native refusal without a qualified handle or participant effects, serializes process-wide fixed activation inventory, claim and pin cleanup before Text INIT, independently reobserve original artifact/invocation after READY, create fresh private byte Grant and close/join it before returning separate completed-current preparation provenance. Failed ambiguous cleanup retains the inventory gate. Genuine consumers: `ardents-next execution prepare-permission`, `execution route-holder` and the independently admitted `endpoint start-installed` Source/permission bootstrap, retaining real Stock and signed Network/Route composition. Live Invocation lends one sealed operation, interrupts before borrower join, retains first physical failure synchronously and joins original worker cleanup before exact completed-current handoff. Bounded actual-manager installed Source/permission startup has a genuine command consumer. Qualified Reader and selected-snapshot Publisher retain Text's sole protocol reader and transfer one authenticated Connection stream under the original operation; pending Reader output remains private until Source, operation and original worker join with completed-current checking. A private worker protocol parameterizes the one fixed launch flow (gate/artifact/Job/Grant/lease/joined cleanup) over the closed Text inventory and the trusted Stream qualification inventory; no caller-visible selector exists. Trusted LaunchStreamQualification/RunStreamConnections admit the fixed Stream qualification worker (INIT/READY, installed reobservation) and run the fixed qualification bridge over the retained attachment inside the original invocation through an authority-rechecking borrow, one transfer attempt per invocation, joining the attachment and every supplied authenticated Service stream; no attachment is exported and ordinary Text one-use claims are unchanged. The architecture gate restricts the qualification seams to the trusted qualification composition. The complete installed NET-14 qualification runner remains separate. | `internal/execution`, `internal/execution/worker`; `internal/application/document` through `initialization_linux.go`, `publisher_worker_linux.go`, its Linux behavior test and `reader_worker_linux.go` only; `internal/application/connection` through `publisher_worker_linux.go`, `publisher_stream.go`, `reader_worker_linux.go` and `reader_stream.go` only; `internal/application/streamqualification` through `stream_qualification_linux.go`, `stream_qualification_other.go` and `stream_qualification_installed_linux_test.go` only; standard library |
| `internal/enrollment` | `package enrollment` | Authenticate an independently pinned portable v6 Endpoint bundle, exact original program and optional complete protected group; retain private copied byte provenance. Root-contained bounded actual reads and inventory, original cancellation and justified native ownership mechanisms. Read-only `Bundle.MetadataNames` classifies metadata outside trusted-root/static companion inventory using the same grammar identities. Separate `ReadCandidate` retains an opaque untrusted snapshot without initial-pin or running-program provenance; its declarations and byte copies grant no successor authority. Both results use one private bounded inventory and projection implementation; the genuine fresh-pair consumer is `cmd/ardents-next/installation.go`. `ValidateProtectedGeneration` supplies the same canonical static-group grammar to new Installation inspection without pin provenance or filesystem/Release authority. `ValidateInfrastructureInventory` checks the closed seven-file infrastructure byte grammar and returns fresh copied declarations/digests; its transitional JOIN/Descriptor selected-generation validation consumers retired (#604 FJD stage D, ADR-0125), so it is retained as reviewed mechanism code with genuine native coverage (see the deadcode allowlist) and supplies no pin, native custody, Candidate, Bundle or running-program proof. `ExecutableArtifactName` owns the same platform artifact identity used internally and by the two read-only packaging operations in `cmd/ardents-next/enrollment.go`. No Release inputs/URLs, signing, installation, execution, Network or old runtime authority. Actual consumers: `cmd/ardents-next/enrollment.go`, `cmd/ardents-next/release.go` and `cmd/ardents-next/installation.go`; the latter consumes authenticated facts, metadata projection and frozen bytes. | `internal/canonicaldigest`, `internal/pathgrammar`, standard library |
| `internal/route` | `package route` | Own exact role-purpose eligibility, participant conflicts and retained Interior pair rules; grant no Network, token or physical authority. | standard library |
| `internal/route/selection` | `package selection` | Own independent exclusive installation Entry and local Interior roots, canonical committed pairs, context-local Rendezvous choices and bounded alternative, original retention and time floors; consume coherent Network observations and retain exact duties without redraw on failure. Endpoint first preparation uses `InitializeInstallation` for empty Entry/role histories; funding uses `OpenRetainedInstallation` and retained role borrows, which cannot initialize lost history. Portable Entry-pair eligibility/draw/bounds, retained Leg checks and recipient rules compose with the durable selection owner and root lease/permission mechanisms; all are Unix-portable (Flock, O_NOFOLLOW, Stat_t.Uid) with no Linux-only dependency (ADR-0125 portable core). The Windows owner-only floor enforces the shared protected owner-only DACL through `internal/durablefile` (#793), because Go synthesizes Windows mode bits from FILE_ATTRIBUTE_READONLY rather than the DACL; the Unix floor keeps its read-only permission-bit and endpoint-UID checks unchanged. The closed Entry set and local Interior set share one parameterized generation-store mechanism (`generation_store.go`, R2 #764) over the shared `internal/durablefile` durable-file leaf; each keeps its own marker, schema, validator, root lease and refusal prose, and the Entry lease enforces the same strict regular-file/single-link identity as the duty and State durable owners (G1 #764). | `internal/route`, `internal/network`, `internal/durablefile`, standard library; `golang.org/x/sys/windows` only in `root_permissions_windows_test.go` |
| `internal/route/ardp` | `package ardp` | Own canonical generation-three bounded frame/HELLO, OPEN recipient/envelope encoding JOIN and Descriptor lookup/publication operation/result grammar; reject impossible headers before body reads or allocation. Decoded OPEN fields grant no current duty, role or deadline authority; channel and operation state own permitted effects. | standard library |
| `internal/route/transport` | `package transport` | Own portable ordered-stream/exporter and Node/role request/shared-listener contracts, bounded TLS record writes through original physical capacity projections, exact accepted Carrier/ALPN identities, literal endpoint/pre-effect bounds, shared certificate/TLS authentication and direct/Node classification. Peer-handshake and physical-retirement categorization retain original causes and cannot hide an unrelated aggregate leaf. A separate unstarted-write marker retains failed output provenance only; raw/mixed closed errors grant neither that marker nor peer classification. Actual physical adapters consume these same requirements. No concrete adapter, channel framing, Network/Admission or durable-root authority. | standard library |
| `internal/route/transport/tls` | `package tls` | Own actual literal TCP role/Node dial, inner role TLS client/server handshake over selected lanes, shared direct/Node TCP listener, finite handshake capacity, absolute bounds, deadline reset and physical Node socket interruption. Original native socket-write admission beneath TLS distinguishes wholly unstarted Node output from partial/started physical failure; neither refusal nor zero output grants success. Shared transport owns requests/authentication/classification; supplied raw connections remain the caller's ownership on refusal, while failed dials close the adapter's own socket. No path, token, durable-root or QUIC authority. | `internal/route/transport`, standard library |
| `internal/route/channel` | `package channel` | Own portable framing: sole reader, serialized writer, bounded principal memory, lanes, scheduling, credit/debits, parent exchanges and joined physical retirement with retained original failures. Projects only its own immutable frame overhead for nested TLS record sizing; the projection grants no live credit or authority. The selected writer publishes actual physical counts/failure before becoming inactive; nested output separately retains real lower payload progress and actual lower physical failures, so successful lower bytes followed by retirement remain failed upper work without inventing cleanup failure. Consumes selected transport and caller currentness; no concrete adapter, Network/Admission/Hosting decisions or durable root. Real prefix, receiver and JOIN callers consume this mechanism. Also owns `MinDeadline`, the one shared pure setup/cleanup bound for prefix, pairing and receiver: the earlier original deadline wins and equal deadlines keep the first argument's exact reading; it grants no deadline authority. | `internal/route/ardp`, `internal/route/transport`, standard library |
| `internal/route/bootstrap` | `package bootstrap` | Own one duty's finite issuer-bootstrap live claims, per-physical-adjacency cap, original 10-second bounds, shared queued-work budget and aggregate output credit. Receiving Node children reserve before TLS and transfer capacity return through the actual lane finish. No Network authority, Admission permission/quota/signing, token stock or ordinary channel upgrade. | `internal/route/ardp`, `internal/route/channel`, standard library |
| `internal/route/issuer` | `package issuer` | Own portable single-terminal request/result exchange, nonce/outcome agreement and role TLS termination for actual Prefix and Receiver callers. No listener, selection, Stock completion, token spend/quota/signing, physical queue/reservation or root authority. | `internal/admission`, `internal/route/ardp`, standard library |
| `internal/route/role` | `package role` | Own portable original-duty/profile reobservation, exact HELLO purpose/recipient/bounds, negotiated role exporter binding and holder HELLO/Stock-presentation/ADMIT/ACCEPT ordering with original-caller checks. Admission supplies the actual durable presentation; receiving owns Grant transfer and joined physical retirement. No intake authentication, replacement selection, spending, Hosting reservation or root. | `internal/network`, `internal/admission`, `internal/route`, `internal/route/ardp`, `internal/route/channel`, `internal/route/transport`, standard library |
| `internal/route/join` | `package join` | Own exact Source/Responder acquisitions, holder attempt/Context and Joined stream over opaque Prefix contracts. Prefix owns generations; native selection owns roots. The separately receiving Rendezvous matching, two-RESULT barrier and relay accounting/credit live in the `join/pairing` subpackage, which Receiver consumes directly without the holder's Prefix/selection imports. No token spend, listener or Service authority. | `internal/route/prefix`, `internal/route`, `internal/route/selection`, `internal/route/ardp`, `internal/route/channel`, `internal/route/role`, `internal/route/transport`, `internal/network`, `internal/admission`, standard library |
| `internal/route/join/pairing` | `package pairing` | Own one exact receiving JOIN duty's Rendezvous matching: two complete RESULT writes before data, relay accounting/credit and joined physical retirement of both original sides. Capacity reserves one side's bounded frame memory from the receiving principal's shared budget before irreversible admission spend and releases it exactly once after Serve joins. Receiver supplies admitted channels and retains actual Grant/Hosting return until Serve completes; pairing neither verifies nor releases an Admission Grant. No holder acquisition/generation, durable root, selection, listener, token spend or Service authority. Real consumer: `internal/route/receiver` through `receiver_unix.go` and `admission_retirement_unix.go`. | `internal/route/ardp`, `internal/route/channel`, `internal/route/role`, standard library |
| `internal/route/prefix` | `package prefix` | Own portable exact Entry/Interior physical generations, pending setup and sealed borrows, original Source/Responder pair commits, authenticated terminal openings, separate finite issuer-only bootstrap, exact-generation issuer and Descriptor exchanges, parent refill and joined resource return. Reachability owns proof/history completion after the original physical exchange joins. Actual Registration/JOIN consumers retain opaque claims; no listener, durable root, token spend or Hosting budget. | `internal/network/state` through `prefix.go` only for the opaque original RuntimeWitness refusal check; `internal/route`, `internal/route/selection`, `internal/route/ardp`, `internal/route/issuer`, `internal/route/bootstrap`, `internal/route/channel`, `internal/route/role`, `internal/route/transport`, `internal/route/transport/tls`, `internal/network`, `internal/admission`; `internal/reachability` through `descriptor_exchange.go` only; standard library |
| `internal/route/receiver` | `package receiver` | Own actual listener, exact-purpose dispatch to Introduction Registry, JOIN Pairing, genuine issuer composition and separately owned Descriptor Store, directed Node Carrier borrowing and forwarding children, receiving Admission composition and transferred reservations through joined physical retirement. Recheck the current Introduction assignment before Descriptor effects/output; Store owns proof/conflict/durability decisions. No holder operation/generation, token quota/spend policy or Hosting budget. | `internal/route`, `internal/route/transport`, `internal/route/transport/tls`, `internal/route/channel`, `internal/route/bootstrap`, `internal/route/role`, `internal/route/ardp`, `internal/route/issuer`, `internal/route/introduction`, `internal/route/join/pairing`, `internal/network`, `internal/admission`, `internal/admission/receiving`; `internal/reachability` through `receiver_unix.go` and `descriptor_unix.go` only; standard library |
| `internal/network` | `package network` | Own Candidate View eligibility, rejection precedence, collision exclusion, family totals and deterministic Role Domain assignment; own the accepted current/pending Epoch relation, conflict refusal, exact successor selection and completion-time validity; bind signed role assertions to exact authenticated Node Records and expose copied members with unambiguous identity/key lookup and bounded validity. Evaluate every authenticated observation before choosing a generation. Own separate local-participation restriction decisions, producer-scoped replacement, live Source retention, identity/family collisions and finite record/producer/exposure bounds; own finite Acquisition attempts, recovery, original deadlines/order, exposure history and backoff inside the State commit boundary; own profile history, trusted-time confidence, coherent AcceptedState observations with copied public issuer inventory and exact issuer/duty binding, and retained-duty binding. Proposals do not publish authority; the State application adapter verifies inputs and commits effects. | standard library |
| `internal/network/canonicalcursor` | `package canonicalcursor` | Network-domain pure bounded cursor for the canonical Network byte grammars (R5, #804): bounded reads over one caller-supplied canonical byte slice, big-endian uint16/uint32/uint64/int64 readers, the exact consumed-offset observation and the single "truncated canonical bytes" bounds refusal formerly repeated byte-identically by every consumer. Reads alias the caller's bytes and never copy. It is a Network-domain mechanism leaf, not a grammar or authority owner: every consumer is a network package, so the cursor lives inside `internal/network` rather than as a top-level leaf (Phase 4, #834); it holds no root, marker, schema, authority or lifecycle of its own, and each consumer keeps its distinct magic, field order, validation bounds and refusal prose. Epoch's canonical text grammar (length byte, UTF-8, printing range) stays in `internal/network/epoch` (`text.go`) over this cursor. Real consumers: `internal/network/epoch` (Record, Epoch and materialization grammars), `internal/network/closedprofile` (signed-profile body), `internal/network/state` (distribution journal) and `internal/network/source` (response bundle framing). It grants no installed restart or native qualification. | standard library |
| `internal/network/duty` | `package duty` | Coordinate the bounded, owner-only, durable installation-local participation generations, watermark, expiry, owner-held `direct-source/live` guards, and current conflict truth. Network supplies producer-scoped restriction replacement, conflict, expiry, classification and bounds; this adapter owns the root, canonical journal, commit and recovery lifecycle. A live Direct Source remains effective until its producer replaces or removes it; time-held exposure still expires, and another producer cannot prune the live guard. The root schema is version 2 without the spend field: ADR-0107 retired the historical receiving one-use Transit Grant spend ledger by a bounded in-place conversion, so a strictly validated version-1 generation loads as version 2 with its spend records dropped while conflict duties, generation continuity, and the watermark survive, and every committed generation is version 2. Short Source-write and conflict-read operations acquire the same exclusive lease with a cancellable one-second wait; retained owners keep non-waiting Open. Successful acquisition verifies the current generation, and a busy or invalid root remains unavailable. Independent Linux read-only Source and stopped-Node inspections retain original native identities and existing leases, with distinct exposure-versus-zero-producer policies and joined physical completion; neither initializes, repairs or prunes history. It retains no issuer root, private generation, public profile, budget, request reservation, or signing operation; it owns no profile/request grammar, Route, Target, Service, or participant identity. The durable-file mechanism (bounded read, staged replacement, pruning, watermark codec, root inspection, directory sync, the strict-identity Unix root lease and the Windows owner-only root DACL enforcement, #793) is the shared `internal/durablefile` leaf (R1, #764); duty keeps its own marker, lock, schema, generation immutability pre-check and refusal prose. | `internal/network` through `participation_policy.go` only, `internal/durablefile`, `golang.org/x/sys/windows` only in `root_lease_windows.go` and `root_permissions_windows_test.go`, `internal/pathgrammar`, standard library |
| `internal/network/source` | `package source` | Own the finite Direct-Origin Source plan, credential binding, private TLS transport, network-scoped request digest and bounded response-bundle framing, material selector, ordering, and exposure identity. The pure local serving-declaration byte grammar lives in the `internal/network/source/servingdeclaration` subpackage (AR-N-07a, #691); it returns declaration facts only, opens no runtime and grants no authority. `ObservePublicTrust` owns the offline public X.509 observation for trust preparation. The transitional `ObservePublicContact` installation-input observation is retired (#604 stage 1.6, ADR-0125). It does not verify Epoch authority or admit State. | `internal/network/canonicalcursor` for bounded bundle reads, `internal/network/epoch` for member bounds, standard library |
| `internal/network/source/inspection` | `package inspection` | Retain and reobserve original bounded local inputs and the protected client key through one authenticated Source contact. The Linux Source CLI decodes arguments and renders the unaccepted result. The contact composes the Linux-only State source preparation and stays Linux; every other platform receives the same explicit typed unavailable result (ADR-0129, #856). | `internal/network/source`, `internal/network/source/servingdeclaration`, `internal/network/state`, `internal/platform/artifacts`, `internal/processresource`, standard library. |
| `internal/network/source/serving` | `package serving` | Owns Direct-Origin Source operational input custody and the original admitted process lifetime, through joined State, native observations and retained completion. Serving classifies bounded first-failure and joined cleanup owners. The command selects fresh or retained State and encodes those facts in the CLI report. The operation runs on the Unix family (Linux and macOS) on the cleared Cohort 0/1 seams; every other platform receives the same explicit typed refusal (ADR-0129, #856). | `internal/network/source/servingdeclaration`, `internal/network/state`, `internal/pathgrammar`, `internal/platform/artifacts`, `internal/processresource`, standard library. |
| `internal/network/source/servingdeclaration` | `package servingdeclaration` | Own the pure closed Source serving-declaration byte grammar (`serving_declaration.go`): exact field names, duplicate/null refusal, one complete object, exact 32-byte array shapes and one finite bounded input, mirroring the original node-role declaration codec in `internal/node` (`plan_declaration.go`, #673). It returns declaration facts only and opens no resource: no State acceptance, root creation, signing, runtime or process policy. Extracted from `internal/network/source` as AR-N-07a (#691) with unchanged canonical declaration bytes. Real consumers: `cmd/ardents-next/source_input_linux.go`, `internal/network/source/command/prepare_linux.go` (prepare-state), `internal/network/source/command/inspect_linux.go` (inspect-latest) and State's read-only `source_inspection_linux.go`; the closed-grammar behavior oracle is `serving_declaration_linux_test.go`. | standard library |
| `internal/network/epoch` | `package epoch` | Authenticate bounded Epoch/Record/View grammar, authority signatures, commitments and materializations from an explicit policy. Project decoded input facts and authentication results into Network's Candidate View policy; compare the returned disposition and summaries to signed commitments. Network is the sole production owner of eligibility, collisions, family totals and deterministic Role Domain assignment. Return one owned verified Decision; `Inspect` exposes only a bounded untrusted Header for exact-byte reuse and chain loading, and `InspectMaterializationIndex` reads an untrusted bounded prefix before the full proof check. Called by State. `ardents-next network` and the bounded `ardents-control` provisioning commands consume `PrepareInitialClosedRecord`/`PrepareInitialClosedEpoch` for unsigned fresh closed Node Records and initial Epoch commitments. Preparation shares Network evaluation and grants no signature or State acceptance (ADR-0120). | `internal/network/canonicalcursor` for bounded grammar reads, `internal/network` through `candidate_evaluation.go` and `assignment.go` only, `tests/epochfixture/assignment` and `tests/epochfixture/network` in behavior tests, standard library |
| `internal/network/closedprofile` | `package closedprofile` | Own only the bounded ARDCPR03 signed-profile grammar and canonical unsigned preparation, validated fixed-purpose public signing transcript and verification against an explicit caller-supplied State context. Its public token-key adapter delegates exact RSA-PSS SPKI validation to the Admission issuerprofile leaf. Return owned parsed public facts; State alone joins them to verified Epoch candidates, decides durable acceptance/conflict, and exposes runtime views. Unsigned preparation is an explicit subsequent provisioning contract; private signing remains outside Network. New State consumes verification. | `internal/network/canonicalcursor` for bounded body reads, `internal/admission/issuerprofile` through `token_spki.go` only, standard library |
| `internal/network/state` | `package state` | Own selected closed-profile intake, authenticated history recovery, finite acquisition and serving, ordered durable publication, clock observation and joined stop/close. `CurrentRuntimeWitness` additionally retains a synchronous original-owner refusal projection without granting authority; `CurrentRuntime` returns one opaque checked observation with copied public profile and exact membership; `AcceptClosedProfile` returns only a committed identity receipt. `Current` is non-authorizing intake/acquisition diagnostics with the local Source exclusion identity. Process admission is borrowed through `PermitWork`; no process-resource monitor, Node lifecycle projection, role/Purpose selection, private signing, token debit, reservation or Carrier execution belongs here. Separate Linux NodeInspection authenticates retained signed/time/security and accepted-profile/member history plus zero-producer participation, returning only copied binding facts without runtime rights. Product tests use genuine signed closed-v3 inputs and independent canonical builders. | `internal/network/canonicalcursor`, `internal/canonicaldigest`, `internal/network/closedprofile`, `internal/network/duty`, `internal/network/epoch`, `internal/network/source`, `internal/network/source/servingdeclaration`, `internal/network/state/durable`; `tests/epochfixture/network` in behavior tests; `internal/network` through `acquisition_history.go`, `epoch_history.go`, `membership.go`, `accepted_observation.go`, `runtime_observation.go`, `runtime_witness.go`, `closed_profile_accept.go`, `clock_observation.go`, `closed_profile_role_join_test.go` and `closed_runtime_view_test.go` only; `internal/platform/systemd` only in `source_repair_linux.go`, `source_repair_linux_test.go`, `node_duty_repair_linux.go`, `node_duty_repair_linux_test.go`, `introduction_repair_linux.go`, `forwarding_repair_linux.go`, `join_repair_linux.go` and `descriptor_repair_linux.go`, `internal/platform/cgroup` only in `source_repair_linux.go`, `introduction_repair_linux.go`, `forwarding_repair_linux.go`, `join_repair_linux.go` and `descriptor_repair_linux.go`, `internal/platform/process` only in `source_repair_linux.go`, `internal/reachability` only in `descriptor_repair_linux.go` for the bounded existing-only Store uncertain-stage recovery; `internal/pathgrammar`, standard library; `internal/platform/artifacts`, `internal/platform/process` and `internal/processresource` for the finite adapter in `source_state_files_linux.go`. Its owner test also uses `internal/platform/artifacts`. |
| `internal/network/state/durable` | `package durable` | Own the exclusive State-root lease, bounded opaque immutable Epoch/input generations, distribution journal, closed-profile bytes and physical state, atomic pointer replacement, fsync and recovery staging cleanup. Independent Linux Inspection retains existing read-only native history/lease without mutation or role policy; Source and Node semantic consumers verify its bytes separately. Accept exact framing limits from State; never verify Epoch authority, select Source results, decide profile conflicts, or publish a runtime View. The bounded read, directory sync, generation-name grammar and strict-identity Unix root lease open are the shared `internal/durablefile` leaf (R1, #764); State keeps its own marker, lock, wrapped lease release, pointer durability boundary and refusal prose. | `internal/durablefile`, `golang.org/x/sys/windows` on Windows, `internal/pathgrammar`, standard library |
| `internal/admission` | `package admission` | Own canonical Permission and holder request grammar, token classes and validation of externally supplied authority facts. Child packages own allocation, quota, issuance, stock and spending transitions; no Network authority. | `internal/admission/issuerprofile`, standard library |
| `internal/admission/issuerprofile` | `package issuerprofile` | Own the canonical public issuer profile, complete class/hour inventory and RSA-PSS SPKI grammar; mint opaque purpose-bound signature requests and verified public profiles. PublicPins/Inspect authenticate independently selected public identities without clock acceptance; Verify additionally fixes the exact interval. The genuine public Control consumer reads only signed public bytes. No storage, private key, quota or State authority. | standard library |
| `cmd/ardents-next` | `package main` | Adapt holder DTOs/results through `endpoint/session`, which owns retained Prefix/JOIN/registration/history; dispatch independent receiving composition through `internal/node`; adapt selected Network epoch/profile intake and inspection through `network.go`; project the read-only diagnostic timeline through `diagnostic_timeline.go` (`internal/diagnostics/timeline`) and request local withdrawal through `endpoint_administration_client.go` (`internal/application/administration`); adapt Admission holder, issuer and receiver inputs to the owners of live Network observations with separate roots and no fallback; adapt standalone Admission holder, receiver, unsigned allocation and current-issuer operations, Admission ledger initialization, issuer key initialize/inspect/export and confirmed token initialize-results/issue and Hosting inputs to independent inspection and durable budget initialize/observe, Node identity import, issuer profile initialization/inspection and Admission binding preparation, finite classified JSON results and explicitly local bounded OTLP export. | standard library; `internal/canonicaldigest`, `internal/diagnostics/telemetry`, `internal/diagnostics/timeline`, `internal/application/administration`, `internal/endpoint/funding`, `internal/endpoint/publisher`, `internal/network`, `internal/network/state`, `internal/endpoint`, `internal/network/closedprofile`, `internal/admission`, `internal/admission/issuerprofile`, `internal/hosting`, `internal/admission/operator`, `internal/admission/issuer`, `internal/admission/quota`, `internal/admission/receiving`, `internal/admission/allocation`, `internal/node`, `internal/enrollment`, `internal/release/endpoint`, `internal/execution/runtime`, `internal/publication/durable`, `internal/publication/instance`, `internal/application/document`, `internal/application/connection`, `internal/release/infrastructure`, `internal/endpoint/session`, `internal/platform/launch`, `internal/platform/artifacts`, `internal/node/provisioning`, `internal/release/infrastructure/signing`, `internal/release/infrastructure/launch`, `internal/network/source/command`, `internal/node/command` |
| `internal/network/source/command` | `package command` | Portable Source CLI boundary with Linux implementations and typed platform refusal for serve, inspect-latest, prepare-trust and prepare-state. Retained repair uses the same serving report projection through the release launch owner. | `internal/endpoint` and `internal/pathgrammar` in `prepare_linux.go`; `internal/network/state`, `internal/platform/artifacts`, `internal/network/source/serving`, `internal/network/source/inspection`, `internal/canonicaldigest`, standard library. ; test-only `internal/network/state`, `internal/network/duty`, `internal/network/source`, `internal/network/source/servingdeclaration`, `tests/epochfixture/network` and `tests/e2e/source` for real State, output and original input custody tests ; `tests/networkfixture` in `state_lifetime_linux_test.go` |
| `cmd/ardents-custody` | `package main` | Start the separate custody process with its signal context and original terminal streams. | `internal/custody/command`, standard library |
| `internal/custody/command` | `package command` | Run the separate custody-process adapter. `create-service-authority` generates a Service Authority inside one encrypted active record; `issue-service-credential` requires the exact independently transferred host request digest before Vault/password access, advances only a bounded request, and renders its receipt only after same-directory staged no-replace publication or exact reopen of the public response. `create-admission-authority` creates the separate closed admission root; `issue-admission-permission` independently confirms the holder request digest, writes its 228-byte permission only to an owner-only no-replace destination, and emits only a digest receipt. `verify-record` consumes a password only through an interactive no-echo terminal; `export-recovery-bundle` and `restore-recovery-bundle` expose respectively a distinctly passworded test-restored Bundle and an authority-locked quarantine restore; `purge-record` deletes one exact verified encrypted record only after terminal confirmation while retaining its Authority floor. It never accepts Authority material, permission body, or a password through arguments, environment, configuration, or shared application stdin; it does not activate a recovered Authority. | `internal/custody` only in `command.go`, `terminal_secret.go`, `command_test.go`, `service_authority.go`, `admission_authority.go`, `verify_record.go`, `recovery_bundle.go`, `purge_record.go` and `admission_permission_client_linux_test.go`; `internal/admission` only in `admission_permission_client_linux_test.go`; `internal/publication` only in `command_test.go`; `github.com/aymanbagabas/go-pty` only in `terminal_secret_linux_test.go`; `golang.org/x/term`; `golang.org/x/sys/windows` only in `terminal_line_windows.go`; `golang.org/x/sys/unix` only in `terminal_line_linux.go`, standard library |
| `internal/platform/artifacts` | `package artifacts` | Read bounded standalone files through nonblocking native opens. Retain original Unix-family descriptors and custody of one bounded offline input and output lifetime; the observed-input and bounded-report seams are Unix-generic (Linux and macOS), while the ext4 storage floor, namespace output and protected-input policy stay Linux. Own exact path identity, root-controlled input and secret buffer retirement, directory reobservation, publication, exact private-file retries and checked readback, and the original configuration inputs and executable identity used by Endpoint and infrastructure delivery. Portable typed declaration decoding checks exact field spellings, duplicates and fixed-array lengths. The package also owns operator artifact codecs, canonical Ed25519 PKCS#8 encoding and admission, and bounded native report writes. Consumers choose the private key and signing policy. | `internal/durablefile`, `internal/pathgrammar`, Standard library |
| `internal/node/provisioning` | `package provisioning` | Produce the offline Node and Source genesis inputs, Epoch, token inventory and Closed Profile from independently admitted material. Own the Network artifact command arguments and bounded public receipts. Fixed-role operations select a Source or Node role. Initial Network key preparation produces one State key and the exact declared number of distinct Node keys under exclusive protected output. | `internal/admission/issuance`; `internal/admission/issuerprofile`; `internal/durablefile`; `internal/network`; `internal/network/closedprofile`; `internal/network/epoch`; `internal/network/source`; `internal/platform/artifacts`; test-only: `internal/admission`; `internal/admission/issuer`; `internal/admission/quota`; `internal/admission/receiving`; `internal/admission/stock`; `internal/admission/token`; `internal/custody`; `internal/network/source/servingdeclaration`; `internal/network/state`; stdlib and recorded dependencies. Exact file imports are enforced by the offline operator isolation check.; `internal/node`, `internal/hosting`, `internal/network/state` (detached AuthorityDeclaration shape only), `internal/pathgrammar` and `internal/processresource` in `runtime_plan.go`; `internal/canonicaldigest` for the shared canonical hex-32 shape; test-only `tests/epochfixture/network` for independent signed Network preparation controls |
| `internal/release/infrastructure/signing` | `package signing` | Produce original initial/forward infrastructure Release artifacts and the separate Source Timestamp operation. Each closed role binds the original program, generation, template and protected key set. | `internal/enrollment`; `internal/platform/artifacts`; `internal/platform/systemd`; `internal/release`; `internal/release/infrastructure`; stdlib and recorded dependencies. Exact file imports are enforced by the offline operator isolation check. |
| `tests/e2e/endpoint` | `package endpoint` with external `endpoint_test` cases | Compiled-command evidence for authenticated Endpoint plan construction and one-time history preparation, closed-alpha Enrollment, genuine Endpoint pre-start trust and staged generation, and two distinct admitted systemd managers. Native fixture helpers own signed Release inputs, dedicated UID/GID and immutable resources, staged platform delivery and guest cleanup. The manager-pair test checks positive launch and causal refusal in both managers. The existing participant journey separately checks exchange. | `internal/endpoint/plan`, `internal/endpoint/unit`, `internal/enrollment`, `internal/release`, `internal/release/infrastructure/launch`, `tests/e2e/source`, `tests/epochfixture/network`, standard library, `github.com/sigstore/sigstore/pkg/signature`, `github.com/theupdateframework/go-tuf/v2/metadata` ; `internal/network/state`, `internal/network/source` ; `tests/releasefixture` in `endpoint_platform_native_linux_test.go`, `native_delivery_linux.go`, `launch_releases_linux.go`, `journey_participant_linux.go`, `release_process_test.go` ; `tests/networkfixture` in `endpoint_history_linux_test.go`, `endpoint_platform_native_linux_test.go`, `launch_refusals_native_linux_test.go`, `launch_replacement_native_linux_test.go` ; `internal/hosting` for idle measurement and provider accounting ; `internal/application/administration` for native snapshot-worker discovery and installed document withdrawal ; `internal/application/connection` and `internal/application/document` in `journey_participant_native_linux_test.go` for original participant exchange ; `internal/admission`, `internal/admission/allocation` for the installed permission fixture ; `internal/platform/cgroup` and `internal/platform/systemd` in `replacement_scopes_linux.go` for kernel and activation-socket observations after Stop ; `internal/network/source` in `launch_roots_linux.go` for the signed Source bundle. `internal/reachability` in `service_refresh_linux.go` for authenticated Store proofs. `tests/e2e/node/processfixture` in `service_instance_linux.go` supplies independently signed Instance responses. `internal/publication` in `recovery_custody_instance_linux.go` verifies the issued Credential against the original Instance request. `launch_stand_linux.go` uses the registered State declaration and signed Epoch fixture to provision the shared native Endpoint stand. ; `internal/connection` and `internal/execution/runtime` in the installed Connection authority and Worker observations. ; `internal/network`, `internal/publication/durable` and `internal/publication/instance` in `connection_reopen_linux.go` for joined reopen checks. |
| `tests/e2e/node` | `package node` with external `node_test` cases | Process profile for the compiled Node CLI and shared test-driver input and diagnostic capture. Admission includes a standalone issuer, holder and receiver cycle; its current Windows storage and console prerequisites remain unavailable. Offline Issuance tests cover key inventory, confirmed issuance, profile and binding commands, refusal, reopening after interruption, and finite telemetry. Their independent fixtures use `internal/admission`, `internal/admission/quota`, `internal/admission/issuance`, `internal/admission/issuerprofile`, `internal/admission/issuerprofile/signing`, the CIRCL blind signature client, and the OTLP trace and metric protobuf readers. The Network commands test covers separate signed intake, inspection and Admission roots using `internal/admission/receiving` bindings. The Source refresh scenario verifies two live TLS observations, compiled profile intake and refusal of stale authority after a signed successor. Verifies refusal before missing retained Network history is initialized. On macOS and Windows it checks that the compiled Node command refuses before reading private input, including retained repair dispatch. | `internal/admission/issuance`, `internal/admission/issuerprofile`, `internal/admission/issuerprofile/signing` for offline Issuance behavior and fixtures; standard library; runs the built `ardents` process. The Admission CLI console fixture decodes `internal/admission` permission data and exercises `internal/admission/receiving` bindings. The untagged `route_issuer_recovery_test.go` exercises holder restart and offline issuer retry with signed Network State and the exact Admission/funding fixture imports. The Route Bootstrap, Prefix, Registration, replenishment, Descriptor, paired JOIN, failed Prefix-close, clock loss, invalid HELLO and retained-token replay process scenarios use signed Network State, actual Admission and Hosting owners, real receiver processes and the holder console; their successful execution currently requires the supported host implementations. The Issuer draft authority-loss, JOIN stale-handle, caller-cancelled Registration and exact Descriptor refusal cases also call the genuine `internal/endpoint/session` client directly against real receiving processes to control these lifetimes or inspect the detailed refusal. The Linux Route process tests interrupt and join the original receiver or holder process. Forwarding exchange uses the actual Admission console, with `internal/admission/stock`, `internal/admission/token`, `internal/endpoint/funding`, `internal/endpoint/session`, and `internal/network` for signed, offline fixture input. `hosting_fixture_linux.go` imports `internal/hosting` to initialize an isolated process budget. `provisioning_linux.go` uses `internal/platform/artifacts` and the signed epoch fixture while exercising the original Node key, Record and certificate commands. The native Forwarding, Descriptor and JOIN lifecycle tests use Hosting, Network Duty, Process Resource and the signed epoch fixture to observe the original retained process. The Descriptor case also imports `internal/reachability` to reopen its retained Store after the crash. `issuer_inputs_linux.go` uses `internal/admission/issuer` and `internal/admission/quota` to verify original leases are released. The native Node startup tests check refusal using independently signed Network State. Descriptor, Introduction and JOIN share the same failure matrix while retaining their distinct history, spend and listener assertions. Issuer prepares Admission quota, keys and results. Forwarding checks existing Hosting and State using `internal/hosting` and `internal/network/state`. `tests/networkfixture` admits independently signed fixture bytes through the real State owner. Shared genesis and the two moved native Source scenarios use `internal/admission/issuer`, `internal/endpoint`, `internal/network/epoch`, `internal/network/source`, `internal/network/source/serving`, `internal/network/source/servingdeclaration`, `internal/network/state`, `internal/platform/artifacts`, `internal/processresource`, `tests/e2e/source`, `tests/epochfixture/network`. The signed Release fixtures and retained Source scenario use `github.com/sigstore/sigstore/pkg/signature`, `github.com/theupdateframework/go-tuf/v2/metadata`, `internal/enrollment`, `internal/network/duty`, `internal/network/epoch`, `internal/network/source`, `internal/network/source/servingdeclaration`, `internal/network/state`, `internal/platform/systemd`, `internal/processresource`, `internal/release`, `internal/release/infrastructure`, `tests/e2e/source`, `tests/epochfixture/network`. ; `internal/release/infrastructure/launch` only in `source_installed_native_linux_test.go` for the genuine installed platform declaration Native tests use `tests/e2e/node/processfixture` for original command, signed input and host state. ; `internal/network/closedprofile` ; `tests/releasefixture` in `infrastructure_fixture_linux.go`, `source_installed_native_linux_test.go`, `source_lifecycle_native_linux_test.go`, `source_release_linux_test.go` ; `tests/networkfixture` in `completion_native_linux_test.go`, `descriptor_exchange_native_linux_test.go`, `forwarding_exchange_native_linux_test.go`, `forwarding_startup_native_linux_test.go`, `issuer_exchange_native_linux_test.go`, `issuer_startup_native_linux_test.go`, `join_exchange_native_linux_test.go`, `network_commands_linux_test.go`, `receiving_startup_native_linux_test.go`, `retained_lifecycle_native_linux_test.go`, `route_bootstrap_linux_test.go`, `route_descriptor_linux_test.go`, `route_hello_linux_test.go`, `route_issuer_recovery_test.go`, `route_join_linux_test.go`, `route_quic_linux_test.go`, `route_registration_linux_test.go`, `source_installed_native_linux_test.go`, `source_lifecycle_native_linux_test.go`, `source_native_linux_test.go`, `source_preparation_process_linux_test.go`, `source_refresh_linux_test.go` ; `internal/release/infrastructure/launch` and `internal/platform/systemd` in the Forwarding, JOIN and Descriptor platform lifecycle scenarios. |
| `tests/e2e/node/processfixture` | `package processfixture` | Node process test driver for the package-owned CLI build, pinned process execution, native host state, signed Network and Admission input, and joined cleanup. | `internal/admission`, `internal/admission/allocation`, `internal/admission/issuance`, `internal/admission/issuer`, `internal/admission/quota`, `internal/admission/spending`, `internal/admission/stock`, `internal/admission/token`, `internal/endpoint/funding`, `internal/hosting`, `internal/network`, `internal/network/duty`, `internal/network/epoch`, `internal/network/state`, `tests/epochfixture/network` and standard library. ; `internal/admission/receiving` in `receiving_spend_linux.go` for replaying actual spends after joined reopen. |
| `tests/e2e/source` | `package source` with external `source_test` cases | Compiled Source command checks for authenticated inspection and before-effect host refusal. Native fixtures provide independent public TLS and root-controlled signing workspace inputs and the shared native initial key and certificate signing fixtures. | `internal/network/source`, `internal/network/source/servingdeclaration`, `internal/platform/artifacts`, standard library |
| `tests/e2e/service/fixturecommand/publish-app` | `package main` | E2E-only separately granted publication-operator fixture. It uses only the bounded local Service Administration socket and is built solely by the Endpoint recovery process test. | standard library |
| `tests/e2e/service/fixturecommand/stream-app` | `package main` | E2E-only paired direct opaque-stream workload used by retained qualification evidence. It owns deterministic bounded bytes and receipts but no Endpoint-local transport grammar. | standard library |
| `tests/epochfixture/assignment` | `package assignment` | Independently construct deterministic role-domain assignments for black-box Network State fixtures; never serve maintained product code. | standard library |
| `tests/epochfixture/merkle` | `package merkle` | Independently construct canonical Epoch commitment roots and proofs for black-box Network State fixtures; never serve maintained product code. | standard library |
| `tests/networkfixture` | `package networkfixture` | Prepare independent signed Network inputs, issued test-key inventory, clock observations and reopened State histories for owner and process tests; no runtime authority. | `internal/admission/issuance`, `internal/admission/issuer`, `internal/admission/quota`, `internal/network/epoch`, `internal/network/state`, `tests/epochfixture/network`, standard library ; test-only `internal/admission`, `internal/admission/allocation`, and `internal/admission/token` in `issuer_conflict_test.go` ; `internal/network` in the signed Admission fixture |
| `tests/releasefixture` | `package releasefixture` | Construct independent signed Release authority and inventory bytes for owner and process tests; no runtime authority. | `internal/enrollment` and `internal/release` only in the Linux infrastructure fixture, `github.com/sigstore/sigstore/pkg/signature`, `github.com/theupdateframework/go-tuf/v2/metadata`, standard library |
| `tests/epochfixture/network` | `package network` | Construct canonical signed Node Record, Epoch and materialization bytes for black-box Network State and successor provisioning tests. It supplies deterministic test input bytes only and cannot accept State, choose a runtime Route or serve maintained product code. | `tests/epochfixture/assignment`, `tests/epochfixture/merkle`, standard library |






| `internal/architecture` | `package architecture` | Test repository structure, ownership inventory, package/import boundaries, artifact-lane separation, extraction rehearsals, names, dependencies, formatting, and quality wiring. It owns the bounded AAI2 retirement closure guard whose exact receipt and scope are defined by [Testing](testing.md#reachability-audit). | standard library |
| `internal/diagnostics/process` | `package process` | Owner-private heap dump verification and parsing harness. | Standard library. Heap dump role-map tests consume `internal/admission`. |
| `internal/diagnostics/telemetry` | `package telemetry` | Own explicit loopback Collector admission and the bounded OTLP trace and metric exporter lifecycle for one command invocation. Admission, Hosting, Issuance, issuer-profile and issuance-results CLI adapters supply operation labels and project their own outcomes. | Standard library; `go.opentelemetry.io/otel/attribute`, `go.opentelemetry.io/otel/metric`, `go.opentelemetry.io/otel/sdk/{resource,trace,metric,metric/exemplar,metric/metricdata}`, and the two selected OTLP HTTP exporters. |
| `internal/diagnostics/timeline` | `package timeline` | Project bounded local Node, Source, and Endpoint runtime JSON or journalctl events into one safe, time-ordered operator view without retaining raw input or opening an authority root. The `ardents-next diagnostics timeline` command selects its input and output. | standard library |
| `internal/application/administration` | `package administration` | Own the separately authorized Application Interface v1 Publish/Withdraw contract, lifecycle, closed grammar, failure response, conformance vectors, local server transport, and Application client. Its bounded snapshot operation dispatches only to an implementing publication owner; unsupported owners refuse without bodyless fallback, and one snapshot allocation/transition excludes another while withdrawal remains available. Its server serializes accepted-socket close and ownership retirement, joins handlers before returning shutdown, and retains actual close failures. Its Linux text-client published-Link query projects bounded printable destination text only from an implementing separately authorized owner; it carries no caller-supplied Target, Connection bytes, Credential, Service key, State, Route, peer, or Custody fact. | standard library; `internal/application/connection` only in `byte_stream_linux.go` for framed publication ; `internal/application/document`, `internal/application/connection` |
| `internal/release` | `package release` | New isolated offline Release verifier with separately owned trust history, serialized original evaluations and joined Close. Keeps Root-only versus complete metadata floor publication distinct, rejects lost retained pointers, and returns private immutable authorization. Genuine `cmd/ardents-next/release.go` consumes pinned new Enrollment bytes; `cmd/ardents-next/installation.go` opens complete retained history with `OpenRetained` for fresh candidate evaluation, without creating cold trust; no old runtime, installation, signing or live readiness. Portable metadata/target/policy and floor rules use native lease/publication/flush adapters; other Unix platforms have no runtime qualification from compilation. The content-addressed generation-name grammar is the shared `internal/durablefile` leaf (R1, #764); Release keeps its separate TOCTOU-hardened bounded read, retained-file flush and joined-error native lease custody, which this mechanism does not flatten. | `internal/canonicaldigest`, `internal/durablefile`; `internal/pathgrammar`, Standard library; `github.com/theupdateframework/go-tuf/v2/metadata` only in `metadata_authentication.go`, `target_identity.go`, `safety_policy.go`, `floor_encoding.go`; `github.com/theupdateframework/go-tuf/v2/metadata/trustedmetadata` only in `metadata_authentication.go`; `golang.org/x/sys/windows` only in `history_native_windows.go`; behavior signing fixtures only: `metadata` and `github.com/sigstore/sigstore/pkg/signature` in `rotation_test.go`, `metadata` in `generation_authorization_test.go` |
| `internal/release/endpoint` | `package endpoint` | Projects immutable Enrollment bytes as exact Release inputs, authenticates the coherent program and protected-generation pair, and checks the retained metadata needed for initial staging before effects. Real consumers are `ardents release verify-initial` and Endpoint launch. | `internal/enrollment`, `internal/release`, standard library; `tests/releasefixture` only in `authorization_integration_test.go` and `execution_artifact_test.go` for independently signed inventory fixtures |
| `internal/endpoint/launch` | `package launch` | Composes Endpoint initial trust, initial and stopped-predecessor staging, platform unit rendering and privileged pre-start checks. Retains the original installed observations separately at startup. The platform manager owns process start and stop. Real consumers are Endpoint participant and the existing `ardents platform` entrypoints. | `internal/canonicaldigest`, `internal/endpoint/generation`, `internal/endpoint/plan`, `internal/endpoint/unit`, `internal/enrollment`, `internal/platform/artifacts` only in `platform_delivery_linux.go` and `platform_staging_linux.go`, `internal/platform/cgroup`, `internal/platform/process`, `internal/platform/systemd`, `internal/release`, `internal/release/endpoint`, `internal/durablefile`, standard library |
| `internal/endpoint/plan` | `package plan` | Validates bounded Endpoint declarations, platform launch identity, writable paths and read-only Source credentials on every host. Linux request-file custody retains byte, inode and access provenance. Real consumers are Endpoint launch and the platform CLI. | `internal/canonicaldigest`, `internal/durablefile`, `internal/pathgrammar` only in `platform_launch.go`, standard library |
| `internal/endpoint/unit` | `package unit` | Owns pure Endpoint unit rendering and typed systemd policy. The renderer and the loaded-unit verifier use the same privileged pre-start command arguments. Endpoint launch and Worker activation are real consumers. | `internal/canonicaldigest`, `internal/platform/systemd`, `internal/pathgrammar` only in `platform_rendering.go`, standard library |
| `internal/platform/systemd` | `package systemd` | Owns the signed infrastructure unit template, typed systemd observations, original manager references and bounded transport with joined cancellation. Real consumers include platform launch, Endpoint launch, Worker unit checks and State role repair. | `github.com/godbus/dbus/v5` only in exact transport and manager files registered below, standard library |
| `internal/platform/launch` | `package launch` | Platform service template and native pre-start and delivery command composition; Enrollment and Release retain artifact authority. | `internal/endpoint/launch`, `internal/platform/artifacts`, `internal/platform/systemd`, `internal/release`, `internal/release/infrastructure/launch`, standard library |
| `internal/platform/cgroup` | `package cgroup` | Reads bounded kernel scopes and checks quiescence for Endpoint replacement and fixed-role State repair. | `internal/pathgrammar`, Standard library |
| `internal/application/document/worker` | `package main` | Adapt only the fixed installed reader and Publisher worker entrypoints. The artifact remains `ardents-text`. The trusted document commands in `cmd/ardents-next` own only local terminal/pipe presentation and the bounded Application and Administration clients. | `internal/application/document`, standard library |
| `tests/qualification/worker-lifecycle/stream-worker` | `package main` | Implement only the fixed reader or Publisher entrypoint of the separately pinned NET-14 stream-test Application. It accepts one fixed local initialization record, returns its matching readiness acknowledgement, and transfers only the fixed worker-frame bridge; no argument supplies a profile, target, address, executable, authority, Principal or Grant. | `internal/application/streamqualification`, standard library |
| `internal/application/connection` | `package connection` | Own AAI3 document requests and AAI4 byte-stream requests, refused reserved Name destinations, bounded ordered streams, directional EOF, terminal outcomes and joined cancellation. The server, request decoder and typed refusal constructor compile with the protected Linux Endpoint; the client, request encoder, shared data framing and outcome validation remain portable. This grammar does not attest a worker launch or create local authority. | standard library  |
| `internal/application/streamqualification` | `package streamqualification` | Own the fixed `ARDTQP01` local initialization/readiness and bounded worker-frame bridge, authenticated Service-stream transfer and per-stream workload verdict for the separately pinned NET-14 stream-test worker. Its two retained 64/16 client and 256/64 Publisher workload shapes send only deterministic predeclared bytes after Endpoint OPEN/CREDIT and validate received bytes before replenishing exact credit. It defines no target, Route, address, executable, authority, Principal, Grant, or arbitrary caller-selected schedule. The initialization/readiness codec, worker-frame grammar, worker workload and Endpoint-side connection bridge are portable and compile and behavior-test in the portable deterministic inventory; the Endpoint-side `InitializeWorker`/`RunConnections` have only the Linux qualification composition (`internal/execution/runtime`) as production consumer, and installed stream worker activation remains Linux-only. | `internal/application/connection`, standard library |
| `internal/application/document` | `package document` | Own the trusted Unix-family no-follow import of an explicitly selected regular file, the portable immutable bounded UTF-8 snapshot and its fixed document responder, the shared document reader, one bounded trusted local Target Link exchange and independently escaped plain-text presentation, nonce-bound reader/Publisher worker exchange, finite stream credit and round-robin output. Presentation validates the whole body before writing and escapes C0/C1 and Unicode bidi controls, preserving LF and tab. Its reader bridge transfers exactly one admitted Service stream, validates the complete fixed request before forwarding, waits for authenticated terminal completion, validates bounded worker RESULT, and joins owned I/O before returning bytes. Its Publisher bridge receives at most 256 admitted Service streams, schedules up to 64 worker exchanges, and gives each active response a 64 KiB byte queue plus one at-most-16 KiB write. Credits follow consumed or discarded bytes; malformed requests and Service loss remain stream-scoped, while worker protocol violations close the job. Endpoint still owns job/Grant checks and cgroup cleanup. The fixed Linux entrypoint checks inherited local descriptors before its readiness exchange; it identity-verifies and closes any manager-passed resource-control descriptor of its own unified cgroup (systemd 255 `cpu.max` passing for resource-limited units) and refuses every other surviving descriptor, including own-cgroup migration/lifecycle surfaces; it cannot attest the installed unit or issue a Grant. The document grammar, snapshot exchange, worker INIT/READY and frame codecs, reader/Publisher bridges and worker multiplexer are portable and compile and behavior-test in the portable deterministic inventory; the no-follow file import (`snapshot_file_unix.go`) is Unix-family (Linux and macOS) through the shared `internal/durablefile` inspection port (#850) with an explicit typed unsupported-platform refusal on every other platform, and the installed inherited-descriptor entrypoint (`worker_process_linux.go`) remains Linux-only with its explicit typed unsupported-platform refusal, and the portable codecs' production consumers remain the selected Linux installed-worker, Execution and Endpoint compositions. | `internal/application/connection`, `internal/durablefile`, standard library |
| `internal/publication` | `package publication` | Own canonical public Service v3 request/response grammar and unsigned fixed-purpose transcript, verify exact public Credential/Publication v3 signatures, Target/Network/validity and separate read versus publish delegation. Retain copied public proof bytes; no Authority issuance, private Instance signer/root, readiness or Route/Store ownership. | standard library |
| `internal/publication/durable` | `package durable` | Independently owned exclusive v3 Publication root, synced ancestor/marker/layout preparation, exact consumed-generation reservation, immutable public record/current persistence and bounded canonical floor/public-record reconciliation. Record publication joins staging/generation/pointer barriers; exact retry preserves bytes/inodes, and original generation Close retires pointer before record without lowering the floor. Restored public evidence grants no private Instance or accepting Publisher. Real consumer: `cmd/ardents-next/publication.go` through `publication prepare-root`. The durable owner joins original root closure before returning the floor. Portable tests isolate required flush failures; Linux tests use actual lease and directory barriers. | `internal/publication`; `golang.org/x/sys/windows` only through `lease_windows.go`; standard library |
| `internal/publication/instance` | `package instance` | Host-generated non-exporting Instance key, stable public approval request and verified external response through pure Publication and exclusive durable pending/accepted/rejected/conflicting lifecycle. Exact accepted retries preserve the response; terminal or ambiguous transitions deny further authority receipts and clear private material. Unix-family private custody (Linux and macOS) retains original root/lease identity; other platforms refuse before key/filesystem creation. Public grammars and sealed Credential verification remain portable. Real consumer: `cmd/ardents-next/publication.go` for public initialization, request and response operations. The Instance owner joins original root closure before returning each receipt. Original private binding now reconciles a concrete `publication/durable` owner, advances its exact reserved floor and persists consumed redaction before retaining volatile purpose-specific signing. Publication-record signing requires the original genuine Introduction registration receipt and persists the exact consumed record/current through its retained durable reservation. Canonical Descriptor signing consumes Reachability transcript construction and an independent volatile X25519 recipient with strictly increasing real registration revision and capacity two; public preparation commands do not establish that live signing consumer or readiness. Purpose-specific capsule opening retains the original recipient through an opaque user handle, checks exact registration/publication/profile binding and joins all users before erasure/capacity return. Candidate plaintext grants no downstream or replay acceptance. No generic signer or Authority custody. ServiceAttachment confines TLS1.3 CertificateVerify and one-use verified initial challenge proof to the original private opening and admitted JOIN; actual coalesced response write plus final live checks transfer setup into unchanged Work Safety, while the separate immutable ServiceLease retains original Instance authority across physical Attachment gaps and earlier cleanup failures. Logical retirement joins current TLS users before returning that borrow; Publication joins logical leases before key erasure. | `internal/canonicaldigest`, `internal/publication`, `internal/publication/durable`, `internal/reachability`, `internal/route/introduction`, `internal/route/capsule`, `internal/connection/wire`, `internal/route/join`, `internal/execution/runtime`; standard library |
| `internal/publication/runtime` | `package runtime` | One original qualified snapshot Publisher, exclusive Instance lifetime borrow, genuine Introduction registration and actual Source Descriptor Store exchange. Own pending/current/bounded predecessor pairs, exact retry, strictly increasing revision, creation+300s real-clock refresh, joined withdrawal and terminal result. Canonical Link is visible only after actual synced Store ACK and final original Execution/Instance/registration/Descriptor checks. Real non-test consumer: `internal/endpoint/publisher/opening_linux.go` through the qualified holder console; capsule opening, replay and native acceptance remain separate obligations. | `internal/connection`, `internal/execution/runtime`, `internal/network`, `internal/publication`, `internal/publication/instance`, `internal/reachability`, `internal/route`, `internal/route/introduction`, `internal/route/prefix`, `internal/route/capsule`, `internal/route/join`; `internal/connection/wire` only in `publisher_streams.go` for copied full-tuple dispatch commitments, with exact-file isolation enforced separately; standard library |
| `internal/connection` | `package connection` | Immutable initial recipient logical/Attachment binding under the exact qualified Execution operation, sealed independently verified Publication and original local Work Safety bounds. Refuse substitution, extension and recovery without retained continuity. Real non-test consumer: `publication/runtime` capsule opening. The one-use initial authentication consumer composes logical send/receive and Terminal state with a checked original Execution byte contract and joined TLS users; Publication NextAuthentication consumes a genuine initial delivery and the same Source-bound Responder JOIN. Reader's initial Source factory retains the same genuine Source across independent Target-Link/Descriptor verification, Introduction submission, JOIN and Instance authentication; original history and Rendezvous selection remain caller-owned. Recovery and real two-role command/worker acceptance remain required. Copied contexts grant no Instance TLS, accepted replay, Responder, ordered stream or Application bytes; full two-role command/recovery acceptance remains separate. | `internal/execution/runtime`, `internal/publication`, `internal/route/capsule`, `internal/connection/wire`, `internal/publication/instance`, `internal/route/join`; `internal/network`, `internal/reachability`, `internal/route`, `internal/route/introduction`, `internal/route/prefix`, `internal/route/selection` only in `initiator_source.go`; `internal/route/prefix` additionally only in `publisher_recovery.go` for retained original Source/Responder recovery scope; `internal/application/connection` only in `stream.go` and `stream_test.go`; standard library |
| `internal/connection/wire` | `package wire` | Closed canonical Service Connection v3 records, immutable logical/Attachment transcripts, TLS exporter binding and one-use verified initial request receipt. Genuine non-test callers are Connection recipient binding and Instance Service key use. Pure checked values grant no local permission, accepting Publication or Route. No generic signer or owner callback. | Standard library only |
| `internal/reachability` | `package reachability` | Construct the canonical public Descriptor transcript and independently verify its completed signature; verify bounded private Descriptor v3, independently selected lookup Target/current profile and publication delegation before receiving effects; own the separate receiving Store root, durable conflict floors and bounded interrupted-write recovery. The bounded `RepairRetainedStoreStage` clears only an uncertain staging file of an EXISTING retained root under its exclusive lease after validating every committed floor (the Store half of platform-led Descriptor repair, called after independent dead-unit proofs); it initializes no missing root, resets/evicts no floor, promotes no staging, deletes no foreign entry and grants no publication or runtime authority, and `OpenStore`/`OpenRetainedStore` behavior is unchanged. Store retains original native root/records/marker/lock/committed-record custody and sync descriptor, permanent operational denial, separately serialized publications and joined original borrows before first physical release. Receiver consumes Check without foreign observer recursion; request refusal cannot retire a healthy owner. Local History retains only context-private monotonic facts and the exact joined lookup flight; no offline proof cache. Pure verification, history and floor transitions are portable; native Linux/Windows lease and directory synchronization remain mechanisms. No Route selection/transport, private Instance authority or readiness. | `internal/publication`, `golang.org/x/sys/windows` through `store_platform_windows.go` only, `internal/pathgrammar`, standard library |
| `internal/hosting` | `package hosting` | Shared durable provider-period accounting, work/cleanup deadlines, opaque reservations and bounded confirmed persisted snapshots. Reservation accounting (`reservation.go`) and the bounded read (`counter_file.go`) are portable pure Go (P1-B batch 1, #781); the durable flock lease and `/proc`/`/sys` counter measurement stay Linux (`platform_linux.go`) with unchanged `!linux` typed refusals (`platform_other.go`), and their behavior tests remain Linux. Separate Linux read-only existing-history inspection retains original policy/principal/lease and requires returned reservations, without charging or refunding. No placement or network authority. | `internal/canonicaldigest`, `internal/pathgrammar`, standard library ; `internal/platform/artifacts` |
| `internal/admission/issuance` | `package issuance` | Own canonical batch-facts declarations and profile-to-ledger binding preparation, immutable offline RSA material, unsigned inventory and its offline export, immutable signed issuer profiles and durable confirmed blind-token results; exclusive Unix leases where supported, no live authority. | `internal/admission`, `internal/admission/issuerprofile`, `internal/admission/issuerprofile/signing`, `github.com/cloudflare/circl/blindsign/blindrsa`, `internal/admission/quota`, `internal/pathgrammar`, `internal/canonicaldigest`, standard library; `internal/platform/artifacts` only in `result_declaration.go` for portable unique JSON grammar and `artifact_export_unix.go` for Linux and macOS private output publication |
| `internal/admission/operator` | `package operator` | Adapt offline quota initialization, issuance inventory/results and Issuer profile operations to bounded command input and receipts. Called by the Admission, Issuance and Node identity CLI dispatch. | `internal/admission`, `internal/admission/issuance`, `internal/admission/issuer`, `internal/admission/quota`, `internal/diagnostics/telemetry`, `internal/platform/artifacts`, standard library |
| `internal/admission/issuer` | `package issuer` | Compose offline and current-authority open/debit/sign/retain/close and pinned issuer profile preparation/inspection; retain the primary result and every acquired owner cleanup before finite command projection. No quota arithmetic or private signing implementation. | `internal/admission`, `internal/admission/issuance`, `internal/admission/issuerprofile/signing`, `internal/admission/quota`, `internal/pathgrammar`, standard library ; `internal/network`, `internal/canonicaldigest`, `internal/platform/artifacts` |
| `internal/admission/issuerprofile/signing` | `package signing` | Import independently pinned Ed25519 Node key into exclusive immutable Linux storage; sign only opaque validated issuer-profile requests. No private-key or general signing API. | `internal/admission/issuerprofile`, `internal/pathgrammar`, standard library |
| `internal/custody` | `package custody` | Encrypted purpose-specific Vault/Recovery Bundle, original serialized operation/cancel/join/erasure, Admission signing through genuine Allocation and durable successor/readback/floor. Service Authority creation and independently approved fixed-purpose Credential issuance use the pure public v3 contract; predecessor Service/retired Name records remain encrypted evidence; restored quarantine has no signing/activation API. No predecessor backend or live-root adoption. Credential and Permission delivery share flushed no-replace publication with exact retry and separate private Permission access checks. Real consumer is `cmd/ardents-custody`. | `internal/admission`, `internal/admission/allocation` and pure `internal/publication`, standard library; `golang.org/x/crypto/argon2` only in `canonical_envelope.go`; `golang.org/x/sys/windows` only in `vault_lock_windows.go`, `vault_lock_windows_test.go` and `durability_windows.go` |
| `internal/admission/allocation` | `package allocation` | Permission allocation, role maxima and retained allocation decisions. | `internal/admission`, standard library |
| `internal/admission/attempts` | `package attempts` | Durable holder presentation history. | `internal/pathgrammar`, standard library |
| `internal/admission/quota` | `package quota` | Exclusive issuer quota journal, atomic debit, exact retry and opaque debit confirmation. | `internal/admission`, `internal/admission/issuerprofile`, `internal/pathgrammar`, standard library |
| `internal/admission/receiving` | `package receiving` | Receiver binding, redemption and finite token allowance; separate original-operation serialization and synchronous Close denial, with foreign observation/reservation and refused cleanup outside the state mutex and spend-lease return after user join. Accepted work owns transferred reservations. | `internal/admission`, `internal/admission/token`, `internal/admission/spending`, standard library ; `internal/network`, `internal/platform/artifacts` ; `tests/networkfixture`, `tests/epochfixture/network`, `internal/network/state` in `registration_initialization_integration_test.go` for signed-State observation ; `internal/admission/allocation`, `internal/admission/issuer`, `internal/admission/quota`, `internal/hosting` in the signed Network reservation integration test |
| `internal/admission/spending` | `package spending` | Receiver token spend history and distinct Linux existing-only read-only inspection with original numeric principal/lease and joined physical completion; no Introduction slot ownership or runtime authority from inspection. | `golang.org/x/sys/windows`, `internal/pathgrammar`, standard library |
| `internal/route/introduction` | `package introduction` | Own holder Registration request/reader/owning withdrawal and joined result over opaque Prefix channels, separately receiving Registry/live slots, canonical requests/results and durable non-reclaim History. No shared principal, root or lifetime, listener, token spend, Hosting budget or Publication readiness. The durable History root is the Unix-family (Linux and macOS) slot-history port (Cohort 1 N2, #853): every mutation resolves through the held directory descriptor via the shared `internal/durablefile` Openat/Renameat and full-identity inode comparison, and a kernel root-move watch (Linux inotify, macOS kqueue EVFILT_VNODE) retains original move/loss/restoration evidence; every other platform receives the same explicit typed refusal, never a weaker accepting path. | `internal/admission/receiving` only through `receiving_history_unix.go` for the genuine one-use root creation fact and `receiving_history_other.go` for the typed platform refusal, `internal/admission/spending`, `internal/durablefile` through the Unix-family slot-history port (`slot_history_unix.go`) and its custody test, `internal/route`, `internal/route/ardp`, `internal/route/role`, `internal/route/prefix`, `internal/network`, `internal/admission`, `internal/route/capsule`, `internal/pathgrammar`, standard library |
| `internal/route/capsule` | `package capsule` | Closed v3 fixed envelope and recipient-only request grammar, exact domain/header transcript and X25519/HKDF-SHA256/AES-128-GCM public-key sealing. Immutable copied envelopes and candidate requests grant no authority, private key, replay acceptance, Connection binding or live Route. Actual consumers: `publication/instance` purpose-specific borrowed opening and native `ardents-next` Source preparation followed by its genuine admitted Submit exchange; independent codec/suite controls and genuine Registration composition exercise the same grammar. No new package-level authority or old runtime connection. | standard library |
| `internal/admission/stock` | `package stock` | Holder permission, pending exact batch, stock and irreversible presentation. Application owns exchange execution and cancellation. | `internal/admission`, `internal/admission/allocation`, `internal/admission/attempts`, `internal/admission/token`, standard library; behavior tests only: `internal/admission/issuer`, `internal/admission/issuance`, `internal/admission/receiving`, `internal/admission/spending`, `internal/admission/quota` |
| `internal/admission/token` | `package token` | Holder blind preparation, finalization, token verification and batch grammar. | `internal/admission`, `internal/admission/issuerprofile`, `github.com/cloudflare/circl/blindsign/blindrsa`, standard library |
| `internal/platform/process` | `package process` | Retains the actual Endpoint invocation facts, independently observes Source principal quiescence, checks the invocation credentials for Source State preparation and validates the fixed service account ID from retained platform input snapshots. Real consumers are Endpoint launch, Source State repair and platform pre-start. | `internal/canonicaldigest`, `internal/pathgrammar`, Standard library |
| `internal/endpoint/generation` | `package generation` | Owns Endpoint generation inventory, exclusive creation/sealing and independent retained snapshots. Real consumer is Endpoint launch. | `internal/canonicaldigest`, `internal/durablefile`, `internal/pathgrammar`, Standard library |

The exact Service Custody process scenario `tests/e2e/service/service_custody_process_linux_test.go` imports only pure `internal/publication` beyond the existing process mechanisms. It drives built `ardents-next` host request/acceptance and real no-echo `ardents-custody` Authority/issuance commands, with no Instance import or fixture signing authority. `internal/custody/command/service_authority.go` now consumes `internal/custody` directly; the former Service backend is not called.

The Direct-Origin Source adapter in `internal/network/source/command/serve_linux.go` calls
`internal/network/source/serving` for `serve` and `serve-retained` and projects
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
`platform/cgroup/join_quiescence_linux.go`
(`CheckJoinRecoveryQuiescent`).

The alpha bundle packager invokes reviewed `cmd/ardents-next` source for
`enrollment artifact-name` and `enrollment check-protected-generation`.
`enrollment.go` imports only `internal/enrollment` and the standard
library. It passes the static root and the two supplied executable paths to
Enrollment. Enrollment loads them as data using its existing protected resource
inventory. The document worker's source resources live in
`packaging/document-worker` under the existing signed resource names.

The standalone `scripts/sign-qualification-release.go` implements only the
ADR-0120 initial Endpoint Release operation. Its exact permitted imports are
`internal/enrollment`, `internal/release`,
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

| Source under `internal/node/provisioning` | Exact domain imports |
|---|---|
| `node_record_unix.go`, `node_provision_linux.go`, `epoch_plan_linux.go`, `node_certificate_unix.go` | `internal/network/epoch` |
| `epoch_signing_linux.go` | `internal/network/epoch`, `internal/network/source` |
| `profile_plan_linux.go` | `internal/admission/issuerprofile`, `internal/network`, `internal/network/closedprofile`, `internal/network/epoch`, `internal/network/source` |
| `profile_signing_linux.go` | `internal/network/closedprofile` |
| `token_material_linux.go` | `internal/admission/issuance` |

### JOIN retained signing/provisioning import inventory

The transitional JOIN first-start/replacement/recovery/transition orchestration
and its canonical Node consumers retired (#604 FJD stage D, ADR-0125); the
platform-led JOIN lifecycle owns installed delivery. The retained offline JOIN
producers keep their own fixed purpose and original input/output custody; the
read-only repair runtime manager keeps its own godbus connection.

- `internal/node/provisioning/node_certificate_unix.go` (shared with Forwarding/Descriptor): `internal/network/epoch`.
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

| `internal/endpoint/funding` | `package funding` | Application coordination of original Source stock preparation, issuance and presentation; shared Entry/Hosting parent lifetime. Its holder Route declaration preserves the existing JSON fields and converts them to the owner's plan without granting permission or changing validation. Reader, Publisher and bootstrap callers remain in `cmd/ardents-next`. Admission retains finite rights/debit and durable presentation; Network retains current authority; Route retains selection and physical join. P1-B batch 2 (#787, #691) de-tagged the whole composition (`source.go`, `preparation.go`, `prefix.go`, `issuance.go`, `parents.go`, `observation.go`, `reservation.go`) to portable pure Go after per-file purity verification: every composed symbol (hosting reservation accounting, stock/token/prefix/selection helpers) became portable in batch 1 or earlier, and no file adds a native mechanism. The genuine consumers remain Linux (session/endpoint/`cmd/ardents-next`), so the newly portable symbols are windows-production-dead under reviewed deadcode allowances; the portable `preparation_test.go` exercises the Stock preparation coordination on the windows/macos lane. | `internal/admission`, `internal/admission/stock`, `internal/admission/token`, `internal/hosting`, `internal/network`, `internal/network/state`, `internal/route`, `internal/route/ardp`, `internal/route/bootstrap`, `internal/route/prefix`, `internal/route/role`, `internal/route/selection`; `internal/pathgrammar`, standard library. ; `internal/admission/issuer`, `internal/admission/receiving`, `internal/admission/quota`, `internal/platform/artifacts` ; `tests/networkfixture` in `intent_network_test.go` ; `tests/epochfixture/network` in `intent_network_test.go` |

| `internal/endpoint/reader` | `package reader` | One original qualified Reader operation for the installed Endpoint, Source/history ownership, sole stream handoff, local document request/result projection, interruption and joined retirement. The installed participant constructs the Reader for the local Connection surface used by `ardents read`. Borrows participant authority/Stock/Execution/Parents; no CLI or publication decisions. P1-B batch 3 (#794, #691) de-tagged the pure local Text result projection (`result.go`: `newResultStream`/`resultStream`/`checkedOutput` over `io.Pipe`, composing only portable `application/connection.Outcome` and `application/document.NewSnapshot`) with its portable behavior test (`result_test.go`). The Reader coordinator and Source opening stay Linux (`reader_linux.go`, `operation_linux.go`, `source_linux.go`) because they compose the Linux `LaunchReader` native worker seam and selection; the de-tagged projection is windows-production-dead under a reviewed deadcode allowance. Cohort 2b (#858, #691) re-verified the boundary at d4fd6855f after Cohort 0/1/2a: a cross-compile de-tag probe still fails darwin because the coordinator composes the Linux-only `LaunchReader`/`LaunchByteStreamReader` native worker seam (`execution/runtime`), `connection.StreamFailureReason` and `application.Refuse`, none of which is Unix-family portable yet, so the coordinator and Source opening stay Linux. | `internal/application/connection`, `internal/application/document`, `internal/admission`, `internal/admission/stock`, `internal/connection`, `internal/endpoint/funding`, `internal/execution/runtime`, `internal/hosting`, `internal/network`, `internal/reachability`, `internal/route/prefix`, `internal/route/selection`; standard library. |

| `internal/endpoint/publisher` | `package publisher` | Qualified snapshot publication application lifecycle: pending/active handoff, Source and role selections, Instance/history custody, stream delivery, withdrawal and joined cleanup. Real callers: installed participant and holder console in `cmd/ardents-next`. Domain Publication retains readiness/refresh/recipient policy. Root isolation uses the shared portable `pathgrammar.DisjointRoots`. The portable document stream contract (`contract.go`: `DocumentContract`) composes `Operation.BindStreamContract` and `document.MaximumBytes`. The Publisher coordinator and live Open stay Linux (`publisher_linux.go`, `opening_linux.go`, `snapshot_linux.go`) because they compose the Linux `LaunchPublisher` native worker seam, selection and Instance custody; `DocumentContract` has reviewed Windows production and native test deadcode allowances. Cohort 2b (#858, #691) re-verified the boundary at d4fd6855f after Cohort 0/1/2a: live Open and the coordinator still compose the Linux-only `LaunchPublisher`/`LaunchByteStreamPublisher` native worker seam, Hosting `/proc` measurement and Instance custody (all Linux-only with typed refusals), so the `!linux` boundary stays; new `!linux` refusal-behavior tests (`opening_other_test.go`, `snapshot_other_test.go`) and an architecture assertion pin that other hosts refuse before any state or authority. | `internal/application/document`, `internal/admission`, `internal/admission/stock`, `internal/endpoint/funding`, `internal/execution/runtime`, `internal/hosting`, `internal/network`, `internal/publication/durable`, `internal/publication/instance`, `internal/publication/runtime`, `internal/route/prefix`, `internal/route/selection`; `internal/pathgrammar`, standard library.; `internal/application/connection` only in `byte_stream_linux.go` |

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

- `internal/node/provisioning/node_certificate_unix.go` (shared with Forwarding/JOIN): `internal/network/epoch`.
- `internal/release/infrastructure/signing/descriptor_release_native_linux_test.go`: `internal/release/infrastructure`, `internal/platform/systemd`, `github.com/theupdateframework/go-tuf/v2/metadata`.
- `internal/release/infrastructure/signing/descriptor_forward_release_native_linux_test.go`: `internal/release/infrastructure`, `internal/platform/systemd`.
- `internal/release/infrastructure/descriptor_admission.go`: `internal/enrollment`, `internal/release`.
- `internal/platform/systemd/descriptor_manager_linux.go`: `github.com/godbus/dbus/v5`.

`internal/release/infrastructure/signing/descriptor_release_keys_linux.go`,
`descriptor_release_plan_linux_test.go` and `descriptor_command_linux.go`
use only the standard library.
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
`duty.OpenNodeInspection` / `reachability.OpenStoreInspection` /
`spending.OpenInspection` / `hosting.OpenInspection` read-only inspection
mechanisms lose their last production consumer and are retained as reviewed
mechanism code with genuine native coverage (see the deadcode allowlist); their
shared custody/floor/reader helpers remain production-live through
SourceInspection, the live Store and the bounded repair.

`route/receiver/outer_authority_linux.go` owns fresh parent HELLO and original
adjacent-peer reobservation for JOIN and Descriptor. Its exact imports are
`internal/network`, `internal/route/ardp` and the standard
library. Its real caller is `receiver_unix.go` at every existing parent frame
boundary. `outer_authority_linux_test.go` additionally imports the existing
`internal/route/role` contract for supplied-fact refusal controls;
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
| `internal/endpoint` | `package endpoint` | Original installed participant lifetime, bound Endpoint declaration projection, bounded preparation inputs and exact permission file exchange. `ardents-next endpoint construct-plan` validates declarations and writes fresh canonical preparation plans from accepted retained State pins, bounded operator declarations and the distributed Source artifact, without runtime authority. `ardents-next endpoint prepare-history` explicitly authenticates initial signed State/profile and prepares vacant participation, Entry/Interior and role Stock; `start-installed` requires retained histories. Owns the offline Source trust preparation and reuses its detached declaration types. IPC and Service users join before shared funding, Stock, State, local Execution and Startup. The installed command adapter owns its bounded progress and exit projection; domain policies stay with their respective owners. P1-B batch 3 (#794, #691) de-tagged the pure network presentation guard (`authority.go`: `presentationGuard`, composing only portable `stock.Presentation`, `network.RuntimeView` and `funding.NetworkFacts`); its genuine consumer `participant_linux.go` stays Linux, so it is windows-production-dead and platform-native test_dead under reviewed deadcode allowances. The plan constructor (`plan_construction_linux.go`) stays Linux because it composes the Linux `declarations_linux.go` projections (`NetworkConfig`/`installedLocalConfig`/`HeadlessPlan.Network`, native `syscall` identity/config). | `internal/application/administration`, `internal/application/connection`, `internal/admission`, `internal/admission/stock`, `internal/endpoint/funding`, `internal/endpoint/reader`, `internal/endpoint/publisher`, `internal/execution`, `internal/execution/runtime`, `internal/hosting`, `internal/endpoint/launch`, `internal/network`, `internal/network/source`, `internal/network/state`; `internal/network/source/servingdeclaration`, `internal/platform/artifacts` and `internal/processresource` only in `source_trust_preparation_linux.go`; `internal/network/duty` and `internal/route/selection` only in `history_preparation_linux.go` and `history_preparation_linux_test.go`; tests only: `internal/admission/attempts` in `history_preparation_linux_test.go`, `tests/epochfixture/network` in `history_preparation_linux_test.go` and `plan_construction_linux_test.go`; `internal/application/connection` and `internal/application/document` tests only in `latency_native_linux_test.go` (the #650 cold/warm latency participant runs as the installed Endpoint MainPID, retains the original dual-role owner composition and measures the ordinary Reader composition directly, opening the public Connection request and reading the validated document response; it owns no Application server); `internal/canonicaldigest` only in `declarations_linux.go`; `internal/pathgrammar`, standard library.; `source_trust_native_linux_test.go` uses `tests/e2e/source` for independent TLS inputs and `internal/platform/artifacts` to verify the protected test parent ; Installed qualification test imports `internal/admission/issuer`, `internal/admission/quota`, `internal/admission/receiving`, `internal/admission/token`, `internal/connection`, `internal/endpoint/session`, `internal/execution/worker`, `internal/network/epoch`, `internal/publication`, `internal/publication/instance`, `internal/publication/runtime`, `internal/reachability`, `internal/route`, `internal/route/ardp`, `internal/route/capsule`, `internal/route/introduction`, `internal/route/prefix`, `internal/route/receiver`, `internal/route/role`, `internal/route/transport`, `tests/e2e/endpoint`, `tests/e2e/node/processfixture`, `tests/networkfixture`. |

| Path | Package | Responsibility and real consumer | Permitted imports |
|---|---|---|---|
| `internal/endpoint/session` | `package session` | Serial original holder scenario: attempt/intent, Prefix/JOIN, registration, Publication, lookup history and joined Stock closure. Separate local, preparation and live-operation constructors; each typed method checks original Execution permission. Real consumer: ardents-next holder console. Native Route construction retains selection and Hosting through physical join; no command DTO, JSON, output or process status. P1-B batch 2 (#787, #691) de-tagged the pure route reservation helpers (`reservation.go`: release/root-independence/deadline-min) to portable with a new portable behavior test; they compose only the batch-1 portable hosting reservation release and `pathgrammar`. The `OpenPrefix`/`OpenBootstrap`/`OpenJoinContext` compositions stay Linux (`prefix_linux.go`, `bootstrap_linux.go`, `join_linux.go`) with their unchanged `!linux` typed refusals (`route_other.go`), because they open the native selection/Hosting durable roots; de-tagging that composition boundary is a later batch. Cohort 2b (#858, #691) re-verified at d4fd6855f after Cohort 0/1/2a: the compositions compile on darwin, but they still open the Linux Hosting `/proc` durable root whose native measurement is not Unix-family portable, so shifting the `!linux` stub would relocate the refusal behind selection I/O while the required protection stays Linux-only; the boundary therefore stays `!linux`, pinned by a new `!linux` refusal-behavior test (`route_other_test.go`) and an architecture assertion. The unblocking prerequisite is porting the Hosting measurement, native worker launch, Instance custody and reachability Store seams. | `internal/admission`, `internal/admission/stock`, `internal/endpoint/funding`, `internal/endpoint/publisher`, `internal/execution/runtime`, `internal/hosting`, `internal/reachability`, `internal/route/ardp`, `internal/route/bootstrap`, `internal/route/capsule`, `internal/route/introduction`, `internal/route/join`, `internal/route/prefix`, `internal/route/role`, `internal/route/selection`; `internal/pathgrammar`, `internal/execution`, `internal/network`, `internal/network/state`, `internal/platform/artifacts`, `internal/route/channel`, standard library ; signed issuance integration test only: `internal/admission/issuer`, `internal/admission/quota`, `internal/admission/token`, `tests/e2e/node/processfixture`, `tests/networkfixture` |

Retained State/duty opens require their original lease and refuse missing,
symlink/reparse-point or hardlinked lease files before initializing any history.
State's portable retained opener changes no authority or native-runtime support.
Admission `attempts.OpenRetained` and `stock.OpenRetained` require existing
presentation history; the Reader and existing participant directories consume
them. Only a genuinely new participant directory can initialize Stock.

`internal/release/endpoint/execution_artifact_test.go` directly verifies real signed
worker/unit inventory through `internal/enrollment`,
`internal/release` and the current `internal/release/endpoint`
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

The installed Reader's `operation_linux.go` and `byte_stream_linux.go` use
`internal/application/connection` for profile selection, directional limits
and the local terminal projection. The byte-stream adapter also uses the fixed
failure categories from `internal/application/document`.

Execution's `byte_stream.go` and `byte_stream_linux.go` use the fixed worker
protocol in `internal/application/document`. Its `stream_contract.go` and exact
limit test consume the participant limit from `internal/application/connection`.

The installed Publisher uses the fixed byte-stream port in
`internal/application/document` from `publisher_linux.go`, `opening_linux.go`
and `byte_stream_linux.go`. Only the last consumes Application Connection
limits and terminal classes from `internal/application/connection`.

The Linux Administration upgrade in `byte_stream_linux.go` reuses the local
framing in `internal/application/connection` after Publisher admission.
