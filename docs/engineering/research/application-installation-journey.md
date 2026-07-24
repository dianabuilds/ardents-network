# Application Installation Journey Research Packet

## Decision

- Decision owner: Application Interface / Principal Identity and Access
- Date: 2026-07-25
- Baseline commit:
  `main@180decc1b03f94a6115b59a4046b4795308ec235`
- Research class: R1 bounded investigation
- Recommendation: implement two bounded acceptance slices; retain release
  qualification as R3

The current product has one continuous supported installation path:

```text
Operator creates protected ticket
  -> Application reads ticket and proves its root key
  -> Node atomically enrolls Principal, Credential and initial Access Grant
  -> Application authenticates a memory-only Session
  -> public Go SDK Content.Put
  -> public Go SDK Content.Get
```

The production interfaces and a Linux process E2E already exercise that path.
The remaining local gaps are not missing identity or Content mechanisms. They
are (1) a caller-facing Ticket Handoff completion contract after successful
enrollment and (2) one lifecycle acceptance scenario that proves safe retry,
restart and revocation through supported interfaces. Neither requires a new
wire protocol, trust model or public RPC.

Current maturity for this journey is:

| Dimension | Status | Basis |
|---|---|---|
| Implemented | yes | Operator ticket issuance, Application enrollment/session and Content adapters exist |
| Reachable | yes | Operator CLI plus public Go SDK use only the protected Operator/Application sockets |
| Operable | partial | typed failures, retry and individual lifecycle controls exist; ticket-file cleanup and the combined recovery procedure are not enforced |
| Qualified | no | the Linux process E2E exists but did not run in this Windows research environment; no complete commit-bound release matrix exists |

## User Outcome

An Operator can authorize a prospective Application Principal for exactly
`application.content.put` and `application.content.get`, transfer one
short-lived enrollment secret through a protected file, and let the
Application enroll and use immutable content without receiving an Operator
Credential or importing an Ardents internal package.

After a safe failure, the Operator and Application can tell whether to retry
the same ticket or issue a replacement. After restart, the Application can
reauthenticate and continue using its durable enrollment and content. The
Operator can terminate authority by revoking the Application device
Credential and/or its Access Grant.

## In Scope

- Operator creation of one Application Enrollment Ticket;
- protected file handoff to a same-host Application;
- Application Principal and Key Credential creation under Application-owned
  custody;
- root-possession enrollment on the Application Interface;
- initial Application Access Grant creation;
- Application Session authentication and one refresh;
- direct `Content.Put` and `Content.Get`;
- ticket, session, device and grant lifecycle behavior;
- safe failure, retry and restart semantics;
- public Go SDK reachability;
- current unit, contract and tagged process evidence.

## Out Of Scope

- Application Discovery and AD-01 through AD-04;
- Messaging, Hosting or direct service interaction;
- remote Application transport or SSH forwarding of the Application socket;
- a general Application key store or installer framework;
- streaming or large-content protocol changes;
- new Application actions, resource scopes or Delegation semantics;
- production Channel Grant authority;
- release, multi-host or deployment qualification.

## Current Supported Interface

The supported path intentionally uses two interfaces.

1. The Operator invokes
   `ardentsctl identity application-ticket issue` against the protected
   Operator Unix socket. The command requires the prospective Application
   Principal, one or two registered Application actions, and an absolute new
   `--out-file`. The ticket is never returned in human or JSON output
   (`internal/cli/identity/administration.go:93-177`,
   `docs/operations/cli-principal-sessions.md:30-48`).
2. The Application reads that private file, parses the canonical value with
   `client.ParseApplicationEnrollmentTicket`, and calls
   `client.EnrollApplication` against the protected Application Unix socket
   (`sdk/go/client/enrollment.go:28-79`).
3. The Application creates `client.Client` with its pinned Node Principal and
   typed `SessionSigner`; `Client.Content` and `Client.Session` are the public
   operations (`sdk/go/client/client.go:20-63`).
