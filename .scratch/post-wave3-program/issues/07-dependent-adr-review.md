# PW3-07: Review dependent ADR-0013, ADR-0014, and ADR-0015

Status: ready-for-human
State: open
Labels: ready-for-human
Research class: dependent decision review

## Parent

`../PRD.md`

## User story

As a maintainer, I can review the bounded multi-host, discovery-only service
handoff, and Application conversation decisions after their respective
authority or Hosting dependencies are accepted, without accidentally
authorizing implementation or qualification.

## Complete review behavior

Perform three independent maintainer reviews:

1. ADR-0013, bounded multi-host reachability;
2. ADR-0014, discovery-only Direct Service handoff;
3. ADR-0015, authority-backed bounded Application conversations.

Each review records its own `review-ready`, `returned with blockers`, or
`rejected with rationale` result. ADR-0013 and ADR-0015 require explicit
ADR-0011 acceptance and a compatibility recheck against the accepted authority
text. ADR-0014 instead requires explicit ADR-0012 acceptance and a recheck
against the accepted Application Discovery boundary. One review outcome does
not block an otherwise ready independent review.

This issue reviews decisions only. It does not start MR, DSI, or AM
implementation, create a topology, expose Messaging, create a Direct Service
adapter, execute qualification, or change capability claims.

## In scope: ADR-0013

- exactly three `service_node` Nodes on three operator-owned Linux amd64 hosts;
- separately supported `private_lan` and `public_direct` variants;
- at least two independently hosted bootstrap and persistent Store Nodes;
- at least two static cross-host recovery peers per Node and optional,
  replaceable signed DNS;
- one verified translated private address per Node for private LAN and one
  AutoNAT-gated public TCP/WSS address per public Node;
- five bounded topology operations: `validate`, `status`, `rollout`, `fence`,
  and `recover`;
- workstation-side, host-key-pinned `ardentsctl --ssh` stream-local forwarding,
  workstation-held signer, and per-Node sessions;
- host-local Node state ownership and coordinator-owned manifest/journal only;
- one authority slot, distinct authority consistency group, independent
  checkpoint repository, bounded clocks, fencing/rejoin, upgrade and recovery;
- explicit unsupported topology and transport boundaries.

## In scope: ADR-0015

- opaque realm-scoped conversations and five bounded Application operations:
  list, send, receive, acknowledge, and delivery inspection;
- Operator-only create, membership change, delivery-Node rebind, close, and
  recovery procedures;
- fresh authority-owned Application-channel generation for every membership
  version;
- durable local send acceptance, transactional idempotency/message/outbox, and
  bounded per-recipient delivery projection;
- recipient authorization before replay admission and inbox persistence before
  Node receipt;
- Node receipt distinct from Application consumption acknowledgement;
- recipient-local cursor order only and bounded long polling;
- 32 KiB inline payload or at most sixteen immutable Content References;
- TTL, backpressure, retry, expiry, revocation, restore, migration, privacy,
  abuse, and compatibility boundaries;
- no Waku, selector, Channel Grant, generation, Store, encryption, recipient,
  or retry control in the Application API.

## In scope: ADR-0014

- Ardents responsibility ends after authenticated, authorized,
  privacy-filtered Application Discovery resolution;
- Resolve returns bounded locator facts and does not grant permission to
  connect, invoke, read, write, or authenticate to the service;
- no Ardents `Do`, `Dial`, proxy, tunnel, client adapter, service Credential,
  Access-Grant translation, automatic retry, DNS resolution, or TLS pinning;
- ordinary Application-owned HTTP/TCP clients own dial, deadlines,
  cancellation, TLS, credentials, methods, paths, schemas, payload limits,
  redirects, retry, idempotency, service errors, and audit;
- existing Operator-published HTTPS uses normal PKIX with URI-host IP identity;
- Node-signed discovery facts attest only the publisher's locator statement,
  not the identity or authorization semantics of the process at that locator;
- revocation blocks a later Resolve but cannot terminate an external
  connection already created by an Application;
- Application-owned HTTPS Hosting remains outside v1.

## Out of scope

- modifying or accepting ADR-0011 in this issue;
- MR-01 through MR-08, DSI-01 through DSI-04, or AM-01 through AM-06
  implementation;
