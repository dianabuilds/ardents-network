# Principal Identity And Access Release Acceptance

Date: 2026-07-23

Scope: PIA-001 through PIA-018 for the greenfield first release. This is an
acceptance record, not a compatibility plan. Ardents Network has no released
bearer-authenticated or `p_`-identified installation to migrate.

## Outcome

Operator and Application calls use separate Principal-session listeners, wire
schemes, action catalogues, and server-derived resource mappings. Portable
authority is rooted in a Principal; routine authentication uses a root-signed
Device Credential. Application installation creates its own Principal and can
act for another Principal only through one bounded, one-hop Delegation. Durable
grants, enrollments, revocations, ticket digests, administration idempotency,
and mutation audit outbox records are transactionally stored in
`identity-access.db`; sessions, challenges, proofs, raw tickets, and private
keys are not. The first-release runtime has no normal bearer path, legacy
identifier parser, dual-ID alias, or authentication fallback.

## Public Contract Changes

- Principal IDs are exclusively canonical full-digest `p1_` values; Device IDs
  are canonical `d1_` values.
- Operator authentication accepts only
  `Authorization: ArdentsOperatorSession <session>`.
- Application authentication accepts only
  `Authorization: ArdentsApplicationSession <session>`.
- Bootstrap and Application Enrollment Tickets are accepted only by their
  typed enrollment flows. They are not normal call credentials.
- `Actor`, `Effective`, `Audience`, action, and resource authority are
  server-derived and carried in a sealed `AuthorizedCall`.
- Application Delegation is one hop, Application-delegatee-specific, Audience
  bound, at most 24 hours, and intersected with both Principals' live grants.
- Content ownership is `(Effective Principal, Content Reference)`; CID
  knowledge alone provides neither ownership nor read authority.
- Principal access audit events use random 128-bit `c1_` correlation IDs.
  Delivery from `identity-audit-outbox-v1` to `operations.json` is at least once.
- No public generic `Sign([]byte)` operation exists. Clients sign only frozen,
  domain-separated challenge and artifact structures.

No Operator/Application protobuf shape changed during PIA-018. The persisted
contract changed by adding `identity-audit-outbox-v1` to the single greenfield
`identity-access.db` schema version 1. There is no older supported identity
schema and no migration reader.

## Credential Acceptance Matrix

| Presentation | Operator listener | Application listener |
|---|---|---|
| Operator Principal Session | accepted for an exact registered Operator action | rejected |
| Application Principal Session | rejected | accepted for an exact registered Application action |
| First-Operator Bootstrap Ticket | enrollment RPC only | rejected |
| Application Enrollment Ticket | issued by an Operator; never normal auth | Application enrollment RPC only |
| Delegation | rejected as Operator call authority | optional one-hop attenuation attached to an Application Session |
| `Bearer`, raw token, proof, Credential, ticket, or unknown scheme | rejected without fallback | rejected without fallback |
| failed Principal Session | rejected; never retried as another scheme | rejected; never retried as another scheme |

## Required Review Answers

### 1. What identifies Actor and Effective at every public handler?

Only a sealed `access.AuthorizedCall` created by successful `Admit` or
`AdmitTarget`. Actor is the Principal bound to the authenticated Session.
Effective equals Actor for a direct call, or the verified Delegator for a valid
one-hop Delegation. The seal and fields are private. Operator code propagates
the call through `internal/localapi/rpc`; Application code uses its separate
`applicationapi/call` channel. Handlers reject missing or unsealed values.

Evidence: `internal/identity/access/authorization.go`,
`internal/identity/access/context.go`,
`internal/localapi/identity/operator_interceptor.go`,
`internal/applicationapi/admission/interceptor.go`, and their tests.

### 2. Can a client string select action, Audience, owner, or Effective?

No. Procedure catalogues select action and resource kind. The concrete listener
and transport peer select Node/interface/protocol/transport Audience. The
authenticated Session selects Actor. A verified Delegation can select only its
signed Delegator as Effective. Canonicalizers validate the typed resource
target, and content owner is derived from Effective. Unknown or malformed
actions, scopes, identifiers, and resource kinds fail closed.

Evidence: `internal/localapi/auth/access_catalog.go`,
`internal/applicationapi/content/access_catalog.go`,
`internal/identity/access/authorization.go`, and resource-target tests.

### 3. Can authority cross Node, interface, protocol, or delegatee?

No. Credential proof, challenge, Session, grant, and Delegation checks compare
the complete Audience and AuthenticationBinding. A Delegation additionally
requires `Delegatee == Session.Principal`. Alpha and Beta grants and Session
caches remain independent for the same Alice.

