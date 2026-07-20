# STB-605 Evidence — Repository Onboarding And Release Artifacts

## Accepted Product Properties

- The root README now explains product scope, trust boundaries, Waku-based
  architecture, supported profile, safe quick start, CLI use, test entry points,
  limitations, and security posture.
- Ardents is licensed under the MIT License. The repository and release bundle
  contain the canonical `LICENSE`, and the OCI image declares `MIT`.
- `ard version` and `ardd --version` report the injected version, commit, build
  date, Go version, OS, and architecture without requiring a running node.
- The release workflow produces Linux amd64 CLI/daemon binaries, a tar bundle,
  an OCI-compatible Docker image archive, SHA-256 checksums, CycloneDX SBOM, and
  SLSA/in-toto provenance.
- Operator documentation covers supported platforms, deployment, backup and
  restore, upgrades and migration, key rotation, and incident response.

## Docker/Linux Evidence

| Check | Result |
| --- | --- |
| Targeted build identity packages | 3/3 packages passed in Docker/Linux |
| Two independent release builds | passed; complete `SHA256SUMS` files identical |
| Archive checksum/version smoke | passed in 2.7 seconds |
| Clean Docker image import | passed; version `v0.0.0-stb605`, license `MIT` |
| Clean quick start from extracted bundle | passed in 27.2 seconds |
| Runtime network truth | seed network ready; two peers ready and joined |
| Disposable deployment cleanup | passed; containers, volumes, and network removed |

The accepted reproducible checksums were:

```text
112c33b1c4e21a09213e596868ba8f46c95db1adf6b416c6e302e0785b644082  ard-linux-amd64
4e973026c266c6c8cb390772d20ba91f58600dbae4a50e81b3194c524937e5af  ardd-linux-amd64
8694384a216cdf52f222d3b30cddce4c3565fb7c3d48efadc5a2ca47d98a7601  ardents-v0.0.0-stb605-linux-amd64.docker.tar
601facab05a47928737600b8a0fcd4c2384023c210eb1741fd4341a5fa8d6bed  ardents-v0.0.0-stb605-linux-amd64.tar.gz
b67355c7d0b37c4751316c8bc09c6f101a5b44b957c99b2818f5f6243359b3b1  provenance.intoto.json
48c635597d8bdac66169d93532b45e05cd145956efaac919f5caf6d6aceb1cb4  sbom.cdx.json
```

## Failure Found By The Gate

The first artifact-level quick start terminated at its 90-second deadline
because the local Compose profile exposed its isolated Docker-in-Docker engine
over unauthenticated TCP. Ardents correctly rejected that endpoint under the
runtime security contract. CPU, memory, and disk checks ruled out resource
exhaustion.

The local profile now shares only a Unix socket through a dedicated named
volume. It still never mounts the host Docker socket. A 24.9-second seed-only
regression check proved `healthy` with zero restarts, and the original clean
three-node artifact scenario then passed. Production continues to require an
external TLS-authenticated Docker endpoint and per-node client credentials.

## Truthful Release Boundary

The repository currently has no Git `HEAD`, so this local packaging proof used
the documented all-zero sentinel commit, a fixed source epoch, and
`source_dirty=true`. `scripts/release/build.ps1` rejects dirty release sources by
default; `-AllowDirty` is explicitly limited to local verification. STB-606
must create release candidates only from committed source.

Linux amd64 is the supported release target. Arm64 remains explicitly
unsupported because the canonical Waku/RLN build requires CGO and was not
cross-compiled by disabling it.

The local nodes remain truthfully degraded when no private capability is
provisioned. Network state is real and ready; privacy readiness is not faked.

## Primary Artifacts

- `LICENSE`
- `README.md`
- `CHANGELOG.md`
- `ardents.ps1`
- `scripts/release/build.ps1`
- `scripts/release/bundle.sh`
- `scripts/release/metadata.ps1`
- `scripts/release/smoke.sh`
- `docs/license-decision.md`
- `docs/supported-platforms.md`
- `docs/upgrade-migration.md`
- `docs/operator-runbook.md`
- `docs/incident-response.md`
- `docs/deployment-contract.md`
