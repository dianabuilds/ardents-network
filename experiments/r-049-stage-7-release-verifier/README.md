# R-049 Stage 7 release-verifier experiment

This disposable experiment answers
[R-049](../../docs/research/records/r-049-stage-7-release-verifier.md): which
maintained Go TUF client and bounded profile may back the Stage 7 Release
Decision Module without importing repository/signing authority?

It is not maintained Ardents code. All cloned sources, module/build caches,
binaries, generated repositories, conformance environments, raw output, and
evidence remain in an owned system-temporary directory outside Git. The root
`go.mod` is unchanged and this directory contains no nested module.

## Frozen candidates and sources

Inputs were frozen before candidate execution on 2026-08-20:

| Input | Version | Commit | Role |
|---|---|---|---|
| `theupdateframework/go-tuf/v2` | `v2.4.2` | `f5edbde31e5507f46db2069402dc38903fe6d9d4` | O1 client candidate |
| `DataDog/go-tuf` | `v1.1.1-0.5.2` | `8c61504ba9faabf79d42a801a350464863a601e7` | O2 maintained legacy-fork candidate |
| `theupdateframework/tuf-conformance` | `v2.4.0` | `500c525c9ce287a472fd334fe8d885cace667d32` | independent client-workflow corpus |
| TUF specification | `1.0.36` | published specification | normative workflow |

The current official TUF implementation list names `go-tuf` as its Go reference
implementation. O2 is evaluated because it remains a real downstream Go client,
not because a second independent reference implementation was found.

## Hypotheses and pre-result falsifiers

- **H1:** O1 plus an Ardents-owned profile can pass the client workflow and
  expose only metadata verification, bounded fetching, and trusted target
  lookup.
- **H2:** O2 has equal conformance and misuse resistance with a smaller safer
  closure.
- **H0:** neither candidate meets the complete R-049 contract.

The exact R-049 falsification criteria were frozen before this experiment. A
candidate is rejected on one mandatory workflow misclassification, trusted-root
gap/reuse/one-sided acceptance, target/path escape, unbounded input handling,
unpatched called high/critical advisory, signing/repository authority required
by the client closure, incompatible license, cgo/unsafe requirement, or failure
of the `2 s`/`128 MiB` maximum-metadata envelope. Results are conjunctive; no
score or popularity can offset a falsifier.

## Environment and exact run

Use Go `1.26.x`, Python `3`, Git, and `govulncheck`. Create an owned scratch root
outside the repository. The online preparation step clones the three exact
commits and downloads their declared test dependencies into scratch-local
caches. No candidate performs a runtime download after preparation.

```powershell
$scratch = Join-Path $env:TEMP 'ardents-r049-release-verifier'
New-Item -ItemType Directory -Force -Path $scratch
git -c core.autocrlf=false clone --branch v2.4.2 --depth 1 https://github.com/theupdateframework/go-tuf.git "$scratch/go-tuf-v2"
git -c core.autocrlf=false -c core.longpaths=true clone --branch v1.1.1-0.5.2 --depth 1 https://github.com/DataDog/go-tuf.git "$scratch/datadog-go-tuf"
git -c core.autocrlf=false clone --branch v2.4.0 --depth 1 https://github.com/theupdateframework/tuf-conformance.git "$scratch/tuf-conformance"
git -C "$scratch/go-tuf-v2" rev-parse HEAD
git -C "$scratch/datadog-go-tuf" rev-parse HEAD
git -C "$scratch/tuf-conformance" rev-parse 'v2.4.0^{}'
```

Run each upstream Go suite with an empty scratch-local module/build cache, then
record `go list -m all`, `go list -deps`, `go version -m`, license inventory,
test duration, binary size, and `govulncheck` output. Run the conformance suite
against each candidate CLI only after verifying that its adapter implements the
suite's `init|refresh|download` protocol without changing decision behavior.

Run the build-ignored Ardents profile tests from a temporary module so the root
module remains unchanged:

```powershell
$profile = Join-Path $scratch 'profile'
New-Item -ItemType Directory -Force -Path $profile
Copy-Item ./experiments/r-049-stage-7-release-verifier/*.go $profile
Push-Location $profile
go mod init ardents.local/r049
go get github.com/theupdateframework/go-tuf/v2@v2.4.2
go get golang.org/x/crypto@v0.52.0 golang.org/x/sys@v0.45.0 golang.org/x/term@v0.43.0
$env:GODEBUG = 'rsa1024min=0'
go test ./profile.go ./profile_test.go -count=10
go test ./profile.go ./profile_test.go -run '^$' -bench '^Benchmark(VerifiedProfileEnvelope|ProfileEnvelope)$' -benchmem
Pop-Location
```

The profile corpus owns public synthetic keys and metadata only. It contains no
Ardents Authority, private release key, credential, real update source, User
history, or production root.

## Expected measurements and retained evidence

Record candidate commit/tag, Go directive, complete module graph, licenses,
test/conformance counts, reachable advisories, source packages imported by the
client path, cgo/unsafe facts, maximum fetch count/bytes, wall time, allocations,
peak RSS where measurable, writes under and outside the owned cache root, and
every expected/actual classification. Retain only this small harness and the
summarized measurements in R-049; raw clones, caches, binaries, and logs remain
external and disposable.

## Result and disposition

Executed 2026-08-20. Generated repositories and binaries remained outside Git;
the candidate owned no runtime network client or writable cache.

| Check | O1 `go-tuf/v2 v2.4.2` | O2 `DataDog/go-tuf v1.1.1-0.5.2` |
|---|---|---|
| source identity | tag and commit matched | tag and commit matched |
| current upstream suite | pass on Windows and Linux | `81/86` client cases and `12/13` file-store cases passed on Linux |
| TUF conformance | `108/108`, no xfail | no current compatible adapter/equivalent evidence |
| client-path scan | zero called vulnerabilities | zero called findings are insufficient to offset test/conformance failure |
| maintenance/API | current v2 client | deprecated legacy design/fork |
| disposition | recommend with frozen profile | reject |

The frozen profile passed ten Windows repetitions and ten Linux repetitions in
a no-network, read-only, no-cgo container limited to one CPU, `128 MiB`, `64`
PIDs, and no capabilities. The Linux process high-water mark was at most
`21,360 kB`; each conservative full test completed in `0.36–0.51 s`. Three
three-second Windows benchmark runs measured `35.52–41.29 ms/op` for the full
verified maximum envelope and at most `0.428 ms/op` for shape plus artifact
verification. The raised-closure Linux no-cgo test binary was `16,514,037`
bytes.

The client build path used `12` modules and `55` non-standard-library packages,
with Apache-2.0, MIT, or BSD-3-Clause licenses and no cgo requirement. After
raising `x/crypto`, `x/sys`, and `x/term` to `v0.52.0`, `v0.45.0`, and
`v0.43.0`, upstream tests and ten profile repetitions still passed;
`govulncheck` found no symbol or imported-package vulnerability. Its sole
module-only finding was unimported `x/crypto/openpgp`.

O1 is therefore proposed for Product Owner acceptance under the exact profile
in `profile.go`. The experiment does not authorize root `go.mod` or maintained
Stage 7 code changes. Because candidate cache is disabled, the maintained
Release Decision Module must independently compare durable `version + digest`
floors for root/timestamp/snapshot/top-level targets and publish the complete
verified root chain plus successor floors before `release-accepted`; cache is
never a watermark. R-054 owns that serialization. R-049 remains `review`, and
S7.1 stays closed until the joint decision and the Stage 7 entry gate are
recorded.
