---
id: R-055
title: Which canonical artifact profile proves Stage 6 development completion?
status: open
owner: Product Owner
started: 2026-08-20
reviewed: 2026-08-20
---

# R-055 — Stage 6 evidence serialization

## Decision this unlocks

Freeze the manifest, observation, cleanup, and verdict bytes that S6.6 must
produce and independently verify. Stage 6 cannot be complete while valid,
failed, and contaminated runs are distinguishable only by runner summaries or
human interpretation.

## Current contract

R-039 and the Stage 6 brief require three disjoint authorities: immutable
manifest, runner observations without verdict power, and an independently built
verifier that emits only `pass`, `fail`, or `invalid`. The mandatory behavior
inventory is A0-D6. Stage completion is maintained development evidence, not
production Qualification, anonymity validation, or a long-duration reliability
claim.

R-041 fixes Service Name encoding, R-043 fixes persistence properties, and
R-046/R-047 fix S6.2 role views and private resolution. R-042, R-044, and R-045
remain open and must be accepted before their exact proof fields become
normative. Recovery cryptography does not select the artifact hash.

## Hypotheses

- **H1:** strict canonical JSON indexes plus ordinal-derived bounded stream
  files and SHA-256 commitments give the verifier all required raw inputs
  without one unbounded aggregate artifact.
- **H2:** one canonical binary container is safer than multiple indexed files.
- **H0:** no same-account local harness can maintain a meaningful manifest,
  evidence, and verdict trust split; S6.6 then needs a stronger execution
  boundary before Stage completion.

## Evaluation criteria

1. Unknown, missing, duplicated, reordered, non-canonical, oversized, stale,
   symlinked, changed-during-read, or path-escaping artifacts are `invalid`.
2. A valid observation that violates expected behavior is `fail`, never
   `invalid`; an expected runtime denial can produce `pass`.
3. The runner cannot author expected verdicts or write the verifier output.
4. The verifier recomputes state, signatures, ordering, admission, role views,
   terminal classes, cleanup, and every content commitment from retained bytes.
5. Every file and aggregate is finite; no base64 aggregate duplicates all raw
   streams in memory.
6. One Developer can reproduce the development campaign locally without a
   hidden operator, external auditor, or qualification laboratory.

## Evidence plan

### Primary sources

- R-027/R-029 and the Stage 5 evidence contract, accessed 2026-08-20 — existing
  project patterns for immutable manifests, canonical JSON, complete input
  corpora, independent recomputation, stable reads, and disjoint roots.
