# Stage 7 lifecycle specification

Status: **accepted and normative for Stage 7 development on 2026-08-20.** Exact
serialization, technology profiles, Installed/Portable and desktop integration,
and numeric resources are fixed by decided R-049–R-054/R-056 and ADR-0015,
ADR-0016, and ADR-0021. Per-run host/artifact identities remain manifest inputs.

This specification defines shared behavior for Ubuntu and Windows. `MUST`,
`MUST NOT`, `REQUIRED`, `SHOULD`, and `MAY` are normative after acceptance.
Platform Adapters may implement the Interface differently but may not rename,
weaken, or omit a shared state or outcome.

## 1. Environment and release identity

Every deployable artifact and metadata root belongs to exactly one environment:

- `development` — local development only;
- `h3-test` — project-controlled Closed Test Network; or
- `public` — future H4 identity, unavailable in Stage 7.

An Endpoint deployment MUST have one immutable environment marker, network identity,
and initial trusted release root. Roots, release floors, Authority state,
Network State, caches, and evidence MUST NOT be imported or merged across
environments. Deploying another environment requires a separately owned
Endpoint state root or explicit destructive removal of the old one; it is not an
update.

The H3 release profile MUST authenticate at least:

| Field | Rule |
|---|---|
| `schema_version` and profile identity | Exact supported value; unknown critical semantics reject |
| environment and network identity | Exact deployed values; no cross-environment recovery |
| release/build identity and version | Canonical, non-empty, monotonic policy inputs; display version is not authority |
| target platform | Exact frozen OS family, architecture, and payload-layout compatibility |
| target length and digest | Verified before staging or execution; source filename is not identity |
| source revision and build-input commitment | Exact immutable source/supply identity used by the H3 candidate |
| dependency/SBOM commitment | Complete declared runtime/build closure for the target |
| builder attestations | Bound to target digest and inputs; H3 records project control and makes no independence claim |
| qualification identity/state | Explicit qualified, development-only, revoked, or unavailable; missing is not qualified |
| build safety and protocol phase | Separate state machines with independent deadlines |
| `no-new-work-after` and `terminate-or-no-recovery-after` | Finite authenticated bounds; local policy may shorten only |
| metadata versions and expiry | Persisted non-decreasing floors and fixed update-start time |

TUF root/targets/snapshot/timestamp separation is REQUIRED unless R-049 rejects
the TUF candidate and a superseding accepted ADR preserves equivalent behavior.
Snapshot and timestamp duties MUST NOT authorize new executable target bytes.
Consistent target identity MUST be content-addressed or otherwise immune to a
mix of repository generations.

H3 test thresholds exercise the `3-of-5` ordinary and `4-of-5` expiring
emergency mechanics using visibly project-controlled test identities. A
threshold count is not evidence of independent custody. Two project-controlled
rebuild records are not independent-builder evidence. Every status/report MUST
state this limitation.

## 2. Retrieval modes

One update operation has exactly one owner-selected mode:

| Mode | Allowed source | Failure behavior | Disclosure |
|---|---|---|---|
| `private-only` | Current qualified Ardents path or preconfigured external privacy proxy | Explicit unavailable; never direct fallback | H3 Ardents path remains unqualified until S9.6; proxy has its own trust/privacy boundary |
| `direct-allowed` | Explicit ordinary network source | Fail or retry only within precommitted finite source list | Source sees origin, platform, exact target/digest, timing, repetition, probable Ardents use |
| `offline-import` | Owner-supplied local complete artifact set | Reject incomplete/untrusted bytes; no network fallback | Local media/path and operator may observe artifact choice |

Metadata lookup MUST carry no account, installation ID, Service/Target/Name
list, rollout cohort, `from-version`, or exact deployed history. Distribution
bytes MUST be verified identically in every mode. Selecting a mode never grants
deployment permission; local lifecycle policy remains separate.

If Release Safety is expired or the build is revoked, Ardents MUST NOT open a
new Ardents Service Route to repair itself. Only a preconfigured external privacy
proxy, explicit `direct-allowed`, or `offline-import` may proceed. Local status,
repair, and Authority export remain available.

## 3. Release and protocol state

These machines are independent:

