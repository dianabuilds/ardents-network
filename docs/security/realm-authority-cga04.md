# Realm Authority CGA-04 membership and fencing contract

CGA-04 extends the accepted single-authority generation lifecycle with one
bounded channel membership mutation. It does not implement deployment
isolation, accept ADR-0013, qualify a production topology, or change the
canonical capability qualification.

## Protected procedures

The protected Operator Interface exposes two CGA-04 procedures and the
Application Interface exposes neither:

- `ChangeChannelMembership` uses
  `realm.channel.membership.change` on the exact
  `realm/<RealmID>/channel/<ChannelID>` resource;
- `SubmitDeploymentFenceEvidence` uses
  `realm.channel.membership.fence` on the same exact channel resource.

Both require a directly authenticated Operator Principal with
`Actor == Effective`; Delegation, sibling actions, parent resources and
wildcards do not imply authority. Product Policy can fail both procedures
closed through `policy.disable_realm_channel_membership`.

The existing delivery, activation-commit and active-acknowledgement procedures
retain their exact delivery/operation resources. When their retained operation
is a membership mutation, the membership Product Policy gate is used.

## Add and remove truth

Every accepted add or remove creates one fresh random secret and the next
strictly monotonic generation:

- add delivers the next generation to every existing member and the candidate;
  the candidate bundle is explicitly authority-signed as a candidate bootstrap
  and contains no previous-generation grant or secret;
- remove delivers the next generation only to survivors and includes an
  authority-signed revocation for the removed member's current grant;
- the complete next-generation sender snapshot contains exactly the intended
  recipients;
- the old generation becomes receive-only for the bounded drain after the
  signed activation checkpoint; publishing never rolls back.

The retained membership transition is versioned and bound to the operation,
target and authority sequence:

```text
add:    removed -> candidate -> active
remove: active  -> suspended -> removed
```

The pending state survives restart and checkpoint reconciliation. The terminal
state is written only when completion conditions are true. Authority
membership/generation truth changes at activation commit and recovery rolls
forward.

The recipient capability store applies the signed revocation atomically with
activation. Private-envelope admission already authorizes the sender before
touching the replay ledger, so traffic using the removed grant fails before
replay capacity can be consumed.

## Receipt and fencing boundary

An active receipt is accepted only with the deployment-owned
`approved_host=true` disposition. A valid receipt MAC proves possession of the
receipt key, not honest persistence or runtime behavior. A suspect, modified,
unapproved or noncompliant host therefore requires deployment fencing even
when it can return a valid MAC.

`DeploymentFenceEvidence/v1` is a bounded, non-secret assertion supplied by
the accepted DR-04 deployment procedure. Authority validates and retains:

- exact Realm, membership operation and target Principal bindings;
- a SHA-256 manifest digest, stable request ID and bounded reason;
- a canonical UTC observation no more than five minutes old;
- an asserted absolute clock skew no greater than 30 seconds;
- at most sixteen unique controls, each attributed to the directly
  authenticated Operator and bound to a SHA-256 protected receipt;
- the required `target_ingress_blocked`, `discovery_withdrawn` and
  `peer_id_denied` controls.

Authority does not execute or infer these controls. The evidence is accepted
only through the protected exact-resource procedure. A removed target must
have valid evidence. Every next-generation recipient must provide an approved
active receipt or be covered by valid evidence; an add candidate cannot be
substituted by fencing. Conflicting evidence for one target fails closed.

Removal completion is therefore:

```text
fresh survivor generation activated
  + every survivor active or explicitly fenced
  + removed target explicitly fenced
  + checkpoint and audit retained
  = completed
```

Partition, timeout, missing evidence, repository failure, excessive clock skew
or a missing survivor result leaves the operation
`activation_committed`/`recovery_required`; it never reports a false terminal
removal.

## Bounds, replay and audit

- one pending generation and one membership mutation per channel;
- one through 256 next-generation recipients;
- at most 256 Realm/channel members and at most 257 retained evidence targets
  for the single in-flight membership operation;
- no more than sixteen controls per evidence object;
- evidence reason at most 128 bytes and request ID at most 128 bytes;
- operation lifetime at most 24 hours and previous-generation drain at most
  30 days;
- the operation reserves `2 * recipient_count + 3` audit and audit-outbox
  records before emitting a secret-bearing result.

The original request, operation, delivery, envelope, audit and checkpoint
identities survive retry and restart. Identical evidence replays the retained
result; a conflicting assertion for the same target is rejected. Audit records
retain exact Actor/Effective attribution without evidence controls, Channel
Grants, selectors, secrets, receipt keys or private endpoints.

## Qualification boundary

The protected three-host integration scenario exercises add/remove, candidate
bootstrap, survivor activation, a partitioned removed target, forged
attribution and fencing completion. It is implementation evidence only. It
does not provide real-host DR-04 deployment evidence, accept Proposed
ADR-0013, or change `realm.channel-grant-authority` from `Q=no`.
