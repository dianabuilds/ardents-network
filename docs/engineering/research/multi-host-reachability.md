# DR-04: Multi-host reachability

## Metadata

- Status: accepted research recommendation; ADR-0013 remains Proposed
- Research class: R2 deep research
- Decision owner: Network Foundation, Deployment, Operations, Release
- Research owner: Wave 3 DR-04
- Date: 2026-07-25
- Frozen baseline commit: `8b9f8ad87fb78fccd7a73d445f2d72dbf2e51b4c`
- Preparation baseline observed: `cbec069c37df9cf57756970a2c3a0eef8c232778`
- Parent program: `.scratch/wave3-deep-research/PRD.md`
- Blocking research: none; compatibility with the accepted DR-03 research
  recommendation was reviewed before packet acceptance
- Downstream consumers: production support matrix, deployment implementation,
  Wave 3 synthesis, DR-06

## Answer first

For the first release, support one deliberately narrow multi-host shape: three
`service_node` Nodes on three operator-owned Linux amd64 hosts, joined by
operator-routed TCP, with at least two independently hosted bootstrap/Store
Nodes. The same shape has two ingress variants: `private_lan`, where all
advertised addresses are private routed addresses, and `public_direct`, where
at least two Nodes have one explicitly configured, firewall-forwarded public
TCP or WSS address each. A Node behind NAT without a manual forwarding rule is
`outbound_only`, not `public_direct`. Static peer multiaddrs are the recovery
floor; signed DNS ENR trees are an optional replaceable discovery source, not a
sole restart dependency.

Expose one versioned topology manifest and five deployment operations:
`validate`, `status`, `rollout`, `fence`, and `recover`. Keep topology validation,
per-host plan generation, SSH execution, the durable cross-host rollout
journal, reachability checks, and redacted aggregation behind one deployment
module. Do not expose remote Operator/Application APIs, raw Waku controls, or a
scheduler. Host-local Compose remains the runtime owner.

This design turns already implemented reachability mechanisms into a supportable
operator journey, but it is not qualified at the frozen commit. It costs three
hosts, manual routing/firewall/DNS/certificate ownership, and a new
cross-host deployment coordinator. Because it fixes a public topology,
configuration artifact, failure ownership, and upgrade contract, write
Proposed ADR-0013 before implementation. Do not promote any capability to
`Q=yes`.

## User outcome

An Operator can deploy, inspect, upgrade, partition, and recover a three-Node
Ardents realm across real hosts without copying hidden peer identity, exposing
control APIs, or claiming an inbound endpoint that has not been verified.

## Scope

### In scope

- three operator-owned Linux amd64 hosts;
- `service_node` with `tcp_only` or `tcp_wss`;
- `private_lan` and `public_direct` ingress variants;
- static bootstrap peers and optional signed DNS ENR trees;
- manual NAT/firewall rules and explicit advertised endpoints;
- WSS server certificates, rotation, and controlled restart;
- churn, partitions, Store loss/recovery, backup/restore;
- one-at-a-time mixed-generation upgrade and compensation;
- deployment ownership and bounded observability.

### Out of scope

- Kubernetes, Docker Swarm, a scheduler, or a service mesh;
- QUIC, WebTransport, WebRTC, Circuit Relay, hole punching, UPnP, and NAT-PMP;
- automatic public address or certificate acquisition;
- remote Operator/Application transports;
- arbitrary node counts or elastic membership;
- public anonymous realms or a new Channel Grant authority;
- a guarantee that Waku Store is an authoritative message database.

## Current product truth

### Supported interfaces

| Surface | Supported contract at the frozen baseline | Truth |
|---|---|---|
| Operator | protected local Operator socket; `ardentsctl node status`, Network status/peers/records | workstation-side `ardentsctl --ssh` opens SSH stream-local forwarding to the remote Unix socket; no remote shell or helper service |
| Application | protected local Application socket | deliberately unrelated to topology control |
| Internal | `internal/network/waku` over go-waku/libp2p | TCP/WSS, static bootstrap, signed DNS discovery, reachability observation, Store and bounded recovery are implemented |
| Deployment | `ardents.ps1`, `scripts/deploy/*`, single-host Compose and native installer candidate | supported production contract is one Linux amd64 Docker host; no supported real multi-host workflow |
| QA | `tests/run-multihost.ps1` and testnet Compose | adversarial multi-network evidence harness, explicitly not an operator quick-start |
| True external | host routing/firewall/NAT, DNS resolver and authoritative DNS, PKI, Docker Engine, SSH | operator/provider-owned and not made reliable by Ardents |

### Reachable journey

At the frozen baseline an Operator supplies a versioned per-Node configuration.
`cmd/ardentsd` loads it; `internal/daemon/configuration.go` validates the node,
transport and reachability combination and passes it through
`internal/daemon/assembly.go` to `internal/network/waku`. Startup validates the
WSS material and persistent Store/key continuity, creates the libp2p/Waku host,
refreshes signed DNS sources, dials static and discovered bootstrap peers, and
reports joined/readiness through the protected Operator surface.

For `private_lan`, a bound listener becomes LAN-reachable truth without a
public claim. For `public_direct`, configured advertised addresses are withheld
until the libp2p reachability event is `Public`; `Private`, `Unknown`, or a
closed observation stream withdraws them. The event is aggregate Node
reachability, not proof of every configured address. In `private_lan`, the
runtime currently publishes bound container/listener addresses and rejects
explicit advertised addresses. It therefore has no supported way to represent
a Docker host's translated LAN endpoint across independent Engines. This
journey becomes unsupported at the host boundary: production Compose, its
rollout journal, and its cluster manifest own Nodes on one Docker Engine only.
The multi-host runner proves useful behavior in Docker networks but is a QA
surface and carries no production deployment or real WAN qualification.

### Implementation and evidence

