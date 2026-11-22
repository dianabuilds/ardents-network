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

JOIN's fixed initial/forward Release producers are separate command composition.
`release/infrastructure/join_admission.go` uses only the standard library,
Enrollment and Release to retain a distinct exact JOIN candidate pair; its real
consumer is canonical Control's fixed forward-Release predecessor verification.
The former initial admission entry point and detached inventory accessors retired with the transitional JOIN orchestration (#604 FJD stage D, ADR-0125); the platform-led lifecycle owns installed admission. No Source/Forwarding right or Startup is supplied.
`join_release_plan_linux.go`, `join_release_native_linux_test.go` and
`join_forward_release_native_linux_test.go` may import only the standard library
and `internal/release/infrastructure`; the initial native signing
test alone may also use `github.com/theupdateframework/go-tuf/v2/metadata`.
After the #604 FJD stage D signing cutover these bind the static `ardents-next`
program and `infrastructure.PlatformUnitTemplate`. `join_release_linux.go` may
import only the standard library, Infrastructure, `internal/release`,
`github.com/sigstore/sigstore/pkg/signature` and that TUF metadata package.
`join_release_predecessor_linux.go` may additionally use
`internal/enrollment` for its independently frozen public candidate.
`join_release_arguments_linux.go`, `join_release_keys_linux.go` and the shared
`infrastructure_release_keys_linux.go` use only the standard library. The shared
key-birth mechanism creates five distinct fresh files; each command keeps its
own fixed ceremony/receipt. It signs nothing and supplies no trust or runtime
right. JOIN forward signing verifies its own exact public predecessor pair;
Source/Forwarding signing purposes and live histories are never reused.

Forwarding, JOIN and Descriptor create each individual provisioned Node or
State key through the command-local `provisioning_key_linux.go`. The fixed
role commands select their respective Node/State file, output directory and
public receipt. Record, Epoch, profile and Release signing retain their
purpose-specific inputs and checks.

The command-local `release_key_receipt_linux.go` renders the public key
receipt for Source, Forwarding, JOIN and Descriptor. Each command fixes its
operation and completes its own key output before calling the formatter.

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

The separate canonical `ardents-control forwarding prepare-descriptor` and
`prepare-inventory` consumers in `forwarding_artifact_linux.go` use exactly the
standard library, `internal/enrollment` and
`internal/release/infrastructure`. They retain original protected
native input and exclusive durable output through the existing physical command
mechanisms. They open no Source/Node runtime, Release history or installation.
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
`cmd/ardents-next/forwarding_artifact_linux_test.go` consumes exactly those three
owners and `github.com/theupdateframework/go-tuf/v2/metadata`; its public signed
artifact oracle shares no live root or installed runtime. Source's unchanged
exact descriptor vector remains separately checked.
The retained Forwarding release-plan argument producer
(`forwarding_release_arguments_linux.go`) also owns the shared public-option
uniqueness grammar consumed by the JOIN/Descriptor argument parsers; it imports
only the standard library. No Source request, account, Startup or native
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

`cmd/ardents-control/source_release_predecessor_linux.go` additionally imports
exactly `internal/enrollment`, `internal/release/infrastructure`,
`internal/release`, `github.com/theupdateframework/go-tuf/v2/metadata`
and the standard library. It retains the finite public predecessor input and
checks the fixed offline forward-signing purpose; it grants no Installation or
runtime authority. `source_release_linux.go` remains the direct signing caller.
| Directory | Declaration | Responsibility and consumer | Permitted imports |
|---|---|---|---|
| `internal/release/infrastructure` | `package infrastructure` | Prepares and verifies canonical infrastructure artifact descriptors and admits coherent Release pairs from original Enrollment snapshots. Control uses it for offline preparation. Platform pre-start uses runtime authorization. Source distribution inspection owns the independent history path check, Enrollment and joined Release verifier. The `ardents-next infrastructure authenticate-*` adapter supplies parsed inputs and renders the receipt. | `internal/enrollment`, `internal/release`, standard library |

Platform launch composition is command-local; it introduces no Installation
domain or new runtime supervisor. Exact additional import permissions:

Explicit Source repair stays with Network State and participation. Portable
`state/source_repair.go` consumes only `network/duty`, `network/epoch` and
`network/state/durable`; `state/retained_history.go` consumes only `network/epoch`
and `network/state/durable`. Its Linux native adapter and detached policy tests
(`state/source_repair_linux.go`, `state/source_repair_linux_test.go`) additionally
consume exactly `platform/systemd`; only the implementation also consumes
`platform/cgroup` and `platform/process`. These are
independent physical observations, not Installation transaction authority.
The genuine command consumer `cmd/ardents-next/source_repair_linux.go` imports
only `network/state`, `platform/systemd` and the standard library; its
`source_repair_inputs_linux.go` uses only the standard library. The independent
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

Explicit Introduction repair stays with Network State and participation. Portable
`state/introduction_repair.go` consumes only `network/duty`, `network/epoch` and
`network/state/durable`. Its Linux native adapter and detached policy tests
(`state/introduction_repair_linux.go`, `state/introduction_repair_test.go`)
additionally consume exactly `platform/systemd`; only the implementation also
consumes `platform/cgroup`. Unlike Source, Introduction shares the
`ardents-node` account with sibling Node role units, so quiescence is the per-unit
`cgroup/introduction_quiescence_linux.go` observation only — never an account-wide
process scan. The original Node producer identity is random, so
`duty/retained_history.go` `CheckNodeProducer` discovers the sole live `node-duty`
fence and binds it to the current generation's node identity; slot claims and spend
obligations are never repairable. The genuine command consumer
`cmd/ardents-next/introduction_repair_linux.go` imports only `network/state`,
`platform/systemd`, `processresource` and the standard library. The separate
`systemd/introduction_manager_linux.go` owns the fixed Introduction read-only
runtime and root control connections (distinct `introductionUnitReply` /
`introductionControlReply` vocabularies), granting no installed admission or
domain/native join. No old Introduction runtime is used.

Explicit Forwarding, JOIN and Descriptor repair stay with Network State and
participation, mirroring Introduction. Portable `state/forwarding_repair.go`,
`state/join_repair.go` and `state/descriptor_repair.go` consume only
`network/duty`, `network/epoch` and `network/state/durable`; they share the
`nodeDutyMember` binding that ties the discovered sole live `node-duty` fence to
an accepted Candidate of the current generation with the exact authenticated
family digest. Their Linux native adapters
(`state/forwarding_repair_linux.go`, `state/join_repair_linux.go`,
`state/descriptor_repair_linux.go`) additionally consume exactly
`platform/systemd` and `platform/cgroup`; the Descriptor adapter
additionally consumes `reachability` for the bounded existing-only Store
uncertain-stage recovery (`RepairRetainedStoreStage`) under the same dead-unit
proofs and retained leases. The portable behavior tests
(`state/forwarding_repair_test.go`, `state/join_repair_test.go`,
`state/descriptor_repair_test.go`) consume only `network/duty` and
`network/state/durable`. All three roles share the `ardents-node` account with
sibling Node role units, so quiescence is the per-unit
`cgroup/forwarding_quiescence_linux.go` / `cgroup/join_quiescence_linux.go` /
`cgroup/descriptor_quiescence_linux.go` observation only — never an account-wide
process scan. JOIN repair binds the `/var/lib/ardents/roles/join/` root prefix,
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
are never repairable. The genuine command consumers
`cmd/ardents-next/forwarding_repair_linux.go`,
`cmd/ardents-next/join_repair_linux.go` and
`cmd/ardents-next/descriptor_repair_linux.go` import only `network/state`,
`platform/systemd`, `hosting`, `processresource` and the standard library;
the JOIN and Descriptor declarations' process profiles are
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
| File `internal/network/state/introduction_repair_linux.go` | File `internal/platform/systemd`; `internal/platform/cgroup` |
| File `internal/network/state/forwarding_repair_linux.go` | File `internal/platform/systemd`; `internal/platform/cgroup` |
| File `internal/network/state/join_repair_linux.go` | File `internal/platform/systemd`; `internal/platform/cgroup` |
| File `internal/network/state/descriptor_repair_linux.go` | File `internal/platform/systemd`; `internal/platform/cgroup`; `internal/reachability` (bounded existing-only Store stage recovery) |
| File `cmd/ardents-next/source_repair_linux.go` | File `internal/network/state`; `internal/platform/systemd` |
| File `cmd/ardents-next/introduction_repair_linux.go` | File `internal/network/state`; `internal/platform/systemd`; `internal/processresource` |
| File `cmd/ardents-next/forwarding_repair_linux.go` | File `internal/network/state`; `internal/platform/systemd`; `internal/hosting`; `internal/processresource` |
| File `cmd/ardents-next/join_repair_linux.go` | File `internal/network/state`; `internal/platform/systemd`; `internal/hosting`; `internal/processresource` |
| File `cmd/ardents-next/descriptor_repair_linux.go` | File `internal/network/state`; `internal/platform/systemd`; `internal/hosting`; `internal/processresource` |
| File `cmd/ardents-next/platform_linux.go` | File `internal/enrollment`; `internal/release`; `internal/release/infrastructure`; `internal/network/state` for original retained SourceInspection; `internal/network/source/servingdeclaration` for the original Source plan |
| File `cmd/ardents-next/release.go` | Files `internal/enrollment` and `internal/release` for initial and candidate Release inputs used by both software delivery paths |
| File `cmd/ardents-next/platform_endpoint_staging_linux.go` | File `internal/enrollment`; `internal/release/endpoint`; `internal/endpoint/launch`; `internal/release` for first-delivery floor verification and signed successor staging under the retained Release lease |
| File `cmd/ardents-next/platform_release_linux_test.go` | File `internal/enrollment`; `internal/release`; `internal/release/infrastructure`; `internal/network/state`; `internal/network/duty`; `tests/epochfixture/network`; `github.com/theupdateframework/go-tuf/v2/metadata` |
| File `cmd/ardents-next/platform_unit_linux.go` | File `internal/platform/systemd` |
| File `cmd/ardents-next/platform_linux_test.go` | File `internal/platform/systemd` |

The Source CLI and its process fixtures consume `servingdeclaration` directly
in the exact files listed by the import guard. File custody stays with the
native adapters, and composition converts Source's embedded network
declaration only where it builds the existing State configuration.

`cmd/ardents-next/source_state_preparation_linux.go` composes native input
custody with `network/source`, `network/source/servingdeclaration`,
`network/state` and `processresource`. Its behavior test additionally
uses `network/duty` and the signed `tests/epochfixture/network` fixture.
State owns the public pin projection and initial signed history operation.
The Control `inspect-latest` adapter consumes that same State projection
and checks the Source process profile through `processresource`.

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

Canonical Control's separate `join prepare-descriptor` and `join prepare-inventory`
commands are owned by `cmd/ardents-control/join_artifact_linux.go`. Its exact
project imports are `internal/enrollment` and
`internal/release/infrastructure`, plus the standard library.
It uses the command's existing native physical readers/output writer without
borrowing Source or Forwarding permissions. Infrastructure's `join_artifact.go`
owns only the closed JOIN public descriptor/byte comparison; Enrollment owns its
separate exact seven-file inventory. The public operations do not validate native
unit policy, sign, authenticate an initial pin or admit installed startup. Their
independent canonical oracles use the standard library; the checked
`network_signing_native` profile exercises actual root/ext4 original input/output
custody and a pinned compiled Control. Unsupported platforms refuse before input
or output effects. No Route runtime import or R01 exception is added by these
public preparation consumers.

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

Node certificate provisioning is Linux-only offline composition in
`cmd/ardents-next/node_certificate_linux.go`, using the standard library and
exactly `internal/network/epoch`. Its genuine consumer is the closed
`network prepare-node-certificate` operation through the existing original
signing custody/output lifetime. `epoch/initial_record_verification.go`
adds `VerifyInitialClosedRecord`: exact canonical preparation, generation-one
ARNR-v2 signature and detached public facts, without State acceptance. It uses
only the standard library; its behavior tests do too. No package, private-key
Interface, runtime credential mechanism or dependency is added.

Source public preparation is unexported composition in exactly
`cmd/ardents-next/source_preparation_linux.go`, importing only the standard
library, `internal/network/source` and `internal/processresource`
(the existing Source profile constant). Non-Linux refusal uses the standard
library only. `source_preparation_process_linux_test.go` additionally imports
exactly `network/source`, the command's already admitted `network/state` and
independent `tests/epochfixture/network`. Public grammar/fixture/native custody
tests use only standard-library command composition. No new package is created.
Linux-only `network/source/public_trust.go` owns public X.509 observation, sharing runtime
declaration/collision/key-digest mechanisms without constructing a live Plan
for callers. Its platform matches its genuine native preparation consumer;
other platforms retain runtime and explicit command refusal, without test-only
public observation reachability. It adds only standard-library imports. The existing installed
Source composer retains one pure `installedNetworkAuthority` projection used by
both runtime and preparation; private loading remains runtime-only. Native
finite output is the existing `network_output_linux.go` mechanism, with final
inventory following the last original-input I/O; its ordering is unchanged.
`network/source/connection_cancellation.go` is a standard-library physical
mechanism used by both Fetch and serving; it joins the exact original context
callback before their connection/key/user ownership can return. It adds no
domain callback, policy, new clock or accepting transport substitute.

Initial key preparation uses only standard-library imports in the unexported
`cmd/ardents-next/network_keys.go`, `network_keys_linux.go` and
`network_keys_other.go` command responsibilities. Closed declaration parsing and
its grammar tests compile only on Linux with the genuine producer; other
platforms retain explicit refusal without a test-only parser. Linux generation and owned
secret encoding remain separate from public Network artifacts. The cohesive
`network_output_linux.go` physical finite-file mechanism is synchronously borrowed
by public signing and private key creation; its explicit secret lifetime clears
owned readback buffers before return. No private data crosses Network or a
public-artifact Interface, and no package/dependency is added. Native key tests
use standard-library command composition and the existing registered preparation
chain harness; that harness may consume independently generated test keys while
its Admission fixture remains separate. It imports no new domain owner.

The shared signing entrypoint and public completion metadata in
`cmd/ardents-next/network_signing_command.go` use only standard-library imports.
Initial Network signing in exactly Linux-only `cmd/ardents-next/network_signing.go`
additionally imports `internal/network/epoch` and
`internal/network/closedprofile`. The exact Linux-only
`network_profile_signing.go` and `network_profile_signing_test.go` import only
the latter public owner beyond their standard-library/registered fixture imports.
Closedprofile's Linux producer Interface `SigningTranscript` validates public context and canonical body
and shares its one domain separator with Verify; private keys remain command-only.
Composition reprepares and
verifies public Network artifacts; fixed-purpose private signing and original
native custody remain unexported command responsibilities in
`network_signing_linux.go`, with non-Linux refusal in `network_signing_other.go`.
No private key enters Network and no new package or generic signing Interface
is introduced. `network_signing_test.go` uses only the already registered
independent `tests/epochfixture/network` on Linux, where the genuine production
consumer exists; Windows retains explicit no-effect refusal tests. Native
signing tests use command
composition and the standard library. The shared preparation chain test can
exercise genuine Record/Epoch/profile signing; the preparation-only profile
retains explicit test-only signers and conveys no product signing qualification.

Public Network preparation composition in exactly
`cmd/ardents-next/network_preparation.go` imports `network/epoch` and
`network/closedprofile` alongside the command's common dependencies. It calls
their existing canonical unsigned preparation directly. Native input and fresh
output custody remain standard-library mechanisms in
`network_preparation_linux.go`, with explicit unsupported-platform refusal in
`network_preparation_other.go`; no package or private signer is added.
`network_preparation_chain_linux_test.go` may import those same two public
owners and the already registered independent `tests/epochfixture/network`.
It exercises exact prepared artifacts through test-only distinct-purpose
signatures and actual new State commands; it supplies no product signing right.
The portable `network_preparation_test.go` uses the registered independent
Network fixture for canonical Record bytes. Native custody and interruption
tests use only command mechanisms and the standard library.

`cmd/ardents-next/node_failure_linux.go` imports exactly
`internal/node` and the standard library to encode its copied original
first-failure projection inside the established single terminal report. Node's
`failure_projection_linux.go` imports only `processresource` and the standard
library; it records local phase/cause/elapsed facts without foreign I/O, authority
or private identity. Process-resource failure facts retain original numeric
measurements separately from Node's physical cleanup causes.
`processresource/failure_linux.go` exposes the copied native failure projection
only on Linux, where the genuine Node consumer exists. Shared policy/fact
representation remains portable; unsupported-platform commands refuse acquisition.

[ADR-0122](../adr/0122-select-tcp-only-closed-release.md) selects TCP/TLS alone
for the nearest closed release. The QUIC adapter and its import grants retire;
historical signed profile identifiers stay distinguishable but grant no runtime
execution or fallback. Route owns current Carrier admissibility, Network retains
canonical signed facts, and each retained/new runtime enforces its own boundary
without an old/new bridge. Historical both-Carrier descriptions are not an import
grant or current qualification of a removed mechanism.

The installed pair scenario in exactly `cmd/ardents-next/installed_pair_linux_test.go`
consumes `internal/application/administration`, `internal/application/connection`
and `internal/application/textdocument` for actual public client operations.
Its local controller transfers public signed Network inputs and actual permission
requests/responses between separately installed participants. It neither imports
private Instance/Publication runtime nor transfers a live domain owner.

The original JOIN command in exactly `cmd/ardents-next/node_join_linux.go`
additionally imports `internal/node` and `internal/processresource`.
`node_join_native_linux_test.go` additionally imports `processresource` and
`route/transport`; `node_join_startup_native_linux_test.go` additionally imports
`processresource`, `network/duty` and `route/transport`;
`node_join_authority_native_linux_test.go` additionally imports `node`,
`network/duty` and `route/transport`; `node_join_completion_native_linux_test.go`
additionally imports `network/duty` and `route/transport`;
`node_join_operation_native_linux_test.go` additionally imports `node` and
`route/transport`. All these abbreviated paths are under `internal`.
Other imports remain the command row's exact existing owners. These native
controls delay genuine observations; they provide no accepting authority.

The platform-led JOIN retained evidence and repair (#604 FJD stages A+B+C,
ADR-0125) add exactly `cmd/ardents-next/join_repair_linux.go` (imports
`network/state`, `platform/systemd`, `hosting` and `processresource`; the
declaration binds `processresource.JoinProfile`),
`join_lifecycle_native_linux_test.go` (`node_process_native`; imports
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
A+B+C, ADR-0125) add exactly `cmd/ardents-next/descriptor_repair_linux.go`
(imports `network/state`, `platform/systemd`, `hosting` and
`processresource`; the declaration binds `processresource.DescriptorProfile`
and the five ordered writable roots including `descriptor_root`),
`descriptor_lifecycle_native_linux_test.go` (`node_process_native`; imports
`processresource`, `network/duty`, `route/transport` and `reachability` for the
genuine fresh Store-root provisioning and post-crash retained reopen) and
`descriptor_platform_native_linux_test.go` (`descriptor_platform_native`;
imports `release/infrastructure`, `network/duty`, `processresource`,
`route/transport` and `reachability` for the genuine Store floor commit and
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

The original Introduction command in `cmd/ardents-next/node_introduction_linux.go`
additionally imports exactly `internal/node` and
`internal/processresource`. The exact `node_introduction_native_linux_test.go`
additionally imports `internal/processresource`, `internal/network/duty`,
`internal/route/capsule` and `internal/route/transport`.
`node_introduction_authority_native_linux_test.go` additionally imports exactly
`internal/node`, `internal/network/duty` and
`internal/route/transport`; `node_introduction_completion_native_linux_test.go`
additionally imports exactly `internal/network/duty` and
`internal/route/transport`. `node_introduction_operation_native_linux_test.go` additionally imports exactly
`internal/node` and `internal/route/transport`. These genuine native composition tests
supply no accepting State, History, process or recipient substitute.

The original Forwarding consumer in `cmd/ardents-next/node_forwarding_linux.go`
additionally imports exactly `internal/node` and
`internal/processresource`; `node_process.go` imports only the latter.

The original Descriptor consumer in `cmd/ardents-next/node_descriptor_linux.go`
additionally imports exactly `internal/node` and
`internal/processresource`. Node's `descriptor_linux.go` additionally
imports exactly `internal/reachability` for the genuine original Store;
that import grants no other Node source a Reachability or proof-policy boundary.
These exact command files extend the command row, without granting these
dependencies to other composition files.

The exact `node_native_linux_test.go` scenario additionally imports
`internal/processresource`, `internal/network/duty`, `internal/route/transport` and
the existing independently encoded `tests/epochfixture/network` input owner.
It starts genuine canonical commands with actual native placement; these imports
grant no adapter construction or authority shortcut.

The exact `node_refill_native_linux_test.go` scenario additionally imports
`internal/processresource` and `internal/route/transport`
to select the same actual command profile and real client Carrier. Its other
imports remain the command row's genuine Admission, Hosting and signed-input
owners. It grants no accepting substitute for the Node runtime.

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

The existing command row additionally permits exactly
`cmd/ardents-next/connection_reader_linux.go` to import `connection`,
`execution`, `execution/runtime`, `reachability` and
`internal/application/textdocument`; its other new-domain imports remain within
the command row. This real bounded Text consumer retains original qualified
Reader authority and closes its Connection/Source before joined result handoff.
The checked installed component's
`cmd/ardents-next/connection_response_loss_linux_test.go` additionally imports
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
authenticated profile. The existing `route_linux.go` command consumer checks
the receipt before returning registration facts. These are seams in the existing
Introduction/Prefix packages, with unchanged import directions and no Descriptor
ACK, Publication readiness, private recipient or Execution authority.

The new command's portable `execution_holder.go` consumes Execution and its runtime;
`admission_holder.go` consumes only the runtime's sealed preparation/live checks.
The registered installed command test `execution_installed_linux_test.go` may
import `route/transport` for the two exact Carrier identities; concrete adapters
remain solely with the registered Route fixture and production Route owners.
Those exact files extend the command row's imports. The Text Application
grammar is independently owned: only runtime `initialization_linux.go` borrows
`textdocument.InitializeWorker`. No old Endpoint, Broker, Source or Service
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

The exact `publication_network_linux_test.go` fixture independently accepts
signed Epoch/profile inputs into a separate genuine receiving State root.
Alongside the command's common dependencies it imports only `network/epoch`
for the accepted profile and `route/transport` for selected Carrier identities.
It constructs no Carrier adapter or Publication runtime and grants no ACK.

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
| `internal/node` | `package node` | One original Forwarding, Issuer, Descriptor, Introduction or JOIN runtime startup, admission/retirement latch, actual new receiving/Hosting/Route composition, original participation producer, purpose-specific runtime TLS key custody, physical join and retained completion. Introduction retains one prepared Route Registry/History through synchronous denial and physical join, without slot/storage policy. Descriptor composes a genuine separately owned Reachability Store, without proof/conflict/storage policy. Owns the original node-role runtime invocation declaration grammar (`plan_declaration.go`): a pure closed byte codec mirroring `network/source/servingdeclaration.ServingDeclaration`, shared by the runtime, the retained repair commands and `node prepare-plan`. This is configuration validation kept with the owner (ADR-0125 decision 6), returning declaration facts only; it is not effectful provisioning and supplies no universal role policy. No State acceptance, token/spend policy, provider-period policy, transport implementation, Source, effectful provisioning/signing, Service or unselected Node role. Real consumers: `cmd/ardents-next/node_forwarding_linux.go`, `cmd/ardents-next/node_issuer_linux.go` and `cmd/ardents-next/node_descriptor_linux.go` and `cmd/ardents-next/node_introduction_linux.go` and `cmd/ardents-next/node_join_linux.go` and `cmd/ardents-next/forwarding_repair_linux.go` and `cmd/ardents-next/join_repair_linux.go` and `cmd/ardents-next/descriptor_repair_linux.go` and `cmd/ardents-next/introduction_repair_linux.go` and `cmd/ardents-next/node_prepare_plan.go`. | `internal/admission`, `internal/admission/receiving`, `internal/hosting`, `internal/network`, `internal/network/duty`, `internal/processresource`, `internal/route/ardp`, `internal/route/receiver`, `internal/route/role`, standard library; `internal/admission/issuer`, `internal/admission/quota` and `internal/route/bootstrap` through `issuer_linux.go` only; `internal/reachability` through `descriptor_linux.go` only; `internal/route/introduction` through `introduction_linux.go` only; `internal/hosting` and standard library through `plan_declaration.go` (the pure declaration grammar) |
| `internal/execution` | `package execution` | Volatile exact Principal/surface capability generation, independent 64 Connection and 6 combined Administration admission limits; exact Session current/last Job and once-only lifetime and operation claims, synchronous revoke and first immutable cleanup result; a distinct supervisor retains physical cleanup capacity after revoked leases and permanently after failed cleanup. Real consumer: installed Execution preparation and live invocation runtime. Local rules grant no installed confinement, Service, Network or token authority. | standard library |
| `internal/execution/worker` | `package worker` | Fixed root-installed worker artifact and manager inventory, selected Ubuntu 22.04/24.04 systemd 249/255 ordinary Execution admission, credentialed attachment rejecting and closing received descriptors, original pinned cgroup observation and finite original-invocation stop/join. Failed Stop still observes original descendants while retaining failure; original helper threads survive through physical child join. Public Activate/ObserveInstance selects only the closed Text inventory; the trusted qualification entries ActivateStream/ObserveInstanceStream select the fixed Stream qualification inventory (identical confinement contract; qualification identity, owner slice and nested qualification cgroup root). No caller-visible inventory selector exists; the architecture gate restricts Stream callers to the trusted qualification launch path, and each inventory refuses the other's units/slices. The unsupported-platform adapter returns only failure and no activation/artifact/cleanup pin. Internal manager/file observations are private, and original cleanup reports its first definite failure to the bound launch owner before further descendant waiting. Real consumer: Execution preparation and live invocation runtime; the installed Stream activation test is the bounded qualification launch path until the accepted runner lands. Mechanisms grant no authority or readiness. The actual loaded socket is checked before activation and again at original invocation/READY observation through the pure fixed-unit contract. Installation admission remains separate. | `internal/platform/systemd` for detached typed values; `internal/endpoint/unit` for the expected fixed socket contract; `internal/application/streamqualification` through `stream_activation_installed_linux_test.go` only; standard library |
| `internal/execution/runtime` | `package runtime` | Common portable composition consumes exact local Session, joins unsupported native refusal without a qualified handle or participant effects, serializes process-wide fixed activation inventory, claim and pin cleanup before Text INIT, independently reobserve original artifact/invocation after READY, create fresh private byte Grant and close/join it before returning separate completed-current preparation provenance. Failed ambiguous cleanup retains the inventory gate. Genuine consumers: `ardents-next execution prepare-permission`, `execution route-holder` and the independently admitted `endpoint start-installed` Source/permission bootstrap, retaining real Stock and signed Network/Route composition. Live Invocation lends one sealed operation, interrupts before borrower join, retains first physical failure synchronously and joins original worker cleanup before exact completed-current handoff. Bounded actual-manager installed Source/permission startup has a genuine command consumer. Qualified Reader and selected-snapshot Publisher retain Text's sole protocol reader and transfer one authenticated Connection stream under the original operation; pending Reader output remains private until Source, operation and original worker join with completed-current checking. A private worker protocol parameterizes the one fixed launch flow (gate/artifact/Job/Grant/lease/joined cleanup) over the closed Text inventory and the trusted Stream qualification inventory; no caller-visible selector exists. Trusted LaunchStreamQualification/RunStreamConnections admit the fixed Stream qualification worker (INIT/READY, installed reobservation) and run the fixed qualification bridge over the retained attachment inside the original invocation through an authority-rechecking borrow, one transfer attempt per invocation, joining the attachment and every supplied authenticated Service stream; no attachment is exported and ordinary Text one-use claims are unchanged. The architecture gate restricts the qualification seams to the trusted qualification composition. The complete installed NET-14 qualification runner remains separate. | `internal/execution`, `internal/execution/worker`; `internal/application/textdocument` through `initialization_linux.go`, `publisher_worker_linux.go`, its Linux behavior test and `reader_worker_linux.go` only; `internal/application/connection` through `publisher_worker_linux.go`, `publisher_stream.go`, `reader_worker_linux.go` and `reader_stream.go` only; `internal/application/streamqualification` through `stream_qualification_linux.go`, `stream_qualification_other.go` and `stream_qualification_installed_linux_test.go` only; standard library |
| `internal/enrollment` | `package enrollment` | Authenticate an independently pinned portable v5 bundle, exact original program, Authority Custody artifact and optional complete protected group; retain private copied byte provenance. Root-contained bounded actual reads and inventory, original cancellation and justified native ownership mechanisms. Read-only `Bundle.MetadataNames` classifies metadata outside trusted-root/static companion inventory using the same grammar identities. Separate `ReadCandidate` retains an opaque untrusted snapshot without initial-pin or running-program provenance; its declarations and byte copies grant no successor authority. Both results use one private bounded inventory and projection implementation; the genuine fresh-pair consumer is `cmd/ardents-next/installation.go`. `ValidateProtectedGeneration` supplies the same canonical static-group grammar to new Installation inspection without pin provenance or filesystem/Release authority. `ValidateInfrastructureInventory` checks the closed seven-file infrastructure byte grammar and returns fresh copied declarations/digests; its transitional JOIN/Descriptor selected-generation validation consumers retired (#604 FJD stage D, ADR-0125), so it is retained as reviewed mechanism code with genuine native coverage (see the deadcode allowlist) and supplies no pin, native custody, Candidate, Bundle or running-program proof. `ExecutableArtifactName` owns the same platform artifact identity used internally and by the two packaging helpers. `packaging/alpha-bundle/{build,test}.sh` reach the new owner through `scripts/enrollment-artifact-name.go` and `scripts/protected-generation-check.go`; the latter consumes the existing static protected-generation validator. No Release inputs/URLs, signing, installation, execution, Network or old runtime authority. Actual consumers: `cmd/ardents-next/enrollment.go`, `cmd/ardents-next/release.go` and `cmd/ardents-next/installation.go`; the latter consumes authenticated facts, metadata projection and frozen bytes. | standard library |
| `internal/route` | `package route` | Own exact role-purpose eligibility, participant conflicts and retained Interior pair rules; grant no Network, token or physical authority. | standard library |
| `internal/route/selection` | `package selection` | Own independent exclusive installation Entry and local Interior roots, canonical committed pairs, context-local Rendezvous choices and bounded alternative, original retention and time floors; consume coherent Network observations and retain exact duties without redraw on failure. Endpoint first preparation uses `InitializeInstallation` for empty Entry/role histories; funding uses `OpenRetainedInstallation` and retained role borrows, which cannot initialize lost history. Portable Entry-pair eligibility/draw/bounds, retained Leg checks and recipient rules compose with the durable selection owner and root lease/permission mechanisms; all are Unix-portable (Flock, O_NOFOLLOW, Stat_t.Uid) with no Linux-only dependency (ADR-0125 portable core). | `internal/route`, `internal/network`, standard library |
| `internal/route/ardp` | `package ardp` | Own canonical generation-three bounded frame/HELLO, OPEN recipient/envelope encoding JOIN and Descriptor lookup/publication operation/result grammar; reject impossible headers before body reads or allocation. Decoded OPEN fields grant no current duty, role or deadline authority; channel and operation state own permitted effects. | standard library |
| `internal/route/transport` | `package transport` | Own portable ordered-stream/exporter and Node/role request/shared-listener contracts, bounded TLS record writes through original physical capacity projections, exact accepted Carrier/ALPN identities, literal endpoint/pre-effect bounds, shared certificate/TLS authentication and direct/Node classification. Peer-handshake and physical-retirement categorization retain original causes and cannot hide an unrelated aggregate leaf. A separate unstarted-write marker retains failed output provenance only; raw/mixed closed errors grant neither that marker nor peer classification. Actual physical adapters consume these same requirements. No concrete adapter, channel framing, Network/Admission or durable-root authority. | standard library |
| `internal/route/transport/tls` | `package tls` | Own actual literal TCP role/Node dial, inner role TLS client/server handshake over selected lanes, shared direct/Node TCP listener, finite handshake capacity, absolute bounds, deadline reset and physical Node socket interruption. Original native socket-write admission beneath TLS distinguishes wholly unstarted Node output from partial/started physical failure; neither refusal nor zero output grants success. Shared transport owns requests/authentication/classification; supplied raw connections remain the caller's ownership on refusal, while failed dials close the adapter's own socket. No path, token, durable-root or QUIC authority. | `internal/route/transport`, standard library |
| `internal/route/channel` | `package channel` | Own portable framing: sole reader, serialized writer, bounded principal memory, lanes, scheduling, credit/debits, parent exchanges and joined physical retirement with retained original failures. Projects only its own immutable frame overhead for nested TLS record sizing; the projection grants no live credit or authority. The selected writer publishes actual physical counts/failure before becoming inactive; nested output separately retains real lower payload progress and actual lower physical failures, so successful lower bytes followed by retirement remain failed upper work without inventing cleanup failure. Consumes selected transport and caller currentness; no concrete adapter, Network/Admission/Hosting decisions or durable root. Real prefix, receiver and JOIN callers consume this mechanism. | `internal/route/ardp`, `internal/route/transport`, standard library |
| `internal/route/bootstrap` | `package bootstrap` | Own one duty's finite issuer-bootstrap live claims, per-physical-adjacency cap, original 10-second bounds, shared queued-work budget and aggregate output credit. Receiving Node children reserve before TLS and transfer capacity return through the actual lane finish. No Network authority, Admission permission/quota/signing, token stock or ordinary channel upgrade. | `internal/route/ardp`, `internal/route/channel`, standard library |
| `internal/route/issuer` | `package issuer` | Own portable single-terminal request/result exchange, nonce/outcome agreement and role TLS termination for actual Prefix and Receiver callers. No listener, selection, Stock completion, token spend/quota/signing, physical queue/reservation or root authority. | `internal/admission`, `internal/route/ardp`, standard library |
| `internal/route/role` | `package role` | Own portable original-duty/profile reobservation, exact HELLO purpose/recipient/bounds, negotiated role exporter binding and holder HELLO/Stock-presentation/ADMIT/ACCEPT ordering with original-caller checks. Admission supplies the actual durable presentation; receiving owns Grant transfer and joined physical retirement. No intake authentication, replacement selection, spending, Hosting reservation or root. | `internal/network`, `internal/admission`, `internal/route`, `internal/route/ardp`, `internal/route/channel`, `internal/route/transport`, standard library |
| `internal/route/join` | `package join` | Own exact Source/Responder acquisitions, holder attempt/Context and Joined stream over opaque Prefix contracts. Prefix owns generations; native selection owns roots. The separately receiving Rendezvous matching, two-RESULT barrier and relay accounting/credit live in the `join/pairing` subpackage, which Receiver consumes directly without the holder's Prefix/selection imports. No token spend, listener or Service authority. | `internal/route/prefix`, `internal/route`, `internal/route/selection`, `internal/route/ardp`, `internal/route/channel`, `internal/route/role`, `internal/route/transport`, `internal/network`, `internal/admission`, standard library |
| `internal/route/join/pairing` | `package pairing` | Own one exact receiving JOIN duty's Rendezvous matching: two complete RESULT writes before data, relay accounting/credit and joined physical retirement of both original sides. Capacity reserves one side's bounded frame memory from the receiving principal's shared budget before irreversible admission spend and releases it exactly once after Serve joins. Receiver supplies admitted channels and retains actual Grant/Hosting return until Serve completes; pairing neither verifies nor releases an Admission Grant. No holder acquisition/generation, durable root, selection, listener, token spend or Service authority. Real consumer: `internal/route/receiver` through `receiver_linux.go` and `admission_retirement_linux.go`. | `internal/route/ardp`, `internal/route/channel`, `internal/route/role`, standard library |
| `internal/route/prefix` | `package prefix` | Own portable exact Entry/Interior physical generations, pending setup and sealed borrows, original Source/Responder pair commits, authenticated terminal openings, separate finite issuer-only bootstrap, exact-generation issuer and Descriptor exchanges, parent refill and joined resource return. Reachability owns proof/history completion after the original physical exchange joins. Actual Registration/JOIN consumers retain opaque claims; no listener, durable root, token spend or Hosting budget. | `internal/network/state` through `prefix.go` only for the opaque original RuntimeWitness refusal check; `internal/route`, `internal/route/selection`, `internal/route/ardp`, `internal/route/issuer`, `internal/route/bootstrap`, `internal/route/channel`, `internal/route/role`, `internal/route/transport`, `internal/route/transport/tls`, `internal/network`, `internal/admission`; `internal/reachability` through `descriptor_exchange.go` only; standard library |
| `internal/route/receiver` | `package receiver` | Own actual listener, exact-purpose dispatch to Introduction Registry, JOIN Pairing, genuine issuer composition and separately owned Descriptor Store, directed Node Carrier borrowing and forwarding children, receiving Admission composition and transferred reservations through joined physical retirement. Recheck the current Introduction assignment before Descriptor effects/output; Store owns proof/conflict/durability decisions. No holder operation/generation, token quota/spend policy or Hosting budget. | `internal/route`, `internal/route/transport`, `internal/route/transport/tls`, `internal/route/channel`, `internal/route/bootstrap`, `internal/route/role`, `internal/route/ardp`, `internal/route/issuer`, `internal/route/introduction`, `internal/route/join/pairing`, `internal/network`, `internal/admission`, `internal/admission/receiving`; `internal/reachability` through `receiver_linux.go` and `descriptor_linux.go` only; standard library |
| `internal/network` | `package network` | Own Candidate View eligibility, rejection precedence, collision exclusion, family totals and deterministic Role Domain assignment; own the accepted current/pending Epoch relation, conflict refusal, exact successor selection and completion-time validity; bind signed role assertions to exact authenticated Node Records and expose copied members with unambiguous identity/key lookup and bounded validity. Evaluate every authenticated observation before choosing a generation. Own separate local-participation restriction decisions, producer-scoped replacement, live Source retention, identity/family collisions and finite record/producer/exposure bounds; own finite Acquisition attempts, recovery, original deadlines/order, exposure history and backoff inside the State commit boundary; own profile history, trusted-time confidence, coherent AcceptedState observations with copied public issuer inventory and exact issuer/duty binding, and retained-duty binding. Proposals do not publish authority; the State application adapter verifies inputs and commits effects. | standard library |
| `internal/network/duty` | `package duty` | Coordinate the bounded, owner-only, durable installation-local participation generations, watermark, expiry, owner-held `direct-source/live` guards, and current conflict truth. Network supplies producer-scoped restriction replacement, conflict, expiry, classification and bounds; this adapter owns the root, canonical journal, commit and recovery lifecycle. A live Direct Source remains effective until its producer replaces or removes it; time-held exposure still expires, and another producer cannot prune the live guard. The root schema is version 2 without the spend field: ADR-0107 retired the historical receiving one-use Transit Grant spend ledger by a bounded in-place conversion, so a strictly validated version-1 generation loads as version 2 with its spend records dropped while conflict duties, generation continuity, and the watermark survive, and every committed generation is version 2. Short Source-write and conflict-read operations acquire the same exclusive lease with a cancellable one-second wait; retained owners keep non-waiting Open. Successful acquisition verifies the current generation, and a busy or invalid root remains unavailable. Independent Linux read-only Source and stopped-Node inspections retain original native identities and existing leases, with distinct exposure-versus-zero-producer policies and joined physical completion; neither initializes, repairs or prunes history. It retains no issuer root, private generation, public profile, budget, request reservation, or signing operation; it owns no profile/request grammar, Route, Target, Service, or participant identity. | `internal/network` through `participation_policy.go` only, `golang.org/x/sys/windows` on Windows, standard library |
| `internal/network/source` | `package source` | Own the finite Direct-Origin Source plan, credential binding, private TLS transport, network-scoped request digest and bounded response-bundle framing, material selector, ordering, and exposure identity. The pure local serving-declaration byte grammar lives in the `internal/network/source/servingdeclaration` subpackage (AR-N-07a, #691); it returns declaration facts only, opens no runtime and grants no authority. `ObservePublicTrust` owns the offline public X.509 observation for trust preparation. The transitional `ObservePublicContact` installation-input observation is retired (#604 stage 1.6, ADR-0125). It does not verify Epoch authority or admit State. | `internal/network/epoch` for member bounds, standard library |
| `internal/network/source/servingdeclaration` | `package servingdeclaration` | Own the pure closed Source serving-declaration byte grammar (`serving_declaration.go`): exact field names, duplicate/null refusal, one complete object, exact 32-byte array shapes and one finite bounded input, mirroring the original node-role declaration codec in `internal/node` (`plan_declaration.go`, #673). It returns declaration facts only and opens no resource: no State acceptance, root creation, signing, runtime or process policy. Extracted from `internal/network/source` as AR-N-07a (#691) with unchanged canonical declaration bytes. Real consumers: `cmd/ardents-next/source_input_linux.go`, `cmd/ardents-control/source_state_linux.go` (prepare-state), `cmd/ardents-control/source_inspection_linux.go` (inspect-latest) and State's read-only `source_inspection_linux.go`; the closed-grammar behavior oracle is `serving_declaration_linux_test.go`. | standard library |
| `internal/network/epoch` | `package epoch` | Authenticate bounded Epoch/Record/View grammar, authority signatures, commitments and materializations from an explicit policy. Project decoded input facts and authentication results into Network's Candidate View policy; compare the returned disposition and summaries to signed commitments. Network is the sole production owner of eligibility, collisions, family totals and deterministic Role Domain assignment. Return one owned verified Decision; `Inspect` exposes only a bounded untrusted Header for exact-byte reuse and chain loading, and `InspectMaterializationIndex` reads an untrusted bounded prefix before the full proof check. Called by State. `ardents-next network` and the bounded `ardents-control` provisioning commands consume `PrepareInitialClosedRecord`/`PrepareInitialClosedEpoch` for unsigned fresh closed Node Records and initial Epoch commitments. Preparation shares Network evaluation and grants no signature or State acceptance (ADR-0120). | `internal/network` through `candidate_evaluation.go` and `assignment.go` only, `tests/epochfixture/assignment` and `tests/epochfixture/network` in behavior tests, standard library |
| `internal/network/closedprofile` | `package closedprofile` | Own only the bounded ARDCPR03 signed-profile grammar and canonical unsigned preparation, validated fixed-purpose public signing transcript and verification against an explicit caller-supplied State context. Its public token-key adapter delegates exact RSA-PSS SPKI validation to the Admission issuerprofile leaf. Return owned parsed public facts; State alone joins them to verified Epoch candidates, decides durable acceptance/conflict, and exposes runtime views. Unsigned preparation is an explicit subsequent provisioning contract; private signing remains outside Network. New State consumes verification. | `internal/admission/issuerprofile` through `token_spki.go` only, standard library |
| `internal/network/state` | `package state` | Own selected closed-profile intake, authenticated history recovery, finite acquisition and serving, ordered durable publication, clock observation and joined stop/close. `CurrentRuntimeWitness` additionally retains a synchronous original-owner refusal projection without granting authority; `CurrentRuntime` returns one opaque checked observation with copied public profile and exact membership; `AcceptClosedProfile` returns only a committed identity receipt. `Current` is non-authorizing intake/acquisition diagnostics with the local Source exclusion identity. Process admission is borrowed through `PermitWork`; no process-resource monitor, Node lifecycle projection, role/Purpose selection, private signing, token debit, reservation or Carrier execution belongs here. Separate Linux NodeInspection authenticates retained signed/time/security and accepted-profile/member history plus zero-producer participation, returning only copied binding facts without runtime rights. Product tests use genuine signed closed-v3 inputs and independent canonical builders. | `internal/network/closedprofile`, `internal/network/duty`, `internal/network/epoch`, `internal/network/source`, `internal/network/source/servingdeclaration`, `internal/network/state/durable`; `tests/epochfixture/network` in behavior tests; `internal/network` through `acquisition_history.go`, `epoch_history.go`, `membership.go`, `accepted_observation.go`, `runtime_observation.go`, `runtime_witness.go`, `closed_profile_accept.go`, `clock_observation.go`, `closed_profile_role_join_test.go` and `closed_runtime_view_test.go` only; `internal/platform/systemd` only in `source_repair_linux.go`, `source_repair_linux_test.go`, `introduction_repair_linux.go`, `introduction_repair_test.go`, `forwarding_repair_linux.go`, `join_repair_linux.go` and `descriptor_repair_linux.go`, `internal/platform/cgroup` only in `source_repair_linux.go`, `introduction_repair_linux.go`, `forwarding_repair_linux.go`, `join_repair_linux.go` and `descriptor_repair_linux.go`, `internal/platform/process` only in `source_repair_linux.go`, `internal/reachability` only in `descriptor_repair_linux.go` for the bounded existing-only Store uncertain-stage recovery; standard library |
| `internal/network/state/durable` | `package durable` | Own the exclusive State-root lease, bounded opaque immutable Epoch/input generations, distribution journal, closed-profile bytes and physical state, atomic pointer replacement, fsync and recovery staging cleanup. Independent Linux Inspection retains existing read-only native history/lease without mutation or role policy; Source and Node semantic consumers verify its bytes separately. Accept exact framing limits from State; never verify Epoch authority, select Source results, decide profile conflicts, or publish a runtime View. | `golang.org/x/sys/windows` on Windows, standard library |
| `internal/admission` | `package admission` | Own canonical Permission and holder request grammar, token classes and validation of externally supplied authority facts. Child packages own allocation, quota, issuance, stock and spending transitions; no Network authority. | `internal/admission/issuerprofile`, standard library |
| `internal/admission/issuerprofile` | `package issuerprofile` | Own the canonical public issuer profile, complete class/hour inventory and RSA-PSS SPKI grammar; mint opaque purpose-bound signature requests and verified public profiles. PublicPins/Inspect authenticate independently selected public identities without clock acceptance; Verify additionally fixes the exact interval. The genuine public Control consumer reads only signed public bytes. No storage, private key, quota or State authority. | standard library |
| `cmd/ardents-next` | `package main` | Adapt holder DTOs/results through `endpoint/session`, which owns retained Prefix/JOIN/registration/history; retain independent receiving composition in `route_linux.go`; adapt selected Network epoch/profile intake and inspection through `network.go`; project the read-only diagnostic timeline through `diagnostic_timeline.go` (`internal/diagnostics/timeline`) and request local withdrawal through `endpoint_administration_client.go` (`internal/application/administration`); compose live new Network observers for Admission holder, issuer and receiver through `network_authority.go` with separate roots and no fallback; adapt standalone Admission holder, receiver, unsigned allocation and current-issuer operations, bounded offline permission, Admission ledger initialize/debit, issuer key initialize/inspect/export and confirmed token initialize-results/issue and Hosting inputs to independent inspection and durable budget initialize/observe/hold, Node identity import, issuer profile initialization/inspection and Admission binding preparation, finite classified JSON results and explicitly local bounded OTLP export. | `internal/diagnostics/timeline` only in `diagnostic_timeline.go`; `internal/application/administration` also in `endpoint_administration_client.go` for local withdrawal; `internal/endpoint/funding`, `internal/endpoint/reader`, `internal/endpoint/publisher`, `internal/route/introduction`, `internal/route/prefix`, `internal/network` and `internal/network/state` are package-wide grants; the `internal/endpoint` root is consumed only by the composition and its tests in `endpoint_construction_linux.go`, `endpoint_construction_linux_test.go`, `endpoint_history_linux.go`, `endpoint_history_linux_test.go`, `installed_endpoint_linux.go`, `installed_source_linux.go`, `installed_source_linux_test.go` and `source_preparation_process_linux_test.go`; `internal/network/epoch` and `internal/network/closedprofile` are file-restricted to `network_preparation.go`, `network_preparation_chain_linux_test.go` and `network_signing.go`; `internal/network/epoch` additionally through `node_certificate_linux.go` and the registered tests `network_admission_fixture_linux_test.go`, `publication_network_linux_test.go`, `source_native_linux_test.go`, `source_lifecycle_native_linux_test.go` and `source_installed_native_linux_test.go`; `internal/network/closedprofile` additionally through `network_profile_signing.go` and `network_profile_signing_test.go`, `internal/admission`, `internal/admission/issuerprofile`, `internal/hosting`, `internal/admission/issuance`, `internal/admission/issuer`, `internal/admission/issuerprofile/signing`, `internal/admission/quota`, `internal/admission/stock`, `internal/admission/receiving`, `internal/admission/allocation`, `internal/route`, `internal/route/selection`, `internal/route/join`, `internal/route/receiver`, `internal/route/channel`, `internal/route/role`, `internal/route/ardp`, standard library; `internal/node` only in `node_forwarding_linux.go`, `node_join_linux.go`, `node_descriptor_linux.go`, `node_introduction_linux.go`, `node_issuer_linux.go`, `forwarding_repair_linux.go`, `join_repair_linux.go`, `descriptor_repair_linux.go`, `introduction_repair_linux.go`, `node_prepare_plan.go`, `node_prepare_plan_test.go`, `node_issuer_authority_native_linux_test.go`, `node_issuer_operation_native_linux_test.go`, `node_descriptor_authority_native_linux_test.go`, `node_descriptor_operation_native_linux_test.go` and `node_authority_native_linux_test.go`; `internal/processresource` only in `node_forwarding_linux.go`, `node_join_linux.go`, `node_descriptor_linux.go`, `node_introduction_linux.go`, `node_issuer_linux.go`, `node_prepare_plan.go`, `node_prepare_plan_test.go`, `node_process.go`, `node_issuer_native_linux_test.go`, `node_descriptor_native_linux_test.go`, `node_descriptor_authority_native_linux_test.go`, `node_issuer_startup_native_linux_test.go`, `node_issuer_completion_native_linux_test.go`, `node_native_linux_test.go`, `node_refill_native_linux_test.go` and `node_completion_native_linux_test.go`; `internal/network/duty` only in `node_native_linux_test.go`, `node_descriptor_native_linux_test.go`, `node_descriptor_authority_native_linux_test.go`, `node_descriptor_completion_native_linux_test.go`, `node_issuer_startup_native_linux_test.go`, `node_issuer_completion_native_linux_test.go`, `node_issuer_authority_native_linux_test.go`, `node_authority_native_linux_test.go`, `node_completion_native_linux_test.go`, `forwarding_lifecycle_native_linux_test.go`, `join_lifecycle_native_linux_test.go`, `descriptor_lifecycle_native_linux_test.go`, `forwarding_platform_native_linux_test.go`, `join_platform_native_linux_test.go` and `descriptor_platform_native_linux_test.go`; `internal/reachability` only in `route_linux.go`, `route_composition_linux_test.go`, `route_descriptor_linux_test.go`, `node_descriptor_native_linux_test.go`, `node_descriptor_authority_native_linux_test.go`, `node_descriptor_operation_native_linux_test.go`, `publication_registration_linux_test.go`, `descriptor_lifecycle_native_linux_test.go` and `descriptor_platform_native_linux_test.go`; `internal/enrollment` only in `enrollment.go`, `release.go` and `release_process_test.go`; `internal/release` only in `release.go` and `release_process_test.go`; `internal/release/endpoint`, `internal/enrollment` and `internal/release` also in `installation_test.go` for portable Release authorization controls; `internal/execution` only in `execution_holder.go`; `internal/execution/runtime` only in `execution_holder.go` and `admission_holder.go`; `internal/network/source` only in `installed_source_linux.go`, `source_native_linux_test.go`, `endpoint_history_linux.go`, `endpoint_history_linux_test.go`, `installed_startup_linux_test.go` and `endpoint_launch_native_linux_test.go`; `internal/network/source/servingdeclaration` only in `source_input_linux.go` for the pure closed serving-declaration byte grammar; `internal/publication/durable` only in `publication.go`; `internal/publication/instance` only in `publication_instance.go`; canonical issuer request and current-work token composition only in `route_issuer_linux.go` and `route_token_linux.go`: `internal/admission/token`, `internal/route/bootstrap`; `route_token_preparation_linux.go` additionally consumes `internal/admission/token` for missing Stock issuance through the acquired Source; `go.opentelemetry.io/otel/attribute`, `go.opentelemetry.io/otel/metric`, `go.opentelemetry.io/otel/sdk/metric`, `go.opentelemetry.io/otel/sdk/metric/exemplar`, `go.opentelemetry.io/otel/sdk/metric/metricdata`, `go.opentelemetry.io/otel/sdk/resource`, `go.opentelemetry.io/otel/sdk/trace`, `go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp`, `go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp`; behavior tests only: `github.com/theupdateframework/go-tuf/v2/metadata` and `github.com/sigstore/sigstore/pkg/signature` only in `release_process_test.go`; `internal/route/issuer` and `internal/route/bootstrap` in `route_bootstrap_linux_test.go`; `internal/route/transport` in exactly registered Route behavior tests (including `route_bootstrap_linux_test.go`, `route_token_linux_test.go`, `route_descriptor_linux_test.go`, `node_issuer_native_linux_test.go`, `node_descriptor_native_linux_test.go`, `node_descriptor_authority_native_linux_test.go`, `node_descriptor_completion_native_linux_test.go`, `node_descriptor_operation_native_linux_test.go`, `node_issuer_startup_native_linux_test.go`, `node_issuer_completion_native_linux_test.go`, `node_issuer_operation_native_linux_test.go`, `node_issuer_authority_native_linux_test.go`, `forwarding_lifecycle_native_linux_test.go`, `join_lifecycle_native_linux_test.go`, `descriptor_lifecycle_native_linux_test.go`, `forwarding_platform_native_linux_test.go`, `join_platform_native_linux_test.go` and `descriptor_platform_native_linux_test.go`); concrete `internal/route/transport/tls` construction through `route_fixture_linux_test.go` only, `internal/network/epoch`, `tests/epochfixture/network`, `internal/admission/spending`, `internal/admission/token`, `github.com/cloudflare/circl/blindsign/blindrsa`, `go.opentelemetry.io/proto/otlp/collector/trace/v1`, `go.opentelemetry.io/proto/otlp/collector/metrics/v1`, `google.golang.org/protobuf/proto`; Initial Service authentication native test only in `connection_installed_linux_test.go`: `internal/connection`, `internal/execution`, `internal/execution/runtime`, `internal/publication/runtime`, `internal/reachability`, `internal/route/transport`; `internal/application/textdocument` only in `execution_publisher_linux.go`; `internal/application/connection` only in `connection_response_loss_linux_test.go` for defined Stream outcome assertions; Live Publication composition is portable in `publication_holder.go` (consumes `internal/endpoint/publisher`); the durable carrier is `publication.go`, the instance carrier is `publication_instance.go`, and `internal/publication/runtime` has no production command carrier (test-only in `connection_installed_linux_test.go` and `connection_response_loss_linux_test.go`); Publication record composition tests only: `internal/publication`, `internal/publication/durable`, `internal/publication/instance` only in `publication_registration_linux_test.go`; `internal/route/capsule` only in `publication_registration_linux_test.go`, `publication_installed_linux_test.go`, `route_submission_linux_test.go` and `route_linux.go`; `installed_reader_linux.go` additionally consumes `internal/application/connection` and `internal/application/textdocument` for fixed local projection, and `internal/connection` plus `internal/execution/runtime` for original worker/Source retirement; `installed_endpoint_linux.go` consumes `internal/application/connection` and `internal/application/administration` only for the admitted local listeners; `installed_publisher_linux.go` consumes `internal/execution/runtime` for the original snapshot worker and retained Publication lifecycle; `installed_service_linux_test.go` consumes the public `internal/application/administration` client and independently checks the public `internal/publication` codec, owns the real receiving `internal/reachability` fixture and names the selected `internal/route/transport` Carrier; it consumes no private Instance or Publication runtime  `internal/release/infrastructure` through exactly `infrastructure_linux.go`, `infrastructure_linux_test.go`, `infrastructure_caller_linux_test.go`, `infrastructure_preflight_native_linux_test.go`, `infrastructure_stopped_native_linux_test.go`, `forwarding_platform_native_linux_test.go`, `join_platform_native_linux_test.go` and `descriptor_platform_native_linux_test.go`; the former new-zone OLD-path installed QA registration (`forwarding_installed_native_linux_test.go`) retired with the transitional Forwarding orchestration (#604 FJD stage D, ADR-0125); `internal/endpoint/session` ; `internal/platform/systemd` only in `platform_unit_linux.go`, `platform_linux_test.go`, `source_repair_linux.go`, `introduction_repair_linux.go`, `forwarding_repair_linux.go`, `join_repair_linux.go` and `descriptor_repair_linux.go`; `internal/processresource` also in `introduction_repair_linux.go`, `forwarding_repair_linux.go`, `join_repair_linux.go`, `descriptor_repair_linux.go`, `forwarding_lifecycle_native_linux_test.go`, `join_lifecycle_native_linux_test.go`, `descriptor_lifecycle_native_linux_test.go`, `forwarding_platform_native_linux_test.go`, `join_platform_native_linux_test.go` and `descriptor_platform_native_linux_test.go`; `platform_linux.go` and `platform_release_linux_test.go` additionally consume exactly Enrollment, Release and infrastructure as registered above; protected text Endpoint platform launch only in `platform_endpoint_linux.go`, `platform_endpoint_linux_test.go`, `endpoint_platform_native_linux_test.go` and `endpoint_launch_native_linux_test.go`: `internal/enrollment` (Headless candidate inventory), `internal/endpoint/plan` (canonical installation declaration projection), `internal/endpoint/unit` (enrolled fixed Endpoint template preparation/rendering and loaded confinement verification), `internal/release/endpoint` (fresh generation Release authorization), `internal/platform/systemd` (loaded manager version and Endpoint unit observation) and `internal/release` (retained trust verifier); no old Endpoint runtime, manager or startup authority. The `endpoint_platform_native` real-host qualification (`endpoint_platform_native_linux_test.go`, root driver) provisions the genuine dedicated account, enrolled protected candidate, retained Release floors, selected generation executable and rendered platform main unit loaded in the real systemd manager, then runs the actual privileged `platform check` consumer (F-25 pre-start trust transfer) with positive acceptance and causal pre-effect refusals; it reuses the same five successor owners and the package-local genuine Release/Network/Source fixtures, opens no Network State and starts no participant. It is component/acceptance evidence with fixture Release authority, not Service readiness or installed two-participant privacy qualification. The `endpoint_launch_native` real-host qualification (`endpoint_launch_native_linux_test.go`, root driver) proves the FULL platform-led launch chain to protected text Service readiness for ONE dual-role Endpoint: host-owned account/Worker delivery and genuine `platform initialize-trust` + `platform stage-endpoint`, `endpoint prepare-history`, `hosting initialize`, the platform main unit + canonical socket-activated worker units, systemd start (no root ACK), F-25 pre-start transfer, `endpoint start-installed` platform-led participant, an independent fixture permission granter and the original-process listeners; it additionally consumes `internal/network/source` (live Source servers), `internal/admission`/`admission/allocation` (permission grant) `tests/epochfixture/network` and the original `internal/platform/systemd` Endpoint unit reference to retain the service exit observation through Stop. It uses `internal/platform/cgroup` to retain original scope descriptors and check the post-Stop kernel inventory, and typed systemd properties to verify stopped activation sockets with no queued jobs. The fixture checks positive Service readiness and causal pre-effect refusals. It is component/acceptance evidence with fixture authority for one dual-role Endpoint, not the two-participant privacy qualification (#391 S2). Supporting production repair: `endpoint/launch` `createEndpointAccount` now allocates a dedicated uid==gid account (matching the infrastructure Source and platform pre-start account contract), and `inspectFixedResources` admits the platform-led main unit (its bytes verified by the F-25 pre-start check and loaded confinement by the running platform observation) while transitional callers keep the exact enrolled-unit comparison. The `endpoint_pair_native` real-host qualification (`endpoint_pair_native_linux_test.go`, root driver) is a standard-library-only environment orchestrator (#391 S2.1): it imports no successor or old runtime owner, provisions two separately admitted Ubuntu/systemd managers as fresh `systemd-nspawn` guests (own PID-1 systemd, separate PID/mount/cgroup namespaces, pivoted patched Ubuntu 24.04 root, shared host network namespace), asserts each is a distinct admitted manager, and runs the checked `endpoint_launch_native` full-launch qualification inside each guest to protected text Service readiness with distinct original InvocationIDs and joined cleanup. It is component/acceptance evidence with fixture authority for two independently admitted managers reaching platform-led Service readiness, not the cross-Endpoint publish/link/read journey (#391 S2.2) or whole-host privacy qualification; `systemd-nspawn` is a qualification-environment tool, not a product runtime dependency. The `endpoint_pair_native` journey harness (`endpoint_journey_native_linux_test.go`, compiled with `text_worker_installed`, root driver) reuses one unchanged platform-led participant for the #391 S2.2 protected-text journey and adds three controllers: single-real-host publication within the finite publication deadlines, the single-host nspawn two-manager journey, and the cross-host two-REAL-manager journey (`TestNativeEndpointCrossHostJourney`) that lifts the fixture route network onto the controller's reachable address before epoch signing, distributes static participant/consumer/worker binaries over SSH, admits both participants by original nonce, and coordinates publish → canonical Link → byte-equal cross-host read → withdrawal → platform-led restart continuity with joined two-host cleanup; it additionally consumes `internal/network/source`, `internal/route/transport` and `internal/publication` (canonical Link codec) plus the public `internal/application/administration`, `internal/application/connection` and `internal/application/textdocument` clients, and no old runtime owner. It is fixture-authority component/acceptance evidence, not whole-host privacy qualification The `endpoint_launch_native` cold/warm latency harness (`endpoint_latency_native_linux_test.go`, compiled with `text_worker_installed`, root driver, #650 slice 3b) keeps the genuine publication Route network and signing authority, provisions one platform-led dual-role installed root through the shared launch stand against that Route fixture, initializes the publication Instance credential, then pins a temporary qualification unit whose non-root MainPID is the endpoint-package participant test binary (latency_native_linux_test.go in internal/endpoint); it approves each exact qualified permission request through the original file handover (the signing key never leaves the driver) and observes the reported nearest-rank cold/warm verdict. It names the selected `internal/route/transport` Carrier exactly as the journey controller does and consumes no old runtime owner. It is bounded component evidence on the fixture stand (a few cold/warm pairs exercising the verdict logic), not the full 1,000+1,000 x 3-seed paced campaign and not whole-host privacy qualification; `internal/execution/worker` only in `connection_accepted_request_installed_linux_test.go` to reobserve the original installed worker identity and cgroup  `internal/endpoint/launch` through `platform_endpoint_staging_linux.go` for native generation staging only |
| `cmd/ardents-custody` | `package main` | Run the separate custody-process adapter. `create-service-authority` generates a Service Authority inside one encrypted active record; `issue-service-credential` requires the exact independently transferred host request digest before Vault/password access, advances only a bounded request, and renders its receipt only after same-directory staged no-replace publication or exact reopen of the public response. `create-admission-authority` creates the separate closed admission root; `issue-admission-permission` independently confirms the holder request digest, writes its 228-byte permission only to an owner-only no-replace destination, and emits only a digest receipt. `inspect-envelope` is a pure public-header reader with no Vault root; `verify-record` consumes a password only through an interactive no-echo terminal; `export-recovery-bundle` and `restore-recovery-bundle` expose respectively a distinctly passworded test-restored Bundle and an authority-locked quarantine restore; `purge-record` deletes one exact verified encrypted record only after terminal confirmation while retaining its Authority floor. It never accepts Authority material, permission body, or a password through arguments, environment, configuration, or shared application stdin; it does not activate a recovered Authority. | `internal/custody` only in `command.go`, `terminal_secret.go`, `main_test.go`, `service_authority.go`, `admission_authority.go`, `verify_record.go`, `recovery_bundle.go`, `purge_record.go`, `inspect_envelope.go` and `admission_permission_client_linux_test.go`; `internal/admission` only in `admission_permission_client_linux_test.go`; `internal/publication` only in `main_test.go`; `golang.org/x/term`; `golang.org/x/sys/windows` only in `terminal_line_windows.go`; `golang.org/x/sys/unix` only in `terminal_line_linux.go`, standard library |
| `cmd/ardents-control` | `package main` | Run the bounded closed-profile operator adapter and the offline Source/Forwarding/JOIN/Descriptor preparation compositions. The enrollment-pinned ACA1 reader retired with ADR-0127 (#647): `inspect-bundle` and `inspect-transitions` refuse before parsing arguments or opening any file, root, or floor, no unified replacement report is selected, and successor per-domain commands verify the same authority facts. `prepare-qualification-evidence` and `prepare-qualification-catalog` refuse with the retired ACA1 preparation; `prepare-qualification-node-record` and `prepare-qualification-epoch` refuse with pointers to the successor `ardents-next network` preparation commands that cover the same initial Network need. The exact retired `accept-alpha-corpus` and `inspect-alpha-corpus` routes refuse before parsing arguments or opening any file, root, or floor; the command retains no ACA2 verifier and no corpus-floor mutation authority (ADR-0110). `prepare-closed-profile`, `sign-closed-profile`, and `inspect-closed-profile` respectively prepare, State-authority-sign, and verify only canonical `ARDCPR03`; the signer is an owner-only file and no generic signing route exists. `inspect-closed-issuer-profile` verifies the raw public Node-signed issuer profile or existing JSON export against independently supplied Network, Node and key pins, then renders its public inventory for offline State preparation; it grants no State acceptance. Caller-keyed low-level ACA1 inspection and future-public-claim projection are not command routes. | `internal/admission/issuerprofile` only in `closed_issuer_inspection.go`; standard library; `internal/network/source`, `internal/network/state` and `internal/processresource` only in `source_state_linux.go`, with `source` and `processresource` additionally in `source_state_linux_test.go`; `internal/network/source` additionally in exactly `source_inspection_linux.go` and `source_inspection_linux_test.go`; `internal/network/source/servingdeclaration` only in `source_state_linux.go`, `source_state_linux_test.go`, `source_inspection_linux.go`, `source_inspection_linux_test.go`, `source_state_preparation_linux_test.go` and `source_initial_network_native_linux_test.go` for the pure closed serving-declaration byte grammar; `internal/enrollment` and `internal/release/infrastructure` only in `source_artifact_linux.go`; `internal/release/infrastructure` additionally in `source_release_plan_linux.go`; `internal/release/infrastructure`, `internal/release`, `github.com/sigstore/sigstore/pkg/signature` and `github.com/theupdateframework/go-tuf/v2/metadata` only in `source_release_linux.go`; `internal/release/infrastructure` and `github.com/theupdateframework/go-tuf/v2/metadata` additionally in `source_release_native_linux_test.go` and `source_release_timestamp_native_linux_test.go`, with `internal/release` additionally in the latter for actual same-history timestamp-expiry verification; `internal/enrollment`, `internal/release/infrastructure`, `internal/release`, `github.com/sigstore/sigstore/pkg/signature` and `github.com/theupdateframework/go-tuf/v2/metadata` additionally in exactly `source_release_coherence_native_linux_test.go` for hostile threshold-signed pair admission controls; `internal/network/epoch` and `internal/network/source` only in `source_initial_network_linux.go` for the fixed one-record genesis preparation/signing chain; `internal/network/source` additionally in `source_initial_network_native_linux_test.go`; `source prepare-install-inputs` and `source prepare-template` were removed (ADR-0125); the platform-led Source lifecycle and `ardents-next platform template` replace them; `internal/release/infrastructure`, `internal/release`, `github.com/sigstore/sigstore/pkg/signature` and `github.com/theupdateframework/go-tuf/v2/metadata` additionally only in `forwarding_release_linux.go`; `internal/release/infrastructure` additionally only in `forwarding_release_plan_linux.go` and `forwarding_release_native_linux_test.go`, with `github.com/theupdateframework/go-tuf/v2/metadata` additionally only in that genuine native signing test. After the #604 FJD stage D signing cutover these bind the static `ardents-next` program and `infrastructure.PlatformUnitTemplate`. These distinct fixed Forwarding initial/strictly-forward producers share physical input/output/key-codec mechanisms, never Source signing purpose or installed authority; `internal/enrollment`, `internal/release/infrastructure`, `internal/release` and `github.com/theupdateframework/go-tuf/v2/metadata` additionally only in `forwarding_release_predecessor_linux.go` for exact protected public predecessor custody, original five-key/three-threshold Root matching and genuine Forwarding coherent-pair authentication before forward signatures. `internal/release/infrastructure` additionally only in `forwarding_forward_release_native_linux_test.go` for the canonical platform template resource; no installed runtime or live root is composed by that test. Canonical Control has separate `prepare-forward-release-plan` and `sign-forward-release` consumers; forward metadata preserves exact initial Root bytes and strictly raises Targets/Snapshot/Timestamp versions; the former forwarding prepare-state producer and its state tests retired with the transitional Forwarding orchestration (#604 FJD stage D, ADR-0125); forwarding_state_preparation_native_linux_test.go uses standard-library command orchestration only and reads no private fixture keys; the former forwarding, JOIN and Descriptor prepare-hosting producers retired with the transitional orchestration (#604 FJD stage D, ADR-0125); `internal/network/epoch` additionally only in `forwarding_node_record_linux.go`, `forwarding_node_certificate_linux.go`, `forwarding_node_provision_linux.go`, `forwarding_node_certificate_linux_test.go` and `forwarding_node_provision_native_linux_test.go` for fixed local Node Record and certificate preparation/signing through new canonical-byte verification, with original protected Node-key custody and no membership or runtime authority; `internal/network/epoch` additionally only in `forwarding_epoch_plan_linux.go`, `forwarding_epoch_signing_linux.go` and `forwarding_epoch_provision_native_linux_test.go`, with `internal/network/source` additionally only in the latter two for the existing opaque single-materialization bundle codec; these fixed genesis producers grant no Source serving or runtime authority; `internal/admission/issuance` additionally only in `forwarding_token_material_linux.go` and `forwarding_token_material_native_linux_test.go`, with `internal/admission/issuerprofile` additionally only in that native test for independent public cohort validation; this finite offline material preparation grants no State, Permission or installed Issuer authority; `internal/network`, `internal/network/closedprofile`, `internal/network/epoch`, `internal/network/source` and `internal/admission/issuerprofile` additionally only in `forwarding_profile_plan_linux.go` for detached actual Epoch/inventory/profile joins; `internal/network/closedprofile` additionally only in `forwarding_profile_signing_linux.go` and `closed_profile.go` for the fixed State signing transcript and verifier (closed_profile.go cut over from the old closed-profile owner, #493); closedprofile, epoch and source additionally only in `forwarding_profile_native_linux_test.go` for genuine canonical public preparation controls. These offline consumers grant no State or installed Node/Issuer authority; `internal/admission`, `internal/admission/issuer`, `internal/admission/quota`, `internal/admission/receiving`, `internal/admission/stock`, `internal/admission/token`, `internal/custody`, `internal/network`, `internal/network/source` and `internal/network/state` additionally only in `forwarding_token_authority_native_linux_test.go` for actual encrypted Permission allocation, canonical material, authenticated State, finite blind issuance and presentation controls; these test-only imports grant no installed Issuer, Route or runtime authority; the former JOIN `prepare-template`/`prepare-request`/`prepare-plan`/`prepare-hosting`/`prepare-state` producers retired with the transitional JOIN orchestration (#604 FJD stage D, ADR-0125); the retained `join prepare-descriptor`/`prepare-inventory` consumers in `join_artifact_linux.go` use exactly `internal/enrollment` and `internal/release/infrastructure`. This supplies no runtime, Route import or Source/Forwarding signing or startup authority; the exact JOIN Release plan/signing/predecessor/native consumers and standard-library key producer are further constrained by the JOIN declaration and protection policy section above; the former Descriptor `prepare-template`/`prepare-request`/`prepare-plan`/`prepare-hosting`/`prepare-state` producers retired with the transitional Descriptor orchestration (#604 FJD stage D, ADR-0125); the retained `descriptor prepare-artifact`/`prepare-inventory` consumers in `descriptor_artifact_linux.go` use exactly `internal/enrollment` and `internal/release/infrastructure`, and the Descriptor Release plan/signing/predecessor/native consumers bind the static `ardents-next` program and `infrastructure.PlatformUnitTemplate` after the signing cutover; `internal/platform/systemd` only for the canonical infrastructure unit template in exact Release plan and native signing fixtures. |
| `tests/e2e/endpoint` | `package endpoint` with external `endpoint_test` cases | E2E-only public-command evidence for the closed-alpha Enrollment Pin and the retired-route refusals. It builds and executes the exact bundle artifact, accepts only its pinned local inventory through the successor enrollment verifier with a byte-identical diagnostic output contract, proves changed manifests fail before parsing, and proves the persisted-input enrollment-check form refuses before effects against the exact Endpoint artifact (ADR-0127, #647). It owns its ephemeral canonical TUF fixture writers for the offline artifact pin. The per-user Portable Endpoint lifecycle process evidence (enroll readiness/release floors, fixed-unit replacement/recovery, live two-Endpoint alpha-control agreement) is retired with that command surface (ADR-0125, #617 S3.5), and the enrollment-pinned ACA1 inspection regressions retired with the reader; the corpus-intake refusal stays covered by the control command tests. The Linux package-e2e case proves the direct-package payload shape and that install/upgrade/remove/purge retain user state outside package ownership while the retired enroll-installed route refuses before effects. It never signs or releases a maintained bundle and proves neither participant provenance nor an enrolled network runtime. | `github.com/sigstore/sigstore/pkg/signature`, `github.com/theupdateframework/go-tuf/v2/metadata`, standard library |
| `tests/e2e/service/fixturecommand/publish-app` | `package main` | E2E-only separately granted publication-operator fixture. It uses only the bounded local Service Administration socket and is built solely by the Endpoint recovery process test. | standard library |
| `tests/e2e/service/fixturecommand/stream-app` | `package main` | E2E-only paired direct opaque-stream workload used by retained qualification evidence. It owns deterministic bounded bytes and receipts but no Endpoint-local transport grammar. | standard library |
| `tests/e2e/node/fixturecommand/netem-relay` | `package main` | Carrier-qualification-only static TCP relay. It accepts one declared Linux netem mode (`delay`, 100% `drop`, or fixed delay/loss/reorder), applies it only to its own Docker `eth0` through a supplied `tc`, and forwards at most 1 MiB in each direction to one supplied upstream. It receives no Node State, credentials, Route, or Service input and is neither a maintained Node nor a Carrier implementation. | standard library |
| `tests/epochfixture/assignment` | `package assignment` | Independently construct deterministic role-domain assignments for black-box Network State fixtures; never serve maintained product code. | standard library |
| `tests/epochfixture/merkle` | `package merkle` | Independently construct canonical Epoch commitment roots and proofs for black-box Network State fixtures; never serve maintained product code. | standard library |
| `tests/epochfixture/network` | `package network` | Construct canonical signed Node Record, Epoch and materialization bytes for black-box Network State and successor provisioning tests. It supplies deterministic test input bytes only and cannot accept State, choose a runtime Route or serve maintained product code. | `tests/epochfixture/assignment`, `tests/epochfixture/merkle`, standard library |






| `internal/architecture` | `package architecture` | Test repository structure, ownership inventory, package/import boundaries, artifact-lane separation, extraction rehearsals, names, dependencies, formatting, and quality wiring. It owns the bounded AAI2 retirement closure guard whose exact receipt and scope are defined by [Testing](testing.md#reachability-audit). | standard library |
| `internal/diagnostics/process` | `package process` | Owner-private heap dump verification and parsing harness. | Standard library. Heap dump role-map tests consume `internal/admission`. |
| `internal/diagnostics/timeline` | `package timeline` | Project bounded local Node, Source, and Endpoint runtime JSON or journalctl events into one safe, time-ordered operator view without retaining raw input or opening an authority root. The `ardents-next diagnostics timeline` command selects its input and output. | standard library |
| `internal/application/administration` | `package administration` | Own the separately authorized Application Interface v1 Publish/Withdraw contract, lifecycle, closed grammar, failure response, conformance vectors, local server transport, and Application client. Its bounded snapshot operation dispatches only to an implementing publication owner; unsupported owners refuse without bodyless fallback, and one snapshot allocation/transition excludes another while withdrawal remains available. Its server serializes accepted-socket close and ownership retirement, joins handlers before returning shutdown, and retains actual close failures. Its Linux text-client published-Link query projects bounded printable destination text only from an implementing separately authorized owner; it carries no caller-supplied Target, Connection bytes, Credential, Service key, State, Route, peer, or Custody fact. | standard library |
| `internal/release` | `package release` | New isolated offline Release verifier with separately owned trust history, serialized original evaluations and joined Close. Keeps Root-only versus complete metadata floor publication distinct, rejects lost retained pointers, and returns private immutable authorization. Genuine `cmd/ardents-next/release.go` consumes pinned new Enrollment bytes; `cmd/ardents-next/installation.go` opens complete retained history with `OpenRetained` for fresh candidate evaluation, without creating cold trust; no old runtime, installation, signing or live readiness. Portable metadata/target/policy and floor rules use native lease/publication/flush adapters; other Unix platforms have no runtime qualification from compilation. | Standard library; `github.com/theupdateframework/go-tuf/v2/metadata` only in `metadata_authentication.go`, `target_identity.go`, `safety_policy.go`, `floor_encoding.go`; `github.com/theupdateframework/go-tuf/v2/metadata/trustedmetadata` only in `metadata_authentication.go`; `golang.org/x/sys/windows` only in `history_native_windows.go`; behavior signing fixtures only: `metadata` and `github.com/sigstore/sigstore/pkg/signature` in `rotation_test.go`, `metadata` in `generation_authorization_test.go` |
| `internal/release/endpoint` | `package endpoint` | Authenticates the exact Endpoint program and generation as a coherent Release pair using the original Enrollment inventory. Real consumers are `platform_endpoint_linux.go` and Endpoint launch. | `internal/enrollment`, `internal/release`, standard library |
| `internal/endpoint/launch` | `package launch` | Stages authenticated Endpoint generations, requires original predecessor completion before signed replacement, and independently retains original installed observations at startup. Real consumers are Endpoint participant and `platform_endpoint_staging_linux.go`. | `internal/endpoint/generation`, `internal/endpoint/plan`, `internal/endpoint/unit`, `internal/enrollment`, `internal/platform/cgroup`, `internal/platform/process`, `internal/platform/systemd`, `internal/release`, `internal/release/endpoint`, standard library |
| `internal/endpoint/plan` | `package plan` | Validates the bounded Endpoint declarations and computes writable roots on every host. Linux request-file custody retains byte, inode and access provenance. Real consumers are Endpoint launch and `platform_endpoint_linux.go`. | Standard library |
| `internal/endpoint/unit` | `package unit` | Pure Endpoint unit template rendering and policy over typed systemd facts. Endpoint launch, platform pre-start and Worker activation are real consumers. | `internal/platform/systemd`, standard library |
| `internal/platform/systemd` | `package systemd` | Owns the signed infrastructure unit template, typed systemd observations, original manager references and bounded transport with joined cancellation. Real consumers include platform launch, Endpoint launch, Worker unit checks and State role repair. | `github.com/godbus/dbus/v5` only in exact transport and manager files registered below, standard library |
| `internal/platform/cgroup` | `package cgroup` | Reads bounded kernel scopes and checks quiescence for Endpoint replacement and fixed-role State repair. | Standard library |
| `cmd/ardents-text` | `package main` | Adapt the fixed installed reader/Publisher worker entrypoints, the trusted one-read AAI3 client and the owner-selected text publication client. Publication imports a stable bounded file through textdocument and transfers only its snapshot through the existing Administration boundary; a missing snapshot publication owner refuses. The UI takes a bounded Target Link from standard input, joins the complete exchange before escaped plain-text presentation, and retains no history or automatic retry. An explicit `link` command queries the committed publication through Administration and presents its bounded public destination through interruptible output without publication effects. Worker dispatch precedes UI signal setup. Diagnostics contain only local failure classes. | `internal/application/textdocument`, `internal/application/connection`, `internal/application/administration`, standard library |
| `cmd/ardents-stream-qualification` | `package main` | Implement only the fixed reader or Publisher entrypoint of the separately pinned NET-14 stream-test Application. It accepts one fixed local initialization record, returns its matching readiness acknowledgement, and transfers only the fixed worker-frame bridge; no argument supplies a profile, target, address, executable, authority, Principal or Grant. | `internal/application/streamqualification`, standard library |
| `internal/application/connection` | `package connection` | Own AAI3 typed local requests, refused reserved Name destinations, bounded ordered streams, directional EOF, terminal outcomes and joined cancellation. The server, request decoder and typed refusal constructor compile with the Linux text Endpoint; the client, request encoder, shared data framing and outcome validation remain portable. This grammar does not attest a worker launch or create local authority. | standard library |
| `internal/application/streamqualification` | `package streamqualification` | Own the fixed `ARDTQP01` local initialization/readiness and bounded worker-frame bridge, authenticated Service-stream transfer and per-stream workload verdict for the separately pinned NET-14 stream-test worker. Its two retained 64/16 client and 256/64 Publisher workload shapes send only deterministic predeclared bytes after Endpoint OPEN/CREDIT and validate received bytes before replenishing exact credit. It defines no target, Route, address, executable, authority, Principal, Grant, or arbitrary caller-selected schedule. The initialization/readiness codec, worker-frame grammar, worker workload and Endpoint-side connection bridge are portable and compile and behavior-test in the portable deterministic inventory; the Endpoint-side `InitializeWorker`/`RunConnections` have only the Linux qualification composition (`internal/execution/runtime`) as production consumer, and installed stream worker activation remains Linux-only. | `internal/application/connection`, standard library |
| `internal/application/textdocument` | `package textdocument` | Own the trusted Linux no-follow import of an explicitly selected regular file, the portable immutable bounded UTF-8 snapshot and its fixed document responder, the shared document reader, independently escaped plain-text presentation, nonce-bound reader/Publisher worker exchange, finite stream credit and round-robin output. Presentation validates the whole body before writing and escapes C0/C1 and Unicode bidi controls, preserving LF and tab. Its reader bridge transfers exactly one admitted Service stream, validates the complete fixed request before forwarding, waits for authenticated terminal completion, validates bounded worker RESULT, and joins owned I/O before returning bytes. Its Publisher bridge receives at most 256 admitted Service streams, schedules up to 64 worker exchanges, and gives each active response a 64 KiB byte queue plus one at-most-16 KiB write. Credits follow consumed or discarded bytes; malformed requests and Service loss remain stream-scoped, while worker protocol violations close the job. Endpoint still owns job/Grant checks and cgroup cleanup. The fixed Linux entrypoint checks inherited local descriptors before its readiness exchange; it identity-verifies and closes any manager-passed resource-control descriptor of its own unified cgroup (systemd 255 `cpu.max` passing for resource-limited units) and refuses every other surviving descriptor, including own-cgroup migration/lifecycle surfaces; it cannot attest the installed unit or issue a Grant. The document grammar, snapshot exchange, worker INIT/READY and frame codecs, reader/Publisher bridges and worker multiplexer are portable and compile and behavior-test in the portable deterministic inventory; the no-follow file import (`snapshot_file_linux.go`) and the installed inherited-descriptor entrypoint (`worker_process_linux.go`) remain Linux-only with explicit typed unsupported-platform refusals, and the portable codecs' production consumers remain the selected Linux installed-worker, Execution and Endpoint compositions. | `internal/application/connection`, standard library |
| `internal/publication` | `package publication` | Own canonical public Service v3 request/response grammar and unsigned fixed-purpose transcript, verify exact public Credential/Publication v3 signatures, Target/Network/validity and separate read versus publish delegation. Retain copied public proof bytes; no Authority issuance, private Instance signer/root, readiness or Route/Store ownership. | standard library |
| `internal/publication/durable` | `package durable` | Independently owned exclusive v3 Publication root, synced ancestor/marker/layout preparation, exact consumed-generation reservation, immutable public record/current persistence and bounded canonical floor/public-record reconciliation. Record publication joins staging/generation/pointer barriers; exact retry preserves bytes/inodes, and original generation Close retires pointer before record without lowering the floor. Restored public evidence grants no private Instance or accepting Publisher. Real consumer: `cmd/ardents-next/publication.go` through `publication prepare-root`; this exact command file additionally imports this package. Portable tests isolate required flush failures; Linux tests use actual lease and directory barriers. | `internal/publication`; `golang.org/x/sys/windows` only through `lease_windows.go`; standard library |
| `internal/publication/instance` | `package instance` | Host-generated non-exporting Instance key, stable public approval request and verified external response through pure Publication and exclusive durable pending/accepted/rejected/conflicting lifecycle. Exact accepted retries preserve the response; terminal or ambiguous transitions deny further authority receipts and clear private material. Native Linux private custody retains original root/lease identity; other platforms refuse before key/filesystem creation. Public grammars and sealed Credential verification remain portable. Real consumer: `cmd/ardents-next/publication_instance.go` public initialization/request/response operations. Original private binding now reconciles a concrete `publication/durable` owner, advances its exact reserved floor and persists consumed redaction before retaining volatile purpose-specific signing. Publication-record signing requires the original genuine Introduction registration receipt and persists the exact consumed record/current through its retained durable reservation. Canonical Descriptor signing consumes Reachability transcript construction and an independent volatile X25519 recipient with strictly increasing real registration revision and capacity two; public preparation commands do not establish that live signing consumer or readiness. Purpose-specific capsule opening retains the original recipient through an opaque user handle, checks exact registration/publication/profile binding and joins all users before erasure/capacity return. Candidate plaintext grants no downstream or replay acceptance. No generic signer or Authority custody. ServiceAttachment confines TLS1.3 CertificateVerify and one-use verified initial challenge proof to the original private opening and admitted JOIN; actual coalesced response write plus final live checks transfer setup into unchanged Work Safety, while the separate immutable ServiceLease retains original Instance authority across physical Attachment gaps and earlier cleanup failures. Logical retirement joins current TLS users before returning that borrow; Publication joins logical leases before key erasure. | `internal/publication`, `internal/publication/durable`, `internal/reachability`, `internal/route/introduction`, `internal/route/capsule`, `internal/connection/wire`, `internal/route/join`, `internal/execution/runtime`; standard library |
| `internal/publication/runtime` | `package runtime` | One original qualified snapshot Publisher, exclusive Instance lifetime borrow, genuine Introduction registration and actual Source Descriptor Store exchange. Own pending/current/bounded predecessor pairs, exact retry, strictly increasing revision, creation+300s real-clock refresh, joined withdrawal and terminal result. Canonical Link is visible only after actual synced Store ACK and final original Execution/Instance/registration/Descriptor checks. Real non-test consumer: `internal/endpoint/publisher/opening_linux.go` through the qualified holder console; capsule opening, replay and native acceptance remain separate obligations. | `internal/connection`, `internal/execution/runtime`, `internal/network`, `internal/publication`, `internal/publication/instance`, `internal/reachability`, `internal/route`, `internal/route/introduction`, `internal/route/prefix`, `internal/route/capsule`, `internal/route/join`; `internal/connection/wire` only in `publisher_streams.go` for copied full-tuple dispatch commitments, with exact-file isolation enforced separately; standard library |
| `internal/connection` | `package connection` | Immutable initial recipient logical/Attachment binding under the exact qualified Execution operation, sealed independently verified Publication and original local Work Safety bounds. Refuse substitution, extension and recovery without retained continuity. Real non-test consumer: `publication/runtime` capsule opening. The one-use initial authentication consumer composes logical send/receive and Terminal state with a checked original Execution byte contract and joined TLS users; Publication NextAuthentication consumes a genuine initial delivery and the same Source-bound Responder JOIN. Reader's initial Source factory retains the same genuine Source across independent Target-Link/Descriptor verification, Introduction submission, JOIN and Instance authentication; original history and Rendezvous selection remain caller-owned. Recovery and real two-role command/worker acceptance remain required. Copied contexts grant no Instance TLS, accepted replay, Responder, ordered stream or Application bytes; full two-role command/recovery acceptance remains separate. | `internal/execution/runtime`, `internal/publication`, `internal/route/capsule`, `internal/connection/wire`, `internal/publication/instance`, `internal/route/join`; `internal/network`, `internal/reachability`, `internal/route`, `internal/route/introduction`, `internal/route/prefix`, `internal/route/selection` only in `initiator_source.go`; `internal/route/prefix` additionally only in `publisher_recovery.go` for retained original Source/Responder recovery scope; `internal/application/connection` only in `stream.go` and `stream_test.go`; standard library |
| `internal/connection/wire` | `package wire` | Closed canonical Service Connection v3 records, immutable logical/Attachment transcripts, TLS exporter binding and one-use verified initial request receipt. Genuine non-test callers are Connection recipient binding and Instance Service key use. Pure checked values grant no local permission, accepting Publication or Route. No generic signer or owner callback. | Standard library only |
| `internal/reachability` | `package reachability` | Construct the canonical public Descriptor transcript and independently verify its completed signature; verify bounded private Descriptor v3, independently selected lookup Target/current profile and publication delegation before receiving effects; own the separate receiving Store root, durable conflict floors and bounded interrupted-write recovery. The bounded `RepairRetainedStoreStage` clears only an uncertain staging file of an EXISTING retained root under its exclusive lease after validating every committed floor (the Store half of platform-led Descriptor repair, called after independent dead-unit proofs); it initializes no missing root, resets/evicts no floor, promotes no staging, deletes no foreign entry and grants no publication or runtime authority, and `OpenStore`/`OpenRetainedStore` behavior is unchanged. Store retains original native root/records/marker/lock/committed-record custody and sync descriptor, permanent operational denial, separately serialized publications and joined original borrows before first physical release. Receiver consumes Check without foreign observer recursion; request refusal cannot retire a healthy owner. Local History retains only context-private monotonic facts and the exact joined lookup flight; no offline proof cache. Pure verification, history and floor transitions are portable; native Linux/Windows lease and directory synchronization remain mechanisms. No Route selection/transport, private Instance authority or readiness. | `internal/publication`, `golang.org/x/sys/windows` through `store_platform_windows.go` only, standard library |
| `internal/hosting` | `package hosting` | Shared durable provider-period accounting, work/cleanup deadlines, opaque reservations and bounded confirmed persisted snapshots; separate Linux read-only existing-history inspection retains original policy/principal/lease and requires returned reservations, without charging or refunding. No placement or network authority. | standard library |
| `internal/admission/issuance` | `package issuance` | Own immutable offline RSA material, unsigned inventory, immutable signed issuer profiles and durable confirmed blind-token results; exclusive Linux leases, no live authority. | `internal/admission`, `internal/admission/issuerprofile`, `internal/admission/issuerprofile/signing`, `github.com/cloudflare/circl/blindsign/blindrsa`, `internal/admission/quota`, standard library |
| `internal/admission/issuer` | `package issuer` | Compose offline and current-authority open/debit/sign/retain/close and pinned issuer profile preparation/inspection; retain the primary result and every acquired owner cleanup before finite command projection. No quota arithmetic or private signing implementation. | `internal/admission`, `internal/admission/issuance`, `internal/admission/issuerprofile/signing`, `internal/admission/quota`, standard library |
| `internal/admission/issuerprofile/signing` | `package signing` | Import independently pinned Ed25519 Node key into exclusive immutable Linux storage; sign only opaque validated issuer-profile requests. No private-key or general signing API. | `internal/admission/issuerprofile`, standard library |
| `internal/custody` | `package custody` | Encrypted purpose-specific Vault/Recovery Bundle, original serialized operation/cancel/join/erasure, Admission signing through genuine Allocation and durable successor/readback/floor. Service Authority creation and independently approved fixed-purpose Credential issuance use the pure public v3 contract; predecessor Service/retired Name records remain encrypted evidence; restored quarantine has no signing/activation API. No predecessor backend or live-root adoption. Real consumer is `cmd/ardents-custody`. | `internal/admission`, `internal/admission/allocation` and pure `internal/publication`, standard library; `golang.org/x/crypto/argon2` only in `canonical_envelope.go`; `golang.org/x/sys/windows` only in `vault_lock_windows.go`, `vault_lock_windows_test.go` and `durability_windows.go` |
| `internal/admission/allocation` | `package allocation` | Permission allocation, role maxima and retained allocation decisions. | `internal/admission`, standard library |
| `internal/admission/attempts` | `package attempts` | Durable holder presentation history. | standard library |
| `internal/admission/quota` | `package quota` | Exclusive issuer quota journal, atomic debit, exact retry and opaque debit confirmation. | `internal/admission`, `internal/admission/issuerprofile`, standard library |
| `internal/admission/receiving` | `package receiving` | Receiver binding, redemption and finite token allowance; separate original-operation serialization and synchronous Close denial, with foreign observation/reservation and refused cleanup outside the state mutex and spend-lease return after user join. Accepted work owns transferred reservations. | `internal/admission`, `internal/admission/token`, `internal/admission/spending`, standard library |
| `internal/admission/spending` | `package spending` | Receiver token spend history and distinct Linux existing-only read-only inspection with original numeric principal/lease and joined physical completion; no Introduction slot ownership or runtime authority from inspection. | `golang.org/x/sys/windows`, standard library |
| `internal/route/introduction` | `package introduction` | Own holder Registration request/reader/owning withdrawal and joined result over opaque Prefix channels, separately receiving Registry/live slots, canonical requests/results and durable non-reclaim History. No shared principal, root or lifetime, listener, token spend, Hosting budget or Publication readiness. | `internal/admission/receiving` only through `receiving_history_linux.go` for the genuine one-use root creation fact, `internal/admission/spending`, `internal/route`, `internal/route/ardp`, `internal/route/role`, `internal/route/prefix`, `internal/network`, `internal/admission`, `internal/route/capsule`, standard library |
| `internal/route/capsule` | `package capsule` | Closed v3 fixed envelope and recipient-only request grammar, exact domain/header transcript and X25519/HKDF-SHA256/AES-128-GCM public-key sealing. Immutable copied envelopes and candidate requests grant no authority, private key, replay acceptance, Connection binding or live Route. Actual consumers: `publication/instance` purpose-specific borrowed opening and native `ardents-next` Source preparation followed by its genuine admitted Submit exchange; independent codec/suite controls and genuine Registration composition exercise the same grammar. No new package-level authority or old runtime connection. | standard library |
| `internal/admission/stock` | `package stock` | Holder permission, pending exact batch, stock and irreversible presentation. Application owns exchange execution and cancellation. | `internal/admission`, `internal/admission/allocation`, `internal/admission/attempts`, `internal/admission/token`, standard library; behavior tests only: `internal/admission/issuer`, `internal/admission/issuance`, `internal/admission/receiving`, `internal/admission/spending`, `internal/admission/quota` |
| `internal/admission/token` | `package token` | Holder blind preparation, finalization, token verification and batch grammar. | `internal/admission`, `internal/admission/issuerprofile`, `github.com/cloudflare/circl/blindsign/blindrsa`, standard library |
| `internal/platform/process` | `package process` | Retains the actual Endpoint invocation facts and independently observes Source principal quiescence. Real consumers are Endpoint launch and Source State repair. | Standard library |
| `internal/endpoint/generation` | `package generation` | Owns Endpoint generation inventory, exclusive creation/sealing and independent retained snapshots. Real consumer is Endpoint launch. | Standard library |

The exact Service Custody process scenario `tests/e2e/service/service_custody_process_linux_test.go` imports only pure `internal/publication` beyond the existing process mechanisms. It drives built `ardents-next` host request/acceptance and real no-echo `ardents-custody` Authority/issuance commands, with no Instance import or fixture signing authority. `cmd/ardents-custody/service_authority.go` now consumes `internal/custody` directly; the former Service backend is not called.

The Direct-Origin Source composition in `cmd/ardents-next/source_linux.go` consumes only
new `network/state` and `processresource`; `source_failure_linux.go` additionally
consumes only `processresource`. `source_native_linux_test.go` consumes exactly
new `network/state`, `network/source`, `network/epoch`, `processresource` and
`tests/epochfixture/network`. `source_lifetime_linux_test.go` consumes
new State snapshots for refusal/join ordering only. The `source serve <plan>`
consumer supplies original process admission; State retains serving/duty policy
and joins transport before command releases process pins. No Node or Route owner
is imported. Source output uses bounded original native stream writes.
The Source command additionally retains State's opaque SourceCompletion through
all original command cleanup/output, finishing it once with actual errors.
State's source_completion.go owns only that original completion/result and
producer release, using the existing local participation mechanism. It creates
no installer, package, persisted format, policy callback or software authority.
The explicit Source repair boundary is registered with the platform composition
above. Its native adapter accepts only a real original manager connection and
independent kernel quiescence under both retained leases; no caller-supplied
process-death callback grants authority. The command retains the public plan
without loading a private transport key and initializes no runtime or root.

`source_custody_linux.go` and its native scheduling/file-custody tests use only
the standard library. Its genuine consumer is the existing Source command;
the unexported lifetime retains original fixed files/ancestors, synchronous
denial, native-use joins and copied terminal results. It borrows only unchanged
direct-open/check/close primitives from `network_preparation_linux.go`, without
offline signing's admission policy. No new package, dependency, authority or
exported Interface is introduced. State/Source preserve their own effects and
join; command cancellation can start that same original Close while native
input observation is held.

`source_state_linux_test.go` additionally imports only new `network/duty` and
`network/source` for actual serving/guard retirement on failed readiness output.
`source_fixture_linux_test.go` contains independent public TLS fixture preparation
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

The standalone `scripts/sign-qualification-release.go` implements only the
ADR-0120 initial Endpoint Release operation. Its exact permitted imports are
`internal/enrollment`, `internal/release`,
`github.com/sigstore/sigstore/pkg/signature`,
`github.com/theupdateframework/go-tuf/v2/metadata`, `golang.org/x/sys/unix` and
the standard library. Enrollment validates the static protected resources;
Release verifies both actual signed targets in fresh diagnostic histories.
No old runtime, Network, Route, installation or generic signing authority is
admitted. This permission does not extend to adjacent signing scripts.

### JOIN retained signing/provisioning import inventory

The transitional JOIN first-start/replacement/recovery/transition orchestration
and its canonical Node consumers retired (#604 FJD stage D, ADR-0125); the
platform-led JOIN lifecycle owns installed delivery. The retained offline JOIN
producers keep their own fixed purpose and original input/output custody; the
read-only repair runtime manager keeps its own godbus connection.

- `cmd/ardents-control/join_release_linux.go`: `internal/release/infrastructure`, `internal/release`, `github.com/sigstore/sigstore/pkg/signature`, `github.com/theupdateframework/go-tuf/v2/metadata`.
- `cmd/ardents-control/join_token_material_linux.go`: `internal/admission/issuance`.
- `cmd/ardents-control/join_epoch_signing_linux.go`: `internal/network/epoch`, `internal/network/source`.
- `cmd/ardents-control/join_release_plan_linux.go`: `internal/release/infrastructure`, `internal/platform/systemd`.
- `cmd/ardents-control/join_profile_plan_linux.go`: `internal/admission/issuerprofile`, `internal/network`, `internal/network/closedprofile`, `internal/network/epoch`, `internal/network/source`.
- `cmd/ardents-control/join_artifact_linux.go`: `internal/enrollment`, `internal/release/infrastructure`.
- `cmd/ardents-control/join_node_certificate_linux.go`: `internal/network/epoch`.
- `cmd/ardents-control/join_epoch_plan_linux.go`: `internal/network/epoch`.
- `cmd/ardents-control/join_profile_signing_linux.go`: `internal/network/closedprofile`.
- `cmd/ardents-control/join_node_record_linux.go`: `internal/network/epoch`.
- `cmd/ardents-control/join_release_predecessor_linux.go`: `internal/enrollment`, `internal/release/infrastructure`, `internal/release`, `github.com/theupdateframework/go-tuf/v2/metadata`.
- `cmd/ardents-control/join_node_provision_linux.go`: `internal/network/epoch`.
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

| `internal/endpoint/funding` | `package funding` | Application coordination of original Source stock preparation, issuance and presentation; shared Entry/Hosting parent lifetime. Reader, Publisher and bootstrap callers remain in `cmd/ardents-next`. Admission retains finite rights/debit and durable presentation; Network retains current authority; Route retains selection and physical join. | `internal/admission`, `internal/admission/stock`, `internal/admission/token`, `internal/hosting`, `internal/network`, `internal/network/state`, `internal/route`, `internal/route/ardp`, `internal/route/bootstrap`, `internal/route/prefix`, `internal/route/role`, `internal/route/selection`; standard library. |

| `internal/endpoint/reader` | `package reader` | One original qualified Reader operation shared by console and installed adapters, Source/history ownership, sole stream handoff, local fixed Text request/result projection, interruption and joined retirement. Real callers: installed and console Reader in `cmd/ardents-next`. Borrows participant authority/Stock/Execution/Parents; no CLI or publication decisions. | `internal/application/connection`, `internal/application/textdocument`, `internal/admission`, `internal/admission/stock`, `internal/connection`, `internal/endpoint/funding`, `internal/execution/runtime`, `internal/hosting`, `internal/network`, `internal/reachability`, `internal/route/prefix`, `internal/route/selection`; standard library. |

| `internal/endpoint/publisher` | `package publisher` | Qualified snapshot publication application lifecycle: pending/active handoff, Source and role selections, Instance/history custody, stream delivery, withdrawal and joined cleanup. Real callers: installed participant and holder console in `cmd/ardents-next`. Domain Publication retains readiness/refresh/recipient policy. | `internal/application/textdocument`, `internal/admission`, `internal/admission/stock`, `internal/endpoint/funding`, `internal/execution/runtime`, `internal/hosting`, `internal/network`, `internal/publication/durable`, `internal/publication/instance`, `internal/publication/runtime`, `internal/route/prefix`, `internal/route/selection`; standard library. |

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

- `cmd/ardents-control/descriptor_release_linux.go`: `internal/release/infrastructure`, `internal/release`, `github.com/sigstore/sigstore/pkg/signature`, `github.com/theupdateframework/go-tuf/v2/metadata`.
- `cmd/ardents-control/descriptor_token_material_linux.go`: `internal/admission/issuance`.
- `cmd/ardents-control/descriptor_epoch_signing_linux.go`: `internal/network/epoch`, `internal/network/source`.
- `cmd/ardents-control/descriptor_release_plan_linux.go`: `internal/release/infrastructure`, `internal/platform/systemd`.
- `cmd/ardents-control/descriptor_profile_plan_linux.go`: `internal/admission/issuerprofile`, `internal/network`, `internal/network/closedprofile`, `internal/network/epoch`, `internal/network/source`.
- `cmd/ardents-control/descriptor_artifact_linux.go`: `internal/enrollment`, `internal/release/infrastructure`.
- `cmd/ardents-control/descriptor_node_certificate_linux.go`: `internal/network/epoch`.
- `cmd/ardents-control/descriptor_epoch_plan_linux.go`: `internal/network/epoch`.
- `cmd/ardents-control/descriptor_profile_signing_linux.go`: `internal/network/closedprofile`.
- `cmd/ardents-control/descriptor_node_record_linux.go`: `internal/network/epoch`.
- `cmd/ardents-control/descriptor_release_predecessor_linux.go`: `internal/enrollment`, `internal/release/infrastructure`, `internal/release`, `github.com/theupdateframework/go-tuf/v2/metadata`.
- `cmd/ardents-control/descriptor_node_provision_linux.go`: `internal/network/epoch`.
- `cmd/ardents-control/descriptor_release_native_linux_test.go`: `internal/release/infrastructure`, `internal/platform/systemd`, `github.com/theupdateframework/go-tuf/v2/metadata`.
- `cmd/ardents-control/descriptor_forward_release_native_linux_test.go`: `internal/release/infrastructure`, `internal/platform/systemd`.
- `internal/release/infrastructure/descriptor_admission.go`: `internal/enrollment`, `internal/release`.
- `internal/platform/systemd/descriptor_manager_linux.go`: `github.com/godbus/dbus/v5`.

`cmd/ardents-control/descriptor_release_keys_linux.go`,
`descriptor_release_arguments_linux.go`, `descriptor_release_plan_linux_test.go`
and `descriptor_artifact_native_linux_test.go` use only the standard library.
Fixed initial/forward key/plan/signing commands share native key/input/output
codecs, never Source/JOIN signing purposes. The generic `descriptor
prepare-artifact`/`prepare-inventory` consumers retain original protected public
input and exclusive durable output; `prepare-template`/`prepare-request`/
`prepare-plan`/`prepare-hosting`/`prepare-state` were removed.
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
library. Its real caller is `receiver_linux.go` at every existing parent frame
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
| `internal/endpoint` | `package endpoint` | Original installed participant lifetime, bound Endpoint declaration projection and exact permission file exchange. `ardents-next endpoint construct-plan` renders canonical preparation plans from accepted retained State pins, bounded operator declarations and the distributed Source artifact, without runtime authority. `ardents-next endpoint prepare-history` explicitly authenticates initial signed State/profile and prepares vacant participation, Entry/Interior and role Stock; `start-installed` requires retained histories. Offline Source preparation consumes detached declaration types. IPC and Service users join before shared funding, Stock, State, local Execution and Startup. No domain policy or process exit projection. | `internal/application/administration`, `internal/application/connection`, `internal/admission`, `internal/admission/stock`, `internal/endpoint/funding`, `internal/endpoint/reader`, `internal/endpoint/publisher`, `internal/execution`, `internal/execution/runtime`, `internal/hosting`, `internal/endpoint/launch`, `internal/network`, `internal/network/source`, `internal/network/state`; `internal/network/duty` and `internal/route/selection` only in `history_preparation_linux.go` and `history_preparation_linux_test.go`; tests only: `internal/admission/attempts` in `history_preparation_linux_test.go`, `tests/epochfixture/network` in `history_preparation_linux_test.go` and `plan_construction_linux_test.go`; `internal/application/connection` and `internal/application/textdocument` tests only in `latency_native_linux_test.go` (the #650 cold/warm latency participant runs as the installed Endpoint MainPID, retains the original dual-role owner composition and measures the ordinary Reader composition directly, opening the public Connection request and reading the validated text response; it owns no Application server); standard library. |

| Path | Package | Responsibility and real consumer | Permitted imports |
|---|---|---|---|
| `internal/endpoint/session` | `package session` | Serial original holder scenario: attempt/intent, Prefix/JOIN, registration, Publication, lookup history and joined Stock closure. Separate local, preparation and live-operation constructors; each typed method checks original Execution permission. Real consumer: ardents-next holder console. Native Route construction retains selection and Hosting through physical join; no command DTO, JSON, output or process status. | `internal/admission`, `internal/admission/stock`, `internal/endpoint/funding`, `internal/endpoint/publisher`, `internal/execution/runtime`, `internal/hosting`, `internal/reachability`, `internal/route/ardp`, `internal/route/bootstrap`, `internal/route/capsule`, `internal/route/introduction`, `internal/route/join`, `internal/route/prefix`, `internal/route/role`, `internal/route/selection`; standard library. |

Retained State/duty opens require their original lease and refuse missing,
symlink/reparse-point or hardlinked lease files before initializing any history.
State's portable retained opener changes no authority or native-runtime support.
Admission `attempts.OpenRetained` and `stock.OpenRetained` require existing
presentation history; the Reader and existing participant directories consume
them. Only a genuinely new participant directory can initialize Stock.

`cmd/ardents-next/execution_artifact_linux_test.go` directly verifies real signed
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
owner. `network/state.PrepareInitialSource` joins the signed State acceptance
and empty participation initialization. The real caller in
`cmd/ardents-next/source_state_preparation_linux.go` retains the original native input
and vacant root descriptors and supplies their reobservation. No runtime authority or
transport is created; retained runtime openings remain noncreating.