- choosing topology hosts, provisioning WAN/NAT/firewall/PKI, or creating a
  checkpoint repository;
- freezing Messaging implementation-profile constants beyond recording the
  pre-implementation decision gate;
- Application-owned conversation administration, remote Application
  transport, non-Go SDKs, arbitrary node counts, automatic NAT traversal,
  schedulers, Kubernetes, suppressed transports, exactly-once/read receipts,
  federation, or MLS;
- changing `I`, `R`, `O`, or `Q`.

## Dependencies and review gates

- ADR-0013 and ADR-0015 are hard blocked by explicit maintainer acceptance of
  ADR-0011.
- ADR-0014 is independently blocked by explicit maintainer acceptance of
  ADR-0012 and confirmation that AD-01 through AD-04 preserve the selected
  Discovery boundary.
- ADR-0013 must be rechecked against the accepted authority rules for
  protected reachability, deployment fencing evidence, survivor active
  receipts, authority/checkpoint placement, clock bounds, restore freshness,
  mixed-generation behavior, and authority migration order.
- ADR-0015 must be rechecked against the accepted authority rules for
  Application-channel creation, fresh membership generations, revocation,
  fencing, receipts, checkpoint truth, and restore.
- ADR-0014 must be rechecked against the accepted Hosting contract so that
  Hosting Status does not expose/open endpoints and Discovery remains the sole
  Ardents locator disclosure seam.
- ADR-0013 and ADR-0015 reviews may run in parallel after the ADR-0011 gate.
  ADR-0014 may run independently after its ADR-0012/Discovery gates.
- ADR acceptance authorizes the design only. Separate tracker transition and
  review are required before MR-01, DSI-01, or AM-01 implementation.

## Authority and state ownership checks

### Multi-host

- DR-03/ADR-0011 remains the sole owner of realm membership, Channel Grant
  issuance, activation, revocation, checkpoint truth, and acceptance of
  fencing evidence.
- Deployment owns topology intent, host-local plans, rollout/fence journals,
  enforced isolation, and `DeploymentFenceEvidence`.
- Each Node remains sole owner of its Node Principal, Waku key/Store, runtime,
  capability state, and stopped-Node consistency group.
- The coordinator has no Ardents Principal and gains no authority from SSH.
- Waku Peer ID, TLS identity, SSH host identity, Node Principal, and Operator
  Principal remain distinct.

### Messaging

- Messaging owns conversation projection, idempotency, immutable messages,
  inbox/outbox entries, subscriptions, cursors, receipt projection, expiry,
  bounded retry state, and backup checks.
- The Realm Authority alone owns Application-channel grants, generations,
  revocations, survivor activation, fencing acceptance, and checkpoint truth.
- The Application owns caller idempotency input and local consumption behavior,
  not conversation membership or transport policy.
- Waku Store is a carrier/cache and never reconstructs authoritative accepted
  message, inbox, outbox, or cursor truth.

### Discovery-only handoff

- The authenticated Application Session and exact Discovery Access Grant own
  Resolve admission; a returned locator creates no new Ardents authority.
- Discovery owns only its maintained record snapshot, trust/policy projection,
  withdrawal/freshness checks, endpoint eligibility, deterministic ordering,
  deduplication, and response cap.
- The Application and remote service own all connection, TLS, credential,
  service authorization, protocol, retry, and application-error state.
- Ardents persists no connection, TLS session, service Credential, request
  replay, or remote application audit state.

## Bounds checklist

### ADR-0013

- exactly three hosts/Nodes and at least two Store/bootstrap providers;
- at least two static recovery peers per Node;
- at most four DNS roots and 128 DNS results;
- exactly one supported advertised address per Node/mode;
- one in-flight Node mutation;
- 65,536 immutable checkpoint heads, never pruned or overwritten;
- no more than 30 seconds absolute inter-host skew and a 60-second authority
  validity safety margin;
- finite routes, timeouts, retries, journals, pages, and metric labels.

### ADR-0015

- two through thirty-two members per conversation;
- at most 128 conversations and sixteen subscriptions per Principal;
- at most 1,024 conversations and 128 subscriptions per Node;
- page limit at most 100, long poll at most 30 seconds, acknowledgement advance
  at most 100 entries;