| Claim | Source or contract | Evidence | Baseline disposition |
|---|---|---|---|
| Waku is the canonical network substrate | `docs/protocols/canonical-network-foundation.md` | go-waku `v0.10.3` in `go.mod` | implemented |
| Reachability modes are explicit and separate from `joined` | `docs/protocols/network-participation-profiles.md`; `internal/network/reachability.go` | reachability unit/integration tests | implemented, Operator-reachable |
| Public advertisements are validation- and AutoNAT-gated | `internal/network/waku/reachability.go` | `transport_reachability_test.go`; `transport_reachability_integration_test.go` | implemented and locally operable; not WAN-qualified |
| Private-LAN translated host address can be explicitly advertised | `internal/network/waku/reachability.go` rejects configured addresses outside `public_direct` | no independent-host Compose evidence | not implemented |
| Signed DNS knowledge is bounded and replaceable | `internal/network/waku/discovery.go` | DNS discovery unit/integration tests | implemented; no retained production DNS evidence |
| Store has finite count/age/byte bounds | `docs/protocols/network-participation-profiles.md`; Waku Store adapter | Store retention and network integration tests | implemented and locally operable |
| WSS rejects incomplete/self-signed product configuration | `docs/protocols/network-participation-profiles.md`; transport startup | WSS certificate and transport integration tests | implemented; not public-PKI-qualified |
| Segments, WSS, bootstrap loss, partition, churn and Store recovery are exercised | `tests/run-multihost.ps1`; `deploy/docker/compose/docker-compose.testnet.yml` | retained report format `NFM-001`; no matching-commit production qualification snapshot | reachable only through QA |
| Production deployment is multi-host | `docs/operations/deployment-contract.md` | contract says one Docker Compose host | not implemented, not reachable, not operable, not qualified |

The canonical capability catalogue consequently records
`network.waku-foundation` as `Q=no` and `deployment.multi-host` as
implemented-partial/reachable-no/operable-no/qualified-no.

### External primary-source checks

All external sources below were accessed 2026-07-25. Component versions are
the frozen baseline selections where applicable.

| Fact used | Primary official source | Version/date |
|---|---|---|
| Waku bootstrapping can use static peers and signed `enrtree://<key>@<fqdn>` DNS discovery; DNS entries carry connection details and are operationally replaceable | https://docs.waku.org/run-node/configure-discovery and https://docs.waku.org/learn/concepts/dns-discovery | Waku documentation current on access; baseline go-waku `v0.10.3` |
| Waku Store is configured as message caching with an explicit retention policy, not an authoritative application database | https://docs.waku.org/run-node/configure-nwaku | Waku documentation current on access; baseline go-waku `v0.10.3` |
| AutoNAT asks other peers to dial presumed public addresses; unreachable advertisements waste network resources; AutoNAT v1 does not test each address independently | https://docs.libp2p.io/concepts/nat/autonat/ | libp2p documentation current on access; baseline go-libp2p `v0.48.0` |
| Inbound service behind NAT needs an explicit mapping; automatic router mutation varies by implementation | https://docs.libp2p.io/concepts/nat/ | libp2p documentation current on access; baseline go-libp2p `v0.48.0` |
| A libp2p Peer ID is bound to a transport public key and is not an Ardents Principal | https://docs.libp2p.io/concepts/fundamentals/peers/ | libp2p documentation current on access; baseline go-libp2p `v0.48.0` |
| Published Docker ports create host firewall/NAT rules and are externally reachable unless further restricted | https://docs.docker.com/engine/network/port-publishing/ | Docker Engine documentation current on access |
| Compose production guidance is a single-server shape; multi-host overlay networking belongs to Swarm | https://docs.docker.com/compose/how-tos/production/ and https://docs.docker.com/compose/how-tos/networking/ | Docker Compose documentation current on access |
| TLS reference identity must match a DNS-ID or exact IP-ID in subjectAltName and the full certification path must validate | https://www.rfc-editor.org/rfc/rfc9525.html | RFC 9525, November 2023 |

## Actors, assets, and trust boundaries

| Actor | Identity | Authority | Protected assets | Trust boundary |
|---|---|---|---|---|
| Realm Operator | Ardents Principal plus SSH/deployment identity | exact Node Access Grants; separate host administration | topology manifest, rollout journal, backups | Operator workstation to each SSH host and local Operator socket |
| Node | Node Principal | its own signed records and configured Channel Grants | `ardents.db`, Identity store, Node key | host-local process and protected volumes |
| Waku participant | Waku Peer ID | transport participation only | Waku key and Store | libp2p connection; never grants Ardents authority |
| Deployment coordinator | no Ardents Principal of its own | invokes workstation-side `ardentsctl --ssh` under an explicitly selected Operator Principal | manifest, known-host pins, redacted journal | operator workstation; never reads signer material or owns a reusable session |
| Realm authority operator | authority identity and DR-03-defined signing identity | issues/checkpoints membership, generation and removal truth under DR-03 | authority ledger, signing identity, delivery/acknowledgement state | designated authority slot; state is separate from every Node |
| DNS publisher/provider | DNS tree signing key / provider account | publishes bootstrap hints only | signed tree and DNS availability | true external; not a realm or Channel Grant issuer |
| PKI operator/CA | CA and server certificate identities | WSS server identity only | CA and leaf keys | true external; not a Node Principal or Access Grant |
| Remote peer, including malicious authorized peer | Waku Peer ID and possibly an Ardents Principal | only separately granted channel actions | connection, Store and protocol budgets | untrusted network input |

Credential, Access Grant, Delegation, Channel Grant, Waku Peer ID, WSS
certificate identity, SSH host identity, and DNS tree signing identity remain
distinct.

## Invariants

- Application interfaces never expose topology or Operator authority.
- No network connection, Peer ID, certificate, DNS record, or AutoNAT result
  creates an Ardents Principal or Channel Grant.
