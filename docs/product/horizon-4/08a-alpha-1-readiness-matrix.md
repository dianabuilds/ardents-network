# H4-alpha-1 readiness matrix

Status: **active H4-8A candidate matrix as of 2026-08-27. It records the
evidence required to accept or reject one bounded alpha candidate; no row
currently authorizes a public release claim.**

## Candidate identity and claim

| Field | Value |
|---|---|
| Source revision | `70bf425ef427188694232a6ea873ac3c10f4b5fd` |
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
| A1 | Source integrity: format, architecture, build, unit, process, race, static analysis, and called-vulnerability scan | `make check` at the candidate revision | Passed on 2026-08-27 at `70bf425ef427188694232a6ea873ac3c10f4b5fd` |
| A2 | Ubuntu first-run: real command byte, pin check before execute, non-lingering user unit, start/stop/restart and retained protected state | `make qualification-h4-1a` on a clean unprivileged Ubuntu user session | Passed on 2026-08-27 at `70bf425ef427188694232a6ea873ac3c10f4b5fd` on a clean unprivileged Ubuntu 22.04 VPS user session with `systemd` `249`, `Linger=no`, and Go `1.26.6`; the official runner passed in 1.712 s. This is native Linux user-session evidence, not a GUI or participant enrollment claim. |
| A3 | Ubuntu replacement: Release-authorized successor, self-test, interruption and no ambient updater | `make qualification-h4-1b` on the same selected Linux profile | Passed on 2026-08-27 at `70bf425ef427188694232a6ea873ac3c10f4b5fd` on that clean Ubuntu 22.04 VPS user session; the official native replacement and retained-program authorized rollback runner passed in 4.120 s. |
| A4 | Artifact provenance: immutable published artifact, manifest/digest inventory, and independently delivered first-contact pin | Concrete artifact URL/digest, authenticated Product Owner direct-message contact record, H4-1 enrollment run, and the [closed-alpha release ceremony](../../development/closed-alpha-release-ceremony.md) | Active: the contact class is selected, but no immutable artifact or independently received/enacted enrollment record exists yet |
| A5 | H4-6A control: cached restart plus two fresh enrolled Endpoint state roots obtain the same catalog/Release/Network verdict | `make qualification-h4-3b-docker` | Passed locally and repeated in A7 on 2026-08-27 for `reference-c2-linux-amd64` SHA-256 `ad0c405a81ae312459566f35445bf40b561c91eec8019d52a5a72b551927cf7b`; the controlled fixture gives cached-restart and two-fresh-root agreement. Concrete published component identities remain A4 inputs. |
| A6 | Dynamic H4-3B: all four HTTP/1.1 success/failure terminal cases, exact no-fallback, and Linux alpha-corpus floor; companion alpha-origin resource cell enforces `16 KiB` request/response heads, `1 MiB` request/response bodies, `1 s` header read, and `5 s` browser keep-alive idle ceiling | `make qualification-h4-3b-docker` | Passed locally and repeated in A7 on 2026-08-27 at 1 vCPU/1 GiB/128 PIDs. The C-2 cells covered normal dynamic HTTP/1.1, withdrawal, Publisher Application reset, and Publisher Endpoint loss; the separate local-origin cells covered known/chunked body, oversized-head, header-timeout, and idle keep-alive enforcement. The resource cell is not a second two-Endpoint journey. |
| A7 | Constrained VPS Docker repetition of A5/A6 with exact uploaded byte digests, host envelope, and residue check | `make qualification-h4-3b-vps` with `ARDENTS_H4_3B_VPS` and `ARDENTS_H4_3B_SSH_KEY` | Passed on 2026-08-27 at `8d5c88513bf77d891c4b2ff109323d0f837ce272`: Linux `5.15.0-185-generic`, `x86_64`, 4 vCPU, Docker `29.4.1`, `golang:1.26.6` image `sha256:0d1d3a794be25f809dd2cb3160d8c73276c4056a9f8242a138e908ddeee7b6b6`; all four C-2 cases, 13 HTTP-limit cases, and two H4-6A reader cases passed, followed by a clean residue check. |
| A8 | Full H4-3B two-host journey: Publisher and User endpoints on distinct hosts, with exact Target, dynamic workload, withdrawal and one declared loss case | `make qualification-h4-3b-multihost` and the topology in `tests/qualification/h4-3b-multihost/README.md` | Passed on 2026-08-27: Windows User side and independent Linux VPS Publisher side completed dynamic HTTP, withdrawal, Publisher Application reset, and Publisher Endpoint loss in 116.5 s total, followed by a clean residue check. The stage was built from `f41f4def33e0bfecd9ed9342137d3784fb985e1f`; the later `8d5c8851` change is confined to the A7 Windows VPS runner. |
| A9 | Selected browser/platform: browser observes the dynamic workload and failure state at the candidate's loopback origin | Windows + Firefox `154.0.1`, clean-profile runner | Passed on 2026-08-27 at `8d5c88513bf77d891c4b2ff109323d0f837ce272`: endpoint observation, dynamic Firefox C-2 flow, and following no-Firefox C-2 leg passed. This selection is browser-observation only; it does not claim a Windows H4-1 lifecycle or participant Browser Entry. |
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
