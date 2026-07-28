# Realm Authority CGA-03 Security Contract

## Scope

CGA-03 rotates one existing channel through a fresh pending generation,
installed receipts from every approved member, one signed activation checkpoint
and approved-host active receipts. The immediately previous generation remains
receive-only until a bounded drain deadline. Membership changes, deployment
fencing evidence, renewal, restore and release qualification remain outside
this slice.

The protected Operator Interface adds:

- `realm.channel.generation.rotate` on the exact
  `realm/<RealmID>/channel/<ChannelID>` resource;
- `realm.channel.activation.commit` and
  `realm.channel.generation.activate` on the exact
  `realm/<RealmID>/operation/<OperationID>` resource;
- `realm.channel.activation.acknowledge` on the exact generation-delivery
  resource.

There is no Application Interface equivalent. Every mutation requires direct
Actor/Effective equality and Product Policy admission through
`policy.disable_realm_channel_rotation`. The generic delivery
install/acknowledge RPCs retain their CGA-02 actions and exact resources.

## Generation state and cutover

Rotation generates a new 256-bit channel secret, selector and grant identity.
Authority and member stores admit at most one pending generation. Installing
the sealed bundle records pending state but deliberately makes the member not
ready; only the signed activation binds the checkpoint digest and promotes the
stable capability reference.

After activation, publish resolves only the new generation. The immediately
previous grant may resolve only subscribe or Store-fetch use and only before
the drain deadline. Discovery and private transfer subscribe to both bounded
topics during that interval while every new envelope is sealed with the
current generation. A running transfer exchange atomically establishes the new
current/previous subscriptions before the member RPC releases its active
receipt; failed resubscription retains the old live subscription and returns a
retryable failure. The durable activation remains explicitly
`runtime_adopted=false`, so member readiness stays false until a successful
retry commits runtime adoption.

The Authority ledger retains explicit versioned current, pending and one
previous grant snapshot. Drain is capped by both the requested limit and the
old grant expiry. A second rotation for the channel conflicts until the first
operation completes.

## Receipts and deployment disposition

Installed and active receipts are HMAC assertions under the per-delivery key.
Authority rechecks their complete sealed binding, phase, activation sequence,
timestamps and deadlines. A valid MAC still proves only possession.

`approved_host=true` is therefore an explicit deployment-owned disposition, not
cryptographic fencing evidence. Without it, active acknowledgement is denied.
The CLI requires the explicit `--host-disposition approved` input and never
synthesizes approval from receipt possession.
CGA-04 must supply supported fencing evidence for suspect, noncompliant or
removed members; CGA-03 does not claim that a receipt can replace fencing.

## Crash, replay and recovery

Rotate, installed acknowledgement, activation commit and active
acknowledgement use the same ledger/checkpoint compare-and-append transition.
Tests inject crashes after the ledger commit and after checkpoint creation at
every transition. Restart reconciles and replays the same operation, delivery,
activation, audit and completion identities.

If activation state reached the Authority ledger before checkpoint publication,
recovery rolls forward and publishes that retained checkpoint. It never
restores the old current generation or decreases Authority sequence. Member
activation likewise commits before returning and replays the identical active
receipt after restart. Operation-keyed activation history preserves retries
across later strictly monotonic rotations while the channel-keyed record tracks
only the latest checkpoint.

Every transition checks audit-log and audit-outbox capacity before signing or
mutating. Before creating pending state, rotation reserves the full
`2 * member_count + 2` audit capacity required for rotate, all installed
receipts, activation commit and all active receipts. Rotation also checks
operation and rotation-record capacity. Insufficient completion capacity
returns resource exhaustion without stranding a pending generation.

## Disclosure and qualification

The CLI transfers attestations, sealed rotations, activations and receipts only
through bounded private files. Human and JSON output contains identifiers,
phase and bounded status, never plaintext grants, channel secrets, selectors,
receipt keys or delivery private keys.

The slice has unit, protected two-host integration, negative authorization,
restart and race coverage. It does not change canonical qualification:
`Q=no`.
