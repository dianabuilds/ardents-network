# H4-alpha-1 readiness matrix

Status: **historical split-candidate evidence ledger. RC1 has A1-A10; RC2 has
a separate two-fresh-Endpoint control result and accepted A11 campaign
evidence. No immutable candidate has A1-A12. This ledger does not qualify the
post-refactor baseline or authorize a Public Beta claim.**

## Candidate identity and claim

| Field | RC1 | RC2 |
|---|---|---|
| Historical release identity | `h4-alpha-1-rc-1` | `h4-alpha-1-rc-2` |
| Source revision | `70bf425eec937edcc22e8f0534db992aa2002a16` | `2c18bdf92f11f84075915576f595202f48eb05bc` |
| Endpoint SHA-256 | `33473599f7902508d1ca9cb9d09eb6777aff05d9c7c652e96f841b196bfd1fe1` | `b73060105aaed09ed91d77bd560f5a0c7085c5caad41fe0dbea861cdda398e9c` |
| Control SHA-256 | `d69b4c5d5f6fae76cbeacfb6acee8abaec9b6cbb56afd339982ea6d55ef9449c` | `8999004b1074f7c87dcdea004ce46e3ecadc436f3b7364f446731e6b08ccae49` |
| Archive SHA-256 | `e7ff0b26257978fd14bc3583c5de7d36eb7626bac7b43586bcb9442c53f7dba7` | `22acb89ac7abdebf197b8177e9fd84397c0e21316d2ba26991c6e37f25e90d44` |
| Manifest / enrollment pin | `8ed0fd25c60a6988fcc8938baf86547c7c646744f57fb0c39186f184d13afefd` | `1e90db9800efd903e0e0ca58a3f2f54acf4c7c6414df3e8b7c9ab825e0fa2c60` |
| Valid evidence boundary | A1-A10, including fresh/cached/fresh control inspection | separate two-fresh-Endpoint control run with no cached repeat, plus A11 |

Both candidates used the bounded Ubuntu LTS `x86-64` Portable profile,
State-selected TCP/TLS v1 release path, explicit Target Link, and
application-transparent HTTP/1.1 workload. QUIC remained a separately labelled
compatible profile, never a fallback. The control bundle verified separate
Release, Network, and Compatibility components; it made no signed-corpus,
participant Browser Entry, or public naming claim.

Either candidate may establish only a bounded project-operated alpha journey. It
does not establish independent operation, capacity, availability, censorship
resistance, Application-level privacy, public DNS/HTTPS, public Namespace, or
Public Beta.

## Readiness cells

Rows A1-A10 below apply to RC1 only. A11 applies to RC2 under the separately
recorded candidate and harness identities. A12 records closure/disposition work
and supplies no executable-candidate qualification.

Every active row has a checked entry point. A required unavailable environment
is invalid, not a passing skip. An inactive row names the missing decision or
input that prevents it from becoming an executable gate.

