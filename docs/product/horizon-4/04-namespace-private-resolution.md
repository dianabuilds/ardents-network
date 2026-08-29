# H4-4 — Service names and private resolution

Status: **the signed-corpus/private-resolution alpha slice is closed. H4-4B
canonical-name lifecycle and H4-4C root-claim simulations retain
project-control mechanics evidence only; they do not create a public Namespace
or shared governance claim. Participant Browser Entry and public Namespace
remain deferred to later selected work.**

## Decision

H4-4 promotes the explicit [Target Link](../../../CONTEXT.md) path proved in
H4-3 into a stable human-readable **Service Name** and its shareable **Service
Link**, for example `ardents://blog.alice`. In product terms this is Ardents'
DNS-like naming layer: the same canonical name resolves to one authenticated
Service Target for every honest compatible Endpoint.

It is not the public DNS protocol or infrastructure, a publicly delegated DNS
zone, a Web PKI name, a search directory, a registrar, or an application
identity provider. In particular, an ordinary browser must not be taught that
`blog.alice` is an Internet hostname, and H4-4 does not authorize a global DNS
override, a public CA, or a destination fallback.

The epic has two deliberately distinct outcomes:

1. **Named alpha:** a bounded, pre-provisioned Service Name resolves through
   the selected private-resolution profile and can be opened through H4-3's
   explicit local Browser Adapter path. This proves the user value of names;
   it is not a public or permissionless Namespace.
2. **Canonical Namespace:** a person may claim, lease, update, release, and,
   where precommitted, recover a name without discretionary registrar action.
   Every compatible Endpoint verifies one current binding or fails explicitly.

The first may make the usable-network alpha friendlier. The second carries
public expectations and cannot be declared complete until H4-6 supplies the
shared authenticated control/close operation it requires.

**Current research outcome:** the Product Owner selected the clearly separate,
bounded alpha-label overlay for H4-4A. Its only shareable name form is an
[Alpha Service Link](../../../CONTEXT.md), `ardents-alpha://<name>`, backed by
one finite signed [Alpha Name Corpus](../../../CONTEXT.md). It never enters the
maintained canonical Namespace verifier or uses `ardents://<name>` as a public
claim. A provisional Firefox-first Browser Entry profile hands its exact
`http://<name>.ard/` route to the selected add-on/native-host path through an
Endpoint-owned loopback proxy. Its source-level release binding and per-user
native-manifest lifecycle are selected by
[ADR-0045](../../adr/0045-firefox-first-unlisted-browser-entry-delivery.md),
and Mozilla has returned one concrete signed XPI for the fixed 0.1.0 add-on.
That exact XPI has passed an explicit clean Firefox Release install-and-route
qualification. No enrolled participant release exists yet.
Neither form supplies
`https://*.ard`, DNS, a public certificate, or a trust-store change. See
[R-097](../../research/records/r-097-named-alpha-private-resolution.md).

### Browser-visible address gate

The Product Owner does **not** accept an explicit loopback origin as a
participant address. That former tracer is not an alternative alpha journey.

The selected alpha address presentation is Firefox-only
`http://<name>.ard/…`, without a visible `localhost` label or ephemeral port.
It uses one Mozilla-signed unlisted fixed-ID XPI, explicitly installed by the
participant, and a release-bound per-user native manifest; it does not
configure ordinary DNS/DoH, a global browser proxy, a trust store, or a browser
binary. A clean Firefox 154 resolver trace nevertheless made native resolver
calls for `.ard` names before its HTTP proxy route. The profile is therefore
not a participant Browser Entry and makes no no-DNS/DoH claim.
The Endpoint itself selects the same fixed Browser Entry state profile. Its
participant claim remains unqualified until a concrete signed XPI, enrolled
release, Windows/Ubuntu install/replace/remove evidence, and failure behavior
exist. HTTPS/Web PKI, public DNS, non-Firefox behavior, collision policy, and
the canonical Namespace remain separate decisions.