- Private-envelope authorization precedes durable replay admission.
- `public_direct` publishes exactly one first-release public address per Node
  only after fresh observed `Public`; any loss of confidence withdraws it.
- `private_lan` publishes exactly one configured private literal host address
  only after a different configured host completes a bounded dial probe; it
  never implies Internet reachability.
- A Node with no manual routable ingress is `outbound_only`; Ardents does not
  mutate routers.
- Every Node has at least two static recovery peers on different hosts. Signed
  DNS is additive and may fail or be replaced without deleting static peers.
- At least two Nodes retain and serve Store; Store remains bounded cache and
  never becomes authoritative application truth.
- One manifest slot is the designated DR-03 authority operations slot. Its
  authority ledger/signing identity/delivery state is a separate consistency
  group, never Node state. A latest signed-checkpoint repository lives in a
  different failure domain and immutably retains every accepted head for the
  realm lifetime. The manifest fixes `max_checkpoint_heads: 65536`; exhaustion
  blocks security mutation and requires a new realm, never pruning or
  overwriting the anti-rollback history.
- All three hosts use an operator-configured UTC source. Preflight rejects
  absolute inter-host skew above 30 seconds; authority validity calculations
  reserve a 60-second safety margin. Clock failure blocks issuance, fencing,
  rejoin and rollout, but never extends expired authority.
- Waku identity and Store form one stopped-Node backup/restore consistency
  group. Partial regeneration fails closed.
- At most one Node is unavailable for planned change. Two known-good
  bootstrap/Store paths remain before the next mutation.
- All peer, DNS tree, advertised-address, retry, timeout, Store, metric-label,
  rollout, and retained-journal sets have explicit finite bounds.
- Mixed generation is allowed only inside a declared compatibility window.
  Unknown/new incompatible config, wire, or persisted schema fails preflight.
- Logs, public metrics and rollout journals contain no Principal IDs, channel
  selectors, secrets, endpoint credentials, or private keys. The protected
  fence journal and audit contain only the Actor/target attribution required
  below, never a credential or channel secret.

## Dependency classification

| Dependency | Classification | Owner | Failure ownership | Substitutable locally? |
|---|---|---|---|---|
| topology compiler/reconciler | in-process | Deployment | Ardents | yes |
| Waku adapter and reachability state | in-process | Network Foundation | Ardents | yes |
| host-local Compose/runtime | local-substitutable | each host Operator | host deployment | yes, native remains a separate candidate |
| `ardentsctl --ssh` stream-local adapter and known-host database | local-substitutable | Operator tooling | local CLI owns tunnel/session errors; remote Node owns RPC/authz errors | yes, via host console and the same local Unix-socket command |
| peer Nodes and their Store | remote-owned | realm Operators | owning host/Node | no |
| LAN/WAN routing, NAT and firewall | true-external | network Operator/provider | Operator/provider | no |
| authoritative DNS and recursive resolver | true-external | DNS Operator/provider | Operator/provider | static peers provide bounded recovery |
| CA and certificate issuance | true-external | PKI Operator/provider | Operator/provider | private CA or public PKI |
| accepted DR-03 authority | remote-owned research dependency | Identity/Security decision owners | Wave 3 integrator | no |
| signed-checkpoint repository | remote-owned | realm authority operator | Operator; it cannot issue authority | no, but the latest accepted checkpoint also remains in the protected authority consistency group |

## Alternative designs

### Alternative A — independent host runbooks

- External interface: per-host config plus documented `install/up/status`.
- Internal seam: none beyond existing host-local deployment.
- State ownership: each host owns independent state; Operator keeps notes.
- Authority model: unchanged Ardents authority; SSH is manual.
- Failure and recovery: Operator correlates partial rollout and topology state.
- Compatibility and migration: release notes and human sequencing.
- Operational cost: lowest implementation cost, highest incident ambiguity.

### Alternative B — manifest-compiled host-local orchestration (selected)

- External interface: one `ardents.topology/v1` manifest and
  `validate/status/rollout/fence/recover`.
- Internal seam: topology compiler plus durable cross-host transaction
  reconciler; host adapters invoke existing local deployment contracts.
- State ownership: each Node remains host-local; coordinator owns only the
  topology intent and rollout journal.
- Authority model: no new Ardents authority; pinned SSH hosts and existing
  protected local Operator calls.
- Failure and recovery: durable mutation-before-action journal, bounded
  per-host deadlines, reverse compensation, explicit operator resume.
- Compatibility and migration: manifest schema is versioned; each release
  declares config/wire/persistence compatibility and downgrade rules.
- Operational cost: one new deployment module and qualification environment;
  no scheduler or long-running control plane.

### Alternative C — central long-running cluster controller

- External interface: controller API with desired-state reconciliation.
- Internal seam: scheduler/control plane with agents on every host.
- State ownership: new distributed cluster state and leader/lease semantics.
- Authority model: new remote administrative credential and controller trust
  root.
- Failure and recovery: automatic, but controller partition and split-brain
  become product failures.
- Compatibility and migration: controller/agent protocol and state migrations.
- Operational cost: largest; effectively creates an orchestrator.

### Decision matrix

Scores are 1 (poor) through 5 (best).

| Criterion | Weight | A | B | C | Evidence or reasoning |
|---|---:|---:|---:|---:|---|
| Module depth | 3 | 1 | 5 | 3 | B hides topology and transaction detail behind five operations |
| Caller leverage | 3 | 2 | 5 | 4 | B removes repeated cross-host sequencing and fencing without a service |
| Change locality | 2 | 2 | 4 | 2 | B reuses host adapters; C touches trust and runtime |
| Trust-model fit | 4 | 4 | 4 | 1 | C introduces remote administrative authority |
| Failure clarity | 4 | 1 | 5 | 3 | durable coordinator journal makes partial progress explicit |
| Migration cost | 2 | 5 | 4 | 1 | B is additive; C needs controller migration |
| Operability | 4 | 1 | 5 | 4 | B provides one bounded aggregate without continuous control |
| **Weighted total** |  | **47** | **106** | **51** | Select B |

