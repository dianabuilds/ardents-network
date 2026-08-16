# Stage 5 blocked-entry evidence harness

Status: maintained S5.4 development contract.

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

The complete construction tree is owner-only: Unix rejects group/other access,
and Windows applies a protected current-user DACL to every directory and file
before atomic publication.

Before hashing or starting a cell, the harness copies the runner, client, and
server into its owner-only external secret tree, synchronizes those private
copies, removes write permission, hashes the copies, and executes only them.
The mutable caller paths are therefore not reopened during the campaign.

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
blocked-entry-lab -workspace-root <repo> -evidence-root <new-external-root> \
  -run-id <id> -mode pass -registry-root <authoritative-external-registry> \
  -runner <hostile-cell-runner> \
  -verifier <pinned-verifier> \
  -client <pinned-client> -server <pinned-server>

blocked-entry-verify-lab -manifest <root>/publishable/manifest.json \
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