```text
protocol: announced -> overlap-supported -> preferred -> required -> retired

build:    current -> superseded
          current | superseded -> vulnerable -> revoked
```

An old protocol does not imply a vulnerable build. A compatible build does not
override revocation. A superseded authenticated safe build may be a rollback
target; a vulnerable build may continue only within its exact signed deadlines;
a revoked build starts no new network work.

### 3.1 Trusted release-root transition

The active root version is a non-decreasing security floor. An automatic
check MUST fetch and validate consecutive root versions from the active root
to the newest available version, within R-049's frozen count and byte bounds.
Each next root MUST be signed by the current root threshold and its own declared
root threshold, preserve the exact environment/network identity, and define
non-zero bounded role/key thresholds. A skipped version, version reuse,
one-sided threshold, cross-environment root, expired candidate, unknown critical
field, or floor decrease is `release-invalid` and changes no trusted root.

The client durably publishes each fully verified next root before relying on it
and never infers trust from a package, distributor, snapshot, timestamp, or
executable signature. A root transition authorizes future metadata verification;
it does not itself authorize executable bytes. Emergency authority cannot add a
release root or executable target.

### 3.2 Automatic safety refresh and protocol transition

The Endpoint and every Contributor role MUST automatically refresh authenticated
Release Safety at startup, before accepting new work, and before the current
Work Safety Lease expires. The request carries none of the identifiers forbidden
in section 2. Failure or uncertainty shortens capability availability; it never
extends a lease or silently enables direct retrieval.

An ordinary protocol generation may become `required` only when both conditions
are authenticated:

1. at least `90 days` have elapsed since the generation entered
   `overlap-supported`, with the previous generation still supported; and
2. every Role Domain plus required control/discovery role has qualified
   current-generation capacity and bounded drain reserve.

Missing, stale, conflicting, or insufficient capacity evidence blocks
`required`. Build revocation does not satisfy or bypass this protocol gate.

Only an expiring `4-of-5` emergency transition may shorten the overlap or bypass
the capacity gate. It MUST name a credible exploitable flaw, compromised
primitive/key, or demonstrated safety incompatibility; bind a finite expiry;
stop or shorten affected work without adding any executable/root authority; and
report possible network unavailability. Ordinary metadata MUST ratify or replace
it before expiry. Expiry without ratification restores no retired or unsafe work;
the affected capability remains unavailable pending an ordinary current policy.

The accepted decision for one local target is exactly one of:

- `release-accepted` — artifact may be staged under local policy;
- `no-update` — active accepted target is current;
- `update-required` — current build/protocol cannot start new work;
- `release-expired` — freshness cannot be established;
- `release-conflict` — authenticated inputs disagree;
- `release-revoked` — target opens no new network work;
- `release-incompatible` — environment/platform/schema/protocol cannot run;
- `release-unavailable` — no complete current metadata/artifact set; or
- `release-invalid` — malformed, unauthorized, rollback, mix, digest, size,
  or policy input.

These runtime outcomes are not verifier `invalid`.

## 4. Deployment state ownership

Program artifacts and protected Endpoint state have different owners. Exact
Installed paths, Portable artifact contents, state-root selection/locking, and
ACL/mode rules are frozen by R-050/R-053 per platform.

| State class | Owner | Update/repair/removal rule |
|---|---|---|
| Installed package and registration | Install Lifecycle | Owns only enumerated package objects; repair/update/uninstall cannot rewrite protected state implicitly |
| Portable executable and unavoidable declared companions | Endpoint Owner | Authenticated release artifact copied, run, replaced only while stopped, or deleted directly; no separate lifecycle Adapter or implicit system object |
| immutable environment and initial release roots | Owning Endpoint Modules | First start creates/selects protected state independently of program location; program replacement or Distribution Profile change cannot rewrite/merge it implicitly |
| Installed stable bootstrap and optional platform registration | Install Lifecycle | Changed only by an authenticated Installed repair/update path; no Portable equivalent is required |
| immutable versioned payloads | Update Transaction | Created only after release acceptance; never mutated after staging verification |
| activation record and transaction journal | Update Transaction | Bounded, atomic/durable, contains no Authority secret |
| Authority Vault and authority signing watermarks | Authority Custody | Preserved by repair/update/removal; purge only by separate confirmed action |
| Endpoint config, Local Grants, runtime Instance Keys/Credentials | Owning Endpoint Modules | Preserved by repair; never derived from Bundle; policy decides removal retention |
| release/epoch/Namespace/freshness/generation/rollback floors | Owning security Modules | Non-decreasing across update/rollback/repair; normal removal retains required floors |
| authenticated disposable caches | Owning domain Modules | May be discarded and rebuilt; cannot become a watermark source |
| ephemeral routes/connections/sessions/process handles | Runtime owner | Never survive process restart or update |
| diagnostics and evidence spool | Diagnostics/evidence owner | Finite, grant-scoped, secret-free export; removed per explicit retention policy |

