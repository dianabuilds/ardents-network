# PW3-24: MR-06 admit verified public-direct endpoints

Status: ready-for-agent
State: open
Labels: ready-for-agent
Research class: R0 implementation plus deferred R3 three-host qualification

## Parent

`../PRD.md`

## Canonical source

`../../../docs/engineering/research/multi-host-reachability.md`, MR-06, under
accepted `../../../docs/adr/0013-bounded-multi-host-reachability.md`.

## User story

As an Operator, I publish only externally verified TCP/WSS endpoints and
replace a public address or certificate through a controlled restart that
requires fresh reachability truth.

## Complete vertical behavior

```text
strict ardents.topology/v1 public_direct manifest
  -> exact protected route/firewall/certificate preflight per public Node
  -> deterministic idempotent host apply for all three Nodes
  -> controlled restart for changed address/certificate inputs
  -> public endpoints remain withheld while AutoNAT is Unknown or Private
  -> fresh libp2p AutoNAT Public makes exactly one admitted endpoint publishable
  -> protected topology status proves both public Nodes and the outbound Node
  -> reconciliation repeats after observation loss or configuration rotation
```

The existing Waku runtime remains the sole owner of `Public`, `Private`, and
`Unknown` reachability truth. Deployment preflight proves only that the exact
operator-owned route/firewall/certificate inputs are ready; it cannot create
or substitute an AutoNAT observation.

## Frozen MR-06 contract

- The coordinator strictly re-admits one `public_direct` manifest and derives
  all targets from that exact byte sequence and deterministic host plan.
- Every public target binds manifest digest, slot, SSH pin references, exact
  advertised address, and WSS certificate reference/identity where present.
  Outbound-only targets contain no public ingress or certificate material.
- Protected preflight is required for every public target before any host
  mutation. Its observation must exactly echo digest, slot, address and
  certificate binding, report route and firewall readiness, and be fresh.
  WSS additionally requires certificate readiness. TCP has no certificate
  input and cannot gain one through preflight.
- Preflight is an operator-input check only. It never claims external
  reachability, publishes an endpoint, mutates a router, or weakens the
  runtime's AutoNAT gate.
- Host apply is idempotent for the exact manifest and target. The host adapter
  reports one closed action: `installed`, `unchanged`, or `restarted`.
  A changed public address or certificate must report `restarted`; the
  production adapter contract must stop, replace the complete material pair,
  restart, and await fresh runtime observation.
- An apply observation is accepted only when it exactly binds digest and slot,
  preserves the admitted Node identity, and reports the closed action. A lost
  or invalid response fails closed; the next reconciliation observes actual
  host truth rather than assuming mutation success.
- Final protected status is required. Public Nodes are ready only under
  `public_direct/public/reachable`; the outbound-only Node is ready only under
  `outbound_only/outbound_only/not-reachable`. Required Store and immutable
  image checks remain inherited from MR-02.
- `Private`, `Unknown`, observation stream loss, service stop, address change,
  and restart withdraw public endpoints. Only a fresh `Public` observation in
  the current process may republish the currently admitted address.
- Ordinary result and errors contain only bounded counts and stable reason
  codes. They never contain addresses, DNS names, certificate references,
  paths, host aliases/pins, Peer IDs, Principals, images, or signer material.
- All steps and the complete operation are bounded. There is no remote shell,
  firewall/router administration, certificate issuance, DNS mutation, or
  production host adapter in R0.

## TDD seams

- `deployment.PublicDirectCoordinator` owns deterministic preflight, apply and
  final protected-status reconciliation.
- `deployment.PublicDirectPreflight` owns protected operator-input
  observations and no reachability truth.
- `deployment.PublicDirectHosts` owns idempotent install and controlled
  restart behavior.
- `deployment.PublicDirectStatus` owns the existing protected topology status
  projection.
- The Waku runtime's existing AutoNAT subscription and WSS validator remain
  the production reachability and certificate-material boundaries.

Tests substitute only these consumer-owned deployment boundaries. They must
not introduce a second reachability source.

## Acceptance criteria

- [ ] Invalid mode, unbound/stale preflight, unsafe certificate binding, and
      missing adapters fail before host apply.
- [ ] Both public targets pass exact fresh route/firewall preflight; WSS also
      passes exact certificate preflight.
- [ ] All three host plans apply deterministically and only closed, exactly
      bound apply observations are accepted.
- [ ] Address/certificate rotation is represented by a controlled restart and
      never inherits old reachability.
- [ ] Runtime tests cover `Public`, `Private`, `Unknown`, observation loss,
      withdrawal and fresh recovery without opening Windows firewall prompts.
- [ ] Final status distinguishes public and outbound-only truth and requires
      immutable image and Store readiness.
- [ ] Focused, full, race, tooling, architecture, capability, API-generation,
      vet, vulnerability and diff checks pass.
- [ ] Independent Spec, Standards and Security review findings are resolved.
- [ ] `deployment.multi-host` remains `Q=no`.

## Out of scope

- production SSH, Linux service-manager, firewall/router, DNS, ACME/private-CA
  administration, certificate distribution, or host adapter composition;
- real independent-host WAN dialback, NAT/firewall denial, PKI rotation, or
  matching-commit qualification;
- rollout journaling (MR-07), qualification (MR-08), release, push, or
  deployment;
- changing Realm Authority, Waku AutoNAT semantics, or endpoint truth.

## Required evidence

- red-first coordinator contract tests and Waku regression tests;
- exact binding, freshness, mode rejection, redaction, timeout and invalid
  observation negatives;
- local-substitutable TCP/WSS install, unchanged and controlled-restart cases;
- runtime withdrawal/recovery tests for all AutoNAT states and stream loss;
- independent Spec and Standards review of the complete diff;
- focused security audit;
- all repository gates listed in the acceptance criteria.

## Admission decision

MR-05 is accepted and closed. The current runtime already owns strict public
address validation, WSS chain/SAN/expiry validation, one-address publication,
AutoNAT gating, observation withdrawal, and restart-freshness. PW3-24 is
therefore admitted as the bounded deployment reconciliation seam around that
truth, without duplicating it or claiming R3 evidence.

Admission authorizes repository implementation and local verification only.
It changes no production state or capability qualification.
