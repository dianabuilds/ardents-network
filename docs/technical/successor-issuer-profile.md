# Offline Node identity and issuer profile

This completed design implements explicit existing-key import and immutable
Node-signed ARDCIP01 provisioning. It establishes neither Node duty nor State,
Time Confidence, network admission or a replacement State-profile digest.

## Interfaces and ownership

Admission owns the canonical profile grammar, opaque IssuerProfileRequest and
VerifiedIssuerProfile. PrepareIssuerProfile validates all facts/cohorts before
creating a signing request. VerifyIssuerProfile validates the exact external
Network/Node/signer/interval binding and signature. PrepareLedgerBinding fills
only keys in an explicitly supplied ledger binding; its State profile digest,
authority and duty remain independently chosen facts. Snapshots are defensive;
zero opaque values refuse. No dependency on identity or Issuance is allowed.

Nodeidentity owns Import/Open/Public/SignIssuerProfile/Close. It imports one
bounded PKCS#8 Ed25519 PEM matching a separately supplied public pin; stores
canonical DER; never returns a private key or signs arbitrary bytes. Its only
non-standard import is successor Admission. Issuance constructs requests from
its own open inventory and owns InitializeProfile/OpenProfile/Bytes/Close.
Tokenissuance composes identity -> issuer keys -> profile; reverse cleanup
retains errors. Command adapters own strict JSON, export and bounded OTel.

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
cannot change storage decisions. Cleanup failures override success.

## Rule/source/independent test mapping

| Rule | Current dev source | Independent verification |
|---|---|---|
| Ed25519 PKCS#8 import | internal/node/identity.go | independently generated PEM, bad types/pin/framing, source links/permissions |
| Profile grammar and signature domain | internal/admission/issuer_profile.go | separate fixture encoder, stdlib Verify, every cohort, malformed signed fixtures |
| Cohort/SPKI uniqueness | successor Admission binding/token_key, Issuance inventory | missing/reordered/duplicate keys, wrapped seconds, exact 1/6-hour bytes |
| Opaque signing request | successor Admission confirmation pattern | zero refusal, defensive copies, mismatched identity refuses |
| Identity/profile durable roots | successor Issuance storage | real fault files, pending, links/substitution, lifetime lease, crash/reopen |
| Offline binding preparation | successor Admission ledger binding | State digest/duty/authority preserved, exact independently pinned keys |
| Real composition | successor tokenissuance and ardents-next | compiled import/profile/binding/debit/issue/replay and OTLP |

Acceptance requires host quick/full gates, pinned Go1.27.1 Linux behavior/race/
process, bounded PEM/profile fuzzing, prior token-issuance regressions, all-attempt
source checkpoint and scoped local dev commit. This is not power-loss/full-
rollback or network qualification evidence.
