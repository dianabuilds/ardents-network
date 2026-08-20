# R-053 Authority-envelope logic prototype

This disposable prototype asks one question: can a single strict password-
derived envelope represent both `authority-vault` and `recovery-bundle`
purposes while making wrong-secret/tamper failure uniform, rejecting KDF
parameter denial-of-service before derivation, and preserving the locked restore
→ strictly-higher reconciliation state machine without Grants or runtime keys?

The answer is about format/state coherence only. The prototype uses synthetic
in-memory material, writes no file, performs no OS vault operation, and cannot
prove crash durability, swap/dump cleanup, secure deletion, weakest-host KDF
latency, or cryptographic implementation suitability. Both Go files are
build-ignored and are not maintained Stage 7 code.

## Hypothesis and falsification

Retain the shared envelope candidate only if manual actions show:

- correct unlock and isolated test verification expose the synthetic authority
  only after AEAD authentication and exact environment binding;
- wrong password and authenticated-byte tamper return the same
  `bundle-unlock-failed` result with no payload;
- an unsupported `512 MiB` parameter returns before any KDF time is recorded;
- restore remains `authority-locked`; stale reconciliation cannot activate;
  strictly higher generation **and** revision creates a fresh runtime-key
  commitment while Grants remain absent; and
- 64/128/256 MiB measurements remain only calibration observations. The final
  profile cannot receive weakest-native-host qualification from the current
  Windows development machine alone.

One unexpected success, partial payload, downgrade, active stale restore,
restored Grant, or reused runtime key falsifies the candidate.

## Run

From the repository root:

```text
make prototype-r053
```

Use `e,u,w,t,x,p,r,s,h` in that order, then repeat `e,u` after selecting `2`
and `3` to compare local KDF latency. Press `q` to exit. The screen redraws after
every action and displays the complete relevant state.

## Result

Executed on 2026-08-20 with Go `1.26.6 windows/amd64` and the root module's
`golang.org/x/crypto v0.51.0`. `profile.go` SHA-256 was
`a74f0270032924b9971d223bff17d4398ecbf0136f278794d9fccb90597f1dfd`;
`main.go` was
`ccd0a41765ca7b71f12cf4c7d8c2e6015b2720936b1ae575504e0307865f918f`.

The synthetic canonical envelope was `1,121–1,122` bytes. Observed single KDF
times were about `27–33 ms` at 64 MiB, `66–69 ms` at 128 MiB, and `134–147 ms`
at 256 MiB. Correct unlock/test verification passed. Wrong password and a
canonical ciphertext-bit mutation both returned `bundle-unlock-failed`; wrong
environment returned `bundle-wrong-environment`; a 512 MiB parameter returned
`bundle-unsupported` with `0 s` KDF time. Restore stayed
`authority-locked`; generation `3`/revision `8` could not advance a restored
`3`/`7` authority, while `4`/`8` activated with a fresh runtime-key commitment
and `grants=false`.

The state/format question passed. The speed result does **not** satisfy the
precommitted weakest-host `0.5–3 s` band on this current development host and
therefore does not qualify any final KDF cost. R-053 retains 256 MiB/t=3/p=4 as
the exact qualification candidate; a sub-0.5-second weakest-host result falsifies it
rather than changing the band after measurement. The selected integration
dependency candidate is `x/crypto v0.52.0`. The same full 64 MiB logic sequence
also passed from a temporary isolated module using exact `v0.52.0` at upstream
commit `a1c0d9929856c8aba2b31f079340f00578eda803` and module checksum
`h1:RMs7fP2rXdep0CftQlK8Uf+kibLm7qkCcradZWYz988=`; the TUI read that version
from Go build information. Official vectors and the exact 256 MiB scheduled
resource run remain part of the development campaign; weakest-native-host
performance remains a separate qualification gate.

## Disposition

Keep only until the R-053 format/state question is answered and recorded in the
research record. Delete the TUI when a maintained Authority Custody module is
authorized behind a real Interface and caller; do not promote this package.
