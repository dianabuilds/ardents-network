---
id: R-054
title: Which canonical evidence profile independently proves Horizon 3 Stage 7?
status: decided
owner: Product Owner
started: 2026-08-20
reviewed: 2026-08-20
---

# R-054 — Stage 7 evidence and verifier profile

## Decision this unlocks

Freeze the canonical manifest, private-fixture, evidence, cleanup, and verdict
serialization; campaign identity; exact complete A–H reference inventory;
pre-execution coverage partition; episode counts; clocks; resources; observers;
mutation corpus; and independent `pass|fail|invalid` predicates before any S7.1
result can be cited.

## Current contract

R-048, the [Stage 7 evidence contract](../../development/stage-7-platform-evidence.md),
R-023, and the H3 technical design require disjoint artifact authorities,
immutable precommitment, native-resolution security/transition observations,
controlled platform observers, complete cleanup, deterministic independent
verdicts, and no secret-bearing repository/evidence. H3 development results are
not Route Qualification or H4 independent-supply evidence.

## Hypotheses

- **H1:** one bounded canonical profile with platform-specific observation
  records, explicit coverage partitions, and shared behavior predicates can
  verify the scheduled development surfaces without importing candidate
  decision logic or overstating supported-host qualification.
- **H2:** separate Ubuntu/Windows evidence schemas are necessary, joined only by
  a higher-level report.
- **H0:** required platform facts cannot be observed reliably enough to judge
  principal/isolation/update behavior; affected candidates remain `invalid`.

## Evaluation criteria

- deterministic canonical bytes with explicit schema/profile/campaign versions;
- manifest frozen before candidate execution and private canary release;
- content-addressed immutable paths and no manifest/evidence/verdict overlap;
- exact host/source/supply/package/root/metadata/artifact/config/tool/observer/
  verifier identity, project-control declaration, and exhaustive pre-execution
  `scheduled|authorization-pending|environment-deferred` partition;
- monotonic per-host timings, wall-clock correlation only, no result-selected
  replacement or seed reroll;
- complete raw transition, process-tree, filesystem/registry/package/service,
  ACL/mode, handle/FD, listener/packet/DNS, resource, and cleanup observations;
- a deterministic expected runtime class and predicate set per cell;
- mutation vectors for schema, ordering, identity, hash, path, cross-cell,
  authority overlap, secret leakage, candidate verdict, and cleanup;
- finite bytes/files/events/attempts/resources/runtime/retention; and
- one-to-one reproducibility without pretending the verifier is an independent
  security reviewer.

## Evidence plan

### Primary sources

Repository sources accessed 2026-08-20:

- Stage 7 lifecycle and evidence specifications;
- H3 technical design evidence/decision model;
- R-023 immutable Qualification Evidence Bundle and measurement semantics;
- R-028 resource/evidence split;
- R-031 Application Interface evidence limitations;
- R-037 Stage 5 evidence-integrity campaign design; and
- Stage 6 private naming evidence contract for the same `pass|fail|invalid`
  authority split.

External format sources accessed 2026-08-20:

