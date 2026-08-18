# Stage 5 blocked-entry evidence harness

Status: maintained S5.4 verifier and an in-progress S5.5 campaign contract.
The qualifying S5.5 run remains pending both the maintained full-campaign
runner/raw recomputation slice and the later non-overcommitted final stand.

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
are frozen only by the S5.5 campaign manifest.

For S5.5, the same command first creates a new external `final-spec.json` from
committed source and an external configuration root. Preparation rejects a
dirty tracked tree, an existing destination, repository-local output, mutable
or symlinked input, missing configuration, and a configuration hash mismatch.
It binds the Git commit, SHA-256 of `git archive HEAD`, Ubuntu image and kernel,
pinned WebTunnel binaries, the exact reference/strong/collector classes,
network and clock envelopes, seven configuration artifacts, all `594` ordered
cells, distinct content-addressed product/tool/Go-builder image IDs, and one
unique random 32-byte seed per cell. Preparation materializes that Git archive
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

This intermediate slice deliberately cannot emit an S5.5 `pass`. The frozen
`network-live.test` binary implements strict plan decoding, exact 594-cell
identity/seed ordering, and the selected-test dispatch map for the `144`
non-hostile cells plus the currently maintained hostile workers: G1, G2, all
eight G3 replay/replacement variants, G4's atomic regime/exposure-0 restart,
G6 Network/Route-Profile substitutions, G8 cancellation/Endpoint restart, and G9 unknown
Invite field. Before each mapped worker it rehashes the exact clean Git
archive, requires the frozen commit and source hash, verifies both preloaded
content-addressed image IDs and their product/tool/base/source labels, then
uses `--no-build` paths and bounded stdout/stderr capture. The worker result is
now terminal-bearing and reads the completed role-owned path/DNS results. The
child cannot set `evidence_complete`: the parent owns the worker's only
temporary root and process group, verifies that the group has no descendant,
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
coordination label. C0/direct/negative/recovery
cells that do not yet expose every required
boundary therefore remain structurally incomplete. Each mapped final cell now
retains its parser-level boundary evidence in the secret bundle and binds its
canonical hash in the public cell record; the independent verifier re-reads
and rejects a missing, altered, incomplete, or non-clean boundary set. Unmapped
hostile cells remain fail-closed, and the verifier still records `maintained
final runner raw-to-verdict recomputation is not implemented` until it can
derive every aggregate from the retained per-second/raw observations. Merely
duplicating a runner-authored summary into JSONL is not sufficient.

Every final observation is additionally bound index-for-index to the frozen
594-cell identity and seed schedule. Its start, terminal, and cleanup offsets
must be monotonic, non-overlapping with the next cell, and within the frozen
cleanup bound. The 450 hostile observations must reproduce the same offsets in
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
ordered `450`-event plan to one separately built, precommitted hostile-cell
runner over bounded streaming stdin and waits at most `31 s` for the matching
strict observation before admitting the next cell. It derives event gates from
the frozen expected terminals. The manifest commits the fixture mode and the
only accepted process/cgroup-to-owner mappings; the harness, rather than the
runner, writes each canonical attribution record and the verifier independently
recomputes its owner from those inputs. The runner must
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
pipeline-contamination cells therefore carry no invented product terminal.

Forbidden-packet and residual ownership is copied from that independently
derived per-cell attribution, never from a runner-authored owner field. A
candidate-attributed diagnostic is scanned against the private canary corpus
before another cell can start.

`cmd/blocked-entry-verify-lab` is built separately. Its Module imports neither
the harness nor Bridge/Camouflage code. It independently fixes the exact nine
hostile groups and all `450` five-episode event identities, eleven process and
network namespaces, nine observed Endpoint/Bridge/publisher/ordinary-Route
boundaries, three positive controls per boundary, monotonic event/terminal/
cleanup ordering, the `6 s` Adapter cleanup bound, the separate `15 s`
post-terminal whole-cell cleanup bound, and all ten residual kinds. It rejects
unknown/non-canonical JSON, missing or
duplicate events, replayed run identities through a durable external
consumed-nonce registry, commitment changes, incomplete or
ambiguous observers, missing cleanup inventory, secret-tree tampering, and raw
canaries in publishable artifacts.

The verifier flattens the manifest matrix into one canonical 450-entry sequence
and compares evidence index-for-index. Its eight dedicated canary fixture modes
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
they are schema/collector fixtures, not S5.5 product evidence. S5.5 supplies
the same immutable schemas with observations from the complete Docker campaign.
Both the manifest and evidence therefore carry `campaign_kind`, and every S5.4
verdict is explicitly scoped to `development-fixture`; it cannot be cited as
an S5.5 campaign result. Its source identity binds the harness and runner
executable hashes and its supply class is `unrestricted-schema-fixture`; S5.5
must instead bind the accepted repository source and exact registered
WebTunnel supply.

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
