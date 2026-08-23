# Stage 5 blocked-entry evidence harness

> Historical provenance only. R-090 retired the generator-less independent
> H3 verifier in M14: no immutable campaign bundle or current native-v1 claim
> names it. This document and `tests/live/stage5-final` remain C4 records; the
> text below describes the 2026-08-19 development contract, not an executable
> Qualification instruction or result.

Status at record close: completed maintained Stage 5 development evidence and
a frozen, fail-closed S9.6 qualification contract. The Product Owner closed
Stage 5 development on 2026-08-19 without claiming a qualifying run.

The complete `h3-s5-b1-v1` campaign execution, stand-specific supply freeze,
long sustained runtime, and immutable final verdict are S9.6 work. Stage 5 still
owns every scheduled worker and reducer. Until the stand identities and complete
evidence exist, `final-local` preparation and verification must remain `invalid`
rather than emitting a partial `pass`.

## Maintained harness snapshot (2026-08-19)

The following snapshot is the most recent maintained development state, not a
qualification claim. It exists so a reader can see what the harness already
covers versus what is explicitly reserved for S9.6.

**Implemented in the harness:**

- 564 ordered candidate cells plus six independent five-episode
  evidence-integrity campaigns are frozen as the S5.5 contract; the
  `420`-event product-hostile plan and the six disjoint mutation campaigns
  already drive the `blockedentry` dispatcher.
- File-backed telemetry, safe handoff between the harness and the external
  secret tree, a single campaign-monotonic timeline, and independent
  re-reading by the separately built `blockedverify` are wired into the
  runner.
- Raw telemetry and aggregation cover capacity and sustained cells; the
  recovery path now consumes actual measured results rather than summary
  counters.
- All `420` product-hostile cells have concrete workers. G4 retains eleven
  phase identities but records the actual atomic Bridge boundary: regime
  publication and exposure zero are one durable generation; import-only
  restart continues normally; a recorded `opened` terminal survives restart.
- G5 terminal results are caused by the external candidate child/control
  path. G7 binds each forbidden class to its exact production input or
  knowledge-exclusion boundary, proves same-namespace reachability with a
  positive control, and requires zero measured forbidden use. G8 restart and
  Time Confidence loss, and G9 ledger,
  candidate-envelope, and new-operation paths use their production seams.
- Hostile-injection receipts use bounded raw v4 evidence. Both the harness and
  separately built verifier rehash the bytes and independently parse the G4
  phase and G7 zero-use receipts instead of trusting a runner-authored label.
- Pressure offers/injections/state, capacity, recovery, sustained resources,
  mutation sources, and hostile receipts retain their raw-to-verdict basis.
- The host input is only a frozen reservation. It cannot carry runtime claims.
  Qualification additionally requires process-derived CPU-set, cgroup,
  memory-limit, and network-namespace attestation, which the S9.6 stand
  collector owns.

**Maintenance gates currently green:**

- `internal/lab/blockedentry` unit and command E2E: `PASS`.
- `internal/lab/blockedverify` decision tests: `PASS`.
- live compilation and targeted hostile/runner tests: `PASS`.
- `git diff --check`: clean.

**Explicitly reserved for S9.6 and not a Stage 5 result:**

- stand-specific supply freeze (image IDs, tool-lock hash, Go-builder
  archive, builder verification receipts, exact `594`-episode suite seeds);
- the complete `564`-cell candidate campaign plus six integrity campaigns
  executed end-to-end against the cleaned integrated H3 candidate;
- the immutable `pass|fail|invalid` final verdict published by
  `blocked-entry-verify-lab` against a real `final-spec.json`;
- the long sustained, capacity-offer, recovery, pressure, hostile, and
  cleanup thresholds from the R-023 / R-037 / R-038 evidence contract;
- collection of the runtime host attestation from the real allocated process
  trees; the Stage 5 runner deliberately leaves it absent, so the verifier
  cannot emit `pass` before S9.6;
- the Sustained/Normal/Horizon 4 cross-platform qualification cells.

A development-fixture verdict is therefore explicitly `development-fixture`
campaign-kind and is not citable as a qualification result. The remaining items
above must be closed before any final `pass` can be emitted; their thresholds
do not soften because the dispatcher already covers every product-hostile
candidate cell.

