# Reference systems

Last source review: 2026-08-06

Reference systems are complementary. Ardents does not need one winner that
provides naming, identity, real-time routing, metadata protection, storage,
groups, governance, and application execution.

| System | Useful product or protocol patterns | Risks to avoid copying |
|---|---|---|
| I2P | Internal application-network boundary; router identity separate from application destination; independently selected inbound/outbound paths; application-facing APIs | Router-first UX, small anonymity set, stable discovery observation points, and assuming a distributed DHT is Sybil-safe |
| Tor | Guards, path isolation, rendezvous, bridges, pluggable transports, mature response to malicious infrastructure | Treating low-latency onion routing as protection from global timing correlation; hiding semi-trusted directory control roots |
| Nym | Mixing, fixed envelopes, randomized delay, cover traffic, anonymous replies, unlinkable admission credentials | Treating a privacy carrier as a complete application platform; making a token or high-latency mix path mandatory for every action |
| Session | No phone/email onboarding, Message Requests, replicated offline delivery, explicit retention, recovery journey | One long-term key spanning identity/encryption/recovery; hidden metadata compromises; centralizing large groups or attachments without clear product boundaries |
| ENS | Registry/registrar/resolver separation, hierarchical namespaces, delegation, TTL, custom records, forward-verified reverse names | Mandatory public chain/wallet linkage, public private-name records, transferable human identity, unresolved root governance |
| Human Passport | Optional contextual Credentials and selective proofs for local admission | Universal humanity score, KYC dependency, or credential graph used as network identity |
| Gitcoin | Mechanism plurality, transparent review, appeals, and public-goods funding | One token or voting mechanism controlling every kind of decision |
| Legacy Ardents | Root/Device separation, Credential distinct from authority, scoped Capabilities, transport identity separation | Stable global Principal, Node-centric enrollment, single Realm Authority, and architecture inherited without current evidence |

## Primary starting sources

### Tor

- [Protocol introduction](https://spec.torproject.org/intro/index.html)
- [Onion-service rendezvous overview](https://spec.torproject.org/rend-spec/protocol-overview.html)
- [Guard specification](https://spec.torproject.org/guard-spec/)
- [Stream isolation](https://spec.torproject.org/path-spec/stream-isolation.html)
- [Remaining attacks](https://support.torproject.org/about-tor/security/attacks-on-onion-routing/)
- [Circumvention](https://support.torproject.org/tor-browser/circumvention/unblocking-tor/)

### I2P

- [Technical introduction](https://i2p.net/en/docs/overview/intro/)
- [Tunnel routing](https://i2p.net/en/docs/overview/tunnel-routing/)
- [Network database](https://i2p.net/en/docs/overview/network-database/)
- [Naming](https://i2p.net/en/docs/overview/naming/)
- [Threat model](https://i2p.net/en/docs/overview/threat-model/)
- [I2CP overview](https://i2p.net/en/docs/specs/i2cp-overview/)

### Nym and Session

- [Nym mixnet](https://nym.com/mixnet)
- [Nym whitepaper](https://www.nym.com/nym-whitepaper.pdf)
- [Nym zk-nyms](https://nym.com/zk-nyms)
- [Session protocol documentation](https://docs.getsession.org/session-network/session-protocol)
- [Session swarms](https://docs.getsession.org/session-network/session-nodes/swarms)
- [Session account restoration](https://docs.getsession.org/session-network/session-protocol/account-restoration)
- [Session Protocol V2](https://getsession.org/session-protocol-v2)

### Naming, identity, and governance

- [ENS protocol](https://docs.ens.domains/learn/protocol/)
- [ENS registry](https://docs.ens.domains/registry/ens/)
- [ENS resolution](https://docs.ens.domains/resolution/)
- [Human Passport key terms](https://docs.passport.human.tech/overview/key-terms)
- [Human Passport privacy](https://passport.human.tech/privacy)
- [Gitcoin mechanism plurality](https://gitcoin.co/research/plural-funding-mechanisms)

Each research record must verify that a source is still current and must add the
specific paper, specification revision, code commit, audit, or advisory actually
used. This page is orientation, not evidence sufficient for a decision.
