# W3-00: Freeze the post-R1 research baseline

Status: ready-for-agent
State: closed
Labels: ready-for-agent
Research class: preparation

## Parent

`../PRD.md`

## What to build

Create one clean, pushed source revision from which every Wave 3 researcher can
derive current product truth. Reconcile the completed R1 tracker, canonical
capability catalogue, generated projection, global plan, Wave 3 charter,
decision register, packet template, and agent prompt without changing product
behavior or claiming release qualification.

The resulting baseline must make completed R1 work discoverable, identify
post-R1 capability status at one exact product commit, and record the
documentation/governance preparation revision separately.

## Acceptance criteria

- [x] Completed AIJ-01/02, OCS-01 through OCS-05, and FEC-001/002 are represented in the tracked local tracker.
- [x] The R1 parent and global plan describe R1 research and implementation as completed.
- [x] Canonical capability truth reports the post-R1 product source and its generated projection is current.
- [x] No capability is promoted to `Q=yes`.
- [x] Capability, documentation, architecture, traceability, formatting, vet, and applicable unit/tooling checks pass.
- [x] Wave 3 charter, PRD, prompt, and issues identify one exact product baseline and one preparation revision.
- [x] Local `main` equals `origin/main` and the final worktree is clean.

## Blocked by

None - can start immediately.

## Comments

- Completed on 2026-07-25.
- Frozen product baseline:
  `8b9f8ad87fb78fccd7a73d445f2d72dbf2e51b4c`.
- Wave 3 preparation revision:
  `e1e5299bf3b83cb534605811c032eeb2fe1bdd0c`.
- Canonical catalogue: 24 capabilities, 8 domains, 0 qualified.
- API generation, vet, full unit/tooling tests, capability/doc/architecture/
  traceability gates, and fresh Windows checkout formatting passed.
