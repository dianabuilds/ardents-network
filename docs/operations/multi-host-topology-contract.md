# Multi-host topology manifest contract

MR-01 admits and compiles one pure `ardents.topology/v1` document before any
host, network, DNS, PKI, Authority, checkpoint repository, Node, process, or
journal is touched. The implementation owner is `internal/deployment`;
`Compile([]byte)` returns a deterministic `ardents.topology.plan/v1` projection
or one stable redacted `ValidationError`.

## Frozen v1 shape

The document is UTF-8 JSON no larger than 256 KiB. Decoding rejects duplicate
or unknown fields, trailing values, malformed JSON, and every version other
than `ardents.topology/v1`.

The top-level contract contains:

- mode `private_lan` or `public_direct`;
- transport profile `tcp_only` or `tcp_wss`;
- an opaque workstation Operator signer alias;
- exact clock bounds of 30 seconds maximum skew and 60 seconds Authority
  validity margin;
- one Authority slot with distinct opaque state and backup references;
- one external checkpoint-repository reference with immutable history and
  exactly 65,536 retained-head capacity;
- zero through four signed `enrtree://<key>@<fqdn>` roots;
- exactly three Node records.

Each Node is an operator-owned Linux amd64 `service_node` on a distinct `host`
failure domain. It has unique stable slot, SSH alias, host-key-pin reference,
Node-state reference, canonical Node Principal and canonical Waku Peer ID
bindings, plus an immutable canonical SHA-256 container image reference. Every
Node names both other slots as static recovery peers. At least two distinct
Nodes must each be bootstrap-capable and a persistent Store provider with one
of `bounded_24h`, `bounded_7d`, or `bounded_30d` retention.

Failure-domain classes are closed:

- `host` for a Node and the Authority consistency group hosted on the
  designated Authority Node;
- `backup` for the independently retained Authority backup;
- `external_repository` for the independently administered immutable
  checkpoint repository.

All protected aliases are bounded opaque references, not filesystem paths or
credentials. Node state, Authority state, Authority backup, checkpoint
repository and host-key-pin references are unique. Fields that imply passwords,
private keys, signer paths/files, Sessions, Credentials, Channel Grants,
channel secrets, selectors, certificate keys, or repository credentials are
rejected.

## Reachability modes

`private_lan` requires exactly one canonical literal private IP TCP multiaddr
per Node. The compiled plan marks it `private_probe_required`; it never reports
public reachability. Publication remains blocked until a later slice records a
bounded cross-host probe.

`public_direct` requires at least two Nodes with exactly one canonical literal
public IP TCP or WSS multiaddr. A Node without explicit inbound routing is
`outbound_only` and has no address or certificate fields. Public plans are
`public_autonat_required`; they do not report public reachability. WSS is valid
only under `tcp_wss` and requires an opaque certificate reference whose
declared identity equals the literal advertised IP. Certificate acquisition
and verification remain later Operator/PKI work.

Loopback, unspecified, link-local, multicast, cross-scope, duplicate,
non-literal, UDP, QUIC, Circuit Relay, DNS-address, `/p2p`, and mixed-mode
addresses are rejected. The compiler never mutates a router, firewall, DNS
provider, certificate service, or runtime.

## Redacted plan

Plans are sorted by stable Node slot, and every static peer list is sorted.
Canonical-equivalent input therefore produces the same value on Linux and
Windows tooling runners.

An ordinary plan contains only:

- mode and transport profile;
- stable Node slots, authority/member role, profile, bootstrap/Store booleans,
  and peer slots;
- required later checks such as host-key pin, identity binding, immutable
  image, clock, private cross-host probe, public AutoNAT, and WSS certificate
  identity;
- the fixed Authority/checkpoint/clock invariants.

It contains no hostname, IP, multiaddr, DNS root, SSH alias, host-key
reference, Principal, Waku Peer ID, state/repository/signer/certificate
reference, image name, or digest. Validation errors return a stable bounded
code only and never echo manifest values.

MR-01 does not add topology status, SSH forwarding, signer or Session access,
Authority placement/recovery, fencing, rollout journals, host mutation, or
real-host qualification. `deployment.multi-host` remains `Q=no`.
