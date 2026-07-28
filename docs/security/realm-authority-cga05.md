# Realm Authority CGA-05 renewal and channel-class contract

CGA-05 extends the accepted Authority lifecycle with bounded grant renewal and
independent channel classes. It does not accept Proposed ADR-0015, create
Application conversations, define group policy or message semantics, restore
an Authority, qualify a release, or change canonical capability qualification.

## Independent channel classes

Authority supports exactly four Channel Grant classes:

- `realm.discovery`;
- `data.exchange`;
- `channel.application`;
- `realm.capability_control`.

Every channel has its own random Channel ID, 256-bit secret, grant IDs,
generation sequence, member snapshot, selectors, replay namespace and exact
audit resource. A member may be present in more than one channel, but removing
it from one channel does not remove Realm-level membership truth while another
current channel still contains that Principal.

All class-bearing mutations pass the existing operation policy gate and a
separate Product Policy class gate. The generic gate is evaluated before
ledger lookup; the class gate follows only after a validated exact channel or
signed delivery binding is known. A denied class cannot consume randomness or
create ledger, cryptographic or replay state.

`channel.application` is only a generic class consumed by DR-01 Product
Policy. Authority does not assign conversation identity, select recipients,
interpret group policy, or inspect Application messages.

## Renewal lifecycle

`RenewChannelGrants` reuses the accepted
`realm.channel.generation.rotate` action on the exact
`realm/<RealmID>/channel/<ChannelID>` resource. It is admitted only when every
current grant is unexpired and has at most 24 hours remaining.

An accepted renewal:

- always issues the next generation with a fresh random secret and fresh grant
  ID for every current member;
- grants exactly 30 days from the canonical Authority clock;
- seals a complete, bounded sender snapshot for every recipient;
- uses the existing durable installed receipt, signed checkpoint activation,
  runtime adoption and approved-host active receipt flow;
- preserves the previous generation as receive-only for the requested drain,
  capped by old-grant expiry;
- retains operation kind `channel_renewal` and an explicit renewal marker.

The request identity and complete result survive retry and Authority restart.
A conflicting payload for the same request ID, renewal before the threshold,
renewal after expiry, an incomplete recipient set, a pending generation, an
invalid duration or exhausted ledger/audit capacity fails closed.

## Readiness and expiry

The protected Operator `InspectChannel` procedure uses
`realm.channel.audit.read` on the exact channel resource. It returns only
redacted class, generations, member count, grant expiry, renew-by timestamp and
stable readiness/reason values:

- `ready` with no reason while the channel is healthy;
- `degraded/channel_grant_renewal_due` inside the renewal window;
- `degraded/channel_grant_pending` while a generation is pending;
- `unavailable/channel_grant_expired` at or after grant expiry.

Member-local generation readiness uses the same
`channel_grant_expired`/`channel_grant_pending` reasons and never reports an
expired current grant ready. Expiry or pending state affects only the selected
Channel ID; sibling channel and Realm readiness remain independent.

No status or audit field exposes a capability secret, grant ID, selector,
receipt key, delivery envelope, private endpoint or member list. Channel audit
records retain the exact class and generation together with the established
Actor/Effective, operation and resource attribution.

## Bounds and qualification boundary

The existing limits remain authoritative: at most 1024 active channels, 256
Realm members and 256 deliveries per bounded operation, at most 4096 retained
operations/audit records, at most one pending generation per channel,
operation lifetime at most 24 hours and previous-generation drain at most 30
days. Initial generation now admits multiple independently bounded channels
without weakening those limits.

Unit evidence covers four-class selector/key/generation/replay/audit
separation, class policy denial, renewal threshold and restart-idempotency, and
isolated expiry. Protected-process integration completes renewal through
install, acknowledgement, checkpoint activation, runtime adoption and terminal
acknowledgement. This is implementation evidence only:
`realm.channel-grant-authority` remains `Q=no`.
