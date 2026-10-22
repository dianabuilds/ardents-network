# Operator-prepared qualification release

The nearest closed release uses TCP/TLS alone under
[ADR-0122](../adr/0122-select-tcp-only-closed-release.md). Recognized QUIC
profiles remain distinct historical signed inputs and receive an explicit
unsupported outcome before transport or Admission effects. There is no enabling
switch or TCP fallback. Current positive runs select TCP/TLS; retained dual-Carrier
receipts describe their original artifacts and do not qualify this release.

[ADR-0120](../adr/0120-authorize-operator-prepared-qualification-release.md)
selects this isolated operation. It does not qualify installation or independent
distribution. The Release verifier and participant first-pin contract remain
unchanged. Never use diagnostic keys as the operational release authority.

## Source infrastructure artifact component

`python3 -I -S scripts/installation/build-source-candidate.py
--source-manifest PATH --source-manifest-sha256 SHA256 --source-revision REVISION
--output FRESH_OUTPUT` builds the fixed five-command inventory twice with the
selected Go1.27.3 Linux-amd64 toolchain, no cgo and canonical flags. The outside
controller retains the common development lock and whole build-scope join,
supplies a read-only complete source snapshot, independently pinned manifest
and checked module cache, and keeps output outside the repository. No network
fetch or module replacement is admitted. Both executions use project control;
the tool explicitly reports no builder independence or installed qualification.
Each actual execution retains its own command hashes; the tool also retains
canonical build inputs, actual compiled-package/module inventory and its scoped build SBOM, checks
matching output and reobserves the original source/manifest and module integrity.
The Go module files remain exact hashed inputs; ordinary module/dependency gates
still cover their declared graph. The build SBOM inventories the five actual
commands' compiled closure and selects no unused package or protocol.
An unfinished output is preserved and never adopted. `READY` describes this
build result only. A preliminary dirty snapshot must use the explicit revision
`snapshot-sha256:<manifest pin>`; it cannot claim the old HEAD as its source.
The final installed candidate uses the committed exact source revision and still
requires every normal gate and the separately accepted exact bootstrap receipt.

The separate public plan transformation is:

```text
python3 -I -S scripts/installation/prepare-source-release-plan.py --build-root PATH --descriptor PATH --template PATH --network NAME --expires UTC_TIME --no-new-work-after UTC_TIME --terminate-after UTC_TIME --qualification-evidence PATH --qualification-evidence-sha256 SHA256 --output FRESH_OUTPUT
```

After the original build scope has joined, the controller delivers the exact
public build outputs/records into root-controlled custody and rechecks their
retained commitments; the preparer requires that custody, both actual command
inventories and coherent source/input/module/SBOM records. It generates the
existing Source signing-plan schema and two target-specific rebuild records,
using actual current time and supplied finite bounds. It reads no private key.
Supply separately reviewed, pinned prestart qualification evidence describing
the exact candidate's checks and limits; the tool retains its digest in its
public receipt. The existing metadata's proposed `qualified` value describes
that bounded component evidence, never installed qualification or independent
builders. Review the plan/receipt alongside that evidence before signing.
The output is `source-release-plan.json`; no manual JSON editing is needed.
The native Source signer independently checks original plan/resource/key custody
and obtains actual Release decisions. This public transformation and any signed
metadata still supply no direct PO bootstrap acceptance or runtime permission.

ADR-0123 accepts a distinct Source program and generation contract. Canonical
`ardents-control source prepare-descriptor` and `source prepare-inventory`
directly prepare its public artifact bytes through new Infrastructure/Enrollment
owners, without installing a program, account, unit or Source process. Temporary
`ardents-next infrastructure` artifact consumers remain component mechanisms,
separate from the canonical [command forms](../reference/commands.md).
Use the actual canonical compiled Linux-amd64 command bytes, publicly supplied
Source unit-template bytes, a release identity and positive version to prepare
the descriptor in a fresh native output directory. Retain original protected
inputs through physical output Close and readback. Preparation produces public
unsigned bytes and grants no signing or trust authority.
`source prepare-template` supplies that exact selected public template through
the same fresh native output mechanism; no manual resource copying is required.

