# W3-00: Freeze the post-R1 research baseline

Status: ready-for-agent
State: open
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

- [ ] Completed AIJ-01/02, OCS-01 through OCS-05, and FEC-001/002 are represented in the tracked local tracker.
- [ ] The R1 parent and global plan describe R1 research and implementation as completed.
- [ ] Canonical capability truth reports the post-R1 product source and its generated projection is current.
- [ ] No capability is promoted to `Q=yes`.
- [ ] Capability, documentation, architecture, traceability, formatting, vet, and applicable unit/tooling checks pass.
- [ ] Wave 3 charter, PRD, prompt, and issues identify one exact product baseline and one preparation revision.
- [ ] Local `main` equals `origin/main` and the final worktree is clean.

## Blocked by

None - can start immediately.

## Comments

None.
