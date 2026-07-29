# MR-08 admission audit — run 1

Audit instant: 2026-07-29

Exact clean admission head:
`2017dfc33740590b572fa91f1605e18c6996c0c6`

## Decision

`needs-info` — no R3 qualification attempt was started.

MR-03 through MR-07 and the accepted DR-03 compatibility dependency are
complete. The current workspace cannot supply the external evidence required
by the frozen MR-08 matrix.

## Sanitized observed facts

- Worktree at the audit instant: clean.
- Release tags at the exact head: 0.
- Declared MR-08/multihost/WORM/qualification environment bindings: 0.
- Retained local MR-08 qualification artifact candidates: 0.
- Configured SSH host entries visible to the workstation: 1.
- Non-test production composition of the MR-03 through MR-07 mutating
  coordinators/adapters: absent.

No host alias, address, Principal, Peer ID, signer, key, certificate, backup
path, repository credential or secret value is retained in this report.

## Missing authoritative inputs

- Explicit release version and immutable release/compatibility matrix.
- Authorized protected exact-three-host context bundles for both modes.
- Independent NAT/firewall and external dialback/hostile-client environments.
- Production mutating adapter composition.
- Independent WORM checkpoint repository and separate Node/Authority backup
  failure domains.
- PKI/DNS rotation administration and approved recovery windows.
- Independent release runner, retained evidence destination and reviewer.

## Why local evidence is not a substitute

The canonical matrix requires three real Linux hosts for both modes and an
independent release lab. One Docker Engine, mocked consumer adapters, local
Windows tests or the known single remote host cannot prove host loss,
independent firewall/NAT behavior, WORM/backup failure-domain separation or
matching-commit release custody.

## Effects

No qualification command, remote connection, host mutation, deployment,
backup, restore, release, push or capability promotion occurred.
`deployment.multi-host` remains `Q=no`.