`cmd/blocked-entry-lab` owns only the external hostile-campaign bundle. It
requires a new evidence root outside the repository, hashes its own executable,
the separately built verifier, and its explicitly unrestricted S5.4 schema-
fixture client/server inputs, creates eight variant-specific private four-canary
sets under `secret/`, and freezes read-only `publishable/manifest.json` before
the first measured runner process starts. After collection it writes canonical
`publishable/evidence.json` and the separate hash-binding
`publishable/closure.json`. It never writes a verdict.

S5.4 uses the distinct profile `h3-s5-b1-development-fixture-v1`. It does not
claim the accepted `h3-s5-b1-v1` campaign profile, whose repository commit,
Linux image/kernel, configuration, cgroups, workload seeds, clocks, and budgets
are frozen only by the deferred final qualification manifest.

For S9.6 qualification, the same command first creates a new external
`final-spec.json` from
committed source and an external configuration root. Preparation rejects a
dirty tracked tree, an existing destination, repository-local output, mutable
or symlinked input, missing configuration, and a configuration hash mismatch.
It binds the Git commit, SHA-256 of `git archive HEAD`, Ubuntu image and kernel,
pinned WebTunnel binaries, the exact reference/strong/collector classes,
network and clock envelopes, seven configuration artifacts, the ordered `564`
candidate cells and six ordered five-episode evidence-integrity campaigns,
distinct content-addressed product/tool/Go-builder image IDs, and one unique
random 32-byte seed per episode across the entire `594`-episode suite.
Preparation materializes that Git archive
outside the repository and builds the product from only that frozen tree, with
network disabled and the exact preloaded Go-builder image ID. It independently
verifies that builder against the versioned recipe, official archive hash,
hash-bound complete Go module cache, base ancestry, embedded receipts, and exact Go version. The
product carries those receipts forward for runner and independent-verifier
revalidation. It reads and freezes the resulting source/seven-binary receipt and the Carrier
tooling base/tool-lock/source/binary receipt. The Compose topology is copied
from the same materialized archive into the external runtime supply with its
own hash and byte count. Builder/tool image IDs, the official Go archive hash,
the tool-lock hash, and the expected Carrier binary hash must first match the
versioned `tests/live/stage5-final/supply.lock.json` inside that same archive.
Its current `pending-qualifying-stand` entries intentionally prevent final
preparation until those reviewed stand-specific identities are accepted. The
checked-in non-secret
configuration vocabulary lives under `tests/live/stage5-final/`; raw Invites,
addresses, TLS/path secrets, and Route credentials remain external.
The runner environment fixes `/usr/bin:/bin`, the local Docker Unix socket,
and an empty owner-only Docker config. It does not inherit a Docker context,
credential helper, home directory, or caller-selected executable path.

The final manifest uses `campaign_kind=final-local`, profile `h3-s5-b1-v1`,
mode `final-campaign`, and supply class `pinned-offline-webtunnel`. The verifier
requires complete C0-C6 floors, five reference and five stronger capacity
batches, ten sustained runs and 100 ordered windows, paired direct baselines,
both carrier ratios, P0-P4, five recovery episodes, the complete hostile
matrix, exact resource series, non-overcommitted host allocation, and twelve
hash-checked raw measurement/candidate/capture artifacts. Missing or malformed
measurement basis is `invalid`; a trustworthy missed product/resource gate is
`fail`. Development summaries cannot enter this branch.

The sustained fixture uses one manifest-start clock across both directions.
Per-second CPU/memory and process/state streams reject a gap outside the
declared cadence instead of treating a record count as completeness. Exact
Endpoint, Bridge, and publisher network-namespace counters are read by charged
collector sidecars before release and after product cleanup; the harness never
injects `docker exec` into a measured role. Harness-monotonic observations of
cumulative Application progress bracket each fixed manifest boundary, and the
harness interpolates the counter at that boundary before deriving each
non-overlapping 60-second window. Carrier ratios use the complete before/after
integer deltas, including terminal and cleanup traffic. Process/state streams
also retain an explicit post-cleanup boundary sample before their collectors
release the product wrappers. For P4, zero-delta reconciliation uses explicit
same-lifecycle `baseline` and `after-churn` samples. The later `post-cleanup`
sample separately proves zero candidate sockets, descendants, and state bytes;
harness-control sentinels are excluded from product evidence accounting.

