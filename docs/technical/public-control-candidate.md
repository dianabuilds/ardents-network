# Public-control candidate evidence contract

Status: **historical project-control campaign evidence; diagnostic reader retained.**
This defines the bounded mechanical evidence the Product Owner and Codex
inspect. It grants no Endpoint, release, Network Epoch, Namespace, or emergency
authority, and it makes no public-operation or independent-participant claim.

## Candidate identity and custody

The simulation has a content-addressed candidate identity and exactly five
fresh custody-role keys. Their declared boundaries exercise collision detection;
they are not claims about real operators or organisations.

Routine Network Epoch and public executable authorization require `3-of-5`.
An emergency requires `4-of-5`, carries a finite expiry, and is limited to
stopping unsafe new work, terminating an unsafe build, or withdrawing/draining
an unsafe duty. It cannot seize or release a Name, rewrite a live destination,
select a Route, decrypt traffic, or install code.

Every simulated successor names its exact predecessor candidate identity and
requires both predecessor and successor quorums. The reader rejects lower or
divergent generations; loss, compromise, removal, replacement, and emergency
recovery are explicit finite simulation cells.

## Candidate View evidence

For every audited Epoch, the package retains exact content-addressed artifacts
for:

- the threshold-authenticated Epoch and its authority/threshold roster;
- the append-only input-log sequence in its committed canonical index order,
  cutoff, and input-log root;
- the deterministic inclusion/rejection rule revision, complete rejection
  outcomes, Candidate View root/length, and global count/capacity/concentration
  summaries; and
- the deterministic Candidate Materialization rule revision and all published
  indexed material/proofs needed for a verifier to check an Endpoint's chosen
  index.

An Endpoint checks only its selected material and proof. Withholding retries
only the same selected index through another declared source; it never selects
a different index or invents a Route. Silence is `unavailable`, not a claim of
malice. A full auditor retrieves the complete input log, applies the exact
rule revision, reconstructs the View/rejections/summaries, and binds its output
digest plus tool/source revision to the candidate package. A partial View,
locally generated input log, or audit without the complete input set is not a
full-audit result.

This does not select public byte, count, execution-time, or storage limits.

## Build and audit evidence

Each simulated package retains the artifact digest, source revision, resolved
dependency digests, build recipe, SBOM, selected qualification identity where
claimed, and two matching simulated builder attestations. Each attestation
binds its exact inputs/outputs; it does not identify an external trust boundary.

Two simulated full auditors publish a complete evidence-package digest,
reconstruction output digest, tool/source revision, and result. A disagreement
is an explicit failing cell and a later success does not erase it.

## Reader outcomes and limits

The maintained H4-6C reader is diagnostic only. It must expose the candidate
identity, declared role boundaries, required evidence identities, verified
cryptographic/format facts where a selected encoding supports them, and these
outcomes:

| Condition | Required result |
|---|---|
| malformed/forged evidence | `forged` |
| lower generation or predecessor mismatch | `replayed` |
| expired roster, action, Epoch, or package evidence | `stale` |
| authenticated competing successor/auditor result | `conflicting` |
| revoked build/action | `revoked` |
| missing or withheld declared object | `unavailable` |
| duplicate/overlapping declared control boundary | `independence-conflict` |
| mechanically complete declared package | `declared-evidence-complete` |
| real independence not externally corroborated | `external-evidence-required` |

`declared-evidence-complete` is not an authority decision. The reader's
`external-evidence-required` outcome applies only to a future public-operation
claim; it does not invalidate the completed H4-6C simulation.

## Historical project-controlled mechanics simulation

The retired command
`ardents-control simulate-public-control --source-revision LOWERCASE_40_HEX_COMMIT`
emitted the accepted H4-6C receipt. Each historical invocation generated
fresh in-memory identities for five simulated custodians, two builders, two
auditors, and a successor custody set. It verifies one routine `3-of-5` action,
one expiring disable-only emergency `4-of-5` action, loss/compromise/removal/
replacement/emergency-recovery rotations signed by predecessor and successor
quorums, two matching complete Candidate View reconstructions with a cutoff,
summary, materialization revision and indexed proof, and two matching builder
attestations over retained source/dependency/recipe/SBOM/qualification inputs.
It requires rejection of threshold, expiry, emergency-escalation, predecessor,
Candidate View, and builder mismatches, and it drives the maintained reader's
malformed, forged, stale, replayed, revoked, conflicting, unavailable and
declared-boundary-collision results.

