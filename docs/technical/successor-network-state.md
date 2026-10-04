# Network domain

Network owns which authenticated network generations, profiles and assignments
may be treated as current. Its domain model is `internal/successor/network`.
Its application owner is `internal/successor/network/state`. The new application
uses its own authentication, Source and journal adapters and does not import the
old `internal/network` implementation. Retained byte mechanisms preserve named
wire and recovery obligations. The old tree and its consumers remain until a
separately selected final cutover; their integration is outside this work's
regression scope. Execution status and verification receipts belong to issue #493.

## Model and invariants

- `EpochHistory` represents the retained current/pending relation and conflict.
  Reconciliation examines every authenticated observation before selecting a
  successor. A higher numbered candidate cannot hide an earlier conflict.
  Pending activation is bounded by completion time; a future genesis cannot
  become a recoverable pending state without an active predecessor.
- `ProfileHistory` represents the sole accepted role-profile digest and its
  conflict for one generation/Epoch. An exact repeat is unchanged; a second
  digest preserves both identities and refuses work. Neither reopening nor
  repeating either digest resolves a conflict.
- `Membership` binds each signed assignment to one exact authenticated Node
  Record and the Epoch-assigned role. It belongs to a specific profile binding.
  Missing, duplicate or mismatched evidence is refused. Expiry of one member
  does not erase evidence or retire unrelated members. Lookup by transport key
  refuses ambiguity even if one matching assignment is expired or has a role
  unsuitable for the consumer.
- `TrustedTime` represents one confirmed observation from wall time, monotonic
  continuity and independent clock evidence. Network owns the two-second
  confidence bound and persisted whole-second floor. It never lowers that
  floor to accommodate a clock correction. The application owns sampling.
- `AcceptedState` binds the authenticated Epoch, accepted generation, profile
  identity/window, copied public issuer inventory and profile-bound membership.
  Its `ProfileFacts` requires the selected issuer's exact signed Node/duty
  relation; Admission's format owner retains SPKI/cohort validation. The public
  profile retains its signed interval even when a member becomes unavailable.
  It produces a `RuntimeView`
  only with confirmed current time. The view has private construction state,
  invalid zero value, copied public facts and bounded use; it is not a lease.
  `RuntimeView.Profile` and members come from the same accepted value; input
  and returned inventory slices cannot mutate that value. The application
  checks a particular issuer's live duty before issuance; that check is not a
  requirement for unrelated receivers to verify current-profile tokens.
- `RetainedDuty` identifies a Node assignment held by an execution owner.
	`RuntimeView.RetainDuty` constructs it from that same authenticated observation,
	including the exact Epoch interval, without a separate diagnostic read.
  `RuntimeView.MatchDuty` checks that assignment against current Network facts.
  Its Epoch and Record intervals must exactly match authenticated values.
  A live member may have a shorter interval than the signed profile; that
  interval bounds its work without rewriting the profile or token cohorts.
  Purpose, path eligibility and execution lifetime remain with consumers.

These are value objects and immutable aggregate values. Evaluating a proposed
change does not mutate committed authority. The application commits the
resulting decision before publishing new runtime facts; recovery rebuilds the
same domain relation from authenticated persisted evidence.

## Boundaries

The application accepts only `ardents-route-v3` with an explicitly pinned
profile authority. Empty configuration selects that same profile; earlier
profiles cannot select live execution. Named historical grammar readers remain
for authenticated predecessor/recovery evidence. Retired current or pending
schemas refuse without replacing retained floors.

`CurrentRuntime` is the sole member-authority observation. The separate
`Current` Snapshot reports intake and finite-Source diagnostics and the local
Source exclusion identity; it has no member inventory or Node lifecycle view.
`AcceptClosedProfile` returns a `ProfileReceipt` containing committed identity,
not permission to work. `RetainedDuty` contains exact identity and intervals;
caller-declared freshness/conflict/presence flags do not grant authority.

State borrows `PermitWork` before effects and observation, outside its lock.
The callback may be concurrent and cannot call back into State. Process resource
sampling, protect/drain policy and process monitoring stay with the application
owner. State joins only its own Source and refresh work.

Unsigned record, Epoch and profile preparation remain explicit contracts for
subsequent provisioning composition, sharing the canonical grammar and
Candidate View rules. They hold no private signing key, cannot change a floor,
and cannot accept a generation. Profile signing is absent from new Network.

