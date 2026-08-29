# H4-5 — Contributor viability and permissionless admission

Status: **Rendezvous selected for the H4-5A/B dedicated-host functional-alpha
profile; implementation and declared-host qualification are active. No supported
profile, voluntary co-resident contribution, or public admission is accepted
until the named evidence closes.**

## Decision

H4-5 is the operator-facing counterpart to H4-1 through H4-3. It answers one
concrete question first: can a person install, understand, run, limit, update,
drain, and withdraw **one selected Network Contributor duty** on a dedicated
host, and does that duty make a measured useful contribution to the live alpha
network?

It does not redesign the complete route-role system. H4-2 owns the network
profiles, role separation, and which duties the selected route requires.
H4-5 turns one of those duties into a supportable operator product, then only
later considers admission of more operators. A Node is infrastructure; a
Publisher is a separate Endpoint capability that exposes a Service and remains
H4-3 work.

The Product Owner selects the already maintained H4-2 **Rendezvous duty** for
H4-5A/B. R-092 found it to be the smallest useful data-plane role: it joins one
authenticated Initiator and Responder leg, has explicit handshake/waiting/pair
reservations, and already participates in the selected Publisher-to-User C-2
topology. This selection authorizes only one dedicated Ubuntu host profile. It
does not authorize another duty, co-residence, public admission, capacity or
availability claims, or independent-operation language.

The default installation still creates an Endpoint, never a Contributor Node.
Public qualified contribution remains a dedicated host/Endpoint with no User
connection or Service-publication role. This is the current safety boundary,
not a judgment that co-resident voluntary contribution can never be useful.

## What counts as a useful contribution

For H4-5, “useful” has a deliberately narrow, observable meaning. A selected
Contributor duty is useful only if, in the declared alpha topology, it:

- performs one exact required role in live H4-2 connections rather than merely
  registering or relaying synthetic keepalives;
- admits a declared bounded amount of work and preserves the selected role's
  route, source-separation, and failure rules under load;
- can be taken down, restarted, updated, or drained with the documented effect
  on new and existing work; and
- adds measured available role capacity after the Endpoint's own mandatory
  family, role, and source exclusions.

This is alpha utility evidence, not proof of decentralization. A project-run
fleet, nominal identities, addresses, or many copies of a binary do not become
independent capacity merely by being counted.

## Boundaries between roles

| Thing | Owner / epic | H4-5 treatment |
|---|---|---|
| Client | H4-1/H4-3 Endpoint capability | Must work without contributing. |
| Publisher | H4-3 Endpoint capability | Publishes a local Service; is not a Contributor by default. |
| Node duty | H4-2 selects compatible route/control roles | H4-5 selects one duty to operate first; it does not make a speculative role catalogue. |
| Network Contributor | H4-5 operator product | Operates the selected Node duty within fixed resources, reachability, update, drain, and withdrawal rules. |
| Voluntary Client/Publisher contribution | R-093 research | An opt-in future candidate only; never an implicit outcome of Client or Publisher installation. |

## Delivery slices

### H4-5A — dedicated Contributor operating profile

**Goal:** a declared operator can bring up and retire one selected Node duty on
a dedicated host without undocumented project intervention.

The profile must specify its supported platform and account model; installation
and authenticated configuration inputs; required public reachability; role
identity/family declaration; inbound and outbound exposure; CPU, memory,
bandwidth, storage, connection, and queue ceilings; local health/diagnostics;
release/update; finite drain; withdrawal; and residue/removal handling. The
exact platform and duty are selected from the evidence of
[R-092](../../research/records/r-092-native-node-operating-profile.md), not
assumed from the Client Portable profile.

The candidate resource placement is named
`h4-5-rendezvous-alpha-v1`: one Ubuntu LTS `x86-64` dedicated host, one
unprivileged Rendezvous process, `1` CPU of process quota, `256 MiB` cgroup
memory maximum with a `192 MiB` high boundary, `128 MiB` Go memory limit,
`64` tasks, `256` file descriptors, and an aggregate Network-State plus
local-role-state ceiling of `384 MiB` and 5,000 regular files (`PROTECT` at
`320 MiB`, recovery below `256 MiB`). Rendezvous owns no application queue:
its queue item and byte ceilings are exactly zero; finite handshake, waiting,
and pair reservations are measured separately. The Node resource governor must enter
`PROTECT` only after its fixed high observation and terminal `DRAIN` at an
emergency threshold; systemd/cgroup ceilings are enforcement backstops, not a
capacity claim. These values remain a candidate until the declared-host matrix
accepts or rejects them.

**Product Owner host selection (2026-08-29):** the existing project-operated
Ubuntu hosts are eligible for this functional-alpha campaign regardless of
their physical CPU, RAM, disk, or link size. Those facts are captured as the
host envelope rather than used to reject a run. The supported boundary remains
one role-exclusive Contributor service with the exact cgroup/runtime limits
above; temporary qualification fixtures on the same project host do not create
a supported co-resident Endpoint-plus-Contributor profile or an independence,
capacity, or availability claim.

