# R-054 S7E1 canonical-profile experiment

This disposable experiment answers the locally testable part of
[R-054](../../docs/research/records/r-054-stage-7-evidence-profile.md): can one
strict indexed canonical-JSON envelope admit the shared Stage 7 campaign
identity and reduce valid observations deterministically without candidate
verdict authority?

It does not implement a Stage 7 runner, verifier package, platform observer, or
candidate. It creates no maintained project API or nested module. The Go files
are build-ignored so the root module cannot treat this directory as maintained
code.

## Hypothesis and falsification

S7E1's shared envelope is retained if:

- 100 encode/admit/re-encode rounds are byte-identical for all 91 ordered cell
  commitments;
- unknown, missing, duplicate, reordered, whitespace-bearing, trailing,
  oversized, wrong-version, malformed-digest, and ordinal-gap inputs reject;
- unsafe relative paths reject on both Windows- and POSIX-shaped input;
- structural/observer/secret contamination always reduces to `invalid` before
  behavior, while fully valid bad behavior/cleanup reduces to `fail`; and
- no candidate-authored status field exists in admitted campaign input.
- the exact `1 GiB` synthetic zero stream hashes within `60 s` and `16 MiB`
  Go heap allocation on the current local host, while overlong/oversized
  commitments reject before unbounded reading/allocation.

One accepted mutation, byte drift, ambiguous verdict, or unbounded admission
falsifies this shared candidate. Platform observer meaning, control fidelity,
and the final expanded attempt schedule remain separate R-050–R-052/R-054
falsifiers.

## Run

From the repository root with the selected Go toolchain:

```text
go test ./experiments/r-054-stage-7-evidence-profile/profile.go ./experiments/r-054-stage-7-evidence-profile/profile_test.go -count=1
```

All inputs are synthetic public strings and digests. No Authority, Service,
Name, Target, browser/VPN state, credential, or real host observation is read.
Go build/test caches and console output remain outside the repository.

## Expected and actual result

Executed on 2026-08-20 with Go `1.26.6 windows/amd64`. `profile.go` SHA-256 was
`8a9af45a6ac9c3a548cfc8f255e93abeddfab83f56030acbbb4ca7fed11aba24` and
`profile_test.go` was
`da41de6de6212121115584c87b5699964b2c04788da12bd728c2a4feb2e454e4`.
The exact command exited `0` in `0.691 s`.

The run completed 100 byte-identical rounds over two host and 91 contiguous
cell commitments; rejected nine campaign mutation classes and thirteen unsafe path
forms while admitting two derived paths; and matched six verdict-precedence
cases. It also hashed exactly `1,073,741,824` synthetic zero bytes to
`49bc20df15e412a64472421e13fe86ff1c5165e18b2afccf160d4dc19fe68a14`
in `418.0511 ms` with `65,888` Go heap-allocation bytes and rejected an extra
byte plus an oversized commitment. Unknown/candidate-verdict, missing,
duplicate, whitespace, trailing,
wrong-schema, uppercase-digest, ordinal-gap, and oversized campaign inputs all
rejected.

This pass supports only the shared serialization/path/reduction logic. It does
not establish frozen-weakest-host RSS/deadline, real multi-file/index admission,
filesystem link defenses, Ubuntu parity, observer controls, or native fact
semantics. The 392-episode schedule is specified separately and remains
unexecuted.

## Disposition

Retain as bounded R-054 design evidence until S7E1 is accepted, revised, or
rejected. Delete rather than promote this package when the maintained verifier
is authorized behind a real Interface and caller.