`CandidateView` owns individual eligibility, ordered rejection reasons,
collision exclusion, canonical Node ordering and family totals. The format
adapter supplies decoded public facts and signature examination results;
malformed input is the zero authentication value. Wrong Network precedes bad
signature, individual refusals precede collision counting, and every eligible
participant in a collision is excluded. An invalid-signature record cannot
poison another member. Duplicate Node identity precedes duplicate generation;
the retained generation rejection code remains reserved without a redundant
generation-counting pass. These priorities preserve rejection commitments.

The same View computes verification and unsigned initial preparation summaries.
Network assigns each whole family by the retained assignment transcript;
encoding, signature verification, accepted/rejected Merkle commitments and
materialization proofs remain in the Epoch adapter. Earlier profile and Carrier
names are interpreted for retained compatibility, not offered as new runtime
protection modes. The exact `candidate_evaluation.go` and `assignment.go`
bridges grant no imports to other Epoch files or back into the old tree.

| Owner | Responsibility |
|---|---|
| New Network domain | Candidate and acquisition policy; generation and profile acceptance, conflict/rollback, trusted time, exact membership and retained-assignment binding; separate local participation restrictions |
| Epoch and closed-profile authentication adapters | Canonical grammar, signatures, commitments and verified public document facts; successful verification alone does not select current state |
| State application adapter | Exclusive operation ordering, loading verified history, invoking domain decisions, committing their effects, publishing observations under one read lock, and coordinating joined background work |
| Durable adapter | Exclusive filesystem root, immutable bytes, transactions, sync and recovery reads; no decision about authoritative generations |
| Source transport | Bounded authenticated retrieval/serving and framing; no candidate acceptance or winner selection |
| Local participation restrictions | Separate durable local role/exposure exclusions and retention until their using work joins; not the global Epoch aggregate |
| Process resource owner | Resource observation, protect/drain signals and process sample rendering |
| Node / Endpoint / Route | Role/Purpose rules, participant and path selection, operation binding, transport, deadlines and joined execution |
| Admission / Hosting | Token issuance/spending and physical budget/reservations respectively |

The domain imports only the standard library. It cannot call State, Node,
Endpoint, Route, Admission or Hosting. The new State application consumes the domain only
through the exact adapters listed in package-map and enforced by architecture
tests. This migration boundary grants no general access to successor packages.
New runtime composition imports only its registered application or evidence
package. It gains no physical-root access from that import.
The State application cannot import token debit, private signing or Hosting.

The adapters have independent reasons to change: `epoch` owns authenticated
Epoch/View grammar and proofs, `closedprofile` the signed role profile grammar,
`source` the finite TLS protocol, `state/durable` the global root transaction
mechanics, and `duty` the distinct local restriction journal. The two physical
roots cannot be collapsed into one lifetime or transaction. Keeping these
byte mechanisms preserves recovery obligations; it does not preserve their
former ownership of Network policy. New command composition exercises the new
owner. Future Node, Endpoint and Route replacements consume explicit observations
and retained-duty contracts; they do not require changes to old consumers now.

The closed-profile public token-key adapter is the exact permitted consumer of
Admission's `issuerprofile` leaf. It delegates canonical SPKI validation there;
no second production RSA-PSS parser is retained in Network. This gives the
format adapter no quota, stock, spending, private keys or signing access and
does not grant an Admission import to the pure Network model.

## Application ordering

`LocalParticipation` is the separate installation restriction aggregate. It
owns producer-scoped replacement, identity/family collision decisions, ordinary
Initiator and same-producer Source-pair exclusions, finite limits and expiry.
Other producers may prune expired time-held facts but cannot expire or remove
a live Direct Source. Its producer releases that live fact only after dependent
work joins. Node restriction classification cannot choose the ordinary Initiator
exemption or turn an assignment into a Direct Source; it retains the earlier
Epoch/Record bound. Copied facts cannot mutate either aggregate.

The exact `duty/participation_policy.go` adapter applies these decisions to the
existing role journal. Root leases, watermark/generation continuity, canonical
schema and strict version-1 conversion remain with that physical owner. Retired
class names are recovery inputs, not new role modes. Record, producer, Source
capacity and conflict refusals remain separately identifiable to consumers.

`Acquisition` owns the immutable two-Source/four-selector cycle, original
deadline and order, exposure-history bound, consumed and interrupted attempts,
terminal outcomes and bounded backoff. A received BY_DIGEST response without a
recorded verified outcome remains consumed on recovery. Resumption uses the
recorded deadline and seed; expiry terminalizes the cycle and records failure
rather than extending its contact horizon. Transition proposals own their
copied exposure slices. Invalid Source indices and new exposure growth refuse
before contact; terminal outcome values and exact digest selectors are checked.
Closure also marks consumed attempts with missing results as interrupted;
absence of a new observation cannot erase an already recorded outcome. A wave
with no valid observation enters backoff instead of clearing failure history.
The domain itself refuses a new cycle before its durable retry time.