The simulation had no persistent keys, network authority, participant data, or
VPS dependency. Its output is explicitly `simulation: true` and
`qualified: false`, `simulation_result: passed`, caller-declared
`declared_source_revision`, and a `receipt_digest`: qualification here means no
public claim, not failure of the H4-6C acceptance criterion. The caller retains
its JSON receipt outside the repository.

The current bounded declaration encoding is JSON object
`ardents-public-control-evidence-v1`, limited to 1 MiB and decoded with no
unknown fields. It contains a candidate identifier; `transition` generation,
predecessor SHA-256 identity, expiry, revoked/conflicting flags; five custody
actors and `3`/`4` thresholds; Candidate View Epoch/input-log/materialization
digests and two auditor outputs; two builders; two auditors; and one or more
packages binding artifact/source/dependencies/recipe/SBOM/qualification digests
to two matching builder attestations. Each actor declares id, Ed25519 public
key, operator,
organization, administration boundary, and an evidence digest. The reader
compares all declared boundaries, checks expiry against `--at`, and rejects a
generation below `--audit-floor-generation` or a predecessor that differs from
the explicitly supplied external-audit predecessor identity.

This declaration encoding is not an authority wire format and has no signature
or endpoint-acceptance semantics. Its values are only content-addressed claims
until the selected custody and independently corroborated evidence supply a
future signed operation format.

## Transition matrix

| Transition | Required public evidence | Failure boundary |
|---|---|---|
| routine Epoch/package successor | predecessor and successor `3-of-5`, exact inputs and full-auditor outputs | retain floor; no downgrade or replacement by source choice |
| custody/root rotation | predecessor and successor quorum plus outgoing/incoming roster evidence | halt affected new work if continuity cannot be verified |
| custodian compromise/loss | `3-of-5` successor excluding compromised key and retained incident record | no unilateral recovery; expired control fails closed |
| emergency disablement | `4-of-5`, scope, reason, issued/expiry times, and terminal effect | may stop; cannot install, retarget, seize, or persist past expiry |
| builder mismatch | both reproduced artifact digests and retained input differences | candidate blocked; do not substitute a different package |
| auditor disagreement/withholding | all received auditor outputs and missing-object identity | `conflicting`/`unavailable`; do not choose a winner locally |

R-124 and ADR-0055 own the accepted H4-6C evidence. ADR-0060 retired its
maintained generator and command without changing the historical schema or
receipt. A future public-operation claim needs a new Product Owner decision and
must not be inferred here.

## Historical H4-6D controlled project-control transitions

The retired command
`ardents-control simulate-public-control-transitions --source-revision LOWERCASE_40_HEX_COMMIT`
was a distinct local, non-authorizing simulation. Its evaluator had no network,
Endpoint root, retained authority, or fallback source. Historical receipts
remain versioned with `simulation: true` and `qualified: false`.

| Input condition | Required outcome |
|---|---|
| continuous one-generation overlap | `overlap-accepted` |
| expired control | `stop-expired` |
| revoked control | `stop-revoked` |
| incompatible generation | `stop-incompatible-generation` |
| candidate below retained floor | `stop-rollback` |
| declared distributor unavailable | `unavailable-distribution` |
| live disable-only emergency | `stop-emergency-disabled` |

The receipt additionally recorded rejection of overlap without continuity,
emergency scope escalation, and expired emergency. Neither accepted nor stopped
simulated result selects an alternate source, lower generation, Route, or
Endpoint action. R-125 and ADR-0056 own this H4-6D evidence; ADR-0060 retired
the generator and command. It is not public control or Public Beta
qualification.
