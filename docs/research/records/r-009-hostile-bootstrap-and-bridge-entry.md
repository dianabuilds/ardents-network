---
id: R-009
title: How does a fresh or blocked endpoint join without one bootstrap owner?
status: active
owner: product research
started: 2026-08-08
reviewed: 2026-08-08
---

# R-009 — Hostile bootstrap and Bridge entry

## Decision this unlocks

Choose the bootstrap-state, entry-recovery, and Bridge-distribution architecture
worth prototyping with the R-004 route front-runner. This record does not select
signers, quorum, transports, bridge distributors, cryptography, or release
governance.

## Accepted gate

A fresh or blocked Windows or Ubuntu endpoint must:

- obtain enough authenticated current state and one usable entry path without a
  central User account, phone, email, wallet, administrator, or manual routing;
- reach network readiness within `p95 <= 15 s` on clean first start and
  `p95 <= 5 s` on routine restart in the normal qualification network;
- retain only installed immutable inputs on clean start and validate retained
  authenticated state on routine restart;
- tolerate stale, conflicting, malicious, seized, unavailable, and blocked
  bootstrap sources without silently accepting another network;
- use replaceable public entries or non-public Bridges and expose blocked,
  degraded, partitioned, or incompatible state honestly;
- avoid one permanently necessary address, operator, signer, directory, DNS
  provider, bridge broker, or trust key;
- treat Transport Camouflage as measurable best effort, not invisibility;
- keep the Bridge or entry within the User-adjacent role view: it may know User
  location and traffic metadata, but not Service Name, Service Target, opposite
  endpoint, full Route, or Application Data.

Bootstrap cannot be trust-free. A clean client needs immutable evidence for
which Ardents network and protocol it intends to join. The requirement is to
avoid **one** necessary root and to make threshold capture, expiry, conflict, and
fork behavior explicit under R-012.

## Primary-source findings

### Tor pattern

Tor clients obtain a multiply signed, time-bounded directory consensus and can
bootstrap through directory caches selected from a compiled fallback list. This
provides a coherent network view and strong freshness semantics, but the
authority set and shipped fallback list are explicit Control Plane roots.

Tor separates public relays from non-public Bridges and provides pluggable
transports below the routing protocol. Bridge distribution currently includes
project-operated web, in-client Moat, Telegram, and email channels. That shows
why multiple distribution paths matter, but copying them would make Ardents
depend on external accounts and one project broker.