The distinct canonical `ardents-control source sign-initial-release` producer
describes only `ardents/linux-amd64/node` and
`ardents/linux-amd64/infrastructure/source` in one initial signed set under the
existing five-key/ordinary-three-signature and rebuild policy. Its separate
[closed plan and original custody](../reference/commands.md) neither reinterpret
the Endpoint signer below nor accept an arbitrary target/message. Supply fresh
protected operational keys from the separate `source prepare-release-keys`
ceremony, real recorded rebuild inputs and honest scoped
qualification facts; native fixture keys/declarations are not operational input.
The actual Source candidate/normal preparation chain and its exact public
bootstrap receipt remain separate from a tested signing mechanism. Assemble a
separate seven-file public distribution with the
program named `ardents-node-linux-amd64`, `ardents-source.service`,
`infrastructure-source.json`, `root.json`, `timestamp.json`, `snapshot.json` and
`targets.json`. Inventory preparation requires exactly those original regular
files and writes its canonical document to another fresh output directory.
The final distribution contains these seven files plus
`infrastructure-enrollment.json`; no Endpoint companions or additional files.

Deliver and verify the inventory SHA-256 independently before first execution.
Running self-verification cannot establish independent trust in that first
execution. The initial command must execute the identical enrolled file and use
empty independent Release history. It verifies the pin before parsing and keeps
original initial provenance in the same process as the coherent Release pair.
After any floor exists, use only the candidate route with complete retained
history. There is no fallback from missing history to an initial pin and no
reset/conversion of Endpoint, Network or Release roots. Explicit reference time,
environment and Network remain caller bindings, not distributor-selected facts.

Success attests this bounded artifact check only. The public receipt carries
digests and fresh program/generation outcomes; copying it transfers no proof.
Retained template bytes do not attest rendered installed policy or current
native resources. Native installation/startup and final supported command/tool
cutover remain separate, as does the exact public bootstrap receipt requiring
Product Owner acceptance before any future installed execution. Existing
ADR-0120 releases and Endpoint inventories retain their scope.

## Source read-only native preflight

