---
id: R-017
title: Is Named Private Site + Anonymous Mailbox the right first tracer?
status: decided
owner: product research
started: 2026-08-06
reviewed: 2026-08-06
---

# R-017 — Named Private Site + Anonymous Mailbox

## Decision this unlocks

Decide whether **Named Private Site + Anonymous Mailbox** is the smallest
coherent vertical slice that both demonstrates Ardents' differentiated value and
provides a stable target for naming, identity, routing, storage, application
isolation, and implementation-language research.

This decision accepts or changes the tracer contract. It does not claim
product-market fit, select the first user segment, or select a protocol or
language.

## Current contract

- [Product vision](../../product/vision.md)
- [Functional map](../../product/functional-map.md)
- [J-02: open a private site](../../product/journeys.md#j-02--open-a-private-site)
- [J-03: publish a site or client application](../../product/journeys.md#j-03--publish-a-site-or-client-application)
- [Threat model](../../security/threat-model.md)
- [ADR-0001](../../adr/0001-public-carrier-private-services.md)

Already fixed: Ardents is an internal application network with a public carrier,
private or capability-gated Services, human-facing Service Names, no mandatory
wallet, no clearnet exit in the first product, and no universal Person identity.

At research start it was still open whether a Site and Mailbox formed one
necessary product experience. P1-D2 resolved that product-scope question by
rejecting the built-in Mailbox experience.

## Pre-registered hypotheses

- **H1 — Combined tracer:** a named read surface plus a protected asynchronous
  reply path is the smallest coherent slice. The Site establishes context and
  intent; the Mailbox enables a relationship or transaction without requiring a
  general stateful backend.
- **H2 — Site-only:** a named, replicated private site is independently valuable
  and differentiated enough; Mailbox adds avoidable identity, storage, abuse,
  and metadata scope.
- **H3 — Mailbox-only:** anonymous asynchronous messaging or submission is the
  actual urgent job; the Site and application runtime add avoidable scope and
  duplicate ordinary publishing tools.
- **H0 — Reject current tracer:** none of these slices has sufficiently clear
  first-user value or differentiation to constrain the network architecture.

## Pre-registered evaluation criteria

Each option is scored qualitatively as `fails`, `weak`, `adequate`, or `strong`.
Evidence must support the explanation; the labels are not measurements.

1. **Coherent user job:** one actor can explain the outcome without naming
   routing, replicas, cryptographic addresses, or protocol components.
2. **Differentiation:** the outcome is not already achieved with comparable
   safety and effort by composing mature existing products.
3. **Vision coverage:** the slice exercises both a Person and a Developer and
   demonstrates an application network rather than only a messenger or proxy.
4. **Vertical completeness:** naming, publication, retrieval, endpoint privacy,
   protected response, update, recovery, and one failure path are observable.
5. **Bounded scope:** it avoids general stateful compute, large groups, payment,
   public social discovery, and clearnet exit.
6. **Hostile-environment value:** loss of a host, censorship, malicious storage,
   unsolicited traffic, and endpoint-location exposure are material to the job.
7. **Honest safety:** a prototype can expose limitations without encouraging a
   high-risk user to depend on unproven anonymity.
8. **Adoption path:** a Developer can publish something useful and a Person can
   use it without both first recruiting a large social graph.
9. **Research leverage:** the slice forces decisions that remain useful for
   later private applications rather than one-off features.
10. **Falsifiability:** usability and comparison tests can reject the slice
    before a production network is built.

## Decision rule

Recommend H1 only if it is at least `adequate` on all ten criteria and stronger
than H2 and H3 on vision coverage, vertical completeness, adoption path, and
research leverage without failing bounded scope or honest safety.

If H1 is architecturally useful but first-user evidence is absent, classify it
as an **architecture tracer**, not a validated V1 product, and keep R-016 open.

## Evidence plan

### Existing systems

Compare official product and protocol documentation for:

- Tor onion services, I2P applications/eepsites, IPFS/IPNS, and
  Hyphanet/Freenet-style publishing;
- Session, SimpleX, Briar, Cwtch, SecureDrop, and OnionShare-style asynchronous
  contact or submission;
- any maintained product that already combines publisher location privacy,
  human naming, authenticated releases, and asynchronous protected reply.

For each, record the exact job, onboarding, naming, endpoint privacy, offline
delivery, updates/recovery, operator burden, and control roots.

### Scenario analysis

Evaluate at least these plausible first-user situations without claiming they
have been validated:

- a censored independent publisher needing safe two-way contact;
- a small organization publishing information and accepting protected inquiries;
- a Developer shipping a private client-side tool to invited users;
- a community publishing a private resource and receiving asynchronous requests.

### Current one-to-one decision evidence

As of 2026-08-06, the active team is the Product Owner and Codex working
one-to-one. No participant panel or additional product, engineering, operations,
or security staff is available or assumed.

R-017 may therefore accept an **architecture tracer** through sourced research,
an incumbent comparison, and a structured Product Owner walkthrough. That
decision means “this is the most useful hypothesis for our next research,” not
“users want this product.” It cannot validate market demand, first-user fit,
novice comprehension, or real-world anonymity.

R-016 and the external-observation part of R-018 remain open future gates. They
do not block reversible architecture research, but no market-facing claim may be
upgraded by substituting the Product Owner for an independent participant.

### Experiment

No protocol code is required for the initial decision. If desk research supports
H1, create a non-networked journey specification or disposable interaction
prototype covering publish, open, permission, send, offline reply, update,
recovery, and failure messaging. Walk it through together from Developer and
Person perspectives before network implementation. This tests consistency and
Product Owner acceptance, not external usability.

## Failure scenarios to test

- the Site is useful but nobody needs to reply;
- users need submission but do not trust active application content;
- Mailbox abuse makes a public name unsafe by default;
- human naming exposes the publisher or visitor graph;
- the combined permission and recovery journey is incomprehensible;
- a high-risk user mistakes the prototype for protection from a global observer;
- mature separate tools solve the job more safely with acceptable operational
  burden;
- the first useful interaction already requires a stateful Service Instance.

## Findings

Sources were checked on 2026-08-06. Product and protocol claims below are the
projects' own documented claims unless an independent review is named; they are
not treated as proof that Ardents can provide the same property.

### A real user job exists, but the feature pairing is not unique

- **Sourced fact:** SecureDrop gives an anonymous source a service-specific
  submission site, a random return codename, asynchronous text/file submission,
  and later journalist replies. Its production safety also depends on dedicated
  hardware, Tor, isolated workstations, and a specialized newsroom procedure.
  See [source workflow](https://docs.securedrop.org/en/stable/source/how_to_submit.html),
  [threat model](https://docs.securedrop.org/en/stable/threat_model/threat_model.html),
  and [installation overview](https://docs.securedrop.org/en/latest/admin/installation/installation_overview.html).
- **Sourced fact:** OnionShare already offers website, receive, and chat modes
  over onion services. Receive and persistent website modes depend on the
  publisher's machine remaining online, while Chat Mode does not retain
  history. See [features](https://docs.onionshare.org/2.6.3/en/features.html)
  and [security design](https://docs.onionshare.org/2.6.3/en/security.html).
- **Inference:** Those modes do not form an independent store-and-forward reply
  path when the publisher is offline.
- **Sourced fact:** CPJ guidance treats protecting sources, contact metadata,
  targeted phishing, endpoint compromise, and continued protected
  communication as operational concerns for journalists and small independent
  organizations. See the [digital safety kit](https://cpj.org/2019/07/digital-safety-kit-journalists/)
  and [source-protection guide](https://cpj.org/2021/11/digital-physical-safety-protecting-confidential-sources/).
- **Inference:** A contextual read surface plus a protected asynchronous reply
  path is a coherent job in hostile environments. SecureDrop and OnionShare
  also show that merely putting those two feature labels beside each other is
  not Ardents' differentiation.
- **Assumption:** Independent publishers, small organizations, private-tool
  developers, or community stewards experience this job often enough to switch
  tools. Desk research does not establish that assumption; R-016 must test it.

### Publishing and naming landscape

| System | Sourced fact | Decision-relevant inference |
|---|---|---|
| Tor onion services | An onion service hides the server location, authenticates the service through its address, and uses introduction and rendezvous points. A v3 address is a 56-character encoding derived from the service identity key; operating a durable service still requires origin operational security and key backup. [Overview](https://community.torproject.org/onion-services/overview/index.html), [setup](https://community.torproject.org/onion-services/setup/) | Tor is a strong candidate source of established network patterns, but an onion address is deliberately not Ardents' ordinary human name, and Tor does not supply replicated releases or an asynchronous service reply contract. |
| I2P naming | I2P maps human-readable names through local address books and optional subscription sources. Names are only locally unique; the documentation explicitly describes the resulting differences of opinion and impersonation risk. [Naming](https://beta.i2p.net/en/docs/overview/naming/) | I2P demonstrates a human-facing internal application network, but local naming disagreement is incompatible with silently presenting one globally verified service identity. The trade-off belongs in R-003. |
| IPFS/IPNS | IPNS is a mutable, self-certifying pointer to immutable content identifiers. IPFS does not guarantee persistence without pinning, and its public DHT and provider interactions expose PeerIDs, CIDs, and access signals; content is not private by default. [IPNS](https://docs.ipfs.tech/concepts/ipns/), [persistence](https://docs.ipfs.tech/concepts/persistence/), [privacy](https://docs.ipfs.tech/concepts/privacy-and-encryption/) | Content-addressed releases and signed mutable pointers are reusable patterns. Public IPFS/IPNS by itself does not meet Ardents' endpoint, query, content, or relationship-privacy contract. |
| Hyphanet | Hyphanet inserts content into a decentralized adaptive cache. Content Hash Keys authenticate immutable data; Signed and Updatable Subspace Keys support signed versioned sites. Ordinary access still starts from a long key, updates are discovered by version search, unused data may disappear, deletion is unavailable, and first retrieval is inherently high-latency. Its social functions are separate plugins. [Documentation](https://www.hyphanet.org/pages/documentation.html), [operational help](https://www.hyphanet.org/pages/help.html) | Hyphanet is the strongest publishing precedent for origin-independent signed sites and censorship-resistant retention. Its opaque addressing, probabilistic persistence, slow retrieval, deletion semantics, and fragmented application UX are precisely the product costs Ardents must not inherit silently. |
| OnionShare | Static sites and anonymous receive are simple to start, but use an opaque onion address and a publisher-hosted endpoint. | It is the closest proof that the combined interaction is understandable, and the clearest baseline for testing whether human naming, independent replication, offline reply, and update continuity justify Ardents. |

### Messaging and first-contact landscape

| System | Sourced fact | Decision-relevant inference |
|---|---|---|
| Session | A long-lived Account ID is the contact address; message requests and replicated swarm storage support offline messaging. The documented path uses low-latency onion requests, and Session's contributor model uses token staking. [Account IDs](https://docs.getsession.org/session-network/session-protocol/account-ids-and-self-managed-keys), [routing](https://docs.getsession.org/session-network/session-protocol/onion-requests-and-message-routing), [network](https://docs.getsession.org/session-network/) | Useful patterns are request quarantine and expiring replicated storage. No global timing-correlation guarantee can be inherited from that routing description. A universal long-lived messaging identifier and token network are not required parts of Ardents' product. |
| SimpleX | One-time invitations or replaceable contact addresses create pairwise connections backed by separate one-way queues; the network has no shared user identifier. IP hiding from messaging servers requires Tor or another proxy configuration, and traffic correlation remains a documented limitation. [Connections](https://simplex.chat/docs/guide/making-connections.html), [messaging](https://simplex.chat/messaging/), [privacy](https://simplex.chat/docs/guide/privacy-security.html) | This is the strongest reference for separating initial service discovery from pairwise delivery identifiers. It nearly solves the Mailbox component, but not verified publishing, application isolation, or Ardents' stronger Shielded Route question. |
| Nym | Nym's maintained implementation exposes mixnet clients for applications; mix nodes shuffle Sphinx packets, and gateways can retain messages for offline or firewalled clients. The current network also couples operator rewards and Sybil resistance to token staking, and its components use multiple licenses. [Official repository](https://github.com/nymtech/nym), [network overview](https://nym.com/network) | Nym is a serious candidate source of Shielded Route patterns or implementations, not an answer to Service naming, signed Site releases, application isolation, or the reply-thread product. Its observer claim, latency/cover budget, gateway metadata, economics, licensing, and integration fit belong in R-005 and R-013. |
| Briar | Contacts are established through mutual QR verification or a bilateral link exchange. Offline delivery otherwise requires overlap or a separate always-on Android Mailbox device. In July 2026 the project entered maintenance mode. [Quick start](https://briarproject.org/quick-start/), [Mailbox](https://briarproject.org/download-briar-mailbox/), [maintenance notice](https://briarproject.org/news/2026-maintenance-mode/) | Briar is valuable for blackout and local-first patterns, but not for an open anonymous service inbox. Maintenance status also makes it evidence, not a presumed dependency. |
| Cwtch | Profiles expose Tor-native addresses; direct chat needs simultaneous presence, while untrusted-server storage and groups remain experimental. Its risk model explicitly considers relationship, location, timing, and pattern-of-life metadata. [Profiles](https://docs.cwtch.im/docs/profiles/introduction/), [servers](https://docs.cwtch.im/docs/servers/introduction/), [risk model](https://docs.cwtch.im/security/risk/) | Its metadata vocabulary and application overlays are useful. Binding a durable profile to a durable onion endpoint is not Ardents' desired separation of Persona, service, route, and delivery identity. |
| SecureDrop | A service-specific bearer codename lets a source return for asynchronous replies without creating a public person identity. Its threat model documents endpoint, bearer-secret, server-memory, file, and operational limits. | This is the strongest proven first-contact workflow, but not a generic decentralized application network and not a safety level Ardents may claim without equivalent evidence and operating discipline. |

### Composed alternatives, not only individual products

The differentiation criterion requires comparison with realistic compositions.
The following are desk-research baselines, not measured setup or usability
results.

| Composition | What it can already provide | Residual cost or gap inferred from the cited components |
|---|---|---|
| Tor onion site + SimpleX Contact Address | Hidden publisher location, a site, pairwise queues, message requests, and offline text delivery. | The visitor handles an opaque onion address plus a second product address and trust flow. The publisher still operates a durable origin and a service/bot client; signed replicated release continuity and one recoverable human Service Name are absent. [Tor setup](https://community.torproject.org/onion-services/setup/), [SimpleX business](https://simplex.chat/docs/business.html) |
| OnionShare Website + SimpleX | Low-friction self-hosted site plus a mature asynchronous reply channel. | It is a credible small-team workaround. The site host must remain online, discovery uses an opaque address, updates and reply identity are separate, and availability does not survive loss of the publisher host. [OnionShare features](https://docs.onionshare.org/2.6.3/en/features.html), [SimpleX connections](https://simplex.chat/docs/guide/making-connections.html) |
| IPFS/IPNS + Tor access + SimpleX | Signed mutable content, third-party pinning, location-hiding access when correctly placed behind Tor, and pairwise messaging. | It introduces separate naming, pinning, gateway, Tor, and messaging control roots. Public IPFS discovery/access metadata is not private by default, and comparable end-to-end safety depends on a deployment that has not been measured here. [IPNS](https://docs.ipfs.tech/concepts/ipns/), [IPFS privacy](https://docs.ipfs.tech/concepts/privacy-and-encryption/), [SimpleX privacy](https://simplex.chat/docs/guide/privacy-security.html) |
| Hyphanet Freesite + social plugin | Origin-independent cached signed publishing plus email/forum-style applications inside one anonymous network. | It is the closest architecture-level counterexample. Long keys, high first-load latency, probabilistic retention, no deletion, and separate plugin UX remain; whether users prefer Ardents' proposed integration is untested. [Hyphanet documentation](https://www.hyphanet.org/pages/documentation.html), [operational help](https://www.hyphanet.org/pages/help.html) |
| SecureDrop | One coherent protected page, asynchronous submission, return secret, and reply workflow. | It solves a narrower high-risk job with much stronger operating discipline. Ardents' proposed gain is generic signed applications, human naming, independent replication, and lower origin burden—not automatically equivalent source safety. [SecureDrop overview](https://docs.securedrop.org/en/latest/index.html), [threat model](https://docs.securedrop.org/en/stable/threat_model/threat_model.html) |

- **Inference:** The full Ardents contract remains differentiated on paper, but
  users may rationally prefer one of these compositions because its components
  are mature and its failure modes are understood.
- **Measurement gap:** No participant has compared the alternatives, and no
  operator has completed matched setup/recovery tasks. Differentiation therefore
  remains `weak`, not `adequate`, at this gate.

### The actual differentiation

- **Inference:** Within the systems sampled, the unoccupied product contract is
  the complete journey below, not any single primitive:

  ```text
  recoverable human Service Name
  + authenticated immutable Site Bundle and signed updates
  + availability without a permanent publisher origin
  + protected service-scoped first contact
  + pairwise asynchronous reply without a universal visitor identity
  ```

- **Inference:** `Site-only` is a weak architecture tracer. It can demonstrate
  names, publication, retrieval, and replication, but it does not demonstrate a
  private application relationship or force delivery-identity separation.
- **Inference:** `Mailbox-only` has a clear job but enters a mature messenger and
  secure-submission field. Without the Site, the user still needs an external
  context and discovery surface, usually an opaque address or ordinary website.
- **Inference:** The Site gives the visitor authenticated context for the
  request; the reply path turns a static publication into the smallest useful
  Private Service. This is why the combination has more research leverage than
  either half.
- **Inference:** The safe product term should not be the blanket adjective
  `anonymous`. The user-visible contract is better described as **Named Private
  Site + Protected Reply Thread**. Each protection claim must still name its
  adversary and limitation.

### Evaluated H1 contract — rejected by P1-D2

The desk-research recommendation before the Product Owner walkthrough was to
keep `Mailbox` as an internal bounded delivery primitive and expose two narrower
product concepts:

- **Service Inbox:** a Private Service's capability to accept an initial
  text-only request under an explicit local admission and retention policy;
- **Reply Thread:** a service-scoped pairwise channel created by that request,
  with no identifier intended for reuse at another Service.

The observable tracer is:

> **Assumed publisher job:** Publish one stable, verifiable private page and
> receive and answer text inquiries without maintaining a public origin or
> requiring visitors to create accounts.

This is a falsifiable JTBD formulation, not yet an observed user statement.

1. A Developer obtains a recoverable Service Name and publishes a signed,
   immutable static Site Bundle to multiple independent Replicas.
2. A Person opens and verifies the Site without either endpoint learning the
   other's network location under the declared Interactive Route claim.
3. The Client, not unconstrained site code, displays the Inbox policy and asks
   for explicit permission to create one Reply Thread.
4. The Person sends a text message using fresh service-scoped state through the
   required Shielded Route contract and may go offline. The Developer may also
   be offline at send time. Until R-001 and R-005 resolve that contract, an
   interaction prototype may simulate its delay and warning but may not claim
   its network protection.
5. Bounded Replicas retain encrypted envelopes. The Developer later retrieves
   the request, replies, and the Person later polls and receives the reply.
6. A Site release is updated without changing its name or existing threads.
   The journey survives one unavailable Replica and one ordinary blocked relay.

The tracer explicitly excludes attachments, arbitrary stateful service code,
person-to-person discovery, contacts, groups, calls, presence, read receipts,
third-party mobile push, a public directory, payments, and clearnet exit. It is
not a general messenger and is not presented as a SecureDrop replacement.

For the first bounded interaction test, use text of at most 8 KiB, one
outstanding initial request per single-use Invite, seven-day simulated
retention, polling rather than third-party push, and the existing one-Replica /
one-relay failure case. These are deliberately provisional experiment budgets,
not V1 protocol decisions. A separate prototype variant may show a public Inbox
with quarantine, but open Sybil-resistant admission remains R-010.

### Evaluated H1 safety envelope — not a product commitment

These are candidate claim envelopes for later falsification, not accepted
security claims.

| Candidate | Information | Adversary | Required conditions | Falsification / measurement | Honest limitation |
|---|---|---|---|---|---|
| Text content | Message confidentiality, integrity, and service authenticity | Carrier and storage Nodes, including colluding ordinary Nodes without endpoint keys | Authenticated Service release/key binding, fresh thread keys, uncompromised endpoints, no key export to site code | Capture every envelope at each role; attempt cross-thread key use, tampering, replay, rollback, and malicious-Replica substitution | Publisher and visitor read plaintext; their Devices and the text itself can disclose the author |
| Endpoint location | Visitor IP from the Service and publisher IP from the visitor; both from one ordinary Node | Opposite endpoint and any one relay or Replica | Accepted Route Profile, required non-colluding path diversity, no direct bundle networking, uncompromised endpoint | Run controlled malicious endpoints and each intermediary position; inspect packet/log views and force route/Replica failures | A Local Traffic Observer may classify Ardents use; Broad Traffic Observer correlation is not an Interactive Route claim, and multi-party collusion remains open in R-001 |
| Cross-Service relationship | Whether two Reply Threads belong to one Person | Two colluding Services and any one ordinary relay or Replica | Separate keys, capabilities, storage identifiers, Persona/application state, and standardized Client behavior per Service | Inventory all identifiers and run paired two-Service correlation tests with controlled timing/content | Text, behavior, timing, fingerprinting, or a compromised Device can still link the Person |
| Availability | Site release and retained text remain retrievable | One unavailable ordinary Replica and one blocked ordinary relay | Sufficient independently controlled Replicas and an alternate accepted path | Remove each selected Replica/relay before and during publish, retrieve, send, poll, update, and rollback | Correlated operator loss, censorship, retention expiry, or loss of all replicas can still make data unavailable |

- **Blocked claim:** relationship unlinkability against a Broad Traffic Observer
  is not an Interactive Route promise. R-001 must still name collusion and
  observation conditions; R-005 must justify any stronger Route Profile with a
  delay, cover, bandwidth, and anonymity-set budget and measurement.
- **Recommendation — untrusted input:** accept text only. File submissions add
  malware, parser, metadata, and safe-opening workflows that the first tracer
  cannot honestly contain. SecureDrop's procedures and OnionShare's warning
  show that transport encryption alone does not remove this endpoint risk. See
  [SecureDrop submission handling](https://docs.securedrop.org/en/latest/journalist/submissions.html)
  and [OnionShare receive warning](https://docs.onionshare.org/2.6.3/en/features.html).

### Maintenance, license, and adoption boundary

- **Inference:** The compared products have materially different operating
  and governance models: publisher-owned Tor services, local address books,
  public DHT/pinning providers, token-staked Session nodes, generic SimpleX
  relays, user-hosted Briar/Cwtch mailboxes, and the specialized SecureDrop
  deployment.
- **Inference:** No comparison product is selected as a dependency by R-017.
  Therefore source-code license compatibility, maintained implementation
  quality, audit history, and integration risk remain mandatory R-013 evidence;
  feature similarity is not permission to reuse code or inherit a security
  claim.
- **Assumption:** A Developer will accept the cost of acquiring a name,
  selecting Replicas, and managing recovery if this replaces an always-online
  hidden origin. R-016 may eventually compare this with the actual burden of
  OnionShare, SecureDrop, IPFS, Tor, and ordinary hosting.

## Options

Scores are qualitative applications of the pre-registered criteria, not
measurements.

| Criterion | H1: narrowed Site + Reply | H2: Site only | H3: Mailbox only |
|---|---|---|---|
| Coherent user job | adequate | strong | strong |
| Differentiation | weak | weak | weak |
| Vision coverage | strong | weak | weak |
| Vertical completeness | adequate | weak | weak |
| Bounded scope | adequate | strong | adequate |
| Hostile-environment value | strong | adequate | strong |
| Honest safety | adequate | strong | adequate |
| Adoption path | adequate | adequate | weak |
| Research leverage | strong | weak | adequate |
| Falsifiability | strong | strong | strong |

### H1 — Named Private Site + Protected Reply Thread

- Product fit: one `publish → find → verify → understand → request → leave →
  receive reply → update` journey for both Person and Developer.
- Adoption path: one publisher creates a complete read-and-reply experience;
  each visitor receives value without first publishing, creating a public
  profile, or recruiting a contact graph. The Site provides the context that a
  bare Mailbox would otherwise need from a second product.
- Security fit: forces separate interactive and asynchronous claims, per-Service
  state, application isolation, admission, retention, and failure handling.
- Operational dependency: naming, at least two meaningful Replica failure
  domains, client polling, and publisher-side inbox processing.
- Governance roots: naming, client releases, service releases, bootstrap, and
  Replica selection remain explicit and separate.
- Implementation risk: adequate only under the narrowed text/static contract;
  a general runtime or messenger would make it fail bounded scope.

### H2 — Site only

- Product fit: useful censorship-resistant publication and client-side tools.
- Limitation: close alternatives already exist and it does not by itself prove a
  generic application transport or stateful Private Service.
- Product Owner decision: select it as the smallest architecture tracer. Generic
  Application Data belongs to R-019; messenger semantics belong to applications.

### H3 — Mailbox only

- Product fit: anonymous inquiry, support, or submission is a real urgent job.
- Reason to reject as the primary tracer: it needs an external context and
  discovery surface, duplicates mature messaging/submission products, and does
  not constrain site publishing or the private application boundary.
- Appropriate use: a delivery-state experiment inside H1 may be mailbox-only.

### H0 — reject all three

H0 is not selected for architecture research. H2 is accepted as the smallest
tracer, while market differentiation and first-user demand remain unvalidated.

## Recommendation

**Recommendation:** choose H2 as **Named Unlisted Site**, the smallest accepted
architecture tracer. Reject a built-in Service Inbox and Reply Thread.

Ardents transports and protects application-defined data but does not decide
that the data is a chat message or impose conversations, contacts, history,
attachments, presence, or messenger recovery on every application. A developer
may build a messenger on the accepted transport contract.

Transport alone is not a complete messenger: addressing, application-level
end-to-end sessions, offline delivery, ordering, history sync, multi-device
state, abuse handling, and notifications still need explicit ownership. R-019
must decide which of those are generic network responsibilities and which remain
inside the application.

Confidence is **high** in the Product Owner scope decision and **low** in market
differentiation or first-user validation. The strongest counterargument is that
a site-only tracer exercises less of the eventual application network. R-019
addresses that gap without smuggling one application's UX into the core.

### R-017-P1 under the current working model

#### P1-D1 — Unlisted discovery by exact Service Name

**Product Owner decision, accepted 2026-08-06:** the first tracer Service is not
indexed or offered through a network directory, but any Person who already knows
its exact Service Name may resolve and open it. An Invite is not required merely
to open the Service.

This fixes only the product discovery boundary:

- Ardents supplies no browse, recommendation, search, or global service list;
- the Developer may share the exact Service Name through any chosen context;
- knowing a Service Name is not authentication, authorization, or a promise that
  the name cannot be guessed or leaked;
- a Service may later add a separate capability gate;
- access to any application-defined data operation is separate from knowing the
  Service Name and requires its own accepted Capability contract;
- private resolution, resistance to namespace enumeration, registration, and
  recovery remain R-003 research rather than assumed properties.

The canonical term is **Unlisted Service**. The shared glossary and product
journey were updated. No ADR is warranted: this is an explicit, reversible
product-scope decision rather than a hard-to-reverse architecture choice.

#### P1-D2 — No built-in Inbox or messenger semantics

**Product Owner decision, accepted 2026-08-06:** a Service Inbox and Reply Thread
are unnecessary Ardents product features. Applications may build a messenger or
other interaction model on generic protected transport.

Consequences:

- H1 and H3 are rejected as first-tracer product contracts; H2 is selected;
- the first tracer is **Named Unlisted Site**;
- the Ardents Client does not own conversations, Message Requests, chat history,
  attachments, presence, or a universal contact UX;
- the core handles opaque **Application Data**, not built-in messages;
- infrastructure Node IDs are not silently promoted into Person or application
  identities;
- whether Ardents supplies live streams, datagrams, offline store-and-forward,
  ordering, retries, or retained delivery is deliberately unresolved by this
  decision and becomes R-019.

No ADR is warranted yet. Removing product-specific messaging is easy to reverse
before implementation, while the eventual transport boundary may require an ADR
after R-019 evidence.

External validation remains future R-016/R-018 work. It is required before
calling the tracer a validated V1 or claiming that unfamiliar people understand
its security model, but it is not invented as a current resource.

## Disposition

- State: `decided` for architecture-tracer scope.
- H2 selected: **Named Unlisted Site**.
- H1 and H3 rejected for the tracer; applications may still implement their
  capabilities over generic Application Data transport.
- P1-D1 accepted: the tracer uses an Unlisted Service openable by exact Service
  Name without a directory or opening Invite.
- P1-D2 accepted: no built-in Inbox, Reply Thread, or messenger semantics.
- `Mailbox` was removed from the shared glossary; `Application Data` is the
  neutral product term.
- R-016 and external R-018 validation remain open future gates; they are not
  assumed active under the current one-to-one working model.
- R-019 was initially named as the next question; the later network-product
  clarification below rejects and decomposes it before any transport API or
  offline-delivery primitive is accepted.
- No ADR; the decision is reversible before implementation.
- No implementation or experiment code yet.

## Network-product clarification — 2026-08-06

The later network functional-map review narrows what R-017 actually selected.
**Named Unlisted Site is a Reference Application, not a storage or application
runtime architecture.** Its baseline form is an ordinary local HTTP service
exposed through a generic live Service Connection.

The earlier Site Bundle, Replica, permission, and sandbox assumptions in this
record are retained as historical evaluated hypotheses. They are not accepted
network-core requirements. Replicated publishing or a bundled runtime may
return only as separately justified Overlay Services.

R-019 was rejected as an imprecise combined question. Its address, live
transport, routing, failure, and local-boundary decisions are now separated into
R-006, R-002, R-001, R-007, and R-008. This clarification does not reopen the
accepted Unlisted Service rule or the rejection of built-in Inbox and messenger
semantics.
