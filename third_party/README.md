# Third-Party Governance

`third_party/` contains dependency-backed substrate, not product-owned Ardents
domain code.

The active network foundation for `v1` remains Waku-backed, but as of
`2026-03-25` it is no longer wired through the local fork trees in
`third_party/forks/`. Those trees are now inactive fork snapshots kept only as
temporary governance evidence until they are archived or deleted.

## Rules

- No new product logic may be implemented directly inside `third_party/`
  without an explicit governance update and a matching decision entry.
- Direct imports of `github.com/waku-org/go-waku` and
  `github.com/libp2p/go-libp2p` remain restricted to `internal/transport/**`
  and are enforced by `tests/cmd/importguard`.
- Every retained local fork snapshot must keep an explicit manifest with:
  upstream source, pinned baseline, known local delta, owners, update policy,
  and return-upstream decision.
- Dependency-security review for these forks is tracked from
  `docs/process/repository-quality-control/dependency-security-review.md`.
- Fork removal and return-upstream work is tracked from
  `docs/process/repository-quality-control/fork-exit-plan.md`.
- While these snapshots still exist, upstream release drift can be checked
  explicitly through `go run ./tests/cmd/forkwatch`.

## Inactive Fork Snapshots

- `third_party/forks/go-libp2p/FORK.md`
- `third_party/forks/go-waku/FORK.md`
