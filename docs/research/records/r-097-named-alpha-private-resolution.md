---
id: R-097
title: Named alpha and live private-resolution proof
status: decided; implementation-linked
owner: Product Owner and Codex
started: 2026-08-24
reviewed: 2026-08-26
---

# R-097 — Does a bounded named-alpha slice provide enough user value to justify live Private Resolution before a permissionless Namespace, and what exact proof/failure contract keeps it distinct from DNS or a registrar?

## Decision this unlocked

Decide whether to build H4-4A after H4-3, or keep Target Links as the only
alpha destination form until canonical Namespace/public-control work is ready.
On 2026-08-25, the Product Owner selected the bounded non-Namespace alpha
overlay (option 4) for implementation. The selected private-resolution role
evidence and terminal-state behavior are now maintained; later public naming
and participant qualification are distinct delivery gates rather than an open
R-097 decision.

## Current contract

- Target Links remain complete authenticated destination paths and never become
  fallback from a failed Name.
- A Service Name binds to a Target through Name Authority, lease and finite
  validity; it is not DNS, a directory, identity, or authorization.
- Current naming/resolution modules and ADR-0017 through ADR-0023 are technical
  inputs, but no global Epoch-close producer/public Namespace is selected.
- H4-6 is required before competing permissionless root claims.

## Hypotheses

- **H1:** a small pre-provisioned corpus with explicit owner/expiry/withdrawal
  proves named navigation value and exercises exact private proof verification.
- **H2:** names add little before user testing and divert effort from the
  Target-Link web loop; H4-4A should be deferred.
- **H0:** existing private-resolution inputs cannot be safely connected to the
  live profile without conflating project corpus with a public registrar.

## Evaluation criteria

- user journey relative to exact Target Link, without assuming external users;
- exact proof/current binding, stale, conflicting, expired, unavailable, and
  withdrawal outcome; no DNS/HTTP/alias/Target fallback;
- query-origin/name separation and all alpha corpus authority disclosures;
- name-to-browser presentation without ordinary hostname/Web PKI claims; and
- implementation/operation scale bounded for the actual team.

## Evidence plan

### Primary sources

- Current naming technical contract, accepted naming ADRs, product journey, and
  threat model.
- OHTTP and relevant private-resolution specifications already selected by the
  maintained contract.

### Experiment

After H4-2/3 exist, run two alpha Endpoints against a declared finite corpus.
Exercise exact resolution, stale/revoked/conflicting proof, corpus withdrawal,
restart, and no-fallback handling. Capture resolver role observations and
participant-visible states.

### Failure scenarios

- One resolver learns both origin and exact name.
- A stale/current conflict or withheld proof reaches a prior Target.
- An operator manually reassigns a name or alpha corpus silently persists.
- A browser rewrites a Service Link into DNS/HTTPS.

## Findings

- **Current-contract fact:** `naming/namespace` treats an Authority-signed
  successor as pending, not current; a current Binding appears only after its
  authenticated Epoch installation and threshold-attested materialization.
  The retained contract also says no global-close producer is selected, so
  root-claim current behaviour is unavailable. [Naming and private
  resolution](../../technical/naming.md) (inspected 2026-08-24).
- **Inference:** a hand-maintained or merely Authority-signed alpha list cannot
  be silently presented as the current Ardents Namespace or as H4-4A private
  resolution. Doing so would bypass the exact current-binding rule that the
  product relies on to reject stale and conflicting proof.
- **Current-contract fact:** the naming contract requires the named alpha to disclose
  its small pre-provisioned corpus and authority, and forbids a hidden registry
  operator, manual per-request approval, or fallback destination. [Naming
  contract](../../technical/naming.md) (inspected
  2026-08-24).
- **Inference:** H4-4A has a predecessor not made explicit in the initial
  ordering: R-098 must first select a readable, signed alpha-control input and
  its expiry/withdrawal/replay contract. R-097 may then determine whether that
  input can supply a finite alpha corpus to the existing proof verifier without
  claiming a public Namespace. If it cannot, Target Links remain the only
  alpha destination form.
- **Implementation fact:** the current `naming/namespace.ResolutionVerifier`
  calls `epoch.VerifyBinding` with an installed materialization policy, minimum
  Epoch, and expected Epoch digest. The Gateway likewise obtains only a Store
  proof and verifies it against those same inputs before passing a Binding to
  Resolution. Its focused Namespace and Resolution tests passed on 2026-08-24.
  [Resolution view](../../../internal/naming/namespace/resolution_view.go) and
  [materialization verifier](../../../internal/naming/namespace/epoch/materialization_proof.go)
  (inspected 2026-08-24).
- **Inference:** a R-098 signed disclosure catalog, even with a separately
  signed named-alpha component, cannot enter that verifier: it has neither an
  authenticated Epoch installation nor the threshold-attested current proof
  required by the maintained contract. Replacing these inputs with a project
  key would create a second current-Namespace authority and contradict the
  stated H4-4A boundary.
- **Product decision (2026-08-25):** the Product Owner selected the separate
  alpha-label overlay, with `ardents-alpha://<canonical-name>` as its exact
  destination form and a finite signed Alpha Name Corpus. The choice accepts
  no authority claim beyond that declared alpha corpus; `ardents://` remains
  reserved for canonical Namespace work.
