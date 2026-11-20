# Command reference

`ardents-control forwarding prepare-profile --network-id HEX32
--authority-key HEX32 --issuance-authority-key HEX32 --bundle ABSOLUTE_BUNDLE
--token-inventory ABSOLUTE_PUBLIC_INVENTORY --duty NODE_HEX32:DOMAIN:SUBROLE
--output FRESH_ABSOLUTE_DIRECTORY` prepares only the initial signed Epoch's
closed profile. Repeat `--duty` up to 32 times in canonical Node-ID order;
every other option is required once. Domains/subroles are the existing closed
profile numbers. No role or family is selected automatically. The genuine
canonical token-key producer's public inventory supplies the fixed aligned
interval and keys; its issuer/public Node signer must match the actual Epoch.
The State pin and separate permission authority pin must be independently
reviewed. The exact signed Epoch, canonical key cohorts, Record references,
domain assignments and profile interval are verified before output. On protected
root-owned ext4, fresh output contains `profile-plan.json` and `profile.unsigned`.

`ardents-control forwarding sign-profile PLAN PREPARED REVIEWED_SHA256 STATE_KEY
FRESH_OUTPUT` rebuilds that exact original public preparation before reading
the matching root-owned 0600 PKCS8 State key and writes only `profile.bin`.
The frozen preparation reference allows five minutes of elapsed time and one
minute of future skew within the original profile interval; it is never renewed
by signing. Unknown/duplicate scalar options, changed bytes or pins, conflicting
roles, stale time, wrong signer and existing/partial output refuse without
adoption or reset. Receipt SHA names the unsigned/signed artifact respectively;
State acceptance, runtime authority and installed qualification remain false.
State must independently authenticate and persist this profile before use.

`ardents-control forwarding prepare-token-keys --network-id HEX32
--issuer-node-id HEX32 --issuer-node-key HEX32 --start UTC_HOUR --end UTC_HOUR
--output FRESH_ABSOLUTE_DIRECTORY` prepares one to six aligned UTC hours of
offline Admission material. Each scalar option is required exactly once. The
public issuer Node key is explicit; this command opens no Node private key and
appoints no issuer. Run as root on protected ext4 with an existing root-owned
0700 output parent. The new output contains `token-key-plan.json`, Admission's
private `material` root and the separate public `public/token-inventory.json`.
Only the public inventory SHA-256 and false authority/qualification flags reach
stdout. Retain the private material root separately from public distribution;
the public file cannot authorize issuance or replace a Node-signed issuer
profile or State-signed closed profile. Existing or partial output refuses
without adoption or reset. Cancellation joins original Admission/native users;
no installed Node/Issuer, token issuance or bootstrap acceptance occurs.

Canonical Linux `ardents-control forwarding prepare-state-key FRESH_PRIVATE_OUTPUT`
creates a separate protected `state-key.pem` and returns only its public key.
It grants no installed trust or State acceptance. `forwarding prepare-epoch`
requires `--network-id`, `--authority-key`, `--assignment-seed`, `--node-id`,
`--valid-from`, `--valid-until`, `--domain`, `--record-plan`, `--record` and
`--output`. Repeat `--domain` in canonical order (1..16), and pair each of 1..64
`--record-plan` paths with the corresponding `--record` in the supplied order.
These are actual outputs from Node Record preparation/signing; no JSON editing
or private Node-key read is required. The fixed seed is never redrawn. It writes
`epoch-plan.json` and `epoch.unsigned`. The selected Node must occur in the exact
signed inputs; selecting its proof does not select or grant a duty.

`forwarding sign-epoch PLAN PREPARED REVIEWED_SHA256 STATE_KEY FRESH_OUTPUT`
rebuilds exact AREP-v3 genesis bytes, matches the reviewed unsigned commitment,
and only then admits the protected matching State key. This bounded producer
supports one explicit State authority and threshold one; it neither changes
runtime trust nor weakens another configured threshold. New Epoch verifies the
actual signature and every signed Record. Output is exactly `epoch.bin`,
`material.bin` and `state-bundle.bin`, containing all inputs and one verified
proof for the explicitly selected Node, including a nonzero candidate index.
Original reference/current time checks, exclusive output and joined key/input
custody remain mandatory through acknowledgement. Historical Node declaration
references authenticate no current operation; their signed facts must still
cover the Epoch under its owning verifier. Receipts mark State acceptance,
installed qualification and runtime authority false. There is no Source serving
right, automatic successor, assignment redraw or output adoption.

Canonical Linux `ardents-control forwarding prepare-node-key FRESH_PRIVATE_OUTPUT`
creates one protected `node-key.pem` and prints only its public key. It creates
no NodeID, effective family, State authority or membership.
`forwarding prepare-node-record` requires exactly `--network-id`, `--node-id`,
`--node-key`, `--family`, `--endpoint`, `--valid-from`, `--valid-until`,
`--capacity` and `--output`. Network ID, NodeID and public key are separately
supplied lowercase 32-byte hex values. The endpoint is canonical literal-IP TCP with an unprivileged
port; validity is whole-second UTC RFC3339, at most seven days. Capacity is
1..1024. It creates `node-record-plan.json` and `node-record.signing-input` in
fresh protected output, without opening a private key or assigning a duty.

`forwarding sign-node-record PLAN PREPARED REVIEWED_SHA256 NODE_KEY FRESH_OUTPUT`
reconstructs the exact unsigned Record from that public plan and matches the
reviewed SHA-256 before admitting the original protected matching Node key.
It writes only `node-record.bin`. Then `forwarding prepare-node-certificate
PLAN SIGNED_RECORD REVIEWED_SHA256 NODE_KEY FRESH_OUTPUT` verifies the entire
signed Record and writes only `node-certificate.pem`. Each frozen preparation
reference must remain within five minutes past/one minute future and inside
the original validity interval through final acknowledgement. Outputs are
exclusive; existing or partial output is never adopted or overwritten. Public
receipts mark State acceptance, installed qualification and runtime authority
false. These are fixed offline Node purposes, with no Source/Next backend,
generic signer, exported private key or accepting State/process right.