## Selected design

### Minimum support matrix

| Property | `private_lan` | `public_direct` |
|---|---|---|
| Nodes/hosts | exactly 3 `service_node` Nodes on 3 Linux amd64 hosts | same |
| Routing | mutually routable operator-owned private TCP network | outbound Internet plus manual inbound TCP mapping on at least 2 Nodes |
| Ingress truth | exactly 1 routed private literal host multiaddr per Node, verified by a cross-host dial; never a public claim | exactly 1 public advertised TCP or WSS multiaddr per public Node; AutoNAT `Public` required |
| Bootstrap | at least 2 static peer multiaddrs on different hosts; optional signed DNS tree | same; signed DNS strongly recommended for planned address rotation |
| Store | at least 2 persistent Store providers with declared finite retention | same |
| Realm authority | exactly 1 designated authority Node/slot; Realm Authority Principal and consistency group remain distinct from Node identity/state | same |
| Anti-rollback checkpoint | immutable compare-and-append repository in a failure domain independent of the authority archive, capped at 65,536 heads | same |
| Clock | operator UTC source; at most 30 seconds inter-host skew and 60-second authority safety margin | same |
| WSS | optional; private CA or trusted public CA | optional; certificate SAN exactly matches advertised DNS/IP identity |
| Partition promise | a 2-Node connected component may remain joined; isolated Node degrades | same; public endpoint withdrawal does not erase retained Store |
| Upgrade | one Node at a time; keep 2 known-good bootstrap/Store Nodes | same |
| Qualification | real three-host routed-LAN E2E | real three-host WAN/NAT/firewall plus public-PKI or controlled private-PKI E2E |

Exactly three is a first-release support bound, not a protocol maximum.
Topologies with one or two Nodes, more than three Nodes, multiple addresses per
public Node, asymmetric unverified routes, or mixed ingress modes may work but
are unsupported until separately qualified.

### External interface sketch

```text
ardents topology validate --manifest topology.json
ardents topology status   --manifest topology.json
ardents topology rollout  --manifest topology.json --image <immutable-ref>
ardents topology fence    --manifest topology.json --node <slot> --reason <code>
ardents topology recover  --manifest topology.json
```

`ardents.topology/v1` contains exactly three stable Node names, pinned SSH host
aliases (not passwords/keys), reachability/transport profiles, one address per
public Node, static bootstrap references, optional signed DNS roots, Store
retention class, failure-domain labels from a closed three-value set, and
immutable image references. It references protected file paths but contains no
secret, Channel Grant, certificate key, or raw private selector. It is
operator-protected because it contains topology and the expected public Node
Principal/Waku Peer ID bindings needed for exact fencing.
For `private_lan` the compiler also renders one private literal translated-host
multiaddr per Node. The runtime reachability seam must accept that address
under `private_lan`, keep it distinct from a public claim, and publish it only
after a bounded probe from a different manifest host succeeds.

The manifest also identifies:

- exactly one `authority_slot` and its failure domain;
- a checkpoint-repository locator and independent failure domain;
- checkpoint retention (`immutable_history: true`,
  `max_checkpoint_heads: 65536`);
- `max_clock_skew: 30s` and `authority_safety_margin: 60s`;
- per-Node SSH host alias and pinned host-key reference;
- a workstation-local Operator signer alias, never a signer path or key bytes;
- the exact expected Node Principal and Waku Peer ID for equality checks and
  the fence resource binding (kept out of metrics and ordinary logs).

### Operator reachability and signer custody

The coordinator does not run `ssh host command`, install a remote helper, or
authenticate as an Ardents Principal. For every protected query or mutation it
starts the local `ardentsctl --ssh <pinned-host-alias> --addr
unix:///run/ardents/control.sock ...` adapter. That adapter owns one
stream-local SSH forward from the workstation to the remote protected Unix
socket and then uses the normal Operator protocol.

The Operator signer remains in the workstation's protected OS key store or a
`0600` local signer file selected by alias outside the manifest and journal.
`ardentsctl`, not the coordinator, reads or invokes it. Each Node gets its own
Node/interface-bound session. The local CLI cache keys sessions by
`(Operator Principal, Node identity, Operator interface, host-key pin)`, never
copies them to a host, never shares them across Nodes, and discards them at
expiry or host/Node identity mismatch. One `Unauthenticated` result permits one
signer-backed refresh for that Node; `Forbidden`, host-key mismatch, tunnel
failure, timeout and `Unavailable` are never converted into session refresh.

Failure ownership is exact: the local adapter owns DNS/SSH negotiation,
host-key, stream-forward and local signer failures; the remote `ardentsd` owns
socket, authentication, authorization and domain outcomes; the topology
reconciler owns ordering, deadline, journal and aggregate projection. Errors
retain these stable classes without exposing paths, signer details, session
secrets or remote socket metadata.

### Internal seam and state machine

The `TopologyReconciler` compiles the manifest into host-local plans, validates
all cross-Node invariants, queries protected composite readiness through
workstation-side stream-local adapters, and owns
`topology-rollout-transaction/v1`.

```text
validated -> preflighted -> node_pending -> node_applied
                      |           |              |
                      +-------- failure ----------+
                                      |
                                compensating
                                  /       \
                             restored  recovery_required
```

The journal is flushed before each host mutation and records manifest digest,
target/fallback immutable images, compatibility declaration, ordered Nodes,
per-Node phase, last redacted reason, and recovery requirement. A new rollout
cannot start while recovery is pending. It does not copy Node databases or
secrets.

### Fencing state machine and durable evidence