The deferred qualification branch deliberately cannot emit a final `pass`. The
`network-live.test` binary implements strict plan decoding, exact 564-cell
identity/seed ordering, and dispatch for all `144` non-hostile and `420`
product-hostile cells. G4 uses phase-specific durable/process observations;
G5 consumes actual child failure; G7 retains the raw Plan/candidate input,
component observation, positive control, and zero-use receipt; G8 and G9
exercise the production lifecycle, candidate, and ledger
seams. Before each mapped worker it rehashes the exact clean Git
archive, requires the frozen commit and source hash, verifies both preloaded
content-addressed image IDs and their product/tool/base/source labels, then
uses `--no-build` paths and bounded stdout/stderr capture. The worker result is
now terminal-bearing and reads the completed role-owned path/DNS results. Raw
observer and telemetry payloads do not enter either streaming control channel.
The exact-cell worker writes a canonical observer artifact, a bounded canonical
telemetry index, and one at-most-2-MiB file per telemetry stream into a
parent-created staging root outside the secret tree. The streaming runner
snapshots them only after the worker exits, publishes them under cell-ID- and
ordinal-derived no-replace names in the external secret tree, and returns only
path/hash/size commitments. The aggregate telemetry volume is therefore not
constrained by the 16-MiB control or index bound. The harness independently
performs the structural admission: it reopens stable regular files, requires
the cell-derived root count and ordered Endpoint/Bridge/Publisher
resource/carrier inventory for every root, and rejects path substitution,
mutation, non-canonical indexes, or commitment mismatch. The separately built
verifier repeats those checks and owns semantic JSONL schema, ordinal, boundary,
and cadence validation. The maintained collector child receives disposable
stable-read copies of its three read-only executable/compose inputs under
staging; the runner rechecks each copied byte stream against the frozen
client/server/runtime-compose SHA-256 commitment before execution. It does
not inherit a runner secret-tree path; candidate containers receive neither
host root as a mount. This same-account child isolation is a trust boundary
between maintained harness components, not an OS sandbox claim.
The child cannot set `evidence_complete`: the parent owns its process group,
verifies that the group has no descendant,
removes and rechecks every token-owned Docker object, and requires the root to
be empty before removing it. A terminal marker is timestamped as it reaches
the parent, before the armed project cleanup starts; start, terminal, and final
cleanup therefore share the runner's monotonic origin. Only after cleanup does
the parent encode the fixed ten-kind zero-residual inventory. It admits the
result only when every batch and exact `capacity-00`…`03/15` Endpoint contains
all nine boundary records. Each boundary has separate IPv4 UDP, IPv6 UDP, and
IPv4 TCP positive controls injected as checksummed, per-run nonced unicast
frames and captured on the manifest-bound Linux interface index. The boundary,
nonce, and protocol class travel in the frame rather than a candidate-writable
coordination label. C0/direct/profile/capacity/pressure/recovery and hostile
cells must expose every required boundary and telemetry stream or remain
structurally incomplete. The ten sustained run cells
retain the ordered Endpoint/Bridge/Publisher resource/carrier inventory. The
runner anchors each worker to its campaign-monotonic offset, and the Linux
collectors translate the host/container shared monotonic clock into that same
origin without using wall time. The runner and verifier require each role's
periodic resource and carrier stream to cover the complete ten-minute window
independently of its cleanup boundary record. Resource aggregates use only
periodic samples inside that active window. The Publisher stream
remains its own endpoint-role evidence rather than being charged to the
WebTunnel Bridge helper boundary. The verifier independently reproduces the
combined Bridge-helper and Endpoint Adapter aggregates and both carrier ratios
from the raw streams. Each final candidate cell
retains its parser-level boundary evidence in the secret bundle and binds its
canonical hash in the public cell record; the independent verifier re-reads
and rejects a missing, altered, incomplete, or non-clean boundary set. Unknown
cells remain fail-closed. The verifier derives capacity, refusal, sustained,
pressure, recovery, hostile, resource, carrier, cleanup, and evidence-integrity
components from the retained per-second/raw observations; merely duplicating a
runner-authored summary into JSONL is not sufficient.

