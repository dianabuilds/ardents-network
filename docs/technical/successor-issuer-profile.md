# Offline issuer-profile signing

This completed design implements explicit existing-key import and immutable
Node-signed ARDCIP01 provisioning. It establishes neither Node duty nor State,
Time Confidence, network admission or a replacement State-profile digest.

## Interfaces and ownership

`admission/issuerprofile` owns the canonical public grammar, opaque `Request`
and `Verified`. `Prepare` validates public pins and complete class/hour cohorts
before creating a signing request. `Verify` validates the exact external
Network/Node/signer/interval binding and signature. `ValidateCohorts`, `ParseKey`
and `EncodeKey` are shared by the ledger and private-material owner; no fake
ledger or Authority/Profile/Duty is constructed to validate a public inventory.

Admission's `PrepareLedgerBinding` fills only keys in an explicitly supplied
ledger binding; its State profile digest, authority and duty remain independently
chosen facts. Snapshots are defensive; zero opaque values refuse. The public
contract uses only the standard library and imports neither ledger nor private signing, including in its tests.
Request and Verified have distinct private representations: explicit Go type
conversion cannot promote an unsigned request into verified evidence. Only a
successful signature check creates Verified.

`admission/issuerprofile/signing` owns Import/Open/Public/SignIssuerProfile/Close. It imports one
bounded PKCS#8 Ed25519 PEM matching a separately supplied public pin; stores
canonical DER; never returns a private key or signs arbitrary bytes. Its only
project import is `admission/issuerprofile`. `admission/issuance` constructs
requests from its open inventory and owns InitializeProfile/OpenProfile/Bytes/
Close. `admission/issuer` composes identity -> issuer keys -> profile; reverse
cleanup retains the primary result and each acquired owner's outcome. Command
adapters own strict JSON, export and bounded OTel. The production graph is
Issuer/Issuance/selected command adapters -> Signing -> pure Issuerprofile ->
standard library. There is no generic signer, Identity domain, parent-to-child
import or retired `internal/successor/nodeidentity` facade. RSA token material,
quotas and results keep their separate owners; runtime Node TLS and Network duty
are outside this offline owner. Relocation preserves existing roots and all CLI,
JSON, diagnostics and signature identities below; it performs no data migration.

Signing serializes original native custody checks and key use with Close under
its own mutex. Public Request snapshots are detached pure data. It calls no
foreign live owner or I/O callback while holding that mutex; Close joins these
original users before clearing the retained key and shares its original outcome
across copies. A detected custody loss is permanent for that owner even if the
same path/inode is restored. Only a fresh explicit Open independently verifies
restored storage; restoration does not revive the earlier owner.

## Formats and durability

Profile body: ARDCIP01, Network[32], Node[32], start/end u64, count u16,
sorted window u64/class u8/SPKI-length u16/SPKI[346]; append Ed25519 signature
over ardents-closed-issuer-keys-v1\0 plus body. Maximum 6580 bytes. Require
nonzero IDs, nonnegative aligned overflow-safe seconds, 1..6 full hours, each
class 1..3 exactly once per hour, distinct dedicated keys and canonical SPKI.
Encoding and decoding share validation, not a permissive legacy writer.

Identity root marker ardents-node-identity-v1; retained identity.pin/key/lock,
exclusive temporary identity.pending. Profile marker ardents-issuer-profile-v1;
retained profile.pin/bytes/lock, profile.pending. Pins bind exact public facts
and content digests; profile additionally binds unsigned RSA inventory digest.
Root0700/files0600, effective owner, regular single-link files, canonical absolute
paths and no symlink. Exact retained sets, opened descriptor/path identities
and substitution checks. Nonblocking lifetime Linux lease. Shared copy handles.
Fresh initialization refuses existing directories; cancellation before mkdir
has no effects, after mutation completes durability or returns storage-uncertain.
Exclusive pending write/sync/rename/readback, directory sync, final pin/readback,
root and parent sync. Open verifies content then completes durability barriers.
Partial/pending/foreign roots refuse without repair, regeneration or deletion.
Source PEM <=64KiB, one header-free PRIVATE KEY block with LF or CRLF line
endings, opened through its owner-only directory and checked around
reading, remains untouched. Unsupported platforms refuse before private reads.

## Commands