- inline payload at most 32 KiB or at most sixteen Content References;
- TTL minimum one minute, default 24 hours, maximum seven days;
- finite inbox/outbox message and byte capacity, receipt retention, retry
  count/backoff/deadline, query pages, audit retention, and labels.

Exact Messaging capacity and retry-profile constants may remain versioned
implementation constants, but maintainers must record that they are frozen
before the first implementation slice is accepted.

### ADR-0014

- Resolve returns at most eight deterministic eligible targets.
- Only literal, non-loopback, policy-eligible endpoint hosts are returned in
  v1; DNS names remain ineligible.
- A caller may re-resolve at most once after a bounded pre-effect connection
  failure; ambiguous partial effects are never replayed automatically.
- All caller deadlines, payload/stream sizes, redirect policies, retries,
  idempotency windows, and service error bodies are Application/service
  concerns and remain finitely bounded outside Ardents.

## Restart, recovery, and failure checks

### ADR-0013

- Topology and fence journals are durably written before external mutation and
  are authoritative after coordinator restart.
- A new rollout cannot start while compensation or recovery is pending.
- Fencing is terminal only after isolation evidence remains enforced, the
  authority removal checkpoint is independently retained, and both survivors
  acknowledge active state.
- Authority, repository, clock, or survivor failure yields
  `recovery_required`, never false `fenced`.
- Rejoin never erases removal or reuses old grants; it requires a fresh
  authority membership/generation checkpoint.
- Ordinary compatible rollout upgrades the authority host last; an authority
  schema/protocol migration follows authority-first, stopped-member,
  fresh-generation order.
- Node and authority partial restore, stale checkpoint, identity mismatch, and
  incompatible schema fail closed.

### ADR-0015

- Send success means durable local outbox acceptance only.
- Crash after acceptance resumes internal delivery from the original durable
  message/idempotency truth; the caller does not invent a new key.
- Inbox authorization and replay/dedupe commit before the recipient Node emits
  `received`.
- Full/offline recipient remains bounded pending until receipt or terminal
  expiry/revocation/fencing.
- Messaging persistence, replay state, cursors, and authority checkpoint
  reference form one stopped-Node consistency contract.
- Same-realm restore cannot precede the latest independent authority head;
  stale/partial restore becomes recovery-required.
- Waku Store replay never recreates missing authoritative state.
- Incompatible upgrade fails preflight; downgrade requires a complete stopped
  backup.

### ADR-0014

- A consuming Node restart reloads the maintained discovery snapshot and
  re-evaluates freshness, withdrawal, trust, route policy, and eligibility.
- A publishing Node restart may leave an old record visible only until its
  signed expiry; Discovery does not probe the endpoint to repair it.
- Trust or policy revocation affects the next Resolve but cannot revoke an
  external TCP/TLS connection already handed to the Application.
- Resolve failure, transport failure, TLS failure, service authorization
  failure, and application-protocol failure remain distinct; Ardents does not
  collapse them into an invocation result.

## Acceptance criteria

### ADR-0013 review

- [x] The support matrix is finite and does not turn QA Compose topology into
      production real-host evidence.
- [x] Private-LAN and public-direct claims have distinct truthful admission and
      withdrawal rules.
- [x] Coordinator, SSH, signer, session, Node, authority, and checkpoint state
      ownership are unambiguous.
- [x] Fence/rejoin semantics preserve accepted ADR-0011 authority ownership
      and do not create a second membership authority.
- [x] Restart, compensation, recovery, backup/restore, clock, rollout,
      migration, and downgrade behavior fail closed.
- [x] Unsupported node counts, schedulers, automatic NAT traversal, remote
      APIs, and suppressed transports remain explicit.
- [ ] Maintainer records an independent ADR-0013 review outcome.

### ADR-0015 review

- [ ] The Application interface exposes only the five bounded product
      operations and exact action/resource admission.
- [ ] Operator membership and DR-03 authority boundaries remain intact.
- [ ] Durable acceptance, Node receipt, Application acknowledgement,
      idempotency, ordering, retry, expiry, and terminal outcomes are not
      conflated.
- [ ] Content References do not grant fetch authority or cause send-time fetch.
- [ ] Restore/restart cannot resurrect stale membership or silently lose
      accepted messages.
- [ ] Remaining finite implementation-profile constants are a named gate
      before AM implementation acceptance.