`fence` is a fifth typed topology operation, not a `recover` side effect:

```text
requested -> isolation_pending -> evidence_persisted -> authority_pending
               |                      |                    |
               +-------- failure -----+--------------------+
                                      |
                            recovery_required

authority_pending -> checkpoint_persisted -> peers_acknowledged -> fenced
```

Before any action, the coordinator durably writes a
`topology-fence-transaction/v1` record containing manifest digest, target slot,
expected Node Principal/Waku Peer ID hashes, reason code, Actor Principal,
request ID, start/deadline, DR-03 authority generation/checkpoint digest,
per-survivor acknowledgement, isolation/evidence phases and stable failure
class. It contains no grant, channel secret, signer material or session.

Isolation first stops the target when reachable, blocks deployment-managed
target ingress, removes it from static/DNS usable sets, and makes both
survivors close and deny its expected Waku Peer ID. These actions produce a
bounded `DeploymentFenceEvidence/v1`: realm and target bindings, manifest
digest, request ID, reason, UTC observation, clock/skew result, each enforced
control and attributable protected Operator receipt. It contains no channel
material. The evidence is stored with the fence journal before submission to
the authority.

The exact new Operator action is `topology.node.fence` on
`node:<target Principal>`. The Actor is the workstation-authenticated Operator
Principal; Effective equals Actor and Delegation is not accepted for this
deployment operation. The coordinator has no Principal. The designated
authority workflow uses the accepted DR-03 interface to remove/revoke the
target, accepts the explicit `DeploymentFenceEvidence/v1` for a
non-acknowledging target, and produces its signed removal/activation checkpoint;
DR-03 owns the membership operation, checkpoint format, generation,
survivor-receipt and authorization semantics. DR-04 supplies only the
deployment fencing evidence that DR-03 requires.

`fenced` is truthful only when:

1. the isolation controls and durable fencing evidence remain in force;
2. the signed removal checkpoint is durably present both in the protected
   authority consistency group and the independent checkpoint repository;
3. both surviving Nodes return DR-03 `active` receipts for applying the
   checkpoint;
4. the durable audit record attributes Actor, target, request ID, generation,
   checkpoint digest and terminal outcome.

The target may be unreachable or partitioned; its acknowledgement is neither
required nor trusted. If the authority slot/checkpoint repository is
unavailable, clock skew exceeds the bound, or fewer than both survivors
acknowledge before the deadline, the terminal outcome is
`recovery_required`, never `fenced`. A surviving partition cannot
independently complete removal.

Fencing is monotonic. `recover --rejoin <slot>` cannot erase the old removal or
reuse old Channel Grants. It first requires DR-03 to issue a fresh accepted
membership/generation and signed checkpoint, persists it in both repositories,
requires both survivors to acknowledge it, then restores static/DNS/ingress
configuration and starts the Node. Only after composite readiness, identity
match and bounded clock checks does the journal become `rejoined`. Failure
leaves the Node fenced and the journal `recovery_required`.

### Availability and recovery rules

1. Validate routes, port ownership, file existence/permissions, unique
   addresses, static peer diversity, Store count, certificate identity and
   immutable images before mutation. Also validate authority/checkpoint failure
   domains, checkpoint retention and clocks.
2. Prove all three Nodes ready and record their Principal/Waku identity hashes
   only as local equality checks; do not retain the identifiers in metrics.
3. For an ordinary release within the declared authority compatibility window,
   upgrade a non-authority Node first, then the other non-authority Node, and
   the Node sharing the designated authority host last. For an authority
   schema/protocol migration, follow DR-03 instead: migrate the stopped
   authority first, then stopped members, then complete a fresh generation
   activation. Never mutate two hosts simultaneously. Take and verify the
   separate authority consistency-group backup and external monotonic head
   before either order begins.
4. After each restart, require ADR-0008 composite readiness, expected identity,
   joined truth, reachability truth appropriate to the mode, and Store health.
5. On failure, stop advancing and compensate changed Nodes in reverse order
   under ADR-0006 semantics. Restore data only when the release migration
   contract requires the stopped-Node backup.
6. After partition recovery, refresh signed DNS, redial static peers, reconcile
   records and fetch eligible retained private envelopes. Store gaps beyond
   retention are terminally unavailable; no false completeness claim is made.

### Authority and audit semantics

Topology operations are deployment operations, not Application operations.
SSH authenticates workstation-to-host transport only. Each `ardentsctl --ssh`
RPC separately authenticates its workstation-held Operator signer as Actor
Principal and authorizes exact Node actions locally; no Effective Principal or
Delegation is inferred from SSH. Every host result is attributable and the
coordinator records stable outcomes, not credentials or sessions.

Channel Grant issuance, realm membership, generation, revocation, recovery and
audit remain DR-03-owned. The topology design requires only this compatibility
contract: every Node must possess accepted, non-expired discovery/data channel
authority before readiness; mixed-generation rollout must not cause an older
Node to accept a grant generation or realm membership that DR-03 forbids.
This requirement is not a proposed DR-03 decision.

## Delivery and data semantics

| Concern | DR-04 rule |
|---|---|
| Ordering | topology mutations are serial and journal-ordered; Waku message ordering is not strengthened |
| Acknowledgement | host mutation is applied only after composite readiness; fence/rejoin requires both surviving Nodes to acknowledge the DR-03 checkpoint; transport acceptance is not message delivery acknowledgement |
| Deduplication | deployment command uses journal/manifest digest; private replay uses its existing durable ledger |
| Expiry | DNS knowledge refreshes/replaces; messages and Store results obey configured expiry |
| Limits | 3 Nodes, 2 static recovery peers minimum, 4 DNS roots maximum, 128 DNS results maximum, 1 public address per Node in supported topology |
| Backpressure | existing connection/operation/Store bounds remain authoritative; coordinator has one in-flight Node mutation |
| Large payloads | not applicable; Content References remain immutable and DR-04 does not carry payloads |
| Terminal outcomes | ready, degraded, compensated, or `recovery_required`; Store history outside retention is unavailable |