Unknown entries inside an owned root MUST fail inspection or be preserved without
mutation until a bounded explicit owner decision. Symlinks/reparse points,
unexpected mount/volume transitions, path traversal, device files, hard-link
confusion, or insecure ownership/permissions MUST fail closed.

## 5. Update transaction

This managed transaction applies to Installed. Portable has no bootstrap or
activation Adapter: it follows the authenticated stopped-replacement path in
Section 6. A trusted existing verifier MUST authenticate the candidate and the
Owner MUST recheck its exact digest after replacement but before execution. The
authenticated Portable executable then MUST enforce the same release floors,
state compatibility, copy-on-write state migration, self-test, Authority
preservation, and safe previous-build rules before new network work.

### 5.1 States

```text
idle
  -> release-accepted
  -> artifact-verified
  -> staged
  -> rollback-reserved
  -> stop-new-work
  -> draining
  -> activated
  -> self-testing
  -> committed

release-accepted .. self-testing
  -> rollback-pending
  -> rolled-back

activated | rollback-pending
  -> repair-required
```

Every durable transition has a monotonically increasing transaction generation,
previous-state commitment, selected release digest, platform Adapter result,
monotonic observation time, and bounded deadline. Restart MUST recover one
unambiguous state or report `transaction-invalid`; it MUST NOT infer commit from
the presence of files or process success alone.

### 5.2 Preconditions

Before `stop-new-work`, the transaction MUST have:

- one `release-accepted` decision for the exact artifact/platform/environment;
- verified complete immutable staged payload;
- finite disk/inode/entry budget and one authenticated safe rollback payload;
- schema compatibility plan and copy-on-write destination;
- current applicable Work Safety deadlines;
- an enumerated process/duty drain plan; and
- an immutable transaction manifest and cleanup inventory.

Failure before these preconditions changes no live work or current activation.

### 5.3 Drain and activation

New work stops at the earlier of authenticated or local deadline. Existing work
drains only within the earlier bound; owner policy never extends signed safety.
Process replacement may close Service Connections honestly and MUST NOT replay
an Application operation.

Activation atomically replaces one bounded activation record pointing to an
already verified immutable version directory. The Adapter MUST document and
test same-volume/filesystem requirements, durability operation, ACL/mode
preservation, open-executable behavior, and crash outcomes. Unsupported storage
returns `activation-unsupported`; it never degrades to in-place overwrite.

### 5.4 Contributor drain and rejoin

For a Contributor deployment, `stop-new-work` rejects new assignments for
every active role before process replacement. Each role drains only until the
earliest local, authenticated transition, credential, assignment, epoch, and
Work Safety Lease deadline. Update commit does not revive old assignments,
handles, identities, or drain state. Rejoin requires fresh current Release
Safety, protocol/build eligibility, assignment, and role credentials; otherwise
the Contributor remains stopped and may withdraw. A partial role drain or crash
cannot be reported as a successful update while old duty survives.

### 5.5 Self-test and commit

Self-test verifies at least payload digest, environment/root identity, activation
generation, schema readability, Authority Vault non-mutation, release floors,
local IPC readiness without ambient listener, and required capability-specific
offline/online state. Network unavailability alone does not prove payload
failure.

Commit makes the copy-on-write mutable schema current and releases only staging
resources no longer required for safe rollback. It never lowers a security or
authority watermark.

### 5.6 Rollback and repair-required

Rollback is allowed only to a retained payload that is:

- authenticated by the active release roots;
- exact-digest verified;
- schema-compatible with the not-yet-committed mutable state;
- non-revoked and permitted by current build/protocol policy; and
- at or above every applicable local rollback floor.

Rollback reactivates code only; it does not roll back Authority, Network Epoch,
Namespace, release, freshness, generation, exposure, or signing state.

If neither forward start nor safe rollback works, state is `repair-required`.
Normal networking remains stopped. Inspection, offline/direct repair under
explicit mode, Authority export, and bounded diagnostics remain available.

## 6. Deploy, repair, remove, and purge

### Installed deployment

Installation verifies platform package integrity plus embedded H3 release/root
identity, creates only enumerated owned roots, applies owner-only ACL/modes,
installs the stable bootstrap, and creates an unprivileged default Endpoint.
It MUST NOT create a User/account/wallet, Service, Name, Publisher, Contributor,
remote listener, remote administration path, or Authority.

Elevation, if required, is limited to platform package/registration and the
explicit isolation helper selected by R-052. Elevated code MUST NOT receive an
Authority Vault, Bundle secret, Local Grant, Service Instance Key, or
Application Data.

### Portable delivery

Portable is the authenticated platform executable plus only unavoidable
authenticated non-secret static configuration templates/resources. Mutable
configuration remains protected Endpoint state outside the artifact. There is no Portable installer, stable
bootstrap, platform lifecycle Adapter, or implicit package, service, startup,
URI, browser/native-host, proxy, DNS, route, VPN, or machine-wide registration.
The Owner copies it to a chosen path and runs it directly without elevation.

The Owner or an already trusted verifier MUST authenticate the exact executable
digest before first execution and after copying/replacement. A raw executable
cannot authenticate itself after untrusted bytes have started; executing an
unchecked Portable artifact is outside the Ardents security/privacy claim.

First start creates or selects protected Endpoint state outside the program
artifact. Repair is trusted re-verification or re-acquisition of the artifact.
Update is an authenticated replacement while the Endpoint is stopped; restoring an older
artifact remains subject to release/rollback floors before network work. Moving
or deleting the executable never implies moving, merging, or deleting protected
state. Optional per-user URI/browser integration, if selected by R-056, is a
separate explicit and reversible action rather than part of Portable delivery.

Portable MUST expose the same direct-binary, Client/Publisher, Application
Interface, resource, state-compatibility, and security/privacy-claim behavior as
Installed. It need not reproduce package-managed install, automatic activation,
rollback, repair, or uninstall convenience.

### Repair

Installed repair re-verifies/replaces immutable package artifacts and restores
only declared Installed registration. Portable repair is stopped artifact
replacement and owns no registration. Both preserve environment roots, Vault,
configuration, Grants,
credentials, and every non-decreasing floor unless a specific item is proven
corrupt and the owner contract says to lock rather than overwrite it. Repair
never treats an old package/archive as authorization to lower state.

### Remove

Installed uninstall removes only its owned package/runtime/registration state
and retains state required to prevent unsafe rollback. Portable program removal
is direct Owner deletion of the stopped executable and declared companions; it
does not invoke an Ardents lifecycle and never authorizes protected-state
deletion. A separate supported protected-state removal MUST preserve a non-empty
Vault plus required floors in place or block until an Owner-chosen Authority
Recovery Bundle is exported and test-verified. It MUST NOT invent a passphrase,
destination, cloud backup, or help-desk key.

### Destructive purge

Purge is a separate explicit action. Before mutation it enumerates Authority
classes, persistent floors, configuration, diagnostics, and external Bundle
paths in scope; distinguishes disposable cache; states which recovery becomes
impossible; and warns that snapshots/backups/filesystem behavior may retain
copies. Cancellation changes nothing. Evidence records commitments and results,
never plaintext secrets or Name/Target identifiers.

## 7. Authority recovery

An Authority Recovery Bundle is versioned, encrypted, bounded, and bound to
environment/network/root identity. It contains Authority root material,
authority-owned generation/revision commitments, and signing watermarks. It
MUST NOT contain Local Grants, runtime Service Instance Keys, session/bearer
state, Route state, Application Data, plaintext Name/Target labels in filenames,
or an automatically chosen recovery destination.

