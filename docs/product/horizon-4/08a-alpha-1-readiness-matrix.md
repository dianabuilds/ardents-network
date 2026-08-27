# H4-alpha-1 readiness matrix

Status: **active H4-8A candidate matrix as of 2026-08-27. It records the
evidence required to accept or reject one bounded alpha candidate; no row
currently authorizes a public release claim.**

## Candidate identity and claim

| Field | Value |
|---|---|
| Source revision | `70bf425eec937edcc22e8f0534db992aa2002a16` |
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
| A1 | Source integrity: format, architecture, build, unit, process, race, static analysis, and called-vulnerability scan | `make check` at the candidate revision | Passed on 2026-08-27 in clean detached source `70bf425eec937edcc22e8f0534db992aa2002a16`: base checks, full process/e2e suite, and race suite ran sequentially. Retained external evidence: `C:\Users\vitek\Ardents-Release\evidence\h4-alpha-1-a1-70bf425e-final.stdout.log`; exit receipt `h4-alpha-1-a1-70bf425e-final.exitcode` is `0`. An earlier overlapping attempt was explicitly interrupted and is not evidence. |
| A2 | Ubuntu first-run: real command byte, pin check before execute, non-lingering user unit, start/stop/restart and retained protected state | `make qualification-h4-1a` on a clean unprivileged Ubuntu user session | Passed on 2026-08-27 from the exact detached candidate `70bf425eec937edcc22e8f0534db992aa2002a16`, copied without Git history into a new `ardentsh4a` user on project-controlled Ubuntu `24.04.3 LTS`, Linux `6.8.0-134-generic`, x86-64, 1 vCPU. Its live `systemd --user` session had `/run/user/1000` and `linger=no`; the real test completed in `110.797s`. Retained external evidence: `C:\Users\vitek\Ardents-Release\evidence\h4-alpha-1-a2-70bf425e-vps2.stdout.log` ends in the successful Go test result. This qualifies the specified Linux user-session profile only, not a participant enrollment or an independently operated host. |
| A3 | Ubuntu replacement: Release-authorized successor, self-test, interruption and no ambient updater | `make qualification-h4-1b` on the same selected Linux profile | Passed on 2026-08-27 from the exact detached candidate `70bf425eec937edcc22e8f0534db992aa2002a16`, in a separate new `ardentsh4b` Ubuntu `24.04.3 LTS` user session with `/run/user/1001` and `linger=no`. Its own quality cache avoided cross-user state; replacement plus failed-candidate recovery/rollback completed in `114.057s`. Retained external evidence: `C:\Users\vitek\Ardents-Release\evidence\h4-alpha-1-a3-70bf425e-vps2.stdout.log`; exit receipt `h4-alpha-1-a3-70bf425e-vps2.exitcode` is `0`. This is project-controlled external-host evidence, not an ambient updater, independent operation, or a participant release. |
| A4 | Artifact provenance: immutable published artifact, manifest/digest inventory, and independently delivered first-contact pin | Concrete artifact URL/digest, authenticated Product Owner direct-message contact record, H4-1 enrollment run, and the [closed-alpha release ceremony](../../development/closed-alpha-release-ceremony.md) | Active: the contact class is selected, but no immutable artifact or independently received/enacted enrollment record exists yet |
| A5 | H4-6A control: cached restart plus two fresh enrolled Endpoint state roots obtain the same catalog/Release/Network verdict | `make qualification-h4-3b-docker` | Passed as controlled fixture evidence on 2026-08-27 through exact-candidate A7: `TestAlphaControlReaderVerifiesPinnedBundleAndCachedRestart` and `TestAlphaControlReaderTwoFreshEnrolledEndpointsAgree` both passed for Endpoint SHA-256 `33473599f7902508d1ca9cb9d09eb6777aff05d9c7c652e96f841b196bfd1fe1` and control SHA-256 `d69b4c5d5f6fae76cbeacfb6acee8abaec9b6cbb56afd339982ea6d55ef9449c`. Concrete signed component identities remain A4 static-input work; this is not their publication evidence. |
| A6 | Dynamic H4-3B: all four HTTP/1.1 success/failure terminal cases, exact no-fallback, and Linux alpha-corpus floor; companion alpha-origin resource cell enforces `16 KiB` request/response heads, `1 MiB` request/response bodies, `1 s` header read, and `5 s` browser keep-alive idle ceiling | `make qualification-h4-3b-docker` | Passed on 2026-08-27 through exact-candidate A7 at 1 vCPU/1 GiB/128 PIDs: normal dynamic HTTP/1.1, withdrawal, Publisher Application reset, and Publisher Endpoint loss all passed; 13 exact head/body/timeout/idle-limit cases also passed. It is artifact-byte behavior evidence, not A4 enrollment/provenance or a second two-Endpoint journey. |
| A7 | Constrained VPS Docker repetition of A5/A6 with exact uploaded byte digests, host envelope, and residue check | `make qualification-h4-3b-vps` with `ARDENTS_H4_3B_VPS` and `ARDENTS_H4_3B_SSH_KEY` | Passed on 2026-08-27 at `70bf425eec937edcc22e8f0534db992aa2002a16`: Linux `5.15.0-185-generic`, `x86_64`, 4 vCPU, `MemAvailable=5660196 kB`, Docker `29.4.1`, `golang:1.26.6` image `sha256:0d1d3a794be25f809dd2cb3160d8c73276c4056a9f8242a138e908ddeee7b6b6`. Exact uploaded Endpoint/control bytes match this profile; all four C-2 cases, 13 HTTP-limit cases, and two H4-6A reader cases passed, followed by runner-owned cleanup. Retained external evidence: `C:\Users\vitek\Ardents-Release\evidence\h4-alpha-1-a7-70bf425e.stdout.log`; exit receipt `h4-alpha-1-a7-70bf425e.exitcode` is `0`. |
| A8 | Full H4-3B two-host journey: Publisher and User endpoints on distinct hosts, with exact Target, dynamic workload, withdrawal and one declared loss case | `make qualification-h4-3b-multihost` and the topology in `tests/qualification/h4-3b-multihost/README.md` | Passed on 2026-08-27 at `70bf425eec937edcc22e8f0534db992aa2002a16`: Windows User side and project Ubuntu VPS Publisher side completed normal dynamic HTTP, withdrawal, Publisher Application reset, and Publisher Endpoint loss. Each cell retained its stage/config digest, local Windows and remote Docker host envelope, and runner-owned cleanup. Retained external evidence: `C:\Users\vitek\Ardents-Release\evidence\h4-alpha-1-a8-70bf425e.stdout.log`; exit receipt `h4-alpha-1-a8-70bf425e.exitcode` is `0`. It remains a project-operated two-host tracer, not independent operation, capacity, or availability evidence. |
| A9 | Selected browser/platform: browser observes the dynamic workload and failure state at the candidate's loopback origin | Windows + Firefox `154.0.1`, clean-profile runner | Passed on 2026-08-27 at `70bf425eec937edcc22e8f0534db992aa2002a16`: Firefox `154.0.1` in a temporary clean profile performed the dynamic C-2 browser flow; the runner then completed the no-Firefox process leg. Retained external evidence: `C:\Users\vitek\Ardents-Release\evidence\h4-alpha-1-a9-70bf425e.stdout.log`; exit receipt `h4-alpha-1-a9-70bf425e.exitcode` is `0`. This is browser observation only: it does not claim a Windows H4-1 lifecycle, participant Browser Entry, DNS/DoH protection, or general browser isolation. |
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