## Failure, restart, recovery, and migration

| Event | Caller outcome | Persisted truth | Retry rule | Operator action |
|---|---|---|---|---|
| DNS unavailable | degraded source; existing/static peers remain | no DNS peers persisted | bounded 10 s failure retry, normal 5 min refresh | repair DNS or use static peers |
| bootstrap Node lost | remaining mesh may stay joined | Node/Store state unchanged | bounded replenishment | restore host; do not rotate all bootstrap entries |
| NAT/firewall blocks ingress | public address withheld/withdrawn | config unchanged | no automatic router mutation | correct mapping/firewall, then obtain fresh observation |
| AutoNAT `Unknown`/stream closes | public claim withdrawn | last observation is diagnostic only | bounded observer recovery; no spin | inspect peer diversity and ingress |
| WSS certificate invalid/expired | Node fails controlled start | old files remain deployment truth | no insecure fallback | atomically replace matching pair/CA and restart |
| peer churn | joined may degrade/recover | static config and Store remain | bounded redial/replenishment | act only after stable degraded deadline |
| network partition | each component reports its own joined/role truth | no invented merge state | reconnect then normal reconciliation | restore routing; inspect retained gaps |
| one Store unavailable | live Relay may continue; offline fetch redundancy reduced | surviving Stores are bounded caches | try another eligible Store once per bounded plan | restore full Waku consistency group |
| corrupt/missing Waku key or Store | Node start fails continuity | corrupt group preserved | never regenerate/delete implicitly | restore complete stopped-Node group |
| rollout interruption | new operation refused | journal is authoritative | resume compensation first | run `recover`, then retry rollout |
| target unreachable during fence | may still become `fenced` after enforced isolation evidence, authority checkpoint and both survivor active receipts | fence journal, deployment evidence and signed checkpoint | target is not retried as authority | repair or securely destroy target; old membership cannot rejoin |
| authority/checkpoint/clock/survivor failure during fence | `recovery_required`, never removal success | partial fence journal retained | resume from recorded phase after dependency recovery | restore authority group/repository/clock/quorum |
| rejoin requested | old removal remains valid until fresh DR-03 membership checkpoint is acknowledged | both checkpoints and journal retained | no reuse of prior grants | complete DR-03 re-admission, then `recover --rejoin` |
| incompatible persisted schema | preflight/start fails | old group preserved | no in-place downgrade | restore exact backup and fallback image |
| DR-03 revocation/rotation | topology cannot override denial | DR-03 authority store is authoritative | per accepted DR-03 only | follow DR-03 recovery workflow |

## Security, privacy, and abuse analysis

- Public transport is unauthenticated ingress to libp2p, not public Ardents
  authority. Existing connection-per-IP, total connection, concurrent
  operation, rate/burst, Filter and Store bounds apply before expensive work.
- AutoNAT service retains its configured rate limit. Aggregate AutoNAT v1 truth
  is why the first release supports only one public advertised address per
  Node.
- The manifest rejects credentials, inline keys, `/p2p-circuit`, loopback,
  unspecified and profile-incompatible public addresses.
- DNS tree signature is a bootstrap-source trust root only. Returned peers
  remain untrusted transport participants until separate Ardents checks pass.
- WSS validates the full chain, server-auth usage, validity, and exact DNS-ID or
  IP-ID. Self-signed product fallback is forbidden. Rotation is atomic file
  replacement plus controlled restart.
- SSH host keys are pinned. The coordinator never disables host verification
  and never forwards Operator/Application sockets beyond the explicit command.
- Operator signer material and Node-bound sessions never cross the workstation
  boundary. A compromised coordinator can request only commands that the local
  `ardentsctl` invocation and exact grants authorize; it cannot extract the
  signer or reuse a session against another Node.
- A malicious authorized peer can churn connections, retain ciphertext or
  withhold Store results. It cannot create authority or force replay admission;
  quotas bound its resource leverage. Availability requires two independent
  Store/bootstrap hosts but does not promise Byzantine delivery.
- Endpoint, peer, Principal and realm cardinality are not metric labels.
  Diagnostic lists are access-controlled, paginated/bounded, and redacted.

## Observability

`topology status` returns one bounded row per three configured Nodes:
host-reachable, daemon-ready, joined, reachability mode/state, active transport,
bootstrap source count/state, live Relay peer count bucket, Store
healthy/degraded/failed, certificate days-to-expiry bucket, image digest
match, rollout phase, and one stable reason code.

Metrics use only node slot (`seed-a`, `seed-b`, `member-c`), mode, transport,
phase and stable outcome. They exclude hostnames, IPs, multiaddrs, Peer IDs,
Principals, channel references and DNS roots. Alerts cover loss of quorum-like
two-Node reachability, no healthy Store, public-address withdrawal, certificate
expiry windows, repeated churn/restricted-defense, and a stuck rollout journal.
ADR-0008 composite readiness remains the rollout gate; network-only health is
insufficient.

Fencing emits bounded counters by phase/outcome and one protected audit event
per request and terminal transition. Audit contains Actor Principal, target
Node, request ID, reason code, checkpoint generation/digest and result; public
metrics contain none of those identifiers. Authority checkpoint age, repository
availability, acknowledgement count and clock-skew bucket participate in
topology readiness.

## Compatibility consequences

- **Wire:** no new Waku wire protocol. The release manifest must declare the
  compatible go-waku/libp2p and Ardents private-envelope generation window.
- **Persistence:** adds only the operator-side topology rollout journal.
  Node and Waku stores remain host-local consistency groups. The DR-03
  authority ledger/signing/delivery state is a distinct consistency group; the
  checkpoint repository is separately protected retained evidence, not a
  second writable authority.