4. Content calls use `content.Service.Put/Get`; generated Connect messages and
   session refresh remain hidden in `sdk/go/internal/adapter`
   (`sdk/go/internal/adapter/content.go:18-70`).

SSH stream-local forwarding is a supported Operator transport, not an
Application transport. The installation journey is local to the dedicated
Application Unix listener.

## Current Reachable User Journey

| Step | Actor | Credential or secret | Supported operation | State created or used |
|---|---|---|---|---|
| 1. Generate Application identity | Application | newly generated root and device private keys | public `sdk/go/identity` signing helpers | Principal ID and finite Key Credential; custody remains with Application |
| 2. Authenticate Operator | Operator | enrolled Operator device Key Credential | Operator session manager over Operator socket | short-lived Operator Session |
| 3. Issue ticket | Operator | Operator Session plus `identity.principal.enroll` grant | CLI `identity application-ticket issue` | durable ticket digest/state keyed by Node and prospective Application Principal; plaintext only in new private file |
| 4. Read handoff | Application | protected ticket file | Application-owned file read plus public SDK parser | typed in-memory Application Enrollment Ticket |
| 5. Prove root and enroll | Application | ticket, root key, finite device Key Credential | public `client.EnrollApplication` over Application socket | durable Enrollment, Credential and Node-scoped initial Application Access Grant, atomically |
| 6. Authenticate | Application | device Key Credential/private key | `Client.Session.Authenticate` or implicit first protected call | memory-only, Node/interface/peer-bound Application Session |
| 7. Put | Application | Application Session and `application.content.put` grant | `Client.Content.Put` | durable owner binding and verified payload |
| 8. Get | Application | Application Session and `application.content.get` grant | `Client.Content.Get` | verified payload returned only for the Effective Principal's binding |

The tagged process scenario performs all eight steps with real binaries and
sockets in `tests/e2e/applicationapi/process_test.go:31-145`. Its Application
probe imports only `sdk/go/client` and `sdk/go/identity`; its create, enroll,
session and Put/Get path is at
`tests/fixtures/application-probe/main.go:58-157,266-277`.

No production step requires an Ardents `internal/*` import. The fixture owns
key-file encoding because Application key storage is explicitly an embedding
Application responsibility, not an SDK service.

## Current Implementation

### Operator issuance and Ticket Handoff

`runApplicationTicket` validates the Principal and exact registered actions,
refuses an existing or non-absolute output, obtains an authorized Operator
client, issues the ticket and creates a private file atomically
(`internal/cli/identity/administration.go:93-177`).

The access service persists only a domain-separated ticket digest and lifecycle
state. The current record distinguishes `issued`, `delivered` and
`acknowledged`; issuance/reissuance and delivery are durable operations
(`internal/identity/access/application_enrollment.go:21-29,102-214`).

### Principal, Credential, grant and session creation

The Application creates its Principal deterministically from the root public
key and signs a finite Key Credential for its device by using public
`sdk/go/identity` functions. Enrollment validates the Application audience,
root proof and Credential, consumes the delivered ticket, and atomically writes
the Enrollment, Credential, initial Node-scoped Application Access Grant and
acknowledged ticket state
(`internal/identity/access/application_enrollment.go:231-324,455-477`).

The initial actions are exactly those embedded in the ticket. For the current
Content journey both are required:

```text
application.content.put
application.content.get
```

Normal authentication creates a short-lived Application Session bound to the
Node, Application Interface, Unix transport peer and Application Principal.
The SDK cache is process memory only. Authority is re-evaluated from grants on
each Content call; the Session itself carries no permissions.

### Content

Application admission derives the Effective Principal and owner resource. Put
creates the Principal/content binding only after durable content storage. Get
requires the exact owner binding; knowledge of a Content Reference is not
authority. The public SDK maps stable Application error details into typed SDK
errors.

## Existing Deterministic Evidence

Evidence executed on
`180decc1b03f94a6115b59a4046b4795308ec235` during this research:

```text
go test ./internal/identity/access ./internal/applicationapi/... \
  ./sdk/go/... ./internal/cli/identity -count=1
```

