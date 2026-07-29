# Network Participation Profiles

## 1. Purpose

This document defines the supported Ardents `v1` node roles separately from
transport-family selection and from dynamic runtime defense state. It is the
source of truth for configuration validation.

The three independent dimensions are:

- node profile — the product role the process is allowed to perform;
- transport profile — the libp2p carrier families actually configured;
- runtime mode — an observed state selected by the running node.

Conflating these dimensions is prohibited. In particular, mounting a go-waku
protocol object is not evidence that Ardents has a complete product path for
that protocol.

## 2. Current Support Matrix

| Node profile | Startup selection | Transport profiles | Current product status |
|---|---|---|---|
| `service_node` | selectable; explicit default at the `ardentsd` boundary | `tcp_only`, `tcp_wss` | implemented full-node shape with explicit TCP-only and operator-provisioned WSS paths |
| `local_development` | selectable; safe default for embedded/generic runtime construction | `tcp_only` | implemented with loopback binding; no public reachability claim |
| `constrained_light_client` | selectable | `tcp_only` | implemented outbound-only Filter/Lightpush client with Store recovery; no local Relay, Store server, or Filter server |
| `restricted_defense` | never selectable | inherits the running transport | automatic runtime mode; full nodes rebuild as Relay-only and recover by rebuilding the steady provider shape |

Unknown profiles, `tcp_quic`, a manually selected
restricted-defense profile, and every node/transport pair outside this table
must fail configuration before lifecycle, identity, persistence, or transport
startup begins. There is no compatibility fallback to `service_node` or
`tcp_only` after an explicit invalid value.

## 3. Waku Role Mapping

The role states below distinguish actual product use from dependency mounting.

| Waku role | `service_node` | `local_development` | `constrained_light_client` target | Evidence/claim rule |
|---|---|---|---|---|
| Relay | publish and subscribe through private envelopes | same, locally | not maintained | claim only after a live Relay path and carrier exchange check |
| Store | bounded retained private-envelope fetch/service | same, locally | client recovery only | claim only after real Store persistence/fetch evidence; current legacy wire exception remains explicit |
| Filter | serves constrained clients; does not mount the client path | same | private subscription over an observed Filter provider | claim only after peer protocol identification and real Filter delivery evidence |
| Lightpush | serves constrained publication into Relay | same | private publication to an observed Lightpush provider | provider acknowledgement is acceptance, never a network-wide delivery claim |

`service_node` therefore does not imply that every mounted go-waku service is
an active Ardents capability. Capability reporting must be derived from
runtime checks and product-path tests, never from this static profile table.

Persistent Store participation by `service_node` and `local_development`
requires finite positive `network.limits.store_max_messages`,
`network.limits.store_max_age_seconds`, and
`network.limits.store_max_bytes`. Defaults are 100,000 messages, seven days,
and 2 GiB. The count/age limits are passed to go-waku and are also enforced
after every SQLite insert using trusted receiver time; SQLite page/WAL budgets
enforce the byte ceiling. Telemetry includes the main database, WAL, and shared
memory. At 90% of either message or byte capacity the Store pressure state is
`degraded`; an unreadable or missing persistent database is `failed`.
`constrained_light_client` and automatic `restricted_defense` never create a
persistent Store, so those states do not apply to them.

## 4. Transport And Exposure Rules

### `tcp_only`

- installs only libp2p TCP transport;
- suppresses QUIC, WebTransport, WebRTC, and WSS;
- may be used by `service_node` or `local_development`;
- does not by itself prove remote inbound reachability.

### `tcp_wss`

- installs TCP plus secure WebSocket listener support;
- is valid only for `service_node`;
- requires an explicit non-zero WSS port, certificate path, private-key path,
  and advertised DNS name or IP address before startup;
- validates that the files are readable regular files, the private key remains
  protected, the pair matches, the leaf is currently valid, permits server
  authentication, covers the advertised address, and chains to the host trust
  store or an explicitly configured private CA bundle;
- must never generate or silently accept a self-signed product certificate.

### Unsupported families

`tcp_quic`, WebTransport, and WebRTC remain unsupported.
Explicit selection fails; no family is silently substituted.

## 5. Bootstrap, Reconnect, And Reachability Expectations

| Profile | Bootstrap | Reconnect/peer replenishment | Address advertisement | Degradation truth |
|---|---|---|---|---|
| `service_node` | zero sources means isolated, `joined=false`; explicit static peers and signed DNS ENR trees are accepted; a live Relay peer is required before joined truth | bounded retry and five-minute DNS refresh replenish toward three Relay peers; failed DNS is retried after ten seconds | bound addresses may be published only as local observations until external reachability is proven | source discovery failure, peer dial failure, and relay readiness failure are distinct degraded reasons |
| `local_development` | optional local/static source; DNS discovery is rejected | bounded local retry only | loopback/local addresses only; never a public reachability promise | missing external peers is not a failure, but joined remains false |
| `constrained_light_client` | static/signed-DNS sources must expose Filter, Lightpush, and Store protocols | bounded reconnect and protocol re-evaluation | `outbound_only`; no published inbound endpoint | missing provider protocols, peer loss, Filter rejection, and Lightpush rejection remain explicit |
| `restricted_defense` | inherits current sources | recovery controller observes health/cooldown and rejoins after node reconstruction | publishes only endpoints of the rebuilt Relay-only node | Store, Filter-server, and Lightpush-server are absent; transition or recovery restart failure is explicit |

