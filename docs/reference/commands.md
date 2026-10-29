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
`forwarding prepare-hosting` runs once as root after completed stopped
installation, before the first start. Supply exactly `--binding`, `--provider`,
`--start`, `--end`, `--unit`, `--quantity`, `--directions`, `--interfaces`,
`--initial-used-bytes` and `--low-watermark-bytes`. Dates use RFC3339 whole
seconds; interfaces are comma-separated actual names. The original binding
selects the one already-born host allowance directory; no alternate root option
exists. Existing contents, any prior invocation or partial preparation refuse.
It preserves the directory inode, delivers original period files to the Node
account and retains a separate root-only one-use transcript. Output supplies no
runtime authority. A failed partial operation cannot reset or retry the period.

`forwarding prepare-state PLAN BUNDLE SIGNED_PROFILE` runs finite offline
preparation as the `ardents-node` system account. The root-controlled plan comes
from `forwarding prepare-plan`; the bundle and profile contain independently
supplied signed public Network inputs. State verifies both and checks the declared
Node before the command closes its State/input owners and prints
`state_accepted:true` with `runtime_authority:false`. The State directory must
already be empty and owned 0700 by that account. Existing/partial directories
refuse another initialization. This command opens no private key, participation,
spend or Hosting root and supplies no installed Startup or role authority.

`forwarding prepare-template <fresh-output>` writes the fixed
`ardents-forwarding.service`. `forwarding prepare-request` requires exactly one
each of `--installation-root`, `--state-root`, `--participation-root`,
`--spend-root`, `--hosting-root`, `--credential-directory`, `--plan`,
`--certificate`, `--private-key`, `--clock` and `--output`. It writes the closed
`forwarding-request.json` to a separate fresh protected directory. Declared
runtime/input paths are not opened or created by this preparation.
`forwarding prepare-plan` requires `--request`, `--output`, `--network-id`,
`--node-id`, `--profile-authority`, one or more `--authority`, `--threshold`,
`--work-tx`, `--work-rx`, `--termination-tx`, `--termination-rx` and
`--retirement-wait-ms`. IDs and independently approved public keys are lowercase
64-digit hex. Output must be the directory already declared for the request's
`forwarding-plan.json`; all runtime paths come from that retained request. The
producer validates canonical declarations and original input custody, not signed
State, private credentials or sufficient runtime allowance. Missing full Forward
coverage still refuses before receiving spend at runtime.

The separate Linux Forwarding Release producers are:

```text
ardents-control forwarding prepare-release-keys FRESH_PRIVATE_OUTPUT
ardents-control forwarding prepare-release-plan --network NETWORK --reference RFC3339 --expires RFC3339 --program NODE --descriptor DESCRIPTOR --template TEMPLATE --program-custom PUBLIC_CLAIMS --generation-custom PUBLIC_CLAIMS --output FRESH_PLAN_OUTPUT
ardents-control forwarding sign-initial-release PUBLIC_PLAN PRIVATE_KEYS FRESH_METADATA FRESH_VERIFICATION
ardents-control forwarding prepare-forward-release-plan --network NETWORK --reference RFC3339 --expires RFC3339 --program NODE --descriptor DESCRIPTOR --template TEMPLATE --program-custom PUBLIC_CLAIMS --generation-custom PUBLIC_CLAIMS --output FRESH_PLAN_OUTPUT
ardents-control forwarding sign-forward-release PUBLIC_PLAN PRIVATE_KEYS FRESH_METADATA FRESH_VERIFICATION PREDECESSOR_DISTRIBUTION
```