node-identity import config: root,binding(network,node,signer),key_file.
issuance initialize-profile/inspect-profile: identity_root,identity_binding,
key_root,key_binding,profile_root,profile_file.
admission prepare-binding: profile_file,expected_profile(network,issuer,signer,
start,end),binding(existing ledger fields except keys),binding_file.
All config16KiB strict JSON with exact fields/no duplicate/null/case aliases;
lowercase hex, whole-second UTC, canonical paths; distinct non-nested roots.
Export raw profile or existing Admission binding JSON via exclusive/idempotent
owner-only output and sync. Outputs must be outside the three selected roots;
export directories and ancestors containing retained or pending successor-state
files refuse before writing, including binding exports. Export refusal retains
committed roots. Inspection
permits future/expired inventories; it grants no current authority. Diagnostics
and OTel expose fixed operation/phase/outcome/duration only; collector failure
cannot change storage decisions. Cleanup failures suppress response export and project storage-uncertain; the
operation retains its primary result and every owner cleanup outcome.

## Rule/source/independent test mapping

| Rule | Current dev source | Independent verification |
|---|---|---|
| Ed25519 PKCS#8 import | admission/issuerprofile/signing store/files/platform | independently generated PEM, bad types/pin/framing, source links/permissions |
| Profile grammar and signature domain | admission/issuerprofile profile/public key grammar | separate fixture encoder, stdlib Verify, every cohort, malformed signed fixtures |
| Cohort/SPKI uniqueness | admission/issuerprofile public grammar and Issuance inventory | missing/reordered/duplicate keys, wrapped seconds, exact 1/6-hour bytes |
| Opaque signing request | successor Admission confirmation pattern | zero refusal, defensive copies, mismatched identity refuses |
| Identity/profile durable roots | successor Issuance storage | real fault files, pending, links/substitution, lifetime lease, crash/reopen |
| Offline binding preparation | successor Admission ledger binding | State digest/duty/authority preserved, exact independently pinned keys |
| Real composition | successor admission/issuer and ardents-next | compiled import/profile/binding/debit/issue/replay and OTLP |

The following invariant map names executable effects and causal controls rather
than issue status. Source identity and observed gate results belong to the
selected issue receipt and external source-matched evidence.

| Invariant / owner | Real caller and effect | Positive / causal refusal control |
|---|---|---|
| Exact canonical bytes / pure Issuerprofile | Signing.SignIssuerProfile consumes only validated Request | TestIssuerProfileIndependentBytesAndCopies; independent stdlib ARDCIP01 encoder and retained-root before/after command comparison for 1h/6h |
| Imported pinned Ed25519 / Signing | Existing node-identity import creates fresh immutable root | TestIdentityEncodingPinnedAndBounded; TestIdentityImportSourceAndRootFailures; malformed/foreign PEM has no new-root effect |
| Original lease and durability / Signing | Issuer opens signer before RSA/profile owners | TestIdentityLeaseCopyAndPinnedReopen, TestIdentityKilledProcessRetainsKey; TestIdentityFaultsAndCancellation refuses uncertain acknowledgement |
| Permanent custody failure / Signing | Public and SignIssuerProfile check original custody before use | TestSigningObservedLossCannotReviveAcrossCopies restores each original path after observed loss; old owner still refuses, independent Open succeeds |
| Join before erasure / Signing | Issuer reverse cleanup calls shared Close | TestSigningAndCopiedCloseJoinOriginalKeyUsers verifies concurrent use/Close, erased retained key, late refusal and released lease |
| Request refusal preserves healthy owner / Signing | Issuance constructs bound public request | TestSigningRefusalsLeaveOriginalOwnerUsable checks zero/foreign/canceled requests then actual valid signature |
| Unsupported pre-effect refusal / platform and command | Original command refuses before private config/source/root reads | TestUnsupportedRefusesBeforePrivateInputsAndEffects; TestIssuerProfileUnsupportedCommandRefusesBeforeConfigRead |
| Real offline pipeline / Issuer and command | Import -> initialize/inspect profile -> binding -> debit/issue/retry/export | TestIssuerProfileCLICompleteTokenCycle verifies PSS response, exact retry, root-export/busy/cancellation refusal and bounded OTLP |

Acceptance requires host quick/full gates, pinned Go1.27.2 Linux behavior/race/
process, bounded PEM/profile fuzzing, prior token-issuance regressions, all-attempt
source checkpoint and scoped local dev commit. This is not power-loss/full-
rollback or network qualification evidence.