- [ ] Exactly-once, global/causal ordering, read receipt, arbitrary topic,
      caller recipients, federation, and public Waku controls remain rejected.
- [ ] Maintainer records an independent ADR-0015 review outcome.

### ADR-0014 review

- [ ] The final Ardents action is exactly bounded Discovery resolution, not
      dial, invocation, proxying, Credential delivery, or service
      authorization.
- [ ] Resolve authority is explicitly distinct from permission to use the
      returned service.
- [ ] Ordinary HTTP/TCP and PKIX responsibilities remain with the Application
      and service.
- [ ] Node-signed locator facts are not presented as TLS process identity or
      an authorization proof.
- [ ] Revocation and restart semantics do not claim control over an external
      connection after handoff.
- [ ] Application-owned HTTPS, DNS targets, TOFU, discovery pinning, implicit
      retry, and an Ardents Direct Service adapter remain unsupported.
- [ ] Maintainer records an independent ADR-0014 review outcome.

## Required checks and evidence

- Manual comparison:
  - ADR-0013 with the multi-host packet, accepted ADR-0011, ADR-0006,
    ADR-0008, ADR-0009, and Wave 3 synthesis;
  - ADR-0015 with the Messaging packet, accepted ADR-0011, ADR-0002,
    ADR-0003, ADR-0004, and Wave 3 synthesis.
  - ADR-0014 with the Direct Service Interaction packet, accepted ADR-0012,
    Application Discovery, Application Hosting, and Wave 3 synthesis.
- Documentation-contract and architecture-acceptance checks.
- Capability catalogue check confirming no premature status or release-scope
  change.
- `git diff --check`.
- Retained review notes bind the reviewed commit and give separate ADR-0013,
  ADR-0014, and ADR-0015 outcomes/blockers.
- Local implementation tests are contextual evidence only and cannot qualify
  real multi-host reachability or Application Messaging.

## Capability impact and no-Q rule

- ADR-0013 maps to `deployment.multi-host`, currently
  `I=partial, R=no, O=no, Q=no`.
- ADR-0015 maps to `application.messaging`, currently
  `I=no, R=no, O=no, Q=no`.
- ADR-0014 maps to `service.direct-interaction`, currently
  `I=partial, R=no, O=no, Q=no`, but the current capability outcome overstates
  the selected discovery-only boundary.
- ADR-0014 acceptance must trigger a separate governance decision to replace
  that outcome with a discovery-handoff claim or keep the capability
  unqualified while only `application.discovery` advances.
- Review success changes design decision state only.
- Neither acceptance permits `Q=yes`. Only later complete matching-commit
  contract, integration, real-host, security, deployment, recovery, and
  release evidence may promote a capability.
- These capabilities remain outside the independent fifteen-capability
  stabilization DR-06 scope.

## Expected files and modules

- Review targets:
  - `docs/adr/0013-bounded-multi-host-reachability.md`;
  - `docs/adr/0014-end-direct-service-interaction-at-discovery.md`;
  - `docs/adr/0015-authority-backed-bounded-application-conversations.md`.
- Evidence sources:
  - `docs/engineering/research/multi-host-reachability.md`;
  - `docs/engineering/research/direct-service-interaction.md`;
  - `docs/engineering/research/application-discovery.md`;
  - `docs/engineering/research/application-hosting.md`;
  - `docs/engineering/research/application-messaging.md`;
  - `docs/engineering/research/channel-grant-authority.md`;
  - `docs/engineering/research/wave3-synthesis.md`.
- Tracker comments may be appended to this issue.
- No deployment, network, Messaging, SDK, API, or persistence module changes
  belong to this review issue.

## Exit condition

The issue closes only when ADR-0011 and ADR-0012 have their required explicit
maintainer dispositions, all three dependent ADRs have separate explicit
maintainer outcomes, and any accepted ADR status/governance updates are made
in separate logical commits. A returned or rejected ADR keeps its
implementation stream blocked without blocking an independently ready stream.
No implementation or qualification is claimed by closing this issue.

## Comments

