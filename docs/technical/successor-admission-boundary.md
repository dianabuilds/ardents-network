# Admission: boundary and ownership

Admission owns the bounded right to obtain and spend a token allowance.
It does not own physical resources, network authority, a transport session or
an Introduction registration. The product meaning is defined in CONTEXT.md;
the current rules are in docs/technical/private-admission.md.

This document fixes the successor's scope and evidence boundary. Completion
requires the implemented owners, real command consumers and verification below;
it does not imply completion of adjacent domains.

## State owners

| Owner | Owned facts and decisions | Commit boundary |
| --- | --- | --- |
| Allocation | Holder request, role, hour, aggregate allocation maxima, exact request repeat, allocation journal transition | Decision must be durably recorded by Custody before its purpose-specific signature is returned |
| Issuance quota | Permission, holder proof, request ID and digest, bootstrap count, class balance, issuer hourly total | Admission ledger commits debit; only then returns opaque DebitConfirmation |
| Issuance material/results | Class/hour RSA keys, canonical public inventory, committed response for an exact debit | ResultStore accepts only DebitConfirmation and retains the exact response; failure never refunds the debit |
| Holder stock | Private holder key, one permission per context/hour, pending blind state, reserved balance, available tokens and presentation marks | Taking a token removes stock and durably records presentation before returning bytes |
| Receiving | Token binding to receiver/class/hour, irreversible spend, finite allowance and refusal cleanup | Spend journal precedes acceptance; post-I/O expiry cannot refund a spend |

These are separate consistency boundaries inside one Admission context, not
five new domains and not a single shared transaction. Issuer is an application
coordinator over quota, key and result owners. Token and issuerprofile are
supporting cryptographic/format code, not additional business owners.
Attempts and spending hold different histories: presentation by a holder and
acceptance by a receiver must not be collapsed into one journal.

## Explicit exclusions

| Responsibility | Owner and reason |
| --- | --- |
| Current Epoch/profile, conflicts, membership, duty generations and trusted time | Network; Admission consumes selected facts and cannot promote signed bytes into current authority |
| Authority private key, unlock, encrypted record, allocation journal persistence | Custody; Admission supplies allocation rules and exact signing grammar |
| Node identity private key | Node Identity; issuer profile preparation uses its purpose-specific signing operation |
| Physical memory, bandwidth reservations and host capacity | Hosting; a token never guarantees capacity |
| TLS/QUIC, HELLO binding, OPEN/ADMIT framing, lane state, route selection and child lifetime | Route/Node/Endpoint composition; contextual checks stay at those boundaries |
| Introduction registration IDs, slot replay and withdrawal | Route's receiving Introduction owner; a fresh token must not erase an earlier registration floor |
| Permission file paths, file publication/import, command output | Application adapters; Stock accepts and returns bytes and request commitments |
| Context/Publisher creation, worker identity and shutdown tree | Endpoint; Admission only retains/revokes the holder's admission state |

Removed from this successor tree during the boundary correction:
the second ClosedTokenIssuer (including its second quota ledger, key root,
signer and platform files); the duplicate ARDCIP01 encoder/decoder;
IntroductionSlots and its linkage to the spend ledger; permissionfile and
Stock.ExportFile/ImportFile. Existing maintained implementations outside
successor were not removed or rewired.

Batch grammar and holder-proof validation are shared by quota and holder code.
Permission structural decoding is shared by inspection and holder/issuance
inputs. SPKI parsing is shared by token and issuer profile. The remaining
issuance path is issuer.Issue -> Ledger.DebitVerified -> ResultStore.Issue.
There is no second live issuer implementation in this tree.

## Review of rules outside the old Admission directory