**Provisional implementation trace (not a promotion):** the maintained alpha
Endpoint can now construct `http://<name>.ard/` through one loopback-only
proxy. In the direct compatibility path, a route is registered only after it
authenticates that exact binding. In the named path, an explicit
`AlphaBrowserResolution` composition keeps that proxy live with no route,
then resolves a browser-requested `.ard` host only from the Endpoint-local
accepted corpus floor and waits for the exact opened Service presentation to
register it. Concurrent requests for one name share that one opening. The
proxy accepts plain HTTP for a registered name alone, rejects all other names
and `CONNECT`, and withdraws a route with its Connection. When explicitly
configured with a per-user Browser Entry state path, it additionally publishes a fresh local proxy port,
probe capability, and separate one-process proxy credential. The maintained
Firefox native host reproves the live proxy before returning either its port or
the credential for a `407` challenge; the fixed-ID extension compares that
freshly proved port to the loopback challenger and answers once. Its
`webRequest` permissions remain limited to `.ard`, and it otherwise chooses a
dead loopback proxy only for an `.ard` request when the local Endpoint route is
unavailable. It registers no listener for other names, which retain the
browser's existing networking behavior. The maintained Windows qualification
starts from `http://reference.ard/` and proves that this request, rather than a
pre-opened route, invokes the accepted-corpus demand path alongside an
ordinary-URL fallback control. The source now has a normal Endpoint
`BrowserEntryProfile: "firefox-alpha"`, an enrollment-v4 binding of Endpoint,
native host, and fixed-ID XPI, and an owned-only per-user
install/replace/remove implementation. Firefox still has to validate the
concrete Mozilla-signed XPI when the participant explicitly installs it. The
fixed 0.1.0 artifact has passed one clean-profile Firefox Release
qualification with its dynamic C-2 HTTP flow, but no two-platform
release-shaped qualification exists yet.
The Firefox resolver result rejects its HTTP-proxy mechanism for the required
DNS/DoH property; an HTTPS trust model also remains unselected. A separate Windows
C-2 qualification now passes one dynamic HTTP/1.1 Publisher flow through that
trace—ordinary document/form POST, redirect/cookies, chunked response, and
close—with no content rewriting. It is evidence for the selected H4-3B shape
only, not an arbitrary-web-application guarantee.

### Browser Entry and application-transport dependency

Name presentation and generic Publisher-Service traffic are distinct gates.
The direct alpha-proxy compatibility path can serve only a route that the
Endpoint has already authenticated and opened for one live C-2 Service
Connection. The separate `AlphaBrowserResolution` composition now supplies
the H4-4 named path: a typed `.ard` host is parsed into its exact alpha link,
resolved from the accepted floor, and delegated to one caller-owned C-2
opening of that verified binding. It never gives the proxy, extension, or
native host a Target-selection fallback. The newer payload-neutral HTTP/1.1
presentation may carry the selected dynamic Publisher application.

The selected H4-4 runtime contract is `name.ard` typed in the browser -> the
Browser Entry passes that exact `.ard` name to the local Endpoint -> private
resolution verifies its current binding -> the Endpoint opens that exact
Service Target -> the local bridge carries the Service's HTTP bytes. The
extension is the browser-facing resolver hook, not a public DNS server and not
a content adapter. H4-3B's bridge, not a CMS adapter, provides the generic
Publisher-Service path.

`OpenAlphaBrowserRuntime` is the maintained composition boundary for that
contract. For each demanded name it accepts only an opened State Resolution
view and an already-imported Entry owner: State supplies the one assigned
Destination Resolution Gateway identity/family and its signed OHTTP profile,
as well as the exact C-2 candidates; Entry supplies the current Initiator
contact. Endpoint verifies the profile and Descriptor, obtains the Target only
from the accepted alpha floor, and opens the resulting Service. When a
Descriptor carries a signed Introduction Transit Grant, the runtime validates
it against current State and uses its exact attachment; it never substitutes a
fresh attachment under that Grant. Its integration test proves the complete
`reference.ard -> State-selected private lookup -> C-2 -> Publisher HTTP`
path. `ardents endpoint alpha-browser` now owns and retains the already
accepted State root, imported Entry root, and alpha-corpus floor while that
runtime is live. Its closed local input has roots, pinned verification keys,
and local broker identities only: it rejects a Target, Descriptor, Gateway
profile, C-2 peer, TLS credential, or URL. It publishes the fixed Browser
Entry state and withdraws it on stop. When its closed plan names the existing
bounded State Source plan, the runtime takes the root's initial source wave
and automatic refresh itself; a second `refresh-sources` process must not
contend for that root lease. The retained State may still become stale and
then a demand fails locally; this command does not add OS DNS, an
upstream proxy, a caller-supplied Target, or a new route planner.