- [RFC 8259](https://www.rfc-editor.org/rfc/rfc8259), accessed 2026-08-20 — JSON
  syntax and interoperability constraints.
- [FIPS 180-4](https://csrc.nist.gov/pubs/fips/180-4/upd1/final), accessed
  2026-08-20 — SHA-256.

### Experiment

S6.6 itself is the bounded experiment: generate one complete S6E1 campaign,
verify it in a separate process, then independently mutate every schema field,
path, ordinal, hash, ordering relation, runtime observation, cleanup entry, and
verdict binding. Generated fixtures and evidence stay outside Git; only golden
vectors and bounded summaries are retained.

### Failure scenarios

- Runner-authored booleans substitute for raw proof recomputation.
- One valid but behaviorally bad observation is misclassified as `invalid`.
- A changed stream, unknown field, duplicate ordinal, symlink, or trailing byte
  passes canonical admission.
- Manifest and evidence roots overlap, or a worker can alter the manifest or
  verifier output.
- An incomplete A0-D6 inventory receives `pass`.
- A cleanup result omits a durable file, process, listener, or temporary root.

## Findings

- **Sourced fact:** existing maintained stages already use canonical strict JSON
  records, SHA-256 commitments, stable regular-file reads, and independent
  verifier output.
- **Sourced fact:** S6.2 already selects SHA-256 commitments independently of
  the still-open threshold Recovery Authority mechanism.
- **Inference:** per-stream files with a bounded canonical index avoid both the
  Stage 5 aggregate-size failure and one large in-memory duplicate.
- **Inference:** one deterministic episode per behavior cell establishes Stage
  6 development behavior but cannot support a statistical Qualification claim.
- **Assumption:** same-account process separation detects ordinary accidental or
  candidate-authored contamination but is not an OS security boundary against a
  malicious process with the same account privileges.

## Options

1. **S6E1 indexed canonical JSON.** Small strict indexes plus ordinal-derived
   JSONL/raw files, SHA-256 commitments, and separate verifier output.
2. **One canonical binary bundle.** Fewer files, but duplicates large evidence,
   complicates streaming, and concentrates parser risk.
3. **Database-backed evidence.** Rejected for this stage: selects a storage
   engine, mutable query semantics, and a larger trust surface without need.

## Decision-ready candidate S6E1

This candidate is not accepted until the Product Owner chooses it.

### Canonical bytes

Every index/record is UTF-8 JSON emitted from a fixed Go struct with
`encoding/json.Marshal`: exact declared field order, lower-case ASCII enum and
hex strings, base-10 integers, no maps, floats, optional unknown fields,
whitespace, BOM, or trailing bytes. Admission rejects unknown fields and
trailing values, re-encodes, and compares byte-for-byte. JSONL is one such
object followed by exactly one LF per line.

SHA-256 is written as exactly 64 lower-case hexadecimal characters. Paths are
not supplied by a worker: the reader derives every path from validated ordinal
and role values and opens a regular non-symlink leaf beneath its already opened
root. Stable read requires pre-open and post-read identity, size, and modification
metadata to match.

### Root ownership and layout

The launcher creates four new mutually disjoint, non-symlink roots outside Git:

```text
private/                         launcher-owned synthetic fixtures and keys
manifest/campaign.json           frozen before any worker starts
manifest/cells/<ordinal>.json    exact A0-D6 cell inputs and expectations
evidence/index.json              runner-published artifact commitments
evidence/cells/<ordinal>/
  observations/<stream>.jsonl    raw ordered role observations
  terminal.json                  observed runtime class and state commitments
  cleanup.json                   complete post-cell inventory
verdict/verdict.json             verifier-only output
```

Workers receive only bounded copies of their cell inputs and a private staging
root. The parent admits stable artifacts, publishes them under derived paths,
removes staging, and only then starts the verifier. Workers receive neither the
manifest root nor the verdict root. The verifier opens manifest/evidence
read-only, receives read-only access to explicitly committed synthetic
verification inputs under `private`, and creates the verdict leaf exclusively.

For synthetic admission cells, the manifest commits the SHA-256 of the
launcher-generated HMAC boot secret. After every worker has terminated and the
evidence root is immutable, the launcher gives the verifier a bounded read-only
copy of that synthetic secret; the verifier checks the commitment and
recomputes every challenge tag. The runner and worker never receive this
verification copy, and no live-network boot secret or User material enters the
bundle. This is development-fixture disclosure, not a production key-export
mechanism.

### Bounds and inventory

- profile: `ardents-h3-stage-6-evidence-v1`;
- exactly `27` cell identifiers, A0-A5, B0-B5, C0-C7, and D0-D6, each once and
  in that order;
- one deterministic development episode per cell; statistical or long-duration
  Qualification is explicitly outside this profile;
- at most `32` episodes per cell, `8` streams per episode, `4 MiB` per stream,
  `1 MiB` per index/manifest/terminal/cleanup file, and `256 MiB` total evidence;
- maximum `64` cells is a parser safety bound, not permission to omit or invent
  S6E1 cells;
- all elapsed values are non-negative signed campaign-monotonic millisecond
  offsets from one launcher origin; wall time is provenance only;
- campaign, cell, episode, stream, and observation ordinals are contiguous from
  zero with no duplicates or gaps.

The campaign manifest binds the profile, random `32-byte` run identifier,
source commit and dirty-state digest, executable hashes, platform/toolchain,
accepted decision/profile identifiers, one clock origin, all cell-manifest
hashes, expected runtime classes, and exact required stream inventory. It does
not contain an expected verifier result.

The evidence index binds the campaign digest and every observed file path,
schema, role, ordinal, byte length, SHA-256, start/end offsets, and terminal
class. It contains no pass/fail/invalid field and no summarized predicate that
can substitute for raw evidence.

The verdict binds the campaign digest, evidence-index digest, verifier
executable hash, status, and bounded diagnostics. `pass` requires every S6E1
cell and predicate; `fail` names behaviorally false predicates; `invalid` names
only structural/provenance defects. Diagnostics contain fixed codes and cell
identifiers, never private fixture material or raw transport errors.

## Recommendation

Choose S6E1 after R-042/R-044/R-045 are accepted and their exact fields are
inserted into the cell manifests. Confidence is high that indexed files and
strict re-encoding fit the existing repository evidence model. The strongest
counterargument is the honest same-account limitation; S6E1 must not be cited as
hostile-OS isolation or independent external audit.

## Disposition

- State: `open`, decision-ready.
- S6.6 implementation remains blocked on Product Owner acceptance of S6E1 and
  the three remaining S6.0 decisions.
- If accepted, update the Stage 6 evidence contract, readiness checklist,
  package map, and implementation brief before maintained code lands.
