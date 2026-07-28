# PW3-17: CGA-07 qualify the authority lifecycle

Status: needs-info
State: open
Labels: needs-info
Research class: R3 matching-commit qualification

## Parent

`../PRD.md`

## What to build

Execute the complete authority acceptance matrix on the declared support
topology, retain immutable matching-commit evidence, and reconcile canonical
capability truth only from the complete gate intersection. This issue adds no
feature behavior.

## Acceptance criteria

- [ ] One exact clean source commit and release version bind all evidence.
- [ ] Unit, contract, integration, adversarial multi-host E2E, security,
      deployment, backup/restore, migration/downgrade and release gates pass on
      the declared support topology.
- [ ] Commands, attempts, start/end times, environment, toolchain, source and
      external dependency versions, results and artifact hashes are retained.
- [ ] Retries never hide the first result; flakes are classified and the
      complete affected gate is rerun on the same commit.
- [ ] DR-04 compatibility and DR-06 scope are explicitly accepted.
- [ ] Capability I/R/O/Q promotion, if any, is derived only from complete
      matching-commit evidence and all documentation/catalogue surfaces agree.
- [ ] Missing real-host, WORM, fencing, PKI, backup, migration or release
      environment blocks the corresponding claim rather than being replaced by
      a weaker local test.

## Blocked by

- PW3-16 / CGA-06 accepted.
- Accepted DR-04 compatibility and declared DR-06 qualification scope.

## Comments

- Published as a blocked qualification slice. `Q` remains `no` until every
  acceptance criterion is satisfied on one matching commit.
- 2026-07-28 admission audit after CGA-06 acceptance:
  - the maintainer explicitly accepted CGA-06 implementation commit
    `1136def860f30bc452e1b5352c537cbd44a163f6`;
  - DR-04's compatibility recheck says the implemented authority lifecycle is
    compatible with the selected three-host topology, but ADR-0013 remains
    `Proposed` and has no explicit maintainer disposition;
  - the currently declared DR-06 stabilization scope explicitly excludes
    `realm.channel-grant-authority`, ADR-0013 real-host support and the
    production three-host topology. It cannot be reused as CGA-07 scope;
  - no exact authority release-candidate version, dedicated three-host Linux
    private-LAN environment, independent checkpoint/WORM repository,
    separately retained authority backup failure domain, release runners or
    supported-lifetime evidence destination has been declared for this issue;
  - the canonical authority acceptance matrix requires unit, contract,
    integration, adversarial three-real-host E2E, security, deployment,
    backup/restore, migration/downgrade and release evidence on one exact clean
    commit. Docker multinode and local Windows evidence are not substitutes;
  - admission therefore remains `needs-info`; `Q=no`. Continuing requires an
    explicit DR-04/ADR-0013 compatibility disposition and a separately
    declared CGA-07 DR-06 scope, release-candidate version and authorized
    environments. No qualification attempt, deployment change or push was
    made.