Result: passed on Windows using external
`C:\Users\vitek\AppData\Local\Temp\ardents-go-build-cache`.

The passing slice includes:

- protected file creation, refusal to overwrite and safe issuance retry;
- ticket delivery/reissue, one-use concurrency and half-open expiry;
- atomic Enrollment/Credential/Grant persistence and rollback;
- ticket persistence and proof invalidation across restart;
- Application listener binding and session isolation;
- single-flight session authentication and exactly one refresh after
  `Unauthenticated`;
- owner-aware Content admission and SDK typed adapters.

The tagged catalogue contains the Linux process scenario `APP-001`, whose
definition builds real `ardentsd`, `ardentsctl` and Application probe binaries,
uses both protected sockets, enrolls through the public SDK, and performs
Put/Get. Static existence is not a current pass.

## Historical Evidence

The retained stabilization snapshot at
`docs/engineering/evidence/stabilization-baseline-75471a6.md` records passing
local/static gates for commit `75471a6`, but Docker/Linux E2E was unavailable.
It is predecessor evidence only.

The audit files under `docs/audit/2026-07-23/` describe an older commit and are
not used as the current backlog. The current ticket state/retry implementation
and tests supersede any old implementation assumptions.

## Missing Or Unreachable Behavior

### 1. Successful handoff does not retire the plaintext ticket file

The Operator half correctly creates a new private file and never exposes the
ticket in output. The public enrollment API accepts a typed ticket, not a file,
so it cannot safely remove the handoff file. The existing Application probe
reads and enrolls but never removes the file
(`tests/fixtures/application-probe/main.go:113-141,218-229`). The E2E likewise
does not assert its removal.

The server-side digest is acknowledged and one-use, so the remaining file
cannot enroll again. It is nevertheless stale capability material and leaves
the caller without a supported completion procedure comparable to Bootstrap
Ticket cleanup.

The bounded resolution is documentation plus a public SDK helper that owns the
file read/enroll/remove sequence. It must remove only after a validated
successful response. If cleanup fails after commit, it must return a typed
"enrolled but cleanup failed" outcome and must never repeat enrollment.

### 2. Lifecycle controls exist but not as one supported acceptance procedure

- Ticket replacement before enrollment is implemented as reissue, which
  invalidates the older digest. There is no explicit ticket-revoke command.
- `Session.Logout` calls Application `EndSession`, while process exit/restart
  drops memory-only sessions.
- Operator `identity device revoke` invalidates the Application device
  Credential and thereby session refresh.
- Operator `identity grant revoke` revokes the initial or later Access Grant.

These are individually implemented, but the process E2E proves only happy-path
install/use and rejection of an Operator Credential on the Application
listener. It does not prove:

- safe same-ticket retry after a transaction failure;
- stale-ticket rejection after replacement;
- ticket-file cleanup after success;
- reauthentication and Content continuity after Node and Application restart;
- grant revocation denying Content while authentication remains distinct;
- device revocation invalidating the live session and failed refresh.

### 3. No direct ticket-revoke procedure

This is not a blocker for the selected v1 interface: replacement invalidates a
delivered ticket, expiry invalidates it after ten minutes, and successful
enrollment acknowledges it. Adding an explicit revocation RPC/command would
change the public Operator contract and is therefore an R2 candidate unless a
product requirement demonstrates that replacement plus expiry is inadequate.

## Actors, Assets And Trust Assumptions

### Actors

- Operator Principal: may issue/reissue tickets and revoke grants/devices only
  through exact Operator grants.
- prospective Application Principal: owns root and device keys before the Node
  knows it.
- enrolled Application Principal: authenticates on the Application listener
  and receives only finite Application grants.
- Node Principal: signs grants and owns persistent enrollment/access state.
- local service account: obtains filesystem reachability to the Application
  socket; group membership is not Principal authentication or authority.

### Assets

- Application root private key and finite device private key;
- Application Key Credential;
- one-use plaintext enrollment ticket during handoff;
- durable ticket digest and lifecycle state;
- Application Access Grants and revocations;
- memory-only Session secret;
- Principal-owned content binding and payload.

