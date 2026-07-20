# SDE-001 — Self-Forming Docker Deployment Lifecycle

## Scenario ID

`SDE-001`

## Layer

`e2e`

## Domain

`deployment`

## Category

Clean bootstrap, persistence, backup/restore, and rolling lifecycle.

## Purpose

Prove that a clean operator can form and operate the supported single-host
Docker Compose deployment without copying generated Waku peer IDs, node keys,
or hidden runtime values.

## Layer And Environment

- layer: `e2e`
- environment: Docker/Linux containers with Windows or Linux host orchestration
- topology: one seed and two service peers on an isolated Compose network
- tags: `deployment`, `waku`, `persistence`, `backup`, `restore`, `upgrade`

## Scenario

1. Create a unique Compose project and empty deployment state directory.
2. Run `./ardents.ps1 up` with an existing immutable or explicitly local
   development image.
3. Assert that the lifecycle command discovers a non-loopback seed TCP Waku
   multiaddr containing its peer ID and both peers report joined network truth.
4. Stop and start the complete cluster; assert that seed and peers recover
   `ready` network state and peers rejoin.
5. Create a stopped-node backup of one peer, replace its disposable volume from
   that archive, and assert matching Ardents principal, device, and Waku peer ID.
6. Create pre-upgrade backups for every node, recreate nodes one at a time with
   a second image reference, and require readiness after every recreation.
7. Roll back one node at a time to the previous reference and re-prove readiness.
8. Remove only the unique disposable Compose project, its volumes, and private
   test deployment state.

## Safety And Readiness

API secrets are distinct, generated outside images and environment values, and
copied from Compose mounts to private runtime files before daemon start. The API
and observability listener are never published.

The local scenario provisions a real isolated realm issuer and unique
subject-bound signed discovery/data grants through the stopped-node workflow.
Issuer state is separate from node stores; per-node capability/replay keys are
distinct. Every node must reach product readiness and the peers must join the
canonical Waku network. The scenario fails on missing/invalid authority,
plaintext fallback, shared node secrets, or network-only degraded readiness.

## Related Tests

- `tests/ci/deployment-gate.ps1`

## False Positive Risk

Container health alone could pass while product readiness or local surfaces are
broken. The gate parses all node status results and requires readiness, joined
peer truth, and responsive workload/data projections.

## False Negative Risk

Image construction and Waku formation can vary with host scheduling. Every wait
is bounded by the declared timeout, and resource snapshots distinguish slow
execution from CPU, memory, or disk exhaustion.