| Source | Admission-owned part | What remains outside |
| --- | --- | --- |
| internal/custody/admission_authority.go | Allocation decisions and permission grammar already represented by allocation and Permission | Durable encrypted commit, signing key and custody operation |
| internal/endpoint/participant_linux.go; internal/endpoint/runtimeplan/permission_paths.go | Aggregate role maxima represented by allocation.ValidateMaxima | Selecting the user's context/role and configuring file paths |
| internal/endpoint/permission.go; internal/endpoint/issuance.go | Holder request, accepted permission, pending batch and stock | Context authorization, choosing an exchange, joining network work |
| internal/node/authority/token.go; internal/node/admission.go | Receiver token verification and spend scenario represented by receiving | Current authority observation, host reservation and actual channel ownership |
| internal/route/closed_admission_channel.go | Class allowance and deadline bound represented by receiving.Allowance | Channel handshake and delivery of the accepted allowance |
| internal/route/closed_forwarding_channel.go; internal/route/closed_join_replenishment.go | Refill replaces remaining allowance without extending its original deadline | Counting bytes and deciding when a live lane can request refill |
| internal/route/closed_duty_limits.go | Verification admission bound represented by receiving.VerificationGate | Other transport/role counters and scheduling |
| internal/successor/network/closedprofile/token_spki.go and profile key fields | Canonical token key grammar represented by issuerprofile | Signed profile acceptance, membership and currentness |
| internal/admission/spending/introduction_slots.go | None: slot replay belongs to Route's receiving Introduction owner | Whole slot history and registration lifetime |

This is a responsibility review of the named current paths, not a claim that
every line in the repository has been semantically classified. The old runtime
still uses its existing implementations. Switching it is a separate integration
step, not evidence that this successor is complete.

## Implemented ownership boundaries

Stock.Owner owns permission and stock; Attempt exposes copied public requests
and one terminal completion. The application executes and joins transport.
The former Host and exchange-operation implementation are removed. Failed
exchange keeps the exact pending batch and its original deadline; Close erases
the holder and invalidates outstanding attempts.

issuer.IssueCurrent reuses the same quota/key/result operation as offline Issue.
It observes authority before debit, after debit and after result storage/owner
cleanup. A refused response never refunds a committed debit.

receiving.Owner composes authority observation, verification bounds, token
verification, reservation acquisition, durable spending and finite allowance.
The allowance is bounded by both profile and duty; refill retains the original
deadline. A successful Grant transfers reservation release to the work owner.
Closing Receiving does not release already accepted work.

The selected class policy remains Control 64 KiB/30 seconds, Forward
32 MiB/1,800 seconds and Registration 1 MiB/600 seconds. Registration includes
bidirectional capsule/control traffic and has no forwarding refill right.
The new owner restores the selected Registration maximum; the preserved
predecessor's 8 MiB implementation is not a policy amendment. Class IDs,
canonical token bytes, issuance counts and irreversible spend remain unchanged.

The root contains contracts and validation. The quota child owns durable debit
state and opaque confirmation; no compatibility facade remains at the root.

Stock and Receiving have maintained local command consumers in ardents-next.
The compiled-process scenario covers allocation, issuance, presentation,
redemption, restart and interruption. Allocation returns an unsigned decision;
external Custody commits before signing. Its encrypted storage is outside this
domain and is not qualified by the local signing fixture. Acceptance requires
the full repository gate in addition to these functional checks; execution
results and integration status belong to the selected issue ledger.

No additional domain responsibility is planned. These items complete or
separate existing responsibilities; they do not authorize adding Network,
Introduction, Endpoint, Hosting, file transport or route implementations here.
A discovered new responsibility requires updating this boundary explicitly.

## Evidence scope

The independent holder/issuer/receiver cycle uses real permission signatures,
blind issuance, the public Stock operation, durable presentation, quota and
result roots, and a receiver spend root. It includes exact issuance retry after
reopening, foreign receiver generation, expired authority facts, repeated
spending after reopening and repeated stock consumption.

This scenario supplies authority facts at the boundary. It is not a test of
Network authenticity or transport integration. Architecture checks enforce exact
imports and disallow issuer
composition from importing token signing or owning a second signer.

