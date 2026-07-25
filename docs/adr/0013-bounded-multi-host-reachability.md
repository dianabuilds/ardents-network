# ADR 0013: Bounded multi-host reachability

- Status: Proposed
- Date: 2026-07-25
- Decision owners: Network Foundation, Deployment, Operations, Release
- Research source: `docs/engineering/research/multi-host-reachability.md`

## Context

The frozen Wave 3 product baseline implements explicit reachability modes,
TCP/WSS transport profiles, static and signed-DNS bootstrap, AutoNAT-gated
public advertisement, bounded Store, and an adversarial multi-network QA
scenario. Its production deployment contract nevertheless supports Docker
Compose on one Linux amd64 host. The QA topology is explicitly not an Operator
deployment surface and cannot establish real LAN/WAN, NAT, firewall, DNS, PKI,
cross-host upgrade, or recovery support.

Manual per-host runbooks would leave partial rollout and topology truth in
Operator notes. A long-running controller would introduce a remote
administrative trust root, controller protocol, distributed state, and
scheduler-like failure modes. Neither is an acceptable first-release boundary.

## Decision

The first-release multi-host support matrix is exactly three `service_node`
Nodes on three operator-owned Linux amd64 hosts. At least two Nodes are
independently hosted bootstrap and persistent Store providers. Two ingress
variants are supported:

- `private_lan`: mutually routable operator-managed private TCP addresses, with
  exactly one private literal translated-host address per Node, verified by a
  bounded dial from a different manifest host, and no public reachability
  claim;
- `public_direct`: the same three-host shape, with at least two Nodes having
  one manually routed/firewall-forwarded public TCP or WSS address each.

Each Node has at least two static recovery peers on different hosts. Signed
Waku-compatible DNS ENR trees are optional additive bootstrap sources and are
never the sole restart path. DNS observations are replaced on refresh and are
not persisted.

`public_direct` never mutates a router. A Node without explicit inbound routing
is `outbound_only`. Each public Node advertises exactly one first-release
address, withheld until fresh libp2p AutoNAT observation reports `Public` and
withdrawn on `Private`, `Unknown`, or observation loss. The one-address bound
matches the aggregate reachability evidence available from the selected
go-libp2p path.

The per-Node advertised-address validation is extended before the first release
to accept one private literal translated-host multiaddr in `private_lan` and
one public IP/DNS multiaddr in `public_direct`. Cross-scope, loopback,
unspecified, ambiguous, and profile-incompatible addresses fail before
transport startup. A private cross-host probe establishes LAN-scope
reachability only; it cannot create a public claim.

Operators use one versioned `ardents.topology/v1` manifest through five
operations: `validate`, `status`, `rollout`, `fence`, and `recover`. A bounded
operator-side coordinator compiles host-local plans and invokes existing local
deployment and protected status commands through workstation-side
`ardentsctl --ssh` host-key-pinned stream-local forwarding to each remote Unix
socket. It never opens a remote shell or installs a helper. It is not a
long-running service and has no Ardents Principal. Operator/Application APIs
remain local protected sockets.

The Operator signer remains in the workstation's protected key store or
permission-protected signer file outside the manifest/journal. `ardentsctl`,
not the coordinator, uses it. Sessions are separately bound to Operator
Principal, Node identity, Operator interface and SSH host-key pin. They are
never copied to a host or reused across Nodes. One `Unauthenticated` permits one
signer-backed refresh for that Node; authorization, tunnel, pin and availability
failures do not. The adapter owns tunnel/signer/session failures, the remote
Node owns RPC and domain failures, and the coordinator owns ordering/journal
failures.

The coordinator owns only topology intent and a durable
`topology-rollout-transaction/v1` journal. Each Node retains sole ownership of
its runtime, identity, Waku key/Store, capability state, and backup consistency
group. Rollout changes one Node at a time, keeps two known-good
bootstrap/Store Nodes, uses ADR-0008 composite readiness, and compensates in
reverse order under ADR-0006 semantics. Release images and materials follow
ADR-0009. Data is restored only when the release migration contract requires a
complete stopped-Node restore.