- [RFC 8259](https://www.rfc-editor.org/rfc/rfc8259) for the JSON interchange
  grammar and its duplicate-name interoperability warning; and
- [FIPS 180-4](https://csrc.nist.gov/pubs/fips/180-4/upd1/final) for SHA-256.

External platform sources accessed 2026-08-20:

- [Linux cgroup v2](https://www.kernel.org/doc/html/latest/admin-guide/cgroup-v2.html)
  for complete process-tree membership and resource observations;
- the [Linux audit userspace](https://github.com/linux-audit/audit-userspace)
  project and its `auditctl` manual for scoped native audit rules;
- upstream [tcpdump](https://github.com/the-tcpdump-group/tcpdump) source and
  release history for host/peer packet capture;
- Microsoft documentation for
  [WPR built-in profiles](https://learn.microsoft.com/en-us/windows-hardware/test/wpt/built-in-recording-profiles),
  [Packet Monitor](https://learn.microsoft.com/en-us/windows-server/networking/technologies/pktmon/pktmon-syntax),
  [tracerpt](https://learn.microsoft.com/en-us/windows-server/administration/windows-commands/tracerpt),
  [IsProcessInJob](https://learn.microsoft.com/en-us/windows/win32/api/jobapi/nf-jobapi-isprocessinjob),
  and the
  [NetTCPIP module](https://learn.microsoft.com/en-us/powershell/module/nettcpip/)
  for independent Windows facts; and
- the official signed
  [Ubuntu 26.04 LTS image set](https://releases.ubuntu.com/26.04/) as the future
  native desktop qualification baseline; current Ubuntu development evidence
  uses a digest-pinned Ubuntu 26.04 OCI image on the declared Docker engine and
  host kernel.

The exact execution-surface/tool/binary/profile identities and coverage
partitions are campaign-manifest values and cannot be substituted after
candidate results. The current Windows machine is inventoried, not described as
pristine.

### Experiment

The now-retired build-ignored S7E1 shared-profile experiment defined canonical
admission, derived-path validation, and verdict precedence independently of
candidate command code. It ran a synthetic 91-cell campaign through 100
byte-stable rounds plus bounded schema/path/verdict mutations. The recorded
result is historical design evidence, not a maintained verifier.

The local experiment now also streams the synthetic `1 GiB` maximum. The
remaining experiment expands to the real multi-file/index artifact union,
every scheduled filesystem link/reparse control, and the tagged observer union
available in Ubuntu 26.04 Docker and on the current Windows machine. It also
runs the exact positive/negative controls frozen in the
[development-host campaign specification](../../development/stage-7-host-campaign-spec.md).
Native Ubuntu Desktop, pristine Windows, hard-power, and other unavailable facts
stay in the coverage ledger rather than becoming synthetic results.

Run a small observer-control fixture outside the candidate to prove each packet,
DNS, listener, process, file/object, and resource collector detects a known
positive and distinguishes absence from failure to observe.

### Failure scenarios

Candidate writes expected result; runner omits failures; observer starts late;
wall-clock jump changes deadline; platform schema maps missing fact to false;
mutated image/tool after manifest; symlink/reparse path escape; partial cleanup
called pass; private canary/Authority leaked; duplicate event selects favorable
order; a report averages required cells; invalid run silently replaced.

## Falsification criteria

R-054 has two separated phases. Schema prototyping may use synthetic artifacts
only. The canonical schema, campaign identity, cell/episode counts, clocks,
resource limits, observer set, and predicates MUST then be decided and committed
before any candidate Stage 7 execution; prototype results cannot count as a
Stage 7 pass.

O1/O2 is falsified if either host differs across `100` byte-stable canonical
round trips; any precommitted mutation is accepted once; a candidate-authored
verdict affects recomputation; a missing native fact maps to success; any path
escapes an owned root; or any observer misses one of `10` positive controls or
reports a false positive in `10` paired negative controls. The synthetic maximum
bundle is `1 GiB`; verification must stream within `256 MiB` peak RSS and `60 s`
monotonic time on every scheduled verifier surface, rejecting excess before
unbounded allocation. Weakest-native-host performance remains a qualification
deferral. A fact known to be unavailable before execution defers its episode;
an unavailable required observation during a scheduled attempt yields `invalid`.
No threshold, retry, episode, or cell may be revised after candidate results;
if the frozen profile cannot judge both hosts, select O0.

## Findings

- **Inference:** shared behavioral predicates with typed platform observation
  variants preserve comparable outcomes better than two independent schemas.
- **Inference:** observer positive controls are mandatory; “no packet seen” is
  not evidence when the collector itself was not proven active.
- **Assumption:** existing repository canonical evidence patterns can be reused
  conceptually without importing a Stage 5/6 schema or candidate verdict code.
- **Measured prototype result:** on Go `1.26.6 windows/amd64`, the build-ignored
  shared-profile experiment passed 100 byte-identical round trips over two host
  and 91 ordered cell commitments; rejected nine campaign and thirteen unsafe-path
  mutations; admitted two derived paths; and matched six verdict-precedence
  cases. The updated exact source also hashed `1 GiB` of synthetic zero evidence
  to the independently computed digest in `418.0511 ms` with `65,888` Go heap-
  allocation bytes; the whole suite completed in `0.691 s`. This does not
  measure frozen-host RSS or exercise real artifact files, filesystem-link
  races, Ubuntu, or observers.

## Decision-ready candidate S7E1

This candidate freezes the shared format and safety envelope. The host-campaign
specification now freezes the observer-source profile and deterministic
91-cell/392-episode expansion. Exact clean image, executable, package, browser,
observer-binary, parser, rule, profile-export, and run digests remain pre-result
manifest instance values rather than guessed documentation values.

### Canonical bytes and paths

S7E1 uses indexed canonical JSON rather than one aggregate or a database. Every
JSON object is UTF-8 output from one fixed Go struct using `encoding/json.Marshal`
with declared field order, followed by no byte for single-object files and
exactly one LF per JSONL record. Schemas use lower-case ASCII enums, lower-case
hex, base-10 integers, Booleans, fixed-order arrays, and no maps, floats, null,
optional unknown fields, BOM, insignificant whitespace, or trailing value.

Admission decodes with unknown-field rejection, requires exactly one value,
re-encodes, and compares byte-for-byte. This rejects duplicate names, alternate
member order, whitespace, noncanonical escapes, and last-value-wins ambiguity.
Every digest is exactly 32-byte SHA-256 rendered as 64 lower-case hex characters.

Artifact paths are verifier-derived from validated zero-based ordinals. A
relative segment is `1..64` lower-case ASCII letters, digits, dot, underscore,
or hyphen; it cannot be empty, `.`/`..`, a Windows device name, an absolute/
drive/UNC path, contain slash/backslash/colon/percent encoding, or make the
relative path exceed `240` bytes; a segment cannot end in dot. Readers open regular non-link leaves under an
already resolved owned root and require pre/post-read identity, size, and
modification facts to remain equal. Symlink, junction, reparse, mount, hard-link,
or cross-volume substitution is `invalid`.

### Authorities and layout

The launcher creates four new mutually disjoint non-link roots outside Git:

```text
private/fixtures/<ordinal>                 launcher-owned canaries/secrets
manifest/campaign.json                     frozen before candidate work
manifest/hosts/<ordinal>.json              exact native host/observer identity
manifest/cells/<ordinal>.json              exact cases and predicates
evidence/index.json                        runner artifact commitments
evidence/hosts/<host>/controls/<stream>.jsonl
evidence/cells/<cell>/attempts/<attempt>/observations/<stream>.jsonl
evidence/cells/<cell>/attempts/<attempt>/terminal.json
evidence/cells/<cell>/attempts/<attempt>/cleanup.json
verdict/verdict.json                       verifier-only output
```

The launcher alone owns private fixtures and immutable manifest publication.
Each candidate/worker receives only its bounded cell inputs, released fixture
material for that phase, and a private staging root. The runner admits stable
raw artifacts, publishes the evidence index, removes staging, and has no verdict
field or verdict-root handle. The separately built verifier opens manifest,
committed evidence, and only the declared verification copy of private canaries
read-only; it creates a previously absent verdict leaf exclusively. Same-account
process separation detects contamination but is not a security boundary against
an Owner/OS compromise.

`campaign.json` binds profile/schema versions, a launcher-generated random
32-byte run ID, clean source commit, executable/package/verifier/tool digests,
all accepted decision/profile IDs, project-control limitation, exact host and
cell-manifest digests, root identities, clock episodes, resource bounds, and
required artifact inventory. It contains expected runtime classes/predicates,
never an expected verifier result.

Each cell manifest binds logical cell ID, ordinal, host/platform, Distribution
Profile, candidate/profile, exact case IDs, attempt ordinal, initial-state and
private-fixture commitments, fault, deadlines/resources, expected runtime
class, and required raw predicate/stream inventory. `index.json` binds only
campaign digest plus every admitted path, schema/media kind, owner, host/cell/
attempt/stream ordinal, length, SHA-256, start/end monotonic offsets, and
terminal class. It cannot summarize a predicate or contain
`pass|fail|invalid`.

### Inventory, attempts, clocks, and bounds

- profile is `ardents-h3-stage-7-evidence-v1`, schema version `1`;
- logical inventory is exactly `91` cells in order: A0–A13, B0–B14, C0–C11,
  D0–D6, E0–E14, F0–F11, G0–G8, and H0–H6; maximum `128` is only a parser bound;
- the exact reference expansion is `392` episodes in the order and axes
  specified by `ardents-h3-stage-7-development-host-campaign-v1`: A `28`, B
  `30`, C `24`, D `40`, E `60`, F `48`, G `134`, and H `28`. Before the first
  candidate attempt, every reference episode enters exactly one of `scheduled`,
  `authorization-pending`, or `environment-deferred`. Every scheduled tuple
  runs once. There is no retry, averaging, best-of selection, or statistical
  claim. A harness failure is `invalid`; a rerun uses a fresh run ID and cannot
  replace or hide the retained attempt;
- each logical cell binds exactly one `stage7-<lower-case-cell>-v1` case ID;
  any internal probes have contiguous manifest ordinals and precommitted
  inputs/faults/streams. Coverage exclusions use the precommitted pending or
  deferred partitions, not a runtime `not-applicable` verdict. Unexpected
  inability during a scheduled episode is `unsupported` when that is the exact
  required product result, otherwise `invalid`; it is never a skipped success;
- on every host and after every boot, each required observer domain runs exactly
  five paired positive/negative controls before the first candidate attempt and
  five after the last. One missed positive, false positive, late start, gap, or
  changed observer identity invalidates every dependent attempt;
- event ordinals and monotonic nanosecond offsets are contiguous/nondecreasing
  within one declared host boot/observer episode. Reboot creates a new episode.
  UTC RFC 3339 nanoseconds correlate hosts only and never decide a deadline;
- maximums are `16,384` files, `128` cells, `64` attempts per cell, `32` streams
  per attempt, `64 KiB` per JSONL record, `128 MiB` per stream, `16 MiB` per
  manifest/index/terminal/cleanup object, and `1 GiB` over committed evidence;
  excess is rejected before allocation or partial verdict; and
- the verifier streams within `256 MiB` peak RSS and `60 s` monotonic time on
  each scheduled verifier surface. Weakest-native-host performance remains a
  qualification deferral. It emits at most `256` fixed-code
  diagnostics totaling `16 KiB`, with no secret/raw platform-error text.

### Native observations and controls

Shared records carry exact host/cell/attempt/ordinal/time and a tagged platform
payload. Exactly one Ubuntu or Windows payload is present; unknown tags or a
shared Boolean substituted for a required native fact are `invalid`. The frozen
observer domains are process/tree/identity, filesystem/registry/package/service,
ACL/mode/owner, handle/FD/IPC, listener/socket/packet/DNS, route/proxy/VPN,
resource/deadline, transaction/durability, and final survivor/cleanup inventory.
The host-campaign specification selects the native source/tool/API family and
positive/negative control for each domain; exact host binary/profile/rule hashes
are immutable manifest instance values. Candidate self-report remains
diagnostic only.

Absence has three distinct encodings: `observed-absent` after a passing control,
`observed-present` with native facts, or `unobservable`. A fact known to be
unobservable before execution places its dependent episode in
`environment-deferred`. `unobservable`, omitted, contradictory, or control-
failed evidence during a scheduled episode is `invalid`; it can never be
coerced to false. Candidate-caused observer interference is a valid behavior
violation and therefore `fail` when the independent observer can prove it.

### Verdict reduction and mutation corpus

Verification order is fixed: structural/provenance admission first; then raw
predicate recomputation; then cleanup; then publication. Any structural,
identity, observer, clock, secret-contamination, or trust-separation defect is
overall `invalid`. With a fully valid artifact set, one false functional,
security, privacy, resource, platform, or cleanup predicate is overall `fail`.
Only complete validity and every true predicate is `pass`. Results are
conjunctive across all scheduled attempts and surfaces; no score, majority,
average, warning, or stronger cell offsets another failure. Coverage is
reported separately, so a passing scheduled subset never means that pending or
deferred reference episodes passed.

The S7E1 synthetic experiment mutates exactly one class at a time on both
verifier platforms: unknown/missing/duplicate/noncanonical field; schema/enum;
ordinal gap/duplicate/reorder; length/hash; absolute/traversing/encoded path;
symlink/reparse/hard-link/cross-volume leaf; root/authority overlap; worker or
candidate verdict field; cross-campaign/cell/host substitution; clock rollback/
deadline ambiguity; missing/late/false observer control; candidate-authored
summary replacing raw fact; secret/private-canary leak; cleanup omission/
contradiction; valid behavior breach; oversized/count excess; and pre-existing/
replaced verdict. Structural cases MUST yield `invalid`; the valid behavior and
proven candidate residue cases MUST yield `fail`.

Both platforms perform `100` canonical round trips of the synthetic maximum
shape and every mutation once. A byte drift, accepted mutation, class mismatch,
memory/time excess, or platform-dependent verdict falsifies S7E1.

## Options

- **O1:** one canonical profile with shared manifest/verdict and explicit Ubuntu/
  Windows observation variants.
- **O2:** separate platform schemas with a manifest-bound cross-platform join
  verifier.
- **O0:** no valid verdict for facts that cannot be independently observed.

## Recommendation

Retain the passing common S7E1/O1 result and advance the scheduled real-artifact,
filesystem-link, Ubuntu-Docker, current-Windows, and observer cells. Freeze an
exhaustive coverage partition for all 392 reference episodes before execution.
Select O2 only if a required observable native fact cannot be represented by
the tagged platform payload without weakening its meaning. Every scheduled
shared predicate remains conjunctive; no averaging or runtime “not applicable”
may hide a platform failure. Confidence is high for canonical admission/
authority split and medium for the available observer union; native supported-
host qualification remains explicitly incomplete.

## Disposition

- State: `decided`; the common synthetic experiment passed and the Product Owner
  accepted S7E1 for Stage 7 development on 2026-08-20. It freezes the
  exact shared canonical/authority/path/clock/verdict candidate, logical
  91-cell order, deterministic 392-episode reference expansion, coverage
  partitions, safety bounds, observer source profile, controls, and mutation
  classes for experiment. Exact Docker/current-Windows, candidate/package/
  browser/observer binary identities and scheduled results remain per-run
  inputs. Qualification deferrals are accepted as limitations, not passes; no
  maintained verifier package is selected before its real caller exists.
- Generated bundles, secrets, captures, databases, logs, and evidence remain
  outside Git.
