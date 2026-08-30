# H4-6 — Transparent alpha control and public-control transition

Status: **bounded alpha-control inspection and transition mechanics are
maintained evidence. Control remains project-operated; no current candidate or
independent public-control claim is accepted.**

## Decision

H4-6 does not make traffic flow and does not turn a known project topology into
a decentralized network. It makes control over that topology visible and
bounded in alpha, then specifies the evidence required before that provisional
control may be replaced by independently checkable public control.

Control Plane here means the authorities and artifacts that decide network
discovery and eligibility, Namespace materialization, release safety, build
identity, compatible protocol generations, and emergency disablement. These
are distinct decisions with distinct failure consequences. One signature, one
server, or one project's statement cannot honestly serve as proof that they are
independent.

The usable alpha may have known project control. Its contract is transparency:
the participant can see the exact accepted roots and inputs, verify what the
Endpoint accepted, and learn when an input is absent, stale, conflicting, or
revoked. The contract is not governance independence, censorship resistance,
or permissionless public operation.

## Why this is separate from H4-5 and H4-4

H4-5 asks whether a person can operate a traffic-carrying Node duty. H4-6 asks
who can declare that duty eligible, what compatible profile it may run, and
how every Endpoint verifies the declaration. Many Nodes do not make those
control decisions independent.

H4-4A can use a declared alpha corpus. H4-4C cannot honestly admit competing
permissionless root-name claims until an authenticated shared Epoch close can
order inputs and materialize the current Namespace. A temporary project
registrar is not a substitute for that control path.

## Alpha-control contract

For every alpha cohort, publish or bundle without secrets:

- release artifact identity, trusted release-verification roots, supported
  platform/profile, and the manual replacement/revocation procedure;
- exact Network Epoch or other accepted network-state identity, validity and
  compatibility bounds, selected profiles, and declared alpha Node topology or
  source restrictions where disclosure is safe;
- the current Namespace corpus authority, if H4-4A is selected, its finite
  scope and expiry, and the fact that it is not permissionless;
- the build, protocol, and state-generation floors that cause an Endpoint to
  accept, refuse new work, request update, or stop; and
- a small deterministic reader or documented verification flow that lets a
  participant inspect these exact artifacts without trusting an explanatory web
  page.

The Endpoint must show separate, actionable states such as `network state
unavailable`, `network state expired`, `release unsafe`, `build incompatible`,
and `alpha control conflict`. It must never repair an invalid state by silently
using an older build, a direct source, an unverified mirror, a different
protocol profile, or a locally invented network configuration.

## Delivery slices

### H4-6A — observable, reproducible alpha control

**Goal:** an alpha participant can identify exactly what controls their result.

Publish one signed disclosure catalog and verification instructions alongside
the signed H4-1 artifact. Each catalog component remains subject to its own
authority, verifier, and floor: the catalog is an inspectable index, never an
authority for Release, Network State, compatibility, or Namespace. Bind the
running build, accepted network-state digest, profile generation, and any
bounded naming corpus to participant-visible diagnostics. Exercise a clean
install, cached valid restart, expiry, rejection,
conflict, and deliberately unavailable distributor/source.

**Done when:** two fresh Endpoints verify the same declared inputs or explicitly
reject them for the same documented reason. A Product Owner walkthrough can
reproduce the result from the immutable artifacts; it does not count as an
independent control audit.

### H4-6B — separated transition contracts

**Goal:** prevent an alpha convenience from becoming unbounded permanent power.

Specify separately for release safety, Network Epoch, Namespace materialization,
and protocol/build compatibility: authority root, predecessor relation,
validity/freshness, rotation, revocation, rollback floor, emergency action,
human-visible failure, and retained verification evidence. An emergency may
stop unsafe new work; it may not seize a Name, rewrite a live destination,
silently downgrade a Route Profile, or force arbitrary executable installation.

