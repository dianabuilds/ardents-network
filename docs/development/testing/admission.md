# Admission and Custody verification

Use this reference only for the affected owner/profile. [Testing policy](../testing.md)
owns mandatory gates, [evidence limits](../testing.md#evidence-interpretation) and
result validity. The [checked registry](../../../tests/profiles/profiles.json),
[Makefile](../../../Makefile) and linked profile README own execution/prerequisites;
this reference retains the distinct causal oracles, not a second run ledger.

## Offline profiles

## Independent offline Admission profile

The separate `issuer-key-material-linux` profile runs `make issuance-check`: real RSA
keys, owner-only files, exclusive lifetime leases, fault/crash/reopen, compiled
initialize/inspect/export commands, Admission SPKI compatibility and actual OTLP. It
uses the same pinned Go 1.27.4 bookworm image and shares the immutable command build. It
requires proc/sys/loopback for the existing command tests. Windows cannot replace Linux
acceptance; filesystem tests do not qualify power loss, full storage rollback or network
security.

`make admission-check` requires Linux and executes all successor Admission packages
under race with a five-minute package timeout: allocation, quota, issuance, holder
stock, presentation and receiving/spend owners, including lease, crash/reopen and
concurrency tests. Compiled `ardents-next` commands exercise
holder/allocation/current-issuer/receiver operations, SIGINT with inherited stdin, exact
replay and refill, plus actual Hosting budget and OTLP checks. The checked profile is
`admission-ledger-linux`; the Docker acceptance image is
`golang:1.27.4-bookworm@sha256:4fa7c968ba93f1635670522dbb01a2c78150443e1385746924566079f36570ea`.
Command tests share one immutable compiled executable while keeping separate mutable
roots. Linux prerequisites cannot be replaced by a Windows passing result.

The token-issuance-linux profile executes make token-issuance-check with the issuance
owners and the compiled CLI under race in the same pinned image. Its complete command
suite has the same five-minute package timeout as the issuer-profile and Route profiles.
This is a test execution bound; no runtime permission, authority, transport or recovery
deadline is extended. It includes real client blind/finalize and stdlib PSS
verification, opaque debit confirmation, quota retention, journal pairs/floors, faults,
crash/replay and OTLP. CIRCL uses the populated reviewed module cache. No
power-loss/rollback claim.

The active `issuer-profile-linux` profile uses `make issuer-profile-check` on the pinned
Go 1.27.4 Linux image. It exercises Admission, its Issuerprofile/Issuance/Issuer
children, Issuerprofile/Signing and compiled ardents-next with real files, lease/crash
processes, race detection, actual OTLP and the prior durable token cycle. It also
mutation- fuzzes FuzzIssuerProfile and FuzzNodeIdentityPEM for 30 seconds each, two
workers and a 90-second per-target limit; caches and receipts remain outside Git. A
missing Linux/fs/flock/compiler/image prerequisite is invalid, never a skip.

Admission source boundaries are checked across all Go build profiles by
`TestSuccessorImportIsolation` and exact-package negative policy cases. The public
profile fixtures independently encode signed bytes and canonical SPKI; ledger binding
tests separately preserve Authority/Profile/Duty. The Linux issuer regression retains a
canceled debit plus simultaneous result/key close failures and verifies released leases.
These checks cover the migrated offline cycle; they do not accept a holder stock, live
issuer or token-spend engine.

## Bounded Authority Custody native lane

`make custody-check` requires Linux, Go 1.27.4, a race-capable compiler, private actual
filesystem with flock/hardlinks/rename/directory fsync, a PTY and the fixed 256 MiB
Argon2 workspace. It exercises the new encrypted Custody owner and real Allocation,
original Close/cancel/join, canonical envelope vector, native RLIMIT_FSIZE partial
write, native floor failure and exact authenticated pending-successor reopen, quarantine
export/restore/purge and canonical built terminal Admission and Service commands. The
registered Linux Service Custody process scenario builds genuine `ardents-next`
request/acceptance and no-echo `ardents-custody` creation/issuance, with independent
public verification and signature-byte oracle. Service fault controls cover real partial
writes, floor failure after successor publication and exact pending completion after
expiry. Official exact-version Argon2 vectors run too. Unknown prerequisites fail;
reconstructed torn/missing journal states are explicitly distinct from syscall fault
controls.

The new package belongs to the existing deterministic and derived race inventory;
`tests/e2e/service` owns its compiled PTY consumer under process. Native Windows
command/PTY checks add development evidence, not a Windows or Ubuntu supported Vault
permissions/crash/power-loss profile. One fixed-profile vector verifies 256 MiB
derivation, not weakest-host performance qualification. No fixture root signing or old
backend supplies successful new Custody behavior.

## Original spending and receiving lifetimes

`TestSpendInspectionNative*` belongs to the existing root/ext4 `network-signing-linux`
profile. Run its bounded owner selection with `go test -race -tags
network_signing_native ./internal/successor/admission/spending -run
'^TestSpendInspectionNative' -count=1 -timeout=2m` under the profile's original
development lock, private root-controlled native TMPDIR and pinned
toolchain/dependencies. It uses genuine spend persistence and native exclusive leases,
checking unchanged bytes, retained burn, missing or damaged history without
repair/creation, numeric principal, changed inode/bytes/ inventory/root, permanent
refusal after restoration, copied Close, cancellation and held original readers/Close
with actual late close failure. The lease must remain unavailable until original users
and descriptor closes join. Supplied binding/token bytes isolate storage; this is no
signed State, installed duty, retirement, replacement or recovery acceptance. Its
selected execution and source identity remain separate from compilation and ordinary
deterministic checks.

`TestReceivingHeldForeignWorkDeniesBeforeJoin` and
`TestReceivingOriginalAttemptCancellationAndRefill` hold Receiving's original
authority/reservation boundaries across Close, cancellation and concurrent admission.
Genuine token verification and retained spend reopening distinguish pre-spend refusal
from irreversible post-spend burn. They check late cleanup causes, no late Grant,
original refill deadlines and exactly-once reservation transfer. Their supplied
authority facts and reservation callbacks qualify only the Receiving boundary. They run
in the ordinary deterministic profile on Windows and Linux, with race checks for the
concurrent paths. `TestReceivingCloseDuringVerificationDeniesReservation` separately
holds the actual VerificationGate after the original observation's committed time-floor
advance. Close must seal admission before that gate is released; the continued attempt
must reserve and spend nothing and publish no Grant. This is a local effect-admission
oracle, distinct from holding a neighboring owner callback.

Linux `TestNetworkReceivingHeldForeignWorkRetainsOriginalCustody` in the existing
`cmd/ardents-next` deterministic owner instead uses signed Network State, actual tokens
and genuine Hosting reservations. Accept/Refill each hold observation or reservation
across original cancellation/Close. A separately held refusal release proves that
capacity and the spend lease remain retained until physical callback join, including its
late failure. Original spend bytes and predecessor capacity remain unchanged. Linux is
required by these actual durable owners; the scenario starts no installed role and
grants no bootstrap, installed, availability or EOF-repair qualification. Existing
signed successor/post-spend and joined socket-work tests retain their separate coverage.

The in-process Admission Custody command receipt/debit test uses a private
`testing/synctest` clock so a wall-clock hour transition cannot substitute an expiry
refusal for its intended output-failure boundary. Genuine crypto, journal and file
operations remain unchanged. The same test explicitly advances to the next hour and
requires refusal, no success receipt and the unchanged original allocation generation.
Native/installed qualification retains its real clock.

## Funded Source composition

`TestFundedSourceBootstrapAndAdmittedIssuanceTCP` exercises the production cold Source
funding composition against actual signed Network, approved holder, blind issuer,
receiving Admission and Hosting/Route owners. Exactly two finite bootstrap batches
precede fresh admitted Source channels and an ordinary issuer exchange on TCP/TLS; quota
and original physical retirement remain checked. The fixture's second bootstrap batch
funds three issuer Control presentations and a signed-inventory Descriptor Store
Control. Actual Publisher composition seeds four issuer Controls within its original
finite permission. The next retained prefix checks one two-recipient Forward batch
without an intervening Control refill, with separate real admissions and exact remaining
quota, instead of issuing each missing Forward token in a separate exchange. Both funded
physical openings complete while a separate Stock preparation and the command issuance
gate remain held; their original tokens need no new batch. JOIN preparation then
requests only a genuinely missing signed recipient and retains exact quota, original
Source and cancellation checks. These are Stock and transport controls, not successful
Instance authentication. Its signing/permission infrastructure and cleanup observer are
fixtures, not a qualified worker or installed Startup. Stock preparation tests
separately check original coordination, copied/canceled handles, ordered multiplicity,
foreign profile refusal and no allocation/presentation refunds.