- **Configuration:** adds `ardents.topology/v1`; per-Node
  `ardents.config/v1` remains authoritative at runtime. Before the first
  release, its advertised-address validation is narrowed to one address and
  extended so `private_lan` accepts only a private literal translated-host
  address while `public_direct` accepts only a public IP/DNS address. Unknown
  or cross-scope addresses fail before transport startup.
- **Backup/restore:** every Node is stopped and restored independently with its
  complete consistency group; coordinator state is backed up separately and
  contains no secrets. Authority restore uses its complete DR-03 group, checks
  it against the latest retained signed checkpoint, and fails closed on
  rollback, generation mismatch or missing evidence.
- **Rollout:** extends ADR-0006 transaction semantics across hosts and uses
  ADR-0008 readiness. Images remain immutable under ADR-0009.
- **Downgrade:** an older topology tool rejects unknown manifest/journal
  versions. An older Node binary starts only after its release-declared schema
  compatibility or a complete backup restore.
- **Mixed generation:** bounded to one Node during rollout; two known-good
  Nodes remain. Private discovery/data behavior is acceptance-blocked until
  DR-03 confirms authority-generation compatibility.
- **DNS/address change:** signed-tree change may converge live; bind address,
  advertised address, WSS certificate, and reachability mode changes require a
  controlled restart and fresh reachability proof.

## Acceptance matrix

| Level | Required evidence | Environment | Commit-bound artifact |
|---|---|---|---|
| Unit | manifest bounds, authority/checkpoint placement, skew/retention, topology invariants, address/profile rules, redaction, deterministic rollout/fence/rejoin state transitions | Linux/Windows unit runners | JUnit/JSON tied to exact commit |
| Contract | `ardents.topology/v1`, rollout/fence journals and `DeploymentFenceEvidence/v1` golden/negative corpus; exact `topology.node.fence`; unknown fields/version fail; checkpoint repository compare-and-append history is immutable and capped; crash points resume deterministically | clean container | retained schema corpus and report |
| Integration | real workstation-side `ardentsctl --ssh` adapter proves pinned-host forwarding, per-Node session separation/one refresh, signer custody, failure ownership, mutation-before-action, reverse compensation and no secret capture | Linux containers with SSH endpoints | JSON transaction trace |
| E2E private LAN | 3 real Linux hosts, routed private TCP, bootstrap loss, segment partition/rejoin, churn, two-Store recovery, identity-preserving restart | dedicated three-host lab | sanitized topology snapshots/logs |
| E2E public direct | 3 real Linux hosts, at least 2 independent NAT/firewalls, one public TCP/WSS address each, real external dialback, DNS replacement and certificate rotation | WAN lab outside one L2/one Docker Engine | dialback, DNS, certificate and readiness evidence |
| Security | port scan proves only Waku ingress; SSH host mismatch fails; remote shell/helper unavailable; signer/session never leaves workstation; cross-Node session reuse fails; self-signed/SAN/key-permission negatives; connection/rate/Store exhaustion; no secret/identifier leakage | isolated hostile-client lab | SARIF/JUnit/redaction report |
| Deployment | fresh install, Node and separate authority backup/restore, checkpoint rollback/mismatch, authority-host-last upgrade, host loss, clock skew, coordinator interruption at every rollout/fence boundary, unreachable/partitioned target fence, failed survivor acknowledgement, rejoin with fresh authority, compensation | 3 Linux amd64 hosts plus independent checkpoint repository | manifests, journal traces, signed checkpoint/backup verification |
| Release | clean exact commit; immutable materials; declared version matrix; DR-03 compatibility review; all preceding evidence without retry masking | independent release runners/lab | DR-06 qualification snapshot; capability remains `Q=no` until accepted |

## Open questions

There is no unresolved DR-04 question that changes the selected external
interface, topology manifest, state owner, or migration contract.

Acceptance remains explicitly dependent on one cross-stage check: the accepted
DR-03 result must be reviewed against discovery/data authority availability,
revocation, backup/restore and mixed-generation behavior. If incompatible,
DR-04 returns to research rather than inventing or overriding Channel Grant
authority.

The present design has been checked against the current DR-03 packet and
Proposed ADR-0011: its designated authority Node, exact Operator-only realm
procedures, survivor `active` receipts, explicit deployment fencing evidence,
roll-forward generation activation, distinct authority consistency group,
external immutable compare-and-append checkpoint repository, same-realm
restore freshness rule, and authority-first migration order are all preserved.
Acceptance still records the dependency until ADR-0011/DR-03 is accepted; this
is a decision-state dependency, not an unresolved topology, state, wire or
migration question.

## Decision-register proposals

- **W3-DR04-TOPOLOGY:** first release supports exactly three service Nodes on
  three operator-owned Linux amd64 hosts, with `private_lan` and
  `public_direct` variants and at least two bootstrap/Store hosts.
- **W3-DR04-INGRESS:** `public_direct` requires manual routing/firewall and one
  explicit address per Node gated by fresh aggregate AutoNAT `Public`;
  no automatic NAT traversal.
- **W3-DR04-BOOTSTRAP:** two cross-host static recovery peers are mandatory;
  signed DNS ENR is optional additive authority and DNS observations are not
  persisted.
- **W3-DR04-DEPLOY:** select the versioned manifest plus bounded host-local
  coordinator; reject manual-only support and a long-running cluster
  controller.
- **W3-DR04-FENCE:** add typed `fence` and monotonic
  `recover --rejoin`; removal is complete only after durable DR-03 checkpoint,
  both survivor acknowledgements and ingress/peer withdrawal.
- **W3-DR04-OPERATOR-PATH:** coordinator uses only workstation-side
  `ardentsctl --ssh` stream-local forwarding with workstation-held signer and
  per-Node sessions; it has no Principal or remote shell.