Before initial signing, the separate
[controlled Forwarding build and public-plan tools](../operations/qualification-release.md#forwarding-infrastructure-artifact-component)
can produce `forwarding-release-plan.json` from two actual Node/Control builds
and separately pinned prestart evidence. This avoids hand-authoring build claims.
Use the canonical `sign-initial-release` command above with that plan and fresh
protected keys. Build records and signatures do not grant independent builders,
a participant first pin or the separate bootstrap acceptance.

Key preparation creates five distinct Ed25519 PKCS8 files, `release-key-1.pem`
through `release-key-5.pem`, under exclusive root-owned 0700 native ext4 custody;
the files are 0600. Its receipt contains only public keys and commitments. Plan
preparation retains the exact public artifact and reviewed claim files and writes
`forwarding-release-plan.json` with schema
`ardents-forwarding-initial-release-plan-v1`. It requires all nine flags exactly
once, version one, the fixed Forwarding template and an actual static Linux-amd64
canonical Node executable. It invents no build or qualification claim. The
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
The public build-plan script's `--forward` switch produces this separate plan
from actual build records and a forward descriptor, without JSON editing. Its
signatures and diagnostic floors remain separate from Installation's actual
retained trust, stopped replacement, subsequent explicit Start and bootstrap
acceptance. Existing partial output or diagnostic history is never reused.

The Linux-amd64 component commands below implement the Source artifact boundary
under accepted ADR-0123 through direct new owners in `ardents-next`:

- `infrastructure prepare-descriptor <release-identity> <positive-version> <program> <unit-template> <fresh-output>` writes `infrastructure-source.json`.
- `infrastructure prepare-inventory <seven-file-directory> <fresh-output>` writes `infrastructure-enrollment.json`, after actual signed metadata are supplied.
- `infrastructure authenticate-initial <distribution> <independent-inventory-pin> <new-release-history> <environment> <network> <reference-time>` checks its actual executing artifact and two fresh Release proofs.
- `infrastructure authenticate-candidate <distribution> <retained-release-history> <environment> <network> <reference-time>` requires complete retained floors and supplies no initial-pin provenance.
- `infrastructure preflight-initial <distribution> <independent-inventory-pin> <new-release-history> <environment> <network> <reference-time> <source-request>` performs initial artifact admission and Source read-only native preflight in the same original caller.
- `infrastructure preflight-candidate <distribution> <retained-release-history> <environment> <network> <reference-time> <source-request>` uses complete retained floors and adds no initial provenance.
- `infrastructure install-initial-stopped <distribution> <independent-inventory-pin> <new-release-history> <environment> <network> <reference-time> <source-request>` performs original initial authentication, fresh native admission, exclusive account/input/root/generation/journal creation, selection, fixed Source unit publication, one reload and actual loaded stopped-policy verification. It returns only `source-initial-stopped-selected` after original users and physical Close join. Any retained history or partial prefix refuses a new initial attempt; no adoption, reset, rollback, continuation or Start is provided.

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
Only `install-initial-stopped` performs the bounded stopped installation effects;
none starts Source, provides a metadata signer, replaces independent
first-distribution trust or establishes supported `ardents-node` cutover.

The preflight variants additionally require Linux root on the selected
Ubuntu24.04/systemd255/cgroup-v2/native-ext4 stand and the exact canonical
[`ardents-source-preflight-v1` request](../technical/network-route-node.md#component-order-and-the-commandartifact-boundary).
The existing Source plan points to future credential destinations; original
certificate/key/CA source files are separate request inputs. The four future
root/directory destinations and fixed account/unit namespace must be fresh, not
adopted from an existing installation. Preflight prints only
`source-read-only-preflight-observed` after original manager/input/Release users
and Close join. The result grants no creation, delivery, start or later effect;
Release's independent authentication floor effects are the only durable writes.

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

ADR-0119's successor selects `endpoint provision <request-file>`,
`endpoint installation-check <installation-root>`,
`endpoint start-installed <installation-root>`,
`endpoint upgrade-installed <request-file>` and
`endpoint recover-installed <installation-root>`. Provision, installation-check
and start-installed are callable; upgrade and successor recovery also have
production callers, while their complete interruption/installed qualification
remains unfinished. Initial recovery requires an owned initial intent and complete
fixed-file birth records; missing provenance returns repair-required rather than
adopting leftover objects. It returns `installed-recovered-stopped`, preserving
the separate explicit-start operation. The
[implementation handoff](../technical/endpoint-service-runtime.md#selected-protected-installation-handoff)
defines their bounded contract. Full supported installed qualification is still
incomplete. Provision returns only `installed-stopped`; installation-check
returns only `local-integrity-verified`; start-installed accepts only the actual
fixed service main process before delegating to the bound participant plan.
Successful upgrade returns `installed-started`, and successful successor recovery
returns `installed-recovered-started`; neither is a Service readiness or
publication-continuity receipt. If Root has acknowledged the installed invocation
but subsequent completion cleanup fails, upgrade or successor recovery emits
`installed-started-recovery-required` together with a nonzero command result.
The exact recovery guard remains retained; the result does not establish Service
readiness or permit ordinary restart through that guard. Upgrade requests omit
the first-install pin.
No generic privileged launch,
implicit start on provision, authority-key creation or accepting test permission
route is selected. Existing command behavior remains documented below.

`ardents` is the Endpoint and Network State command adapter.

For the Portable `endpoint user-unit` and `endpoint enroll` first execution,
follow the [independently pinned Ubuntu procedure](portable-enrollment.md)
before running the downloaded program. That procedure starts the generic
per-user profile; the protected text Service has a separate installed
system-manager launch boundary.

| Route | Purpose |
|---|---|
| `accept-offline --state-root PATH --network-id HEX --authorities HEX,... --threshold N --at RFC3339 --epoch PATH --inputs PATH --materialization PATH --profile NAME [--closed-profile-authority HEX] [--closed-profile PATH]` | Accept one complete authenticated offline Network State generation under the exact selected State profile. The closed Route profile requires its pinned authority even when submitted later. When `--closed-profile` is supplied, the command accepts that signed profile **after** committing the Epoch; a profile refusal does not undo the accepted generation. Success emits one `ardents-state-event-v1` `generation-accepted` JSON event, with `closed_profile_sha256` when the profile was also accepted. |
| `accept-closed-profile --state-root PATH --network-id HEX --authorities HEX,... --threshold N --at RFC3339 --profile ardents-route-v3 --closed-profile-authority HEX --closed-profile PATH` | Submit one signed `ARDCPR03` only to an already accepted State root under its explicit pinned closed-profile signer. It neither accepts an Epoch nor contacts Source. An exact retry is harmless; a different valid digest records durable conflict and makes the closed profile unavailable. It emits one `ardents-state-event-v1` `closed-profile-accepted` JSON event containing only the State generation, Epoch and profile digest. |
| `refresh-sources --state-root PATH --source-plan PATH [--once|--resume]` | Run one selected Direct-Origin Source wave, resume from current State, or wait for the plan-owned `ardents-source-plan-v1` refresh interval. `--once` and `--resume` are mutually exclusive. The `ardents-source-event-v1` `source-wave-accepted` event is emitted only by a wave that actually ran; `--resume` merely confirms the current accepted State is readable before entering the scheduler and invents no acceptance event. |
| `diagnostics timeline` | Read app JSON lines or journalctl JSON from standard input and stream a local timeline of Node, Source, and Endpoint events. It reads no authority root, contacts no peer, and saves no history. |
| `service-instance initialize --config PATH` | Create or reopen one host-owned Service Instance generation from an `ardents-service-instance-initialize-v1` plan whose `request_file` names one new public output, and emit its stable public request plus `request_sha256` for the independently transferred custody ceremony. The command exposes neither private key, Service Authority, Credential, Target, Route, nor Browser state. |
| `service-instance accept --root PATH --response PATH` | Atomically accept only the exact canonical public Authority response for that pending root. An exact repeat is harmless; malformed or different input terminally rejects/conflicts rather than replacing the generation. |
| `endpoint headless <headless-runtime.json>` | Only an explicit `ardents-headless-runtime-v2` plan selects the protected text participant. With omitted `role`, it requires the closed Route profile, an explicit `closed_profile_authority` Ed25519 public key already present in `network_authorities`, existing `service_instance_root`, distinct absolute State/Entry/local-role/publication/token roots, two sockets, and `reader_permission`/`publisher_permission` objects containing `request_path`, `response_path`, and three-class `maxima`. With omitted `role`, both actual offline permissions must be accepted before command exposure. An explicit `role: reader` requires only its own Connection principal/socket and Reader Permission, together with the same State/Source/time/Entry/local-role/token and qualified-worker inputs; it rejects nonempty Instance/Publication/Administration/Publisher inputs and opens no Administration transport. Unknown roles refuse before effects. This component selection does not establish the supported installed two-Endpoint journey. Legacy acquisition and corpus fields are rejected in v2. Event output must be a pollable pipe or stream socket (including the systemd journal); each JSON line carries `schema: ardents-headless-runtime-event-v1`, `kind`, and UTC `at` for local timeline inspection. Cancellation and bounded writes retain descriptor ownership. Full installed command qualification remains required. |
| `endpoint open <application-socket> <target-link> <input-file> <output-file>` | Return `endpoint open is retired` before opening either file, dialing the local socket, or causing Endpoint/Network work. The AAI2 codec/server/client and exclusive Endpoint adapter are absent; no generic AAI3 translation or fallback is selected. |
| `endpoint publish <administration-socket>` | Return `endpoint publish is retired; use ardents-text publish <administration-socket> <document-file>` before dialing the Administration socket or starting Endpoint/Network work. Use `ardents-text publish /absolute/path/to/administration.sock /absolute/path/to/document` for snapshot publication. |
| `endpoint withdraw <administration-socket>` | Request withdrawal through the exact local one-use Service Administration capability and render its bounded receipt. A publisher plan must explicitly retain its administration listener after publication for this route. |
| `endpoint enrollment-check <bundle-root> <manifest-sha256>` | Diagnose one already-running artifact against an independently pinned closed-alpha inventory. It does not authenticate first execution. |
| `endpoint enroll <bundle-root> <manifest-sha256>` | Run the explicit Ubuntu Portable enrollment and per-user profile start. On first acceptance it verifies the pin and Release Decision before reporting `ready`; that event proves the generic local probe attachment, not protected text Service readiness. An exact current-program restart uses its retained replacement record. |
| `endpoint enroll-installed <package-enrollment.json>` | Run the Installed enrollment and per-user profile start for one root-owned package artifact and versioned static enrollment root. Its `ready` event has the same generic attachment scope; protected text runs through the separate `endpoint headless` route. |
| `endpoint user-unit <bundle-root> <manifest-sha256>` / `endpoint installed-user-unit <package-enrollment.json>` | Render, but never write, enable, or start, the matching `systemd --user` unit. |
| `endpoint replace <replacement-bundle>` | Perform one explicit Ubuntu local Release-authorized replacement against the fixed user unit; it neither downloads nor schedules updates. After a preactivation staging or temporary stop refusal, rerun this same command with a still-valid bundle for the exact candidate bytes. It rechecks Release and the retained transaction; do not edit or delete replacement state manually. |
| `endpoint replacement-recovery` | Report durable replacement recovery classification only; it never starts, replaces, or rolls back a program. An owned interrupted temporary write does not invalidate a matching committed executable; this read-only route leaves the residue untouched. Unexpected state must not be manually edited or deleted. |
| `<journal-bound-recovery-program> endpoint rollback <replacement-bundle>` | Perform the only permitted explicit rollback: the retained predecessor, with a fresh Release authorization for its exact bytes. |
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

`endpoint headless` refuses `ardents-headless-runtime-v1` before opening any
plan-owned runtime resource. The exact refusal and retained-data boundary are
defined by the
[Endpoint startup retirement contract](../technical/endpoint-service-runtime.md#v1-startup-retirement).
The former v1 participant process composition has been removed; refusal does
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

The three-argument v1 forms `endpoint enrollment-check <alpha-enrollment.json>`,
`endpoint enroll <alpha-enrollment.json>`, and `endpoint user-unit
<alpha-enrollment.json>` are retained solely so a pre-existing Portable user
unit can restart or render its explicit migration unit. They verify the same
bundle and manifest digest but are not a new C0 enrollment route; new units use
the independently delivered manifest pin argument.

## `ardents-node`

`forwarding inspect-inputs REQUEST` is a separate root-only Linux read-only
preflight. It retains the original Forwarding request/plan/TLS files and direct
ancestors, checks the actual systemd255 manager and all five absent Node
service/socket slots, absent common account and future runtime destinations,
then joins its native users before public output. It creates no resource or
Release history and grants no install, Start or runtime permission.

`forwarding install-initial-stopped DISTRIBUTION PIN HISTORY ENVIRONMENT NETWORK
REQUEST` directly verifies the actual first Node executable and Forwarding
inventory, obtains the coherent fresh Release pair, and invokes its distinct
initial creator. It creates only the fixed shared Node account, empty semantic
directories, protected credentials, immutable generation and stopped unit. Partial
or uncertain prefixes refuse ordinary retry and remain repair-required. It does
not start a runtime or initialize State/spend/Hosting; full native transaction and
installed startup acceptance remain required.

`forwarding inspect-initial-history DISTRIBUTION HISTORY ENVIRONMENT NETWORK
BINDING` independently opens the completed initial journal and original files,
retains its own exclusive lease, checks the current fixed principal and obtains
fresh coherent Release decisions for the same installed targets. It creates no
history or semantic root and repairs no partial result. The public result
`forwarding-initial-history-verified` supplies no runtime authority or current
manager/invocation admission. Original descriptors and the verifier close before
that result. This is a separate verification step, not an installed Start.

`forwarding start-initial DISTRIBUTION HISTORY ENVIRONMENT NETWORK BINDING`
directly obtains a fresh coherent Release pair and invokes root's separate
first-start owner. It repeats original initial/native admission, admits at most
one fixed-unit Start, verifies the actual non-root peer/process/scope and syncs
the original archive before ACK. The pending marker is published only after
root's original observers and writers join. A completion-unavailable notice
continues waiting for original physical completion; it grants no successful
start or join. Existing/uncertain attempt storage refuses replay or fresh retry.
The public `forwarding-initial-start-completed` result grants no transferable
runtime authority. This command does not supply explicit retirement/restart,
bootstrap acceptance or installed qualification.

`forwarding inspect-start-history HISTORY BINDING` independently reopens only a
fully published finite invocation/retirement chain, retains its own initial/start/retirement journal
leases and checks every original file/socket identity and the retained Release
history directory. It creates or repairs no history. Its public result
`forwarding-start-history-verified` has `runtime_authority:false` and
`retirement_proved:false`; it supplies no current process, Stop or restart right.

`forwarding stop HISTORY BINDING` is the separate root latest-invocation stop
consumer. It requires complete published start history and independently matched
original program/process/scope/manager custody. It interrupts only the original
pidfd, requires the original command's joined-domain report, actual kernel join,
matching native exit and observer closes, then commits independently reopened
retirement bytes. Release expiry does not prevent shutdown. A two-minute bound
may print `forwarding-stop-completion-unavailable`; the command keeps waiting
for original physical completion. Unknown or partial completion retains blocking
history. The successful result `forwarding-original-stop-completed` has
`restart_admitted:false`. It refuses a latest invocation already retired or any
unknown/partial earlier history. An installed run still requires its exact
bootstrap acceptance.

`forwarding restart DISTRIBUTION HISTORY ENVIRONMENT NETWORK BINDING` requires
the entire retained chain and the latest committed original retirement. It
repeats fresh coherent Release and actual stopped/native protection checks,
then starts the next independently admitted invocation of the same immutable
generation. The new intent commits its immediate predecessor's retirement
hash; no old live owner transfers. Missing/damaged/held/partial histories refuse,
and at most 64 retained invocations exist without path reuse or automatic reset.

The non-root `forwarding start-installed BINDING` consumer emits a bounded
`forwarding.start-installed` event after a failed original lifetime has joined.
Its `trigger`, `first_owner` and `cleanup_owner` are fixed local categories;
optional `first_failure` uses Node's existing numeric/category projection, with
no identities, paths or private error text. This event preserves diagnosis only:
it grants no domain retirement proof or restart right, and failure to write it
remains part of the original failed result.
All installed Forwarding lifecycle events use one original nonblocking native
descriptor. The listening event has a one-second write bound, shortened by the
startup caller's deadline or cancellation. Retirement notices and failed joined
results instead receive a fresh one-second cleanup allowance after caller
cancellation; they retain no expired runtime authority. Saturated output and
broken peers remain errors; no detached writer can retain Startup or private
plan material after command completion.
Account, credentials and semantic histories remain retained. Its public result
is `forwarding-same-generation-restart-completed`, with `runtime_authority:false`;
runtime State/duty and role authority remain separately owned. This command
selects no generation replacement, recovery continuation, automatic boot or
restart, and supplies no installed qualification by itself.

`forwarding replace DISTRIBUTION HISTORY ENVIRONMENT NETWORK REQUEST BINDING`
selects one strictly forward stopped generation after fresh coherent Release
authorization and independent complete predecessor retirement. The separate
`forwarding start-selected DISTRIBUTION HISTORY ENVIRONMENT NETWORK BINDING`
admits its first runtime invocation; later `restart` stays within that selected
generation. Credentials, semantic roots, provider allowance and the entire
original history remain bound. Installed qualification requires its own exact
accepted bootstrap receipt.

`forwarding supervise-transition --stop-after-record N --timeout-ms MS -- ACTION ...`
retains an original canonical coordinator from native birth through physical
join. ACTION is exactly `install-initial-stopped`, `replace`, `start-initial`,
`start-selected`, `restart` or `stop`, with that command's usual arguments.
MS is1..120000; N is zero for ordinary completion or a positive original journal
record count, bounded by120 initial,62 replacement,26 start or18 retirement.
A controlled cut reports `forwarding-original-cut-completed` with
`recovered:false`, and retains the original interrupted operation's blockers.
Its separate root-only guarded proof supplies historical physical observations;
recovery still requires independent fresh admission and operation-specific proof.
An arbitrary executable, non-root runtime start, unknown action, late Close
failure or uncertain acknowledgement cannot supply this result.

`forwarding recover DISTRIBUTION HISTORY ENVIRONMENT NETWORK REQUEST JOURNAL`
uses an existing Release trust root and fresh coherent candidate authorization.
Its current safe classifier accepts an original initial-v2 cut after the
`stopped-verified` or `completed` record, or a selected replacement-v2 cut
after its complete after-snapshot-ready or `replacement-verified` record.
It also accepts an original retirement cut after its complete result-ready
or `retired` record, retaining the exact original joined-domain/native result
and complete invocation chain. It cannot reconstruct an earlier missing result.
Replacement recovery independently retains the complete retired predecessor,
original Hosting preparation and existing semantic histories. Fresh permission
for the same payloads may advance metadata floors; the original sealed metadata
stays unchanged. Every other prefix returns
repair-required with blockers preserved. It never rewrites the original journal,
replays account creation/reload/Start/ACK, initializes semantic roots or invents
domain completion. Its separate guarded suffix and independently reopened
completed readback precede removal of the original blocker; the recovery guard
is removed last, after all required physical closes. The result is
`forwarding-recovery-stopped`, with `started:false`.
Ordinary readers independently validate the original cut, complete suffix,
original resources and fresh manager/search-path/kernel context. A subsequent
generation requires the complete v2 replacement before/after snapshot lineage;
all original non-generator observations remain exact. Missing or v1-only
reload evidence cannot supply a context refresh. Installed qualification remains
separately admitted.

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

`ardents-node source start-installed BINDING` is the distinct native entrypoint
for the selected non-root Source unit. Independent Installation Startup verifies
its exact original generation, account, process, namespace, cgroup and manager,
and completes the original root-peer exchange before constructing genuine State
and the mandatory Source Guard. State owns signed serving/time/exposure decisions.
Original checks continue through serving; denial interrupts work and joins State,
observers and native input/key/process custody. A completion-unavailable notice
retains failure and keeps waiting; it releases no resource or original duty.
After every join and native Close, the command reports `retired` only for a nil
result or an error tree containing exclusively ordinary cancellation. Every
other retained result reports `retirement-failed`; a late join cannot erase an
earlier unavailable notice. Reporting failure also remains command failure.
These bounded observations supply no runtime authority or installed qualification.
This selector bypasses optional debug-socket setup.

`ardents-control source prepare-state SERVE_PLAN BUNDLE` is a separate finite
offline producer under the actual Source system UID/GID, without supplementary
groups. It consumes the public serving declaration and bounded signed Epoch /
materialization bundle with original file/ancestor custody, and requires the
already created runtime-owned 0700 State directory to be empty. New State
authenticates and durably accepts the bundle under its independently selected
pins and actual time. The command joins State before releasing inputs and emits
only a bounded outcome. It opens no serving/acquisition transport, process Guard,
Installation Startup or runtime TLS key. Partial failure preserves the resulting
State files; it supplies no reset, adoption, retry or rollback operation.
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

The public closed template producer writes only the fixed selected resource
`ardents-source.service`, without rendering local paths or publishing a unit:

```text
ardents-control source prepare-template FRESH_OUTPUT
```

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
signer's schema. The program must be a static Linux-amd64 Go `ardents-node`,
descriptor version one and template the exact selected Source resource. Both
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

The single-Source public input preparer is:

```text
ardents-control source prepare-install-inputs --network ID --state-key PUBLIC_KEY --identity ID --family FAMILY --handle HANDLE --address IP:PORT --server-name NAME --installation-root PATH --state-root PATH --participation-root PATH --credential-directory PATH --clock PATH --server-private-key PATH --output FRESH_OUTPUT --server-certificate PATH --server-certificate-sha256 SHA256 --server-ca PATH --server-ca-sha256 SHA256 --server-key-pin PIN --client-certificate PATH --client-certificate-sha256 SHA256 --client-ca PATH --client-ca-sha256 SHA256 --client-key-pin PIN
```

It retains four original public files under independently supplied SHA-256
commitments and checks explicit CA, name, EKU, current validity and Source's
transport-key separation. IDs, public key and pins are exact lowercase 32-byte
hex; key pins use Source's transport-key transcript. A fresh protected native
output contains exactly `source-certificate.pem`, `source-ca.pem`,
`client-certificate.pem`, `client-ca.pem`, `source-serve.json` and
`source-request.json`. The request refers to those prepared public copies and
the separately supplied external server private-key path; the serving declaration
uses Installation's fixed future credential names. Private-key input, mutable
roots and clock are declarations only and remain unopened. The command creates
no State, runtime root, TLS credential, second family or acquisition Plan. Its
public receipt commits each file and reports `native_admission: false`.
Installation independently checks actual private-pair delivery and native
admission; offline observation does not supply installed/bootstrap acceptance.

The root command forms are:

```text
ardents-node installation install-initial-stopped DISTRIBUTION PIN HISTORY ENVIRONMENT NETWORK REQUEST
ardents-node installation start-source DISTRIBUTION HISTORY ENVIRONMENT NETWORK BINDING
ardents-node installation stop-source HISTORY BINDING
ardents-node installation restart-source DISTRIBUTION HISTORY ENVIRONMENT NETWORK BINDING
ardents-node installation replace-source DISTRIBUTION HISTORY ENVIRONMENT NETWORK BINDING REQUEST
ardents-node installation recover-source DISTRIBUTION HISTORY ENVIRONMENT NETWORK REQUEST JOURNAL
```

They directly consume separate Enrollment, Release and Source Installation owners
in the same original operation, with actual current reference time and independent
native resources. Initial installation verifies the independent pin and actual
running artifact; first start revalidates fresh coherent proofs against retained
history and the complete original stopped transcript. A separate exclusive
first-start journal refuses replay, reuse or continuation. Durable archival
precedes the one completion ACK. Root completion reports transaction completion,
never State authority, runtime readiness or a transferable permission.

Stop checks the archived original invocation and uses its retained pidfd. A
separate original domain-completion exchange, clean process exit, native scope
join and complete synced root transcript precede the final removal of its
blocking marker. A public `retired` line, old ACK or inactive unit is insufficient.
Restart independently checks every start/retirement record under the original
first journal's lease and admits fresh Release/native/startup work in a new
exclusive invocation directory. It preserves the same generation, account,
credentials, roots and semantic histories; State refuses empty retained history.
Failed or uncertain prefixes remain repair-required, with no reset/adoption or
automatic restart. The finite retained history admits at most 64 total attempts.
Root outputs `source-stopped-completed` or `source-restart-completed` only for
their completed bounded operation; reporting grants no runtime authority.

Replacement requires the complete original explicit retirement and two fresh
coherent forward Release authorizations against that same retained history.
Prepare its descriptor/inventory and inputs with the canonical public commands;
use `prepare-source-release-plan.py --forward` and the distinct
`source sign-forward-release` purpose with the original keys and predecessor
distribution. Initial-only signers and a fresh Enrollment pin cannot authorize
a successor. Replacement preserves account, credentials, semantic roots and
predecessor evidence, and checks State's read-only retained-history inspection.
It seals the candidate, rewrites the original selection/unit inodes and explicitly
reloads/readbacks the manager. Output `source-replacement-stopped-selected`
describes that completed stopped transition. Explicit `restart-source` then
obtains a fresh invocation; no automatic Start or rollback follows. With
unchanged program bytes, an actual permitted listen-address change and strictly
forward descriptor/metadata are still required. Incomplete transitions retain
their original blocking evidence.

For explicit recovery, supply the original retained journal, the matching
prepared Source request and authenticated candidate distribution against the
same retained Release history. Recovery never accepts a new initial pin or
clears damaged evidence. A controlled initial/replacement prefix additionally
needs its genuine completed original-cut result, original native files and
fresh domain admission. It writes only a separate completion suffix and the
permitted remaining effects, returning `source-recovery-stopped-selected`
after required Close. Start remains a separate explicit command with fresh
admission. Startup/retirement journals are never replayed: only complete start
and committed retirement histories plus fresh stopped observations can return
stopped. Unknown birth, torn records, uncertain Sync/Close, missing history,
unproven domain join or substituted manager remains repair-required. Preserve
that prefix; recovery supplies no rollback, account adoption, key rotation,
history reset or arbitrary host repair.

These native operations require the selected Ubuntu24.04/systemd255/cgroup-v2/ext4
profile; unsupported environments refuse. Before any isolated installed Source
execution, ADR-0123 requires the completed public exact-artifact bootstrap receipt
and its separate direct Product Owner acceptance. A compiled command, component
pass or this command reference supplies no such acceptance or qualification.

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

`ardents-control` is a separate alpha-control program. It never starts an
Endpoint, downloads bytes, changes Release/Network State roots, or turns an
alpha name into any canonical Naming state. It has no current two-Endpoint
qualification intake; the retained routes below own their own explicit inputs
and inspection roots.

| Route | Required flags | Result |
|---|---|---|
| `inspect-bundle` | `--enrollment PATH --artifact PATH --state-root PATH --at RFC3339` | Participant closed-alpha route. It first verifies the exact enrolled bundle and artifact, then runs the maintained Release Decision, Network State, ACA1 catalog, and Release/Network/Compatibility component verifiers. The named standalone inspection root owns separate `catalog`, `release`, and `network` floor children and is physically distinct from Endpoint state. Repeating the same accepted input against the same inspection root reports the cached/no-update outcome; a second absent inspection root supplies an independent fresh observation. |
| `inspect-transitions` | `--enrollment PATH --artifact PATH --state-root PATH --at RFC3339` | Participant transition diagnostic. It runs the same enrollment-pinned inspection and emits `ardents-alpha-transition-report-v1`: nested exact closed-alpha control evidence plus independent Release Safety, Network Epoch, Compatibility, and Namespace-materialization outcomes. It advances only the explicitly named standalone inspection floors; it never mutates Endpoint state. `not-selected` for Namespace never creates a close, release, reclaim, or current Namespace state. |
| `inspect-closed-issuer-profile` | `--profile PATH --node-key HEX --network HEX --node HEX` | Read-only offline inspection of the JSON public export from closed `ardents-node issuer initialize`. Verifies its schema, digest, Node signature and independently supplied Network/Node bindings. Reports `ardents-closed-issuer-inspection-v1`, including `NetworkID`, `IssuerNodeID`, `NotBefore`, `NotAfter` and `TokenKeys` in closed-profile-plan form. It does not accept State or decide current validity; future hourly keys may be provisioned before activation. |
| `prepare-closed-profile` | `--plan PATH --output PATH` | Render the canonical unsigned `ARDCPR03` body from a bounded public plan into a new file. |
| `prepare-qualification-evidence` | `--plan PATH --output-root PATH` | Prepare the mutually bound initial unsigned Release/Network/Compatibility payloads and ACS1 signing inputs under ADR-0120. Absolute public plan and previously absent absolute output root; no signatures, State acceptance or participant floor mutation. |
| `prepare-qualification-catalog` | `--plan PATH --output-root PATH` | Prepare the initial unsigned ACA1 signing input from exact component references. Absolute public plan and previously absent absolute output root; referenced bytes and their authorities remain separately verified. |
| `prepare-qualification-node-record` | `--plan PATH --output-root PATH` | Prepare the existing unsigned ARNR-v2 message for a fresh generation-one closed Node Record with its own public key and explicit Node identity. No Node signature, duty or State acceptance. |
| `prepare-qualification-epoch` | `--plan PATH --output-root PATH` | Verify every supplied initial closed Node Record and prepare the existing unsigned AREP-v3 initial Epoch commitments/assignments. State signing keys are separate from Node keys; no State signature, acceptance or floor reset. |
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