Evidence: `internal/identity/access/service_test.go`,
`internal/identity/access/authorization_test.go`,
`internal/cli/client/session_multinode_adversarial_test.go`, and
`internal/applicationapi/admission/interceptor_test.go`.

### 4. Does revocation take effect on the next call?

Yes. Every `Admit` re-reads Device, grant, and Delegation revocation state in
one durable snapshot. Device revocation is keyed by DeviceID, invalidates
current Sessions, and also rejects renewed Credentials for the same key.
No Session rotation is required.

Evidence: `internal/identity/access/service_test.go` and
`internal/identity/access/authorization_test.go`, including concurrent
revocation/admission and restart cases.

### 5. Which private or ticket/session values are persisted?

No Session secret or ID, challenge, nonce, proof, raw Bootstrap/Application
Enrollment Ticket, private key, or unredacted Delegation is persisted in
`identity-access.db`, diagnostics, logs, or snapshots. In-memory Session lookup
uses an HMAC. Only ticket digests are durable, because one-use and restart
semantics require them. Issued/delivered/acknowledged handoff state contains no
plaintext; retry atomically replaces an unacknowledged digest, leaving at most
one valid ticket. Signed/public Credentials and authority artifacts,
enrollment/revocation metadata, command idempotency, and redacted audit-outbox
facts are durable.

Evidence: `internal/identity/access/stores.go`,
`internal/identity/access/bootstrap_ticket.go`,
`internal/identity/access/application_enrollment.go`,
`internal/identity/access/audit_outbox.go`,
`internal/identity/access/redaction_matrix_test.go`, and
`docs/security/persistent-state-security.md`.

### 6. What happens when a key, Session, or Node state is lost?

Loss of a Session causes normal reauthentication; restart deliberately drops
Sessions, challenges, and unfinished proofs. A lost Device key is revoked by
DeviceID and replaced with a new root-signed Credential. A lost root key cannot
silently rotate the Principal; the remaining valid Devices must be retired and
the installation re-enrolled according to incident response. Loss of Node
identity state or key requires restoration of the complete matching
consistency group. Partial restore and automatic key regeneration fail closed.

Evidence: `docs/operations/incident-response.md`,
`docs/operations/upgrade-migration.md`,
`internal/identity/access/service_test.go`, and
`tests/integration/node/node_identity_test.go`.

### 7. Is interrupted upgrade or corrupt authority state safe?

Yes for the supported greenfield schema. bbolt schema creation and mutations
are transactional. Callback error, panic, cancellation, or process loss before
commit leaves no partial state. Process loss after an administration commit
leaves both the idempotency/authority change and its audit outbox event.
Unknown/incomplete schemas and malformed grant, index, revocation, ticket, or
outbox records prevent safe startup/admission. There is no old released
identity schema to translate.

Evidence: `internal/storage/identity_access_test.go`,
`internal/identity/access/first_enrollment_test.go`,
`internal/identity/access/application_enrollment_test.go`,
`internal/identity/access/audit_outbox_test.go`, and
`internal/identity/access/authorization_test.go`.

### 8. Does one trust purpose imply another?

No. Trust registry keys include the exact closed purpose. Discovery publish,
channel issue, and workload execute are separate consumers. Unknown purposes
and the same Principal under a sibling purpose miss; registry replacement is
rechecked on the next use.

Evidence: `internal/identity/trust/registry.go`,
`internal/identity/trust/registry_test.go`,
`internal/discovery/trust/evaluator.go`, and purpose-isolation tests.

### 9. Can another identifier become a Principal through conversion?

No. Principal parsing accepts only canonical `p1_` plus a full 256-bit digest.
ResourceOwner repeats that parser. Valid CID, Waku PeerID, WorkloadID, and
ServiceID values are rejected at Principal boundaries.

Evidence: `internal/identity/principal/principal.go`,
`internal/identity/principal/principal_test.go`, and
`internal/identity/access/identifier_confusion_test.go`.

### 10. Does an obsolete path remain reachable?

No reachable normal credential or identifier compatibility path remains.
Bearer and unknown schemes are rejected. Obsolete environment/token names are
present only in an explicit startup deny-list. There is no `p_` parser,
migrator, alias, coexistence state, or fallback. First Operator and Application
enrollment, restart, and persistence recovery operate without those paths.

Evidence: `internal/config/decode.go`,
`internal/localapi/identity/session.go`,
`internal/applicationapi/admission/interceptor.go`,
`internal/identity/access/first_enrollment_test.go`,
`internal/identity/access/application_enrollment_test.go`, and process e2e.

## Migration, Rollback, And Recovery Evidence

- Identity access schema version 1 is created transactionally from fresh state;
  unknown/incomplete versions and missing buckets are rejected.
