# PW3-26: MR-08 qualify the minimum support matrix

Status: needs-info
State: open
Labels: needs-info
Research class: R3 matching-commit qualification

## Parent

`../PRD.md`

## Canonical sources

- `../../../docs/engineering/research/multi-host-reachability.md`, MR-08 and
  the complete acceptance matrix;
- accepted `../../../docs/adr/0013-bounded-multi-host-reachability.md`;
- accepted MR-03 through MR-07 implementation issues;
- accepted DR-03 / ADR-0011 Authority compatibility.

## User story

As an independent Release Reviewer, I can decide whether both exact
three-host topology variants are production-qualified from one complete,
sanitized, matching-commit evidence set with no hidden retry.

## Qualification behavior

```text
exact clean release candidate + declared immutable version matrix
  -> unit and strict contract corpus
  -> real pinned-SSH integration
  -> dedicated three-host private-LAN E2E
  -> independent-firewall public-direct WAN E2E
  -> hostile-client security matrix
  -> deployment, fencing, rejoin, rollout and interruption matrix
  -> independent Node/Authority backup + WORM checkpoint restore matrix
  -> migration/downgrade and release-material verification
  -> independent review of first attempts and any complete reruns
  -> capability truth derived from the full gate intersection
```

## Frozen MR-08 contract

- Every result binds one exact clean source commit, explicit release version,
  immutable image/material digests and declared Ardents/go-waku/libp2p,
  persistence and DR-03 mixed-generation compatibility.
- Both accepted topology modes use exactly three distinct operator-owned Linux
  amd64 hosts, at least two bootstrap/Store Nodes and one designated Authority
  slot. Docker multinode on one Engine is not real-host evidence.
- Private-LAN evidence includes routed private TCP, bootstrap loss, segment
  partition/rejoin, churn, two-Store recovery and identity-preserving restart.
- Public-direct evidence includes at least two independent NAT/firewall
  domains, one admitted public TCP/WSS endpoint per public Node, external
  dialback, DNS replacement and certificate rotation.
- Integration uses the production workstation-side pin-validated SSH
  stream-local forwarding path with workstation-held signer, one Node-bound
  session per host, no remote shell/helper and no secret capture.
- Security includes ingress port scan, host-key mismatch, session cross-use,
  certificate/SAN/key-permission negatives, resource exhaustion and ordinary
  output redaction from an isolated hostile-client environment.
- Deployment evidence includes fresh install, separate Node and Authority
  consistency-group backup/restore, latest/stale checkpoint cases, clock skew,
  host loss, partitions, fence/rejoin, survivor acknowledgement and
  interruption at every durable rollout/recovery boundary.
- The checkpoint repository is independently administered, immutable,
  compare-and-append and outside the Authority/Node backup failure domains.
- Commands, attempts, start/end times, environment/toolchain/base/dependency
  identities, first results, rerun reasons, artifact hashes and review
  dispositions are retained for the supported lifetime.
- A retry never replaces the first result. Any flake causes classification and
  a complete affected-gate rerun on the same candidate.
- Missing environment, adapter, WORM, backup, PKI, WAN, release or evidence
  custody blocks qualification. No local substitute promotes capability truth.

## Acceptance criteria

- [ ] One explicit release version and exact clean candidate bind all evidence.
- [ ] Production mutation/status adapters are composed and independently
      exercised through exact protected bindings.
- [ ] Unit, contract and pinned-SSH integration gates pass without hidden retry.
- [ ] Three-real-host private-LAN E2E passes.
- [ ] Three-real-host public-direct WAN/PKI/dialback E2E passes.
- [ ] Security and redaction matrix passes from an isolated hostile client.
- [ ] Deployment, interruption, fence/rejoin and rollout recovery matrix passes.
- [ ] Independent Node/Authority backup and WORM checkpoint restore matrix passes.
- [ ] Mixed-generation, migration/downgrade and immutable release-material
      matrix passes against accepted DR-03 compatibility.
- [ ] All first attempts, reruns, provenance and hashes are retained and
      independently reviewed.
- [ ] Capability catalogue, evidence register and release documentation are
      reconciled only from the complete matching-commit intersection.

## Admission decision — 2026-07-29

MR-03 through MR-07 and accepted DR-03 compatibility satisfy the design and
consumer-owned coordination dependencies. MR-08 cannot start a truthful R3 run
in the currently declared environment:

- the clean admission head `2017dfc33740590b572fa91f1605e18c6996c0c6`
  has no declared release tag/version or MR-08 immutable version matrix;
- no authorized protected private-LAN or public-direct topology/context bundle
  is declared; the workstation exposes only one configured SSH host entry,
  not the required exact three-host lab;
- no MR-08 environment binding or retained prior qualification artifact is
  present;
- the accepted deployment coordinators remain consumer-owned seams; no
  non-test production composition exists for the mutating
  host/preflight/Authority/commit adapters required by the matrix;
- no independent compare-and-append checkpoint repository, separate
  Node/Authority backup destinations, hostile-client/dialback lab, independent
  NAT/firewall domains, PKI/DNS rotation authority, release runner or
  supported-lifetime evidence destination is declared.

Starting local Docker or using the known single production host would not meet
the accepted matrix and would mutate an undeclared environment. Admission
therefore remains `needs-info`; `deployment.multi-host` remains unqualified.
No qualification command, remote mutation, deployment, release, push or
capability promotion was performed.

## Required unblock declaration

Provide one protected run manifest or equivalent release-runner configuration
that identifies, without committing secrets:

1. release version, exact candidate commit, immutable images/materials and
   compatibility matrix;
2. authorized exact three-host private-LAN and public-direct environments,
   their protected workstation contexts/pins and external hostile/dialback
   runner;
3. independent WORM checkpoint repository and separate Node/Authority backup
   failure domains;
4. PKI/DNS/firewall administration and approved destructive recovery windows;
5. production adapter composition or the separately reviewed implementation
   range that supplies it;
6. independent release runner, evidence destination, retention and reviewer.

After that declaration MR-08 starts from a new exact clean commit; the present
admission audit is not a qualification attempt.

## Evidence

- `../evidence/mr08-admission/run-1/REPORT.md`

## Out of scope

- weakening the exact three-host or two-mode support matrix;
- treating local Docker, one host, mocked adapters or partial gates as R3;
- inventing credentials, host addresses, WORM/backup custody or release scope;
- production deployment or capability promotion before the full gate
  intersection passes.
