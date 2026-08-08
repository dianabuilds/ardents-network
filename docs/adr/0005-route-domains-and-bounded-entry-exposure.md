---
status: accepted
date: 2026-08-08
---

# Separate hidden Route legs by domain and bound Entry exposure

Initiator Carrier, Rendezvous, Responder Carrier, and Introduction positions use
stable disjoint Role Domains. One Node Identity and one honestly declared
operator family occupy only one domain for at least an Entry lifetime. This
makes the five-distinct-position rule enforceable across independently hidden
endpoint legs without asking a Service to reveal its Entry Set through repeated
Rendezvous rejection. It splits network capacity and still cannot detect a lying
or Sybil operator, but the alternatives either weaken the accepted one-Node
claim or require a novel private-set-intersection protocol and rejection-oracle
defense.

Domain assignment is finite and non-overlapping over time. New Entry,
Introduction, Resolution, or other duty is eligible only when its maximum
terminal lifetime fits before assignment `not-after`. Reassignment publishes
stop-new-work, drains and quarantines the identity and known family until every
old-domain duty terminates, then permits new-domain eligibility. Emergency may
terminate work and reduce availability but cannot overlap old and new domains.

Destination-aware Name/Target/descriptor lookup and publication is a subrole
restricted to the non-adjacent Rendezvous Domain, not a fifth global domain. An
endpoint excludes every resolution identity and known family used for one exact
destination/context from that connection's Rendezvous. The private resolution
path also hides its query from Entry and bounds retries. Public capacity and
concentration gates apply both to resolution-capable Rendezvous families and the
remaining Rendezvous reserve after exclusion. This prevents one valid
identity/family from being both endpoint-adjacent and destination-aware in the
same operation without adding a fifth Route Domain. It does increase the real
supply floor: beta/stable must retain three/five effective families after the
maximum local exclusion union. `12`/`20` are only theoretical pre-exclusion
four-domain counts; with `x_d` maximum excluded families in domain `d`, actual
route-family floors are at least `Σ_d(3+x_d)`/`Σ_d(5+x_d)`, subject to capacity.

A globally advertised direct-origin bootstrap/materialization/time/update source
identity and known family cannot also receive Route or Destination Resolution
eligibility in the same assignment. If an ordinary candidate serves bytes
directly, the contacting Endpoint retains it in a bounded installation-wide
Direct Source Exposure Set and locally excludes it until all derived state/work
expires. Source selection cannot overlap retained endpoint-adjacent/prepared
state or live work; retries and set growth are finite, and exhaustion is explicit
unavailability. Every mandatory pre-Route artifact class has at least three beta
or five stable effective authenticated source-only families with the same
`40%`/`25%` concentration caps. The same source families may serve several
classes and count once; unauthenticated external distribution does not count.
These families are additional to Route-family arithmetic, making `15`/`25` the
all-zero-exclusion theoretical infrastructure floors before capacity effects.

V1 has ordinary and Bridge entry regimes and at most one small long-lived Entry
Set for each activated adjacent Role Domain × regime per installation. Initiator,
Responder, and Service Introduction exposure stays domain-separated even when
client and Publisher are co-resident. Applications, Services, Targets, Instance
generations, Isolation Contexts, destinations, and Bridge Invites create no new
sets. The Direct Source Exposure Set is also installation-wide and cannot be
reset by a context. Every Bridge key is eligible for one domain and its Invite carries or
references epoch-bound proof. Contexts share no channels, keys, Interiors,
destinations, sessions, or recovery state. An Entry already sees and may link
the common endpoint origin; Ardents does not claim to hide same-device contexts
from that accepted view.

The one-Node claim covers role-local protocol knowledge only. A Node operator
that also controls/observes an endpoint or active probe source may confirm a
known Target through low-latency timing/volume. That is an explicit non-claim,
not something Role Domains solve.
