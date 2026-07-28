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