The separate new Network-backed composition exercises actual signed intake,
`CurrentRuntime`, blind issuance, durable presentation/spend and genuine
Hosting reservations. Its command tests cover authority loss before effects,
a signed successor after spend without refund, conflict/clock refusal, refill
deadline retention and release after physical join. Network owns authenticated
facts; Admission still owns quota, verification and irreversible spend. See
[Network's consumer contract](successor-network-state.md#contracts-for-subsequent-domain-replacements)
and the [domain map](../development/domain-map.md) for source/evidence identity.
These tests do not qualify Route or either selected Carrier.


## Behavioral evidence map

Paths below are relative to internal/successor/admission. These are component
contracts; external authority authenticity and application transport remain
outside this evidence.

| Contract | Reproducing scenario | Forbidden effect checked |
| --- | --- | --- |
| Role allocation maxima and exact retry | allocation/request_linux_test.go: TestMixedRoleLimitsExactRetryAndChangedRequest | No extra allocation or mutated journal on refusal |
| Retained allocation time floor | allocation/request_linux_test.go: TestWindowAdvanceCannotResetPriorQuotaAfterRollback | Clock rollback cannot restore spent allocation |
| Canonical request and result grammar | quota/batch_test.go: TestBatchContract; batch_result_test.go | No malformed or noncanonical wire acceptance |
| Single holder attempt, exact retry, revocation | stock/lifecycle_test.go: TestHolderRetryRevocationAndSingleCompletion | No second reservation, deadline extension or revival after Close |
| Authority changes after quota commit | stock/lifecycle_test.go: TestCurrentIssuerChangesAfterDebitDoNotRefund | No response under stale authority, no second debit on exact retry |
| Permission expires after result commit | stock/lifecycle_test.go: TestIssuerWithholdsCommittedResponseAfterPermissionHour | No expired response export; committed result retained |
| Profile bounds accepted work | stock/receiving_cycle_test.go: TestReceivingAllowanceCannotOutliveProfile | Allowance cannot outlive profile even if duty lasts longer |
| Bound shortens during reservation or spend | stock/receiving_cycle_test.go: TestReceivingShrinkingBoundRefusesBeforeAndAfterSpend | Reservation released; spend occurs only in post-spend case |
| Authority disappears after spend | stock/receiving_cycle_test.go: TestReceivingAuthorityFailureAfterSpendRetainsBurnAndReleasesCapacity | No spend refund across reopen, no leaked reservation |
| Refill and ownership transfer | stock/receiving_cycle_test.go: TestReceivingRefillPreservesDeadlineAndTransferredReservations | No deadline extension or premature release at receiver Close |
| Real standalone consumers and durable reopen | cmd/ardents-next/admission_cycle_linux_test.go: TestAdmissionStandaloneCommandsIssuePresentReceiveAndReopen (repository-relative path) | No token revival after holder restart, no repeated receiver spend, no lease left after SIGINT while inherited stdin stays open |
| Selected Registration allowance with genuine signed Network and Hosting | cmd/ardents-next/registration_allowance_linux_test.go: TestRegistrationSelectedAllowanceWithSignedNetworkAndDurableSpend (repository-relative path) | Receiving returns the independent 1 MiB oracle, retains earlier caller bound and work-owned reservation, returns capacity once and refuses spent tokens after reopen |
| Selected class allowance through the compiled command consumer | cmd/ardents-next/network_commands_linux_test.go: TestNetworkCommandsAndAdmissionUseAuthenticatedRoots (repository-relative path) | Real class-2 and class-3 issuance/presentation/spend return selected bytes without extending the caller deadline; command restart cannot revive spend |
| Real Hosting budget with genuine tokens | Same compiled-process scenario, Hosting phase | Authority/budget refusal cannot burn token or leak a reserve; receiver Close cannot free accepted work |

The only batch request parser and result codec live in the Admission root.
Quota and issuer signing consume parsed requests; signing and holder finalization
consume the same result codec. Root file count is not a reason to merge these
contracts with the independently owned durable quota journal.