- Administration, enrollment, ticket consumption, grants/revocations, command
  idempotency, and audit outbox changes roll back together on injected failure.
- A pending audit record survives restart; corrupt outbox state fails startup;
  diagnostics persistence failure leaves the record for retry.
- Restart preserves Credentials, enrollment, grants, and revocations while
  dropping Sessions/challenges/proofs.
- A stopped data-directory backup restores the same Node Principal. Removing a
  member of the identity consistency group is rejected.
- Rollback is whole-group restore to an empty target with the matching released
  binary. Editing schema markers/buckets or attaching a pre-outbox development
  binary is unsupported.

The stopped-directory integration drill covers the whole directory copy and
Node identity. Component recovery tests separately verify
`identity-access.db`, Waku identity/store, content metadata/payload ownership,
and runtime recovery. No test claims a transaction spanning `ardents.db` and
`identity-access.db`; the operational boundary is a stopped consistency-group
backup.

## Repository Removal Evidence

- `214645d` removed dormant legacy access paths.
- `55a1e27` replaced native bearer installation with Principal enrollment.
- `3fd154a` removed the control bearer from observability e2e.
- `0828902` made deployment Principal-only.
- `03fb15d` finalized the Principal-only release surface.
- Obsolete credential environment names remain only as fail-closed rejection
  inputs in `internal/config/decode.go`.

## Independent Review

The Standards review found no repository-rule violation. It noted three
judgement-call duplications: intentional surface-local mutation audit adapters,
repeated publication transport-state checks, and the in-process terminal test
pacer. These do not weaken a security boundary; the separate
Operator/Application adapters remain intentional.

The Spec review found four release blockers: non-atomic administration audit,
pre-`Admit` denials without audit, incomplete administration audit facts, and
missing written acceptance evidence. The implementation now uses a durable
transactional outbox, records structural/presentation denials at all three
protected adapters, persists complete safe authorization facts, and provides
this acceptance record.

## Release Gate Results

The commands below establish the final release evidence on 2026-07-23. The
sequencing of the last denial-only guard and its final reruns is noted below.

| Gate | Command | Result |
|---|---|---|
| Operator API generation | `powershell -NoProfile -File scripts/generate-api.ps1 -Check` | exit 0 |
| Application API generation | `powershell -NoProfile -File scripts/generate-application-api.ps1 -Check` | exit 0 |
| Identity artifact vectors | `powershell -NoProfile -File scripts/generate-identity-artifact-vectors.ps1 -Check` | exit 0 |
| Complete untagged Go suite | `go test ./... -count=1` | exit 0; 58.8 s |
| Focused race suite | `go test -race ./internal/identity/... ./internal/applicationapi/... ./internal/localapi/identity ./internal/diagnostics ./internal/daemon ./sdk/go/... -count=1` | exit 0; 66.6 s |
| Final denial-audit regressions | `go test ./internal/identity/access ./internal/localapi/identity -run 'TestAdministrationDenialEmitsRedactedAudit\|TestProtectedIdentityMalformedRequestIsAuditedBeforeAdmission' -count=1` | exit 0 |
| Linux integration | `powershell -NoProfile -File tests/run.ps1 integration -EphemeralCache -ReportDir tests/.artifacts/reports/pia018-final-integration-audit` | exit 0; 133/133 passed; 384.686 s reported test duration |
| Linux e2e | `powershell -NoProfile -File tests/run.ps1 e2e -EphemeralCache -ReportDir tests/.artifacts/reports/pia018-final-e2e-audit` | exit 0; 18/18 passed; 274.799 s reported test duration |

The first final integration attempt exposed one stale workload rollback test:
it treated the supported `transport stopped` convergence state as a publication
error. The test now uses an integration-build-only publication fault injector,
so it exercises the intended rollback failure path without changing production
semantics. The focused regression, complete workload integration package, and
full 133-scenario rerun pass.

After the tagged runs, the denial-audit helper gained a one-line `attempted`
guard so an already attempted invalid Session cannot trigger a second audit
admission. It does not alter the successful mutation path exercised by the
tagged suites. The named denial regressions, complete untagged suite, and race
suite were rerun after that guard.

Both tagged runners removed their temporary binaries. `tests/.artifacts/testbin`
is empty, the runs used disposable anonymous Go-cache volumes, and no Go build
or module cache exists in the repository. The post-e2e resource snapshot
reported 254.18 GB free on the workspace drive.

The follow-up Standards review found no blocker/high issue and confirmed
`git diff --check`, cache placement, temporary-artifact cleanup, build-tag
isolation of the fault injector, and durable outbox conventions. The follow-up
Spec review verified the four original blockers against the final worktree.
