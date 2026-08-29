# Public-control candidate evidence contract

Status: **H4-6C readiness contract; no public-control candidate is selected.**
This defines the bounded evidence a future independent auditor must inspect.
It grants no Endpoint, release, Network Epoch, Namespace, or emergency
authority, and it does not claim that the current project has independent
participants.

## Candidate identity and custody

One candidate has a stable content-addressed identity and a public roster of
exactly five custody members. Each roster entry binds a durable public identity,
an authority public key, the operating organization, the administrative
organization, the hosting/administration environment, and a digest/reference to
independently reachable custody evidence. A member that shares any of those
control boundaries with another member is one family, not two members.

Routine Network Epoch and public executable authorization require `3-of-5`.
An emergency requires `4-of-5`, carries a finite expiry, and is limited to
stopping unsafe new work, terminating an unsafe build, or withdrawing/draining
an unsafe duty. It cannot seize or release a Name, rewrite a live destination,
select a Route, decrypt traffic, or install code.

Every roster/role successor names its exact predecessor candidate identity and
requires both predecessor and successor quorums. A retained reader floor
rejects lower or divergent generations. A lost/compromised member uses a
published successor procedure; it cannot be silently replaced by a project
operator. Recovery, removal, and emergency records are public, finite, and
independently verifiable before they take effect.

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

The future selected profile must set byte, count, execution-time, and storage
limits from its capacity evidence. Functional Alpha's private small-Epoch
limits are not a default public limit.

## Build and audit evidence

Each public package retains the artifact digest, source revision, resolved
dependency digests, build recipe, SBOM, selected qualification identity where
claimed, and two matching independently operated builder attestations. Neither
builder may be a Release Targets custodian. Each attestation identifies its
build trust boundary and the exact inputs/outputs it observed; a CI brand or
signature alone is not independence proof.

At least two full auditors, independent from each other, the Epoch custody
threshold, Release Targets custodians, builders, and audited Candidate operator
families, publish their complete evidence-package digest, reconstruction output
digest, tool/source revision, and result. Conflicting auditor results block the
candidate; a later success does not erase the earlier record.

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

`declared-evidence-complete` never means public control is qualified. The
reader cannot infer real independent administration from names, addresses,
signatures, a project VPS, local Docker, CI, or a Product Owner walkthrough.
Only independently corroborated custody, builder, and auditor evidence closes
that final outcome.

## Project-controlled mechanics simulation

`ardents-control simulate-public-control` is a deliberately non-qualifying,
short local simulation of the mechanical contract. Each invocation generates
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

The simulation has no persistent keys, network authority, participant data, or
VPS dependency. Its output is explicitly `simulation: true` and
`qualified: false`. It exercises only the stated mechanics; it is not evidence
that the simulated identities, Docker instances, VPS instances, or their
operators are independent.

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

R-124 owns the research evidence. A future accepted custody technology and
operation need a superseding ADR and real participant evidence before this
contract can be marked selected.