### Trust assumptions

- Operator and Application socket directories and handoff directory enforce
  local OS permissions;
- the Operator pins the exact prospective Application Principal and actions;
- the Application pins the exact Node Principal and validates typed challenges;
- Application-owned key files are created exclusively and never handed to the
  Operator;
- the Node database provides atomic transactions for ticket acknowledgement,
  enrollment, Credential and initial grant;
- clock correctness is sufficient for finite Credential, ticket, grant and
  session bounds.

## Proposed Module Boundary And External Interface

Keep the public boundary in `sdk/go/client`. Add a convenience operation with a
file-owning configuration rather than exposing storage or access internals:

```go
type EnrollmentFileConfig struct {
    SocketPath    string
    NodePrincipal string
    TicketPath    string
    Signer        EnrollmentSigner
    HTTPClient    *http.Client
}

func EnrollApplicationFromFile(
    context.Context,
    EnrollmentFileConfig,
) (EnrollmentResult, error)
```

The existing `ParseApplicationEnrollmentTicket` and `EnrollApplication` remain
available for applications with a different protected delivery mechanism.
The new helper supports the documented same-host protected-file journey and
owns strict private-file validation, bounded canonical decoding, enrollment,
zeroing and post-success removal.

The external Operator interface remains the existing CLI command and RPC.
No protobuf or authorization change is proposed.

## Proposed Internal Seam

Extract a small SDK-internal protected-ticket-file component used only by
`EnrollApplicationFromFile`:

```go
type consumedTicketFile interface {
    ReadCanonical(path string) (ApplicationEnrollmentTicket, error)
    RemoveAfterEnrollment(path string) error
}
```

Production uses strict `lstat`, private-mode, bounded read and exact-path
removal. Tests substitute failure at read and removal. The seam must not accept
relative paths, symlinks, trailing data or an output path that changed identity
between read and cleanup. The cleanup implementation should retain file
identity from the opened handle or revalidate the exact file identity before
removal.

Lifecycle acceptance belongs in the existing process E2E harness and drives
the CLI and SDK interfaces; it must not call the access service directly.

## Dependencies

### In-process

- public SDK enrollment and session adapters;
- public SDK identity signing primitives;
- identity/access ticket, enrollment, session, grant and revocation state;
- Application admission and Content adapter;
- Operator CLI ticket/grant/device commands.

### Local-substitutable

- protected Operator and Application Unix sockets;
- private filesystem handoff and Application key custody;
- transactional local database;
- process E2E binaries and local content store.

### Remote But Owned

None for the required local Put/Get tracer bullet. A cache-miss Get may use the
Node's owned transfer subsystem, but the acceptance slice uses local content
written by the same Application.

### True External

- OS account/group provisioning and filesystem permission enforcement;
- canonical Linux runner for Unix-socket process E2E.

## Alternatives

### A. Keep cleanup entirely in each embedding Application

Plausible but not recommended. It keeps the wire and SDK smaller, but every
Application must independently reproduce strict private-file checks and the
"committed enrollment but cleanup failed" distinction. The current fixture
already demonstrates that omission is easy.

### B. Add `EnrollApplicationFromFile` to the public Go SDK

Recommended. It makes the documented local installation path continuous while
preserving the lower-level typed-ticket API for other protected delivery
mechanisms. The change is local to the SDK and tests.

### C. Make the Operator CLI deliver directly to an Application process

Rejected for R1. It requires process discovery/IPC ownership and a new handoff
protocol, expanding the trust model.

### D. Add an Operator ticket revoke RPC and CLI

Deferred as R2. It changes a public administrative contract and requires a
product decision about revocation identifiers, observability and race semantics
relative to enrollment. Existing replacement, expiry and acknowledgement are
adequate for the selected bounded acceptance path.

## Failure, Retry, Restart And Recovery Behavior

