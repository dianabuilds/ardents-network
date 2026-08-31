# Historical Browser/Endpoint compatibility evidence

This directory preserves source evidence removed from the maintained module
graph during the R-128--R-133 truth reconciliation. The files record the old
Firefox presentation inside `internal/endpoint` and the Firefox runtime
qualification harnesses that depended on that presentation.

The evidence came from integration base `89480b18`. Go sources use the
`.go.txt` suffix deliberately: they are historical records, not maintained Go
packages, test profiles, product binaries, or qualification entrypoints. The
larger historical C-2 fixture is available only from the immutable
[`fbb42034757513ac009114a00b933aefa76d8ddf`](https://github.com/dianabuilds/ardents-network/commit/fbb42034757513ac009114a00b933aefa76d8ddf)
source snapshot. None of these former Make targets may be restored without new
current research and an explicit product decision.

Maintained ownership now lives in:

- `internal/application/connection` for the local Application connection
  contract and transport;
- `internal/application/administration` for Publish/Withdraw;
- `internal/browseradapter`, `internal/browserentry`, and
  `internal/browserreference` for Browser-owned behavior;
- `internal/endpoint` for Network-backed implementations only.

The move preserves historical identifiers and claims as provenance. It does
not re-qualify Firefox, the old C-2 presentation, a release, or any public
protocol.