The separate `infrastructure preflight-initial` and `preflight-candidate`
component commands consume the same genuine artifact admission in their original
process, plus one root-controlled canonical Source preflight request. Their
[technical owner](../technical/network-route-node.md#component-order-and-the-commandartifact-boundary)
defines its exact fields and separates original source inputs from absent future
installation, State, participation and credential destinations. No account,
directory, credential, generation, selection or unit is created; no reload,
start/stop/enable or Source runtime is invoked. Release authentication floor
effects are separate and retained even when a later native check refuses.

Run the registered `source-preflight-linux` profile only in its private finite
Ubuntu24.04/systemd255/cgroup-v2/native-ext4 manager. The outside controller owns
the common development exclusive lock; it supplies complete source manifest,
canonical command and all three race test binary SHA-256 pins. The checked
runner revalidates them before/after actual tests. Its private foreign account
and unit fixtures are test prerequisites only and are never installed or run as
Source. Missing manager identity, filesystem, namespace, binary or source pins
is invalid environment. Preserve every earlier failure. See
[testing](../development/testing.md) for the exact invocation and limitations.

Success is a bounded read-only observation and transfers no native permission.
Before any future mutation, repeat complete native admission. Credential delivery
and initial stopped generation are separate selected boundaries. Before future
isolated installed execution the exact bootstrap receipt still requires its
separate Product Owner acceptance; preflight neither requires nor supplies it.

## Initial stopped Source component

The canonical `ardents-node installation install-initial-stopped` consumes the
initial independently pinned distribution, two coherent fresh Release proofs and
the canonical Source request in the same original root process. It repeats native
admission; an earlier authentication/preflight receipt transfers no permission.
Use only the registered `source-stopped-linux` private native profile for this
component's qualification. It creates the fresh locked account, explicit runtime
credential delivery, empty semantic directories, sealed twelve-file generation,
selected binding and fixed unit, then performs one daemon reload and compares
actual complete loaded stopped policy. It never starts or enables Source.

Preserve the original signed inputs, independent Release history and
`<installation_root>.source-initial-journal`, including every failed prefix.
An ambiguous or interrupted operation requires repair; this component supplies
no repair/adoption/rollback/continuation command. Do not remove an account, journal,
floor, unit or generation to retry. The journal contains private local observations
and must remain outside public receipts. A successful bounded stopped outcome
does not accept the exact bootstrap receipt or permit installed execution; those
remain separately admitted gates. See the [technical schema](../technical/network-route-node.md#initial-stopped-source-persistence)
and [checked profile](../development/testing.md) for ownership and evidence.

## Prepare exact public inputs

Build the five linux-amd64 commands twice from the exact recorded source tree
with `CGO_ENABLED=0`, `-trimpath` and `-buildvcs=false`. Retain byte comparisons,
toolchain, module/build settings and build-input commitments. Generate the
protected descriptor twice from the actual nine resources; retain matching
bytes. The descriptor's canonical grammar is owned by enrollment. Validate it
with `scripts/protected-generation-check.go` before signing.

Prepare a bounded public JSON signing plan with schema
`ardents-qualification-release-signing-plan-v1`, `network`, `reference_time`,
`expires`, absolute `endpoint`, `descriptor`, `resource_root` paths, and
`endpoint_custom`/`generation_custom`. Custom values use the existing Release
grammar. The two values must agree on all common policy/build facts; only
their actual target-digest rebuild records differ. Version is 1 for this fresh
cohort. Use two actual project-controlled execution receipts, not fabricated
identities presented as independent builders. Record the component inventory's
scope and the exact checks behind the proposed artifact admission.

The fixed targets are `ardents/linux-amd64/endpoint` and
`ardents/linux-amd64/protected-endpoint`. Metadata expiry is finite, at most
seven days from the current reference time. The signer rejects a reference
older than five minutes or more than one minute in the future. Refresh an
unsigned plan before execution; never edit signed descriptor or metadata bytes.
Use `announced` for the isolated initial protocol, without invented overlap
history or emergency grounds.

These targets and the maintained signer/assembly inventory do not authorize
installed serving Source or any Node role. Headless Node/Custody companions are
initial manifest inventory, not an infrastructure generation authorization.
The [infrastructure contract](../technical/network-route-node.md#proposed-installed-infrastructure)
names the separate exact targets, custody, command/tool and native lifecycle
boundary under [accepted ADR-0123](../adr/0123-bind-installed-infrastructure-generations.md).
Its acceptance and an exact public bootstrap receipt for future installed
execution remain distinct decisions. The Endpoint-only signer below and its
prior receipts cannot be used with additional Source targets.

## Protected operation on Linux

Build the two adapters explicitly from their source files:

`make release-operation-compile-check` vets and cross-compiles the explicit Linux
adapters. It is required by `quick-check` and the complete `check` gate;
compilation is not a key/signing-operation or installed qualification result.

```sh
go build -trimpath -buildvcs=false -o /absolute/new/prepare-release-keys ./scripts/prepare-qualification-release-keys.go
go build -trimpath -buildvcs=false -o /absolute/new/sign-release ./scripts/sign-qualification-release.go
```

Run on the selected root-controlled release host. Supply previously absent
absolute output directories under root-owned direct ancestors with no group or
other write permissions. Private keys are PKCS#8 files protected by root-only
filesystem permissions; these files are not encrypted Custody Vaults and add
no Custody unlock or general Authority signing interface.

```sh
/absolute/new/prepare-release-keys /root/new-release-keys > /root/new-public-key-receipt.json
/absolute/new/sign-release /root/public-plan.json /root/new-release-keys /root/new-release-metadata > /root/new-public-signing-receipt.json
```

Key creation uses a new `0700` directory and exclusive `0600` files, syncs
outputs and ancestors before a public acknowledgement, and never overwrites an
existing directory. Signing admits exactly five distinct keys with the existing
ordinary 3-of-5 roles. It checks descriptor/resources and common target facts,
then verifies both targets through separate fresh diagnostic Release floor
roots before writing the four metadata files. Those verification roots do not
alter participant floors and are not bundle entries.

Any error or interruption leaves its partial output for inspection and emits
no success acknowledgement. Do not reuse that directory or infer completion
from its existence. Preserve the original refusal before an explicit new
operation. A file-size-limit failure exercises bounded write refusal; it is
not a power-loss durability qualification.

## Assembly and acceptance boundary

### Source public trust preparation

`ardents-next source prepare-trust <public-plan> <fresh-output>` prepares public
transport inputs from **externally supplied credentials**. Certificate/key/CA
issuance, operational trust delivery, rotation/revocation and hot reload remain
external responsibilities. This finite operation supplies no State or startup
authority. Runtime alone reads its matching private pair and checks certificates
again at each handshake using State's sole trusted clock.

The development `source serve` consumer retains the original serve plan,
certificate, matching private-key and client-CA files/ancestors until State's
actual Close/Wait and input observers join. Supply canonical direct regular
single-link files; keep the private key owned by the invoking user without
group/other access. This runtime admission does not inherit the root-only
offline key-generation directory policy. Changed/replaced/unlinked inputs or
access/ancestor loss retire the original process; do not edit PEM or restore a
path to revive it. A separate explicit invocation readmits all inputs after
original join. The clock file remains live evidence owned by State. This is
neither credential installation/delivery nor accepted infrastructure startup.

The plan is one complete JSON object, at most 64 KiB, with no unknown, duplicate,
null, missing or trailing fields. Its exact fields are:

- `schema`: `ardents-source-trust-preparation-v1`;
- `reference_time`: canonical whole-second UTC RFC3339, at most five minutes
  behind or one minute ahead of the preparation host clock, rechecked through
  commit. It is only an offline X.509 observation;
- `clock_observed_at`: a **separate** existing runtime Source-plan time
  declaration; State independently verifies it and the declared live clock file;
- `client`: the complete existing `ardents-headless-runtime-v2` declaration,
  including all its named fields and both permission objects (`request_path`,
  `response_path`, exact three-element `maxima`). It is retained as a local
  declaration, not accepted Installation startup;
- `client_certificate`, `client_root`: each exactly `{path, sha256}`, with an
  absolute public PEM path and independently reviewed lowercase SHA-256 of the
  **complete file bytes**;
- `client_leaf_key_digest`: independently reviewed lowercase Source pin;
  `client_private_key`: opaque absolute external runtime key path;
- `order_seed`: nonzero lowercase 32-byte hex; `materialization_index`: integer
  0..63; `refresh_interval_ms`: integer 100..60000;
- `sources`: exactly two objects, each with `identity` (nonzero lowercase
  32-byte hex), `family`, `endpoint_handle`, literal-IP/nonzero-port `address`,
  `server_name`, public `certificate` and `root` objects as above,
  `leaf_key_digest`, opaque external `private_key`, `state_root` and
  `participation_root`.

Each public PEM is at most 64 KiB and contains only 1..4 certificate blocks,
with no mixed/private/trailing material. Explicit CA roots, chain, EKU, name and
reference validity are checked by the new Source owner. The exact compatible
leaf pin is SHA-256 of ASCII `ardents-h3-source-transport-key-v1`, one zero byte
and the raw Ed25519 public key; it is neither SPKI nor a raw-key hash. Distinct
Source identities/families/handles/addresses/keys and separate client/State
authority keys retain the same runtime collision rules. Declared families never
prove independent effective control. There is no TOFU or system-root fallback.

The output is exactly ten 0600 files in one fresh 0700 directory:
`client-certificate.pem`, `client-root.pem`, each
`source-1-certificate.pem` / `source-2-certificate.pem` and corresponding
`source-1-root.pem` / `source-2-root.pem`, `source-1-serve.json`,
`source-2-serve.json`, `client-source.json`, `client-headless.json`.
Serve files directly use `ardents-next source serve`; client files directly use
the existing installed Source declaration/projection. No JSON editing or second
runtime schema is required. All six runtime State/participation roots are
separate from each other and output; none is opened. Private declarations are
distinct, outside output and never opened, generated, copied or exported. Their
presence cannot attest runtime key match/access. Private paths never appear in
stdout or diagnostics. The receipt hashes local plans but grants no authority.

The caller must be Linux root with root-controlled, non-group/other-writable
direct ancestors. Original public descriptors must be root-owned single-link
regular files, not writable by group/other, on actual native ext4; output's
parent is root-owned 0700 on native ext4. Original bytes/inodes/access and every
ancestor remain retained and reobserved through the 30-second operation. The
finite writer performs exclusive creation, file sync/close, directory/parent
sync and exact readback/inventory **after the final original-input barrier**.
Physical close/join precedes receipt delivery. Cancellation, partial output,
replacement and independent close/sync/receipt failures remain failed original
outcomes; prefixes remain incomplete and cannot be adopted, overwritten or
cleaned by a later invocation. No listener, process, State root or floor effect
is admitted offline. The real runtime consumer still independently admits each
process/profile, State and key pair; a successful preparation receipt cannot
repair a later refusal.

### Initial State and Node keys

`ardents-next network prepare-initial-keys <public-plan> <fresh-private-directory>`
is the one-shot new composition producer. Both paths must be canonical absolute
paths. The direct root-owned public plan is at most 1024 bytes and has exactly
`schema: "ardents-initial-keys-plan-v1"`, `operation: "prepare-initial-keys"`
and integral `node_count` from 1 through 64. Duplicate, unknown, mixed, trailing
or supplied private/seed fields refuse. The production command uses fresh
standard-library OS-backed randomness for every Ed25519 key and refuses the
whole attempt on a repeated public key; it never redraws a role or identity.

The actual Linux-root/direct-ancestor/native-ext4 admission below applies.
The parent must be 0700, every direct ancestor root-owned without group/other
write, and the destination previously absent. A fresh 0700 directory contains
exclusive 0600 canonical complete PKCS8 files `state-key.pem`, ordered
`node-00-key.pem` through the requested final Node, and `public-receipt.json`.
Original descriptors, inodes, access, exact bytes, closed inventory and
file/directory/parent durability barriers precede joined Close and success.
Symlinks, hardlinks, replacements and existing destinations refuse.

The public receipt names each purpose, Node index, basename, public key and
SHA-256 of that public key, with signature, State acceptance, installed
qualification and independent custody explicitly false. It contains no private
bytes or arbitrary private path. The same receipt is returned on standard output
only after output completion. Any failure, including final receipt delivery or
cancellation, remains failed even if files or a receipt already exist. Retain
incomplete output; a later invocation cannot adopt, overwrite or clean it.

All owned key, seed, DER, PEM and private readback buffers are cleared after
their synchronous users and original descriptors join, on success and refusal.
These are root-controlled plaintext files, not encrypted Vaults; root, kernel
and Go runtime internals may observe plaintext, and absolute memory erasure is
not promised. No private material enters Network. Separate public preparation,
independent signer admission and State acceptance are still required. NodeID
is separately supplied; no Network identity, duty, floor, Source trust or other
authority is generated. A State key may be separately pinned for Epoch and
profile under the existing contract. Verification uses isolated development keys;
operational bootstrap receipt acceptance remains separate under ADR-0120.

### Fixed-purpose signing

#### Local Node certificate

`ardents-next network prepare-node-certificate <public-plan> <signed-record>
<reviewed-record-sha256> <original-node-key> <fresh-output-directory>` produces
one public `node-certificate.pem` and a public stdout receipt. Paths are canonical
absolute paths; the separately reviewed lowercase commitment covers the entire
signed Record. The closed `ardents-node-certificate-plan-v1` object has exactly
`schema`, `reference_time` and `record`. The latter is the complete matching
`ardents-initial-node-record-plan-v1` public declaration, including its separately
bound public Node key. Canonical preparation and ordinary signed ARNR-v2
generation-one verification must agree exactly; only TCP/TLS is admitted.
CSR, arbitrary subject/extensions, CA requests and mixed purposes refuse.

The protected native input, finite reference and output rules of the initial
signers below apply unchanged. Validity is exactly the Record interval, at most
seven days in whole UTC seconds. Both original reference and actual time must
remain inside it; the original reference stays within five minutes past/one
minute future. Checks follow I/O and precede crypto/output commit and final
acknowledgement. No renewal or interval shift is supplied. Standard-library
crypto/rand, Ed25519 and X.509 produce one self-signed leaf with empty subject,
CA=false, DigitalSignature and server/client transport EKUs. Its random positive
serial is not a Node identity. The actual encoded leaf is parsed and its
signature/key/interval/closed profile verified separately before output.

Only the existing original root-protected canonical PKCS8 Node key is read;
it is neither copied to output nor replaced. No State root, socket or process
is opened. Original descriptors/bytes/access/ancestors remain retained through
finite exclusive 0700/0600 output, sync/Close/readback and final inventory after
the last original-input barrier. Uncertain output remains incomplete and cannot
be adopted, overwritten or cleaned by another invocation. Cancellation, expiry,
randomness, physical and receipt failures stay failed. Owned private copies
are cleared after synchronous users/physical join; absolute Go/kernel erasure
is not promised. Public output contains no private bytes, paths or arbitrary
native errors. Receipt marks State acceptance and installed qualification false.
Self-signature grants no membership or runtime readiness; State, Node and Route
independently admit their exact authority/process/credential/lifetime. Source
X.509 trust, installed infrastructure and ADR-0120 bootstrap receipt remain
separate boundaries.

#### Initial and profile signing

`ardents-next network sign-closed-profile` takes the same ordered arguments as
the initial signers below: public plan, exact `profile.unsigned`, independently
reviewed lowercase SHA-256, protected private-key path and fresh output directory.
Its closed schema is `ardents-closed-profile-signing-plan-v1`, with exactly
`reference_time`, `state_authority_key` and `profile` after `schema`. The latter
is the complete exact `ardents-closed-profile-plan-v1` public preparation plan.
The explicit public `state_authority_key` is the independently pinned State
signing authority from the admitted State trust configuration. It is not inferred
from the private key or the profile's separate `issuance_authority_key`.
An explicit alias of that Admission authority as the State pin also refuses.
The command reprepares and compares every unsigned byte and reviewed commitment,
then obtains the validated fixed signing transcript from new Network closedprofile.
It verifies the exact signed bytes under that State pin and public context before
writing only `profile.bin`. This proves signature and public grammar; State alone
joins current Epoch/Record/role/issuer facts and commits acceptance and floors.

The original native custody, finite reference, post-I/O checks, durable output
and failure rules below also apply. Profile validity is at most **six hours**,
with complete canonical token-key windows inside it; actual signing and final
acknowledgement must remain inside the original interval. Profile contains no
Carrier field and signing grants no transport participation. Unknown, mixed or
retired schemas refuse. No State root, key generation, Custody extension, generic
signing service, rotation/history policy or installed claim is introduced.
Only isolated supplied test authorities qualify engineering verification;
operational bootstrap receipt acceptance remains separate under ADR-0120.

New isolated composition offers `ardents-next network sign-initial-node-record`
and `sign-initial-epoch`. Each takes, in order, an absolute public signing plan,
exact public preparation artifact, independently reviewed lowercase SHA-256 pin,
original protected private key path and fresh output directory. Plans use
`ardents-initial-node-record-signing-plan-v1` with `reference_time` and `record`,
or `ardents-initial-epoch-signing-plan-v1` with `reference_time` and `epoch`.
The nested object is the exact corresponding new public preparation plan.
These commands reprepare canonical bytes before signature; a digest alone is
never signing authority. Node Record uses its exact matching Node key; Epoch
uses exactly one matching pinned initial State key, with ordinary signed Record,
Epoch and materialization verification. They admit TCP/TLS only.

Linux root, root-owned non-group/other-writable direct ancestors, one original
root-owned single-link regular 0600 PKCS#8 Ed25519 file (at most 16 KiB), and
root-controlled native ext4 output are required. Public inputs are retained
bounded direct root-owned files without group/other write. The output parent
must be 0700. Original descriptors, bytes, access and links are reobserved
through use and commit. Key encoding is canonical unencrypted version-zero
PKCS#8 Ed25519 with one seed and no attributes, parameters or trailing DER.
Original reference is at most five minutes past or one
minute future; canonical whole-second UTC validity contains both it and actual
signing time, and spans at most seven days. Failure retains incomplete output
and independent native errors; repeated invocation never adopts a destination.
Plaintext key files are not encrypted Vaults. Owned key/DER/raw copies are cleared
after joined use; root, kernel and runtime may still observe plaintext, and
absolute memory erasure is not claimed. No operational ceremony, new trust,
key generation or installed acceptance follows from these
engineering operations. ADR-0120 bootstrap receipt acceptance remains separate.

The maintained `ardents-control prepare-qualification-node-record` and
`prepare-qualification-epoch` commands prepare the real initial Network inputs
under ADR-0120. Both use `--plan` and `--output-root` with canonical absolute
public paths and a previously absent output directory. Plans are bounded to
8 MiB, reject unknown fields/trailing JSON, and use the existing JSON byte-array,
base64 and whole-second time conventions below. The Node plan schema is
`ardents-qualification-node-record-plan-v1` with a `record` object containing
the exported `epoch.InitialClosedRecord` fields. NodeID and its public key are
distinct explicit facts. Only the two closed Carrier profiles, valid host/port,
canonical ASCII text, finite integral nonnegative validity, and capacity
1..1024 prepare. The output is `node-record.signing-input`, unsigned existing
ARNR-v2 bytes with generation one and closed capability two.

Sign each exact message separately with its matching Node identity key before
Epoch preparation. The Epoch plan schema is
`ardents-qualification-epoch-plan-v1`, with an `epoch` object containing the
exported `epoch.InitialClosedEpoch` fields. Supply 1..64 signed Records in the
exact intended Source order, 1..16 strictly ordered Role Domains, nonzero
Network/assignment seed, finite validity covered by every Record, and 1..16
distinct public State authority keys. Preparation reuses ordinary Record
evaluation and refuses any rejection/collision, including a Node key reused as
a State authority. It computes actual input/view/rejection commitments,
family/capacity/Domain summaries and deterministic assignments. The output
`epoch.unsigned` is existing AREP-v3 with Epoch one, zero predecessor, closed
Route profile and no retired interactive Gateway/Transit projection.

These public commands create exclusive `0700`/`0600` outputs and synchronize
files/directories before acknowledgement; retain partial outputs on failure.
They sign no Record or Epoch and admit no State. Separate State signatures over
the actual unsigned Epoch digest, ordinary verification/materializations,
closed profile and Source delivery preparation remain required. Never use test
fixtures, copied historical authorities or cleared floors as operational input.

The explicit Linux-root adapter `scripts/prepare-qualification-network-keys.go`
takes a previously absent canonical absolute private directory and a Node count
from 1 to 64. It creates one `state-key.pem` and the requested distinct
`node-00-key.pem` through `node-63-key.pem` identities, using fresh Ed25519 keys.
The same root-owned ancestor, exclusive mode, synchronization and interrupted
output rules as the Release operation apply. Its public receipt records roles,
public keys and their digests and declares `independent_custody: false`.
It signs nothing, grants no Node duty and creates no Network identity or State
floor. Keep these roles separate from Release, alpha, Source and Custody keys.
The closed-profile signer must be one of the configured pinned State authorities;
the single initial State key can fulfill that State-owned signing responsibility.
Never reuse diagnostic keys in the admitted qualification operation.

The explicit Linux-root adapter `scripts/sign-qualification-network.go` takes
an absolute public plan, one root-owned `0600` PKCS#8 identity key and a new
absolute output directory. Plan schema is
`ardents-qualification-network-signing-plan-v1`, with `reference_time` and exactly
one of `record` (`InitialClosedRecord`) or `epoch` (`InitialClosedEpoch`). The
reference must be current within the same five-minute/past and one-minute/future
window used by the Release operation. The finite interval must contain both
the reference and current time and span at most seven days.

The Record public key must match the supplied Node key. The Epoch operation
admits exactly one matching initial State key, signs the unsigned digest, runs
ordinary Epoch verification, derives every materialization and verifies again
with all materials required. It writes `node-record.bin` or `epoch.bin` plus
Source-ordered `input-NN.bin` and accepted-View-ordered `material-NN.bin` files.
It rejects invalid signed inputs before creating the output directory. Output
files and directories are synchronized before the public receipt; interrupted
outputs remain retained and cannot be replaced. No State root, floor, Source
credential or installed process is created, and the receipt explicitly leaves
State acceptance and installed qualification false.

The maintained `ardents-control prepare-qualification-evidence` command prepares
initial unsigned disclosure payloads and component signing inputs. Its explicit
source-file adapter is `scripts/prepare-qualification-alpha-evidence.go`; both
use the existing evidence and component codec owners. Pass `--plan` with an
absolute public JSON plan and `--output-root` with a previously absent absolute
output directory. The plan schema is
`ardents-qualification-alpha-evidence-plan-v1`; its other top-level fields are
`not_before`, `not_after`, `release`, `network`, and `compatibility`. Times are
UTC whole-second RFC3339 values. Nested evidence fields use the exported field
names of the current `inspection` evidence contracts: byte slices are JSON
base64 strings and fixed 32-byte arrays are JSON numeric arrays. Unknown fields
and trailing JSON values refuse; the public plan is bounded to 32 MiB.

Preparation checks canonical evidence grammar and shared Release/Network/
Compatibility bindings, including the inspected Epoch identity. It writes
`release.payload`, `network.payload`, and `compatibility.payload`, each with
a `.signing-input` companion containing the exact domain-separated initial
ACS1 signing message. Generation is fixed to one; existing signatures, negative
or fractional timestamps and invalid validity intervals refuse. The new output
directory and files use `0700`/`0600`, exclusive creation and synchronization
before acknowledgement. Preserve partial outputs on failure. This public-input
adapter does not enforce the private-key ancestor policy of the Release signer.
It creates no keys or signatures, authenticates no State Epoch, and grants no
Release, Network or Endpoint acceptance. Separate role signatures and ordinary
complete-bundle inspection remain required.

After separately signing the component bytes, the explicit
`ardents-control prepare-qualification-catalog` command prepares the initial
ACA1 signing message (`--plan` and `--output-root`); its explicit source-file
adapter is `scripts/prepare-qualification-alpha-catalog.go`. Its absolute public JSON plan uses schema
`ardents-qualification-alpha-catalog-plan-v1` and a `catalog` object with the
exported `alphacontrol.Catalog` fields. Use the same JSON array/time conventions
above. It bounds the plan to 1 MiB, rejects unknown fields/trailing values,
fixes catalog and component generations to one, refuses a predecessor digest
or existing signature, and requires every component expiry after catalog start
and no later than catalog expiry. References must name the exact signed
component sizes/digests and their separately pinned roots. Preparation checks
grammar only; it does not verify the referenced bytes or grant their authority.
The new exclusive synchronized output is `catalog.signing-input`, with the
same output permissions and partial-output retention as evidence preparation.
Signing this message under the separate disclosure key and ordinary inspection
of the resulting full bundle remain mandatory.

The root-controlled `scripts/prepare-qualification-alpha-keys.go` adapter
creates a new protected directory with four independently generated Ed25519
PKCS#8 files: `catalog-key.pem`, `release-key.pem`, `network-key.pem`, and
`compatibility-key.pem`. These are disclosure/component roles, separate from
the five TUF keys and the State, Node, Source and Custody authorities. Its
public receipt names each role and commitment. Four keys under one operator
are not independent custody. The required retired `corpus.pub` companion is a
fresh public key whose private key is immediately discarded; it grants no
live corpus operation. Creation uses the same root/direct-ancestor, exclusive
`0700`/`0600`, synchronization and retained-failure rules as Release keys.

The explicit `scripts/sign-qualification-alpha.go` adapter takes an absolute
public plan, that private key directory, and a new absolute output directory.
Plan schema is `ardents-qualification-alpha-signing-plan-v1`, with `cohort`,
`reference_time`, `not_before`, `not_after`, `release`, `network` and
`compatibility`. Evidence JSON uses the field conventions above. The operation
requires a current reference (-5 minutes/+1 minute), finite validity of at most
seven days and expiry after current wall time, generation-one v3 Epoch with zero predecessor, closed Route profile,
the fixed Endpoint target, and current/announced initial Release facts.
It checks mutual evidence bindings, bounded direct input files and four
distinct root-owned `0600` opened key files, signs ACS1 components and ACA1
catalog, then verifies exact envelope bindings/signatures with the ordinary
reader. Its callback reports component-domain decisions as unavailable;
neither Release nor State acceptance is asserted or persisted. Completed
outputs are the four `.ac1` files, four `.pub` files and inert `corpus.pub`.
Private keys and any inspection roots are absent from the distribution.
All outputs are exclusive and synchronized before the public receipt. A
successful envelope receipt still requires ordinary complete-bundle inspection
and bootstrap acceptance before installed execution.

The four metadata files alone are not the complete bundle. Prepare the real
current alpha-control and Network companions with their own maintained owners;
keep authority roles separate. Supply exact signed descriptor/resources and
all required static companions to the maintained alpha-bundle assembler.
Verify the complete 27-entry unpacked inventory and collect its manifest digest.
Wrong-pin, mixed/expired metadata, resource substitution and incomplete inventory
must refuse. Do not copy the signing verification roots or private keys into
the bundle.

Present the completed public root commitments, artifact digests, bounded
artifact-admission evidence and manifest digest to the Product Owner. Installed
execution waits for that exact bootstrap receipt to be accepted. Record the
same-operator trust boundary honestly; this does not prove independent initial
delivery. Two installed Endpoint roots, public Custody/permission operations,
both Carrier journeys and actual containment/empty-scope observations remain
separate acceptance evidence.