| Condition | Current behavior | Required operator/Application action |
|---|---|---|
| output exists or cannot be inspected | CLI refuses before issuance | choose a new protected path; no Node mutation |
| server issued but delivery/response failed | a later issue replaces non-acknowledged state | issue again; only the newest returned ticket is authoritative |
| protected file write fails | CLI returns failure; durable delivered ticket may exist | issue again to a new path; replacement invalidates the unseen ticket |
| malformed/private-mode-invalid ticket file | Application rejects before RPC | repair delivery permissions or issue a new file |
| challenge/proof fails before transaction | no enrollment commit; proof is single-use | repeat Begin/Complete and retry the same still-valid ticket |
| enrollment transaction fails | no partial Principal/Credential/Grant and ticket remains usable | retry with a new proof and the same ticket |
| enrollment succeeds, response is validated | ticket is acknowledged and cannot be reused | remove the exact protected ticket file |
| cleanup fails after success | enrollment is committed; retrying enrollment conflicts | report enrolled-but-cleanup-failed; securely remove the same verified file |
| Application process restarts | Session cache is lost | reload Application-owned Key Credential/key and authenticate again |
| Node restarts before enrollment | ticket state survives; old proof does not | repeat proof using same non-expired delivered ticket |
| Node restarts after enrollment | Enrollment, Credential, grant and content persist; Session does not | authenticate and continue |
| Session expires or is rejected | SDK evicts exact generation, authenticates and replays unary call once | surface second `Unauthenticated`; do not switch credentials |
| initial grant revoked | Content call is denied even with an otherwise valid Session | Operator may issue a new grant; no automatic SDK retry |
| device revoked | live session becomes invalid and refresh with that Credential fails | rotate/enroll an authorized device through the supported Operator procedure |

Ticket issue/reissue is not idempotent: a successful retry intentionally
creates a new authoritative ticket and invalidates the prior one. Enrollment is
atomic but not an idempotent success lookup; callers must distinguish a
validated success followed by cleanup failure from a pre-commit failure.

## Security, Privacy And Abuse Analysis

- The ticket never belongs on argv, stdout, JSON output, logs or error text.
- The file helper must reject relative paths, symlinks, non-regular files,
  group/world permissions, oversized input, padding and trailing whitespace.
- Cleanup must not follow a path swap and delete an unrelated file.
- Root and device private keys remain Application-owned and never cross the
  Operator or Node interface.
- Enrollment binds the ticket Principal, root-derived Principal, Credential,
  Node, Application audience and transport peer.
- Initial actions are exact and currently capped at the two registered Content
  actions; duplicate or unknown actions fail closed.
- Sessions are not authority. Grant and device revocation are evaluated on
  protected calls and authentication.
- Public failures remain generic enough not to expose another Principal,
  ticket digest, grant details, Content ownership or internal path.
- Repeated ticket issuance is an authorized mutation and should retain stable
  audit reason/outcome labels without secret material.
- Bounded unary Content limits remain unchanged; this research does not create
  a streaming or amplification surface.

## Observability And Operator Actions

The access audit already distinguishes ticket issued/reissued, issuance
denial, Application enrollment and enrollment denial without logging the
ticket. CLI mutation output reports Principal, Node, actions, expiry and
protected output path, never the secret.

The lifecycle acceptance slice should make the following operator procedure
explicit:

1. list the Application's current grants;
2. reconcile a failed ticket issue by issuing a replacement to a new protected
   file;
3. after Application enrollment, verify the handoff file has been retired;
4. use grant revocation to remove an action while keeping identity distinct;
5. use device revocation to invalidate authentication;
6. use audit/diagnostic events to distinguish denial categories without
   exposing ticket, Session or private-key material.

Metrics must use bounded operation/outcome labels and must not include
Principal IDs, ticket paths, Content References or secrets.

## Acceptance Matrix

