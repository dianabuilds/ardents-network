# C0 deep-audit run 2: Track A receipt

Status: **immutable finding receipt; Gate A blocked; Track B not started**

This receipt identifies one historical Audit Baseline and its external evidence.
It is not a Qualification result, release decision, product specification, or
claim that the current branch passed Gate A.

## Identities

- Campaign: `c0-deep-audit-run-2`
- Audit activation profile: `c0-source-artifact-track-a-v1`
- Baseline commit: `2e2b63ad2c29d17ffe2f6654f629e79e2de5c909`
- Baseline tree: `7ac32cb8478544323ea3cb567156af21cd05f43a`
- Prior run-1 commit: `426cb103796f93c12b4e3f452f8a470377588f8f`
- Prior run-1 tree: `1ff8ebeeda15808f83403600f3c97079d7d06acc`
- External evidence root: Codex visualization workspace
  `01a0422c-fafb-72d0-8472-6d9dd0379424` on the Product Owner Windows host.
- Absolute locator at capture:
  `C:/Users/vitek/.codex/visualizations/2026/08/27/01a0422c-fafb-72d0-8472-6d9dd0379424/security-audit/ardents-network/run-2/2e2b63ad2c29d17ffe2f6654f629e79e2de5c909/`
- External `evidence-index.json` SHA-256:
  `98b691f1af7a7127d6dbc8ffc92a8235a651d0f8b42b645fbb8497cafc3768b2`

## Exact coverage and change impact

- The baseline tree and machine corpus contain exactly 1,201 paths with matching
  Git blobs.
- 1,130 active paths map exactly once into 141 assignment cells: 1,114 were
  assigned to fresh cell review and 16 byte-identical `internal/contributor`
  paths satisfied the strict cell-reuse rule. Sixteen additional non-exclusive
  cross-review cells own no paths, for 157 total cell records.
- 71 historical-evidence paths are identity-checked and ownership-classified but
  excluded from maintained product conclusions.
- Four exact run-1 paths are retained as tombstones and are absent from run 2.
- 163 paths were touched from run 1 to run 2; 159 current paths have changed or
  new blobs, and 1,042 current paths are byte-identical to run 1.
- No active path is unassigned or duplicated, and the cohesive-cell layer has no
  blocked cell.

The path ledger is mechanical identity and cell-membership evidence, not a
per-file security verdict. Cell artifacts hold review verdicts;
`findings/findings.json` is the sole exact finding-to-source map.

## Artifact identity

Two independent clean normal-clone builds produced identical bytes for eight
artifacts. Their SHA-256 values are:

| Artifact | SHA-256 |
| --- | --- |
| `ardents-browser-entry-windows-amd64.exe` | `c5a865d5f897cc712b31d4028cefbefe095c9838f7079993c04b7216b1662c51` |
| `ardents-browser-windows-amd64.exe` | `0d6715ed1539a0472058f04d3384fe39f54c426fa2442ad2fa870cba94271280` |
| `ardents-control-windows-amd64.exe` | `a81311ab401dc97d9c1844a4834651bd78095b424e4f041194075b17d059fc68` |
| `ardents-custody-windows-amd64.exe` | `4a0113574b078568a898362b643c96214f8270ba999f02a5dbf967e38fc7f3e2` |
| `ardents-node-windows-amd64.exe` | `148d5237efb051644ef769e783b1438dc52efb7b44dd77b386c5a392fa51196a` |
| `ardents-release-custody-windows-amd64.exe` | `742325cb017b4938efe419b47fe0fcd2ce410c6d7463cd1278f9f0986ef32b12` |
| `ardents-state-custody-windows-amd64.exe` | `4e1fd12efe7c800247ed2237d9172e0d1c6098afec4f4fb49c40540b49812d60` |
| `ardents-windows-amd64.exe` | `9b4b6c13c7bf26e5cfc6782aded99b1353b4892412aa4d4a761e62b070a4bf12` |

That evidence also exposed a blocking artifact-boundary defect: the exact same
Endpoint build in a clean linked worktree has SHA-256
`45825ea96fcae4962f31360cff3747b37da0b675c81b947f161552acecba8bb4`
because the selected build does not fix VCS stamping.

## Gate decision

Track A recorded 15 finding or hypothesis entries: 11 Major, three Minor and
one Hardening item. A-COORD-02 has an H2 deterministic artifact proof. The
other 14 entries remain H1 code/document traces and therefore are hypotheses
or structural observations rather than confirmed defects under the audit
policy. Four core Major H1 entries require dedicated deterministic product
proofs before their corresponding runtime repairs can be accepted. Browser H1
entries likewise require proof-backed repair if their affected implementation
remains executable. Retiring that implementation resolves its code findings;
claim-only corrections resolve only ownership and Qualification truth gaps.
The exact list is A-COORD-01 through A-COORD-02,
A-BR-01 through A-BR-07, A-END-01 through A-END-02, A-CTL-01 through A-CTL-02,
and A-RT-01 through A-RT-02. Gate A remains unacceptable regardless of that
evidence distinction.

ADR-0061 already classifies Firefox Entry, XPI and native-host delivery as
compatibility evidence rather than a participant product. The Product Owner
must decide whether to retire the still-active Firefox-v4 lane to immutable
evidence or retain, reclassify and repair it strictly as compatibility and
regression evidence with no participant or network-Qualification claim. The
generic `ardents-browser` compatibility implementation is a separate retain or
retire decision. All other findings have bounded proof or repair routes in the
external checkpoint.

`make check` and `make headless-check` passed on the exact clean baseline. No
runtime topology, platform matrix, external operator, hostile-load, anonymity,
availability, performance, signed-XPI, VPS, soak, or release-Qualification
claim was selected or established.

Any accepted repair changes the candidate identity and invalidates every
affected Track A cell and artifact conclusion. A later run may reuse only the
exact cells that independently satisfy the strict reuse rule in
[`deep-audit.md`](../deep-audit.md); it cannot edit or upgrade this receipt.
