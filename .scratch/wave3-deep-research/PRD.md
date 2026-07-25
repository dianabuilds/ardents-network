# Wave 3 Deep Research

Status: ready-for-research

## Outcome

Resolve the product, protocol, security, and operations decisions that still
block independently implementable work for Application Messaging, Application
Hosting, Channel Grant authority, multi-host reachability, and direct service
interaction.

Each research task must end in one of these outcomes:

- an accepted interface and dependency-ordered implementation breakdown;
- a proposed ADR plus an implementation breakdown;
- a bounded prototype request that resolves one named uncertainty;
- an explicit defer or rejection decision.

An open-ended survey is not a completed research task.

## Admission gate

Wave 3 agents must not claim a frozen baseline until all of the following are
true:

- R1 tracker files are tracked and the R1 parent reflects actual completion;
- completed AIJ, OCS, and FEC slices are represented in the local tracker;
- the global plan no longer says that completed R1 issues await publication;
- the canonical capability catalogue is reconciled with the post-R1 source
  state without promoting any capability to `Q=yes`;
- the capability catalogue, documentation contracts, architecture acceptance,
  and relevant tooling tests pass;
- local `main` equals `origin/main` and the worktree is clean.

Wave 3 product claims are frozen at
`main@8b9f8ad87fb78fccd7a73d445f2d72dbf2e51b4c`, the final post-R1 product
commit. W3-00 records the later documentation/governance preparation commit,
but research facts continue to identify this one product snapshot so a
documentation-only self-reference cannot move the assessed source.

## In scope

- DR-01 Application Messaging.
- DR-02 Application Hosting.
- DR-03 Production Channel Grant authority.
- DR-04 Multi-host reachability.
- DR-05 Direct service interaction.
- Cross-packet synthesis for v1 scope and dependency ordering.

## Out of scope

- Implementing the selected designs.
- AD-01 through AD-04 Application Discovery implementation.
- AD-05 or DR-06 release qualification.
- Kubernetes or scheduler support.
- QUIC, WebTransport, or WebRTC.
- Non-Go SDKs and remote Application transport.
- Promoting current capabilities to production-ready or qualified.

## Provisional dependency order

```text
DR-03 Channel Grant authority -----> DR-01 Application Messaging ----+
                                                                    |
DR-02 Application Hosting ---------> DR-05 Direct interaction -------+--> W3-SYN
                                                                    |
DR-04 Multi-host reachability --------------------------------------+
```

DR-03, DR-02, and DR-04 are the first parallel research wave. DR-04 may develop
topology and reachability alternatives independently, but its final private
realm recommendation must remain compatible with the accepted DR-03 authority
model.

## Program artifacts

- Charter:
  `../../docs/engineering/research/wave3-research-charter.md`
- Integrator-owned decision register:
  `../../docs/engineering/research/wave3-decision-register.md`
- Reusable packet template:
  `research-packet-template.md`
- Reusable agent prompt:
  `AGENT-PROMPT.md`

## Completion

Wave 3 is complete only when:

- DR-01 through DR-05 have accepted or explicitly deferred recommendations;
- every selected design has a small external interface and a deep internal
  module boundary;
- trust, authority, privacy, delivery, failure, restart, recovery, migration,
  abuse, and operability semantics are explicit;
- every task compares at least two materially different designs;
- required ADRs exist as proposed or accepted decisions;
- implementation issues are vertically sliced and dependency ordered;
- a synthesis records which capabilities belong to the first release;
- DR-06 receives a finite qualification scope rather than an open feature list.