On Linux, canonical `ardents-control forwarding prepare-descriptor
<release-identity> <positive-version> <program> <unit-template> <fresh-output>`
writes `infrastructure-forwarding.json`. `ardents-control forwarding
prepare-inventory <seven-file-directory> <fresh-output>` writes the separate
`infrastructure-enrollment.json`; the input directory contains exactly the Node
program, Forwarding template/descriptor and four Release metadata files. Both
operations require root-controlled original inputs/ancestors and create exclusive
protected output. Their public SHA-256 result confirms byte preparation only;
it grants no signature, first-pin provenance, installation or runtime right.
`forwarding prepare-hosting`, `forwarding prepare-state`,
`forwarding prepare-template`, `forwarding prepare-request` and
`forwarding prepare-plan` are retired (#604 FJD stage D, ADR-0125). Each returns
its typed before-effect refusal (`old Forwarding prepare-hosting is retired;
use the platform-led Forwarding lifecycle`, and the corresponding
prepare-state/prepare-template/prepare-request/prepare-plan forms) before
reading any input or attempting any effect. `ardents-next platform template`
emits the exact canonical platform unit template
(`infrastructure.PlatformUnitTemplate`); the platform-led Forwarding lifecycle
owns Hosting/State preparation and installed delivery. The generic network
authority preparation above (profile/token-keys/epoch/node-*) and the
descriptor/inventory preparation remain available.

The separate Linux Forwarding Release producers are:

```text
ardents-control forwarding prepare-release-keys FRESH_PRIVATE_OUTPUT
ardents-control forwarding prepare-release-plan --network NETWORK --reference RFC3339 --expires RFC3339 --program NODE --descriptor DESCRIPTOR --template TEMPLATE --program-custom PUBLIC_CLAIMS --generation-custom PUBLIC_CLAIMS --output FRESH_PLAN_OUTPUT
ardents-control forwarding sign-initial-release PUBLIC_PLAN PRIVATE_KEYS FRESH_METADATA FRESH_VERIFICATION
ardents-control forwarding prepare-forward-release-plan --network NETWORK --reference RFC3339 --expires RFC3339 --program NODE --descriptor DESCRIPTOR --template TEMPLATE --program-custom PUBLIC_CLAIMS --generation-custom PUBLIC_CLAIMS --output FRESH_PLAN_OUTPUT
ardents-control forwarding sign-forward-release PUBLIC_PLAN PRIVATE_KEYS FRESH_METADATA FRESH_VERIFICATION PREDECESSOR_DISTRIBUTION
```

Before initial signing, use the canonical `prepare-release-plan` producer above
with reviewed public claims and fresh protected keys. The former
[controlled Forwarding build and public-plan tools](../operations/qualification-release.md#forwarding-infrastructure-artifact-component)
(`prepare-forwarding-release-plan.py` and its build-records native control)
retired with the transitional Forwarding orchestration (#604 FJD stage D,
ADR-0125); a new-path Forwarding candidate build and release plan builder is a
separate follow-up, exactly as for Source (#604 stage 1.6). Build records and
signatures do not grant independent builders, a participant first pin or the
separate bootstrap acceptance.

Key preparation creates five distinct Ed25519 PKCS8 files, `release-key-1.pem`
through `release-key-5.pem`, under exclusive root-owned 0700 native ext4 custody;
the files are 0600. Its receipt contains only public keys and commitments. Plan
preparation retains the exact public artifact and reviewed claim files and writes
`forwarding-release-plan.json` with schema
`ardents-forwarding-initial-release-plan-v1`. It requires all nine flags exactly
once, version one, the exact canonical platform unit template
(`infrastructure.PlatformUnitTemplate`) and an actual static Linux-amd64
`ardents-next` executable (the #604 FJD stage D signing cutover). It invents no
build or qualification claim. The
public claims must describe the actual artifacts and evidence; component fixture
claims are not operational qualification or independent builds.

Signing retains that plan, the original five-key directory and protected inputs,
checks current time, signs the existing Release root/targets/snapshot/timestamp
metadata with threshold three of five and exactly the Node and Forwarding
descriptor targets, then obtains two coherent authorizations from Release in a
separate fresh verification history before publishing metadata. Every input,
secret and output user joins before success. Partial output remains retained
and cannot be adopted as a fresh retry. Reference time must be within five
minutes behind or one minute ahead of current UTC; expiry is later and at most
seven days after reference. Neither public receipt nor self-verification supplies
the independently delivered initial pin, installation or bootstrap acceptance.
Source, Endpoint and forward-replacement signing plans are refused by this
initial-only purpose.

The separate forward purpose accepts schema
`ardents-forwarding-forward-release-plan-v1` and a descriptor version strictly
greater than the exact complete eight-file predecessor distribution. The same
five private keys must match its original three-of-five Root roles; Root bytes
remain exact, preserving the independently delivered initial pin. Before any
forward signature, a fresh diagnostic Release history verifies the predecessor's
actual coherent Node/Forwarding pair. New Targets, Snapshot and Timestamp versions
each advance by one with matching version/hash references, then the new pair
advances that same diagnostic history. Protected predecessor files and namespaces
remain retained through output. Missing/changed/stale/mixed predecessor authority,
different keys, exhausted metadata versions or non-forward descriptors refuse.
No timestamp-only purpose, Root rotation or Source target enters this operation.
The canonical `prepare-forward-release-plan` producer emits this separate plan
from reviewed public claims and a forward descriptor, without JSON editing; the
former public build-plan script's `--forward` switch retired with the
transitional Forwarding orchestration (#604 FJD stage D, ADR-0125). Its
signatures and diagnostic floors remain separate from the platform-led
lifecycle's actual retained trust, forward program change and bootstrap
acceptance. Existing partial output or diagnostic history is never reused.

The Linux-amd64 component commands below implement the Source artifact boundary
under accepted ADR-0123 through direct new owners in `ardents-next`:

- `infrastructure prepare-descriptor <release-identity> <positive-version> <program> <unit-template> <fresh-output>` writes `infrastructure-source.json`.
- `infrastructure prepare-inventory <seven-file-directory> <fresh-output>` writes `infrastructure-enrollment.json`, after actual signed metadata are supplied.
- `infrastructure authenticate-initial <distribution> <independent-inventory-pin> <new-release-history> <environment> <network> <reference-time>` checks its actual executing artifact and two fresh Release proofs.
- `infrastructure authenticate-candidate <distribution> <retained-release-history> <environment> <network> <reference-time>` requires complete retained floors and supplies no initial-pin provenance.
- `infrastructure preflight-initial`, `infrastructure preflight-candidate` and `infrastructure install-initial-stopped` are retired (#604 stage 1.6, ADR-0125). Each returns the refused receipt `source-orchestration-retired` before reading any input or attempting any effect. The [platform-led Source lifecycle](../operations/source-platform-lifecycle.md) replaces the transitional Source orchestration.

All paths are canonical absolute. Preparation retains root-controlled original
files and private native-ext4 fresh output custody. Authentication refuses
overlapping distribution/history paths and symlink history ancestors before
effects, and uses the exact separate
seven-file inventory plus its inventory document; metadata retain their 1 MiB
per-file bound, descriptor/inventory 16 KiB, other artifacts 64 MiB. Reference
time is explicit RFC3339. All other invocations/platforms refuse. Once Release
floors exist, initial admission refuses; candidate authentication is the retained
route. Public output reports artifact checksums and the two verification outcomes
after joined Release Close, never serialized authorization or startup permission.
See the [operation](../operations/qualification-release.md#source-infrastructure-artifact-component).
None of the retained commands starts Source, provides a metadata signer,
replaces independent first-distribution trust or performs installation effects;
the [platform-led Source lifecycle](../operations/source-platform-lifecycle.md)
owns installation, start and program change.

The former preflight variants' read-only native observation and the
`ardents-source-preflight-v1` request grammar are retired with the transitional
Source orchestration (#604 stage 1.6, ADR-0125). The platform unit's
`platform check` ExecStartPre gate and `source serve-retained` replace the
transitional preflight/installation admission path; see the
[operator procedure](../operations/source-platform-lifecycle.md).

`ardents-next source repair-retained <source.json>` is the Linux operator's
explicit repair for a dead platform Source's sole retained serving producer.
Run outside the service as the fixed `ardents-source` account, using its original
root-controlled public plan, retained roots and loaded stopped/failed fixed unit.
The command loads no private transport key. It requires authenticated consistent
State/floors, both retained leases, the actual original manager and empty kernel
cgroup; other obligations and partial histories refuse without pre-commit writes.
It emits `repaired` only after actual cleanup. This abandons a dead invocation,
not a successful SourceCompletion; relaunch independently repeats the mandatory
platform check and producer Claim. See the [operator procedure](../operations/source-platform-lifecycle.md).

`ardents-next source prepare-trust <public-plan> <fresh-output>` performs one
30-second offline public observation and prepares exactly two Sources and one
client. All paths are canonical absolute. Its closed schema is
`ardents-source-trust-preparation-v1`; see the
[complete input/output and custody contract](../operations/qualification-release.md#source-public-trust-preparation).
It reads supplied public chains/roots against independently reviewed SHA-256
commitments and Source leaf pins, never the three declared private-key paths.
Outputs are existing `source serve` plans, `ardents-source-plan-v1` acquisition
declarations and the supplied `ardents-headless-runtime-v2` declaration, with
directly usable public paths. A public receipt lists only artifact basenames,
hashes and `offline-public-observation`. Local configuration files contain paths
and remain local. Linux root/native ext4 is required; other platforms refuse
before effects. This supplies no CA issuance, State acceptance, Installation
startup, operational trust or independent-family evidence.

`ardents-next network prepare-initial-keys <public-plan> <fresh-private-directory>`
creates one State and 1..64 distinct Node Ed25519 keys in a fresh protected
Linux-root/native-ext4 directory. The closed public plan has schema
`ardents-initial-keys-plan-v1`, operation `prepare-initial-keys` and integral
`node_count`. Files use canonical PKCS8 and compatible `state-key.pem` /
`node-NN-key.pem` basenames; the finite public receipt contains only purposes,
indices, basenames and public key commitments. No NodeID, Network identity,
signature, duty, State acceptance or independent custody is supplied. Existing
output refuses, including after failed receipt delivery. See the
[protected operation](../operations/qualification-release.md#initial-state-and-node-keys)
for input, secret lifetime, native prerequisites and retained-failure rules.


`ardents-next network sign-initial-node-record <public-plan> <prepared-artifact>
<reviewed-sha256> <private-key-path> <fresh-output>` and the distinct
`network sign-initial-epoch` and `network sign-closed-profile` forms perform one bounded offline signing
operation. All paths are canonical absolute. The strict signing schemas,
embedded preparation declarations, original protected-file/time requirements
and acceptance boundary are defined in
[qualification release](../operations/qualification-release.md#assembly-and-acceptance-boundary).
Record emits `node-record.bin`; Epoch emits `epoch.bin`, Source-ordered
`input-NN.bin` and accepted-View-ordered `material-NN.bin`. A successful public
receipt reports verified signatures and leaves State acceptance and installed
qualification false. Profile emits only `profile.bin`; its strict declaration
additionally supplies the independent `state_authority_key` pin, distinct from
the Admission authority inside the profile. It signs the public owner's fixed
ARDCPR03 transcript within the original maximum six-hour validity and finite
reference bounds. No private bytes enter arguments or receipts. These
operations require Linux root/native ext4 and refuse other platforms.

The nearest closed release uses TCP/TLS alone under
[ADR-0122](../adr/0122-select-tcp-only-closed-release.md). Recognized QUIC
profiles remain distinct historical signed inputs and receive an explicit
unsupported outcome before transport or Admission effects. There is no enabling
switch or TCP fallback. Current positive runs select TCP/TLS; retained dual-Carrier
receipts describe their original artifacts and do not qualify this release.

Status: **current maintained command routes.** These commands exercise the
implemented closed-test-network Modules. They are not a public operator API,
installer, supported Node hosting profile, or compatibility promise for old
plans and results, except for explicitly documented bounded migration adapters.

[ADR-0117](../adr/0117-reset-closed-service-formats-to-v3.md) selects the
current Service v3 identities and fresh-root startup. A mixed set of old and
new Service artifacts is not a supported workflow.

Every command fails closed on malformed, unavailable, or unqualified input.
Its bounded input belongs to the owning Module; a plan is not an ambient
configuration format or an authority source.

## `ardents`

### Public Network preparation in the isolated command

The separate Linux-root offline certificate operation is:

```text
ardents-next network prepare-node-certificate <absolute-public-plan> <absolute-signed-record> <reviewed-record-sha256> <absolute-original-node-key> <fresh-output-directory>
```

Its closed schema `ardents-node-certificate-plan-v1` has exactly `schema`,
`reference_time` and `record` (the complete public initial Record declaration
below). The independently reviewed lowercase SHA-256 covers the entire signed
Record. One matching original protected Node key produces only public
`node-certificate.pem`; stdout returns `ardents-network-signed-v1` with operation,
certificate PEM SHA-256, verified signature and false State/installed claims.
The self-signed leaf has no subject identity or CA authority, and its interval
equals the verified Record. Exact native custody, finite reference, failure and
completion requirements are in the [operation owner](../operations/qualification-release.md#local-node-certificate).
It starts no runtime and grants no membership, duty or ready state.

On Linux with native ext4 output, use:

```text
ardents-next network prepare-initial-node-record <absolute-plan> <fresh-output-directory>
ardents-next network prepare-initial-epoch <absolute-plan> <fresh-output-directory>
ardents-next network prepare-closed-profile <absolute-plan> <fresh-output-directory>
```

The plan is one public JSON object, at most 8 MiB. Names are exact lowercase
snake case, identifiers/public Ed25519 keys are 64 lowercase hex characters,
signed Record and SPKI bytes are lowercase hex, numbers are unsigned integers,
and times are whole-second canonical UTC RFC3339 (`...Z`). Duplicates, unknown,
missing, null, trailing and mixed-operation declarations refuse before output.
No private key, State root or signer field is accepted.

| Operation / schema | Required fields after `schema` | Artifact |
|---|---|---|
| `prepare-initial-node-record` / `ardents-initial-node-record-plan-v1` | `network_id`, `node_id`, `valid_from`, `valid_until`, `family`, `endpoint`, `carrier`, `capacity`, `public_key` | `node-record.signing-input` |
| `prepare-initial-epoch` / `ardents-initial-epoch-plan-v1` | `network_id`, `assignment_seed`, `valid_from`, `valid_until`, `domains`, `records`, `authority_keys` | `epoch.unsigned` |
| `prepare-closed-profile` / `ardents-closed-profile-plan-v1` | `network_id`, `state_generation`, `epoch_digest`, `issuer_node_id`, `issuance_authority_key`, `epoch`, `not_before`, `not_after`, `nodes`, `token_keys` | `profile.unsigned` |

Record generation is fixed to one. Carrier must be
`ardents-carrier-tcp-tls-v2`; Epoch preparation also refuses every signed Record
with deferred QUIC or retired Carrier. `domains` is the canonical ordered Role
Domain list (1–16); `records` contains 1–64 genuinely signed initial Node
Records; `authority_keys` contains 1–16 distinct public State pins. The supplied
seed is used exactly once, without searching for an assignment. Each profile
Node object has exactly `node_id`, `record_digest`, `role_domain`, `subrole`,
`duty_generation`; each token-key object exactly `window_start`, `class`, `spki`.
Existing Network owners enforce profile ordering, finite intervals and public
SPKI grammar. Profile preparation does not join those declarations to live State.

The output parent must already exist, be owned by the caller with mode 0700,
and reside on actual native ext4. Every path component must be direct, without
symlinks. The command exclusively creates a fresh directory and one 0600 file,
syncs/closes and verifies exact bytes and original custody before a receipt.
An interrupted/uncertain prefix remains incomplete and retained; choose a new
explicit directory for any separately authorized attempt. Existing output is
never overwritten, adopted, retried or automatically removed. Unsupported
platform/filesystem, invalid input, cancellation, physical failure or receipt
failure returns nonzero. A retained artifact alone is not a success receipt.

The public `ardents-network-prepared-v1` receipt contains operation, artifact
name, exact unsigned SHA-256, `signed:false`, `state_accepted:false` and
`validation:public-grammar-only`. Preparation cannot sign, generate keys, accept
State, grant a duty, start runtime work or complete provisioning. Later private
Node Record, Epoch and profile signing remain purpose-specific separate rights.

The isolated `ardents-next execution prepare-permission --config <file>`
operation consumes a local Grant, executes empty INIT through a verified fixed
installed reader or Publisher worker and joins that original worker before
opening signed Network and holder Stock. Its configuration contains `generation`,
`principal` and a `holder` object with the existing holder plan. A signed `network`
is mandatory; asserted profile files refuse. The finite owned console retains
the holder key while its request/import and issuer-bootstrap operations
execute, checking the exact completed-current preparation before authority
effects and response handoffs. This is permission preparation, not installed
Endpoint startup, Service readiness, private Publication/Connection or a
qualification receipt. The selected active fixed Endpoint and genuine artifact
are prerequisites; missing prerequisites refuse before holder/root effects.

`ardents-next execution route-holder --config <file>` uses the same configuration
with a mandatory `holder.route`. It retains one live qualified worker operation
for genuine protected Route setup, issuer Control and Descriptor lookup. Local
permission checks surround signed currentness, effects and response handoffs.
Route/Stock/Network resources join before the operation and original worker;
physical cleanup failure closes local generation admission. Private Descriptor
publication, registration and JOIN/Service operations refuse. This bounded
consumer establishes neither Service readiness nor installed startup acceptance.

### Protected installation commands — qualification incomplete

The isolated new consumer `ardents-next installation upgrade-installed --request
<file>` retains one original replacement lifetime and uses Candidate plus complete
retained Release history, with two fresh proofs. It joins its original predecessor
before replacement and attempts guarded fixed-unit start. A full ACK followed by
failure retains `installed-started-recovery-required` and a nonzero exit status.
The accepting new installed runtime and complete interrupted successor recovery are absent;
no successful installed start, Service readiness or qualification follows from
the command's presence. Physical cleanup may continue beyond its two-minute
replacement bound; a timeout is not joined completion.

`ardents-next installation recover-installed <installation-root>
<canonical-UTC-reference-time>` recovers terminal completion of
the still-running exact successor. That path requires the archived intent, complete
original removal/replacement provenance, independent process/manager observations
and two fresh floor-compatible proofs. It finishes original guard removal and
archives the retained first transition failure without another ACK, Start or
Stop. A separate pending path admits a complete `generation-staged` prefix
before fixed replacement: exact previous selection/resources, sealed candidate,
original directory birth and a closed journal precede fresh proofs and resync.
It then performs original predecessor join, replacement/selection/reload and
guarded start. Other prefixes, including existing transition failures, still
return `installation-repair-required`. Full ACK survives late errors with
`installed-started-recovery-required` and nonzero exit.
Successful command composition and filesystem controls do not establish a
positive installed recovery; the genuine new runtime consumer remains required.

The duplicate `ardents endpoint provision`, `installation-check`,
`start-installed`, `upgrade-installed` and `recover-installed` commands are retired
before input or root effects. Their implementation has been removed; existing
roots and data remain. The retained new native commands belong to the transitional
`ardents-next` implementation under ADR-0125.

Before first Endpoint runtime, the actual runtime principal runs
`ardents-next endpoint prepare-history HEADLESS_PLAN BUNDLE SIGNED_PROFILE`.
The bounded v2 Headless declaration supplies only trust, clock and storage
targets here; it is not accepting runtime-plan validation. `BUNDLE` uses the
existing Source bundle framing and `SIGNED_PROFILE` the signed closed profile.
Existing vacant 0700 State/participation/token/Entry directories are required. Fresh
State authenticates the public inputs and retains its exclusive claim through
initialization of participation, empty Entry/role Interior histories and
Reader[/Publisher] Stock. No Route member is drawn. No permission,
worker, Source transport, Service or Startup is created. All owners join before
the fixed `history-preparation` diagnostic reports `prepared`; stdout is empty.
Partial/completed preparation refuses retry without cleanup or reset. Ordinary
`endpoint start-installed` requires retained State and role Stock, including
the role child directory. Shared Entry and each role Interior also open retained.
Missing histories cannot trigger initialization. Hosting is initialized separately
after this preparation; Service preparation remains separate. This command requires Linux;
other platforms refuse before reading the declaration.

Detached `ardents-next installation authenticate-initial` and
`authenticate-candidate`, including `--request` forms, are retired. The actual
provision/replacement/recovery operation obtains and retains its own proofs;
a printed authentication result grants no continuation.

`ardents` is the Endpoint and Network State command adapter.

The Portable per-user `endpoint user-unit`/`endpoint enroll` start route is
retired (#617 S3.5); platform delivery owns first execution. Use `ardents-next
platform unit` with `endpoint construct-plan` and `prepare-history`, then
`ardents-next endpoint start-installed` under the platform unit. The
[independently pinned Ubuntu verification procedure](portable-enrollment.md)
still applies before running a downloaded program.

| Route | Purpose |
|---|---|
| `accept-offline --state-root PATH --network-id HEX --authorities HEX,... --threshold N --at RFC3339 --epoch PATH --inputs PATH --materialization PATH --profile ardents-route-v3 --closed-profile-authority HEX [--closed-profile PATH]` | Accept one complete authenticated offline Network State generation through the successor State owner. The live State selects only the closed Route profile (`ardents-route-v3`) and requires its pinned closed-profile authority; retired-profile Epochs are refused, not published. When `--closed-profile` is supplied, the command accepts that signed profile **after** committing the Epoch; a profile refusal does not undo the accepted generation. Success emits one `ardents-state-event-v1` `generation-accepted` JSON event, with `closed_profile_sha256` when the profile was also accepted. |
| `accept-closed-profile --state-root PATH --network-id HEX --authorities HEX,... --threshold N --at RFC3339 --profile ardents-route-v3 --closed-profile-authority HEX --closed-profile PATH` | Submit one signed `ARDCPR03` only to an already accepted State root under its explicit pinned closed-profile signer. It neither accepts an Epoch nor contacts Source. An exact retry is harmless; a different valid digest records durable conflict and makes the closed profile unavailable. It emits one `ardents-state-event-v1` `closed-profile-accepted` JSON event containing only the State generation, Epoch and profile digest. |
| `refresh-sources --state-root PATH --source-plan PATH [--once|--resume]` | Return `refresh-sources is retired; the platform-led runtime owns automatic Source refresh` at command dispatch before reading the plan, opening the State root, or causing network work. The legacy source-plan decoder, credential loader and resource-event output are removed; the successor State owner never samples process resources and the platform-led runtime owns automatic Source refresh, so the standalone command has no successor translation (#493). |
| `diagnostics timeline` | Read app JSON lines or journalctl JSON from standard input and stream a local timeline of Node, Source, and Endpoint events. It reads no authority root, contacts no peer, and saves no history. |
| `service-instance initialize --config PATH` | Create or reopen one host-owned Service Instance generation from an `ardents-service-instance-initialize-v1` plan whose `request_file` names one new public output, and emit its stable public request plus `request_sha256` for the independently transferred custody ceremony. The command exposes neither private key, Service Authority, Credential, Target, Route, nor Browser state. |
| `service-instance accept --root PATH --response PATH` | Atomically accept only the exact canonical public Authority response for that pending root. An exact repeat is harmless; malformed or different input terminally rejects/conflicts rather than replacing the generation. |
| `endpoint headless <headless-runtime.json>` | Return `endpoint headless is retired; use ardents-next endpoint start-installed under the platform unit` at command dispatch before reading the plan or opening any runtime resource. The protected text participant now starts through `ardents-next endpoint start-installed` under the platform unit; the former v1/v2 headless composition is removed. The exact v1 refusal and retained-data boundary remain defined by the [Endpoint startup retirement contract](../technical/endpoint-service-runtime.md#v1-startup-retirement). |
| `endpoint open <application-socket> <target-link> <input-file> <output-file>` | Return `endpoint open is retired` before opening either file, dialing the local socket, or causing Endpoint/Network work. The AAI2 codec/server/client and exclusive Endpoint adapter are absent; no generic AAI3 translation or fallback is selected. |
| `endpoint publish <administration-socket>` | Return `endpoint publish is retired; use ardents-text publish <administration-socket> <document-file>` before dialing the Administration socket or starting Endpoint/Network work. Use `ardents-text publish /absolute/path/to/administration.sock /absolute/path/to/document` for snapshot publication. |
| `endpoint withdraw <administration-socket>` | Request withdrawal through the exact local one-use Service Administration capability and render its bounded receipt. A publisher plan must explicitly retain its administration listener after publication for this route. |
| `endpoint enrollment-check <bundle-root> <manifest-sha256>` | Diagnose one already-running artifact against an independently pinned closed-alpha inventory through the successor enrollment verifier (ADR-0127). It renders only authenticated initial facts and does not authenticate first execution. |
| `endpoint enroll <bundle-root> <manifest-sha256>` | Return `endpoint enroll is retired; use ardents-next platform delivery with endpoint construct-plan and prepare-history` at command dispatch before reading the bundle or causing any effect (#617 S3.5). |
| `endpoint enroll-installed <package-enrollment.json>` | Return `endpoint enroll-installed is retired; use ardents-next platform delivery with endpoint construct-plan and prepare-history` at command dispatch before reading the enrollment or causing any effect (#617 S3.5). |
| `endpoint user-unit <bundle-root> <manifest-sha256>` / `endpoint installed-user-unit <package-enrollment.json>` | Return `endpoint user-unit is retired; use ardents-next platform unit` (respectively `endpoint installed-user-unit is retired; use ardents-next platform unit`) at command dispatch before rendering any unit. The per-user `systemd --user` profile is retired; platform delivery owns the system unit (#617 S3.5). |
| `endpoint replace <replacement-bundle>` | Return `endpoint replace is retired; use ardents-next installation upgrade-installed` at command dispatch before reading the bundle or causing any effect. The successor Endpoint lifecycle is forward-only (#617 S3.5). |
| `endpoint replacement-recovery` | Return `endpoint replacement-recovery is retired; use ardents-next installation recover-installed` at command dispatch before reading any replacement state (#617 S3.5). |
| `<journal-bound-recovery-program> endpoint rollback <replacement-bundle>` | Return `endpoint rollback is retired; the successor Endpoint lifecycle is forward-only` at command dispatch before reading the bundle or causing any effect (#617 S3.5). |
| `entry recipient <entry-import-plan.json>` | Return `entry Invite command is retired; the closed Entry set has no operator import surface` at command dispatch before interpreting the remaining arguments, reading the plan file, or creating any root. It never prints a recipient key; no Invite root reader, converter, or deleter remains (ADR-0106). |
| `entry import <entry-import-plan.json>` | Return the same retirement refusal at command dispatch before any argument interpretation or filesystem effect. Existing Invite roots stay on disk byte-for-byte; the closed Entry set root refuses them by marker and allowed-name inspection (ADR-0106). |
| `name encode <name>` | Return `name network command is retired; protected Service Name access is not selected` at command dispatch before interpreting the argument; ADR-0113 retired the local canonical encoder with its package. |
| `name resolve <input-file> <name> <context-hex>` | Return `name network command is retired; protected Service Name access is not selected` at command dispatch before validating the remaining arguments, reading input, opening State, constructing or using HTTP/OHTTP transport, writing output, or changing Namespace state. |
| `name control <input-file> <operation-file> <context-hex>` | Return the same retirement refusal at command dispatch before validating the remaining arguments or reading the operation; no control operation, Namespace mutation, migration, or fallback is selected. |

The diagnostic timeline accepts the bounded runtime schemas from Node,
Source, and the headless Endpoint. For a live local view, pipe an authorized
journal stream into it, for example:

    journalctl -f -o json | ardents diagnostics timeline

Each tab-separated row is occurrence time (UTC), clock origin (event or journal),
owner, journal process ID, role, Carrier, kind, state, and quoted bounded reason. It preserves input order.
Old Source events without an occurrence time use the journal receipt time when
available. Unknown schemas and unrelated journal messages are ignored; corrupt
input records and malformed recognized categories fail without printing their
raw content. Present projected string fields must be JSON strings; null and other
types fail, while absent optional fields remain unavailable. The projection
omits Network IDs, destinations, addresses, permission commitments, tokens,
document bytes, and all other fields. It is a local navigation view of existing
events, not an authority source, telemetry export, or complete Route trace.
An Endpoint fatal return after event output is acquired adds
`headless-runtime-failed` with a `startup` or
`running` category. The local command error remains on stderr; the timeline
does not include its wrapped details.

`endpoint headless` is retired at command dispatch before opening any
plan-owned runtime resource; the protected text participant starts through
`ardents-next endpoint start-installed` under the platform unit. The exact v1
refusal and retained-data boundary remain defined by the
[Endpoint startup retirement contract](../technical/endpoint-service-runtime.md#v1-startup-retirement).
The former v1/v2 participant process composition has been removed; refusal does
not retain a hidden fallback or migration runtime.
Generic `endpoint open` is separately selected for
[effect-free retirement](../technical/endpoint-service-runtime.md#generic-connection-command-retirement).
Service Administration remains selected and is not carried by the removed AAI2
or by the protected text AAI3 Connection.
The architecture inventory independently guards this exact retirement through
the [bounded closure receipt](../development/testing.md#reachability-audit),
whose exact scope is owned by Testing.

The old Name operator network adapters have their own selected
[retirement boundary](../product/scope.md).
ADR-0113 subsequently retired `name encode` itself and deleted the canonical
Naming wire grammar with its package; the whole `name` verb family now
refuses, and protected Service Name access is still not delivered. The
command dispatch enforces the refusal, and the command-owned HTTP/OHTTP
plan, State-view, receipt, operation, and encoder adapters are absent. ADR-0105 subsequently removed the whole
Namespace subsystem and its never-command-exposed custody operations; an old
Namespace root stays on disk untouched with no read path at all.

The current State and source event schemas are coordinated C0 command outputs:
there is no H3 reader or compatibility window. Resource observations are
Module diagnostics, not a capacity or hosting claim.

The three-argument v1 form `endpoint enrollment-check <alpha-enrollment.json>`
is retired (ADR-0127, #647): it returns `endpoint enrollment-check
<alpha-enrollment.json> is retired; use endpoint enrollment-check <bundle-root>
<manifest-sha256>` at command dispatch before reading the document or touching
the bundle. The v1-document parser is removed; existing persisted documents
stay on disk byte-for-byte with no reader or converter. The three-argument
`endpoint enroll` and `endpoint user-unit` forms are retired at command
dispatch like their two-argument forms (#617 S3.5); no per-user restart or
migration-unit render path remains. New units use the independently delivered
manifest pin argument through platform delivery.

## `ardents-node`

The transitional Forwarding orchestration is retired (#604 FJD stage D,
ADR-0125). `ardents-node forwarding start-installed BINDING` returns
`old Forwarding start-installed orchestration is retired; use the platform-led
Forwarding lifecycle (ardents-next node forwarding --retained-config, platform
check/unit, systemctl start ardents-forwarding)` before reading the binding or
attempting any effect. Every other `ardents-node forwarding ...` form
(`inspect-inputs`, `install-initial-stopped`, `start-initial`, `restart`,
`start-selected`, `stop`, `replace`, `recover`, `inspect-initial-history`,
`inspect-start-history`, `supervise-transition`) returns
`old Forwarding installation transactions are retired; use the platform-led
Forwarding lifecycle (ardents-next platform check, node forwarding
--retained-config, forwarding repair-retained)` before reading inputs or
attempting any effect. The former `installation/forwarding` package, its fixed
`forwarding start-installed` unit and the OLD-path installed QA profile are
removed. The platform-led Forwarding lifecycle owns installed delivery: the
canonical platform unit with the mandatory `platform check` ExecStartPre gate,
`ardents-next node forwarding --retained-config` and
`ardents-next forwarding repair-retained` for a dead retained producer. State
and participation histories are reopened by their own semantic owners, never
reset. A new candidate and the explicit relaunch/program-change scope require
their own exact bootstrap receipt and direct Product Owner acceptance before
any installed execution. The historical transaction contracts remain superseded
provenance in the [Node owner](../technical/network-route-node.md#proposed-lifecycle-and-effect-admission).

The isolated development consumer `ardents-next node forwarding --config <file>`
starts one original authenticated Forwarding duty on Linux. Its closed input
contains `network`, `node_id`, `process_profile`, `participation_root`,
`spend_root`, `hosting_root`, `certificate`, `private_key`, `work`, `termination`
and `retirement_wait_ms` (1..5000). It requires the sole
`ardents-forwarding-process-v1` native envelope described by the
[Node/resource owner](../technical/network-route-node.md#forwarding-development-process-profile).
Roots are canonical, independent and new-domain owned; certificates must match
the exact signed member. Work must cover the receiving class allowance through
Hosting before spend. This development consumer supplies no installed restart
or cutover acceptance; the separately selected installed contract owns those gates.

`ardents-next node issuer --config <file>` composes one original closed Issuer
duty under mandatory `ardents-issuer-process-v1` on Linux. It accepts the same
Network, credential, root and bounded retirement-wait inputs, plus
`bootstrap_work` and `issuance`. The nested `issuance` object uses the exact
`issuance initialize-results` prepared-root/binding declaration; startup opens
these quota/key/result roots without initialization or key generation.
All six local roots and State are canonical, distinct and non-nested. The
credential and provisioned signer must match the authenticated Role2/subrole6
member. Ordinary `work` must cover full Control64KiB, `bootstrap_work` must
cover its actual128KiB lane, with separate termination capacity. No refill or
bootstrap-to-ordinary relabel exists. Original key/journal custody remains
until Route and issuing borrowers join. Bounded `node.issuer` output uses the
same fixed phase/outcome/owner fields. A retirement wait timeout reports
completion-unavailable and retains the process and its original owners until
physical join; the bounded failed result cannot later become successful EXIT.

`ardents-next node descriptor --config <file>` composes one original
Role2/subrole5/PurposeReachability3 duty on Linux under mandatory
`ardents-descriptor-process-v1`. Its closed fields are `network`, `node_id`,
`process_profile`, `participation_root`, `spend_root`, `hosting_root`,
`descriptor_root`, `certificate`, `private_key`, `work`, `termination` and
`retirement_wait_ms` (1..5000). All five roots are canonical, distinct and
non-nested; TLS material must match the exact authenticated member. Ordinary
work must cover full Control64KiB before durable Receiving spend. No bootstrap,
Forward refill, role choice, root adoption or automatic replacement is supplied.
The genuine independent Store owns verification, conflict floors and durable
ACK; this command grants no Publisher readiness or Service Connection. Its
fixed `node.descriptor` projections retain original failures and physically
joined completion. State closes after Node; a bounded retirement report cannot
release original roots, producer, native pins or TLS material ahead of join.

`ardents-next node introduction --config <file>` composes one original
Role4/subrole3 duty under mandatory `ardents-introduction-process-v1`. The
closed fields match Descriptor with `introduction_root` replacing
`descriptor_root`. The independent private Route History is prepared before
listener; only the genuine fresh Receiving birth fact can initialize vacancy.
Retained history strictly reopens without reset/adoption. Purpose4 requires
full Registration1MiB coverage and original600s bounds; Purpose5 requires
full Control64KiB coverage and original30s bounds, further limited by the
original capsule deadline and original slot. The capsule ceiling is thirty
seconds under [ADR-0121](../adr/0121-bound-installed-connection-setup.md);
earlier original bounds still apply. Spend is irreversible. Node
owns original caller/process/producer/TLS and joined retirement; Route owns
slot, opaque delivery and floors. The fixed `node.introduction` projections
supply no Publication readiness, qualified recipient or Service acceptance.
State closes last; completion-unavailable keeps original resources until join.

`ardents-next node join --config <file>` composes one original Role2/subrole4
Rendezvous DataJoin under mandatory `ardents-join-process-v1`. Its exact closed
fields are `network`, `node_id`, `process_profile`, `participation_root`,
`spend_root`, `hosting_root`, `certificate`, `private_key`, `work`, `termination`
and `retirement_wait_ms` (1..5000). Network/participation/spend/Hosting roots
are independent, canonical and unnested. The credential is fixed to the accepted
Node key. Each side requires full class-2 shared work coverage and finite cleanup
coverage before irreversible spend; mismatches never refund. Existing Route
owns both RESULTs, duplex credit/EOF/CLOSE and original thirty-second-or-earlier
unpaired bound. There is no JOIN refill/renewal or automatic restart.
`node.join` reports only fixed listening/refusal/retiring/joined outcomes and
owner categories. Unsupported platforms refuse before reading config. State,
producer, credential and roots stay retained until real physical join; a timeout
report grants no completion, Service authentication or installed qualification.

The command emits bounded `node.forwarding` listening/refusal and joined/retiring
outcomes. Joined cancellation exits 130, failure or unavailable completion exits
1, and malformed input/output failure exits 2. A retirement timeout is not
physical completion. Unsupported platforms refuse before reading configuration.
Joined failures may include fixed `first_owner` and `cleanup_owner` categories;
they expose neither raw errors nor paths. Refused exchanges can retain an actual
Route cleanup failure, which remains failed despite known physical join.
`ardents-next node inspect-process` projects actual process observations or
unavailable, with queue/timer counts explicitly unmeasured. It grants no duty,
capacity, readiness, supported-host or privacy qualification.

`ardents-node hosting initialize --config PATH` accepts a canonical
`ardents-hosting-initialization-v1` plan and initializes the host-owned Resource
policy root for one provider period. Later Node plans reopen that root; they
cannot replace its policy.

The accepted
[old-start retirement contract](../technical/network-route-node.md#old-start-retirement)
selects refusal of the old Node reservations, Source
`native_rendezvous_profile`, and Transit issuer initialize/serve before their
first root, key, listener, supervisor, or Network effect. It preserves the
separately named closed duties, closed Source profile, and closed issuer.
The Node-reservation, Source-profile, and Transit issuer start command gates
are integrated. The retired dedicated-host Contributor subcommand is removed
entirely (ADR-0114): `contributor` is no longer a recognized command shape and
fails with the standard usage error before any effect.

`ardents-node issuer initialize --config PATH` recognizes the bounded legacy
`ardents-transit-issuer-initialize-v1` schema only to return
`old Transit issuer start is retired`. That refusal follows bounded plan decode
and precedes path validation, identity-key access, root creation and output.
It does not convert, erase or reopen an existing Transit issuer root.

`ardents-node issuer serve --config PATH` rejects a legacy `transit_issuer`
reservation through the old-duty gate before plan-owned keys, certificates,
State roots, issuer roots or listeners are opened. Its issuer validator accepts
only one isolated `closed_issuer` runtime. The old Node-side Transit signer,
listener, Handler, and mutable root ledger are absent. Existing root bytes are
left untouched; no command reopens or migrates them. The signed-profile decoder
and one-use client remain solely for the separate Endpoint acquisition path.

The retained closed-profile issuer uses the `closed_issuer` stanza with
an initialized signing root and a separate, existing owner-only admission_root
for receiving Control-token spends, plus connection_limit and drain_timeout_ms.
Both paths must be clean absolute paths and cannot identify the same root.
The Node owns both roots until its listeners have joined. The current State
profile still supplies the issuer identity, keys, recipients and validity;
these local storage paths confer no network authority. Missing admission
storage makes startup unavailable.
`ardents-node source --config PATH` is retired before reading the plan, opening
optional diagnostics, State, keys or listeners. It returns
`old Source profile is retired` for the recognized complete selector; no old
Source backend or automatic plan conversion remains.

`ardents-node source start-installed BINDING` is retired (#604 stage 1.6,
ADR-0125). It returns `old Source start-installed orchestration is retired; use
the platform-led Source lifecycle (ardents-control source prepare-state,
ardents-next platform initialize-trust/unit, systemctl start ardents-source)`
before any installation, State or native effect. The platform-led Source
lifecycle owns the installed Source start: `ardents-control source prepare-state`,
`ardents-next platform initialize-trust`/`unit`, then `systemctl start
ardents-source` with the mandatory `platform check` ExecStartPre gate and
`source serve-retained`. See the
[operator procedure](../operations/source-platform-lifecycle.md).

`ardents-control source prepare-state SERVE_PLAN BUNDLE` is a separate finite
offline producer under the actual Source system UID/GID, without supplementary
groups. It consumes the public serving declaration and bounded signed Epoch /
materialization bundle with original file/ancestor custody, and requires the
already created runtime-owned 0700 State and participation directories to be
empty. State's fresh-only opening exclusively creates the first lease and
refuses every complete or partial earlier attempt. New State
authenticates and durably accepts the bundle under its independently selected
pins and confirmed time, including the serving declaration's fresh independent
clock observation. After acceptance, `network/duty` initializes only an empty
participation history under its independently created original lease, granting
no Source exposure or live duty. The command joins both owners before releasing inputs and emits
only a bounded outcome. It opens no serving/acquisition transport, process Guard,
Installation Startup or runtime TLS key. Partial failure preserves the resulting
State/participation files; another invocation refuses the partial pair. It
supplies no reset, adoption, continuation or rollback operation.
It accepts no Route profile and proves no Node duty, independent-family supply,
installed runtime admission or bootstrap acceptance.

The distinct finite inspection form is:

```text
ardents-control source inspect-latest SERVE_PLAN CLIENT_CERT CLIENT_KEY SERVER_CA SERVER_NAME SERVER_KEY_PIN SOURCE_ID FAMILY HANDLE MATERIAL_INDEX
```

It contacts exactly the Source address in the public serving declaration, using
the independently supplied CA, hostname and lowercase hex server-key pin / Source
identity, one honest family and endpoint handle, and decimal material index 0–63.
The caller-owned client private key must be 0600; original plan, credential and
CA files remain retained through the joined mTLS exchange. It performs one LATEST
request, with no retry/fallback or fabricated second Source. The ordinary
two-family acquisition policy remains unchanged. The bounded local output carries
the response status, object digest and base64 payload, not accepted State or
independence evidence. Keep raw Source responses as private local evidence;
bootstrap/public receipts contain only their reviewed finite commitments and limits.

Canonical public Source artifact preparation uses:

```text
ardents-control source prepare-descriptor RELEASE_ID VERSION PROGRAM TEMPLATE FRESH_OUTPUT
ardents-control source prepare-inventory SEVEN_FILE_DISTRIBUTION FRESH_OUTPUT
```

These Linux root operations retain original protected native ext4 inputs and
write one public descriptor or inventory into a fresh 0700 directory under a
protected root-owned 0700 parent. Output files are exclusive 0600 originals,
synced and re-read through retained descriptors before bounded receipt output.
Existing/partial output is never adopted or overwritten. Inventory requires the
exact seven Source files and independently checks descriptor/resource agreement;
it authenticates no Release signature or initial pin. The ordinary Source signing
producer, exact compiled program and installed bootstrap remain separate gates.

The former public closed template producer is retired (#604 stage 1.6,
ADR-0125):

```text
ardents-control source prepare-template FRESH_OUTPUT
```

It returns `old Source prepare-template is retired; use ardents-next platform
template` before reading inputs or writing output. `ardents-next platform
template` emits the canonical platform unit template
(`infrastructure.PlatformUnitTemplate`); `ardents-next platform unit` renders
and the operator publishes the fixed `ardents-source.service` unit. See the
[operator procedure](../operations/source-platform-lifecycle.md).

The separate closed initial Source key birth and signing forms are:

```text
ardents-control source prepare-release-keys FRESH_PRIVATE_OUTPUT
ardents-control source sign-initial-release PUBLIC_PLAN PRIVATE_KEYS FRESH_METADATA FRESH_VERIFICATION
ardents-control source sign-forward-release PUBLIC_PLAN PRIVATE_KEYS FRESH_METADATA FRESH_VERIFICATION PREDECESSOR_DISTRIBUTION
ardents-control source refresh-release-timestamp PUBLIC_PLAN PRIVATE_KEYS FRESH_METADATA FRESH_VERIFICATION CURRENT_DISTRIBUTION
```

Key birth writes exactly five fresh distinct `release-key-1.pem` through
`release-key-5.pem`, under original protected root/native-ext4 custody. Its
public-only standard-output receipt lists file/public-key/digest facts and the
ordinary threshold three, with `independent_custody: false`. Retain this receipt
outside the private key directory so the signer's exact five-file inventory
stays intact. No key is imported from fixtures or another release ceremony;
this operation signs nothing and grants no trust or bootstrap acceptance.

Its exact public plan fields are `schema`, `network`, `reference_time`, `expires`,
`program`, `descriptor`, `template`, `program_custom` and `generation_custom`.
Schema is `ardents-source-initial-release-plan-v1`; this is not the Endpoint
signer's schema. The program must be a static Linux-amd64 Go `ardents-next`,
descriptor version one and template the exact canonical platform unit template
(`infrastructure.PlatformUnitTemplate`). The signing target cut over from
`ardents-node` and the old Source unit template with the transitional Source
orchestration retirement (#604 stage 1.6, ADR-0125). Both
custom values use the existing Release grammar and common build/policy facts;
their two actual rebuild records bind each respective target digest. Retain the
real execution receipts and honest qualification limits behind those declarations.
Reference must be current, metadata lifetime at most seven days. The initial
form refuses successor plans; neither form accepts an arbitrary target, generic
signing message or Endpoint translation.

The separate forward form requires schema
`ardents-source-forward-release-plan-v1`, a strictly higher descriptor version
than the complete eight-file predecessor distribution, the same five private
keys and unchanged predecessor Root. Timestamp, snapshot and Targets versions
advance by one with exact consistent-snapshot references. The original protected
predecessor inputs remain held through output. Before signing, Release
authenticates the predecessor and its coherent pair in the exclusively born
diagnostic history; the new pair then advances that same history. This verifier
is diagnostic only. Installation must independently verify the candidate against
its actual retained trust history and monotonic floors. No fresh initial pin,
diagnostic history or public signing receipt supplies successor permission.

The private directory contains exactly five distinct canonical PKCS#8 Ed25519
`release-key-1.pem` through `release-key-5.pem`, root-owned 0600 under protected
ancestors. Both fresh destinations have private root-owned 0700 parents on
native ext4. The command keeps original files/keys and namespaces through
three-signature metadata creation, actual new-Release verification of the coherent
pair under one fresh diagnostic history, and durable public output readback.
Diagnostic floors stay separate from participant history and distribution files.
It outputs exactly `root.json`, `timestamp.json`, `snapshot.json`, `targets.json`;
partial output/history is never adopted on another operation. The bounded receipt
claims no installed qualification, independent custody/builders or bootstrap
acceptance. The exact public Source bootstrap receipt still requires direct PO
acceptance before installed execution.

The fixed offline one-Record genesis chain is:

```text
ardents-control source prepare-initial-keys FRESH_PRIVATE_OUTPUT
ardents-control source prepare-initial-plan --network ID --node ID --node-key PUBLIC_KEY --state-key PUBLIC_KEY --family FAMILY --endpoint IP:PORT --valid-from UTC_TIME --valid-until UTC_TIME --assignment-seed SEED --domains DOMAIN[,DOMAIN...] --output FRESH_OUTPUT
ardents-control source prepare-initial-record PUBLIC_PLAN FRESH_OUTPUT
ardents-control source sign-initial-record PUBLIC_PLAN PREPARED_RECORD PREPARED_SHA256 NODE_PRIVATE_KEY FRESH_OUTPUT
ardents-control source prepare-initial-epoch PUBLIC_PLAN SIGNED_RECORD FRESH_OUTPUT
ardents-control source sign-initial-epoch PUBLIC_PLAN SIGNED_RECORD PREPARED_EPOCH PREPARED_SHA256 STATE_PRIVATE_KEY FRESH_OUTPUT
```

Key preparation creates exactly two distinct standard-library keys, named
`source-state-key.pem` and `source-node-key.pem`, with protected original native
output and secret-buffer retirement. Its receipt contains only public keys;
it creates no identity, family, assignment or accepted State. These keys are
separate from Release and external runtime/client TLS credentials.

The public flag preparer writes `source-initial-network.json` in a fresh protected
native output; its current reference time comes from the original command. It
creates no identity/key or random assignment seed: select those public facts
explicitly and preserve the produced plan through the separately reviewed signing
steps. No JSON editing is required. Comma-separated Role Domains keep their supplied
order; Network's later Epoch preparation validates them with the actual signed
Record and computes assignment. A prepared declaration is not an accepted Epoch.

Public plan schema `ardents-source-initial-network-plan-v1` has exactly `schema`,
`reference_time`, `network_id`, `node_id`, `node_public_key`, `state_public_key`,
`family`, `endpoint`, `valid_from`, `valid_until`, `assignment_seed` and `domains`.
Identifiers/pins/seed use exact lowercase 32-byte hex, times are current and within
the canonical bounded interval, and the endpoint is literal-IP TCP. The supplied
domains remain Network's ordered Role Domain declarations, never a sixth Source
role. The producer fixes generation/Epoch one, one signed Record with capacity
one, one honest family and selected TCP/TLS. Network's existing preparers/verifiers
own canonical bytes, signatures, eligibility and assignment. Both signers rederive
and match the original prepared artifact and its separately reviewed SHA-256 before
importing the matching protected private key. They accept no arbitrary message.

Epoch signing writes `epoch.bin`, `material-00.bin` and `source-bundle.bin` after
genuine verification of the signature, original Record and materialization. It
accepts no State or Route profile and creates no Node duty or independent-family
supply; the complete Route remains unavailable under one effective controller.
All operations require fresh output, original root-controlled native files and
actual time, with no prefix adoption, reset or automatic retry. Keep real
operator preparation separate from the native tests' generated fixture keys.

The former single-Source public input preparer is retired (#604 stage 1.6,
ADR-0125):

```text
ardents-control source prepare-install-inputs ...
```

It returns `old Source install-input preparation is retired; use the
platform-led Source lifecycle (ardents-control source prepare-state,
ardents-next platform initialize-trust)` before reading inputs or writing
output. The platform-led Source lifecycle replaces the transitional
install-input preparation; see the
[operator procedure](../operations/source-platform-lifecycle.md).

The former root installation transaction forms are retired (#604 stage 1.6,
ADR-0125):

```text
ardents-node installation install-initial-stopped ...
ardents-node installation start-source ...
ardents-node installation stop-source ...
ardents-node installation restart-source ...
ardents-node installation replace-source ...
ardents-node installation recover-source ...
```

Each returns `old Source installation transactions are retired; use the
platform-led Source lifecycle (ardents-next platform check, source
serve-retained, source repair-retained)` before reading inputs or attempting
any effect. The transitional install/start/stop/restart/replace/recover
orchestration, its journals and its native transaction profiles are removed.
The [platform-led Source lifecycle](../operations/source-platform-lifecycle.md)
owns Source delivery: `ardents-control source prepare-state`,
`ardents-next platform initialize-trust`/`unit`, `systemctl start
ardents-source` with the mandatory `platform check` ExecStartPre gate and
`source serve-retained`, systemctl stop/relaunch and forward program change,
and `ardents-next source repair-retained` for a dead retained producer.
Forward Release signing (`source sign-forward-release`) remains the separate
forward authorization purpose.

Before any isolated installed Source execution, ADR-0123 requires the completed
public exact-artifact bootstrap receipt and its separate direct Product Owner
acceptance. A compiled command, component pass or this command reference
supplies no such acceptance or qualification.

`ardents-node node --config PATH` runs one separately keyed Node process from
an `ardents-node-plan-v1` input. The schema remains shared with current closed
duties, but the old `rendezvous`, `initiator`, `introduction`, `responder`, and
`transit_issuer` reservations return `old Node duty reservation is retired`
after bounded plan recognition and before any plan-owned key, certificate,
Source root, State root, listener, resource, or duty effect. A mixed old and
closed plan receives the same refusal; omission never selects an old default or
a closed fallback. The retained old stanza fields identify refused input. The
Rendezvous, Initiator, Responder, Introduction, and Transit-issuance engines and
their command composition are absent. Their plan stanzas remain only so the
adapter can classify and refuse bounded input; none is a runnable command
profile.

A current closed Node process owns one admitted duty, pressure reaction, drain,
withdrawal, and joined cleanup; lifecycle JSON uses
`ardents-node-event-v1`. On Linux, `SIGTERM` and foreground interrupt request
local withdrawal before process exit. An arbitrary Node config is not a
supported Node operating profile; the bounded config cannot override State's
duty selection.

The generation-3 forwarding reservation is `closed_forwarding`, containing
only an existing owner-only receiving-spend `root`, `connection_limit` (1–16)
and `drain_timeout_ms` (1–60,000). It uses `node --config`, never `issuer serve`.
Exactly one closed issuer, forwarding or resolution stanza may be selected, with no legacy
native duty alongside it. `closed_profile_authority` must name an authority
already pinned in the State plan; no new trust root is admitted. The Node
runtime verifies the accepted State profile and its current adjacent/interior
assignment before opening the listener. It derives the actual listener,
Carrier and allowed next peers from State. The forwarding stanza accepts none
of those facts. Plan parsing/owner dispatch does not itself prove a complete
protected participant journey.

The `closed_resolution` stanza contains absolute separate `root` (Descriptor
Store) and `admission_root` paths, `connection_limit` (1–16), and
`drain_timeout_ms` (1–60,000). It uses `node --config` and the same existing
State-pinned `closed_profile_authority`; `issuer serve` refuses this plan.
State must assign the local Node to the closed resolution duty before the
listener opens. The stanza accepts no endpoint, peer, role, profile digest or
verification/storage callback. Descriptor lookup and publication use actual
Control admission and the durable Store; this configuration alone does not
establish Publisher readiness or private Introduction registration.

The retired dedicated-host Contributor subcommand (`apply`, `restart`,
`diagnose`, `drain`, `withdraw`, `remove`) is removed entirely by ADR-0114: no
live `ardents-rendezvous-contributor` installation remains, the owned-shutdown
obligation of ADR-0089 has no object, and `internal/contributor` with its
runbook is deleted. A `contributor` argument is no longer a recognized command
shape and fails with the standard usage error before any platform, filesystem,
supervisor, or Network effect.

## `ardents-custody`

`ardents-custody` is separate from Endpoint, Release, and Update. It accepts no
Authority material or password through flags, environment, configuration, or an
Application data stream.

| Route | Required flags | Result |
|---|---|---|
| `create-service-authority` | `--vault-root PATH` plus exact environment, Network, and Authority-root public commitments | Reads a new password and confirmation only from the terminal, generates the Service Authority inside custody, and emits its opaque record ID, public Authority, derived Target, and identity commitment. |
| `issue-service-credential` | `--vault-root PATH --record ID --request PATH --response PATH` plus the exact public Service Authority binding | Before opening the Vault or asking for its password, requires the Custodian to type the lowercase `request_sha256` transferred independently from the requesting host. A mismatch fails without password entry, response, record, or floor mutation. The supported issuer additionally limits one Credential to 24 hours and its terminal horizon to 48 hours. It then writes the monotonic public response to the explicit new file and emits its deterministic encrypted successor record ID. An exact retry returns the same response; a different request cannot advance the stale record. |
| `create-admission-authority` | `--vault-root PATH --environment-commitment HEX --network-commitment HEX --root-commitment HEX` | Creates a separate interactive admission Authority in Custody and emits its record ID, identity commitment, and public key as `ardents-admission-authority-v1`. |
| `issue-admission-permission` | `--vault-root PATH --record ID --request PATH --permission-output PATH --environment-commitment HEX --network-commitment HEX --root-commitment HEX --kind admission --id-commitment HEX` | Requires the Custodian to enter the independently transferred request SHA-256 before opening the Vault or asking for its password. After verifying the exact admission Authority binding, writes the private permission to the explicit owner-only destination without replacing different bytes and emits a public receipt with only the permission digest. |
| `inspect-envelope` | `--envelope PATH` | Validates and prints only canonical public envelope facts as `ardents-custody-inspection-v1`. It owns no Vault root and cannot create custody state. |
| `verify-record` | `--vault-root PATH --record ID --environment-commitment HEX --network-commitment HEX --root-commitment HEX --kind service|name|admission --id-commitment HEX` | Reads one password from an interactive no-echo terminal, verifies one active encrypted record against exact public commitments, and prints bounded non-secret facts as `ardents-custody-verification-v1`. |
| `export-recovery-bundle` | Exact vault record, public Authority commitments, and output Bundle path | Reads the record secret only from the terminal, writes a separately passworded Bundle, and test-restores it before success. |
| `restore-recovery-bundle` | Empty destination Vault, public commitments, Bundle path | Reads the Bundle password from the terminal and writes only an `authority-locked` quarantine record. |
| `purge-record` | Exact vault record, public commitments, and terminal confirmation | Deletes only the exact verified encrypted record after explicit confirmation while retaining the Authority floor. |

The command surface has no automatic Service Credential renewal route. The
interactive issuance, finite validity, and published-Instance restart limits
are owned by [Custody](../technical/release-update-custody.md) and
[Endpoint and Service](../technical/endpoint-service-runtime.md); replacing a
file, changing a Target, or deleting state is not a command-level recovery
procedure.

The custody command deliberately exposes no Service recovery activation. A
restored Service Authority remains locked and issuance-unavailable. ADR-0105
retired the never-exposed Namespace signing, submission, and reconciliation
operations along with the subsystem itself; a restored Name Authority record
remains inspectable, verifiable, exportable, and purgeable through the
generic record commands, and no Namespace witness or activation route exists.

The completed `ardents-release-custody` and `ardents-state-custody` local
ceremony commands have no maintained routes. ADR-0067 retires their current
reader/writer contracts; R-119 through R-121 and Git history retain the exact
historical evidence.

## `ardents-control`

`ardents-control` is a separate control program. It never starts an
Endpoint, downloads bytes, changes Release/Network State roots, or turns an
alpha name into any canonical Naming state. It has no current two-Endpoint
qualification intake; the retained routes below own their own explicit inputs
and inspection roots. The enrollment-pinned ACA1 disclosure reader retired
with ADR-0127 (#647); successor per-domain commands verify the same authority
facts, and no unified replacement report is selected.

| Route | Required flags | Result |
|---|---|---|
| `inspect-closed-issuer-profile` | `--profile PATH --node-key HEX --network HEX --node HEX` | Read-only offline inspection of the JSON public export from closed `ardents-node issuer initialize`. Verifies its schema, digest, Node signature and independently supplied Network/Node bindings. Reports `ardents-closed-issuer-inspection-v1`, including `NetworkID`, `IssuerNodeID`, `NotBefore`, `NotAfter` and `TokenKeys` in closed-profile-plan form. It does not accept State or decide current validity; future hourly keys may be provisioned before activation. |
| `prepare-closed-profile` | `--plan PATH --output PATH` | Render the canonical unsigned `ARDCPR03` body from a bounded public plan into a new file. |
| `sign-closed-profile` | `--plan PATH --authority-key PATH --output PATH` | Reread the exact public plan and sign only `ARDCPR03` with the owner-only PKCS#8 Ed25519 State-authority file; write a new file and report its digest without printing key or profile bytes. |
| `inspect-closed-profile` | `--plan PATH --profile PATH --authority HEX --at RFC3339` | Read-only verification of the signed profile against the independently pinned State authority and exact Network/Epoch/time context. Durable State acceptance is separate. |

The historical exact `accept-alpha-corpus` and `inspect-alpha-corpus` routes
are omitted from usage and each returns `<route> is retired` before parsing
arguments or opening an enrollment input, artifact, catalog, corpus, control
root, inspection root, or corpus floor. Neither creates a new root nor changes
retained floor bytes, and no ACA2 verifier remains in the tree
([ADR-0110](../adr/0110-retire-aca2-corpus-inspection.md)). The
[Endpoint Alpha destination retirement contract](../technical/endpoint-service-runtime.md#alpha-destination-retirement)
owns the wider existing-link and retained-data boundary.

The ACA1 inspection and qualification-preparation routes are likewise omitted
from usage and refuse at dispatch before parsing arguments or opening any
file, root, or floor ([ADR-0127](../adr/0127-retire-aca1-inspection-and-portable-enrollment-diagnosis.md), #647):
`inspect-bundle` and `inspect-transitions` return `<route> is retired`; the
`ardents-alpha-control-report-v1` / `ardents-alpha-transition-report-v1`
schemas and the ACA1 reader/inspection Modules are removed, and existing
inspection floor roots stay on disk unchanged. `prepare-qualification-evidence`
and `prepare-qualification-catalog` return `<route> is retired` with their
alpha preparation scripts. `prepare-qualification-node-record` returns
`prepare-qualification-node-record is retired; use ardents-next network
prepare-initial-node-record`, and `prepare-qualification-epoch` returns
`prepare-qualification-epoch is retired; use ardents-next network
prepare-initial-epoch`; the successor preparation and signing commands cover
the same initial Network need with the same canonical ARNR-v2/AREP-v3 bytes,
and the qualification-network scripts are removed.

The caller-keyed low-level `inspect` route and the always-unqualified future
`inspect-public-control` projection are not maintained command routes. Their
underlying verification Modules and historical evidence remain available to
the actual enrolled flows and Git history; no participant or qualification
profile called either route.

The four completed planning-campaign `simulate-*` routes were retired by
[ADR-0060](https://github.com/dianabuilds/ardents-network/blob/c58268d72f1eaeae173144e76d32d50986de2a77/docs/archive/adr/0060-retire-completed-planning-campaign-generators.md).
Their historical command lines, JSON schema identities, and receipts remain
unchanged in the accepted ADRs, research records, external evidence, and Git
history; they are not current command compatibility promises.

None of these routes launches a browser, opens a Service, chooses a Relay/Gateway, or
makes an `ardents-alpha://` link a public DNS/HTTPS address. The retired intake
route retains no supplied bytes: it refuses before parsing arguments or opening
files and does not fetch or install an Endpoint. The Endpoint has no accepting
Alpha destination adapter: the historical prefix receives `alpha service link
is retired` before a retained corpus-floor read, resolver/network work, or any
Target-Link fallback.

## Trusted text UI and workers

`ardents-text link /absolute/path/to/administration.sock` explicitly presents
the canonical Target Link of the currently committed text publication through
the private Administration socket. It refuses an unavailable, uncommitted, or
withdrawn publication; it neither publishes nor retries. The output is an
interruptible terminal or pipe presentation and is absent from ordinary
diagnostics.

The local client command is `ardents-text read /absolute/path/to/connection.sock`.
Use Linux terminal or pipe input/output; non-interruptible descriptors are refused.
Enter one explicit Target Link on standard input and finish the line. The
destination is not a command argument or diagnostic. The client sends one AAI3
Target-Link request and one fixed text request, with a 30-second setup/exchange
deadline after input. It prints the complete document only after successful
terminal completion and joined local stream cleanup. C0/C1 controls other than
LF/tab, ESC and Unicode bidi controls appear as local `\uXXXX` escapes. Links
remain text; the command does not launch or fetch them. There is no output-file
option, history, automatic retry or alternate-interface fallback. Interrupt or
SIGTERM cancels the input/read and joins the owned local handles.

This client does not establish confinement or authenticate Network facts.
The selected Endpoint must enforce canonical Target admission, verified worker
launch and the protected Service path before accepting its AAI3 request.
Client conformance against a local test peer is not proof that the protected
participant composition or installed Ubuntu workload is ready.

The trusted publication client is
`ardents-text publish /absolute/path/to/administration.sock /absolute/path/to/document`.
On Linux it imports the selected regular UTF-8 file without following symlinks,
rejects changing or oversized input, and sends only its bounded snapshot to the
Administration owner. It returns success only after the owner confirms committed
publication, including the actual Descriptor acknowledgement. It emits no
document or path and does not retry. The Endpoint Administration owner launches
the fixed Publisher worker with that snapshot and retains the publication after
the command returns. Obtain its Target Link with `ardents-text link
/absolute/path/to/administration.sock`; withdraw through `ardents endpoint
withdraw /absolute/path/to/administration.sock`. These commands require the
running text participant and its separately accepted Administration permission.
Component conformance and installed journey results remain separate evidence;
command success does not establish full host qualification.

The installed `worker-reader` and `worker-publisher` entrypoints accept no
additional arguments. They perform their descriptor audit and fixed worker
exchange without initializing the trusted UI's signal handling. Ordinary
failure diagnostics contain no destination, content or underlying peer reason.

## Process and support limits

All maintained commands return a non-zero process status when their bounded input or
owned lifecycle fails. The repository's current process tests cover retained
source and Node lifecycles; they do not qualify public deployment, platform
durability, privacy, or Node capacity. See the current technical contracts for
[Network, Route, and Node](../technical/network-route-node.md),
[Endpoint and Service](../technical/endpoint-service-runtime.md), and
[Release, Update, and Custody](../technical/release-update-custody.md).

### Closed Introduction registration reservation

An `ardents-node` plan may select exactly one `closed_introduction` reservation
with `admission_root`, `connection_limit` (1–16), and `drain_timeout_ms`
(1–60,000). It requires the same independently pinned
`closed_profile_authority` as the other closed duties. State supplies the
Introduction Node identity, assignment and literal Carrier endpoint; the
reservation has no peer, slot, Target or verification callback input. It cannot
be combined with another native or closed duty reservation. This currently
starts the registration/withdrawal receiver; it does not yet provide the
complete Publisher capsule-delivery or publication-readiness journey.
An `ardents-node` plan may instead select one exclusive `closed_data_join`
reservation with `admission_root`, `connection_limit` (1..16) and
`drain_timeout_ms` (1..60000). It requires the same separately named profile
signer already pinned in State. The plan supplies no remote address, role,
profile bytes, peer identity or verifier; current State selects the Rendezvous
DataJoin duty and Carrier. Mixed duty reservations refuse before startup.

## New Direct-Origin Source development invocation

`ardents-next source serve <plan>` runs one Linux development Source. The bounded
JSON plan contains `network` (the existing new Network authority plan),
`process_profile` equal to `ardents-source-process-v1`, `participation_root`,
`address` (literal IP/nonzero TCP port), `certificate`, `private_key`, `client_root`
and one to three `client_pins` (the accepted Source transport-key transcript digests as 32-byte arrays).
Paths are absolute and canonical; State and participation roots are independent.
The Network root must already hold independently authenticated new State.
Transport credentials and CA are supplied inputs; the command creates no keys.
Only signed Epoch bundles are distributed. No Node role, acquisition, refresh,
signing, certificate provisioning, installed startup or operator cutover is selected.
The original native process envelope is mandatory. Output reports fixed safe
`source.serve` phase/outcome fields. Cancellation joins State users before process
release; readiness/output grants no authority.

The original plan, certificate, private key and client CA retain direct original
file/ancestor custody until joined retirement. Symlinks and non-single-link or
nonregular inputs refuse; the private key belongs to the invoking user with no
group/other access. Replacement, byte/access/ancestor loss permanently denies
the lifetime and interrupts State; restoring a pathname cannot revive it. A
separate invocation readmits the original inputs. There is no live PEM reload.
State Close and original Wait join before command key/input/process release;
input observers join before retained bytes/descriptors are released, with
process pins last. The plan fixes the clock path, while State independently
verifies its live mtime evidence under unchanged time/floor policy.

Source reports require an interruptible native stream with a one-second write
bound. Safe failure output additionally names fixed `first_owner` and
`cleanup_owner` categories and available original numeric `resource` facts;
raw errors, paths, keys and request content are absent. A five-second
completion-unavailable notice retains failure and all original custody while
waiting for actual join, rather than returning completion from a timer.

READY output is bounded by the earlier of its one-second write bound and the
original caller context. Terminal reporting keeps its own finite write bound
after caller retirement. Ordinary cancellation cannot become an invented
background/output failure; real retained process and cleanup errors remain failed.

## JOIN public artifact preparation

On Linux, root may run these separate bounded offline producers:

```text
ardents-control join prepare-descriptor IDENTITY VERSION PROGRAM TEMPLATE OUTPUT
ardents-control join prepare-inventory DISTRIBUTION OUTPUT
```

The first operation exclusively creates `OUTPUT/infrastructure-join.json`,
binding exact public program and template bytes. It does not validate the native
unit policy. The second requires exactly `ardents-node-linux-amd64`,
`ardents-join.service`, `infrastructure-join.json`, `root.json`, `timestamp.json`,
`snapshot.json` and `targets.json`, checks descriptor/resource agreement and
exclusively creates `OUTPUT/infrastructure-enrollment.json`. OUTPUT is a fresh
protected directory outside the distribution; existing or partial output is
refused. Original root-controlled regular inputs and ancestors remain checked
through durable output and physical Close. Public receipts report the operation
and output digest only.

Neither operation signs metadata, supplies an independently delivered initial
pin, authenticates a Release, or admits installed JOIN execution. Source and
Forwarding inventories cannot be substituted. Other platforms refuse before
input/output effects.

### JOIN fixed request and template preparation

```text
ardents-control join prepare-template OUTPUT
ardents-control join prepare-request --installation-root PATH --state-root PATH --participation-root PATH --spend-root PATH --hosting-root PATH --credential-directory PATH --plan PATH --certificate PATH --private-key PATH --clock PATH --output OUTPUT
ardents-control join prepare-plan --request REQUEST --output OUTPUT --network-id HEX --node-id HEX --profile-authority HEX --authority HEX --threshold N --work-tx N --work-rx N --termination-tx N --termination-rx N --retirement-wait-ms N
```

All eleven request options are mandatory and unique. The request producer opens
none of the declared runtime or credential paths; it exclusively writes
`OUTPUT/join-request.json` after checking canonical independent paths, every
retained journal sibling and the complete bounded completion-socket namespace.
The output directory cannot overlap any of those paths. The template producer
exclusively writes `OUTPUT/ardents-join.service`, the fixed unsigned release
resource. Its declared future JOIN entrypoint is not a startup permission or
proof that an installed Node supports it. Both commands retain the existing
root-controlled native output/Close rules and public-only digest receipts.

Plan preparation reads only the original protected public request. All twelve
plan options are explicit and unique except `--authority`, which may be repeated
for up to sixteen distinct State keys; threshold must fit that set. The output
must be the request's exact future `join-plan.json` location, outside all other
inputs, roots and retained journals. The producer binds declared installed
credential names, retains original request/output custody through Close, and
opens no private key, clock or semantic root. Its explicit traffic declarations
and notice interval grant neither an allowance nor a completed join or runtime
permission. Native installation and all live owners independently admit those
facts before effects.

### JOIN fixed Release preparation

The separate JOIN Release commands are:

- `join prepare-release-keys FRESH_PRIVATE_OUTPUT`: five fresh distinct Ed25519
  files, with public-only facts and the fixed three-of-five ordinary threshold.
  It creates no signature, initial pin or independent-custody claim.
- `join prepare-release-plan` and `prepare-forward-release-plan`: the same nine
  explicit public flags `--network`, `--reference`, `--expires`, `--program`,
  `--descriptor`, `--template`, `--program-custom`, `--generation-custom`,
  `--output`. They bind the canonical static Linux-amd64 Node executable,
  exact JOIN descriptor and fixed template to reviewed common public claims.
  Initial version is 1; forward version is strictly above 1. Output is the fresh
  `join-release-plan.json`, without opening private keys or runtime roots.
- `join sign-initial-release PLAN PRIVATE_KEYS FRESH_METADATA FRESH_VERIFICATION`
  and `join sign-forward-release PLAN PRIVATE_KEYS FRESH_METADATA
  FRESH_VERIFICATION PREDECESSOR_DISTRIBUTION`: fixed Node/JOIN targets only.
  The forward operation verifies the original complete JOIN predecessor and
  matching five-key/three-threshold Root, preserves its exact Root bytes and
  strictly raises Targets/Snapshot/Timestamp versions. It cannot supply
  predecessor runtime retirement or permission for an installed replacement.

`python3 -I -S scripts/installation/prepare-join-release-plan.py` prepares the
same public plan from two actual matching canonical Node/Control build records,
exact JOIN resources and separately pinned prestart evidence. Its flags match
the existing Forwarding public tool, including the distinct `--forward` purpose.
The shared builder's original evidence schemas remain unchanged; the JOIN plan
and signed targets are separate. Two controlled executions do not establish
independent builders. Signed metadata and independently produced inventory must
be copied unchanged; separate initial trust and exact bootstrap acceptance
remain required before isolated installed execution. No JOIN runtime is started
by any of these operations. Original private input/output users join before
acknowledgement; partial outputs remain retained and cannot be adopted.


Canonical Node's separate `ardents-node join inspect-inputs REQUEST` performs
only root-controlled original JOIN delivery and fresh read-only initial namespace
observation. It joins original input/manager users before reporting
`join-inputs-preflight-completed` with `installed:false`. It creates no account,
unit, runtime root or history and supplies no later installation/Start permission.

### JOIN initial stopped installation

- `ardents-node join install-initial-stopped DISTRIBUTION PIN HISTORY ENVIRONMENT NETWORK REQUEST` verifies the initial running program/pin and coherent Release pair, creates one protected stopped JOIN generation, and joins all original users before publishing completion. It never starts the role or initializes semantic histories.
- `ardents-node join inspect-initial-history DISTRIBUTION HISTORY ENVIRONMENT NETWORK BINDING` independently verifies complete original native history and fresh permission for the same payloads. It closes original readers/verifier before output; no startup right is returned.

An existing account, root or retained partial journal refuses fresh initial
installation. Preserve that prefix and its error evidence; these commands supply
no adoption, rollback or recovery operation.

`ardents-control join prepare-hosting --binding BINDING --provider PROVIDER
--start RFC3339 --end RFC3339 --unit UNIT --quantity QUANTITY --directions DIRECTIONS
--interfaces NAMES --initial-used-bytes FLOOR --low-watermark-bytes FLOOR`
prepares the single known-born allowance after completed stopped JOIN installation.
It requires root and all explicit flags; repeated or partial preparation refuses.
This operation starts no Node and supplies no runtime authority.

### JOIN protected first start

`ardents-node join start-initial DISTRIBUTION HISTORY ENVIRONMENT NETWORK BINDING`
performs one explicitly admitted root first Start, original archival/ACK and joined
observer publication. It refuses retained/partial first-start storage. The fixed
unit alone invokes `ardents-node join start-installed BINDING` under the actual
non-root Node account; independent Startup completes before genuine State/duty,
Guard, Admission/Hosting and receiving JOIN construction.

The separate canonical `ardents-control join` offline producers are
`prepare-state-key`, `prepare-node-key`, `prepare-node-record`, `sign-node-record`,
`prepare-node-certificate`, `prepare-epoch`, `sign-epoch`, `prepare-token-keys`,
`prepare-profile`, `sign-profile` and `prepare-state PLAN BUNDLE SIGNED_PROFILE`.
Each retains fixed-purpose input/output custody; signatures and accepted State
are distinct from installed duty or bootstrap acceptance. State preparation runs
only as the actual numeric system Node principal, on its known-born empty root.
No first-start command grants retirement or replacement permission.
`ardents-node join stop HISTORY BINDING` independently retains the original
invocation and accepts its fresh completion report only after original domain
and Startup users join. Clean original native exit and independent retirement
readback precede publication. Missing reports and uncertain completion refuse.
`ardents-node join restart DISTRIBUTION HISTORY ENVIRONMENT NETWORK BINDING`
requires every preceding committed retirement and fresh Release/native admission.
It preserves the same generation, account, credentials and semantic histories;
no automatic restart or missing-history initialization is admitted.

### Explicit JOIN generation replacement

`ardents-node join replace DISTRIBUTION HISTORY ENVIRONMENT NETWORK BINDING REQUEST`
selects one strictly forward stopped JOIN generation after confirmed original
domain/native retirement. It retains the account, keys and semantic histories;
partial or unknown completion refuses. `join restart` separately admits the next
protected invocation. Replacement grants no automatic restart or recovery.

JOIN's explicit stopped recovery uses `ardents-node join recover DISTRIBUTION
HISTORY ENVIRONMENT NETWORK BINDING REQUEST`. It requires a separately completed
original cut at the complete stopped replacement boundary and never starts or
reloads a unit. The bounded original controller is `ardents-node join
supervise-transition --stop-after-record 62 --timeout-ms N -- replace
DISTRIBUTION HISTORY ENVIRONMENT NETWORK BINDING REQUEST` (N at most120000).
Its result reports original physical cut completion only, never recovered or
runtime-ready state. Unsupported/uncertain prefixes remain blocked.

## Descriptor-role public preparation

Linux canonical Control provides these fixed offline commands (fresh OUTPUT
directories; original inputs must have protected root-owned ancestors):

```text
ardents-control descriptor prepare-template OUTPUT
ardents-control descriptor prepare-artifact ID VERSION PROGRAM TEMPLATE OUTPUT
ardents-control descriptor prepare-inventory DISTRIBUTION OUTPUT
ardents-control descriptor prepare-request --installation-root PATH --state-root PATH --participation-root PATH --spend-root PATH --hosting-root PATH --descriptor-root PATH --credential-directory PATH --plan PATH --certificate PATH --private-key PATH --clock PATH --output OUTPUT
ardents-control descriptor prepare-plan --request REQUEST --output OUTPUT --network-id HEX --node-id HEX --profile-authority HEX --authority HEX --threshold N --work-tx N --work-rx N --termination-tx N --termination-rx N --retirement-wait-ms N
```

The exact immutable resources are `infrastructure-descriptor.json` and
`ardents-descriptor.service` alongside the Node program and four Release metadata
files. `prepare-artifact` binds supplied program/template bytes; it neither
validates loaded unit policy nor signs metadata. The request keeps Store separate
from State, participation, spending and the one Hosting root. The plan retains
explicit public trust declarations; repeated `--authority` is allowed within its
finite bound, other duplicate options refuse. Declared runtime inputs/roots are
not opened or created. Input/output users join before the public hash receipt.
These operations supply no initial trust, Release permission, installed startup,
Store ACK, publication readiness or live Node duty. Other platforms refuse.


The distinct root-only `ardents-node descriptor inspect-inputs REQUEST` retains
original delivery and independently observes the actual manager/all five absent
Node slots, future resource namespaces and account. The result
`descriptor-inputs-preflight-completed` follows original native/input joins and
explicitly reports `installed:false`. It opens no Release verifier or semantic
root, creates no account/unit and grants no future creation/Startup permission.


### Descriptor initial Release and stopped installation

```sh
ardents-control descriptor prepare-release-keys FRESH_PRIVATE_KEYS
ardents-control descriptor prepare-release-plan --network NETWORK --reference UTC --expires UTC --program PROGRAM --descriptor ARTIFACT --template TEMPLATE --program-custom PUBLIC_CLAIMS --generation-custom PUBLIC_CLAIMS --output FRESH_OUTPUT
ardents-control descriptor sign-initial-release PUBLIC_PLAN PRIVATE_KEYS FRESH_METADATA FRESH_VERIFICATION
ardents-node descriptor install-initial-stopped DISTRIBUTION INDEPENDENT_PIN HISTORY ENVIRONMENT NETWORK REQUEST
ardents-node descriptor inspect-initial-history DISTRIBUTION HISTORY ENVIRONMENT NETWORK BINDING
```

The separate public `scripts/installation/prepare-descriptor-release-plan.py`
consumes original actual two-execution canonical build records with explicit
prestart evidence. Its initial purpose is fixed; it signs nothing. The key
receipt states shared project-controlled custody and supplies no trust or
independence. Initial installation verifies the independent inventory pin and
actual running executable before opening Release history. It creates one fixed
stopped unit and five empty semantic roots, including Store, and never starts
Node or initializes any semantic history. Independent history inspection uses
its own native journal lease and fresh retained Release permission; output grants
no runtime authority. Uncertain or existing native prefixes remain blocking.

Descriptor first startup additionally provides fixed canonical Control operations
`descriptor prepare-node-key`, `prepare-node-record`, `sign-node-record`,
`prepare-node-certificate`, `prepare-state-key`, `prepare-epoch`, `sign-epoch`,
`prepare-token-keys`, `prepare-profile`, `sign-profile`, `prepare-hosting` and
non-root `prepare-state`. Their finite public/offline formats are separate from
runtime duty. Hosting takes explicit binding/provider/period/unit/quantity/
directions/interfaces/initial-used-bytes/low-watermark-bytes options and never
reinitializes an existing allowance.

`ardents-node descriptor start-initial DISTRIBUTION HISTORY ENVIRONMENT NETWORK
BINDING` admits only the first completed stopped generation under fresh retained
Release floors. The fixed unit alone invokes `descriptor start-installed BINDING`
as `ardents-node`; ordinary execution cannot supply its original native Startup
or root-peer exchange. Listening is not Store ACK, Service readiness or lifecycle
acceptance. No automatic restart, key reload, adoption or rollback is provided.


Descriptor original lifecycle commands:

- `ardents-node descriptor stop RELEASE-HISTORY BINDING`: explicitly retire the
  original invocation through actual domain/native completion. Missing completion
  stays blocking; this command does not admit restart.
- `ardents-node descriptor inspect-started-history RELEASE-HISTORY BINDING`:
  independently verify the complete retained invocation/retirement chain without
  runtime permission or history repair.
- `ardents-node descriptor restart DISTRIBUTION RELEASE-HISTORY ENVIRONMENT NETWORK BINDING`:
  fresh exact-artifact/native admission and independent non-root startup for the
  same generation after committed original retirement. Missing histories refuse.


`ardents-node descriptor replace DISTRIBUTION RELEASE-HISTORY ENVIRONMENT NETWORK
BINDING REQUEST` selects an explicit strictly forward stopped Descriptor resource
or program generation after complete original retirement. It preserves account,
credentials and all five semantic histories; fresh Release, independent existing
history checks and joined native readback precede completion. It does not Start
the candidate. Partial/uncertain transitions retain their blockers.


`ardents-control descriptor prepare-forward-release-plan` uses the same explicit
public flags as `prepare-release-plan`, with a separate forward schema.
`ardents-control descriptor sign-forward-release PUBLIC_PLAN PRIVATE_KEYS
FRESH_METADATA FRESH_VERIFICATION PREDECESSOR_DISTRIBUTION` independently verifies
the exact original signed predecessor and unchanged root/key purposes before
raising Targets/Snapshot/Timestamp versions. Its output grants no installation
or runtime permission. The root-only actual-build preparer accepts `--forward`
for this distinct reviewed purpose; the initial default still requires version1.


Descriptor stopped recovery uses two distinct canonical operations:

```text
ardents-node descriptor supervise-transition --stop-after-record 62 --timeout-ms 120000 -- replace DISTRIBUTION RELEASE-HISTORY ENVIRONMENT NETWORK BINDING REQUEST
ardents-node descriptor recover DISTRIBUTION RELEASE-HISTORY ENVIRONMENT NETWORK BINDING REQUEST
```

The first owns an original interrupted replacement and its separate physical cut;
it grants no recovery or runtime authority. The second admits only the complete
stopped prefix with fresh proofs and original predecessor completion. It neither
starts a Node nor repeats Reload. Partial or uncertain prefixes retain their
blockers and require repair; files alone supply no recovery permission.
