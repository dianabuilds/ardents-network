# Testing

`source-stopped-linux` / `make source-stopped-check` is the separate initial
stopped Source component profile. Its outside controller holds the same original
development exclusive lock, builds an immutable full source snapshot with
Go1.27.3 and canonical command flags, and supplies race-enabled owner/command
test binaries built with `network_signing_native`, `-trimpath -buildvcs=false`.
Set `ARDENTS_SOURCE_STOPPED_{COMMAND,OWNER_TEST,MANAGER_TEST,COMMAND_TEST,SOURCE_MANIFEST}` and
each `_SHA256` to independently pinned absolute original files; set
`ARDENTS_SOURCE_STOPPED_FUTURE_PARENT` to its root-controlled 0755 ext4 parent.
TMPDIR is separate root-owned 0700 ext4. The immutable snapshot is mounted
read-only as the runner's repository. The runner uses Python3.12 `-I -S` and
only its fixed `-timeout=3m` argument. Missing prerequisites fail, never skip.

Use a fresh private Ubuntu24.04/systemd255/cgroup-v2 rootfs with the exact
`/etc/ardents-source-stopped-native-stand` marker
`ardents-source-stopped-native-v1` plus LF and original manager container value
`ardents-stopped`. Before account/unit effects, the outside controller verifies
private rootfs, mount/PID/cgroup/UTS boundaries relative to its own outer stand;
the runner and test independently match caller/manager namespaces. No host
account, unit, hostname, limits or foreign workload may change. Each mutable
scenario needs its own fresh stand, never deletion/reset of a retained partial
installation to enable another run. Original driver/manager/children must join
before the shared lock returns. Record native tool versions/integrity, full
source and actual binary pins, loaded manager/account facts and bounded result
outside Git. These same-controller test inputs prove no operational trust or
independent families.

The compiled positive directly obtains initial provenance and fresh Release
proofs in its original process. Independent inventory/bytes/access/account,
empty semantic-root, selected-binding and actual stopped manager oracles check
the effects. The retained retry must preserve original journal/selection bytes.
Pure vectors independently encode loaded protection and account preimage rules,
including missing/wrong typed facts and changed native policy. Native refusal,
cancellation/Close, sync uncertainty and process-cut coverage is part of the
selected issue's acceptance, alongside the complete existing Source preflight,
artifact, Endpoint/v3 and ordinary quick/full/hook gates. A prepared test or a
narrow pass does not complete that matrix or installed Source startup.

`ARDENTS_SOURCE_STOPPED_CASE` selects exactly `compiled` (default), `owner`,
`cut`, `refusals` or `original`.
The owner case relaunches the exact signed test executable from its enrolled
inode to obtain genuine initial provenance and expose native causes locally;
it does not qualify the product command. Each needs a fresh private manager.
Native physical controls independently hold original inode admission, substitute
the born leaf, retain an independent actual Close failure, inject uncertainty
only after real fsync, and force an actual RLIMIT_FSIZE partial write. The
manager cases retain copied one-reload state, a real idle-deadline regression,
held original observation/cancellation/Close, and actual socket loss without
reconnect. None supplies fake successful Release or runtime neighbors.
The original case requires `ARDENTS_SOURCE_STOPPED_CONTROL` from
`input-substitution`, `caller-cancellation`, `semantic-content`, `hook-substitution`.
It causes actual original input replacement, SIGTERM cancellation, semantic-root
content insertion or a forbidden useradd hook after durable admission, and
requires refusal. Foreign bytes/hooks remain retained; a hook execution marker
must stay absent. The refusals case covers mismatched runtime key, existing
foreign journal and preexisting semantic root before any installation effect.
Post-journal physical controls retire admission after real intent/born fsync and
after body I/O, requiring no later birth/access promotion or ready record.
The cut case additionally requires `ARDENTS_SOURCE_STOPPED_CUT_PHASE` from
`account-intent`, `account-ready`, `directory-born`, `file-born`, `sealed`,
`selected`, `reload-intent`. Each runs in its own fresh private manager. It kills
the real compiled original process after observing the chosen durable phase,
joins that child and verifies retained-history retry refusal without journal
mutation. Its actual last complete phase may be later than the requested cut;
neither the requested phase nor child Wait proves all-descendant completion.
The outside original controller must join its complete private stand/cgroup
before running the read-only `scripts/installation/inspect-source-cut.py` with
Python `-I -S`, the original stand root and in-stand future parent. Record that
join separately. The oracle checks retained signed inputs/history, complete
recorded inode/bytes/access and empty semantic roots; torn/born-only prefixes
remain uncertain repair-required. A process-cut marker additionally requires the
actual last complete phase to equal its requested frontier; merely finding an
earlier phase in history cannot qualify the requested uncertainty class. Foreign
semantic-content controls separately require their exact inserted bytes retained.
The oracle supplies no continuation or startup right.

`source-preflight-linux` / `make source-preflight-check` is the separate checked
read-only Source installation profile. It executes actual prebuilt race binaries
under one finite private Ubuntu24.04/systemd255/cgroup-v2/native-ext4 manager,
without installing or executing Source. The outside controller owns the existing
shared `run-development.py exclusive` lock and original manager lifecycle. Use
reviewed Python3.12 `-I -S`; the runner accepts only its fixed `-timeout=3m`.

Set `ARDENTS_SOURCE_PREFLIGHT_{COMMAND,OWNER_TEST,MANAGER_TEST,COMMAND_TEST,SOURCE_MANIFEST}`
and each corresponding `_SHA256` to original absolute files. Build command with
the canonical flags, owner test from `installation/infrastructure`, manager test
from `installation/systemd` and command test from `cmd/ardents-next`; use
`-race -trimpath -buildvcs=false -tags network_signing_native` for all test
binaries. The manifest contains every exact source input. Mount the immutable
snapshot read-only as the runner's repository and retain actual Go1.27.3,
source/binary/mount/manager/namespace/account/unit observations outside Git.
`TMPDIR` is private root-owned 0700 ext4; the separate
`ARDENTS_SOURCE_PREFLIGHT_FUTURE_PARENT` is root-controlled 0755 with traversable
ancestors. No existing future root or credential delivery is a prerequisite.

The dedicated fixture root must contain the exact
`/etc/ardents-source-preflight-native-stand` marker
`ardents-source-preflight-native-v1` plus LF, and its original systemd
`/run/systemd/container` value is `ardents-preflight`. These are test-environment
guards, not product authority. Only that private rootfs receives foreign
account/group/unit/drop-in fixtures; no host account or Source unit is changed.
The real manager/client transport observes complete fixed facts and retains
original cancellation, loss, copied handle and independent Close errors. Held
last-input controls mutate original ancestor access/inode and an already-read
named input before final reobservation; late manager socket loss is refused even
without caller cancellation. Live clock replacement remains permitted under its
writer/access policy, and a full actual output pipe cannot produce success. The
compiled consumer obtains genuine signed artifacts and two fresh Release proofs
before bounded native observation. Initial and retained candidate cases remain
distinct. Independent request/template/rendering vectors and path/access/foreign
namespace refusals preserve absence of installation/runtime effects; Release
floor effects are separately retained. The runner validates complete test
discovery, exact source and every binary pin before and after execution.
Missing prerequisites or a failed case fail, never skip. Existing Endpoint/v3,
Source artifact and ordinary quick/full/hook checks remain required. Test
authorities and native observations supply no operational trust, installed
readiness, independent security review or bootstrap acceptance.

Source infrastructure artifact tests independently encode the exact descriptor
and seven-file inventory, use real ordinary Release signatures and run a canonical
compiled consumer from the identical enrolled program file. Linux command cases
cover foreign actual executable, initial/candidate separation, retained no-update,
wrong pin/local binding/target/resources/metadata/expiry, canonical/physical
inventory refusal, rollback/conflict/reopen and original caller/Close handoff.
Existing Endpoint/v3 and Release consumer positives/refusals remain separate.
The existing `network-signing-linux` profile additionally selects
`TestNetworkSigningInfrastructure*`: genuine public preparation to the compiled
initial consumer, independent byte/digest oracles, original input/output mutation,
cancellation plus native Close failure, retained partial output and a fresh
healthy counterpart. Run that complete checked native profile with root-controlled
private ext4 TMPDIR; missing prerequisites fail. Release owner race tests retain
their genuine uncertainty/cancellation/floor-publication/reopen/physical-join
controls. No test installs or starts an infrastructure role. Test signatures and
independently supplied test pins are controlled fixtures, not operational trust,
bootstrap acceptance or independent security validation.

Forwarding resource discovery partitions out the separately registered Source
profile as well as the four other receiving profiles. Its exact nine-test
inventory and execution selector remain unchanged. Source's four native
resource tests still require their own exact discovery and actual execution in
`run-source-native.sh`; a role's PASS cannot qualify another profile.

Node certificate provisioning uses the existing `network-signing-linux` native
profile and its `TestNetworkSigningNodeCertificate*` cases. The real initial
key, unsigned preparation and Record signer commands produce original inputs.
Independent standard-library DER/PEM/key/signature/interval/profile oracles and
causal wrong proof/purpose/key/Carrier/time controls examine the actual output.
The certificate purpose also runs the original input/refusal/partial-write and
killed-then-Wait-joined process-cut controls. Shared native writer/custody rules
are unchanged; their complete existing profile remains required.

`node-process-linux` includes `TestNativeNodeProvisionedCertificateTCP`, and
`node-join-linux` includes `TestNativeJoinProvisionedCertificateTCP`. Genuine
key/preparation/Record/certificate commands supply the exact original runtime
paths. The independent State fixture's canonical Record must match the actual
producer bytes. Signed State/profile and Admission/cohort authorities remain
isolated fixtures, not operational bootstrap or independent families. Actual
Forwarding traffic and JOIN duplex workload retain original process protection,
durable spend/resources and physical stop/join checks. Runtime credential and
transport mechanisms are unchanged. This proves no installed infrastructure,
operational ceremony, power-loss or anonymity qualification.

The deferred-QUIC refusal fixture still qualifies at most eight actual joint
TCP/UDP loopback candidates before any carrier assertion. After the first
allocator choice, it spreads probes across the unprivileged port range: Windows
TCP allocations can cluster inside a distinct UDP exclusion range. Each failed
candidate's original TCP listener closes before another is selected; success
requires both real sentinels. The occupied-UDP control requires that cleanup and
an independently qualified replacement. Exhaustion remains invalid environment,
never a skip or retry of a carrier operation. No host exclusion or foreign
workload is changed, and no QUIC traffic or fallback is authorized.

Source trust preparation uses the existing `network-signing-linux` native
custody profile, whose exact command selector also includes
`TestSourcePreparationNative*`. The genuine public-only command succeeds with
all three external private paths absent. Closed declarations and commitments,
explicit roots/name/purpose/pins, collisions and original public/native custody
have causal refusals. Finite output checks include final-byte/inventory mutation
after original-input I/O, independent Close/receipt failure, a real RLIMIT_FSIZE
partial write and original process cuts followed by Kill/Wait and non-adoption.
Original-reference expiry is checked before use and after held real input I/O.
An actual tmpfs original descriptor separately refuses public mount admission;
this isolates Fstatfs from the command's independent direct-ancestor refusal.
No failed attempt is erased by a restored healthy control. The independent
Source pin vector fixes the literal domain separator, zero byte and raw key;
real TLS controls separately check CA/name/server pin and CA/client pin, EKU and
validity. Offline observation does not attest runtime key access.
The public observation helper and its semantic tests compile only on Linux,
where their genuine preparer exists; the independent pin vector remains portable
with the runtime credential mechanism. No Windows production-deadcode allowance
or test-only accepting consumer is added.
`TestSourceCancellationJoinsOriginalNativeClose` holds the real original TCP
Close and requires cancellation retirement to wait for its actual completion;
normal retirement separately stops the callback without closing caller-owned
work. Fetch and serving consume this same private mechanism. A finite negative
observation is never used as evidence of physical completion.

`source-process-linux` additionally dispatches exactly
`TestNativeSourcePreparedCohort`: the actual preparer outputs start two genuine
Source processes under the unchanged Source processresource envelope. A real
client through the existing installed Network projection obtains both signed
AREP-v3 bundles, with LATEST/BY_DIGEST checks and separate State/profile
acceptance/refusal. An invalid external second key fails and physically joins
partial startup while the first Source remains separate; changed public PEM
retires the original process without hot reload; restored paths do not revive
it, and a separate new invocation readmits the restored original credential.
Original process Wait and genuine State-root reopen follow retirement. Isolated
external CA/leaf/State/Admission fixtures share one controller and prove no
independent families, operational ceremony, installed startup, power loss or
privacy qualification. The complete original Source cancellation/protection/
pressure/resource suite and Network race suite remain separately mandatory.

`source-process-linux` also discovers and executes exactly
`TestNativeSourceOriginalInputsRetire`: each original plan/certificate/key/CA
is replaced, mutated, unlinked, access-changed or loses its ancestor during
real held Source socket I/O. Fixed input refusal, original process Wait,
State-root reopen and non-revival are required. Its selected native limits and
the four separately discovered resource tests are unchanged.
Command race controls exercise each input before exposure and after real
State exposure I/O but before listener, with genuine signed State, native root
reopen and absence/removal of the original guard. Held original native reads,
concurrent/copied Check/Close, partial acquisition, late independent native
Close/output failure and held State.Close retain original custody. Original
cancellation must start actual State.Close while a native Check is still held;
the test joins that Close and then releases/joins the observation. A separate
control permits live clock-file evidence updates without changing State policy.
Another held original input Close checks the same failed completion notice,
retained key descriptor and independent late native Close failure through join.
These scheduling/fault fixtures do not supply successful process admission,
installed startup or operational authority.

The existing `network-signing-linux` profile also checks initial State/Node key
preparation through the real `prepare-initial-keys` command. It requires actual
Linux root, root-owned direct ancestors and a private 0700 native-ext4 TMPDIR.
Lower/upper counts have exact complete inventories and independent standard-library
PKCS8, seed/public, purpose, index and digest verification. Generated keys feed
the real public preparations, Record/Epoch/profile signers and State acceptance
on independent development roots; Admission authority is a separate fixture.
Wrong generated role/key and reviewed pin refuse before healthy commands.
Isolated failure seams cover generation, encoding, duplicate/inconsistent keys
and owned-buffer clearing. Native mutations, cancellation, contention, retained
physical errors, real partial writes and killed/Wait-joined original children
check incomplete-output refusal. Public receipt presence cannot convert failed
work to success. Shared physical writer changes renew preparation/signing checks.
Final readback follows original input reobservation; actual byte/inventory/link
changes at that boundary refuse, including through the existing signed-output
and profile consumers. Original cancellation and a separately failed physical
Close both survive the same refusal.
Linux declaration tests execute with their genuine production consumer; other
platforms check explicit pre-effect refusal. These controls qualify neither power loss, operational bootstrap,
independent custody/families nor installed execution.

The `network-signing-linux` profile (`make network-signing-check`,
`network_signing_native` tag) requires actual Linux root, direct root-controlled
ancestors and private 0700 TMPDIR on native ext4. It exercises the genuine
initial Record/Epoch and closed-profile signing commands with isolated supplied keys, exact public
preparation commitments, independent signature transcripts and new State
acceptance on independent roots. The positive chain uses three distinct State,
Node and Admission keys and the actual profile-signing command. Independent
canonical profile bytes and the fixed-domain transcript are compared; wrong
purpose, pin, private key, public context and State Record/role/issuer inputs
refuse with healthy controls. Profile custody and process-cut controls exercise
the actual private operation; imminent actual expiry checks follow input I/O
and output I/O without changing the original reference or six-hour grammar.
Exactly one of two original profile-signing attempts wins fresh output custody.
Original input replacement/access/cancellation refusals and deterministic
exclusive output inventory, readback, file/directory sync/Close failures,
concurrency and killed/joined original writer children are separate controls.
Process cuts establish no power-loss qualification. Missing prerequisites fail;
ordinary gates cross-compile this profile and do not imply native execution.
Signature and PKCS#8 semantic tests compile only on Linux with their genuine
production consumer. Other platforms retain explicit refusal before native
effects; no private signing helper is kept reachable only by their tests.

The `network-preparation-linux` profile (`make network-preparation-check`,
`network_preparation_native` tag) exercises the real three `ardents-next network
prepare-*` commands, exact canonical unsigned Record/Epoch/profile outputs and
independent Network fixture bytes. Isolated test-only purpose-specific signers
sign those exact outputs; actual new State accept-epoch/accept-profile and
inspection consume them on independent roots, with altered, foreign-key,
purpose-confused and signed foreign-record refusals followed by healthy controls.
Fixture families are not independent operators. These tests deliver no private
product signer, complete provisioning or installed qualification.

Portable plan tests reject duplicate/unknown/trailing/mixed declarations and
noncanonical key/identifier/time/number shapes. Native Linux ext4 tests retain
original parent/file descriptors through cancellation, exclusive creation,
write/sync/close/readback, replacement/symlink/hardlink/inventory refusal and
independent physical failures. Five child-process cuts hold the actual native
writer at directory birth, file birth, write, close and directory-sync barriers;
the parent kills and joins each original child, preserves its prefix and proves
that a new invocation refuses adoption. This is process-interruption evidence,
not power-loss or hardware durability. Selected native execution of these cases
requires a private caller-owned temporary directory on actual native ext4;
overlay/tmpfs/Windows is not an accepting native durability environment. Windows
retains portable grammar checks and explicit native-effect refusal.
The native profile also requires a real tmpfs refusal control, and an isolated
child's finite file-size limit produces a real partial write with retained EFBIG
and a non-adoptable prefix. Only that child changes its own limit. Ordinary
quick/full gates cross-compile the tagged tests; selection of the native profile
is a separate required execution, never inferred from that compilation.
The exact former unused-symbol allowances for new Network unsigned preparation
are removed: the new public commands are genuine production consumers. Remaining
local-participation diagnostic allowances confer no provisioning permission.

The registered `node-development-linux` profile invokes
`make node-development-check`: a finite three-iteration series of all five
complete original Node profiles, genuine non-test preflight, continued actual
prerequisite checks and shared own-job serialization. The runnable
[native recipe](../../scripts/node/README.md) owns source/artifact pins, external
evidence, scheduling and failure retention. Ordinary success and intentional
resource/evidence/caller/placement refusals remain distinct. It establishes a
bounded development baseline only; no installed, platform or capacity claim.

## Selected release Carrier coverage

`channel/native_refusal_test.go` independently decodes a real lower BYTES
prefix before retiring that lane during an incomplete upper frame. The upper
operation must remain failed without inventing a native cleanup failure from
successful lower output. Its paired control introduces an actual late native
write failure and requires that original cause in both lower and upper physical
results. Existing raw partial-write, deadline, close and late-failure controls
remain required; this mechanism fixture supplies no successful domain authority.

