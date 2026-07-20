# STB-307 Multi-Host Recovery Evidence

Status: completed and accepted on 2026-07-19.

## Implemented

- `docker/docker-compose.testnet.yml` defines seven real Ardents nodes across
  two isolated network segments, including dual-homed bootstrap/bridge nodes,
  TCP and WSS paths, and a constrained Store-recovery client.
- `tests/run-multihost.ps1` injects restart, address change, bootstrap loss,
  partition/rejoin, restricted-defense, peer churn, WSS-only participation,
  and Store recovery failures while retaining snapshots, logs, versions, and
  container resource measurements.
- `tests/cmd/store-probe` starts fresh constrained clients against the real
  Waku provider path. A publisher must confirm Store retention after Lightpush
  before a second client retrieves the opaque envelope across the other
  network segment; Lightpush acknowledgement alone is not treated as delivery.
- the runner builds the Linux testnet image when missing and retains explicit
  build-mode and parallelism overrides.

## Static Validation

- PowerShell parser validation: passed.
- `docker build --check --build-arg GO_BUILD_PARALLELISM=1`: passed with no
  Dockerfile warnings.
- `docker compose -f docker/docker-compose.testnet.yml config --quiet`: passed.

## Resolved Host Performance Incident

Three earlier local cold-build attempts produced unacceptable host pressure:

1. the initial runner build did not complete within ten minutes;
2. a direct BuildKit build raised `vmmemWSL` to about 21 GiB and left orphaned
   `docker-buildx` clients after cancellation;
3. the resource-limited retry (`GOMAXPROCS=1`, `go build -p=1`) still caused
   about 75 percent total host CPU, with WMI Provider Host near 19 percent and
   Microsoft Defender near 12 percent.

The running `social-*` Docker services use less than 0.5 GiB combined and must
not be interrupted. Stopping the orphaned build clients reduced `vmmemWSL`
from about 21 GiB to about 6.8 GiB and returned sampled background CPU to normal.
The root cause was an unbounded Docker build context: the repository had no
`.dockerignore`, so local Go caches, reports, runtime data, and IDE state were
submitted to BuildKit and rescanned by Windows Defender/WMI. A second issue was
an empty `.git` marker that caused orphaned Git probes. The build context is now
bounded, Git provenance probing is disabled for local builds, and canonical
tests run inside Linux with Docker-managed Go caches.

The first cold container fast-suite validation passed with host memory below
48 percent, `vmmemWSL` below 6 GiB, and about 230 GiB free disk remaining.

## Accepted Runtime Evidence

The retained canonical report is
`tests/.artifacts/reports/stb-307-multihost-container`.

- result: 13 passed, 0 failed;
- topology: seven Ardents nodes on `zone_a` and `zone_b`;
- toolchain: Docker 29.1.3, Compose 2.40.3, Go 1.26.5 linux/amd64;
- segment partition retained healthy local participation in `steady` mode;
- isolated `b1` transitioned to degraded `restricted_defense` with Relay as
  its only active provider capability;
- rejoin restored `b1` to ready `steady` mode with Relay, Store, Filter, and
  Lightpush provider capabilities;
- three peer-churn restarts recovered;
- the fresh constrained Store client recovered the retained opaque envelope
  across the dual-homed bridge;
- retained container samples were 24-37 MiB RAM per running node and no more
  than 2.3 percent CPU in the final sample.

The probe validates the real Waku opaque carrier path. It does not claim that
an unprovisioned daemon has a private discovery capability: those nodes remain
truthfully degraded with `privacy.capability.missing`, and product deployment
must provision the Identity-owned capability rather than fall back to plaintext.

## Security And Acceptance

- test API tokens, CA private key, and WSS private keys are created in a unique
  system temp directory and removed after Compose teardown;
- retained reports contain no `secrets` directory or PEM private-key marker;
- canonical Docker `fast` suite passed after the harness changes;
- the Store probe compiled in Linux and passed the Go code-size guard;
- the acceptance gate passed because both healthy and degraded/recovery paths
  are observable and the delivered path remains Waku-backed.

## Reproduction

Run locally or on CI through the canonical Docker paths:

```powershell
tests/run.ps1 fast
tests/run-multihost.ps1 -BuildMode IfMissing
```

Slow execution is a trigger for resource diagnosis, not a reason to stop the
development loop.