Reachability is independent from `joined` and is reported with an explicit mode,
state, reason, and observation time. Supported modes are:

- `local_only`: loopback-only `local_development` listener;
- `private_lan`: LAN-scoped `service_node` listener with exactly one private
  literal translated-host TCP address; the address is withheld until a fresh
  bounded probe from a different topology host and never creates a public
  claim;
- `outbound_only`: joins and uses Waku but publishes no inbound node endpoint;
- `public_direct`: deployment-managed public ingress whose explicit addresses
  remain unpublished until libp2p AutoNAT peer dialback reports `Public`.

`Unknown` and `Private` AutoNAT results withhold or withdraw direct-public
addresses. A configured address change requires controlled restart and fresh
observation. Automatic UPnP/NAT-PMP, Circuit Relay reservations, hole punching,
and browser inbound participation are not currently supported or advertised.

## 6. Configuration Contract

The local runtime validates the normalized node profile and transport profile
before invoking the lifecycle command. The `ardentsd` loader performs the same
validation before constructing or starting a node.

Stable startup errors explain only the invalid profile or combination. Invalid
configuration must leave lifecycle and transport state unchanged and must not
create a partial Waku runtime.

Operator input:

- `ARDENTS_NODE_PROFILE`: `service_node` or `local_development` today;
- `ARDENTS_TRANSPORT_PROFILE`: `tcp_only` or, after complete WSS material is
  supplied through the supported boundary, `tcp_wss`;
- `ARDENTS_WSS_PORT`: externally reachable WSS listener port in `1..65535`;
- `ARDENTS_WSS_CERT_PATH`: operator-managed PEM server certificate path;
- `ARDENTS_WSS_KEY_PATH`: operator-managed protected PEM private-key path;
- `ARDENTS_WSS_CA_PATH`: optional trusted CA bundle for private PKI; when it is
  omitted, the host system trust store is used;
- `ARDENTS_WSS_ADVERTISE_ADDRESS`: DNS name or IP covered by the certificate;
- `ARDENTS_BOOTSTRAP_PEERS`: explicit static bootstrap multiaddrs;
- `ARDENTS_DNS_DISCOVERY_URLS`: at most four comma-, semicolon-, or
  newline-separated signed `enrtree://` roots for `service_node`;
- `ARDENTS_DNS_DISCOVERY_NAMESERVER`: optional DNS resolver IP address used only
  with signed DNS roots; hostnames and address-plus-port values are rejected.
- `ARDENTS_REACHABILITY_MODE`: `outbound_only` (safe service default),
  `private_lan`, `public_direct`, or `local_only` for `local_development`;
- `ARDENTS_ADVERTISE_ADDRESSES`: exactly one TCP/WSS multiaddr without a
  `/p2p` identity suffix for inbound modes: a private literal translated-host
  address for `private_lan`, or a public IP/DNS address for `public_direct`.
  Other modes reject configured advertisements.

`private_lan` additionally requires the protected host-local topology adapter
to bind the exact manifest digest, target slot, and both other manifest source
slots before transport startup. Those fields are not free-form
`ardents.config/v1` input. Until that production adapter is composed, selecting
`private_lan` through the ordinary daemon configuration fails closed; the safe
service default remains `outbound_only`.

Signed DNS results are transport-filtered, deduplicated, and capped at 128
addresses. A refresh replaces the prior in-memory result; disappeared or failed
DNS knowledge is removed and its connections are closed unless the same address
is also an explicit static peer. Discovered peers are deliberately not persisted:
the signed tree remains the authority after restart. Static peers remain an
independent operator recovery mechanism.

Peer Exchange and Discv5 are not enabled in this profile. Peer Exchange's
available upstream protocol is still alpha and its responder population depends
on Discv5. Discv5 would add a UDP/NAT and dependency-security surface that must
be assessed before silently widening the current TCP/WSS contract.

WSS settings are invalid under `tcp_only`; Ardents does not retain dormant WSS
configuration for a later implicit profile change. The configured advertised
address replaces only the bind host of published WSS multiaddrs. It is not by
itself proof of external reachability.

Certificate files are read and validated on every transport start. Rotation is
performed by atomically replacing both deployment-managed files and executing a
controlled process/listener restart. There is no live reload and no automatic
certificate generation in `v1`. A failed validation leaves the node unstarted.