Sources: [Tor consensus](https://spec.torproject.org/dir-spec/outline.html),
[Tor fallbacks](https://spec.torproject.org/dir-list-spec.html),
[Tor Bridges](https://support.torproject.org/tor-browser/circumvention/), and
[Tor transport adapters](https://spec.torproject.org/pt-spec/).

### I2P pattern

I2P fresh routers use reseed hosts to obtain a signed initial peer set over an
encrypted connection. It also supports file-based reseed bundles transferred by
a friend or another trusted source. After joining, netDb exploration grows the
router view without a central directory consensus.

This is operationally simple and naturally supports an offline bootstrap file,
but a malicious or captured reseed source can still attempt first-contact
eclipse. Signed input authenticates its source, not the truth or diversity of the
peer sample.

Sources: [I2P reseed hosts](https://geti2p.net/en/docs/reseed) and
[I2P file-based reseed](https://geti2p.net/en/blog/post/2020/06/07/file-based-reseed).

### libp2p and Waku pattern

libp2p Kademlia and Waku Discv5 can expand a peer view after one or more seed
peers are known. They do not remove the initial trust problem. libp2p documents
that Kad-DHT nodes can return incorrect or incomplete results and remain exposed
to Sybil attacks. Waku's documented flow starts from bootstrap ENRs, commonly
through DNS discovery; its peer exchange documentation warns that peer exchange
can reduce robustness outside high-trust environments.

Sources:
[libp2p security](https://docs.libp2p.io/concepts/security/security-considerations/),
[Waku Discv5](https://docs.waku.org/learn/concepts/discv5/), and
[Waku peer discovery](https://docs.waku.org/run-node/configure-discovery).

## Option A — Authority consensus and compiled fallbacks

A small explicit authority set votes on an epoch network manifest. Clients
accept a threshold-signed current consensus and fetch it from compiled fallback
mirrors. Separate project-operated brokers distribute Bridge addresses.

### Advantages

- one coherent view, explicit freshness, and straightforward rollback detection;
- fast clean bootstrap from many mirrors;
- well-understood operational separation between authorities, caches, relays,
  Bridges, and transport adapters.

### Costs and risks

- authority membership becomes a high-value governance root;
- compiled mirrors and project brokers are enumerable and blockable;
- bridge acquisition through project web, email, chat, or CAPTCHA can require
  external identity or access and concentrate observation;
- a one-to-one project cannot safely operate a credible authority set alone.

### Assessment

Useful reference, but unacceptable as the complete Ardents answer. Its consensus
freshness and cache ideas may be reused only with the R-012 multiparty power map.

## Option B — Independent signed reseeds plus DHT exploration

The client tries several independently operated signed reseed sources or imports
a signed file bundle. It merges peer samples, connects to several candidates, and
uses DHT exploration and peer exchange to expand and continuously replace its
view.

### Advantages

- simple, cheap, and compatible with direct file or person-to-person transfer;
- no always-online global consensus service;
- many operators can add distribution capacity independently;
- naturally fits a small network's first implementation.

### Costs and risks

- signatures identify sources but do not establish one current network state;
- sample merging can silently combine incompatible forks;
- a fresh client remains vulnerable to a coordinated or single-source eclipse;
- DHT and peer-exchange responses can be Sybil-biased, incomplete, stale, or
  selectively censored;
- source-specific trust settings can create resolver-like network meanings.

### Assessment

Operationally attractive but insufficient without a common authenticated epoch,
conflict rule, and cross-source evidence contract.

## Option C — Threshold epoch bundle plus independent distribution

Ship a small immutable **bootstrap trust set**, not a permanent address list. It
accepts a threshold-signed, expiring **epoch bundle** containing:

- network and protocol compatibility identifiers;
- monotonic epoch and freshness bounds;
- digests of the accepted rule/checkpoint state needed to detect another fork;
- a diverse sample of public discovery and entry candidates with authenticated
  Node keys and supported carrier transports;
- extensible Route Profile, role, and protocol capability versions without
  encoding one permanent route topology;
- signer-set version and transition evidence;
- no User identity, query history, Service Name, or application state.

The same content-addressed bundle may arrive through independent channels:

1. the installation package for clean-start fallback;
2. last-known-good authenticated cache on routine restart;
3. several mirrors at addresses named by an earlier accepted bundle;
4. already-known peers, which may relay but cannot forge a newer bundle;
5. a file, QR code, or copyable out-of-band package supplied by another person;
6. optional public web, CDN, or application channels whose endpoints are already
   authenticated by an accepted bundle and never become the sole source.

The client validates before merging. A newer valid epoch replaces older address
samples only under the authenticated transition rule. Two incompatible bundles
that both satisfy local trust cannot be silently combined; the endpoint reports
conflicting or forked network state. One reachable source is enough to obtain a
bundle, but no one source can define its contents.

After accepting the bundle, the endpoint contacts several candidates over
independent paths, verifies Node keys and protocol state, and expands its view
through bounded DHT exploration or peer sampling. Bootstrap peers supply
candidates, not truth. The client measures concentration and avoids deriving
independence from different Node IDs alone.

### Advantages

- separates authorization of network state from its distribution;
- no single mirror, address, DNS operator, broker, or ordinary peer is necessary;
- supports fast package/cached startup and hostile out-of-band recovery;
- conflict and fork state remain visible instead of becoming source-dependent;
- distribution and Route Profile capabilities can evolve without changing the
  Application Interface or Service Connection contract.

### Costs and risks

- the signer threshold is still a Control Plane and must be independently
  governed, rotated, expired, recovered, and forked under R-012;
- a one-to-one project cannot honestly claim signer independence at launch;
- freshness creates a hard problem when the clock is wrong or all current sources
  are blocked;
- public candidate lists remain enumerable, while private Bridges need separate
  distribution and probing controls;
- threshold capture can still publish malicious state; transparency makes capture
  visible but does not make it impossible.

### Assessment

Best fit for Ardents. It combines Tor-like authenticated epochs with I2P-like
multi-channel and file reseeding while keeping state authority separate from
distribution.

## Bridge entry contract for Option C

Ordinary entry and Bridge entry implement the same User-adjacent route role. A
Bridge changes how the first Carrier Channel is reached and classified; it never
shortens the Interactive Route, becomes a trusted proxy, or learns the Service
destination.

A tentative **Bridge Invite** is an authenticated, expiring, narrowly scoped
package containing only what is needed to reach one or more Bridge transports:

- Bridge Node key and one or more carrier endpoints;
- supported transport-adapter identifiers and bounded validity;
- an optional probing-resistant admission capability;
- the compatible network/epoch boundary;
- no User account, Service destination, global identity, or route trace.

Invites may be transferred as a file, QR code, copyable text, removable media, or
through any third-party channel the User already has. Public brokers may exist,
but none is mandatory or authoritative. A capability must not become a stable
cross-context User identifier; exact group, blind, reusable, or one-use admission
semantics remain R-010 mechanism research.

The Bridge does not answer an unauthenticated active probe with an Ardents-
specific success signal when a selected transport can avoid doing so. Transport
adapters sit below the route and are replaceable. Each candidate must measure
passive classification, active probing, replay, address enumeration, distribution
leakage, and collateral blocking; no adapter is called invisible.

There is an unavoidable limit: if a censor blocks every address and transport a
fresh endpoint knows and the User has no out-of-band channel, the endpoint cannot
discover a secret new address from inside the blocked network. Ardents reports
blocked state instead of promising a cryptographic escape from missing
information.

## Comparative result

| Criterion | A: authority consensus | B: signed reseeds | C: threshold epoch bundle |
|---|---|---|---|
| One coherent current network view | Strong | Weak | Strong with explicit fork |
| One necessary online address | No, with enough fallbacks | No, with enough reseeds | No |
| One source defines truth | Authority set | Often source-local | Threshold signers, not distributors |
| Clean-start eclipse resistance | Medium | Weak | Best current hypothesis |
| Offline/person-to-person recovery | Additional mechanism | Native | Native |
| Censorship resistance | Bridges and transports, broker risk | File and cloud reseeds | Independent channels plus Bridge Invites |
| Governance concentration | Highest and explicit | Hidden in source trust | Explicit and delegated to R-012 |
| One-to-one launch practicality | Poor | Best | Medium with honest provisional trust |
| Current disposition | Reference | Fallback component | Front-runner prototype |

The table is a design assessment, not evidence that any mechanism meets the
startup or censorship-resistance gates.

## Prototype decision gate

A throwaway R-009 prototype must use synthetic keys and peers and test:

1. package-only clean start, cached routine restart, mirror retrieval, peer relay,
   and offline file import;
2. one stale, malicious, selectively incomplete, unavailable, and captured source
   in turn;
3. incompatible threshold-valid bundles, rollback, expiry, wrong local clock,
   partition, signer rotation, and missing quorum;
4. contact with several candidate entries without treating their Node IDs as
   independent operators;
5. normal `15 s` clean and `5 s` routine readiness gates with complete CPU, RSS,
   traffic, and failure evidence;
6. public entry blocking, Bridge Invite import, active Bridge probing, invite
   replay, and transport-adapter replacement;
7. explicit blocked, degraded, stale, conflicting, and forked results without
   direct, weaker-route, DNS-selected, or administrator-selected fallback.

## Recommendation

Advance **Option C, threshold epoch bundle with independent distribution**, to a
bounded prototype. Reuse independent signed reseed files from **Option B** as one
distribution channel, not as source-local truth. Reuse Tor's consensus freshness,
fallback diversity, Bridge-role separation, and pluggable-transport boundary
without copying one project-operated authority or bridge-distribution service.

For an early one-to-one deployment, the initial signer set may necessarily be
provisional and project-controlled. The product must label that centralization
truthfully, publish its power, and make signer diversification a release gate
rather than claiming the final decentralized state already exists.

This recommendation is reversible. R-012 still decides the signer, update,
emergency, capture, and fork power model.

## Disposition

- State: `active`; no Product Owner bootstrap or Bridge mechanism decision has
  been accepted.
- Threshold epoch bundles and Bridge Invites are research candidates, not
  canonical product terms or selected wire formats.
- Public DNS discovery is neither required nor part of the recommended baseline.
  HTTPS, CDN, chat, email, and app delivery are also unselected and could only
  transport already authenticated bundle bytes, never define network state.
- No signer set, quorum, DHT, broker, transport adapter, library, language, or
  production mechanism is selected.
- No ADR and no production code.
