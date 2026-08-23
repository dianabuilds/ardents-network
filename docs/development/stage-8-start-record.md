# Stage 8 start record

Status: **accepted; Stage 8 started and S8.0 current-system truth authorized on
2026-08-22.**

## Bound predecessor and entry identity

- Stage 7 disposition: `stopped`; S7.3-S7.7 are cancelled rather than deferred.
- Maintained-source entry commit:
  `1cf7100da3ada32ba53abb51201aaf7b6183a3da`.
- Annotated entry tag: `stage7-stopped-2026-08-22`.
- Integration: local `main` was fast-forwarded to the entry commit without a
  merge commit; no unrelated or superseded side branch was merged.
- First working branch: `codex/s8-0-current-system-truth`.
- Working tree at entry: clean.

The retained Stage 7 source was evidence about the current system, not the
target architecture and not a commitment to complete cancelled Stage 7 work.
The current Stage 8 controls, accepted ADRs, Product Core, threat model, and
technical documentation govern the migration.

## Reproducible toolchain inputs

| Input | Bound value |
|---|---|
| Go toolchain used by the entry check | `go1.26.6 windows/amd64` |
| `GOOS` / `GOARCH` | `windows` / `amd64` |
| Repository `GOTOOLCHAIN` policy | `local` through `Makefile`; ambient value at capture was `auto` |
| `go.mod` SHA-256 | `b12499dbb2cecb6ad205d9cfb377a4d5b9a27930a6fb2335ada5a4c5b8c91066` |
| `go.sum` SHA-256 | `9fec7cf52cbbbef80d34ad9d09d2aceb2207b467a6459f5c063a0bedde1154b4` |
| `Makefile` SHA-256 | `20d15cd96dc43cc5befd6a32bfdb1c51acd7469dc970c7be8fa28ac7c8c0d439` |

The full `make check` passed on the exact entry commit: architecture and format,
build, vet, unit, module integrity, staticcheck, govulncheck with zero invoked
vulnerabilities, cross-process end-to-end tests, and the complete race suite.
Live, multi-platform, adversarial, sustained, and multi-day qualification are
not implied by this diagnostic entry baseline and remain Stage 9 concerns.

## Product Owner authorization

```text
Product Owner authorization: принимаю
Authorization date: 2026-08-22
Authorized stage: Stage 8 productization and restructuring
Authorized first slice: S8.0 freeze, delta audit, and current-system truth
```

## Authorization boundary

S8.0 may inspect the exact entry identity, reproduce inventories outside the
repository, and write factual current-system documentation and decision inputs.
Existing rules remain binding. S8.0 does not itself authorize a target package,
Interface, state/format, product behavior, claim, dependency, or test-policy
mutation. Those changes require the applicable later Stage 8 disposition and
migration wave defined by the accepted brief.