Every final candidate observation is additionally bound index-for-index to the
frozen 564-cell identity and seed schedule. Its start, terminal, and cleanup offsets
must be monotonic, non-overlapping with the next cell, and within the frozen
cleanup bound. The 420 product-hostile observations must reproduce the same offsets in
the hostile event ledger; a reordered, repeated, or substituted cell is
`invalid` even when its aggregate counters look correct.

The live fixture derives purpose-separated Application, capacity-offer, probe,
and partial-handshake corpora from that cell seed. C5 and C6 execute separate
episodes: C5 sends only the four uninformed requests, while C6 sends only the
single disclosed-path request whose detection is the retained limitation.
P0 additionally derives one ordinal-specific corpus per unit and pairs each
Endpoint Application with a named publisher Application, so the four
concurrent 10 Mbit/s streams cannot share or race a corpus.

The complete construction tree is owner-only: Unix rejects group/other access,
and Windows applies a protected current-user DACL to every directory and file
before atomic publication.

Before hashing or starting a cell, the harness copies the runner, client, and
server into its owner-only external secret tree, synchronizes those private
copies, removes write permission, hashes the copies, and executes only them.
For a final campaign, the runner hash must equal the `network-live.test` hash
from the archive-built product receipt; a separately substituted build is
rejected.
The final runner reopens the repository only to prove that its clean `HEAD` and
`git archive --format=tar HEAD` still equal the precommitted source identities;
it never builds from that path. Product cells consume only the preloaded image
ID whose Ubuntu-base and source labels match the specification. Observer
impairment consumes only the distinct preloaded Carrier tooling image ID.

The harness does not manufacture successful events. It sends each entry of the
ordered `420`-event product plan to one separately built, precommitted hostile-cell
runner over bounded streaming stdin and waits at most `31 s` for the matching
strict observation before admitting the next cell. It derives event gates from
the frozen expected terminals. The manifest commits the fixture mode and the
reserved allocation profile; it does not prove runtime ownership. S9.6 derives
the runtime attestation from allocated process trees, and the verifier requires
exact IDs, disjoint CPU sets, cgroup paths and limits, and network-namespace
inodes. The runner must
start the product fixture, inject the named faults, report all nine observer
boundaries and all ten cleanup kinds for every cell. A trustworthy candidate
security failure stops immediately; any residual or ambiguous observer blocks
the next cell. Missing, reordered, oversized, crashed, or timed-out runner
output prevents bundle publication.

Each hostile event records an exact accepted R-035 owner/import or Route
terminal (`invalid`, `incompatible`, `expired`, `wrong-domain`,
`conflicting-role`, `already-present`, `replay`, `set-full`,
`replacement-rejected`, or one of the six `bridge-*` Connection Results).
Verifier disposition `invalid` remains a separate evidence judgment and is
never substituted for an event terminal. Collector/blocker loss and direct
pipeline contamination therefore run as six separate five-episode mutation
campaigns, not as product cells with invented terminals. The independent
verifier must reproduce the exact component vector `candidate=pass` plus six
`invalid` mutation verdicts before the suite can pass.

Forbidden-packet and residual ownership is copied from that independently
derived per-cell attribution, never from a runner-authored owner field. A
candidate-attributed diagnostic is scanned against the private canary corpus
before another cell can start.

`cmd/blocked-entry-verify-lab` is built separately. Its Module imports neither
the harness nor Bridge/Camouflage code. It independently fixes the exact nine
hostile groups, all `420` product-hostile identities, and the six five-episode
evidence-integrity campaigns, eleven process and
network namespaces, nine observed Endpoint/Bridge/publisher/ordinary-Route
boundaries, three positive controls per boundary, monotonic event/terminal/
cleanup ordering, the `6 s` Adapter cleanup bound, the separate `15 s`
post-terminal whole-cell cleanup bound, and all ten residual kinds. It rejects
unknown/non-canonical JSON, missing or
duplicate events, replayed run identities through a durable external
consumed-nonce registry, commitment changes, incomplete or
ambiguous observers, missing cleanup inventory, secret-tree tampering, and raw
canaries in publishable artifacts.

For development fixtures the verifier still exercises the canonical 450-entry
matrix. For final evidence it verifies the 420 product-hostile sequence and the
six disjoint mutation campaigns, and compares evidence index-for-index. Its
eight dedicated canary fixture modes
independently require every candidate-leak and pipeline-contamination variant
to exercise its Invite, numeric-address, WebTunnel-path, and certificate
canaries; a clean echo of any named variant is invalid.