| Level | Required evidence |
|---|---|
| SDK unit: protected file | absolute regular private file only; symlink, replacement, permissive mode, oversized, padded and trailing input fail closed; secret is redacted and zeroed |
| SDK unit: success cleanup | validated enrollment success removes the same ticket file exactly once |
| SDK unit: failure cleanup | pre-commit/transport/auth failure retains the file for safe retry; cleanup failure returns an enrolled-but-cleanup-failed typed outcome and does not retry enrollment |
| Access unit: ticket retry | unseen/delivery-failed ticket can be replaced; stale ticket fails; transaction rollback keeps current ticket usable; one-use concurrency remains exact |
| Access unit: restart | delivered ticket survives Node restart, proof does not; acknowledged ticket, Enrollment, Credential and grant survive restart |
| SDK/session unit | Session stays memory-only; one refresh occurs only after `Unauthenticated`; `Forbidden` is not retried as authentication |
| Contract: authority | ticket actions become the exact finite Application grant; both Content actions are required for full journey; Operator Credential is rejected on Application listener |
| Contract: revocation | grant revocation denies Content with stable typed `Forbidden`; device revocation invalidates the current session and its one refresh |
| Linux process E2E | real Operator CLI issues protected ticket; public SDK helper enrolls and removes file; Application authenticates, Put/Gets, both processes/Node restart, Application reauthenticates and Gets existing content |
| Linux process E2E negative | replacement invalidates stale ticket; forced pre-commit failure permits same-ticket retry; post-enrollment grant/device revocations have distinct expected outcomes |
| Architecture | process fixture and public example import no Ardents `internal/*`; Operator and Application listeners/schemes remain separate |
| Qualification | canonical Linux/Docker and release gates pass without retry against one exact clean commit; until then `Q=no` |

## Open Questions

No open question can change the proposed public helper or lifecycle acceptance
interface.

Implementation must choose a platform-appropriate exact-file-identity cleanup
mechanism. That is an internal security choice covered by the acceptance
matrix, not a public interface decision.

Whether the product later needs explicit ticket revocation is intentionally
outside this R1 result and must not block the two selected slices.

## Recommendation

Implement the protected-file SDK helper and the lifecycle acceptance tracer
bullet. They deepen the existing public installation journey without changing
the wire, trust model or Application actions.

Do not reopen Application Discovery, add ticket-revocation RPCs, add remote
Application transport or claim release qualification. The current capability
remains `I=yes`, `R=yes`, `O=partial`, `Q=no`.

## Vertically Sliced Issues And Dependency Order

### AIJ-01 - Complete protected Application Ticket Handoff

- Parent: R1 Existing Product Truth / Application installation journey
- User story: As an Application installer, I can enroll from the Operator's
  protected ticket file and know that the one-time plaintext handoff is retired
  only after enrollment commits.
- What to build: public `EnrollApplicationFromFile`, strict file-identity and
  private-mode handling, post-success cleanup, typed committed-but-cleanup
  failure, documentation/example update, and supported-interface tests.
- Acceptance criteria: SDK file matrix and retry/cleanup rows above pass;
  existing typed-ticket API is unchanged; no protobuf/action change; no
  internal imports in caller example.
- Blocked by: none.
- Research class: R1 resolved to implementation.
- Proposed status: `ready-for-agent`.

### AIJ-02 - Prove installation recovery and revocation through real interfaces

- Parent: R1 Existing Product Truth / Application installation journey
- User story: As an Operator and Application owner, I can safely recover the
  installed Application across failure/restart and can distinguish content
  grant revocation from Credential revocation.
- What to build: extend the existing `APP-001` process tracer bullet (or add one
  bounded sibling scenario) to drive Operator CLI and public SDK through ticket
  replacement, same-ticket transaction retry, cleanup, Node/Application
  restart, existing-content Get, grant revoke and device revoke.
- Acceptance criteria: lifecycle E2E rows above pass on the canonical Linux
  runner; test metadata and docs identify exact procedure/outcomes; no internal
  access-service shortcut; current happy path remains one scenario.
- Blocked by: AIJ-01.
- Research class: R1 resolved to implementation; current-head release
  qualification remains R3.
- Proposed status: `ready-for-agent`.

Dependency order:

```text
AIJ-01 protected handoff helper
  -> AIJ-02 supported lifecycle acceptance
  -> existing R3 release qualification program
```