ADR-0047 selects R-118's versioned membership-level dynamic Introduction
submission for the project-operated alpha. A Descriptor declares that a fresh
State/Entry-issued Grant is needed rather than embedding a fixed Grant; a
closed Credential Relay can obtain it without carrying the name, Target,
Descriptor, or Publisher material. The maintained composition now has a
Descriptor-v2 behavior test that creates a fresh Endpoint TLS key and
attachment, passes one opaque request through a separately admitted
Credential Relay to the State-selected issuer, verifies the exact returned
Grant, and opens the selected Introduction and Publisher HTTP flow. The relay
commits the exact State profile digest, the Initiator pins the issuer TLS key,
and the issuer requires the selected Initiator TLS Node key and rechecks its
current State duty before signing. Profile/issuer substitution therefore fails
closed; v1 retains only its embedded fixed-Grant path. The short Descriptor
slot window also bounds that grant exchange and the matching live Introduction
slot.

This is still a project-operated composition trace, not an independently
operable participant lifecycle: a durable cross-restart issuance budget,
withdrawal/rotation operation, retained-key crash treatment, live Entry
operator wiring, and release-shaped State/Entry owner are unresolved. It
makes no hostile-peer or Initiator/issuer-collusion claim; that control problem
remains H4-6 research.

The final named Browser Entry therefore requires the selected R-115/ADR-0045
Firefox address/install model **and** H4-3B's application-transparent,
bidirectional Publisher-Service connection. H4-4 must not add a CMS adapter,
content rewriter, static exporter, or special browser protocol as a substitute
for H4-3B. Until both gates are qualified, a name may open only an explicitly
requested, already-authenticated bounded alpha Service session; an unknown,
withdrawn, or no-longer-open route fails locally.

## The name-to-site loop

```text
Publisher: local HTTP Service
  -> Publisher Endpoint publishes Service Target
  -> Name Authority binds `blog.alice` to that Target
  -> authenticated Namespace materializes the current binding
  -> private resolution returns a proof for the exact name
  -> User Endpoint verifies the binding and opens the exact Target
  -> local Browser Adapter passes the Service's HTTP bytes to the existing browser
```

The Browser Adapter remains an H4-3 compatibility boundary. It receives an
already explicit Ardents destination and may present a local, scoped way to
open its Service Link. Name resolution has no content semantics: it binds a
Service Target, not a static-site profile or a particular application. It does
not make the browser's ordinary hostname lookup, certificate validation,
external-resource loading, or Internet traffic part of Ardents. Those browser
trust and isolation questions remain H4-3/H4-7 work.

## Product contract

### What a Service Name means

- A Service Name is canonical lowercase ASCII, dot-hierarchical text in the
  one Ardents Namespace. It identifies a mutable, authenticated binding, not
  an IP address, Node, Person, account, or search keyword.
- A **Name Authority** controls that binding independently of the Service
  Authority. A normal Publisher does not need Name Authority access to keep an
  already-published Service running.
- The result is a **Destination Binding**: name generation, revision, exact
  Service Target, and finite validity. A changed binding can stop or require a
  new connection; it never retargets a live connection silently.
- A Name Lease is finite. It can be Active, Grace, recovery-pending, Released,
  unavailable, or otherwise explicitly invalid under the selected rules. It is
  not permanent property or proof of a person's identity.

### What resolution protects—and does not

Private Resolution must prevent one ordinary resolution Node from learning
both the querying Endpoint's ordinary location and the exact Service Name. It
does not make predictable names secret, prevent a broad observer correlating
traffic, hide the Service from someone who knows its name, or turn a generic
browser adapter into a protected browsing environment.

The resolver proves a current binding under the authenticated Namespace state;
it does not choose a destination. Missing, stale, conflicting, invalid, or
incomplete proof is a classified failure. It never becomes an ordinary DNS
query, HTTP request, local alias, another namespace, or Target Link.

## Delivery slices

### H4-4A — named Service alpha

