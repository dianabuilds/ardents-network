---
id: R-006
title: What is the V1 lifecycle of a Service Target?
status: decided
owner: product research
started: 2026-08-07
reviewed: 2026-08-08
---

# R-006 — Service Target lifecycle

## Decision this unlocks

Define what Ardents addresses before specifying the Application Interface,
Service Name protocol, route construction, or implementation foundations.

The result must explain creation, publication, routine migration, loss,
compromise, replacement, and retirement without turning a Node ID or User
identity into the Service address.

## Current contract

- [Network functional map](../../product/functional-map.md)
- [J-03: publish a local Service](../../product/journeys.md#j-03--publish-a-local-service)
- [Domain language](../../../CONTEXT.md)
- [Threat model](../../security/threat-model.md)
- [ADR-0001](../../adr/0001-public-carrier-private-services.md)

Already fixed: a Service Target is location-independent, distinct from a Node
and User, and may be reached through a human Service Name. An offline Service
does not receive retained Application Data by implication.

Open at research start: whether ordinary migration copies one permanent secret,
creates a new target behind the same name, or uses a permanent offline authority
that delegates bounded credentials to one or more Service Instances.

## Hypotheses

- **H1a — Online portable Service Authority:** one active V1 Service Instance
  possesses the Target root and routine migration exports it. This is retained as
  the rejected simple-host baseline.
- **H1b — Bounded single Instance:** the active host generates a private Instance
  Key; a durable Authority signs its public key, exclusive generation, validity,
  network, and capabilities. Routine migration creates a new key and higher
  generation without exporting the old runtime secret.
- **H2 — Concurrent delegated Instances:** one durable offline authority
  authorizes several active Instance Keys with an explicit multihoming policy.
- **H3 — Name-only continuity:** every deployment may create a new Service
  Target; only the Service Name provides continuity between hosts.
- **H0 — No accepted lifecycle:** none of the options gives a usable and honest
  recovery story for the first tracer.

## Evaluation criteria

1. A Developer can explain normal migration and compromise recovery without
   learning routing internals.
2. A routine move does not change the identifier used by a machine integration.
3. A compromised old host cannot be described as revoked when it still holds
   sufficient authority to impersonate the target.
4. The design bounds online host compromise without requiring multihoming or a
   permanent root on every runtime; credential freshness is explicit from V1.
5. Service Name recovery remains distinct from Service Target authentication.
6. A lost secret, copied old host, simultaneous publication, and missing Service
   Name each have an honest result.
7. The model can later add concurrent delegation without changing the Target or
   Application Interface.

## Evidence plan

### Primary sources

Sources were checked on 2026-08-07:

- [Tor Onion Service setup](https://community.torproject.org/onion-services/setup/)
- [Tor Onion Service protocol overview](https://spec.torproject.org/rend-spec/protocol-overview.html)
- [Tor OnionBalance v3 proposal](https://spec.torproject.org/proposals/307-onionbalance-v3.html)
- [I2P eepsite creation and key backup](https://i2p.net/en/docs/guides/creating-an-i2p-eepsite/)
- [I2P application Destinations](https://i2p.net/en/docs/develop/applications/)
- [I2P encrypted LeaseSet and offline keys](https://i2p.net/en/docs/specs/encryptedleaseset/)
- [IPNS concepts](https://docs.ipfs.tech/concepts/ipns/)
- [ENS registry](https://docs.ens.domains/registry/ens/)

### Experiment

No protocol code is required. A future implementation experiment must test
host-generated key requests, public credential issuance, expiry and uncertain
time, strict generation supersession, stale simultaneous publishers, Instance
Key compromise, isolated Recovery Bundle import, monotonic post-restore
reconciliation, root loss, and Service Name replacement after root compromise.

### Failure scenarios

- the old host retains its private Instance Key and unexpired Credential;
- old and new hosts race stale and higher Instance generations;
- a public Credential is copied without its private key;
- the only Authority Recovery Bundle is stale, lost, or corrupted;
- an isolated test restore succeeds but current network state cannot be
  reconciled safely before signing;
- an attacker obtains the authority before compromise is detected;
- the Service has no Service Name when its target must be replaced;
- the naming system is unavailable during compromise recovery;
- a Developer mistakes routine migration for safe compromise recovery;
- a future multihoming feature silently shares permanent authority or one
  Instance Key with every host.

## Findings

- **Sourced fact:** a Tor Onion Service address is derived from its identity key.
  Tor's operational guide recommends backing up the private key; disclosure lets
  another party impersonate the Service. The protocol has separate master,
  blinded, descriptor-signing, and introduction keys, and describes offline
  master-key operation, while multi-instance operation introduces additional
  OnionBalance key and coordination machinery.
- **Sourced fact:** I2P documents a Destination as a mobile cryptographic
  application endpoint. Its eepsite guide instructs operators to copy the same
  private key when migrating and to use the same key for basic multihoming. I2P
  also specifies transient offline-signing keys, demonstrating that safer
  delegation is possible but not operationally free.
- **Sourced fact:** an IPNS name is derived from a public key and its records are
  signed by the corresponding private key. The authority must therefore remain
  available to update that self-certifying name.
- **Sourced fact:** ENS separates a stable human name from the machine-readable
  record selected by its owner, so the record can change while the name remains.
- **Inference:** portable cryptographic authority is an established minimum for
  a self-authenticating service address. It has a simple migration story but no
  credible same-target revocation after the authority is copied or stolen.
- **Inference:** Ardents' separate Service Name gives H1 a truthful catastrophe
  recovery path: replace the compromised Service Target and rebind the name.
- **Inference:** offline-root delegation materially limits the duration and
  scope of a server compromise even with one active Instance, so H1b is required
  from V1. Concurrent H2 still adds multihoming selection, coordination, and
  split-brain contracts without a current journey.
- **Inference:** H3 makes even an ordinary move depend on naming availability and
  breaks integrations that intentionally pin the Service Target. It is simpler
  for the host but weaker as a network address contract.

## Options

### H1a — Online portable Service Authority

- Product fit: simple creation and migration, but every host receives the root.
- Security fit: the Service Target authenticates one durable authority; a copied
  authority is a full compromise and is never described as safely revoked.
- Operational dependency: one active Instance, secure backup, retirement of the
  old host, and Service Name recovery for target replacement.
- Governance root: Service Name recovery becomes the continuity root after
  target compromise.
- Main risk: the network cannot prove that only one copy exists or stop a stale
  host from publishing with the same authority.

**Result:** rejected for the normal V1 runtime. Co-location remains an explicitly
weaker custody mode, not the architecture.

### H1b — Bounded single Instance

- Product fit: one active Instance and stable Target with ordinary migration.
- Security fit: the host keeps only one generated Instance Key and a public
  root-signed bounded Credential; copying the Credential alone grants no power.
- Operational dependency: Time Confidence, monotonic generation convergence,
  offline-capable signing requests, and an Authority Recovery Bundle.
- Main risk: a stolen Instance Key impersonates the Target until expiry or
  supersession becomes current; this is bounded, not instant revocation.

### H2 — Concurrent delegated Instances

- Product fit: strongest for several hosts, third-party hosting, and bounded
  server compromise.
- Security fit: the durable root need not be present on a server.
- Operational dependency: credential expiry, renewal, revocation distribution,
  trustworthy time, multi-instance selection, and split-brain policy.
- Main risk: large unvalidated control plane and recovery surface for V1.

### H3 — Name-only continuity

- Product fit: easy host replacement for users who always enter a Service Name.
- Security fit: compromise recovery replaces the target cleanly.
- Operational dependency: naming must be available for every ordinary move.
- Main risk: a Service Target is no longer durable enough for machine pinning or
  name-independent use.

## Recommendation

Choose **H1b — one host-generated Service Instance Key with one bounded public
Credential** for V1. This retains one active Instance and one durable Target but
removes the unnecessary requirement that the Target root remain in the online
runtime.

The Instance Key authenticates a fresh ephemeral endpoint key exchange; it is
not itself the Service Connection traffic key. Completed connections and retired
carrier legs erase ephemeral keys best-effort, so later Service Authority,
Instance Key, or Node long-term-key compromise cannot retrospectively decrypt
recorded traffic from endpoints that were honest during the connection. This is
a required Forward Secrecy property, while live endpoint compromise and physical
memory/snapshot erasure remain honest limitations.

The accepted lifecycle is:

1. **Create:** create a Service Authority and derive one Service Target.
2. **Authorize Instance:** the new host generates a private Service Instance Key
   and sends its public key in a request. After monotonic reconciliation, Service
   Authority signs one public bounded Credential binding that key, Target,
   exclusive generation, validity, network, and capabilities. Root custody may
   be offline or co-located with an explicit warning.
3. **Publish:** one active Service Instance proves the Credential with its
   private Key to publish reachability and authenticate the Target without
   gaining root export or successor authority.
4. **Bound live use:** every Service Connection binds the exact Key/Credential
   proof and has a terminal `not-after` no later than Credential validity and the
   applicable Work Safety Lease. Learned authenticated supersession may stop new
   leg/recovery work earlier and sets a finite terminal deadline. A partition can
   delay learning supersession, so instant revocation is not claimed; expiry is
   the unconditional finite bound.
5. **Stop:** absence of a live Instance makes the Service unavailable; nothing
   is retained for later delivery.
6. **Routine migration:** stop or withdraw the old Instance; the new host creates
   a new Key; Authority advances the exclusive generation and signs that public
   key; the new host republishes the same Target. Neither old Instance Key nor
   Service Authority is exported to the new runtime.
7. **Unsafe overlap:** two still-valid credentials for the same exclusive
   generation are a publisher error or possible compromise. The protocol does
   not pretend they are safe multihoming; a newer accepted generation supersedes
   the older one under bounded freshness rules.
8. **Instance compromise:** stop renewal and advance the exclusive generation.
   The stolen Instance Key plus its public Credential remains dangerous until
   validity ends or the newer generation and its deadlines become known; the
   public Credential alone is not a secret and no instant-revocation claim is
   made.
9. **Root loss or compromise:** create a new Service Authority and Service Target,
   then use the accepted Service Name recovery mechanism to bind the existing
   name to the replacement. A Name-origin connection binds its exact Name
   generation/revision→Target: learned Recovery Pending, Release, or rebind to the
   replacement stops new leg/recovery work and closes it by a finite deadline,
   never silently migrates it. The old target remains permanently untrusted.
10. **No name:** if the target must be replaced without a Service Name, the new
   target must be redistributed through an external trusted context. An explicit
   Target/Target-Link connection is pinned and intentionally receives no Name
   rescue.
11. **Retire:** withdraw publication and never reassign the retired target to a
   different Service.
12. **Restore:** decrypting a Recovery Bundle in isolated no-sign mode proves
    format only. Before signing, reconcile current authenticated generation state
    and advance beyond it; unavailable/conflicting state leaves the root
    export-only and `authority locked`.

Exact key algorithms, credential and connection duration, generation convergence,
supersession deadlines, Recovery Bundle format, local secure storage, descriptor
encoding, and naming recovery mechanism remain R-013, R-002, R-003, and Time
Confidence implementation work.

Confidence is **high** in the key hierarchy and **medium** in the still-unmeasured
credential lifetime and recovery ergonomics. Hostile server compromise is a
normal Ardents threat, so the protocol must support bounded delegation from its
first production form even though V1 still has only one active Instance.

## Disposition

- State: `decided` for the V1 product contract.
- H1 is revised to H1b; bounded single-instance delegation is accepted while
  concurrent multihoming and third-party delegation remain deferred. H3 remains
  rejected as the ordinary lifecycle.
- Service Authority, Service Instance Key, Service Instance Credential, and
  Authority Recovery Bundle are canonical product language.
- R-002's privilege boundary remains: Authority Custody controls Service
  Authority and authorizes/advances a public Credential for a host-generated
  public Instance key, while runtime Service Administration uses the matching
  private Key plus public Credential and can export neither Key nor root.
- R-003 P4-D1 now fixes Name Authority separately from Service Authority so the
  stable name can bind a replacement after target compromise or loss. R-003 now
  also fixes its allocation, authority lifecycle, Private Resolution, and
  governance product boundaries; exact mechanisms remain R-013 work.
- The online-root correction is recorded in ADR-0003 because shipping permanent
  root authority to every runtime would be costly to reverse.
- No experiment or production code created.