The manifest designates exactly one DR-03 authority operations slot and its
failure domain. Authority ledger, signing identity, delivery and
acknowledgement state are a separate consistency group from all Node state. A
separately protected signed-checkpoint repository in another failure domain
immutably retains every accepted head for the realm lifetime. The manifest
fixes a first-release capacity of 65,536 heads; exhaustion blocks security
mutation and requires a new realm rather than pruning or overwriting
anti-rollback history. All hosts use an operator-configured UTC source.
Preflight blocks authority mutation,
fencing, rejoin and rollout above 30 seconds absolute inter-host skew and
reserves a 60-second validity safety margin. Clock failure never extends
authority.

`fence --node <slot> --reason <code>` is a typed, monotonic operation authorized
as `topology.node.fence` on the exact target Node. Actor is the
workstation-authenticated Operator Principal, Effective equals Actor, and
Delegation is not accepted. Before acting, the coordinator persists a
`topology-fence-transaction/v1` record. It first enforces target stop where
reachable, deployment ingress withdrawal, removal from static/DNS usable sets,
and survivor disconnect/deny for the expected Waku Peer ID. It durably records
those attributable controls as bounded `DeploymentFenceEvidence/v1` before
submitting that evidence to the DR-03 membership operation.

`fenced` is terminal only after DR-03 accepts the deployment fencing evidence
for the non-acknowledging target, produces a signed removal/activation
checkpoint, retains that checkpoint in both authority state and the independent
monotonic repository, and both surviving Nodes return DR-03 `active` receipts.
The target may be unreachable and does not acknowledge its own removal.

Authority/repository failure, excessive skew, or failure of either survivor to
acknowledge ends as `recovery_required`, never `fenced`. A partitioned survivor
cannot complete removal independently. Audit attributes Actor, target, request
ID, reason, generation, checkpoint digest and outcome. `recover --rejoin`
cannot erase removal or reuse old grants: DR-03 must issue a fresh accepted
membership/generation checkpoint, both survivors must acknowledge it, and the
Node must then pass identity, clock and composite-readiness checks.

For an ordinary compatible release the authority host is upgraded last. An
authority schema/protocol migration follows DR-03 authority-first, then
stopped-member, then fresh-generation activation order. Its complete authority
group and external monotonic head are backed up and verified before either
order. Restore rejects partial state, checkpoint rollback, non-monotonic or
ambiguous history, or generation mismatch.

WSS uses an operator-managed certificate whose valid DNS-ID or exact IP-ID
matches the advertised identity and whose chain is trusted. Rotation atomically
replaces the pair and uses a controlled restart; there is no generated or
self-signed product fallback.

This decision defines no Channel Grant authority. Acceptance is conditional on
a review against the accepted DR-03 decision for realm membership, discovery
and data authority availability, revocation, generation rotation,
backup/restore, and mixed-generation rollout.

## Consequences

- The first support claim is finite and independently testable on real hosts.
- Existing Waku/libp2p wire behavior and host-local state ownership remain
  unchanged.
- A new versioned topology artifact and operator-side journal require strict
  compatibility and backup rules.
- A separate authority consistency group, bounded independent checkpoint
  repository, clock contract and fence journal become required operational
  inputs; none is Node state or a second authority.
- Static bootstrap diversity survives DNS failure, while signed DNS enables
  bounded address replacement.
- Public reachability cannot be inferred from configuration, a listener, an
  outbound connection, or one Docker port mapping.
- Three hosts and at least two Store/bootstrap Nodes cost more than a minimal
  two-Node demo but permit one planned/unplanned Node loss without claiming
  Byzantine availability.
- Topologies with other node counts, multiple public addresses per Node,
  Kubernetes, Swarm, automatic NAT traversal, Circuit Relay, QUIC,
  WebTransport, or WebRTC remain unsupported.
- Capability `Q` remains `no` until matching-commit LAN, WAN, security,
  deployment, fencing/rejoin, authority restore, upgrade, recovery and release
  evidence is accepted.