**Goal:** prove that names improve the H4-3 browser journey without pretending
that a project-selected list is a public Namespace.

The Product Owner selects a small, pre-provisioned corpus with explicit owners,
expiry, and withdrawal procedure. A User receives a complete Alpha Service Link,
opens it through the scoped local Adapter, and sees one of: verified current
binding, explicitly unavailable name, expired name, or resolution failure. A
Target Link remains available as the fully supported escape path, but the
software never converts one failed form into the other.

**Done when:** two independent alpha Endpoints resolve the same exact
pre-provisioned name through the selected private-resolution profile, reject a
stale/revoked/conflicting proof, and reach the Target that the verified binding
names. The participant-facing UI explains that the alpha name set is bounded
and is not a public registration service.

### Alpha corpus input and replacement

An H4-4A participant does not trust a download location as a name authority.
The fresh v3-or-later enrolled bundle carries the independently pinned `corpus.pub`
companion. A participant obtains the explicitly published `catalog.ac2` and
`corpus.anc` bytes by the current [closed-alpha corpus
procedure](../closed-alpha-name-corpus.md), then invokes:

```text
ardents-control accept-alpha-corpus \
  --enrollment <alpha-enrollment.json> --artifact <enrolled-artifact> \
  --control-state-root <inspection-root> --corpus-state-root <endpoint-corpus-root> \
  --catalog <catalog.ac2> --corpus <corpus.anc> --at <RFC3339-time>
```

The command first authenticates the exact enrollment pin, enrolled Endpoint
executable, and the exact manifest-pinned command that is running,
and accepted Release/Network/Compatibility control, then checks ACA2 against
the enrollment-pinned disclosure and corpus roots. Only after that does it
advance the separately named Endpoint-local corpus floor, which retains the
accepted corpus bytes. It neither downloads an artifact nor starts an Endpoint.
The invoked control command must itself be a manifest-verified regular file in
the same bundle; the operator never substitutes a separately downloaded binary.
A later replacement repeats the same explicit operation; the floor rejects a
lower serial or a conflicting corpus at the retained serial. The selected Linux
C-2 process tracer builds an exact enrolled v3 bundle, runs that real command
over ACA1/ACA2, then resolves the User's alpha link only from the resulting
floor. Its separate command process test accepts one higher serial, rejects a
subsequent attempt to restore the lower serial, and verifies that the successor
Target remains retained. None of this establishes a distribution source,
multi-host operation, or participant-ready browser journey.

**Stop condition:** if the alpha requires a hidden registry operator, manual
per-request approval, a resolver that sees both origin and name, browser-wide
network interception, or a fallback destination, stop rather than label it
Namespace work.

### H4-4B — lifecycle-correct canonical bindings

**Goal:** make a name's lifecycle correct before making it broadly available.

Exercise exact binding publication and update, lease expiry and Grace behavior,
release and reclaim as a new generation, parent validity where delegation is
selected, precommitted delayed recovery, Name Authority compromise, and
restart/replay/fork handling. The maintained Namespace and Resolution contracts
are technical inputs, including the current proof, admission, and record-size
limits; they are not yet a public capacity promise.

This slice proves a binding's safety properties on the selected live profile.
It does not yet let arbitrary people compete for a root name, because choosing
the winner is control-plane work rather than a resolver feature.

**Current implementation boundary:** [ADR-0043](../../adr/0043-derive-grace-from-signed-deadlines.md)
selects proof-local Grace. A V3 threshold-authenticated current-proof lineage
summary derives the Grace warning and availability from signed deadlines,
including an earlier parent boundary; an active Name Authority may renew until
that signed Grace end. It deliberately does not materialize an explicit
Released Record, choose a reclaim winner, or supply a global clock/control
claim. ADR-0057 and R-126 add the H4-4B project-control simulation: a durable
pending successor is installed only through a threshold-attested Epoch; it
exercises publication/update, Grace, Released refusal, next-generation reclaim,
restart, stale replay, forked-successor, and conflicting-current-state refusal.
This closes H4-4B in the
same Product Owner-and-Codex simulation scope as H4-6. It does not select
public Epoch operation, public governance, or a public Namespace claim.

### H4-4C — permissionless root claims

**Goal:** admit root claims under one public deterministic rule rather than a
registrar's discretion.