The active exact R-053 O2 candidate is frozen in the
[Authority Custody specification](stage-7-authority-custody-spec.md): separately
purposed/passworded canonical Vault and Bundle envelopes, fixed
Argon2id/AES-256-GCM profile, encrypted-only atomic persistence, and no DPAPI,
Secret Service, automatic unlock, or platform-account recovery assumption. The
Product Owner accepted the profile and explicit native-host qualification
deferrals in ADR-0021; scheduled development evidence remains a slice gate.

States:

```text
absent -> exported -> test-verified
test-verified -> restored-isolated -> authority-locked
authority-locked -> reconciled -> active
authority-locked -> export-only
```

Test restore runs with no network and no signing Interface. A restored Authority
MUST reconcile current authenticated network/Namespace state and advance
strictly beyond every applicable accepted generation/revision before the first
signature. Unavailable, stale, conflicting, forked, wrong-environment, or
rollback state remains `authority-locked` and export-only. A new Service host
creates a new Instance Key. Local Grants are explicitly reissued or restored by
a separate local-policy mechanism; they never derive from Authority.

## 8. Application Principal and local session

### Principal lifecycle

```text
declared -> launching -> os-bound -> channel-bound -> active
active -> draining -> revoked
active -> revoked
launching | os-bound | channel-bound -> denied
active -> expired
```

For the active R-051 O2 candidate, the broker creates one unpredictable start
identity before launch and binds:

- exact executable/launch policy where declared;
- OS token/UID/SID or sandbox identity and session;
- complete non-breakaway process-tree owner;
- exact private inherited channel;
- parent Local Grant and operations;
- Isolation Context and resource parent;
- broker/Endpoint start identity; and
- finite deadline.

PID, UID/SID, desktop user, pipe/socket path, loopback port, process image, or
copyable capability alone is insufficient. Named owner-scoped endpoints remain
generic/coarse only. The platform Adapter MUST combine the accepted facts from
R-051 and fail closed if any fact cannot be obtained or changes. A failed
Windows impersonation or Linux credential/process/container query MUST NOT
continue under broker privilege. The exact active candidate and unresolved
Windows Go-surface constraint are frozen in the
[Application Principal specification](stage-7-application-principal-spec.md).

A session capability MAY protect framing but works only on the already bound
channel. It is one-start/session scoped, replay-protected, non-exportable by the
Interface, and invalid after Application/broker/Endpoint restart, grant
revocation, channel replacement, or deadline.

Connection, per-Service Administration, and Authority Custody grants remain
disjoint. Revocation denies new work and invalidates descendants. Custody/admin
closes immediately; data closes immediately unless finite drain was explicitly
selected first.

### Direct binary Adapter

Both Distribution Profiles MUST expose the same supported direct-binary path to
the Application Broker and Connection Interface with no browser, extension, URI
registration, daemon, or mandatory SDK. It accepts only explicit Ardents
destination, operation, Isolation Context, deadline, and declared stream/
configuration input; uses the same Local Grant and resource rules; and returns
the same bounded connection/result classes as any external Application. When
the command Adapter and Broker boundary are co-resident, the fresh invocation
is class `direct-invocation` with claim `none`; it does not invent a local peer
or malicious-sibling claim. Any external Broker hop must instead meet the
generic or launcher-bound R-051 contract.

The command Adapter owns parsing, bounded input/output/stream adaptation, result
presentation, cancellation, and exit translation only. It MUST NOT reimplement
or bypass release trust, destination resolution, Broker authorization, Route,
retry, update, or isolation behavior. The exact `connect`/`accept` grammar,
stdio half-close, result schema, exit mapping, and browser handoff are frozen for
platform falsification in the
[Application Adapter specification](stage-7-application-adapter-spec.md).

### Generic profile

The generic profile may group indistinguishable same-user Applications into one
local trust domain. Its readiness/result MUST include:

```text
principal_scope = coarse | launcher-bound
application_networking = unverified
malicious_sibling_isolation = unqualified | qualified
```

It MUST NOT claim Application-level Endpoint Location Privacy or direct-network
denial merely because Ardents carrier traffic is protected.

## 9. Network-isolated profile

The isolated profile is launcher-bound and fail-closed. Before untrusted code
runs, the selected Adapter MUST establish:

- complete process/helper tree ownership with no permitted breakaway;
- closed unrelated inherited handles/descriptors;
- only one scoped Ardents local IPC path/handle and explicit standard I/O;
- deny-by-default ordinary IPv4/IPv6/UDP/raw/QUIC/DNS and external local-network
  access, plus no ordinary ingress/listener;
- declared per-context read-only/read-write filesystem roots and separated
  cache/origin/storage;
- no privilege elevation, debugger/broker escape, or alternate user/session
  channel outside the profile;
- finite CPU, memory, process, handle/FD, storage, and deadline parents; and
- authoritative termination and cleanup observation.

Minimum hostile operations from both client and publisher trees:

1. DNS through system resolver, raw configured server, and helper process;
2. TCP/UDP/QUIC socket connect to controlled IPv4/IPv6/local-network observers;
3. bind/listen on wildcard, loopback, and host interfaces followed by external
   scan/connect;
4. HTTP fetch, redirect, proxy/environment discovery, callback/webhook, SSRF,
   WebSocket, and WebRTC/STUN-style attempts;
5. spawn child, grandchild, shell, interpreter, helper, and delayed process;
6. inherit/duplicate/pass a network handle or escape process-tree ownership;
7. attach to another Application's IPC/storage/context; and
8. retry after broker, Application, and Endpoint restart.

Any listener, packet, DNS query, successful external connection, cross-context
storage access, surviving child, or transparent generic fallback is
`isolation-breach`. Unsupported Application behavior is
`isolation-unsupported`, not success.

## 10. Shared runtime outcomes

Every Interface returns a bounded machine class plus safe human detail. It MUST
NOT expose secrets, raw Route/Node identity, other Application/Service state, or
an attacker diagnosis without evidence.

Required classes include:

- `invalid-input`, `unauthorized`, `principal-denied`, `principal-expired`;
- `release-invalid`, `release-expired`, `release-conflict`, `release-revoked`,
  `release-incompatible`, `release-unavailable`, `update-required`;
- `resource-denied`, `staging-failed`, `drain-expired`,
  `activation-unsupported`, `self-test-failed`, `rolled-back`,
  `rollback-refused`, `repair-required`, `transaction-invalid`;
- `authority-locked`, `reconciliation-unavailable`, `bundle-invalid`,
  `bundle-test-verified`;
- `application-networking-unverified`, `isolation-unsupported`,
  `isolation-breach`; and
- `uninstall-blocked`, `uninstalled`, `purged`, `cleanup-incomplete`.

Candidate runtime outcomes are separate from verifier `pass|fail|invalid`.

## 11. Limits to freeze in S7.0

R-049–R-054 and R-056 MUST freeze before S7.1:

- metadata/file count and byte limits, signature/role/delegation depth, target
  size, update-start time and expiry behavior;
- source attempts, retries, timeouts, bandwidth, staging and rollback disk/inode
  reserve, retained payload count, transaction journal entries;
- drain and terminal deadlines, self-test duration, restart recovery attempts;
- Installed program/state root entries and registration; Portable executable
  and unavoidable companion count/size, stopped replacement, state-root lock,
  path lengths, ACL/mode, and filesystem rules;
- Vault/Bundle size, KDF/encryption profile, export/test/reconcile deadlines,
  password attempts, and memory handling;
- IPC frame/session counts, Applications, processes/helpers, handles/FDs,
  goroutines, queues, timers, CPU, memory, and storage;
- isolation probe destinations/ports/protocols, process-depth/count, observation
  windows, restart cycles, and cleanup deadlines;
- exact per-platform URI objects/quoting, default-browser identity,
  loopback/origin behavior, ordinary Internet/VPN coexistence, registration
  cleanup, unsupported isolated-browser result, explicit limitation, and no-
  fallback observations from the frozen Adapter contract;
- R-054 numeric parents used by the frozen direct-binary destination/operation,
  output/exit, byte-stream, cancellation, deadline, HTTP, and resource contract;
  and
- evidence schema, paths, hashes, clocks, sequence, mutation cases, campaign
  identity, episode count, and verdict predicates.

No placeholder constant or “reasonable default” may cross the coding gate.