- 2026-07-27 partial dependency transition: ADR-0011 was explicitly accepted
  from source `34bccdeef830fde0cd17d99dec14c9bc4cd8929c`. ADR-0013 and ADR-0015
  review gates are therefore open for their required compatibility rechecks.
  ADR-0014 remains independently blocked by Proposed ADR-0012 and its
  Application Discovery compatibility gate. This transition authorizes review
  only, not MR, AM or DSI implementation or capability promotion. Acceptance
  governance commit: `2030d35f1df0a11f8d701ea12e19537a6b4d1c69`.
- 2026-07-28 ADR-0013 compatibility review:
  - outcome: `review-ready`; no actionable compatibility blockers found;
  - reviewed source: clean `main@da39106ce695977c03594a296674229852ea53da`;
    accepted Authority implementation
    `1136def860f30bc452e1b5352c537cbd44a163f6`; maintainer acceptance
    `3775f46c5f35c0077306a14e8688156b6ff47f75`;
  - source comparison covered ADR-0013 line by line against accepted ADR-0011,
    the DR-03/DR-04 packets, Wave 3 synthesis/register, the CGA-04 and CGA-06
    security contracts, and the accepted implementation in
    `internal/authority`, `internal/channeldelivery`,
    `internal/identity/capability`, `internal/localapi`, `internal/config` and
    `internal/provision`;
  - authority boundary: ADR-0013 consumes exactly one designated authority
    slot and one non-federated Realm Authority. Deployment owns manifest,
    rollout/fence journals and enforced isolation evidence; the Authority
    remains sole owner of membership, generation, revocation, activation,
    fencing acceptance and checkpoint truth. The coordinator has no Principal,
    and SSH transport grants no Ardents authority;
  - control and state ownership: protected Operator access reaches the
    authority and every member through host-key-pinned workstation-side
    forwarding. Operator signer and per-Node sessions remain workstation-owned;
    Node runtime/identity/Waku/Store/capability state remains host-local; the
    authority group, independent repository, backups and coordinator journal
    are not merged;
  - fencing boundary: the accepted implementation requires direct
    Actor-equals-Effective membership authority, exact Realm/channel/operation
    binding, fresh `DeploymentFenceEvidence/v1`, at most 30 seconds asserted
    skew, and the `target_ingress_blocked`, `discovery_withdrawn` and
    `peer_id_denied` controls. Removal completes only with the target fenced
    and every survivor either approved-host active or explicitly fenced.
    ADR-0013's terminal `fenced`, `recovery_required` and fresh rejoin rules
    preserve those fail-closed conditions;
  - topology and reachability: the decision is limited to exactly three
    operator-owned Linux amd64 hosts, with private-LAN cross-host probe truth
    distinct from AutoNAT-gated public-direct truth. Static recovery,
    Store/bootstrap diversity, one-address bounds and real-host qualification
    remain explicit; Docker multinode/local Windows evidence is not promoted;
  - checkpoint, backup and time: the implementation reads the complete
    immutable predecessor chain, performs exact compare-and-append, caps it at
    65,536 heads and admits production only with the preprovisioned WORM
    assertion. Configuration keeps that repository outside Node, authority
    store/key and signer paths. ADR-0013 separately places the authority group,
    independent repository and authority backup inputs, and preserves the
    30-second skew/60-second validity-margin fail-closed contract;
  - restart, restore and transition: topology journals precede mutation,
    block overlapping rollout and drive reverse compensation. Accepted CGA-06
    recovery-only startup verifies the exact ledger, signer and unique
    repository head without repair. Planned authority transition remains the
    accepted dual-signed, adjacent-epoch, exact-CAS operation with fresh
    rotation of every channel and predecessor retirement; topology coordination
    does not redefine it. Ordinary compatible rollout is authority-host-last,
    while authority schema/protocol migration is authority-first with stopped
    members. Local-v2 migration, complete stopped-backup downgrade, stale
    restore, repository rollback/fork and identity/generation mismatch all fail
    closed;
  - unsupported boundary: other node counts, mixed/extra public addresses,
    schedulers, Kubernetes/Swarm, automatic NAT traversal, Circuit Relay,
    QUIC, WebTransport/WebRTC, remote Operator/Application APIs and a
    long-running controller remain outside the decision;
  - governance guard: ADR-0013 remains `Proposed`; this review does not accept
    it, update W3-D004, admit MR-01/CGA-07, execute qualification, or change
    `realm.channel-grant-authority` or `deployment.multi-host` from `Q=no`.
    Explicit maintainer disposition is still required.