Replay publication is a two-phase recoverable transaction. The verifier holds
an operating-system advisory lock while it reserves the run, exclusively
publishes the canonical verdict, and commits consumption. A crash releases the
lock; a retry can finish a pending reservation or commit an already published,
hash-bound verdict. Frozen runner/client/server copies are part of the committed
secret inventory and are re-hashed by the verifier.
The verdict link and temporary-file removal receive containing-directory
durability barriers before registry commit. The pending registry entry commits
the stable decision digest (all canonical fields except publication time), so
recovery cannot substitute a different verdict. Any intentionally contaminated
publishable artifact is precommitted by manifest path, size, and hash; removal
or replacement is foundationally invalid.
The external replay registry, its lock, transaction successor, and state file
receive the same owner-only Unix permissions or protected Windows DACL; an
existing permissive tree is tightened before it is read.

The verifier accepts only the literal `<bundle>/publishable` and
`<bundle>/secret` non-symlink sibling roots. Alternate pristine copies cannot
replace tampered canonical bundle trees.

Verdict ownership is fixed:

- trustworthy candidate behavior, leakage, or owned residue is `fail`;
- missing, ambiguous, contaminated, or harness-owned evidence is `invalid`;
- an already established trustworthy candidate failure remains `fail` when an
  operational observer defect is also present, but a broken manifest/evidence/
  closure/artifact commitment remains foundational `invalid`; and
- `pass` is emitted only by the verifier after every conjunctive gate passes.

The harness development modes exercise recomputation and fault precedence;
they are schema/collector fixtures, not S9.6 product evidence. S5.5 freezes the
immutable schemas, workers, and reducers; S9.6 supplies observations from the
complete Docker campaign. Both the manifest and evidence therefore carry
`campaign_kind`, and every development-fixture verdict is explicitly scoped;
it cannot be cited as a qualification result. Its source identity binds the
harness and runner executable hashes and its supply class is
`unrestricted-schema-fixture`; S9.6 must instead bind the accepted repository
source and exact registered WebTunnel supply.

Example invocation (all paths are explicit and no command downloads supply):

```text
scripts/prepare-stage5-final-inputs.ps1 \
  -ConfigurationRoot <new-external-input-root> \
  -InviteRoot <private-invites> -RouteCredentialRoot <private-role-credentials>

blocked-entry-lab -workspace-root <repo> \
  -prepare-final-root <new-external-spec-root> \
  -configuration-root <external-input-root> \
  -linux-image <pinned-ubuntu-image> -image-sha256 <image-hash> \
  -go-builder-image-id <sha256:offline-builder-id> \
  -tool-image-id <sha256:tool-id> \
  -kernel <kernel-id> -client <pinned-client> -server <pinned-server>

blocked-entry-lab -workspace-root <repo> -evidence-root <new-external-root> \
  -run-id <id> -mode pass -registry-root <authoritative-external-registry> \
  -runner <external-spec-root>/runtime/network-live.test \
  -verifier <pinned-verifier> \
  -client <pinned-client> -server <pinned-server>

# The qualifying invocation changes mode to final-campaign and adds:
# -campaign-spec <external-spec-root>/final-spec.json

blocked-entry-verify-lab -workspace-root <repo> \
  -manifest <root>/publishable/manifest.json \
  -evidence <root>/publishable/evidence.json \
  -closure <root>/publishable/closure.json -secret-root <root>/secret \
  -registry-root <separate-external-registry> \
  -canaries <root>/secret/canaries.json \
  -publishable-root <root>/publishable -output <root>/verdict.json
```

Maintained command E2E covers `pass`, candidate `fail`, harness `invalid`,
candidate-versus-pipeline canary attribution, candidate residue, incomplete
inventory, replay, event/clock/observer mutation, unknown fields, secret
tampering, missing evidence, canonical result publication, refusal to replace
an existing verdict, recoverable pending replay publication, trustworthy versus
unattributed forbidden packets, dedicated collector/blocker-loss invalid runs,
and cleanup of temporary construction roots. Secret
artifact paths are resolved inside the committed secret tree and symlink escape
is rejected. Generated bundles and raw secrets remain outside Git and are
deleted by their owning test/session lifecycle.
