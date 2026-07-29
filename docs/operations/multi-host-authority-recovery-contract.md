# Multi-host Authority recovery contract

MR-03 adds one bounded workstation-side coordination command:

```text
ardentsctl topology recover --manifest FILE
```

The command consumes the stopped host-local restore procedure in
[`upgrade-migration.md`](upgrade-migration.md); it does not replace archive
verification, configuration review, process stop/start, or repository
provisioning. Its only mutation is the existing protected DR-03
`VerifyRestoredAuthority` acknowledgement after all fail-closed preconditions
hold.

## Protected binding

The command first applies the complete `ardents.topology/v1` admission
contract. Invalid input opens no connection. Each manifest Node selects the
same SSH host-key-pinned, signer-bound topology context used by `topology
status`. The designated Authority context additionally contains:

- the exact manifest-owned expected Realm ID;
- the manifest Authority state reference;
- the manifest Authority backup reference;
- the manifest Checkpoint Repository reference.

These are protected equality bindings. They are never paths supplied by the
manifest and never appear in ordinary output. Any mismatch fails before
Authority inspection.

## Clock preflight

The coordinator opens one isolated client and Session for each Node. A
protected `GetNodeRuntime` response carries the server's UTC observation. The
adapter brackets it with local request and response times, and Deployment
proves a conservative maximum inter-host offset interval no greater than the
manifest's fixed 30-second bound. Missing, invalid, slow, unauthenticated or
excessively skewed observations fail closed before Authority recovery.

The two member slots are observed in stable order and the Authority slot last,
so the Authority client can proceed immediately after the complete clock
proof. Every Node is bounded to 10 seconds and the aggregate to 30 seconds.
Each Session receives bounded lifecycle termination.

## Authority truth

The Authority client calls `InspectRealmAuthority` for the exact context-bound
Realm. A complete `ready/ready` state is an `already_ready` no-op.

Only this exact state can advance:

```text
phase: recovery_only
readiness: degraded
reason: authority_restore_verification_required
```

The coordinator acknowledges the observed Realm ID, Authority sequence and
checkpoint digest without accepting operator-edited values. The existing
Authority service verifies the strict ledger, signer binding, complete
immutable repository predecessor chain and unique latest head. The response
must preserve the exact Realm/sequence/digest tuple and become `ready/ready`.
Partial repository history, repository rollback, forked history, Authority
generation mismatch, repository loss, signer loss, corrupt state, protected
denial, timeout and ambiguity remain distinct stable redacted failure classes
and return `recovery_required`.

The command never creates, appends, repairs, resets, truncates, prunes, copies
or reconstructs Authority or repository truth. The immutable repository
capacity is exactly 65,536 accepted heads; exhaustion blocks security mutation
and requires a new Realm.

## Redaction and order

Ordinary output contains only the stable Authority slot, outcome, readiness,
phase and one stable reason. It excludes Realm ID, sequence, digest, host,
address, path, signer, Session and repository identifiers.

The reusable pure order projection keeps the Authority slot last for ordinary
compatible releases and first for Authority schema/protocol migration. Both
orders are serial and retain the complete verified Authority backup and
external head precondition. MR-03 performs neither rollout.

Real independent backup/WORM failure-domain and three-host recovery evidence
remain R3 qualification work. `deployment.multi-host` remains `Q=no`.