**Product Owner campaign-budget selection (2026-08-29):** H4-5A/B uses one
no-retry pass of every deterministic cell plus one eight-minute mixed
sustained soak. Independent supporting shards run concurrently across both
declared existing Ubuntu VPS hosts and local isolated Docker containers, while
the selected installed Contributor profile remains bound to one declared VPS.
The complete campaign has a hard 60-minute ceiling, stops starting work at
minute 50, and reserves the remainder for evidence and exact cleanup. Five
ten-minute repetitions of every deterministic lifecycle and fault operation
were rejected as disproportionate to this narrow project-operated Functional
Alpha claim. Every failed attempt remains evidence; after a correction only
the affected cell and one short ordinary smoke are repeated, never the entire
campaign.

The declared-host preflight passed on 2026-08-29 at commit `bdb9a665`: the
selected existing Ubuntu VPS has the required systemd/cgroup-v2 platform,
unused selected listener port, and no prior Contributor installation. This is
host eligibility only; the workload, lifecycle, utility, and fault matrix has
not yet accepted the profile.

The first corrected installed-product smoke passed at commit `174283d5`: an
empty Contributor acquired its first signed State from the two pinned Sources,
carried the real C-2 Rendezvous position, completed restart/drain/withdrawal and
exact removal, and left no managed residue. Three preceding classified failures
remain in the denominator. This advances implementation readiness but does not
replace the frozen repeated workload/fault matrix or accept the profile.

**Done when:** a Product Owner can follow the documented flow on a declared
eligible host, run the dedicated duty through ordinary load and injected loss/restart,
inspect its bounded resource and readiness state, drain it, and verify that it
has stopped accepting new work. No hidden always-on operator, private support
channel, or manual state repair may be necessary.

### H4-5B — alpha utility and operator burden

**Goal:** establish that the operated duty is worth asking someone to run.

Record the exact topology and role position, work admitted and completed,
availability/reachability over the declared observation window, resource
consumption, overload/abuse response, update/drain effect, withdrawal effect,
and the human steps/time/failure points for the operator. Compare the
alpha topology with and without the duty only under the declared conditions;
do not infer independent public capacity from it.

A negative result is valid: if a duty creates more operator burden or endpoint
exposure than useful role capacity, it is rejected or kept project-operated for
the test network. H4-5 is where we find that out before proposing incentives.

### H4-5C — voluntary Endpoint contribution research

[R-093](../../research/records/r-093-voluntary-endpoint-contribution.md)
belongs beside H4-5 because it asks whether a Client or Publisher owner might
*choose* to provide a limited duty. It is not H4-5A's deliverable and must not
block the dedicated-host profile.

Only after R-093 selects a precise candidate may a disposable, visibly
unqualified alpha experiment begin. It must be explicit opt-in, show resource
and exposure effects before activation, retain an immediate stop/drain action,
and remain excluded from independent-capacity and public privacy claims. The
research may conclude that no co-resident duty is acceptable.

**Current research outcome:** H4-5C is deferred. It cannot be a shortcut to
make every Client or Publisher into a Node: Rendezvous is selected only for the
dedicated-host H4-5A/B profile, whose qualification must first show that the
exact duty is operable and useful. A future H4-5C experiment
requires an explicit Product Owner choice after those results; until then the
Endpoint offers no contribution duty. See
[R-093](../../research/records/r-093-voluntary-endpoint-contribution.md).

### H4-5D — permissionless admission

**Goal:** admit a new dedicated Contributor without individual approval while
keeping role, resource, and concentration safeguards enforceable.

This slice needs an authenticated admission/eligibility path, finite probation
and withdrawal rules, abuse handling, declared family treatment, capacity and
role-domain gates, and an exact response to false declaration or unavailable
duty. It can begin only when H4-5A/B show an operable profile and H4-6 can
carry the required transparent control state. It is not a synonym for an
installer download page.

## Evidence and promotion gates

For every selected duty, publish its exact claim, topology, operator steps,
resource ceilings, prerequisites, failures, and measurements. Exercise NAT or
reachability loss, sleep/reboot/crash, overload, malformed/hostile traffic,
drain, withdrawal, update, stale control, lost network, and recovery. Record
what the duty sees about adjacent endpoints and what co-location/family/source
exclusions it imposes.

A future public claim additionally needs the functional-map effective-family
thresholds after every mandatory exclusion, evidence that the operators really
exist independently, and the stated external review gates. Known alpha
operators can improve availability without satisfying these requirements.

## Non-goals

- Making every Client or Publisher a Node, automatically or by an obscure
  default.
- Treating Publisher installation, Service hosting, or browser access as Node
  operation.
- Tokens, staking, payment, mining, rewards, slashing, or an incentive market.
- Treating a project fleet, IP addresses, nominal identities, or Sybil
  installations as independent operation.
- A hidden operations organization, moderation desk, or 24/7 manual repair
  obligation that the actual project team cannot sustain.

## Open Product Owner selections

- Whether the measured dedicated Rendezvous utility justifies retaining the
  profile or selects the explicit negative disposition.
- Whether R-093 should investigate a particular opt-in duty after H4-5A/B, or
  retain the dedicated-host rule without experiment.