[ADR-0122](../adr/0122-select-tcp-only-closed-release.md) selects TCP/TLS alone
for the nearest closed release. Current positive Carrier/native/Service profiles
must execute TCP/TLS with all applicable causal refusal, lifetime, resource,
persistence and cleanup cases. Deferred QUIC requires explicit pre-effect refusal
and no TCP fallback; remove obsolete positive QUIC dispatch from checked profile
inventories rather than using skips or ignoring failures. Update discovery and
runner assertions together with their owning tests. Historical both-Carrier receipts describe their original candidate; current
implementation descriptions below cover the selected TCP/TLS release. Mandatory quick/full gates remain enabled. A changed source, profile
or canonical artifact requires new affected evidence; exclusion is not QUIC
security qualification or permission to change other protection parameters.

Node's `event_writer_linux_test.go` exercises the actual lifecycle writer on
saturated pipes and Unix stream sockets: finite deadline, cancellation, exact
2048-byte line delivery after drain, partial-offset continuation and retained
closed-peer write errors. The partial-write mechanism fixture deliberately
exceeds the emitter's line bound to force a positive partial socket write;
it does not expand the public event grammar. Tests own and join all fixture
actors. The Linux command emitter regression preserves
the prior diagnostic on failed output and checks exact wire/diagnostic bytes
after drain. These tests run in the existing deterministic and race profiles;
they supply no installed Node or systemd qualification.

Deferred-QUIC refusal controls exercise the retained factories and genuine new
signed State, selection, Receiver and Stock. They require the unsupported
outcome before socket acquisition or token presentation, preserve the signed
historical profile and independently observe no UDP output or TCP fallback.
The removed QUIC adapter's earlier deadline and role-observation receipts remain
historical evidence of their original source.
The retained factory probe acquires its TCP and UDP sentinels on one jointly
bindable loopback endpoint before invoking a carrier. Bounded local candidate
selection handles separate protocol reservations; exhaustion and failed handle
closure remain environment failures. An occupied-UDP control verifies candidate
rejection and TCP release. Carrier assertions and operations are never retried.

The signed-Network command fixture keeps its shared state in both checked
profiles. Only `text_worker_installed` adds receiving-root reopen controls and
their joined verification callbacks; ordinary tests have no dormant copies of
those fields. Both profiles retain genuine Network owners and their own checks.

`node/outer/retirement_queue_test.go` holds the return of a fully delivered
original frame while a sibling CLOSE remains queued, then closes the actual
pipe peer. Parent retirement must refuse that unemitted terminal without a
physical cleanup failure and still join the original writer. A late original
Write failure remains in the joined result. These scheduling controls establish
neither authenticated authority nor installed qualification.

The Descriptor original-caller post-commit fixture retains its actual committed
result until the original receiving context observes cancellation. Local caller
cancellation alone cannot establish that remote boundary: releasing the result
earlier can race real response output with CLOSE. The fixture still checks
durable Store bytes, irreversible spend, reservations through join and reopen;
actual physical deadline, write and unrelated cleanup failures remain failures.

Connection's `attachment_records_test.go` checks that joining one original
record owner waits for both its actors and retains both errors; a sibling's
completion cannot substitute. Incomplete stale/retiring actor fixtures refuse
before transport I/O and retain their own refusal without changing replacement
state. These controls supply no authenticated stream or successful recovery.
`terminal_tail_test.go` renews actual control-ledger transitions after a settled
Application result and refuses tail completion at every missing current proof.
This pure rule test supplies neither authority nor a decoded Route witness.
The installed delayed-observation refusal fixture retains the exact joined
logical Close result for its later cleanup comparison. A canceled original Job
can supply its own retirement refusal alongside cancellation; that is not a
successful Application result. After joining the original Source and Publication,
it also retains their complete work results for the same exact-result comparison.
Those errors are not normalized into success. The fixture separately requires
nil physical Stream and worker cleanup, immutable repeated Close, no late
plaintext and no completed-current worker. Unknown sibling failures still fail
the comparison.
The native `after-request-terminal-loss` case holds only the delivery of actual
authenticated request EOF to the original Publisher worker, interrupts the
original Rendezvous and resumes the same Text request/response over replacement.
Its reader/writer and control actors continue during the observation gate; it
does not assert that a particular receipt or confirmation was lost.
Route channel's active nested-deadline test delays the return of an actual lower
Write refusal, then applies deadline, interruption and original-bound updates
while its upper writer remains selected. No physical output or upper cleanup
failure may be invented. Separate raw-connection tests preserve real owned
deadline failures through ordinary, interruption, bound and control-bound uses.
These mechanism fixtures provide no successful domain authority or Service.
`deadline_effect_test.go` holds original initial/deadline/interruption/bound and
control-bound physical calls. Local sealing must remain available; late return
must not restore a longer bound. Close waits for the held effect and original
writer and retains its actual late failure. These are framing mechanism controls,
not Network/Service authentication or native platform qualification.
The issuance draft tests consume an actual signed Network view, refuse changed
profile/network/issuer/receiver/duty and expired or extended horizons, then
withdraw actual fixture clock availability after drafting. Fresh Stock must
refuse before reserving a batch or changing its finite allowance. The detached
Prefix binding in that refusal case supplies no successful transport or Job.

`publication/durable/generation_flush_linux_test.go` controls an original native
directory flush on a genuinely reserved Credential. The local commit comparison
must succeed while that write is held; original reservation/root retirement
must deny it before the write is released, while Close still waits for the
original writer. These Linux cases run with the durable package in
`route-prefix-linux`; they establish neither an accepting Instance nor installed
Connection readiness.

Instance's `binding_observation_linux_test.go` reobserves a genuinely accepted
native root and reserved Publication floor, refusing canceled callers, changed
floor bytes and a replaced Instance inode. Its held original observer controls
only retirement ordering: local denial precedes its join, and original private
material remains until that join. These cases provide no qualified Publisher,
accepting registration or successful Service Connection.
`binding_linux_test.go` additionally checks concurrent consumption of one actual
floor/private state and refusal of foreign Binding retirement even when private
backing bytes and the lifetime are shared. These original-pointer and persistence
controls grant no accepting stream or new qualified Job.
Stock's `observation_retirement_linux_test.go` holds actual fixture observations
before token consumption and before/after blind-signature verification, while
real issuance and presentation journals remain in use. Close must deny immediately
and join the original observation; a late result cannot return a token or deposit
verified stock. Fixture authority facts here do not establish signed Network or
successful Route admission.

`TestFundedSourceBootstrapAndAdmittedIssuanceTCP` exercises the production
cold Source funding composition against actual signed Network, approved holder,
blind issuer, receiving Admission and Hosting/Route owners. Exactly two finite
bootstrap batches precede fresh admitted Source channels and an ordinary issuer
exchange on TCP/TLS; quota and original physical retirement remain
checked. The fixture's second bootstrap batch funds three issuer Control
presentations and a signed-inventory Descriptor Store Control. Actual Publisher
composition seeds four issuer Controls within its original finite permission.
The next retained prefix checks one two-recipient Forward batch
without an intervening Control refill, with separate real admissions and exact remaining
quota, instead of issuing each missing Forward token in a separate exchange.
Both funded physical openings complete while a separate Stock preparation and
the command issuance gate remain held; their original tokens need no new batch.
JOIN preparation then requests only a genuinely missing signed recipient and
retains exact quota, original Source and cancellation checks. These are Stock
and transport controls, not successful Instance authentication.
Its signing/permission infrastructure and cleanup observer are fixtures,
not a qualified worker or installed Startup. Stock preparation tests separately
check original coordination, copied/canceled handles, ordered multiplicity,
foreign profile refusal and no allocation/presentation refunds.

The separate `execution-installed-publication-check` selects
`TestInstalledServicePublication` in the same tagged command binary with an
independently selected Carrier and a separate fresh installation for each Carrier.
Canonical installed commands initialize the non-exporting Instance and accept
the fixture-approved public Credential. The actual dual-role participant obtains
network-issued stock rather than test-preloaded batches, publishes a qualified
snapshot through real REGISTER/Descriptor Store ACK and withdraws without
replacing its original process. The exact profile, finite fixture allocations,
independent pins and limits are owned by
`tests/qualification/execution-installed/README.md`. This component supplies
neither a two-Endpoint exchange nor final Connection acceptance.

The separately selected `execution-installed-timing-check` retains canonical
Installation-admitted participants through actual automatic Publication refresh
and a second authenticated Text exchange. Its fifteen-minute checked profile,
token-hour prerequisite, independent Store revision proof and original invocation
checks belong to `tests/qualification/execution-installed/README.md`. Ordinary
pair limits remain unchanged; this profile does not replace the separate precise
ACK-delay/overlap Publication timing checks or complete interruption audit.

The execution-installed-check target selects the root-native
TestInstalledExecutionStartup with text_worker_installed. Its driver requires
a fresh dedicated Ubuntu24/systemd255 manager without Ardents accounts or fixed
resources. Actual installation provision and upgrade-installed consumers
create these objects and start the canonical non-root participant program; the
root test binary never becomes Endpoint MainPID. Genuine signed Epoch/profile
intake, two actual State-owned TLS servers, the original Root completion exchange,
qualified permission preparation and Stock import precede an original-invocation
permission-accepted diagnostic. The independently observed original process and
its AAI3 listener must match the same PID/UID/GID after explicit provider-budget
initialization through the installed command. The socket probe submits no
Service request and supplies no Job or authenticated stream. The original process and
kernel scope must physically join before fixture infrastructure closes. The
driver pins its test binary and nine-resource inventory independently and refuses
missing prerequisites. Signing, Source and clock infrastructure has one fixture
controller. An isolated manager and its outside driver share a cgroup namespace
whose original PID1 is at `/init.scope`; a foreign outer-unit prefix is rejected
by preflight before starting the four-minute test. This selected scenario grants
no private Service readiness, complete
recovery matrix, independent custody/time/family or whole-host qualification.

`TestExecutionSignedInstalledResources` in the Linux command profile builds the
actual canonical participant and Text programs, combines the seven maintained
packaging resources with independently encoded signed Enrollment/Release inventory,
and executes genuine initial generation byte authentication. A coherent signed
candidate advances the same retained Root and history; the older signed snapshot
must then refuse as a candidate. Its ephemeral signing
and builder facts are fixtures. It does not start a manager, supply a completion
peer or establish installed startup, containment or independent release custody.
Changing the actual Text binary with the original pin and signed inventory must
refuse before a fresh Release history directory exists.
The ordinary metadata fixture retains its separate synthetic static-byte scope.

`TestInstalledSourceInputsRetainGenuineNetworkAuthority` exercises the new command's
bound Source credential projection against two genuine State-owned TLS servers,
signed Epoch bytes and a live clock-observation file. Epoch acquisition alone must
refuse runtime authority. A real compiled `network accept-profile` consumer commits
the signed profile under the same retained participant root before an independent
reopen observes authority. A later automatic wave accepts a signed successor from
both original servers and must refuse reuse of the old profile. All infrastructure
has one fixture controller; declaration labels establish no independent families.
The credential projection's callback isolates that mechanism and supplies no
Installation lifetime. This proves neither a completion exchange nor accepting
installed startup, Execution effects, Service readiness or privacy qualification.

`make execution-lifecycle-check` selects `TestInstalledExecutionLifecycle`
under the shared fixed-worker profile driver, including its original
selected-snapshot Publisher operation and sole Text protocol reader. Its
ordinary Reader case checks the original invocation, immutable local context,
one-use Connection claim and original-caller cancellation through physical
descendant join. Setup-only invocations must refuse Reader authority. The
driver requires the exact ordinary Reader subcase to have executed and passed;
these local operations grant no Service Connection or readiness. Build its test binary
with `text_worker_installed` from `internal/successor/execution/runtime` and
install it at `/usr/lib/ardents/qualification/execution-runtime.test` in the
dedicated manager. The driver requires independent binary/unit digests through
the existing `ARDENTS_TEXT_LIFECYCLE_SHA256` and
`ARDENTS_TEXT_LIFECYCLE_UNIT_SHA256` inputs. Both local surfaces must execute
and pass with genuine installed workers, for joined preparation and a live
single-use operation. Retirement must interrupt the operation before waiting
for its join; original worker cleanup follows. The snapshot case also obtains
the sealed original Administration-context identity from its qualified operation.
Job retirement must deny operation effects while that context remains retained;
joined Job cleanup cannot close its context-completion signal. The original
generation's joined close must complete it. Detached operations and a foreign
operation refuse. These checks identify lifetime ownership, not replay acceptance.
A joined-borrower error control
and attachment-loss cases on both surfaces must execute. Loss interrupts the
operation, withholds completion until its join, preserves the failed result and
permits a fresh preparation only after successful original physical cleanup.
The joined-borrower error control requires synchronous denial of both live
local surfaces, retained first failure
and refusal of late completed-current provenance. Compilation establishes no
runtime result. This profile does not establish accepting Installation startup, live
Service work, the escape/network/recovery matrices or whole-host qualification.

`TestCleanupCollectedUnitQueryCannotReloadItsName` exercises the private worker
manager-query mechanism with a collected name's actual empty `ay` shape and
separate original-cgroup observations. Original invocation unavailability must
wait for removal proof and cannot authorize Stop by name. Its typed query and
kernel-observation fixtures establish no installed authority or native join;
genuine native lifecycle and Route profiles remain separate obligations.

The `startup_manager_native` manager mechanism case
`TestInstallationNativeFreshStartupProperties` requires actual fixed Endpoint
and both activation objects on its selected manager. Under the actual service
UID it reads both complete property rounds, then compares original signatures
and stable required values with independent busctl observations. It attests
read-only mechanism behavior, not a running installed Startup. Independent
binary vectors exercise nested lengths, duplicate dictionaries, truncation,
foreign sender, exact integer/byte projection and scalar-only reference refusal.
Its separate registered `make startup-manager-check` requires the independently
pinned root-owned systemd test binary and fresh qualification unit, a non-root
original Endpoint MainPID, both fixed activation objects and the dedicated
Ubuntu24/systemd255/cgroup-v2 manager. It does not add manager prerequisites to
the separate filesystem-only `installation_native` profile.

`make connection-nonce-check` selects `TestInstalledConnectionNonceTCP`
from the pinned tagged command binary. The genuine accepted capsule's CLOSE is
held while the original Rendezvous is physically stopped, then released.
Both roles must refuse authentication, retaining the original Jobs and Source
failure. After Publisher physical retirement, read-only inspection requires the
accepted nonce still retained. Original unused workers cancel/join without a
successful Reader result. An exterior control deleting accepted reservations on
release must fail the nonce oracle. Both Carrier PASS records are mandatory;
its result, join and tls subcases on each Carrier are all mandatory. The
`closed-peer-write` control also requires a real TCP write failure after observed
peer EOF; read errors, deadlines and unrelated joined leaves must refuse its
expected-interruption classification. `ErrDeliveryInterrupted` is accepted only
inside the genuine canceled `DeliveryResultFailure` after successful RESULT,
never as a raw supplied sentinel. These remain failed joined outcomes.
The TLS case holds
the original client's first TLS record read after ClientHello output, then stops
the same actual Rendezvous. Thus JOIN completed but neither authenticated
Attachment is returned; certificates, keys and signers remain genuine.
The RESULT case revokes the original Reader while the accepted capsule's CLOSE
writer is held after the Publisher's decoded successful RESULT. No authenticated
Attachment is returned. Publisher's independent Job remains live; its unused
authentication wait is canceled separately. Original Publisher and worker
cleanup receipts must prove join and retain any physical interruption in Close.
Receiving Close likewise retains its original error on repeated calls. Only
the deliberately induced cancellation, closed connection, EOF or exact write
reset causes are expected; unrelated joined error leaves still fail the test.
This is a component scenario, not acceptance of two installed Endpoints.

`make connection-withdrawal-check` selects `TestInstalledConnectionWithdrawalTCP`
from the pinned tagged command binary. Genuine initial Publisher bytes provide
a positive oracle before actual Publication Withdraw. Original Publication
physical borrowers must join while both original Jobs remain live; subsequent
Publisher bytes and stream acquisition refuse. Reader must refuse without
authenticated EOF, new bytes or caller-buffer mutation, retaining its logical
failure and independently clean physical join. Root and both Carrier PASS
records are required under the unchanged driver bound. This component does not
qualify remote instantaneous revocation of already delivered bytes or two hosts.

`make connection-refresh-check` selects `TestInstalledConnectionRefreshTCP`
from the independently pinned tagged command binary. After genuine initial Service
authentication, the original Publisher refreshes its accepting Publication.
Independent Store lookup and verification require an accepted Descriptor with the
same Publication digest, profile and Target, revision advanced by one and a new
slot and recipient key. The original Connection then loses its actual Rendezvous
after response bytes arrive and recovers the same request with its original
workers and previously issued finite rights. Exact snapshot bytes, clean terminal
outcomes and original physical joins remain required. The separate profile keeps
the driver's 1500-poll bound and requires the root and both Carrier PASS records;
its accepted and pending-ack subcases on each Carrier are all required. The
pending-ack case holds the actual Store response after its signed record is
written. A genuine Route capsule addressed to the new recipient must return
SubmissionRefusal status1 while both original Streams remain live; only releasing
the actual ACK permits refresh and subsequent recovery. A causal exterior
control admitting the pending pair before ACK must fail this refusal oracle.
This does not establish two installed Endpoint acceptance.

`make connection-stream-check` selects `TestInstalledConnectionStreamTCP`
from the independently pinned tagged command test binary at
`/usr/lib/ardents/qualification/execution-command.test`. Both Carrier cases
retain the original qualified Reader and snapshot Publisher, one authenticated
logical Stream and one Application request. A test observation pauses only
after an actual Stream Read returns the first response bytes, outside owner
barriers. The original receiving Rendezvous is then physically stopped and
reopened with the same duty, durable roots and owners. The response must match
the original snapshot exactly; Source, operation and workers join before the
private result is consumed. The driver requires the exact root, Carrier and
before-request-loss, before-request-terminal-commit-loss,
responder-parent-loss, source-parent-loss,
publisher-source-parent-loss, response-loss, after-request-terminal-loss, caller-observation-loss,
close-observation-loss and both Publisher observation-loss variants' PASS
records under its unchanged bound.
The request-Terminal commit case holds the original Publisher's direct
Responder authority observation after actual TLS body I/O, before that read
returns to the canonical decoder or commits the Terminal. The fixed Text
request sends one Data record then Terminal; only the second complete body is
selected. Architecture checks bind the scheduler's body-read and post-read
source sites so a source move cannot silently change the fault boundary.
The same real Rendezvous is joined and reopened before the observation resumes.
Original worker/result checks still require one exact snapshot and full
Terminal completion. Any original receiving peer-retirement failure remains
the same repeated Close result; unknown or mixed failure leaves still fail.
This supplies no record, synthetic EOF, authority or accepting result.
`make connection-authority-check` exercises both roles and Carriers after
genuine authenticated bytes, with Application Read held at a real completed
Network observation. The fault stops the exact original worker through its
existing non-root unit permission, closes a separate actual local Network State,
or waits for the originally selected safety deadline. Late bytes and EOF refuse,
caller memory stays unchanged, and original physical cleanup remains checked.
Network and time faults must not replace or revoke the Job themselves. These
are qualified component scenarios, not two independently installed Endpoints.