| ID | Surface and oracle | Entry point / inputs | State |
|---|---|---|---|
| A1 | Source integrity: format, architecture, build, unit, process, race, static analysis, and called-vulnerability scan | `make check` at the candidate revision | Passed on 2026-08-27 in clean detached source `70bf425eec937edcc22e8f0534db992aa2002a16`: base checks, full process/e2e suite, and race suite ran sequentially. Retained external evidence: `C:\Users\vitek\Ardents-Release\evidence\h4-alpha-1-a1-70bf425e-final.stdout.log`; exit receipt `h4-alpha-1-a1-70bf425e-final.exitcode` is `0`. An earlier overlapping attempt was explicitly interrupted and is not evidence. |
| A2 | Ubuntu first-run: real command byte, pin check before execute, non-lingering user unit, start/stop/restart and retained protected state | `make qualification-h4-1a` on a clean unprivileged Ubuntu user session | Passed on 2026-08-27 from the exact detached candidate `70bf425eec937edcc22e8f0534db992aa2002a16`, copied without Git history into a new `ardentsh4a` user on project-controlled Ubuntu `24.04.3 LTS`, Linux `6.8.0-134-generic`, x86-64, 1 vCPU. Its live `systemd --user` session had `/run/user/1000` and `linger=no`; the real test completed in `110.797s`. Retained external evidence: `C:\Users\vitek\Ardents-Release\evidence\h4-alpha-1-a2-70bf425e-vps2.stdout.log` ends in the successful Go test result. This qualifies the specified Linux user-session profile only, not a participant enrollment or an independently operated host. |
| A3 | Ubuntu replacement: Release-authorized successor, self-test, interruption and no ambient updater | `make qualification-h4-1b` on the same selected Linux profile | An initial run was an invalid environment before any Endpoint test started: the default shared quality-cache directory was owned by A2's distinct user, so Go stopped with permission denied and exit `2`. The final run used A3's own user-owned cache and passed on 2026-08-27 from the exact detached candidate `70bf425eec937edcc22e8f0534db992aa2002a16`, in a separate new `ardentsh4b` Ubuntu `24.04.3 LTS` user session with `/run/user/1001` and `linger=no`. Replacement plus failed-candidate recovery/rollback completed in `114.057s`. Retained external evidence: `C:\Users\vitek\Ardents-Release\evidence\h4-alpha-1-a3-70bf425e-vps2.stdout.log`; exit receipt `h4-alpha-1-a3-70bf425e-vps2.exitcode` is `0`. This is project-controlled external-host evidence, not an ambient updater, independent operation, or a participant release. |
| A4 | Artifact provenance: immutable published artifact, manifest/digest inventory, and independently delivered first-contact pin | Concrete artifact URL/digest, Product Owner-authorized authenticated task contact record, H4-1 enrollment run, and the historical R-119/R-120/R-121 custody records | Passed on 2026-08-28. Immutable GitHub prerelease [`h4-alpha-1-rc-1`](https://github.com/dianabuilds/ardents-network/releases/tag/h4-alpha-1-rc-1) binds source revision `70bf425eec937edcc22e8f0534db992aa2002a16`, archive SHA-256 `e7ff0b26257978fd14bc3583c5de7d36eb7626bac7b43586bcb9442c53f7dba7`, and Alpha Enrollment Pin `8ed0fd25c60a6988fcc8938baf86547c7c646744f57fb0c39186f184d13afefd`. GitHub attestation/release and local asset verification passed. The separately retained contact receipt, SHA-256 `58979f4b9a8c7162e129b113ef72748770f0383eed0c7bea2ea2ce49c6b0aa39`, records that the Codex agent, acting under the Product Owner's explicit authorization in the authenticated one-to-one task, delivered the cohort/release/platform, URL, and Pin to the Product Owner self-walkthrough participant. The Product Owner enacted it on a clean unprivileged Ubuntu account. This is a Product Owner walkthrough, not independent external-participant validation. |
| A5 | H4-6A control: cached repeat plus two fresh enrollment-pinned inspection roots obtain the same catalog/Release/Network verdict | Published RC plus the exact `ardents-control-linux-amd64 inspect-bundle` sequence below; fixture coverage remains in `make qualification-h4-3b-docker` | Passed on 2026-08-28 against the immutable A4 bundle. Fresh inspection root A, cached A, and fresh inspection root B accepted the same ACA1 catalog plus the three Release, Network, and Compatibility components; Network epoch `1` and genesis digest `86852e7cef6fc3db842e4415721e2d9de8bb926a700900252dace11fb3ca634e` agreed. These standalone reader roots are physically distinct from Endpoint state and do not claim two Endpoint processes. Endpoint/control SHA-256 remained `33473599f7902508d1ca9cb9d09eb6777aff05d9c7c652e96f841b196bfd1fe1` / `d69b4c5d5f6fae76cbeacfb6acee8abaec9b6cbb56afd339982ea6d55ef9449c`. `corpus.pub` was manifest-pinned as an authority companion; no ACA2/signed-corpus acceptance is claimed. The earlier exact-candidate fixture cases remain regression evidence, not a substitute for this published-input observation. |
| A6 | Dynamic H4-3B: all four HTTP/1.1 success/failure terminal cases, exact no-fallback, and Linux alpha-corpus floor; companion alpha-origin resource cell enforces `16 KiB` request/response heads, `1 MiB` request/response bodies, `1 s` header read, and `5 s` browser keep-alive idle ceiling | `make qualification-h4-3b-docker` | Passed on 2026-08-27 through exact-candidate A7 at 1 vCPU/1 GiB/128 PIDs: normal dynamic HTTP/1.1, withdrawal, Publisher Application reset, and Publisher Endpoint loss all passed; 13 exact head/body/timeout/idle-limit cases also passed. It is artifact-byte behavior evidence, not A4 enrollment/provenance or a second two-Endpoint journey. |
| A7 | Constrained VPS Docker repetition of A5/A6 with exact uploaded byte digests, host envelope, and residue check | `make qualification-h4-3b-vps` with `ARDENTS_H4_3B_VPS` and `ARDENTS_H4_3B_SSH_KEY` | Passed on 2026-08-27 at `70bf425eec937edcc22e8f0534db992aa2002a16`: Linux `5.15.0-185-generic`, `x86_64`, 4 vCPU, `MemAvailable=5660196 kB`, Docker `29.4.1`, `golang:1.26.6` image `sha256:0d1d3a794be25f809dd2cb3160d8c73276c4056a9f8242a138e908ddeee7b6b6`. Exact uploaded Endpoint/control bytes match this profile; all four C-2 cases, 13 HTTP-limit cases, and two H4-6A reader cases passed, followed by runner-owned cleanup. Retained external evidence: `C:\Users\vitek\Ardents-Release\evidence\h4-alpha-1-a7-70bf425e.stdout.log`; exit receipt `h4-alpha-1-a7-70bf425e.exitcode` is `0`. |
| A8 | Full H4-3B two-host journey: Publisher and User endpoints on distinct hosts, with exact Target, dynamic workload, withdrawal and one declared loss case | `make qualification-h4-3b-multihost` and the topology in `tests/qualification/h4-3b-multihost/README.md` | Passed on 2026-08-27 at `70bf425eec937edcc22e8f0534db992aa2002a16`: Windows User side and project Ubuntu VPS Publisher side completed normal dynamic HTTP, withdrawal, Publisher Application reset, and Publisher Endpoint loss. Each cell retained its stage/config digest, local Windows and remote Docker host envelope, and runner-owned cleanup. Retained external evidence: `C:\Users\vitek\Ardents-Release\evidence\h4-alpha-1-a8-70bf425e.stdout.log`; exit receipt `h4-alpha-1-a8-70bf425e.exitcode` is `0`. It remains a project-operated two-host tracer, not independent operation, capacity, or availability evidence. |
| A9 | Selected browser/platform: browser observes the dynamic workload and failure state at the candidate's loopback origin | Windows + Firefox `154.0.1`, clean-profile runner | Passed on 2026-08-27 at `70bf425eec937edcc22e8f0534db992aa2002a16`: Firefox `154.0.1` in a temporary clean profile performed the dynamic C-2 browser flow; the runner then completed the no-Firefox process leg. Retained external evidence: `C:\Users\vitek\Ardents-Release\evidence\h4-alpha-1-a9-70bf425e.stdout.log`; exit receipt `h4-alpha-1-a9-70bf425e.exitcode` is `0`. This is browser observation only: it does not claim a Windows H4-1 lifecycle, participant Browser Entry, DNS/DoH protection, or general browser isolation. |
| A10 | Carrier boundary: selected TCP/TLS path retains C-2 behavior and no fallback; separate QUIC evidence remains labelled as a distinct compatible profile | `make qualification-h4-2-local-emulator` and `make qualification-h4-2-multihost` with the exact candidate path and expected digest | Passed. The local emulator passed on 2026-08-27, including TCP/TLS and separately labelled QUIC C-2, both-direction no-fallback, and held-route/Rendezvous-loss behavior. Evidence is `C:\Users\vitek\Ardents-Release\evidence\h4-alpha-1-a10-local-70bf425e.stdout.log`, exit `0`, SHA-256 `309c86073ce29bed4afde0aea0eec5c27fd94b8b846b39c0effc547cc7c7b99e`. On 2026-08-28 the exact Endpoint byte `33473599f7902508d1ca9cb9d09eb6777aff05d9c7c652e96f841b196bfd1fe1` and product Node byte `3e1120a2dffb32b12d90fd3f6be9bc3ce040f9f1a237179107c8eaec86696711` ran on project Ubuntu `24.04.3`, Linux `6.8.0-134-generic`, x86-64, 1 vCPU, 1 GiB. Signed State digest `4e0368f80a3d1542ca6f8fdc932b15089da8ef75e3d8f6bd7094c494e5b91f0e` supplied one explicit TCP/TLS v1 Rendezvous and no alternate Carrier. Product Initiator/Responder/Rendezvous processes all reported TCP/TLS at READY and owned both authenticated Carrier legs while two exact-candidate Endpoint processes retained matching 8 MiB Application bytes, clean terminals, one Route generation, and zero recovery. Retained product-Route evidence is `C:\Users\vitek\Ardents-Release\evidence\h4-alpha-1-a10-exact-product-route-70bf425e.stdout.log`, exit `0`, SHA-256 `08bc3796d99d4c318e21ba4aa82f8d8aa4343e3f1ffded31184ac37611757df1`; remote cleanup was verified. The separate Node/fault record remains `h4-alpha-1-a10-multihost-70bf425e-exact-candidate.stdout.log`, exit `0`, SHA-256 `feae309c045b589085afdd3e88944d224808c13e5df4ded26efc0da3149a5a48`. Browser Entry and public naming were not configured. This is project-controlled functional evidence, not capacity, availability, hostile-network, or independent-operator evidence. |
| A11 | Soak and fault campaign: declared duration, workload, observer, resource ceilings, state/control expiry and crash/loss cases | exact campaign runner and retained observations | Accepted for RC2 on 2026-08-28: `h4-alpha-1-rc-2-h4-8-a11-attempt-14` completed 6/6 cells and all 10 invocations in 2,462,217 ms, under the 125-minute campaign deadline. Windows plus Ubuntu/Docker observers retained each attempt; the remote container limit was 1 vCPU, 1 GiB, and 128 PIDs. Normal soak ran 1,800 paced cycles; Application, Endpoint, Carrier, product Node, and deterministic expiry cells passed. Remote normal/fault cells ran from harness revision `a7147b04c5e4532b189fc319c96b4791baf48c4b`; the observed Node SHA-256 was `ca4830c6f805efe03fe423d652043ddca3b75f318d23021f7f75d1efa2567ae7`. The exact RC2 Endpoint/control bundle was directly exercised by the expiry-boundaries cell. This does not establish RC2 A1-A10. Evidence: `C:\Users\vitek\Ardents-Release\evidence\h4-alpha-1-rc-2-h4-8-a11-attempt-14\campaign-receipt.json`. |
| A12 | Release closure: owned code/docs, findings, dispositions, retained raw evidence, and participant-visible limitation text | historical closure inventory | The closure record and failed-attempt dispositions are retained. Failed attempt 13 remains a parser-defect disposition rather than being erased. The former aggregate conclusion that RC2 had A1-A12 is withdrawn; A12 supplies no missing candidate qualification evidence. |

## RC1 A5 exact enrolled-control entry point

The successful observation used the manifested control companion and exact
enrolled artifact. The first and second commands share `control-a` to prove a
cached repeat; the third uses empty `control-b` to prove a second fresh root.

```sh
set -eu

control=/home/ardentsalpha1/ardents-alpha/ardents-alpha-h4-alpha-1-rc-1-linux-amd64/ardents-control-linux-amd64
enrollment=/home/ardentsalpha1/.local/state/ardents-alpha/h4-alpha-1-rc-1/alpha-enrollment.json
artifact=/home/ardentsalpha1/ardents-alpha/ardents-alpha-h4-alpha-1-rc-1-linux-amd64/ardents-linux-amd64
at=2026-08-28T00:54:15Z
root_a=/home/ardentsalpha1/.local/state/ardents-alpha/h4-alpha-1-rc-1/control-a
root_b=/home/ardentsalpha1/.local/state/ardents-alpha/h4-alpha-1-rc-1/control-b

test ! -e "$root_a"
test ! -e "$root_b"

"$control" inspect-bundle --enrollment "$enrollment" --artifact "$artifact" \
  --state-root "$root_a" --at "$at"
"$control" inspect-bundle --enrollment "$enrollment" --artifact "$artifact" \
  --state-root "$root_a" --at "$at"
"$control" inspect-bundle --enrollment "$enrollment" --artifact "$artifact" \
  --state-root "$root_b" --at "$at"
```

Those two paths were absent before the retained run. A later repetition must
choose two new absent owner-controlled paths; it must not delete an earlier
inspection root merely to manufacture a fresh result.

## RC1 A4/A5 retained execution evidence

The following files are retained outside Git under
`C:\Users\vitek\Ardents-Release\evidence` and contain no private key or
passphrase. A failed attempt remains listed: a later successful rerun never
erases an earlier failure.

| External evidence file | Observed result | SHA-256 |
|---|---:|---|
| `h4-alpha-1-rc-1-input-request.json` | accepted input | `c397544a5d9c8adc811ed5f34b15978b0c315b60ea7692b115fdffdbdbacc36d` |
| `h4-alpha-1-rc-1-alpha-inputs-receipt.json` | verifier preflight accepted | `f143400b7c083971995232d887b6760a7711f75ff3b90a32d607bdac92e816e1` |
| `h4-alpha-1-rc-1-public-release-receipt.json` | deterministic archive accepted | `b9f0940d505ecfd19755ace159ce3a9f47931da4d9098e94b9a9a633a2ccce28` |
| `h4-alpha-1-rc-1-contact-receipt.json` | PO-authorized Codex-to-Product-Owner self-walkthrough handoff; limitation recorded | `58979f4b9a8c7162e129b113ef72748770f0383eed0c7bea2ea2ce49c6b0aa39` |
| `h4-alpha-1-rc-1-attempt-dispositions.json` | both failed attempts, oracles, diagnoses, corrections, and accepted reruns | `927974787eff6b7d2c596efde358364a9e1ccaca70a888906723e551a258af43` |
| `h4-alpha-1-rc-1-locale-sort-reproduction.stdout.log` | exit `0`; `en_US.utf8` reproduced the legacy inventory mismatch and explicit `LC_ALL=C` fixed it | `77585b5fa9590a51b4e548167ce7ba393436f3b2c6d5f498d9c0b49441ba1c2c` |
| `h4-alpha-1-rc-1-github-release-verification.stdout.log` | exit `0`; immutable release and asset attestation verified | `6fc21ff8d412aa089358220a53d94da680e2a5704bef5db0b44df18e1a447339` |
| `h4-alpha-1-rc-1-enrollment-preexecution.stdout.log` | exit receipt `h4-alpha-1-rc-1-enrollment-preexecution.exitcode` is `1`, SHA-256 `4355a46b19d348dc2f57c046f8ef63d4538ebb936000f3c9ee954a27460dd865`; locale disposition is in the retained disposition record | `06426ab67e98d916d9c12b38fc1b2814af7b072094ae4377a14c1960320a58ee` |
| `h4-alpha-1-rc-1-enrollment-preexecution-final.stdout.log` | exit receipt `h4-alpha-1-rc-1-enrollment-preexecution-final.exitcode` is `0`, SHA-256 `9a271f2a916b0b6ee6cecb2426f0b3206ef074578be55d9bc94f6f3fe3ab86aa`; Pin, exact inventory, and every manifest byte accepted | `5326ca427f97f97ecad8ed011766e7830ddacc5cf786a5ddcf45660708227877` |
| `h4-alpha-1-rc-1-enrollment-check.stdout.log` | exit receipt `h4-alpha-1-rc-1-enrollment-check.exitcode` is `0`, SHA-256 `9a271f2a916b0b6ee6cecb2426f0b3206ef074578be55d9bc94f6f3fe3ab86aa`; Endpoint enrollment check accepted | `dd1a01d211684e49dafb77ec552cf8fc9a7aeffe06b01bf68bf774a532e71e94` |
| `h4-alpha-1-rc-1-first-enrollment-start.stdout.log` | exit receipt `h4-alpha-1-rc-1-first-enrollment-start.exitcode` is `0`, SHA-256 `9a271f2a916b0b6ee6cecb2426f0b3206ef074578be55d9bc94f6f3fe3ab86aa`; non-lingering user unit reached ready | `e496a67ecc0fc189a2029baab090cbd0b0781d712e15994fd6b30bbf4ccc1a18` |
| `h4-alpha-1-rc-1-h4-6a-control.stdout.log` | exit receipt `h4-alpha-1-rc-1-h4-6a-control.exitcode` is `0`, SHA-256 `9a271f2a916b0b6ee6cecb2426f0b3206ef074578be55d9bc94f6f3fe3ab86aa`; fresh-A/cached-A/fresh-B inspection verdict accepted | `56e235dc7bdd6389b75f2920a07bc24b8542823244feb93c2cc0b3c6b70f4d31` |
| `h4-alpha-1-rc-1-lifecycle.stdout.log` | exit receipt `h4-alpha-1-rc-1-lifecycle.exitcode` is `1`, SHA-256 `4355a46b19d348dc2f57c046f8ef63d4538ebb936000f3c9ee954a27460dd865`; observer disposition is in the retained disposition record | `ef64adabc314ba4588c2a55431138314829a7004d101c6da547fc986fb8c38e9` |
| `h4-alpha-1-rc-1-lifecycle-final.stdout.log` | exit receipt `h4-alpha-1-rc-1-lifecycle-final.exitcode` is `0`, SHA-256 `9a271f2a916b0b6ee6cecb2426f0b3206ef074578be55d9bc94f6f3fe3ab86aa`; stop/restart/state-retention/final-stop accepted | `c31f1aa75eba0f492fb20f2105888d2172f43c21fcfc1f42f273042389ac5393` |
| `h4-alpha-1-rc-1-post-session-cleanup.stdout.log` | exit receipt `h4-alpha-1-rc-1-post-session-cleanup.exitcode` is `0`, SHA-256 `9a271f2a916b0b6ee6cecb2426f0b3206ef074578be55d9bc94f6f3fe3ab86aa`; inactive, disabled, no exact process, retained state | `96c4682581437b0deafeda2527f38a91c08cefe8507a76d4d71470e224ef3e13` |
| `h4-alpha-1-rc-1-vps-access-cleanup.stdout.log` | exit `1`; exact user, home, and temporary `authorized_keys` entry removal completed, then the final line-count observer had a formatting error | `bdb8d3f3c021dcab2820a8087a0816624f35764e69e4fb1f67c14fd4c1f4bd70` |
| `h4-alpha-1-rc-1-vps-access-negative-verification.stdout.log` | exit `1` as expected; a batch-mode SSH authentication attempt using the corresponding private key received `Permission denied` | `bd2a98b2d47c65d724d4b13a4b112f82981e9633b98af411a4d197e1f7cdf32c` |
| `h4-alpha-1-rc-1-vps-access-cleanup-receipt.json` | accepted cleanup with the post-removal observer failure and expected negative-access result retained separately | `80071e9abfa87f34141ec341868c65ca3f807e34e34f0e85f3302939cbe9672a` |

## Promotion rule

The A1-A10 bounded-alpha condition was met for RC1 only. A11 was not run for
RC1. RC2's A11 campaign and separate two-fresh-Endpoint control run do not
establish RC2 A1-A10; the latter has no cached repeat and is not matrix cell A5.
Neither historical candidate has A1-A12. A9 cannot be substituted by a local
Go HTTP client, and no result authorizes a Public Beta claim.
If a candidate byte, platform, Carrier, control input, topology, or workload
changes, the affected cells become pending again.

Public Beta is outside this matrix: it additionally requires the independent
control, operator-capacity, external-review, and other gates in the H4 briefs.