The State adapter translates this value through `acquisition_history.go` into
the existing distribution journal. Its sequence, Epoch/profile facts and time
floors retain one ordered commit boundary. Source transport supplies bytes;
the application draws randomness, commits before contact and retains local
live exposure guards until its dependent work joins. Status/outcome numbers,
journal bytes, jitter and whole-second deadlines remain compatibility duties.

The live State application's `CurrentRuntime` supplies coherent authenticated inputs while holding
its existing lock. It performs durable profile reads before sampling trusted
time. It asks the domain to bind and observe the accepted state; consumers
cannot combine independently read profile and diagnostic snapshots into a view.
State releases its lock before network I/O, token debit or Hosting operations.
Consumers obtain another observation at their existing recheck points.

Source and offline intake authenticate candidate content, signatures and
committed View before history reconciliation. Successor checks must not discard
a signed second identity at a retained number before the domain sees it. Live
schema, requested object/index and time checks still precede acceptance; invalid
signatures or a different Network cannot create an authenticated conflict.

A discovered conflict must be committed or the live owner must retire. The
application retains the original failure and joins its work during Close.
An uncertain publication never becomes accepting authority. Source publication
must preserve local exclusion guards for any generation that may still serve
work, including predecessors with active users. Separating domain decisions
from effects does not change this ordering or permit early guard release.

A new generation never extends an existing operation deadline, selects a new
path implicitly, refunds durable token debit or prematurely releases a Hosting
reservation. Those effects remain with their respective owners.

`cmd/ardents-next/network_authority.go` maps one fresh new State observation to
new Admission facts. Its issuer provider checks the exact profile-selected
Node, duty, role and provisioned signing public key. Its receiving provider
checks the retained generation/Epoch/profile/Node/duty binding and separately
bounds work by the member's validity, profile and retained horizon. It preserves
the original signed profile and all cohorts. A current profile cannot revive
an expired member. The command closes Admission-owned roots before its Network
root and retains closure failures. There is no old Node adapter on this path.

## Contracts for subsequent domain replacements

| Consumer | Network contract | Consumer-owned responsibilities |
|---|---|---|
| Admission composition | Fresh `CurrentRuntime`, copied `ProfileFacts`, exact live issuer/member lookup | Translate public facts; compare provisioned signer; issue/debit/verify/spend in Admission; no fallback on authority failure |
| Node | Fresh observation, `RetainDuty` and future `MatchDuty`; separate local participation owner | Check role, Purpose, local exclusions and private identity; start/stop/join duties and keep physical reservations until join |
| Endpoint | Same-generation copied membership/profile; ambiguous identity/key lookup refuses | Select participants and recipients; retain operation identity, channel binding and original deadline; recheck before and after durable presentation |
| Route / Carrier | Current selected peer identity, key, signed address/Carrier facts and bounded validity | Apply path/Purpose rules, authenticate the actual peer/channel, perform transport and enforce original lifetime/byte limits |
| Hosting composition | No Network-to-Hosting call; current authority is checked by the work application | Reserve physical work and termination before effects; transfer ownership once; release only after joined work |

Observation is a value at a checked time, never a live lease. Each consumer
obtains another observation at admission and post-commit boundaries. A retained
binding cannot be silently rebound to a successor. A change refuses new effects;
the execution owner cancels and joins dependent work. Observer callbacks cannot
reserve Hosting, execute transport or call back into the Admission owner whose
lock is held. Network never calls consumers under its authority lock.

## Verification boundary

Domain behavior tests exercise reconciliation, ambiguity, observation expiry,
returned-data isolation, profile conflicts and clock evidence without clocks
or I/O hidden in the model. New signed-input State tests exercise the
application boundary, real durable roots, failed/sync-uncertain publication and
reopen. Integration uses only opened new Network, new Admission and new Hosting,
real blind issuance and durable roots. It proves pre-spend refusal, retained
burn after a signed successor, root recovery and reservation retention through
joined socket work. It does not qualify a Route or either selected Carrier.
Those integrations belong to subsequent domain replacements.

This change does not alter wire bytes, signed identities, stored generation
names, root formats or recovery rules. Existing physical data mechanisms may
be reused without preserving their former ownership of domain decisions.