`make connection-control-delivery-check` selects twelve scenarios across
both senders, all three Terminal controls and TCP/TLS. In `before-receive`,
the actual immutable ticket announces its pending Write, the opposite original
reader holds with its matching accepted flag still false, and the coordinator
waits for physical Write completion before retiring the real Rendezvous. Bytes
may already be buffered by TLS; no claim of kernel-level nondelivery follows.
In `after-finish`, the original writer observes its own completed-write flag and
the opposite reader observes its accepted flag before the same physical fault.
Both schedules retain one qualified Application request and joined result.

`make connection-control-received-check` selects twelve separate scenarios:
both senders, all three Terminal controls and TCP/TLS. The original
writer is held after physical Write and before `finishWrite`; the opposite
record reader must independently observe its own accepted generation-one
control fact before the real Rendezvous is retired. That reader alone mutates
the inspected receive flags, and replacement joins it before ledger replacement.
The original qualified workers then complete the same Application request.
This checks peer acceptance before local write completion; it does not prove
a written-but-undelivered cut or full installed qualification.

`make connection-control-check` selects the separate
control-write matrix below. `make connection-reopen-check` separately selects
`TestInstalledConnectionDurableReopenTCP`: after genuine qualified
Text transfer and physical joins, it reopens the same Network, receiving,
Hosting, Descriptor Store and Publication roots. Actually accepted tokens
remain spent, refusal returns a real temporary Hosting reservation, accounting
cannot decrease, the exact Descriptor and Publication generation survive,
and the consumed Instance cannot supply a new Credential. This is orderly
owner reopen, not process-crash or two-manager installed qualification.
The control-write matrix is the
`TestInstalledConnectionControlWritesTCP` profile. It requires every
role, Terminal/receipt/confirmation kind and before-write/after-write case on
TCP/TLS. The receipt cases move here with the same assertions, alongside
the other control kinds; the Stream profile keeps its other cases.
The original 12-case control matrix plus its independent-progress regression
use a twenty-minute test timeout and
`RuntimeMaxSec=1260s` in its pinned qualification unit. Its driver allows 12600
polls at 100ms intervals, matching that runtime rather than the short lifecycle
profile's 1500 polls. This includes race-instrumented execution; individual
scenario and protocol deadlines remain unchanged.
The control write cases select the serialized writer's own immutable
generation-1 control ticket during its existing direct Source or Responder observation.
The scheduler pauses before TLS Write or after actual Write before the writer's
ledger commit, and never reads concurrently updated receipt flags. Data ACK
interleaving cannot select a different record. Replacement joins that writer
before changing its ticket owner; source-site guards bind both write boundaries.
The independent-progress case holds the original initiator Terminal writer,
delays that Reader's genuine Data ACK observation by 1200 milliseconds, and
waits for its next physical input after ACK commit before losing the actual
Rendezvous. Thus the peers retain different authenticated progress anchors.
Recovery must complete the same request while enforcing each original local
cutoff and the unchanged sealed capsule/JOIN deadline. The driver requires this
additional PASS; source-site guards also bind its record-commit and next-input
observations. It supplies neither a record nor an accepting authority.
These cases do not establish every receipt/confirmation delivery boundary.
The Stream before-request-loss case loses the original Attachment before the Text request; response-loss
loses it after genuine response bytes have reached the original Reader.
The Responder-parent case stops both real role-3 Interior candidate receivers
after initial Service authentication and reopens the same duties with their
original receiving roots and owners. The selected original physical parent
must join before a fresh generation serves the same logical Text request.
This scenario independently signs and issues a finite second set of four
role-3 Forward tokens before startup, within the unchanged Publisher allocation
cap; no grant or token is replenished after failure.
The Source-parent case adds two genuinely signed role-1 Interior members and
real receivers before startup. Original exclusion sets leave two eligible
Interior candidates for each role: Publisher and Reader keep disjoint pairs
while both still select two Entry candidates normally. Stopping both Reader
Interior receivers therefore loses its actual selected Source without retiring
Publisher. A finite second Source-token set is signed and issued beforehand;
the same logical request must finish after old physical join and new opening.
The Publisher Source-parent variant stops its separate pair instead. Its original
Publication and logical Connection remain retained while the old Source and its
dependent Responder join. Fresh generations use the same selected legs and
finite Stock issued before startup, including the additional Source and Responder
tokens; no loss permits issuance allowance enlargement or a replacement Job.
The separate caller-observation case first reads genuine authenticated bytes,
then delays return of an actual successful Network observation for that
Reader's Application Read. Test-only stack observation selects the synchronous
Read rather than an unrelated background actor; it supplies no authority or
result. While that observation is withheld, an ordinary empty Write must still
enter the same Stream's original local gate without emitting bytes. Canceling
the original Reader must cancel the logical Stream
without waiting for that observation. Its later return must expose no new
bytes or successful outcome; concurrent and repeated Close retain the same
joined cancellation result. This direct Stream refusal case does not claim a
completed Text Job or a complete authority-loss matrix.
The Close variant retires only the Connection while the same original Reader
Job remains live. A late Read must still expose no bytes; cleanup cannot revoke
or complete that Job. The fixture subsequently cancels the unused worker's own
original caller and requires its genuine physical join.
Publisher variants delay its own original Source's genuine observation instead,
using the same local-gate and late-byte oracle. Caller loss and Connection Close
remain separate; its original qualified Publisher worker and private Publication
resources must physically join without a manufactured Application completion.
An interrupted record actor may return after logical cancellation but before
the lifetime watcher obtains Stream state. That later return cannot replace the
original cancellation cause. The exact actor results and actual lower physical
cleanup failures remain retained independently, including late write failures.
This component does not
establish the complete replay/Terminal/authority matrix or two independently
installed Endpoint managers.

`make execution-route-check` selects `TestInstalledExecutionRouteTCP`
from the tagged `cmd/ardents-next` test binary installed at
`/usr/lib/ardents/qualification/execution-command.test`
and requires `service-authentication` on each selected Carrier. The separate
`service-setup-handoff` control cancels Publisher's individual setup caller after
both genuine authentications, crosses the original capsule setup cutoffs, then
performs the actual Text exchange and original physical joins. Reader's setup
caller has already been canceled by its successful authentication return. The
control preserves the original Work Safety deadline; the authority profile's
earlier deadline and the stream profile's parent-loss controls remain separate
refusal evidence.
The authentication component
uses the genuinely qualified original snapshot Publisher and ordinary Reader,
command Publication composition, synced Descriptor Store, independently
verified signed destination, finite Stock, original Source/Responder JOIN and
coalesced exact-Instance Service TLS authentication. It opens no Application
stream before genuine recovery composition. After authentication it explicitly
cancels the unused Reader, requires retained cancellation and refuses successful
Text completion; independent physical cleanup must still join without error.
Its single installed manager does
not establish two separately installed Endpoints, authenticated replacement,
text-byte exchange, Terminal completion or complete Service acceptance.

The same independently
pinned unit/binary inputs and original non-root Endpoint MainPID prerequisites
apply, with private temporary roots and loopback TCP/UDP. Publication ACK
subcases on TCP/TLS also invoke the production holder
composition under a genuine selected-snapshot operation. An authority-observation
gate holds all receiving observers after the actual public record changes;
the genuine Store and transport still produce the eventual ACK. Independent
physical record verification requires an unacknowledged initial pair to refuse
Link, and the old current to retain Link while replacement waits. Matching
actual Store bytes and joined exchange are required after release. These Link
observations do not prove capsule selection, worker/Network-loss refusal or
elapsed refresh/overlap timing. The profile requires both initial and replacement
subcases explicitly.
The `publication/delivery` case requires independent Source permission and
Stock, an actual verified Store Descriptor and HPKE capsule, and the original
qualified console Publisher on TCP/TLS. Its `accepted/concurrent` child
submits identical sealed bytes concurrently using separate actual role channels,
tokens and request nonces; exactly one matching RESULT/CLOSE succeeds and the
other receives the exact joined refusal. Its `replay` child submits identical sealed
bytes and requires only the exact typed refusal, with strict physical cleanup.
Its `retained-replay` child waits for the original capsule's actual eight-second
deadline, then submits a freshly sealed valid capsule with the same delivery
nonce. The exact refusal must survive original delivery-user retirement.
An external causal control removes only accepted-history commitment: initial
delivery and immediate replay still pass, while the later valid replay accepts
and must fail the maintained assertion. This is not full registration-expiry+60s
or worker-loss history qualification. Four finite Control rights remain within
the original two bounded bootstrap batches; no per-operation allowance grows.
The separate required `publication-delivery-close` case first acknowledges
current and predecessor through actual Store exchanges. Its `retained-replay`
child holds receiving CLOSE after the real Publisher RESULT until the original
capsule deadline, then uses the current recipient for a freshly valid same-nonce
refusal and distinct-nonce success. Original emitted-successful-RESULT failure,
interrupted child and all physical causes remain visible through owning
withdrawal and repeated Close. Peer EOF requires Transport's original read
evidence; raw, local or foreign/mixed failures cannot pass that classification.
The failed and replay Sources have separate genuine User permissions, each
bounded to its original [4,4,0] allocation and two bootstrap batches. Failed
Source cleanup stays failed after join; no Stock refund or original-caller
renewal supplies the replay oracle. Removing only accepted-history commitment
must make the same-nonce delivery accept and fail this assertion. This case
does not establish full registration-expiry+60s or worker-loss history.
The signed fixture includes a separate Rendezvous duty and issues four later
admitted Forward rights for Responder peers without expanding Bootstrap.
These required scenarios do not establish a native pass by compilation, Service
TLS, JOIN, Application bytes or the complete replay/loss/timing matrix.
The separate `publication-withdraw-ack` case first obtains a genuine current
pair, then withdraws while replacement Store output is held. Withdrawal must
join original Publisher borrowers and deny Link before the receiving gate is
released; the late flight refuses and cannot restore Link. The independent
Store retains its actual signed replacement record despite lost readiness.
Both original current/initial and pending/replacement subcases are mandatory
on each Carrier. The cancellation-order regression previously retained EOF
from the pending Registration's idle reader; current WITHDRAW and both Prefix
closes stayed clean. No failed physical output is converted to success.
The separate `publication-caller-ack` case cancels the original Publication
construction caller while the replacement response is held. It requires Link
refusal before release and after the failed flight joins, unchanged actual Store
proof, physically joined original workers and stable repeated Close. Cleanup
of the local Invocation must retain the deliberately canceled outcome; every error leaf must be exactly
`context.Canceled`. EOF, physical errors and mixed trees fail. The Invocation
must refuse completed-current provenance. Both phases on TCP/TLS remain
mandatory; this supplies no worker-loss, Network-loss or capsule-delivery proof.
After actual caller cancellation, a remote receiving fixture may retain a peer
write failure. Its repeated Close must return that exact original failed result;
the fixture records only that result, rejects unrelated or mixed causes and
does not relax the local Invocation's cancellation-only cleanup requirement.

The `publication-worker-ack` case stops the exact original Publisher worker
through the real manager while caller and Network remain live. Its held response
cannot restore Link. Invocation's read-only `CleanupResult` becomes available
only after original operation, callbacks and descendants join; full Close retains
the worker-use failure separately and denies completed-current provenance.
Its initial `accepted-history` subcase completes genuine incoming RESULT/CLOSE
before worker loss. After original Publisher, opening users and worker join,
`retained-history` observes that private accepted nonce. A live context retains
it; a failed original terminal cleanup also retains it. Successful context
retirement remains the only erasure boundary. Read-only private observation
after writer join supplies no new runtime authority or successful substitute.

`make publication-timing-check` separately selects
`TestInstalledPublicationElapsedTCP` with unchanged real clocks, the
same fixed artifact and genuine owners, a fifteen-minute test deadline and
920-second unit bound. TCP/TLS must execute delayed initial ACK,
automatic creation+300s refresh and bounded overlap subcases. Original finite
Stock and Registration bounds remain unchanged; missing time in the original
token hour is an invalid environment. The [profile owner](../../tests/qualification/publication-timing/README.md)
defines independent timing brackets, real capsule success/refusal controls,
exact retry, required evidence and physical join. It changes no ordinary
execution-route limit and establishes no full Service or privacy qualification.

The `publication-network-ack` case commits a genuinely signed conflicting
profile under the same authenticated Epoch after actual Store persistence.
Original caller stays live; genuine State refusal denies Link before and after
the held exchange joins. Both phases on TCP/TLS are required. Receiving
uses a separate genuine State root which independently verifies the same signed
Epoch/profile and stays current after Publisher conflict and exchange join.
A shared-State causal control fails that observation on TCP/TLS. Signing
and clock infrastructure still have one fixture controller. The held-record
oracle does not prove successful late ACK decoding at Source after authority loss.
Deliberate loss retains failed Network/peer outcomes through repeated Close.
Only actual observed State causes and native peer categories are recognized;
raw EOF, foreign or mixed unknown failures cannot pass by matching a message.

The required `publication-decoded-network-ack` case separately pauses Source's
genuine post-decode currentness observation through test-only call-stack
scheduling. Its actual successful Store RESULT precedes Publisher-only signed
conflict; no cached observation is supplied. Independent receiving State remains
current. The original failed exchange must retain `DescriptorAcknowledgedFailure`
with its cause, deny Link and physically join; initial and replacement phases on
TCP/TLS are required. This proves late processing of a decoded ACK, not
successful decoding of an ACK first arriving after Network loss.
Disabling only the original successful-RESULT provenance makes the replacement
oracle fail on TCP/TLS while the other native cases remain passing.

Retained TLS reads preserve their exact result while privately observing EOF on
the original completed TLS channel before local physical Close. Channel uses that
observation only for diagnostic peer retirement after its original dedicated and
orderly-role EOF decisions. No failure becomes clean completion or joined cleanup.
Real TLS peer/local-close controls and a plain pipe control verify the boundary;
disabling only that observation makes the real peer control fail. This is not
Service authentication or whole-host qualification.

The production live
holder must obtain genuine signed permission, bootstrap real stock, retire
bootstrap, spend through fresh admitted Prefix/issuer channels on TCP/TLS,
join physical owners and reopen Hosting without retained reservations. It
also stops the independently observed sole original reader while a spent Prefix
remains open. TCP/TLS must interrupt and join the holder without console
EOF or caller cancellation as the trigger; no later issuer operation may occur.
An additional worker-loss-io case on each Carrier delays the genuine ordinary
signed RESULT after real admission/spend and signing, then stops the original
reader. The holder must refuse completed output, join all borrowers and return
reservations; the ordinary issue and admission still occurred. The fixture
supplies no successful authority or tokens and joins its input/output actor.
The clock-loss-io case independently joins the clock observation updater and
retires its file while the genuine signed RESULT is held. The holder must refuse
that RESULT while its caller is still live; only then does fixture cancellation
retire the remaining local console and worker. Independently reopened State
must refuse specifically because clock confidence is unavailable. The separate
caller-loss-io case cancels the original production caller while RESULT is held.
Neither case cancels the verification context or resets an operation deadline.
Before stopping fixture listeners, the native scenario observes actual receiving
Hosting reservations and bootstrap returns until the original borrowers have
joined, within that same operation context. Holder-local join alone is not a
remote join receipt. Listener Close must not introduce a second local interruption
into the peer-loss control; all retained terminal-error assertions still apply.
After physical receiver join, every original receiving owner closes and reopens
its actual durable root. Each previously accepted token must still verify under
current signed authority and refuse specifically as already spent, with the
new real Hosting reservation returned exactly once. These checks run in all ten
cases, including late signed RESULT after worker, clock or caller loss; expiry or unavailable
authority cannot stand in for the replay refusal.
The deliberate-loss cases retain a failed receiving terminal result when the
actual Carrier reports exact peer retirement during physical output; repeated
Close must return that same result after Done. Every cause must have the native
peer-retirement classification, and unrelated or local failures remain test
failures. Positive joined retirement still requires a clean result. A retained
negative-case physical error is never reported as clean receiving shutdown.
Exact joined, worker-loss, worker-loss-io, clock-loss-io and caller-loss-io
identities on each Carrier are mandatory driver checks.
This establishes no accepting Installation startup, Service bytes or privacy claim.

The genuine issuer lost-RESULT/holder-reopen test deliberately cancels after
actual signing. It observes receiving reservations under the original Route
deadline before fixture Listener Close, requires Done and the identical repeated
Close result, and retains a failed terminal result only when every physical cause
has the adapter's exact peer-retirement classification. Its durable issuer retry,
unchanged histories and refusal to reconstruct holder authority remain required.
A selected BYTES write interrupted by that cancellation is failed physical
retirement, not clean receiving shutdown; local and unrelated failures still fail.

The same execution-route profile requires its `publication` case on each Carrier.
Production holder composition imports the selected snapshot into an original
qualified Administration worker, obtains genuine Publisher permission and Stock,
registers the actual Introduction channel and publishes its signed Descriptor to
the real receiving Store. Pending and withdrawn Links refuse. Exact retry,
explicit replacement with independent slot/key, third-recipient refusal,
physical worker/receiver join, independent Store reopen and consumed Instance
restart are mandatory assertions. External fixture Authority approval and blind
issuance provide signed public inputs, never a successful operation or ACK.
This case does not exercise private capsule/Connection acceptance or the real
300-second refresh and 60-second overlap timing boundary. Compilation alone
does not establish its native result.

`make execution-recovery-check` selects the separately pinned runtime binary's
Root `TestInstalledExecutionEndpointDeath` controller outside the Endpoint
unit. Its independently pinned temporary unit selects only the non-root
held-workers actor. Both local surfaces retain genuine qualified operations
and independently observed hostile parent/child/grandchild trees. Root matches
original manager tuples, kernel event/proc descriptors and the Endpoint pidfd,
then sends SIGKILL to that original MainPID. No Endpoint Go cleanup or Root
worker Stop may supply completion. Original Endpoint and worker scopes and
all pinned processes must disappear, with the exact failed signal/9 outcome
retained. Explicit fresh start repeats this proof with different Endpoint,
worker, local generation and Job commitments. Exact initial/restart controller
PASS receipts are required; the killed actor cannot pass. The ordinary worker's
absent hostile descendants fail the environment control. This is local physical
lifetime evidence, not logical Service recovery, installed product acceptance,
power-loss or whole-host qualification. The [profile owner](../../tests/qualification/execution-recovery/README.md)
retains its artifact, privilege and environment requirements.

Ardents separates deterministic Module checks, local process behavior,
artifact profiles, and explicitly selected qualifications. A historical result
is evidence about its exact candidate; it is not an entrypoint for current
code.

## Qualification evidence