The accepted claim model commits in one Network Epoch and reveals in the next;
the authenticated close chooses the lowest eligible input ordinal for that
exact name. It has visible claim latency and fails closed on withholding,
incomplete evidence, rule conflict, or control fork. Anonymous admission work
is only a bounded local amplification guard—not Sybil resistance, fairness,
anti-squatting, payment, or personhood.

H4-4C cannot start from a local test corpus. It requires H4-6's selected,
operable, authenticated Epoch close/materialization owner plus its operator,
governance, failure, and capture limitations. No substitute "temporary
registrar" is silently permissionless.

ADR-0058 and R-127 close the H4-4C mechanics in the same Product Owner-and-
Codex project-control scope: two ADR-0019-admitted commitments are revealed,
the authenticated close selects the lowest ordinal, and only its derived
Record becomes threshold-current through `EpochInstallation`. Withholding,
incomplete evidence, incompatible rule, and control fork stop before current
state. The simulation publishes its synthetic `2-of-3` control/materialization
conditions, one-hour Active plus one-hour Grace lease, governance/capture
limitation, and abuse limitation. It is not a local corpus fallback and does
not select public Epoch operation, governance legitimacy, Sybil resistance,
anti-squatting, public Namespace availability, or permissionless public
naming.

### H4-4D — delegation and recovery, if selected

Delegation is optional, not an implied DNS feature. If selected, it must define
the parent/child authority and lease relationship, parent expiry, revocation,
proof size, and failure behavior. Recovery is likewise opt-in and
precommitted: a bounded distinct recovery set may authorize only the declared
delayed successor, never discretionary account recovery or proof of real-world
identity.

## Evidence and promotion gates

Before H4-4A, record the selected name syntax, Service Link handoff, exact
private-resolution roles, alpha corpus authority, terminal states, and browser
adapter boundary. Before H4-4B, demonstrate exact-name success and all
lifecycle terminal states; stale, replayed, equivocated, withheld, and
conflicting control; resolver/source role separation; restart; expiry; and
explicit no-fallback behavior.

Before H4-4C, demonstrate the complete claim-to-current path—not simply a
signed record—and publish the control authority, claim timing, materialization
conditions, governance/capture limitation, abuse assumptions, and exact
failure experience. That project-control mechanics gate is closed by R-127.
Permissionless public naming remains blocked until a separately selected public
operation/governance/availability evidence programme exists; R-127 does not
substitute for it.

## Non-goals

- Public DNS, a browser-recognized domain, `https://name` Web PKI, or an
  Ardents certificate authority.
- Search, discovery, recommendation, public profiles, accounts, social graph,
  trademark/dispute court, or concealed registrar discretion.
- Secret names, unguessable destination names, anonymous browsing, or an
  automatic location-privacy claim.
- Name-based authorization to the Service. Knowing a name is discovery only;
  the Application remains responsible for authorization.
- A new token, payment mechanism, staking, or economic anti-squatting claim.

## Current inputs and open decisions

The maintained technical input is [Naming and private resolution](../../technical/naming.md),
with the accepted ordering, recovery, admission, materialization, validity, and
pending-successor decisions in ADR-0017 through ADR-0023. They fix important
technical constraints but do not choose a public Epoch producer, Namespace
governance, supported scale, or a browser origin/trust model.

Open Product Owner selections are:

- H4-4A selected the separate `ardents-alpha://` overlay. The Firefox-only
  `http://<name>.ard/` XPI/native-manifest route remains a signed compatibility
  tracer under ADR-0045, but Firefox resolver evidence rejects it as a
  participant no-leak address route. The finite signed corpus, alpha OHTTP
  Relay/Gateway profile, and durable stale/conflict floor are maintained local
  slices. The fixed 0.1.0 signed XPI has passed a clean Firefox Release
  install-and-route qualification; a real enrolled release,
  installation/recovery qualification, live multi-Endpoint evidence, DNS/DoH
  coexistence, and an HTTPS/browser-trust decision remain open;
- public name syntax and hierarchy policy beyond the retained V1 profile;
- whether delegation is valuable enough to bear its operational and recovery
  complexity; and
- the exact public admission, lease, expiry, recovery, governance, and abuse
  rules for H4-4C.