- **W3-DR04-AUTHORITY-PLACEMENT:** manifest designates one authority slot,
  separate authority consistency group, independent bounded signed-checkpoint
  repository and 30-second clock-skew contract; compatible rollouts upgrade the
  authority host last, while authority migrations follow DR-03 authority-first
  order.
- **W3-DR04-ADR:** accept Proposed ADR-0013 before implementation.
- **W3-DR04-DR03-CHECK:** acceptance is blocked on compatibility with the
  accepted DR-03 authority result; DR-04 asserts no authority design.

## Recommendation

Write ADR before implementation.

## Vertically sliced implementation issues

### MR-01 — Compile and validate a three-host topology

- **User story:** an Operator validates one bounded manifest before touching a
  host.
- **End-to-end behavior:** parse `ardents.topology/v1`, reject unsafe or
  unsupported shapes, emit deterministic redacted per-host plans.
- **Acceptance:** all support-matrix bounds, address/profile/certificate
  references, static-peer diversity, Store count, immutable images, unknown
  fields and secret rejection have unit/contract evidence.
- **Blocked by:** accepted ADR-0013.
- **Research class after packet:** R0.

### MR-02 — Inspect three Nodes through pinned host-local control

- **User story:** an Operator sees one truthful bounded topology status without
  exposing remote control APIs.
- **End-to-end behavior:** workstation-side `ardentsctl --ssh` stream-local
  forwarding invokes the protected Unix socket with a workstation-held signer
  and per-Node session, aggregates readiness/reachability/Store/image truth,
  and redacts identities.
- **Acceptance:** no remote shell/helper; signer never leaves workstation;
  Node/interface/host-pin session binding and one-refresh rule; host mismatch,
  tunnel timeout, remote denial, unavailable Node and partial result are
  distinct.
- **Blocked by:** MR-01.
- **Research class after packet:** R0.

### MR-03 — Place and recover authority checkpoint truth

- **User story:** a realm authority operator can restore the latest accepted
  DR-03 truth without confusing it with Node state.
- **End-to-end behavior:** designate authority slot/failure domain, manage its
  separate consistency group, immutably retain every accepted signed head in
  an independent repository capped at 65,536 heads, enforce clock/skew, and
  select ordinary authority-last versus DR-03 migration authority-first order.
- **Acceptance:** complete restore matches latest checkpoint; partial,
  rollback, generation mismatch, repository loss and excessive skew fail
  closed; retention is bounded.
- **Blocked by:** MR-01, MR-02 and accepted DR-03 compatibility contract.
- **Research class after packet:** R0 plus R3 recovery evidence.

### MR-04 — Fence and rejoin one Node truthfully

- **User story:** an Operator can complete removal even when the target is
  unreachable, and cannot silently resurrect its old authority.
- **End-to-end behavior:** exact `topology.node.fence`, durable fence journal,
  enforced isolation plus `DeploymentFenceEvidence/v1`, DR-03 signed removal
  checkpoint, both survivor active receipts, audit, and fresh-authority
  `recover --rejoin`.
- **Acceptance:** every crash point resumes; unreachable target can be fenced;
  authority/repository/skew/survivor partition yields `recovery_required`;
  old grants cannot rejoin; audit and redaction hold.
- **Blocked by:** MR-03.
- **Research class after packet:** R1 for failure injection, then R3.

### MR-05 — Form and recover the private-LAN topology

- **User story:** an Operator forms three routed private Nodes and recovers from
  bootstrap loss and partition.
- **End-to-end behavior:** host-local install/up uses deterministic static
  peers, explicit translated-host LAN advertisements, optional signed DNS and
  two Stores; a probe from another manifest host gates LAN publication and
  status proves join/recovery.
- **Acceptance:** three-host LAN E2E covers restart, partition/rejoin, churn,
  DNS outage/replacement and retained Store fetch/gap semantics.
- **Blocked by:** MR-03 and MR-04.
- **Research class after packet:** R0 implementation plus R3 evidence.

### MR-06 — Admit one verified public-direct endpoint per Node

- **User story:** an Operator publishes only externally verified TCP/WSS
  endpoints.
- **End-to-end behavior:** preflight firewall/NAT/certificate inputs; withhold
  until `Public`; withdraw on `Private`/`Unknown`; rotate certificate/address
  by controlled restart.
- **Acceptance:** real external dialback, NAT/firewall denial, observation loss,
  public PKI/private CA, SAN mismatch, expiry and rotation E2E.
- **Blocked by:** MR-05.
- **Research class after packet:** R0 implementation plus R3 evidence.

### MR-07 — Journal one-at-a-time cross-host rollout and recovery

- **User story:** an Operator upgrades without leaving an unrecorded mixed
  cluster.
- **End-to-end behavior:** durable preflight/journal, serial mutation,
  composite readiness, reverse compensation, interrupted recovery and explicit
  data-restore rule.
- **Acceptance:** fault injection at every boundary; two known-good
  bootstrap/Store Nodes remain; immutable image and identity checks; no new
  rollout over pending recovery.
- **Blocked by:** MR-02, MR-03, MR-04, ADR-0006, ADR-0008, ADR-0009, and the
  release's declared compatibility matrix.
- **Research class after packet:** R1 for cross-host failure injection, then R3.

### MR-08 — Qualify the minimum support matrix

- **User story:** a release reviewer can independently decide whether both
  supported topology variants are production-qualified.
- **End-to-end behavior:** run the complete acceptance matrix on the exact
  candidate, retain sanitized evidence, and update capability truth only after
  review.
- **Acceptance:** independent three-host LAN and WAN evidence, security,
  backup/restore, mixed-generation, release materials and DR-03 compatibility
  all match one commit with no hidden retry.
- **Blocked by:** MR-03 through MR-07 and accepted DR-03 compatibility.
- **Research class after packet:** R3.
