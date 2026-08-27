# H4-alpha-1 readiness matrix

Status: **active H4-8A candidate matrix as of 2026-08-27. It records the
evidence required to accept or reject one bounded alpha candidate; no row
currently authorizes a public release claim.**

## Candidate identity and claim

| Field | Value |
|---|---|
| Source revision | `ba3206a7a171bd219b0809f075c5b13279d49a84` |
| Endpoint profile | Ubuntu LTS `x86-64`, unprivileged Portable, explicit Alpha Enrollment Pin |
| Release-gating Carrier | State-selected TCP/TLS v1; QUIC v1 is maintained separately and cannot be used as a fallback |
| Service workload | H4-3B application-transparent HTTP/1.1: POST body/header preservation, cookie/redirect follow-up, chunked response, withdrawal, Publisher Application reset, and Publisher Endpoint loss |
| Destination path | One explicit Target Link through one loopback Browser Adapter origin; no participant Browser Entry or public naming claim |
| Alpha control | enrollment-pinned H4-6A catalog with independently verified Release, Network, Compatibility, and corpus inputs |

The candidate may establish only a bounded project-operated alpha journey. It
does not establish independent operation, capacity, availability, censorship
resistance, Application-level privacy, public DNS/HTTPS, public Namespace, or
Public Beta.

## Readiness cells

Every active row has a checked entry point. A required unavailable environment
is invalid, not a passing skip. An inactive row names the missing decision or
input that prevents it from becoming an executable gate.

| ID | Surface and oracle | Entry point / inputs | State |
|---|---|---|---|
| A1 | Source integrity: format, architecture, build, unit, process, race, static analysis, and called-vulnerability scan | `make check` at the candidate revision | Active; must pass for every new candidate |
| A2 | Ubuntu first-run: real command byte, pin check before execute, non-lingering user unit, start/stop/restart and retained protected state | `make qualification-h4-1a` on a clean unprivileged Ubuntu user session | Active; awaiting a recorded run for this candidate |
| A3 | Ubuntu replacement: Release-authorized successor, self-test, interruption and no ambient updater | `make qualification-h4-1b` on the same selected Linux profile | Active; awaiting a recorded run for this candidate |
| A4 | Artifact provenance: immutable published artifact, manifest/digest inventory, and independently delivered first-contact pin | Concrete artifact URL/digest, contact record, and H4-1 enrollment run | Inactive: artifact and independent contact class are not yet declared |
| A5 | H4-6A control: cached restart plus two fresh enrolled Endpoint state roots obtain the same catalog/Release/Network verdict | `make qualification-h4-3b-docker` | Passed locally on 2026-08-27 for the current fixture byte (`reference-c2-linux-amd64` SHA-256 `7cdb4a1fdeca2310f4a26b64ef875aafbcdb4353346f28d9e95931b3c3896ccb`); real component identities remain A4/A7 inputs |
| A6 | Dynamic H4-3B: all four HTTP/1.1 success/failure terminal cases, exact no-fallback, and Linux alpha-corpus floor | `make qualification-h4-3b-docker` | Passed locally on 2026-08-27 at 1 vCPU/1 GiB/128 PIDs for the current fixture byte; it covered normal dynamic HTTP/1.1, withdrawal, Publisher Application reset, and Publisher Endpoint loss |
| A7 | Constrained VPS Docker repetition of A5/A6 with exact uploaded byte digests, host envelope, and residue check | `make qualification-h4-3b-vps` with `ARDENTS_H4_3B_VPS` and `ARDENTS_H4_3B_SSH_KEY` | Active; pending the A5/A6 rerun and a recorded run on the declared VPS |
| A8 | Full H4-3B two-host journey: Publisher and User endpoints on distinct hosts, with exact Target, dynamic workload, withdrawal and one declared loss case | `make qualification-h4-3b-multihost` and the topology in `tests/qualification/h4-3b-multihost/README.md` | Active; the runner is compiled locally, awaiting its first declared VPS execution |
| A9 | Selected browser/platform: browser observes the dynamic workload and failure state at the candidate's loopback origin | selected Endpoint desktop platform, browser/version, clean-profile runner | Inactive: Windows Firefox 154.0.1 passed the temporary-profile dynamic HTTP/1.1 C-2 compatibility runner on 2026-08-27 (and the following no-Firefox C-2 leg passed), but the Product Owner has not selected it as the participant platform |
| A10 | Carrier boundary: selected TCP/TLS path retains C-2 behavior and no fallback; separate QUIC evidence remains labelled as a distinct compatible profile | `make qualification-h4-2-local-emulator` and `make qualification-h4-2-multihost` | Active for Carrier behavior; it does not yet prove full two-host H4-3B |
| A11 | Soak and fault campaign: declared duration, workload, observer, resource ceilings, state/control expiry and crash/loss cases | exact campaign runner and retained observations | Inactive: duration/load/observer contract not yet accepted |
| A12 | Release closure: owned code/docs, findings, dispositions, retained raw evidence, and participant-visible limitation text | H4-8D closure inventory | Inactive: begins after A1–A11 have an accepted outcome |

## Promotion rule

The Product Owner can accept only a bounded alpha after A1–A10 are green for
one immutable candidate, every observed failure has a disposition, and the
candidate's claim text names the surviving limits. A9 and A11 cannot be
substituted by a local Go HTTP client or elapsed wall-clock time. If a candidate
byte, platform, Carrier, control input, topology, or workload changes, the
affected cells become pending again.

Public Beta is outside this matrix: it additionally requires the independent
control, operator-capacity, external-review, and other gates in the H4 briefs.