This slice chooses no threshold scheme or custodian organization. It makes the
eventual choice reviewable and ensures the alpha states do not collapse into
one all-powerful project key.

### H4-6C — project-control simulation

**Goal:** make the project's shared-control mechanics reproducible and
inspectable by the Product Owner and Codex.

ADR-0055 selects five simulated custody roles, `3-of-5` routine authority, an
expiring disable-only `4-of-5` emergency, predecessor-and-successor lifecycle
rotation, two full Candidate View reconstructions, two builder/auditor roles,
and the bounded reader's failure matrix. `simulate-public-control` exercised
those exact mechanics with fresh in-memory keys and retained in-memory evidence.
ADR-0060 later retired that completed campaign generator and command; the
historical receipt and Git revision remain unchanged provenance.

The result is **complete for H4-6C**. It neither selects a public candidate nor
asserts independent operation, public availability, or Public Beta. Those are
not residual H4-6C work; any future public claim needs a new Product Owner
decision and evidence contract.

### H4-6D — controlled project-control transitions

**Goal:** prove that the already selected project-control simulation rejects
unsafe control transitions without using an older generation, alternate source,
or local repair.

ADR-0056 and R-125 selected one bounded local matrix: continuous overlap is
accepted; expiry, revocation, incompatible generation, rollback, distribution
outage, and an in-scope emergency disablement each produced their exact stop or
unavailable result. An overlap without continuity, an emergency that escalated
its scope, and an expired emergency were rejected. The evidence is the retained
versioned JSON receipt from the now-retired `ardents-control
simulate-public-control-transitions --source-revision
LOWERCASE_40_HEX_COMMIT` campaign.

This is complete historical H4-6D evidence. ADR-0060 retired the generator and
command after cross-checking the useful assertions against their domain owners.
The Product Owner-and-Codex simulation created no authority, modified no
Endpoint root, and made no claim about public operation, independent control,
availability, or Public Beta.

## Evidence and promotion gates

Every selected control artifact needs a stable identity, declared authority,
validity, predecessor/floor, distribution-independent verifier, and exact
reject/stop behavior. Evidence covers forged, stale, replayed, revoked,
conflicting, withheld, and unavailable inputs, along with release replacement
and restart behavior.

H4-6C promotion means only acceptance of its project-control simulation. A
future Public Beta claim has its own scope and must not be inferred from this
simulation.

## Non-goals

- Pretending a project key set is threshold governance or independent control.
- A new blockchain, token, election, treasury, or generalized governance system.
- Administrative Name seizure, discretionary registry recovery, silent Route
  downgrade, or a remote command to install arbitrary software.
- Treating one reproducible local build, one mirror, or one endpoint's partial
  View as proof of an independent public network.

## Selected H4-6A alpha contract

ADR-0038 selects the concrete H4-6A catalog wire, component/root mapping, and
separate reader: an enrollment-pinned catalog and four independently pinned
Ed25519 roots; fixed Release, Network, and Compatibility statements; and
separate catalog, Release, and Network floors. The catalog stays an index and
cannot select a component key, source, or Endpoint authority.

## Selected H4-6B alpha transition contract

ADR-0054 separates the four Functional Alpha transition domains. Release
Safety, Network Epoch, and Compatibility keep their own authority,
predecessor/floor, freshness, rotation, revocation, emergency, and evidence
rules; `ardents-control inspect-transitions` renders their independent result.
Namespace materialization is explicitly **not selected**: the alpha cannot
close, release, reclaim, recover, or materialize a canonical Name. Target Links
remain the complete current destination path. A withheld input is rendered as
unavailable because an Endpoint cannot distinguish withholding from outage; no
fallback root, profile, source, or Namespace authority is allowed.

## Open Product Owner selections

- No H4-6C or H4-6D selection remains open. ADR-0055/R-124 own the completed
  mechanics simulation; ADR-0056/R-125 own the bounded transition simulation.