- **Historic product decision (2026-08-25):** the first H4-4A Browser Adapter
  tracer used an explicit loopback hostname, port, and opaque path. RFC 6761
  reserves localhost names for loopback use, but this was neither public DNS
  nor Web PKI. It was superseded by ADR-0045: the alpha participant
  presentation is `http://<canonical-name>.ard/`; the listener origin remains
  only an internal loopback implementation detail.
- **Inference:** a technically possible *separate* alpha-label overlay would
  be a new contract, not an adapter to the maintained Namespace: it would need
  an explicit alpha-only prefix/cohort, finite signed label-to-Target entries,
  expiry and total withdrawal, a distinct verifier outcome, OHTTP role proof,
  and no conversion of its error into a Target Link. It must never call its
  result `current Namespace`, share `ardents://name` as a public claim, or be
  used as a migration shortcut around H4-4C. Designing that new authority now
  would duplicate resolution work before H4-3 has demonstrated that names add
  enough value for the actual alpha team.

This is early contract analysis. No alpha corpus, resolver change, or public
registration is authorized.

## Options

1. Defer names; retain Target Links only.
2. One finite pre-provisioned named-alpha corpus.
3. Full permissionless root claims.
4. A distinct alpha-label overlay, explicitly outside the Namespace verifier.

## Selected direction

Implement option 4 as a clearly non-Namespace alpha overlay. The maintained
slice verifies a complete finite signed corpus with explicit expiry and
authenticated total withdrawal, retains its serial/digest under an
Endpoint-local durable floor, and proves a bounded OHTTP Relay/Gateway role
split. An already verified binding enters the existing exact C-2 route only
through `OpenAlphaUserReferenceSite`; callers cannot supply or receive its
internal Target Link. This selection does not authorize a shadow registrar, a
public Namespace assertion, or a shortcut around H4-4C/H4-6.
Option 3 remains blocked on H4-6 by contract.

**Confidence:** high that a discretionary static list cannot inherit the
current Namespace claim. **Strongest argument against this recommendation:** a
purpose-built alpha corpus reader may duplicate enough of Namespace control to
be unjustified; deferring all names may be the simpler and more honest result.

## Disposition

Decided and implementation-linked. The maintained alpha/Endpoint tests exercise
OHTTP Relay/Gateway separation without retaining per-request observations;
unavailable, expired, withdrawn, stale, and same-serial-conflicting corpus
outcomes; decision-time validity before durable-floor advancement; persistent
restart floors; ACA2 corpus-component verification; and refusal of a
caller-provided Target Link. The durable floor obtains its exclusive lease
before recovering an interrupted successor, so a concurrent opener cannot
delete an active writer's temporary file; its root is owner-only on the
supported platform boundary.

Two separately opened owner-only persistent floor roots now make independent
private OHTTP requests through the same distinct Relay and Gateway and retain
the same signed `ardents-alpha://blog.alice` Binding, including its Network,
Target, serial, and digest. This is maintained component evidence for the
two-alpha-participant resolution condition; it is not a multi-host or
participant-operational qualification.
The maintained in-memory C-2 integration resolves its exact alpha Binding
through the alpha OHTTP Client, separate Relay, Gateway, and persistent floor
before it can create the C-2 route; it then passes the bounded document,
stylesheet, and SVG journey. The separate-process C-2 fixture starts the same
alpha Gateway and Relay as distinct bounded processes, and separately accepts
the same corpus into a User and an Observer Endpoint floor. Each Endpoint
resolves the exact name through OHTTP and verifies that its result agrees with
its independently retained accepted floor; only the User continues to C-2.
The fixture passes success, Publisher-offline, and local-Application-refusal
outcomes with every long-lived alpha and C-2 role drained. The updated
eleven-process tracer passed locally, in fresh Linux Docker, and in one
ephemeral Docker container on the project VPS on 2026-08-26. The Docker and
VPS cells set 1 vCPU, 1 GiB memory, and 128 PIDs; both mounted current source
read-only and published no ports. The VPS source copy was removed afterwards.
This remains one test-owner fixture, not a multi-host qualification. It is
functional low-resource evidence only, not a capacity measurement or minimum
supported profile.
An explicit Windows Firefox 154 run historically loaded all three resources
from a loopback compatibility origin; the Publisher proof, rather than process
launch, was the success criterion. It is retained as provenance, not as the
participant journey. The explicit `make qualification-h4-4a-firefox`
Windows target now starts from `http://reference.ard/`; an absent selected
Firefox is an invalid environment, not a passing skipped test.

This is not yet a multi-host or participant-operable live OHTTP-to-C-2
deployment: the process fixture provisions its signed corpus directly as test
input, and all roles run under one test owner. It also does not establish user
value without participants, alpha corpus distribution
operations, a public Namespace, browser privacy, or an acceptable final
browser-visible address. The Product Owner explicitly rejects visible
`localhost:<port>` as that final address. A provisional Firefox-only tracer
has since loaded the exact `http://reference.ard/` origin through the
Endpoint-owned local proxy. ADR-0045 now selects its source-level
extension/native-host delivery boundary; the remaining gap is a concrete
Mozilla-signed XPI, enrolled release, and two-platform participant
qualification, not an unselected mechanism. H4-4C still needs public-control
evidence.
