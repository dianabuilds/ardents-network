---
id: R-006
title: What is the V1 lifecycle of a Service Target?
status: decided
owner: product research
started: 2026-08-07
reviewed: 2026-08-07
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

- **H1 — Portable Service Authority:** one active V1 Service Instance possesses
  the authority for a stable Service Target. Routine migration uses an encrypted
  export/import. Suspected compromise or unrecoverable loss creates a new
  Service Target and rebinds the Service Name.
- **H2 — Delegated Instances:** a durable offline authority remains separate and
  authorizes short-lived keys for multiple Service Instances. Compromise of an
  Instance can be bounded without replacing the Service Target.
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
4. The design does not require multihoming, certificate distribution, clocked
   delegation, or revocation infrastructure before the first tracer needs them.
5. Service Name recovery remains distinct from Service Target authentication.
6. A lost secret, copied old host, simultaneous publication, and missing Service
   Name each have an honest result.
7. The model can later add bounded instance delegation without promising that
   extension in V1.

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

No protocol code is required. R-002 will later exercise the accepted lifecycle
through an Application Interface contract. A future implementation experiment
must test encrypted export/import, stale simultaneous publishers, loss of the
old host, and Service Name replacement after compromise.

### Failure scenarios

- the old host retains a copied authority after migration;
- both old and new hosts publish the same Service Target;
- the only authority backup is lost or corrupted;
- an attacker obtains the authority before compromise is detected;
- the Service has no Service Name when its target must be replaced;
- the naming system is unavailable during compromise recovery;
- a Developer mistakes routine migration for safe compromise recovery;
- a future multihoming feature silently shares permanent authority with every
  host.

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
- **Inference:** H2 materially limits the duration and scope of a server
  compromise and supports multiple hosts, but those benefits are not required
  by a one-active-Instance tracer. It adds expiry, revocation, clock, delegation,
  coordination, and split-brain contracts before they have a user journey.
- **Inference:** H3 makes even an ordinary move depend on naming availability and
  breaks integrations that intentionally pin the Service Target. It is simpler
  for the host but weaker as a network address contract.

## Options

### H1 — Portable Service Authority

- Product fit: simple creation, encrypted backup, routine migration, and honest
  catastrophic replacement.
- Security fit: the Service Target authenticates one durable authority; a copied
  authority is a full compromise and is never described as safely revoked.
- Operational dependency: one active Instance, secure backup, retirement of the
  old host, and Service Name recovery for target replacement.
- Governance root: Service Name recovery becomes the continuity root after
  target compromise.
- Main risk: the network cannot prove that only one copy exists or stop a stale
  host from publishing with the same authority.

### H2 — Delegated Instances

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

Choose **H1 — Portable Service Authority** for V1.

The accepted lifecycle is:

1. **Create:** create a Service Authority and derive one Service Target.
2. **Publish:** one active Service Instance publishes reachability for that
   target.
3. **Stop:** absence of a live Instance makes the Service unavailable; nothing
   is retained for later delivery.
4. **Routine migration:** stop or withdraw the old Instance, transfer an
   authenticated encrypted authority export, import it on the new host, and
   republish the same Service Target. The Service Name does not change.
5. **Unsafe overlap:** if two hosts retain the authority, V1 cannot distinguish
   the legitimate one and does not promise deterministic multi-instance
   behavior. Treat unexplained overlap as possible compromise.
6. **Loss or compromise:** create a new Service Authority and Service Target,
   then use the accepted Service Name recovery mechanism to bind the existing
   name to the replacement. The old target remains permanently untrusted.
7. **No name:** if the target must be replaced without a Service Name, the new
   target must be redistributed through an external trusted context.
8. **Retire:** withdraw publication and never reassign the retired target to a
   different Service.

Exact key algorithms, export format, local secure storage, descriptor encoding,
and naming recovery mechanism remain R-013, R-002, and R-003 work.

Confidence is **high** for the bounded first tracer and **low** for long-term
single-authority operations in high-value deployments. The strongest argument
against H1 is that hostile server compromise is a normal Ardents threat, so
bounded instance delegation may become necessary earlier than multihoming does.

## Disposition

- State: `decided` for the V1 product contract.
- H1 selected; H2 deferred until a concrete multi-instance, third-party hosting,
  or bounded-compromise journey exists; H3 rejected as the ordinary lifecycle.
- Service Authority becomes canonical product language.
- R-002 is next and must expose creation/import, single active publication, and
  honest failure without selecting a key format.
- R-003 must provide Service Name replacement after target compromise or loss.
- No ADR: the decision is explicit, scoped to V1, and reversible before a wire
  format or production implementation exists.
- No experiment or production code created.