A Qualification Evidence Bundle is an immutable, content-addressed record for
one exact candidate and its declared conditions. It retains precommitted
inputs, complete raw observations, invalidations, and deterministic verdict
outputs so the result can be recomputed. A selected log excerpt or ordinary
test report is insufficient. Public retention and change-impact requirements
remain NET-14AJ and NET-14AK in the
[requirements registry](../product/functional-map.md#accepted-requirements-registry).
An internal audit uses the separate [audit method](deep-audit.md) and cannot
substitute for independent review.

## Ordinary checks

Installation's staging, selection, Unix completion and staged-recovery filesystem
scenarios, including generation-file birth refusal cases, run independently in
parallel with distinct temporary installation roots, writer leases, original
journals, sockets and file descriptors. Their
scenario-local subcase scheduling is retained; no shared manager/account effects
are made parallel. They use one actual initial generation staging per case to isolate the sealed
provenance reader, without unrelated predecessor/fixed-resource setup. A separate
positive checks the complete original birth inventory; all nine foreign/missing
record cases perform their stated mutations. They neither mutate a shared
manager/account nor substitute native authority. Pure reload configuration matching consumes detached typed
request/selection fixtures and performs no durable setup; separate provenance
and reopen probes retain actual filesystem observations. All cases, assertions
and native terminal bounds remain in the selected profile.

Native start-intention checks cover exact durable bytes/private inode, same-inode
resync, cancellation before and after visible journal write, and socket mutation
after journal I/O. Terminal recovery checks exact optional intent/socket binding,
foreign schema/digests/noncanonical bytes and hardlinks, retaining historical
no-intention compatibility. These filesystem probes establish no manager Start,
accepting invocation or delivered ACK.

Original replacement-group prefix and conflict subcases also run independently
in parallel. Each constructs its own physical preimages, journal directory and
retained record group; no manager or account effects occur. The prefix counts,
mutations, resync checks, original-byte assertions and native timeout are retained.

Writing-generation prefix, invalid-prefix and staged-journal refusal subcases
also run independently in parallel. Each creates and retains its own candidate,
journal, lease, selection inode and original descriptors before its stated mutation.
Their reader fixture stages one real generation; the previous selection is a
detached expected byte image, without unrelated predecessor installation or
archival. It supplies no inspected predecessor, Release proof or manager fact.
Production successor staging and original predecessor custody have separate
checks. They share no live state or manager/account effects; complete inventories,
exact birth checks, refusal cases and the selected native deadline remain unchanged.

The genuine issuer exclusive-work, signed-State-loss and receiver-cancellation
scenarios in `cmd/ardents-next/route_bootstrap_linux_test.go` and receiving
Registration in `route_registration_linux_test.go` run in parallel.
Each constructs its own authenticated State, Admission/Hosting histories, holder
and reserved listeners; they share no live authority, roots or transport. Their
subcases remain sequential within each scenario, with unchanged operation bounds,
join assertions and the command package's five-minute terminal timeout. This
schedule avoids adding their independent complete matrices serially to that
package's elapsed time; it neither filters checks nor qualifies installed work.

Issuer transport scenarios in `cmd/ardents-next/route_bootstrap_linux_test.go`
and `route_process_linux_test.go` start with genuine permission but no offline
token stock. Actual holder commands obtain two bootstrap batches, refuse a third,
join bootstrap and establish fresh admitted channels before ordinary issuance on
TCP/TLS. Compiled receiving processes retain genuine new Network,
Admission and Hosting roots; their joined termination and zero outstanding
Hosting reservations are asserted. Linux is required by those durable owners,
not by bootstrap, selection rules, role framing or issuer exchange mechanics.
Portable `route/issuer` tests reject malformed operations before issuing and
contradictory envelope/payload outcomes and padding. Its shared Exchange and
Serve are used by the maintained Prefix and Receiver; generic role presentation
does not own the purpose-specific issuer protocol.
Prefix refusal tests check original cancellation before any authority or Stock
effect through both ordinary and typed BootstrapPrefix lifetimes: a refused
request leaves parent readiness intact, and Close joins physical work and
returns capacity. Done reports readiness retirement, not a physical join.
Native Stock tests use genuine blind issuance with explicitly supplied
authority facts to isolate final lifetime and post-verification observation loss.
They do not establish Network authenticity. A causal guard-removal trial must
fail these controls; retained failures are evidence, not passing skips.

`TestRouteIssuerSignedOutputBlocksPhysicalTLSUntilCancellation` uses genuine
signed Network, permission Stock, issuing Admission and Hosting with real role
TLS over an unbuffered pipe. The peer leaves RESULT unread after actual signing;
original cancellation must join that blocked write with both its physical timeout
and cancellation cause, while Hosting remains held until physical cleanup.
Independent Stock signature verification under the canceled completion guard
refuses deposit. This is an inner TLS output control, not a complete outer
TCP/TLS or QUIC operation or successful network Stock delivery. The genuine
TCP/TLS holder and receiver scenarios establish the maintained consumers.

`TestRouteSingleBootstrapIssuerJoinsTCP` isolates one genuine issuance
followed immediately by bootstrap retirement. It reuses the same genuine roots
and receiving owners as the full holder sequence, asserts one issuer operation
without private receiving Admission, joins every receiver and checks all three
bootstrap reservations return. No ordinary prefix or additional batch is needed
for its late-CREDIT failure oracle. The scenario has no timing delay or tolerated
physical write failure; a green setup or verified Stock result alone cannot pass
its retirement assertions.

Portable channel retirement controls send canonical lower EOF/CLOSE over a
pipe and gate real CREDIT completion. They distinguish clean peer termination
from raw closure, refusal, original cancellation and a missing terminal; EOF
alone cannot pass. A deliberate removal of the peer-terminal wait must fail
those controls. Shortening the actual lane horizon also interrupts an already
partially emitted CREDIT within that bound and retains its physical failure.
These mechanical tests grant no successful TLS, Network or Admission authority;
the genuine holder and compiled consumers above supply that integrated evidence.

`TestReadCreditRetirementPreservesConsumedBytesOnlyForCleanPeer` holds an actual
sibling frame in physical output while a consumed byte queues CREDIT. Exact
peer CLOSE(0) cancels that unstarted credit without turning the consumed byte
into a read failure; CLOSE(1) still refuses. Independent accounting charges
the incoming CLOSE and no canceled CREDIT. The sibling stays usable. This
portable framing control grants no TLS or Admission authority, and existing
started-write controls still require every physical CREDIT failure to survive.

`TestRoutePendingBootstrapLimitLeavesOrdinaryChildUsableOnSameCarrier` also
sends raw Node OPEN frames over actual authenticated TCP/TLS Carriers.
Missing and unknown restrictions produce peer CLOSE before TLS/bootstrap
reservation; the normal outgoing API cannot emit the unknown restriction.
The subsequent mixed Carrier holds four pending restricted children and refuses
a fifth. A genuinely issued, durably presented token on one restricted child
is refused before receiving Admission; a distinct token admits its ordinary
sibling. The refused operation retains its actual peer lane terminal and local
TLS shutdown EOF rather than being counted as a completed exchange. Both
Carrier and receiving owners join before all physical reservations return.
Removing only the restricted-child guard must admit and spend the forbidden
token and fail this control on TCP/TLS. This test provisions genuine stock
offline to isolate receiving restrictions; network stock acquisition remains
the separate permission-only holder/compiled issuer scenario above.

- `make route-check` runs only the new Network, Admission, Hosting,
  Publication, Connection, Reachability, Enrollment, Release, Installation and Route
  plus Execution local rules, installed preparation and physical mechanisms
  owners plus `ardents-next`, with Linux race execution, both selected real
  Carriers, compiled prefix/receiver processes and causal lifecycle cases.
  Packages execute sequentially because genuine Hosting measurements count
  whole interfaces: concurrent packages would spend one another's loopback
  headroom between Observe and Reserve. All package tests, internal concurrency
  and the unchanged five-minute package bound remain mandatory.
  Focused architecture checks enforce isolation and registered profiles.
  Connection includes its genuine initial recipient binding and isolated v3
  transcript/send/recovery/Terminal mechanisms. Their inclusion does not supply
  missing Service TLS, authenticated stream or installed acceptance.
  Missing Linux, counters, loopback, race compiler or durable filesystem
  prerequisites invalidate the profile. Repository quick/full gates are
  separate and remain required before integration. The bounded behavior and
  evidence limits belong to the [new Route owner](../technical/successor-route-prefix.md).

`TestNestedCloseParentRetirementBeforeOutput` queues a lower CLOSE or holds its
selected observation before physical output, then retires the original parent.
Nested cleanup must preserve the parent's original failure without inventing
a physical write failure, new output or debit. The same portable oracle runs
on Windows and Linux. `TestNestedCloseStartedPhysicalClosedErrorRetained`
requires the equal native closed-connection error from a partially written CLOSE
to retain its exact lower physical owner. These are framing controls; actual
Descriptor State/caller retirement uses genuine command composition on both
selected Carriers.

- `make unit` runs the positive deterministic package inventory, including the
  canonical Network epoch fixture, with one explicit 15-minute terminal timeout
  per package. The retained 256-stream
  Linux Endpoint setup stays in that profile; exceeding the bound fails with
  the Go runtime's goroutine dump.
- `make e2e` runs the positive local process package inventory.
- `make quick-check` runs the unit inventory, including architecture and
  formatting checks, plus vet, the four named command builds, module tidiness,
  and the canonical artifact representation proof. `make format-check` remains
  available as a focused architecture and formatting check.
  On Linux, run ordinary `unit`, `quick-check` and `check` as an unprivileged
  user: caller-owned enrollment and chmod-denial controls require that identity.
  Root is an invalid environment for those rows. Root-only Installation profiles
  run separately under their explicit selectors; they do not replace this gate.
- On Linux x86-64, `make headless-check` builds the exact Network command
  inventory, checks the enrollment-v3 artifact, runs bounded Endpoint, Source, Node, and Service
  process evidence, then rebuilds and tests the headless command candidate in
  a fresh temporary tree containing no Browser/Application implementation. It
  also proves that canonical command bytes are unchanged when the same owned
  source is represented as a Git repository or as a VCS-free extraction.
- `make check` runs unit, process, race, command build, formatting,
  Staticcheck, and vulnerability checks. On Linux it also runs the bounded
  two-host qualification fixture command tests through `fixture-network-test`;
  invoking that target elsewhere fails with an invalid environment. This is
  the pre-integration gate. Pull request CI uses `scripts/select-pr-checks.go`
  to run the changed Go owners,
  their imported consumers, and explicitly registered non-Go fixture owners;
  affected valid Fuzz declarations execute their deterministic seed corpus through
  ordinary `go test -run` in direct execution and split matrix jobs. PR selection
  does not enable mutation fuzzing, and unrelated checks remain excluded.
  declaration selection and transitive race requirements reach a monotone closure
  independently of package visitation order, including already selected consumers.
  Package and dependency loading errors fail selection before a matrix is
  published. A deleted Go package without a current owner selects the repository
  architecture check; changed surviving consumers retain their narrow checks.
  Independent selected jobs all finish and report their failures. The exact
  candidate must still pass `make check` before integration, and a push to
  `main` repeats that complete gate.
- `make staticcheck` also runs `standalone-staticcheck`: each maintained ignored
  Go entrypoint in `scripts/` is analyzed separately with `GOOS=linux`,
  `GOARCH=amd64` and `CGO_ENABLED=0`. The select-pr entrypoint uses its two
  source files together. This cross-analysis can run on a Windows host and
  does not execute or qualify Linux release operations.
- Focused Linux race checks retain raw command, stdout, stderr, and exit status
  outside Git. They may compose existing owner tests for a bounded lifecycle
  fact, but do not turn that composition into an end-to-end qualification.

Linux Node command tests can select `ARDENTS_NODE_SCOPE_PROFILE=system` to
place every serving Node in its own systemd scope. The selected profile requires
a running system manager and permission to create and stop system scopes;
missing prerequisites fail. The runner verifies the READY process's cgroup and
sole PID, then joins and verifies scope cleanup. Selected Node process jobs in
Ubuntu 24.04 PR CI use this profile because worker manager queries must not
share the Node's measured cgroup. Ordinary developer-host execution without
this selector retains its shared-cgroup limitation. Scope verification alone
does not qualify installed workers, a second Endpoint, or whole-host protection.
The complete Ubuntu CI gate selects the same profile through
`NODE_PROCESS_TEST_COMMAND`; only the Node process package uses that runner.
The other process packages and the unprivileged component checks retain their
ordinary runner. Both process invocations remain serial and run every package
in the declared process profile.
The full Ubuntu CI job has a 60-minute orchestration budget for the sequential
complete gate, including cold compiler caches and the race suite. The Go test
timeouts and required checks remain unchanged; cancellation is an incomplete
gate, never a passing result.

Installation filesystem component checks select
`ARDENTS_INSTALLATION_COMPONENT_PROFILE=root` and run
`go test -count=1 ./internal/endpoint/installation` with actual UID 0.
The selected prerequisite test fails under a non-root UID; an ordinary
unprivileged run cannot supply Root filesystem evidence. In Docker explicitly
select `--user 0` and the profile environment variable rather than inferring
privilege from the image name. Enrollment's caller-owned package test retains
its separate unprivileged profile. Root filesystem checks do not qualify a
system manager, cgroup containment or an admitted installed Ubuntu journey.
The completion-channel component also requires writable `/run` and permission
to spawn a child with kernel UID/GID 65534 and cleared supplementary groups.
The child connects through the Root-owned directory and socket, while actual
kernel permission refusals prevent private-intent reads and directory writes.
Unavailable credential-switch privileges fail this selected profile. This is
local Unix credential/access evidence, not systemd namespace qualification.

The two-Node lifecycle process test retains TCP port reservations throughout
State and plan preparation. It releases each reservation immediately before its
Source or Node starts, while other addresses remain owned. This prevents its
own ephemeral allocator from assigning a Node address to a Source. The ordinary
child bind still has a release-to-bind gap in which an external process could
claim that address; bind failures remain failures without a retry or waiver.
These reservations do not qualify installed networking or systemd/cgroup use.

- The retained text Publisher setup keeps its real 4-by-64 authenticated
  loopback composition in the ordinary Linux profile. Its race counterpart
  checks the same 256 workload bindings, retained identities, concurrent Reader
  ownership, and ready barrier at an in-memory stream seam; race-detector
  slowdown is not a wall-clock throughput verdict for the loopback fixture.
- Its serial race inventory gives each package an explicit 15-minute terminal
  timeout. This keeps the race-instrumented Linux Endpoint's cryptographic
  fixtures inside the checked profile without inheriting Go's shorter default;
  exceeding the bound remains a failure with the runtime's goroutine dump.
- `make text-role-durable-state-capture` runs the Linux-only isolated Role
  publication/lookup/withdrawal observation. It requires
  `ARDENTS_TEXT_ROLE_OBSERVATIONS` to name a writable capture directory outside
  Git; the test writes secret-bearing raw observations there and retains them
  locally. A non-Linux host or missing capture directory is an invalid profile,
  not a passing skip.
- The installed `text-command-network` runner can retain its complete raw
  output in an explicitly supplied owner-private directory outside Git. It
  prints the file path before the long-running test and keeps failures and
  timeouts for diagnosis; the caller retains candidate and host inventories
  separately.
- `make fuzz` mutation-fuzzes the selected State target for a bounded
  30 seconds. State owns canonical Epoch/Node Record framing and checks
  successful parser digest/key/raw invariants. The command fails if the named
  target is absent or fails, and does not claim coverage of every untrusted
  grammar. The deterministic architecture profile independently
  checks the selected inventory, its declarations, the Make entry point, and
  the exact mutation commands. It does not mutation-fuzz under an ordinary
  deterministic profile.

Ordinary checks do not build or run Docker and never install tools implicitly.
`make check` also verifies pinned tool versions and the dead-code inventory.
Its vulnerability check uses the [Go vulnerability database](https://go.dev/doc/security/vuln/)
and requires network access unless the needed data is already cached. On Linux,
it additionally runs `package-e2e`, which uses root to exercise `dpkg` and
`setpriv`; missing privilege invalidates that selected profile. Dedicated
Rendezvous qualifications have their own targets and prerequisites. Local
setup and hook installation belong in [CONTRIBUTING.md](../../CONTRIBUTING.md#local-setup).

## Dependency security evidence

The [dependency acceptance rule](dependencies.md#maintenance-and-vulnerability-acceptance)
covers current and future components and their continued support.
`make vuln` currently runs `govulncheck ./...` for the selected environment;
it does not by itself cover every target, tests, build tool or delivered
artifact. State exactly which of those were inspected. Review source and
test closures for the applicable `GOOS`, `GOARCH`, `CGO_ENABLED`, build tags
and Go version; use `-test` when assessing test dependencies. Inspect the exact
built artifact and its package/tool/environment inventory when qualifying
delivery. Cross-target source analysis does not execute that target's tests
or qualify its artifacts.

Record the full finding classification, including imported-package and
module-only findings, and the dated non-applicability evidence required by the
register. The pinned scanner's
[official documentation](https://pkg.go.dev/golang.org/x/vuln@v1.1.4/cmd/govulncheck)
(accessed 2026-09-07) describes build-specific coverage and limitations around
reflection, `unsafe` and binary call information. Structured output modes can
return success despite findings, so exit status alone is not a verdict for
such reports. Source checks, exact-artifact inspection, primary advisory review
and the relevant behavior tests provide complementary evidence; none proves
the absence of unknown vulnerabilities or continued upstream maintenance.
Required `make check` gates remain binding.

The successor [privacy/anonymity map](privacy-anonymity-map.md#verification-and-test-environment-map)
defines additional evidence obligations for a selected new scheme. These are
design and qualification requirements; no new execution profile or automated
anonymity verdict exists merely because the map names them.

The selected installed text Endpoint and its context, token-journal, protected
publication and read component tests compile on Linux. Run those behavior
checks in the existing Linux deterministic/race profiles; a Windows unit pass
does not execute or qualify them. Windows retains the pre-existing Endpoint
journey and an explicit unsupported text-command refusal. Shared State,
Custody, Node and protocol consumers keep their own platform contracts. The
[measured Linux Endpoint test cost](https://github.com/dianabuilds/ardents-network/blob/4764ae1c567e93180f3aa643b2544abfdb2a67dc/docs/development/endpoint-test-cost.md) records one
development baseline and the deliberate workload and Permission-hour waits;
it does not change this checked profile. The Endpoint-originating Route
bootstrap/prefix client and its stream, credit,
JOIN and network-issuance tests also execute in the Linux profiles. The
Endpoint role-network fixture names its selected Carrier, resolution, Publisher,
and JOIN roles explicitly; it substitutes accepted State and worker qualification
while starting the actual Node runtimes, Custody allocation, token stock,
Route forwarding, and role TLS. Its reserved-window variant is used only when
the child observation process inherits a Permission hour selected by its parent.
On a failed test, the fixture reports the last 16 bounded state events per
Node after their cleanup attempts. Periodic resource samples cannot displace
the transition history; the retained fields are time, kind, state, Carrier,
and reason, without resource samples or assignment material.
Receiving Node checks that do not require that client remain on both platforms. Codec
round trips, private-capsule cryptography and network tests that construct
Endpoint operations execute with the Linux client. Shared outer-handshake
admission and receiving-listener address/certificate fixtures remain separately
available to the platform-independent Node tests.
The Linux Node deterministic/race checks run actual TCP/TLS forwarding
open success, refusal, cancellation, and concurrent same-key one-HELLO cases.
The Linux parent-reader regression holds one downstream HELLO, then proves a
distinct selected TCP child and lane-zero control progress; its pending CLOSE
variant observes the first carrier close before accepting that result. Route
queue bound and Node Stop/invalidation tests remain separate owner evidence,
not one end-to-end lifecycle qualification.
Portable session checks retain blocked-HELLO/ready-unrelated progress and
independent waiter/creator cancellation ownership.
### QUIC return-path reply-budget evidence

The Product Owner's 2026-10-15 methodology decision for
`ARDENTS-QUIC-2026-09-21-RETURN-PATH-REPLY-BUDGET` permits composite evidence;
active native/OOB attack reproduction is not the sole admissible method.
This changes the proof method, not the RFC 9000 per-unvalidated-address bound,
the selected TCP/TLS plus QUIC Carrier set or the dependency acceptance rule.
The [finding disposition](dependencies.md#existing-quic-finding-return-path-reply-budget)
still requires a supported fix or enforceable reviewed non-applicability for
all affected admitted uses. The method itself supplies neither.

For a fixing candidate, retain and review all of the following:

- The actual supported upstream correction and exact maintained source,
  module, toolchain, configuration, build and canonical artifact identities.
  Establish that the built Ardents actually includes the reviewed correction;
  release notes or a similar patch title alone are insufficient.
- Deterministic causal regressions of the real dependency component at its
  actual send-admission boundary, using the independent RFC 9000 oracle and
  a control distinguishing original and corrected behavior. Cover cumulative
  per-address allowance before output and validation transitions. A separate
  behavioral model, copied implementation or first-party counter is no substitute.
- A complete caller table with source/build identity, server/client role,
  TLS/ALPN, configuration, socket/backend and supported platform. Group only
  callers whose relevant behavior is demonstrably equivalent. Prove the common
  mechanism once per equivalent group, retain controls detecting a divergent
  listener, and cover each distinct path separately. Retained predecessor and
  new isolated Route consumers have separate closure; no migration bridge or
  automatic predecessor repair follows from their inventory.
- Closure over actual output accounting, including padding, retransmission,
  segmentation, backend fallback and other send paths. Explain why these paths
  preserve the reviewed component's admitted byte count and destination. An
  unexplained backend or bypass remains unresolved; a source-only assumption
  or successful ordinary handshake cannot supply its missing proof.
- Ordinary source-matched native checks of genuine authority, Receiving/Hosting,
  original process/producer/TLS custody, real TCP/TLS and Service consumers,
  finite bounds, cancellation, late work and joined physical resource return.
  Preserve role-specific authority/spend/lifetime checks and affected race
  coverage even when a shared transport mechanism is proved once.

For non-applicability, retain the dependency owner's complete enforceable-boundary
argument and real causal regression instead of assuming an upstream repair.
Every admitted caller/platform/backend still requires explicit coverage.
Preserve failures, historical observations and exact supersession of any earlier
method. Label inspection, component execution, ordinary native integration and
direct wire/OOB measurement separately. The composite proof does not claim a
direct native packet measurement; absent measurements stay absent. Changed
dependencies, artifacts, configuration, callers or backend assumptions invalidate
affected receipts. Full required gates, the normal commit hook and scoped verified
delivery remain mandatory; passing this method's prerequisites does not close the
finding. It authorizes no active attack reproduction, host change or privileged
experiment.

## Reachability audit

Receiving Store native tests in `internal/successor/reachability` exercise actual
Linux/Windows roots and lease/directory sync, independent signed Descriptor
inputs, full retained conflict/capacity/reopen cases and original operation join.
`store_lifetime_native_test.go` delays only a real publication's foreign
observation: Lookup must progress, Close must deny and retain the original lease
until that observation returns, and pre-effect denied work must leave no floor.
Original root/records/marker/record loss permanently refuses after restoration;
fresh explicit reopen revalidates exact signed bytes. Windows can physically
block retained-object rename with sharing violation, or retained-directory rename
with access denied; the positive control requires original custody and exact
proof to remain healthy. Linux performs and detects every actual object loss.
`store_lease_linux_test.go` separately replaces/restores the actual lock path and
closes the actual original lease descriptor, checking permanent denial and the
same retained physical Close error across concurrent callers. These cases are
component native/race evidence, not command protection, power-loss, full rollback
or installed qualification. Receiver consumes the real custody check at its
existing Descriptor effect/result boundaries; TCP/TLS composition remains
separately required.

`make deadcode` runs `golang.org/x/tools/cmd/deadcode` for the maintained
Windows and Linux production builds, then for their test executables. The test
result must be empty. Production results must match
[`tests/profiles/deadcode-allowlist.json`](../../tests/profiles/deadcode-allowlist.json)
exactly: an entry names every retained symbol, its classification, rationale,
owner, and deletion condition. Prefix, package, or count-based exceptions are
not permitted.

Most retained production entries are intentionally unwired closed-alpha
tracers with deterministic behavior evidence; this is not a claim that they
are a selected product path. A newly reported symbol fails the gate until its
owner either removes it or reviews it with a concrete retirement condition.
An absent listed symbol also fails the gate, so the registry cannot silently
accumulate stale exemptions. `make check` includes this audit.

Service Connection's portable implementation has exact Windows allowances only
for functions reached by genuine Linux Reader, Publisher and installed command
composition. Linux production remains fully checked against its own inventory;
both platforms' test analysis must report no unreachable functions. Independent
Instance proof vectors exercise the same wire verifier used by both initial
response owners, and active streams consume the stream-only record parser.
Physical Source fixture constructors and scalar manager-vector wrappers remain
in test scope. Reader transfer fixtures invoke the maintained retained-completion
contract with immediate release; they provide no Execution or Service authority.

The new Route prefix has a real Linux command caller. Its TCP/TLS
mechanisms and common Carrier behavior tests are portable; actual adapter
execution on Windows does not qualify durable Route roots or an installation.
Portable Route rules and canonical framing retain their portable tests. The migration also records the
predecessor Network's tested unsigned preparation, local participation and
diagnostic/wait contracts as awaiting their genuine production composition,
with individual retirement conditions. Those entries do not qualify Node or
provisioning integration. The signed-State builder remains test composition.
The uncalled standalone direct-role listener and bootstrap decoder were removed;
the shared receiving listener is the actual Route caller.

The dedicated Initiator closure audit removed the production-dead Entry
admission engine, receiving relay grammar, and direct credential/reachability
OHTTP adapters. The User Route closure oracle additionally requires the
uncalled Open/Attach owner, private reachability exchange, and exclusive relay
and Introduction sender files and declarations to remain absent. It also names
the shared Attachment evidence and current Introduction receiver surfaces that
must remain. Endpoint and Node behavior tests may use bounded test-local
reciprocal fixtures; those fixtures do not restore production reachability.
Shared credential-relay codec leaves and the standalone reachability Relay
remain exactly classified under their current consumers in the deadcode
registry.

The old Node-leg dial, TCP/QUIC client adapters, and client confirmation
entrypoint are absent. The retained reciprocal decoder remains in its exact
reviewed grammar group. Current closed TCP/TLS tests prove exact peer
rejection, cancellation before handshake, caller-owned lifetime after a
completed handshake, and unchanged Carrier profile bounds.

Removing the old cross-platform Rendezvous listener once exposed the shared
server-side Carrier and admission closure as production-dead. The current
analysis reaches it through the composed `ardents-node node --config` command,
and ADR-0101 resolved the last two Node entries that still carried the stale
"no selected C0 command" claim: the `dutyFacts` DutyView test-seam projection
moved into test scope and the unimplemented local admission-timeout helpers
were retired, so `internal/node` holds no common deadcode allowance.
ADR-0104 subsequently replaced the getter seam itself with one copied
`state.NodeDuty` value, deleting the test-scope projection along with the
Interface it satisfied.
Additional shared framing leaves are unreachable only in the Windows
projection and remain in its platform allowance because Linux production
still has retained Route consumers. That allowance does not permit removing
or reconnecting shared mechanics in the one-engine retirement slice.

Node's original Linux Forwarding consumer similarly reaches its portable
producer, path and counter rules. Their exact Windows entries are listed as
native-composition reachability, with behavior tests retained. They grant no
Linux production exception or Windows runtime qualification. Local retirement
notice tests preserve foreign, wrapped physical and cancellation cleanup causes.

The same exact platform classification covers new Installation's portable
assembly, inspection, journal and typed manager rules reached by genuine Linux
`ardents-next installation provision/check` consumers. Windows refuses their
native effects. Their individual Windows entries grant no Linux allowance,
successful installation or runtime qualification; both test projections must
still be empty. The native custody representation is Linux-only, while portable
request and byte rules retain the same sources and behavior controls on both
platforms. Assembly separately refuses complete stored observations without
fresh private Release proofs.

The same platform rule covers the live Publication holder's exact Linux closure:
qualified snapshot operation, private Instance binding, durable generation,
Descriptor production and original REGISTER receipt. Windows retains native
refusal before snapshot file I/O; the command obtains that refusal from Execution's
own Publisher launch method. These entries confer no Linux allowance, and both
test projections must remain empty. The public verifier's newly reached Windows
functions are removed from its older exact platform entry.

The headless command inventory declares the four-command Network artifact lane;
the text-worker inventory declares its separately owned Application lane under
`tests/profiles/`. Architecture tests check actual transitive dependency
graphs, the explicit [`ownership.json`](ownership.json) registry, exact
qualification/artifact-lane ownership, and every maintained package and suite.

The AAI2 retirement closure receipt has source `go list -deps -json`, profile
`GOOS=linux GOARCH=amd64 CGO_ENABLED=0`, and command `./cmd/ardents`. Its scope
is limited to repository-owned production Go files in that one Linux command
dependency closure. The architecture gate rejects the removed
`internal/application/interfacev1/connection` dependency and every exact
`RunParticipant` identifier while requiring the selected AAI3 Connection and
Administration dependencies to remain. A temporary local module with the
forbidden import and symbol proves that the gate fails on their return. This is
not an all-platform, all-command, supply-chain, privacy, or release
qualification.

Canonical command builds use the non-overridable
`-trimpath -buildvcs=false -ldflags=-w` policy. Distributed command artifacts
omit DWARF while retaining symbol tables and Go stack/profile metadata.
Installation still freshly compares every byte of the exact signed artifact;
omitting debug sections reduces that repeated work without changing its
integrity, authority or deadline checks. Source-level debugger use requires a
separately built development artifact; it cannot substitute for the pinned
installed command. The public Make proof builds all four current
headless artifacts from two independent normal clones, a linked worktree, and
two independent VCS-free ownership extractions,
including the Endpoint source dependencies owned by the selected protected text
Connection and Administration Interfaces and text-application without changing
their ownership or the artifact lane,
then requires byte-identical
outputs without implicit `vcs.*` settings. Source revision and builder
provenance remain explicit authenticated release-metadata,
attestation, and receipt inputs; command bytes never infer those facts from the
builder's clone or linked-worktree representation.

## Preliminary Gate A1 regression coverage

The current deterministic and race inventories retain the accepted preliminary
Network authority/lifecycle repairs:

- the alpha-corpus diagnostic cannot accept or create an Endpoint floor, while
  the separately enrolled acceptance command remains the sole floor mutator;
- the enrollment package, Windows behavior tests, and both alpha-bundle scripts
  share the canonical `.exe` control-artifact identity;
- revoked, draining, or closed Connection admission reaches no State, Entry,
  issuer, Route, or Introduction work; pending capability expiry does not end
  active Endpoint work; Connection and Administration budgets are separate;
  and active sessions are canceled, drained only under a permitted finite
  non-extendable bound, and released once;
- Node rejects old-generation Entry and Transit Grant admissions immediately
  after a successor and reports terminal cleanup faults without publishing
  `WITHDRAWN`; and
- concurrent caller and Route Attachment close join one exactly-once cleanup
  and return the same terminal result; and
- Entry close cancels acquisition, joins and terminalizes active attachment
  cleanup before releasing its root, including concurrent and failure cases.

The command tests exercise the retired `accept-alpha-corpus` and
`inspect-alpha-corpus` refusals before effects. These are regression checks
for the repaired candidate, not completion of the formal deep audit or
authorization to begin its later security, concurrency, or wire tracks.

## Current profiles

`join-process-linux`, invoked by `make node-join-check`, uses three independently
SHA-256-pinned canonical command and `node_process_native` command/resource race
artifacts built from the checked source. The root driver stays outside measured
leaves. Actual Linux systemd/cgroup-v2/proc/PSI, exact JOIN CPU/memory/PID/FD/Go
limits, private durable roots and TCP/TLS are mandatory; missing
prerequisites fail. Exact discovery rejects zero/missing/extra/duplicate tests.
Each declared independent command workload group keeps three minutes and the
separate resource group ninety seconds. No profile raises native limits or
protocol allowances. It covers original duplex/EOF/CLOSE/reopen, mismatched
secret/context/unpaired refusal, undercoverage before spend, original caller/
process/clock loss after real I/O, partial startup, held actual State and
pre/post-spend effects, concurrent copied Close, signed successor/conflict/expiry,
key/producer/process loss and unavailable completion retaining original owners.
Existing Route pair/Receiver tests retain independent canonical bytes, withheld
second RESULT, third/duplicate arrivals, queued/partial I/O and late physical
failure coverage. Native development controls do not establish installed,
capacity, privacy or independent-security qualification.

`introduction-process-linux`, invoked by `make node-introduction-check`, runs
independently pinned canonical Node command and command/resource race binaries
under the original Introduction envelope in the Node owner. The driver remains
outside measured leaves and rejects missing/extra/duplicate or zero discovery.
Actual manager, cgroup-v2/proc/PSI, private durable filesystem and TCP/TLS
are mandatory; absence fails. Process workloads retain three minutes and native
resource controls ninety seconds; declared independent workload groups execute
sequentially under those same bounds. The native matrix includes direct original
caller/Stop and held genuine State observations, signed successor/conflict/expiry,
original process evidence/memory loss, History/lease loss, real pre/post-spend and durable claim-before-ACK holds,
concurrent copied Close, late native failure, partial startup and
bounded unavailable completion with retained producer conflict. The isolated HPKE holder supplies only its
declared bounded transport recipient role, never a qualified Job/Instance or
Service/readiness result. Native and full issue acceptance require their exact
source-matched receipts and all causal/lifecycle controls in the selected issue.


`descriptor-process-linux`, invoked by `make node-descriptor-check`, independently
runs the original Descriptor command under `ardents-descriptor-process-v1`, with
the exact selected sole-process Linux cgroup-v2 envelope in the Node owner.
`scripts/node/run-descriptor-native.sh` stays outside the measured leaves and
requires independently SHA-256-pinned canonical command and command/resource
race binaries built with `node_process_native` from the checked source. Actual
systemd, proc/cgroup/PSI, private durable roots and loopback TCP/UDP are mandatory;
missing prerequisites fail. Its exact discovery rejects zero/missing/extra/
duplicate undispatched tests. Resource workload remains 90s and original process
workload 3m; complete independent workload groups may execute sequentially under
those same bounds. Forwarding retains its exact seven-resource dispatch and
Issuer its separate three; Descriptor-specific tests are never relabeled as
another role's receipts. Selected development evidence is not installed,
capacity, platform, privacy or independent-security qualification.

Descriptor's CPU-pressure control also requires installed `taskset` and manager
support for bounded transient helpers. Three separate helpers run with an eight-
second manager lifetime, 50% CPU each (150% demand on one pinned CPU), 32MiB memory, two tasks and NOFILE64,
DynamicUser, NoNewPrivileges and PrivateTmp; exact own-unit stop and driver join
provide ordinary cleanup. Manager expiry bounds them even if the driver exits.
They remain outside the measured Node leaf; selected Node limits are unchanged.

`TestNativeDescriptorPartialStartupRefusalsTCP` runs the canonical
command against wrong profile, unknown field, nested roots, wrong role or TLS
key, missing Hosting and foreign Store/Receiving inputs. Every refused process
joins before separate fresh startup with corrected declarations. Unaccepted
foreign input bytes remain unchanged, no Receiving spend history is created,
and any acquired participation producer has been released. Only the fixture's
unaccepted foreign files are removed; accepted durable records are never reset.

`TestNativeDescriptorUndercoverageBeforeSpendTCP` supplies genuine
Stock/Prefix/Forwarding transport to the canonical Descriptor command with work
one byte below the required Control coverage. Refusal must leave Receiving
history and Store records unchanged. After original command join a separate
invocation with full coverage uses the same durable roots and performs real
publication and exact lookup. No token refund or accepting weaker path is added.

`TestNativeDescriptorOperationBoundariesTCP` schedules only returns of
genuine State observations on the actual request stack. It observes full native
Hosting reservation before spend, irreversible Receiving journal growth before
Store mutation, and the post-write Store observer after real record/directory
commit. Original caller denial refuses ACK, retains the full Control reservation
and original roots through delayed physical join, then independently reopens the
Store to verify absent pre-mutation proof or the exact committed proof. Signal
files coordinate the test driver; they provide no accepting authority or storage
result. These causal cases do not simulate power loss or qualify crash recovery.
The same original-work controls cover Stop, a genuine signed successor and loss
of original process protection before Receiving spend. A post-commit native Store
loss introduced only after original caller denial must retain caller provenance
plus Reachability failure, keep the reservation until join and preserve the
committed proof after separate validated reopen. Scheduling markers never claim
an accepting observation or synchronous revoke on behalf of Node.

The Linux Reachability owner control
`TestStoreNativePartialWriteRetainsOriginalFailureAndPriorFloor` runs in a
separate child and applies a real kernel file-size limit after a genuine prior
commit. It requires an actual one-byte interrupted stage, permanent original
failure after restoring the limit, retained lease until Close and a separate
validated reopen retaining the exact prior floor. A fresh explicit publication
is separate from revival of the failed owner. This tests partial-write failure,
not a successful Sync or power-loss durability.

`TestStoreProcessCrashAtActualPersistenceBoundaries` is a Linux amd64 owner
control requiring native tracing of its own child, including every original Go
thread. It stops after actual stage Write, stage Sync, Rename or directory Sync
syscall returns, kills that process and joins all traced threads before opening
a new Store. Full recovery must retain the prior proof before Rename or the
exact higher revision after Rename, discard only its validated interrupted stage
and refuse revision rollback. The measured filesystem remains mounted; this
establishes process-crash recovery, not power-loss or hardware durability. A
missing selected tracing prerequisite is a failed environment, never a skip.

`TestNativeDescriptorUnavailableCompletionRetainsOriginalProducerTCP`
occupies the real participation journal during canonical command cancellation.
The bounded unavailable report cannot release the command-owned State or exit
ahead of original cleanup. Failed producer cleanup remains failed after actual
manager/process join; process absence and a separate reopen cannot erase or
adopt the unresolved identity/family conflict. These cases exercise the command
projection separately from retained work/reservation join in the operation cases.

`issuer-process-linux`, invoked by `make node-issuer-check`, independently runs
the actual original Node Issuer under `ardents-issuer-process-v1`. The root driver,
three separately hash-pinned artifacts, actual sole-process native envelope,
missing-prerequisite refusal and race build requirements below also apply.
Its workload requires actual bootstrap, verified Stock, fresh ordinary issuance
and spending newly issued tokens on TCP/TLS. Separate original State
return scheduling controls check held key/quota/result roots and participation
through physical join, late canceled startup and authority/freshness refusal.
The Issuer resource controls exercise actual descriptor loss, Go placement and
resident-memory pressure under the distinct profile. Zero-test discovery fails.
This profile grants no installed, throughput or whole-domain qualification.
The command test artifact runs two mandatory sequential workloads, lifecycle
and issuing operations, each with the same three-minute process timeout; the
resource artifact retains its independent ninety-second timeout. Exact discovery
rejects missing, extra or duplicate command-test names before execution. All
declared cases run once under the identical native envelope. This routing keeps
new operation/partial-write coverage within a separately bounded process; it
does not change any case deadline, retry, native setting or protection threshold.

`forwarding-process-linux`, invoked by `make node-process-check`, is the bounded
new Forwarding development lane. A root driver stays outside every measured
Node cgroup. Each canonical Node process uses its original sole-process leaf,
the fixed process limits in the Node/resource owner and genuine signed-State,
token, participation and Hosting owners. TCP/TLS and deferred-QUIC refusal are required.
The driver independently pins the ordinary command and the `node_process_native`
command/resource test binaries by SHA-256, rejects missing inputs and zero-test
discovery, and retains physical manager/process completion. Test binaries may be
built with race instrumentation; that evidence must name the actual build.
Forwarding's resource process runs its original seven cases; the three new
Issuer-specific controls belong to the separate Issuer resource process.
Exact resource discovery requires all seven original Forwarding controls.
Each profile's resident-memory control runs in its own original process:
the first can leave actual native or Go memory pressure for a following control.
`ARDENTS_NODE_NATIVE_COMMAND`, `ARDENTS_NODE_NATIVE_TEST` and
`ARDENTS_NODE_RESOURCE_TEST` name the three absolute external artifact paths;
their corresponding `_SHA256` variables are independent expected digests.
Missing Linux, systemd, cgroup/PSI, privilege, artifact, durable filesystem or
loopback is an invalid environment. This lane establishes no supported Node host,
installed operation, independent controller, capacity or privacy qualification.
If the existing Ubuntu24.04 native host lacks Make, its explicit bootstrap is
`make tools-install NODE_NATIVE_HOST_TOOLS_ONLY=1` from the existing Windows
checkout or the exact root Ubuntu host. The dependency owner pins the one
authenticated package; ordinary gates do not install it. Prebuilt artifact
execution requires no second Go installation on that driver host.

`hosting-budget-linux` runs `make hosting-check`: isolated durable Hosting and
test-only Admission/Hosting I/O composition and compiled command lifecycle with race checks on Linux amd64.
It covers shared persisted samples, deadline/cancellation handoff, independent
concurrent owners, join-before-release, genuine token spending and replay refusal. The pinned Go 1.27.3
bookworm image already selected for diagnostics can provide this component
environment with read-only sources and caches outside Git. It requires actual
proc/sys counters, loopback, flock, rename, fsync and a race-capable C compiler;
non-Linux or missing prerequisites fail. This is not provider invoice,
installed Endpoint, network-role or C0 qualification evidence.

[`tests/profiles/profiles.json`](../../tests/profiles/profiles.json) is the
checked registry. Every active profile has one real Make entrypoint and exact
prerequisites. Missing Docker, binaries, privilege, platform, host input, or
artifact is an invalid environment, never a skip or passing result.
The architecture gate checks a declared timeout against that Make target's
recipe and its target dependencies, so another profile's timeout cannot satisfy
the registry entry.

The maintained local profiles are:

- `installation-native-linux`, invoked by `make installation-native-check`,
  exercises the original manager-reference transport with actual Unix I/O:
  bounded outer/inner length refusal before exposing bytes, exact validated
  frame remainder, original blocked read/write interruption and synchronous
  Close join, and late-call refusal. These physical controls provide no successful
  manager reference, Release, startup or readiness facts. Actual RefUnit and
  manager replacement/reload evidence requires a separately owned real manager.
  It also exercises the fixed-file Module through its actual Create/Replace/Commit/Close
  Interface: exclusive empty birth, detached input bytes, original parent/inode/
  access/link/ctime refusal after journal I/O, same-inode writes, original
  cancellation and kernel descriptor absence after Close. Root integration
  controls retain journal-before-payload and same-record torn-prefix repair.
  These physical fixtures grant no Release, manager or startup authority.
  also exercises the local completion Module: actual root peer credentials,
  exact/rejected/short replies, original guard/socket-record/socket substitution
  before output, retained cancellation and physical descriptor/callback join.
  Wire fixtures grant no root archival, startup, manager or runtime authority.
  The profile selects the `installation_native` Linux tests in new Installation. It requires
  a root driver and `ARDENTS_INSTALLATION_NATIVE_ROOT` naming an existing direct
  root:root temporary parent with trusted non-writable ancestors outside the
  repository. It tests real file ownership, links, inode/content/ancestor changes
  during request reading and original cancellation. Its preparation-journal
  mechanisms check durable ordered phases, refusal of retained/foreign residue
  and preservation of an actually written phase after cancellation. These
  records describe filesystem effects without granting account or recovery
  authority. The initial-root mechanism checks an actual exclusive kernel lease,
  physical release, refusal of existing roots and changed root/lock identities;
  request custody is reobserved against the original inode and bytes. The
  separately owned directory Module retains its own original caller and private
  born inode state: actual filesystem tests exercise exclusive creation, own
  sibling reuse, foreign-root refusal, same-owner/mode inode substitution,
  detached device/inode mutation and original cancellation before birth. Root
  preparation and independently admitted installed inspection are real callers;
  the Module supplies no account selection, journal/lease or startup authority. Generation
  staging checks exclusive directory birth, intent-before-write, root-owned exact
  files, complete owned inventories, retained first refusal and post-write
  cancellation without selection or lease release. Filesystem fixtures supply
  no installation of global paths. Fixed-creation mechanisms check actual
  inode records before payload, refusal without the original phase, exclusive
  file/directory birth and cancellation retaining an empty recorded leaf. They supply
  no manager completion. Access/selection components check same-inode group/mode
  promotion with the original kernel lease, pre-intent refusal, exact selected
  bytes and bounded stopped-unit observation grammar. Intent archival checks
  exact durable copy before original unlink, original cancellation with the
  pending inode retained, and refusal of premature or substituted archives.
  These filesystem records do not attest genuine manager completion.
  Installed inspection components check a retained existing kernel lease,
  original same-byte inode substitution and mutable-ancestor replacement with
  the leaf inode unchanged. Portable stored-observation controls check exact
  canonical binding, descriptor/resources and rendered declarations; public
  observation data cannot recreate Release authorization. They do not qualify
  a successful installed account/resource/manager composition.
  Portable stopped-manager controls preserve D-Bus signatures and exact integer
  observations, reject duplicate/unknown variant fields, and require configured
  Endpoint identity, protection, command/arguments/flags and an unstarted command
  history. Missing/null/wrong-type fields and weaker values refuse. The existing
  249-only absence of selectable lifetime properties is kept explicitly;
  unsupported managers remain refused. These are configuration-rule controls,
  not real loaded Endpoint, containment or installed-start qualification.
  Manager property fixtures
  are parser evidence; they do not invoke reload or qualify a stopped installation. They supply
  no Enrollment, Release or Endpoint-account authority. Missing prerequisites fail;
  this filesystem component profile supplies no systemd, installed-start or
  confinement qualification. Ordinary quick checks cross-compile its tagged
  tests into the external quality cache;
- developer, deterministic, process, package-e2e, headless-network, race, and fuzz;
- `qualification`, the aggregate selected Ubuntu Endpoint lifecycle profile;
- `endpoint-portable-ubuntu` and `endpoint-replacement-ubuntu`;
- `service-credential-response-linux`;
- `text-role-durable-state-capture`, the Linux-only isolated Role and Publisher
  durable-state observation; it retains secret-bearing raw observations only in
  an existing, writable, non-symlink directory outside the Git worktree. State,
  OS placement and pressure/owner-use are explicit component models: participants
  share a container cgroup, so its departing processes cannot supply isolated
  Role owner-use evidence. Duties still run through Node with real durable
  hosting allowance accounting. This profile does not qualify systemd/cgroup
  placement, resource use, latency or full P3 protection;
- the retired two-Endpoint alpha-control runner is no longer a contributor
  route. Its historical receipts remain in their existing evidence locations;
  its last source revision is retained in Git as `6ac9cba0856a7a0b92f2e8a4e11b246625f6716d`;
  current enrolled-bundle and inspection regressions stay with
  `headless-evidence` and its owned package tests;
- `text-worker-network`, the installed confined worker/network composition invoked
  by `make text-worker-network-check`; requires all four TCP document and elapsed-refresh cases,
  exact invocation evidence and terminal service success. State and authority setup
  remain explicit fixtures; ordinary command and full host qualification are separate.
- `text-worker-recovery`, the installed confined-worker recovery smoke invoked by
  `make text-worker-recovery-check`; it interrupts one accepted request over each
  Carrier, requires one fresh matching Route and joined worker/Grant/cgroup cleanup.
  Its two episodes do not establish NET-14 percentiles, impairment, directional
  byte/bitrate accounting, P10 restart safety or complete Route Qualification.
- `text-command-network`, the separately pinned installed ordinary-command journey
  invoked by `make text-command-network-check`; it exercises the real `ardents`,
  `ardents-custody`, `ardents-node` and `ardents-text` candidate binaries through
  TCP/TLS, with exact empty, 64 KiB and 4 MiB command cases. It supplies
  journey evidence with each Node's separate systemd scope and actual single-
  process membership verified at READY, joined empty-scope cleanup, and no
  observed topology loss of READY before completion. Orchestrator commands
  remain outside Node pressure owners. This is not continuous-duty evidence.
  It does not qualify full host, privacy, hostile-network or p95 behavior.

- `text-worker-policy`, the root-driven installed authorization matrix invoked
  by `make text-worker-policy-check`; it does not qualify the complete host.
- `text-worker-lifecycle`, the separately selected installed Endpoint launch/Grant/
  cgroup profile invoked by `make text-worker-lifecycle-check`; its tagged test binary
  and exact temporary unit must be independently pinned. No-tests success is
  refused; this profile does not replace hostile-worker or Service journey tests.
- the fixed stream-qualification worker's deterministic scheduled-byte corpus is
  checked locally for all 256 byte values (including NUL and invalid UTF-8), exact
  verification across unequal fragment boundaries, wrong-offset/corrupted-byte
  refusal and a workload-verdict failure after one-byte truncation. This is
  binary-corpus conformance for the pinned laboratory caller, not an installed
  profile result or support for arbitrary Applications.
- `text-worker-tree`, the separately pinned installed hostile child/grandchild cleanup
  profile invoked by `make text-worker-tree-check`; it requires both role receipts,
  original-cgroup cleanup and independent Publisher sibling snapshot progress. It
  does not replace the escape matrix, protected Route admission or full host
  qualification.
- `text-worker-escape`, the separately pinned installed P6/P7 escape matrix invoked
  by `make text-worker-escape-check`; it tests hostile worker access attempts under
  the effective selected unit policy and does not establish whole-host qualification.

Profile, target, directory, build-tag, environment-variable, and test names use
domain language. Historical wire, persisted, release, and artifact identities
remain unchanged where their exact bytes are a compatibility obligation.

There is no generic live or soak profile. Selecting VPS, hostile-load, soak,
platform-matrix, release, or Firefox runtime qualification requires a new exact
product claim, environment, fixture, and Product Owner decision.

## Candidate boundaries

The C0 Network candidate is exercised by the deterministic/process/race lanes,
`headless-check`, and the selected Network qualifications. This includes the
Endpoint-owned `internal/application/broker`; its directory is not a separate
Application product. Protected text AAI3 and the separate Administration v1
Interface remain covered at their selected seams. The deleted AAI2 package and
vectors are neither a generic Endpoint workload nor compatibility surface. No
Browser command, Browser implementation, Browser artifact, or Browser
qualification is part of the current candidate.

ADR-0067 retires the completed release-seed and fixed State-genesis ceremony
commands, their deterministic writers, and their separate artifact/process
profile. R-119 through R-121 retain the exact historical result; current test
inventories contain no compatibility exception or hidden replacement route for
those writers.

[ADR-0091](https://github.com/dianabuilds/ardents-network/blob/c58268d72f1eaeae173144e76d32d50986de2a77/docs/archive/adr/0091-retire-uncomposed-legacy-artifacts.md) retired the
`tests/compatibility/browser-endpoint-v4` non-executable source tree that
ADR-0061 had retained pending an explicit supersede or retirement decision.
Completed experiments, the former `reference-c2` fixture, and the retired
compatibility tree are available only from Git history and accepted research
records.

## Historical qualification provenance

Earlier RC candidate verdicts and exact evidence limits are retained in the
[archived testing snapshot](https://github.com/dianabuilds/ardents-network/blob/c58268d72f1eaeae173144e76d32d50986de2a77/docs/archive/development/testing-before-consolidation.md#historical-qualification-provenance).
They are not current profile results and cannot be aggregated into a later C0
claim. Required current profiles and candidate bounds remain in this owner.

## Ownership

Unit and single-Module integration tests live beside their implementation.
They replace real time or external I/O only at the owning seam and do not start
Docker. Cross-process tests live under `tests/e2e/<behavior>/`; they build real
named commands, observe public behavior, create fresh temporary fixtures, and
clean every owned process and file.

Every maintained Go package belongs to the deterministic inventory. Every
maintained repository file—including source/test, command, build and CI input,
documentation, packaging, profile, qualification, and retained evidence—plus
every qualification and artifact lane matches exactly one ownership rule.
Environment-owned `.codex-*`, `.git`, and editor state are not repository
inputs and are excluded from the walk. Every Go-bearing `tests/e2e` suite root
belongs to exactly one process profile. A new package, file, or suite cannot
enter through a negative filter, wildcard exception, or directory naming
alone.

Non-executable compatibility provenance lives in Git history and accepted
research records, not in the working tree. Compatibility evidence must name
its former observer and deletion/reactivation condition and does not belong to
a maintained package inventory.

## Validity and reruns

An environment-dependent run has four possible outcomes: product assertion
failure, harness defect, invalid environment, or nondeterministic/unowned
result. None is green until resolved.

Rerunning is diagnostic evidence and never erases the first failure. There is
no automatic retry, permanent quarantine, skip allowlist, or flake budget. A
timeout must leave enough observed state to distinguish product, harness, and
environment.

## Route mechanism and composition tests

`channel/eof_retirement_test.go` pins a real half-written canonical directional
EOF meeting peer/local CLOSE, continued sibling work, refusal retention,
earlier original timeout and late physical failure. A separate held real
deadline operation proves zero new EOF output after decoded peer CLOSE.
These portable pipe controls supply no successful Network/TLS/Admission facts;
the actual original Node JOIN native workload separately carries duplex bytes,
both EOFs and typed CLOSE through genuine TCP/TLS parents.


Prefix's bounded lifetime-borrow test drives an actual pipe generation through
quiet idle events, original caller cancellation and a held user join. It checks
that live ownership prevents idle retirement and that original readers/resources
remain retained until the user joins, including its terminal failure. Its virtual
clock controls mechanics only; real elapsed 300s refresh and 60s overlap still
require genuine Publication evidence with unchanged clocks.

Receiving Introduction pipe/accounting tests separately check original byte
debits, writer-waiter capacity, retained channel-local nonces and a matching
refusal RESULT/CLOSE with a held physical writer. Original retirement waits for
that writer and preserves its late failure after earlier cancellation. They
provide no durable slot claim, admitted channel, qualified Publisher or successful
recipient authority; genuine TCP/TLS submission remains separate evidence.

Route tests state which guarantee they establish. Mechanism tests control
physical I/O, scheduling or completion; they supply no successful Network,
Admission, Hosting or Service authority. Genuine composition tests call those
new owners through the real command composition and TCP/TLS.
Neither category substitutes for the other.

| Rule owner and causal guarantee | Current tests and execution scope |
|---|---|
| Channel framing lifecycle: queued cancellation preserves siblings, partial output retires its physical owner, original deadlines deny later output, readers/writers join before capacity return | `internal/successor/route/channel/session_lifecycle_test.go`; portable controlled physical I/O. The mechanics move with the channel owner; current location grants no Linux dependency |
| Nested framing retirement: only the exact lower lane's unchanged output witness and clean peer CLOSE can establish an unnecessary unemitted control; partial output and raw EOF retain failure | `internal/successor/route/channel/framing_retirement_test.go`, `credit_retirement_test.go` and dedicated JOIN relay controls. Session and relay use the same portable physical-output mechanism; the nested TLS oracle uses standard TLS with the lower transport authentication configuration, without importing a concrete adapter |
| Scheduler fairness: control alternates with ready data and data proceeds by lane order | `internal/successor/route/channel/frame_schedule_test.go`; portable, independently inspected output order |
| Principal memory: failed memory claims consume no child slot, exact 1024-child ceiling, shared control/child memory and concurrent exactly-once return; stale returns cannot release a replacement | `internal/successor/route/channel/budget_test.go`; portable actual Budget interface, with no successful authority or reservation fixture |
| OPEN: immutable setup/original caller bounds, monotonic IDs and cancelled waiter isolation | `internal/successor/route/channel/open_lane_test.go`; portable framing checks, no receiving authority |
| Parent allowance: complete ADMIT/ACCEPT debits, replacement preserving concurrent usage, causal pre-effect refusal | `internal/successor/route/channel/parent_allowance_test.go`; portable channel checks, no successful spend fixture |
| Parent exchange: original caller refusal, exact wire debits and replacement, capacity refusal before output, concurrent child progress and invalid/missing ACK retirement | `internal/successor/route/channel/parent_exchange_test.go`; portable real pipe and controlled I/O. `prefix/presentation_test.go` separately checks the actual upper presentation composition refuses malformed token length or forbidden purpose before output on Windows and Linux; it supplies no successful authority |
| Dedicated input: prepared refusal starts no reader, accepted bytes survive retirement, raw EOF cannot replace inner CLOSE and peer refusal remains visible | `internal/successor/route/channel/join_input_test.go`; portable framing. Actual paired RESULT-before-data ordering remains with the upper JOIN pair tests |
| CREDIT retirement: queued controls remain unemitted; started controls retain original cleanup bounds, late failures and capacity until join | `internal/successor/route/channel/credit_close_test.go` and `credit_retirement_test.go`; portable actual pipe output and controlled completion, without authority substitutes |
| Dedicated framing termination: local CLOSE still requires the peer's inner CLOSE; incoming CLOSE interrupts payload output and joins its result before release | `internal/successor/route/channel/dedicated_close_test.go` and `peer_close_writer_test.go`; portable physical I/O, including raw EOF and refusal controls |
| Paired JOIN terminal race: an opposite seal preserves an already started valid CLOSE and its refusal status, while denying payload and new input | `internal/successor/route/join/relay_test.go`, `TestJoinRelaySealKeepsAlreadyStartedOppositeTerminal`; actual pipe header barrier. The same receiving pairing and relay sources execute on Windows and Linux without native dependencies |
| Retained selection rules: exact original profile/duties, observation expiry floor, context-local choice retention and conflict exclusions | `internal/successor/route/selection/entry_set_test.go`, `leg_test.go` and `rendezvous_test.go`; actual execution on Windows and Linux. Supplied-fact Network models test local rules, not authenticated intake or durable-root qualification. Native durable owners have separate Linux tests |
| Network observations consumed by Route: profile/member expiry during durable reads, older caller time cannot revive a duty, and concurrently published Epoch/profile/member facts cannot mix generations | `internal/successor/network/state/closed_runtime_view_test.go` and `membership_transition_test.go`; actual new State storage and signed-generation controls. These scenarios verify particular input guarantees; migrated source and earlier acceptance do not establish general absence of Network defects. Route's genuine command tests separately reobserve authority around transport and irreversible effects |
| Introduction snapshot format: independently specified canonical bytes, exact binding/generation, sorted distinct hashes, original expiry/time floor and damaged/rebound refusal | `internal/successor/route/introduction/slot_snapshot_test.go`; portable format tests. Actual durable History consumes this same codec; its Linux lease/sync/reopen controls remain in `slot_history_linux_test.go`. Codec success supplies no durable ACK or registration authority |
| Original Introduction native custody and short denial: root/lease/snapshot loss and restoration, identical native substitution and same-inode byte restoration permanently refuse; Stop returns during held actual durable replacement while Close retains lease until writer join and separate reopen retains the late claim | `internal/successor/route/introduction/history_custody_linux_test.go`; actual independently leased Linux roots, original staging/snapshot descriptors, root inotify witness and write/sync/rename/readback. Replacement-boundary controls substitute identical bytes after actual Rename or move/restore the root before pin publication; the failed original retains its cause and visible durable claim through separate strict reopen. Scheduling changes only a real transaction's return; no accepting storage or authority fixture. Full receiving/holder TCP/TLS regressions prepare Registry before Receiver and close it after physical join. Process-crash and power-loss evidence remain distinct obligations |
| Native Introduction partial write and actual process-crash boundaries | `introduction/history_write_linux_test.go` changes only its child kernel file-size limit, requires an actual one-byte retained stage, original permanent failure/lease retention and unchanged prior floor. `history_crash_linux_amd64_test.go` traces every original child thread and kills only after actual Write, Sync, Rename or directory Sync return. Before Rename strict reopen refuses and preserves the stage; after Rename independent reopen retains exact canonical claim/time floors. Linux amd64 tracing is mandatory for this control; missing prerequisite fails. These controls neither simulate successful storage nor qualify power loss/full rollback |
| Role channel binding: reobserve original duty/profile without cached approval or replacement; refuse changed HELLO identities, purpose and absolute bounds, preserve observation errors and known Node/key/family exclusions | `internal/successor/route/role/authority_test.go`, same source on Windows and Linux. Supplied-fact observations qualify the binding rule only; actual signed intake, admitted REGISTER and paired JOIN remain genuine command scenarios |
| Shared holder handshake: negotiated exporter required before presentation; original caller cancellation after presentation refuses ADMIT even while derived physical context remains live; refused bytes are cleared and original initial-frame accounting is retained | `internal/successor/route/role/presentation_test.go`, same actual TLS refusal oracle on Windows and Linux, without an accepting Stock/Grant substitute. Genuine Stock, receiving spend and reservation transfer remain separate command tests |
| Portable directed-pair pool: canceled waiters do not cancel the original dial; Close joins late dial and preserves late physical failures/returns | `internal/successor/route/receiver/carrier_pool_test.go`; same actual physical lifecycle tests execute Windows/Linux. Receiver listen/admission composition still depends on native roots |
| Prefix pending setup: Seal interrupts original setup and refuses late handoff; still-held setup transfers once to an exact-generation borrow; repeated completion does not return published physical work before join | `internal/successor/route/prefix/setup_test.go`, deterministic scheduling with actual pipe parents. Explicit Close joins readers before resource return; separate `prefix/retirement_test.go` retains its stronger autonomous idle Done ordering. The same portable tests execute on Windows and Linux and grant no Network/Admission/registration authority |
| Exact Source/Responder pair handoff: either original Seal refuses; both generation locks remain held through the bounded local transition | `internal/successor/route/prefix/borrowing_test.go`, portable pipe parents and an explicit held-lock barrier. Mutex waits cannot use `synctest.Wait`. The same Prefix owner tests pin parent-before-stream locking and original-caller refusal after role-lock waits; acquisition tests separately pin their original caller and opening join; genuine context/paired JOIN remains command composition |
| Prefix terminal setup: an already stopped AfterFunc cannot block later JOIN or Registration physical retirement; a running callback must join before physical cleanup and capacity return, preserving the late Close failure | `internal/successor/route/prefix/join_channel_test.go` and `prefix/introduction_channel_test.go`; real context callbacks with controlled physical completion on Windows and Linux, without successful authority or RESULT. Both terminals and physical Prefix opening use the same once-owned stop/join. Acquisition cleanup tests in `join/context_linux_test.go` retain late failure before opening completion with the concrete native Context owner; actual admitted REGISTER/WITHDRAW, paired JOIN and compiled consumers remain separate TCP/TLS scenarios |
| Joined inner/lower retirement: authenticated terminal precedes original lower-lane retirement and control return; raw EOF and original/cleanup expiry retain failure, with idempotent joined return | `internal/successor/route/join/stream_test.go`; actual nested framing over pipes on Windows and Linux. Prefix terminal owns the lower lane/control hold; Joined owns the inner session and terminal wait. Native Context selection is not part of this mechanical fixture; genuine admitted paired consumers remain separate TCP/TLS scenarios |
| Exact JOIN acquisition: original Source binding, Source-before-Responder-before-stream locking, original caller refusal after lock waits, and physical opening joined before returning either borrow | `internal/successor/route/prefix/borrowing_test.go` pins exact binding and locking; `internal/successor/route/join/acquisition_test.go` pins caller and join/return lifetime; identical local lifetime controls on Windows and Linux, without successful Network/admission/RESULT substitutes. Client exchange and joined stream are portable. Genuine stocked Responder replacement refusal and two-sided compiled JOIN exercise the same free acquisition operations through real Linux command composition |
| Receiving Introduction exchange: original canceled caller refuses before channel processing, and plain stream without negotiated exporter cannot read REGISTER or write RESULT | `internal/successor/route/introduction/receiving_test.go`, portable refusal-only owners on Windows/Linux. Genuine REGISTER/WITHDRAW and compiled consumers invoke the same Registry exchange with signed Network, real stock/spend/Hosting and independent durable History on TCP/TLS; receiving composition joins connection before Grant/capacity return |
| Original Prefix caller at terminal setup/observation: actual cancellation with delayed derived propagation refuses before a new setup/Network read; cancellation during a failed read preserves both errors and retains parent capacity until physical join | `internal/successor/route/prefix/introduction_channel_test.go`, `TestRegistrationRetiredPrefixCallerRefusesBeforeEffects`, and `prefix/borrowing_test.go`, `TestPrefixOriginalObservationRechecksCallerAfterRead`; portable real pipe parents and refusal-only observations on Windows/Linux. Registration caller and original Prefix caller are independent. The unchanged successful signed-authority/presentation/receiving scenarios remain genuine command composition |
| Prefix Introduction opening refuses an already retired original caller before Network observation or Stock presentation; a failed original observation completes its pending setup without retiring live parents, which return capacity only after physical join | `internal/successor/route/prefix/introduction_channel_test.go`; portable actual pipe parents and failure-only callbacks. Successful authority, irreversible spend and REGISTER/WITHDRAW ACK remain genuine TCP/TLS command scenarios |
| Introduction capacity ownership: one independently leased History transfers to one Registry; a duplicate cannot acquire a second pending counter, exceed 1024 positions before spend or change retained bytes | `internal/successor/route/introduction/registry_linux_test.go`, `TestRegistryHistoryCannotAcquireSecondPendingCapacityOwner`; actual durable root and pending claims without Admission spend. The ownership rule is independent of the native file mechanism |
| Portable History terminal rules: absent retained storage cannot publish a Registry or claim; an original verification failure prevents further capacity/commit and joined Close preserves original verification/release errors exactly once | `internal/successor/route/introduction/slot_history_test.go`; failure-only private storage controls never acknowledge a root or commit bytes. Positive durable and uncertain-write scenarios remain on the actual native file adapter |
| Terminal error provenance: every aggregate leaf remains visible and a physical close may use only its own owner's actual write witness | `internal/successor/route/channel/credit_retirement_test.go`; portable tests of `terminal_outcome.go`, including nested physical owners. Common classification imports only transport; native QUIC recognition belongs to its adapter |
| Original parent cancellation denies new child deadline effects before the framing retirement callback runs; physical join still retains the exact late writer failure | `internal/successor/route/channel/session_lifecycle_test.go`, `TestSessionLifecycleOriginalCancellationDeniesDeadlineBeforeRetirementCallback`; prepared physical reading isolates the unscheduled callback, an actual started writer remains gated through socket close, and deadline/child-close controls execute on Windows and Linux. Live Registration expiry tests qualify the admitted composition separately |
| Inner role TLS: exact server key/certificate times, TLS 1.3 and accepted ALPN, absence of client identity, matching authenticated exporters and actual ordered bytes; invalid original bounds refuse before output | `internal/successor/route/transport/tls/role_test.go`; portable real TLS over pipes. Shared authentication also serves the QUIC handshake; this test establishes no successful Network observation or Admission right |
| TCP transport ownership: shared listener classifies direct and mutual Node identities before payload, both carry actual ordered bytes, and a wrong selected profile refuses before dial | `internal/successor/route/transport/tls/listener_test.go`; portable real TCP/TLS, using the same production listener and dial as Route. Certificate fixtures establish only the stated transport mechanics |
| Native TCP output admission: a record refused by an already closed original socket performs no native write; a started write interrupted by Close retains its original physical failure | `internal/successor/route/transport/tls/native_output_test.go`; actual pipe sockets and independently counted underlying writes. Common `transport/write_refusal_test.go` denies the marker for raw or mixed failures and prevents peer classification. These are mechanism controls, not qualified authority or complete TLS/Route acceptance |
| Nested physical output: an exact unstarted lower refusal remains failure without advancing either session's physical witness; the selected writer publishes counts and failure before becoming inactive | `internal/successor/route/channel/native_refusal_test.go`; actual nested framing owners with a failure-only lower connection, no successful authority. An independent pipe reader observes the first byte of a partial direct frame before a later native refusal; that earlier output and raw/mixed physical failures remain physical. Retained Close results remain failures. Genuine native TCP/TLS composition is separate evidence |
| Introduction recipient reobservation preserves the exact original cancellation, physical failure and mixed cause without inventing a duty mismatch | `internal/successor/route/prefix/introduction_channel_test.go`, `TestRegistrationRecipientObservationRetainsOriginalFailure`; failure-only observations exercise the same member-check owner used by actual registration. Successful signed authority and ACK remain genuine command scenarios |
| Handshake failure category: a refused peer cannot hide an unrelated listener or cleanup failure in a joined error | `internal/successor/route/transport/handshake_failure_test.go`; portable complete-tree classification, with original error causes retained. Receiving composition uses this category to decide whether its listener remains usable |
| Shared actual-adapter contract: TCP/TLS classifies direct/Node arrivals, exchange ordered bytes, bound handshake capacity and join readers interrupted by physical Close | `internal/successor/route/prefix/adapter_contract_test.go` and `adapter_retirement_test.go`, external tests on Windows/Linux. The actual opening/composition owner hosts this external contract matrix and imports the real TCP/TLS adapter; lower transport imports neither, including in its tests. Prefix and receiver select the exact profile at their real opening/listening operations; the former carrier package has no runtime or test facade |
| Actual public refill and paired JOIN: signed/current Network, durable Stock/spend, real Hosting and TCP/TLS | `cmd/ardents-next/route_prefix_caller_linux_test.go`, `route_refill_linux_test.go`, `route_join_linux_test.go` and their real command fixtures; Linux durable composition prerequisites remain explicit |
| Prefix original-generation retention: idle retirement of a stopped terminal borrower waits for its physical join before parent readers and resource return, preserves its late failure and repeated result | `internal/successor/route/prefix/borrowing_test.go`; controlled borrower completion with actual parent pipe readers. No successful authority or Registration is supplied. The actual Prefix mechanics and these tests are portable on Windows and Linux; Prefix is the actual generation owner |

These are behavior-owner links, not a second task ledger or an installed
qualification claim. Test filenames follow their rule owner when extraction
changes its package. A mechanically injected failure establishes the named
ordering guarantee; it cannot establish successful authenticated admission,
publication readiness or recipient acknowledgement.

`route/capsule/envelope_test.go` independently encodes the accepted private
offsets and checks exact 344/360/474-byte bounds, overflow, fixed domain/header
transcript and actual HPKE tampering refusals. Instance's
`capsule_opening_test.go` opens the static RFC9180 base-mode selected-suite
sequence-zero vector from CIRCL v1.6.5's pinned RFC corpus, matching Go's published
key/encapsulation/info; this is suite evidence, not live authority.
`recipient_lifetime_test.go` retains an original failure-only user across Close
and requires key/capacity erasure after its join. Genuine signed Network,
host-generated Instance and TCP/TLS REGISTER composition in
`cmd/ardents-next/publication_registration_linux_test.go` exercises the same
borrowed fixed opening and exact foreign-binding refusal. Candidate Rendezvous,
Connection and Work Safety fields in that scenario remain unaccepted input;
this evidence establishes no replay commit or successful downstream delivery.
The same genuine control shortens the recipient's local cutoff while original
registration, signed Descriptor and capsule expiry remain valid. A later limit
cannot extend it; new openings and retained candidate reads must both refuse
without waiting for physical retirement. Actual root close separately interrupts
the original opening and retains custody until that user returns. The short
cutoff control proves the refusal mechanism, not real elapsed overlap timing.
The genuine Registration `caller-cancel` case joins Close immediately after
canceling its original caller, before waiting for the asynchronous retirement
signal. TCP/TLS must retain that cancellation and the same repeated Close
result; successful owning withdrawal remains a separate positive control.
Receiving JOIN matching, setup/HELLO bounds, the two actual RESULT writes before
data, capacity retention, cancellation and both-side interruption live in
`internal/successor/route/join/pair_test.go`. Relay credit, forbidden refill,
full header/setup debits and selected CREDIT cleanup live in `relay_test.go`.
Both groups execute on Windows and Linux. They isolate physical post-admission
mechanics and supply no accepting Network/Admission/Hosting authority.
`receiver/join_failure_linux_test.go` additionally checks that the real
Receiver retains deadline and partial-terminal failures through JOIN's public
interface; its native receiving dependencies justify Linux composition.
Genuine TCP/TLS admitted and compiled JOIN scenarios remain in
`cmd/ardents-next` and the full new-domain profile.

Original WITHDRAW caller cancellation has two genuine TCP/TLS controls in
`cmd/ardents-next/route_registration_linux_test.go`: cancellation just after a
live caller observation with its callback delayed, and cancellation while its
callback stop completes after the real Registry ACK. Both require canceled
operation and retained Close results, exactly one callback stop, joined cleanup
and unchanged genuine registration/spend counts. The ordinary successful
WITHDRAW remains a separate positive control. These cases use signed State,
real Stock/spend/Hosting roots and receiving Registry; their native dependencies
justify Linux execution. They do not supply successful authority or ACK.

## Test design and retirement

Platform-specific tests require a concrete mechanism justification in their
owning source or technical owner: name the native operation or guarantee, the
behavior being checked, and why the common test cannot exercise it. A filename,
an inherited build tag, or the current platform of an enclosing package is not
that justification. Common framing, queues, credit, byte accounting,
cancellation and joined lifecycle use the same behavioral oracles on Windows
and Linux. A test isolated only because these mechanisms are coupled to a
platform adapter must move with their portable owner rather than retain that
coupling as an accepted platform requirement.

Native socket-error controls may distinguish Winsock from POSIX errno values.
Durable integration tests may require the selected exclusive file lease,
permission checks and directory-sync/rename guarantees. Keep such mechanisms
with their actual owners and do not replace unavailable guarantees with a
successful in-memory fixture. Portable component execution, cross-compilation,
full new-domain integration and installed qualification remain separate evidence.

Stateful Modules own injectable wall-clock, monotonic duration, entropy, and
private fault seams at the smallest real owner. Fixed sleeps are not readiness
or eventual-consistency oracles. Every goroutine, process, listener, file, lock,
timer, and fixture has one owner, cancellation path, join, residue assertion,
and cleanup-failure path.

The fixed stream qualification uses a qualification-only Run seam: its report,
observer callbacks, sampling cleanup and joined streams never belong to an
ordinary text Job. Its cancellation oracle delays an old Run's completion
until a replacement exists, then proves the completion remains on the old Run
and that the first sampling cleanup error is immutable.

Two tests are duplicate-removal candidates only when requirement, owning seam,
oracle, transition/fault, platform/format, and independence role are all the
same. A seam migration moves behavior tests to the new Module Interface,
retains only higher-level tests that prove a distinct fact, and retires obsolete
fixtures and runners in the same change.

Security vectors are deterministic. Shuffle and randomized schedules publish
their seed. Fuzz/property coverage belongs beside each retained untrusted
decoder and canonical encoder. Race coverage is required wherever a Module owns
goroutines, callbacks, locks, mutable admission, cancellation, or durable state,
and must assert terminal join and state invariants in addition to race-detector
cleanliness.

The closed text Endpoint's durable Entry Set owner and its persistence and
class-specific adjacency tests compile on Linux, matching their sole production
consumer. Legacy Entry Invite validation and retained admission-history
decoding remain portable; receiving-admission checks were retired with the old
Initiator closure.

Holder-request construction and its end-to-end Custody allocation tests run on
Linux with the text Endpoint client. The shared request decoder, proof verifier,
Custody command and authority storage remain portable; independent authority
creation and allocation-journal tests stay in the portable profile.

The closed blind-token client batch and result round-trip tests run on Linux
with the Endpoint that retains the opaque pending state. Issuer request parsing,
signature checks and fixed result encoding remain available to receiving Nodes.

Private Descriptor issuance round trips and Store tests that produce those
proofs run on Linux with the Publisher. Independent receiving Gateway and wire
checks remain portable. Protected context and coalesced initial-authentication
tests run with the Linux text Endpoint; shared stream lifecycle tests remain
portable, including the common receipt-consumption representation.

Snapshot construction/response and published-Link client round trips execute on
Linux with their text command consumer. Shared document-read and raw published-Link
server framing tests remain portable; their failure checks are unchanged.

AAI3 server admission, lifecycle, cleanup and request round trips execute on
Linux with the Endpoint. Portable client cancellation/close tests retain their
independent socket peer and explicit terminal-frame fixture; they do not require
the selected Linux server to produce a response. The public AAI3 client/server
conformance oracle writes every byte value exactly once through unequal input
fragments, closes only the input direction, and then reads the reversed bytes
through different response fragments. Existing client tests retain cancellation,
concurrent close, and Write-before-CloseInput ordering coverage; request tests
retain refused Name and explicitly send a complete AAI2 request plus data frame
to prove refusal before the Application owner is called. This mapping does not
retain the removed AAI2 test suite or claim generic Endpoint workloads.

The headless command decoder rejects both a previously valid
`ardents-headless-runtime-v1` plan and mixed v1/v2 fields before creating any
named root or socket or emitting runtime output. A separate dispatch oracle
shows valid and broken v2 plans reach the selected v2 runtime or decoder refusal
without returning the retired-v1 outcome. The command dispatch and plan-decoder
tests exercise the selected runtime and the retired-v1 refusal.

The exact legacy `endpoint open` syntax returns its stable retirement error at
the command adapter. Its regression supplies missing and existing file paths
and an available Unix socket, then proves no output creation or mutation and no
IPC acceptance. The removed accepting client fixtures no longer qualify AAI2;
the AAI2 codec/server/client and exclusive Endpoint adapter are absent, while
the independent Administration client round trip remains covered. Shared
directional-close evidence remains with AAI3 and native Service Connection.

The `ardents-node node --config` retirement oracle submits each old
`rendezvous`, `initiator`, `introduction`, `responder`, and `transit_issuer`
reservation plus a mixed old/closed plan through the real command adapter. Each
must return the typed old-duty retirement outcome while deliberately absent
key, certificate, Source-root, State-root, and role-root paths remain absent.
The complete command package retains independent positive and refusal coverage
for `closed_issuer`, `closed_forwarding`, `closed_resolution`,
`closed_introduction`, and `closed_data_join`. The old Initiator, Responder,
Introduction, Transit-issuance, and Rendezvous engines and their direct server
tests are absent. State and Node tests exercise the State-to-Node value
interface, its projection, and the receipt-time bound. Historical file
inventories do not guard these
boundaries: command behavior proves the old starts refuse before effects, and
the current closed-duty tests prove their accepting paths.

The `ardents-node source --config` retirement oracle submits old-only and
mixed old/closed `native_rendezvous_profile` plans through the real command
adapter. Both return `old Source profile is retired` with no output while the
deliberately absent State and local-role roots remain absent; deliberately
missing TLS inputs also prove the refusal precedes key access and listener
creation. The closed Source process test independently starts the explicit
`ardents-route-v3` profile with its previously pinned signer and verifies the
same signed State bytes through the real Source transport.

The retired `ardents-node contributor` start-retirement oracle and the
Contributor no-start recovery oracle were removed together with the whole
retirement mechanism and its package (ADR-0114). The standard command-usage
test keeps the unrecognized `contributor` shape failing before any effect.

The `ardents name encode/resolve/control` retirement oracle submits all three
recognized command shapes, including absent inputs and incomplete remaining
arguments, through the real command dispatch. The complete inputs are shaped
like the former adapter inputs against one authenticated State root and one
synthetic root shaped like the retired Namespace store. Every retired command
case returns the exact refusal with no output; a recording default transport
observes zero attempts, and all files in those same durable roots remain
byte-for-byte unchanged. The retired `name encode` verb refuses like the
others; its former exact byte vector survives only as inlined fixture bytes
that never execute an encoder (ADR-0113). Removing the command-only
HTTP/OHTTP adapters made the exact private Resolution and adjacent Namespace
verification closure production-dead; ADR-0105 and ADR-0113 then deleted that
closure with its packages, and their deadcode allowance groups were retired
with them.

## Local diagnostic environment

The explicit [protected installation capture driver](../../tests/qualification/protected-installation/README.md)
invokes public provision and installation-check, then asks the admitted system
manager to start the actual installed unit. It retains the first command failure
without retry or reset. This is a preparation/capture surface, not the complete
two-Endpoint Service oracle or evidence that any admitted host was exercised;
manager-start success alone does not establish participant readiness. Its
independent program digest and genuine operator request are selected prerequisites.

The [local diagnostics owner](local-diagnostics.md) and its
[recipes](../../scripts/diagnostics/README.md) define the explicit Docker tools,
private live process mode, event/resource collection, race/profile separation,
source identity, failure/loss and privilege limits. `make diagnostics-build`
is explicit setup; `make diagnostics-check` exercises the Linux collector.
The Linux architecture deterministic profile runs that collector's meaningful
negative/terminal evidence tests without Docker. Process diagnostic behavior
belongs to `internal/diagnostics/process` in the deterministic/race profile.
The live Unix Interface is explicitly enabled by `ARDENTS_DEBUG_SOCKET` only
for Endpoint/Node commands; it supplies no worker-confinement or Network
readiness qualification. Product runtime dependencies are unchanged.

The deterministic inventory has an explicit Linux-only supplement in
`tests/profiles/deterministic-linux-packages.txt` for Endpoint introduction,
source, Service, token and publication owners whose files have Linux build
constraints. Linux unit/race gates include it; Windows checks retain the common
inventory. Architecture checks validate both inventories and Linux selection;
no build-error-based filtering or passing platform skip supplies that membership.

## Independent offline Admission profile

The separate `issuer-key-material-linux` profile runs `make issuance-check`:
real RSA keys, owner-only files, exclusive lifetime leases, fault/crash/reopen,
compiled initialize/inspect/export commands, Admission SPKI compatibility and
actual OTLP. It uses the same pinned Go 1.27.3 bookworm image and shares the
immutable command build. It requires proc/sys/loopback for the existing command
tests. Windows cannot replace Linux acceptance; filesystem tests do not qualify
power loss, full storage rollback or network security.

`make admission-check` requires Linux and executes all successor Admission packages
under race with a five-minute package timeout: allocation, quota, issuance,
holder stock, presentation and receiving/spend owners, including lease,
crash/reopen and concurrency tests. Compiled `ardents-next` commands exercise
holder/allocation/current-issuer/receiver operations, SIGINT with inherited stdin,
exact replay and refill, plus actual Hosting budget and OTLP checks. The checked
profile is `admission-ledger-linux`; the Docker acceptance image is
`golang:1.27.3-bookworm@sha256:21ade014faf952365b9ecbd8dbdb87176fbacb82fac9fb132d9f2a7b8cc3f132`.
Command tests share one immutable compiled executable while keeping separate
mutable roots. Linux prerequisites cannot be replaced by a Windows passing
result. These tests provide no live issuer/State, power-loss storage or whole
Network qualification.

The token-issuance-linux profile executes make token-issuance-check with the
issuance owners and the compiled CLI under race in the same pinned image.
Its complete command suite has the same five-minute package timeout as the
issuer-profile and Route profiles. This is a test execution bound; no runtime
permission, authority, transport or recovery deadline is extended.
It includes real client blind/finalize and stdlib PSS verification, opaque debit
confirmation, quota retention, journal pairs/floors, faults, crash/replay and OTLP.
CIRCL uses the populated reviewed module cache. No power-loss/rollback claim.

The active `issuer-profile-linux` profile uses `make issuer-profile-check` on the
pinned Go 1.27.3 Linux image. It exercises Admission, its Issuerprofile/Issuance/Issuer children,
Issuerprofile/Signing and compiled ardents-next with real files, lease/crash processes,
race detection, actual OTLP and the prior durable token cycle. It also mutation-
fuzzes FuzzIssuerProfile and FuzzNodeIdentityPEM for 30 seconds each, two workers
and a 90-second per-target limit; caches and receipts remain outside Git. A
missing Linux/fs/flock/compiler/image prerequisite is invalid, never a skip.

Admission source boundaries are checked across all Go build profiles by
`TestSuccessorImportIsolation` and exact-package negative policy cases. The
public profile fixtures independently encode signed bytes and canonical SPKI;
ledger binding tests separately preserve Authority/Profile/Duty. The Linux
issuer regression retains a canceled debit plus simultaneous result/key close
failures and verifies released leases. These checks cover the migrated offline
cycle; they do not accept a holder stock, live issuer or token-spend engine.

### Portable Route public-boundary controls

The same Windows/Linux owner suites exercise public Prefix opening refusal
for a canceled caller and a sealed original Source before observation,
presentation or caller-owned reservation return. Public JOIN acquisition
refuses missing originals and consumes a failed attempt without an implicit
retry. Joined methods refuse original cancellation before lane I/O; actual
framing pipes retain metadata and demonstrate deadline interruption. A canceled
receiving registration leaves pre-handoff capacity with its original caller,
whose repeated Release returns it once. None of these controls supplies an
accepting authority, durable claim or recipient ACK. Genuine admitted consumers
remain the separate native TCP/TLS profile.

Portable `channel/child_queue_test.go` checks synchronous child preparation before
pipelined input, shared input/output/control accounting, header costs, retained
unread data, reserved CLOSE space and ordinary sibling progress. Its negative
control disables secondary input accounting and must fail on uncharged handshake
bytes. `channel/open_lane_test.go` also checks reservation return by a joined
parent and waiting for a return already started by Lane Finish. These are
physical ownership controls, not accepting authority or Admission substitutes.

Portable `channel/child_limits_test.go` checks complete OPEN/data/control costs,
prepaid termination, retained input, refusal before output and ordinary sibling
progress. Its session-output control independently counts OPEN, BYTES, CREDIT,
EOF and CLOSE, and proves refused frames reach neither the physical connection
nor the channel debit. A started partial write still retires its physical owner and retains
the original failure. `bootstrap/budget_test.go` separately checks that a held
terminal cannot accumulate another full burst, emission and joined disposal
never refund its debit, fractional refill survives, and copied handles cannot
multiply the actual adjacency or return another claim.

`cmd/ardents-next/route_bootstrap_linux_test.go` uses genuine signed Network,
Stock, receiving spend and Hosting on TCP/TLS: four pending restricted
children retain their actual Hosting work/termination and Route allocations,
the fifth refuses before TLS/admission and
a fresh ordinary child on that same Node Carrier completes actual admission.
The consumer waits for peer TLS shutdown and the child terminal before closing
the shared Carrier. The test observes held provider capacity during pending TLS
and zero reserved capacity with four once-only bootstrap returns after join.
A causal early-return control must fail while TLS is still pending. This is
not successful bootstrap issuance. Native roots
justify this Linux test; the corresponding framing/capacity rules remain in
portable owner tests.

The same native test owner exercises direct issuer bootstrap at the genuine
Entry and its restricted inner channel at the genuine Interior on TCP/TLS.
Both retain actual Hosting reservations until joined TLS/channel termination;
neither invokes private receiving Admission. Replacing the propagated bootstrap
restriction with the ordinary value makes the Interior refuse bootstrap.
This proves Entry-to-Interior transport only: it supplies no issuer response,
verified token stock or holder retained-selection acceptance.

`TestRouteBootstrapObtainsStockFromActualIssuer` uses genuine retained
Domain-1 selection, permission-only Stock and the real receiving issuer
composition on TCP/TLS. Both bootstrap batches travel through the same
Entry/Interior lifetime; genuine `IssueCurrent` produces the result verified
by Stock. The test waits for both TLS directions and the child terminal,
then observes once-only Hosting returns. Native durable owners justify its
Linux profile. This is a network component scenario, not the complete compiled
holder bootstrap-to-ordinary acceptance sequence.

Portable transport tests preserve optional half-close, genuine negotiated TLS
exporter bytes, once-only physical Close and the original close failure.
The half-close control failed before the wrapper correction. The portable
CREDIT control separately failed when a DATA-only write deadline interrupted
an already started CREDIT; it requires original CREDIT bounds and refusal of
subsequent DATA after the correction. Earlier native issuer shutdown failures
remain in the execution ledger; these passes do not qualify the whole Route.

The shared native Route fixture selects a usable real Permission hour before
creating owners: when less than one minute remains, it waits for the next hour
with the original test context. A longer signed profile cannot extend hourly
Permission validity. This is test preparation, without a clock override, skip,
operation retry or change to the operation's original bounds.

Native Context physical-output fixtures live with `context_linux_test.go`;
Receiver retained-budget/physical-write oracles live with
`join_failure_linux_test.go`. Portable pool/Joined mechanics remain portable.
The exact Windows production deadcode closure documents portable functions
behind the actual native selection/receiving-root composition. Every symbol
remains reachable in Linux production, while test analysis on both platforms
requires an empty unreachable-function set. This does not qualify Windows
durable composition; the checker and its exact comparison are unchanged.

### New Installation generation authentication

The public successor handle has a real `installation upgrade-installed --request`
consumer. Portable tests refuse detached handles and cancelled openings; the
compiled command refuses unavailable native/request prerequisites before trust.
The native copied-handle regression uses an actual writer descriptor and a
cancelled original caller, with no successful Release/manager/startup substitute;
it requires physical close, the shared one-use latch and retained cancellation.
These refusal/lifetime checks do not prove a positive successor start or ACK.
The accepting new installed runtime and explicit successor recovery/interruption
matrix remain separate evidence obligations. Candidate process/scope lifetime
lives in `candidate_linux.go`, independently of transition admission/staging.
Its unchanged native missing-custody and unjoined-writer checks reside in
`candidate_native_linux_test.go`, while transaction/selection checks stay with
successor transition. Completion record and ACK sequencing share the candidate
implementation; genuine accepting runtime remains a separate obligation.

Fixed-unit behavior tests separately observe a completed failed start without
turning failed manager state into fresh-stopped or recovery admission. They
refuse live PID, queued/missing Job, changed protection/program, incomplete or
reversed execution clocks, ambiguous result and noncanonical numbers. Candidate
cleanup still repeats actual activation/worker/kernel observations; pure typed
fixtures do not establish that a manager attempted, stopped or joined work.

Completion grammar tests use independently spelled 160-byte frames, reject
noncanonical/oversized digests and zero InvocationID, and retain detached return
values. The root selection-schema refusal and exact byte oracle live with the
native start barrier; both producers consume the completion Module grammar.
These checks grant no accepting startup or installed evidence.

Portable Installation checks consume genuine pinned Enrollment Bundle or
untrusted Candidate snapshots and actual Release authorizations. Command
integration tests independently assemble signed program/generation targets,
authenticate both exact byte sets, exercise retained no-update evaluations,
refuse cold candidate trust and compare private proof facts. Root, program,
metadata substitution and duplicate metadata names must refuse before floor
effects. Refusal of the second target, incoherent target facts and original
cancellation at final handoff retain real already committed Release floors.

`cmd/ardents-next/installation_test.go` runs these owner-backed cases under
Windows/Linux race; the compiled selected Linux consumer also exercises initial
and retained candidate authentication. Other hosts execute actual foreign-platform
refusal before history creation. `internal/successor/release/retained_history_test.go`
covers absent/empty history, genuinely committed Root-only history and complete
reopen. These are byte authentication and native history checks, not installed
journal, predecessor join, manager start or installed/privacy qualification.

## Bounded Authority Custody native lane

`make custody-check` requires Linux, Go 1.27.3, a race-capable compiler,
private actual filesystem with flock/hardlinks/rename/directory fsync, a PTY
and the fixed 256 MiB Argon2 workspace. It exercises the new encrypted Custody
owner and real Allocation, original Close/cancel/join, canonical envelope vector,
native RLIMIT_FSIZE partial write, native floor failure and exact authenticated
pending-successor reopen, quarantine export/restore/purge and canonical built
terminal Admission and Service commands. The registered Linux Service Custody
process scenario builds genuine `ardents-next` request/acceptance and no-echo
`ardents-custody` creation/issuance, with independent public verification and
signature-byte oracle. Service fault controls cover real partial writes, floor
failure after successor publication and exact pending completion after expiry.
These cases do not qualify live Publisher/Connection or installed Service.
Official exact-version Argon2
vectors run too. Unknown prerequisites fail; reconstructed torn/missing journal
states are explicitly distinct from syscall fault controls.

The new package belongs to the existing deterministic and derived race
inventory; `tests/e2e/service` owns its compiled PTY consumer under process.
Native Windows command/PTY checks add development evidence, not a Windows or
Ubuntu supported Vault permissions/crash/power-loss profile. One fixed-profile
vector verifies 256 MiB derivation, not weakest-host performance qualification.
No fixture root signing or old backend supplies successful new Custody behavior.

## Original Direct-Origin Source development profile

`make source-process-check` separately runs the registered `source-process-linux`
profile using independently pinned canonical command and `node_process_native`
race command/resource binaries. `scripts/node/run-source-native.sh` requires exact
discovery and executed cases. Its driver remains outside measured leaves; use the
same external shared lock in the native development recipe, with a predeclared
finite schedule and before/after source/artifact/config/actual-stand identities.
The actual Source command serves independently prepared signed Epoch evidence
through new Source clients. LATEST/BY_DIGEST canonical bytes and materialization
verification, causal authentication/network/digest/index refusals and original
joined State reopen are distinct assertions. Separate original Source resource
controls exercise placement/caller, descriptor evidence, memory pressure and
retained failure without revival; actual established CPU pressure is required.
Unchanged State/duty owner suites supply ADR-0118 blocked handler, A→B→C,
expiry/cap/uncertainty/recovery and foreign-producer controls only on matching
source. Command failure fixtures isolate error retention and join ordering,
never successful authority. Missing prerequisites or overload evidence fail.

Source's strict plan tests reject short/overlong trust arrays, unknown or mixed
acquisition/clock inputs, duplicate fields and trailing JSON. Real-State failed
readiness output verifies guard removal and exclusive root reopen only after
joined Close; original output and Wait/independent Close failures retain their
own causes. Native pipe backpressure/peer loss has a finite output refusal.
Unavailable physical Close emits a bounded failure notice while retaining the
original custody and waiting; it cannot report joined completion from timeout.

`TestServingSourceRetainsRootThroughGuardRemoval` holds the original State-owned
clock seam at actual guard cleanup after the real listener has joined. A second
real serving State must refuse the still-owned root; after original cleanup,
a new serving State opens with its own live guard. This is a causal exclusive
custody control, including prevention of an older Close removing a successor's
guard. No new production mutation interface or second time authority is added.

The blocked READY pipe cancellation control requires the original caller cause,
not an output timeout or a nil-Wait background failure. A separate failure fixture
latches an original process error during failed output and requires both causes
through physical State Close. After joined State users, composition reads only
Guard's retained refusal under its already-denied original context, then closes
process pins; its own exact retirement notice is not an independent failure.
