# Realm Authority CGA-01 security and operations contract

Status: implementation evidence for independent architecture/security review.
This document does not qualify `realm.channel-grant-authority`; Q remains `no`.

## Boundary

CGA-01 implements one production Realm Authority instance for exactly one
Realm. The Realm Authority Principal is self-certifying and distinct from the
hosting Node Principal, Operator Principal, Waku Peer ID, SSH/TLS identities,
and filesystem ownership. None of those identities implicitly receives issuer
authority.

The only remote entry points are the protected Operator procedures
`CreateRealmAuthority` and `InspectRealmAuthority`. They require exact
`realm.authority.create` on `realm-authority-instance/primary` and
`realm.channel.audit.read` on the returned exact `realm/<RealmID>`,
respectively. The domain rejects delegated Actor/Effective pairs even if a
future admission configuration were to pass one. Product Policy may disable
genesis with `policy.disable_realm_authority_creation`.

These action, domain, resource-kind, and mutation declarations are method
options in the versioned Operator protocol and are loaded by the authorization
interceptor catalogue. The authority methods do not have a second handwritten
access-rule registration.

There is no Application Interface Authority service, signing-material route,
Channel Grant route, or administrative filesystem bypass.

## Provisioning

Enable the authority with four absolute, mutually separated paths:

```json
{
  "authority": {
    "enabled": true,
    "store_path": "/var/lib/ardents-authority/realm-authority.db",
    "store_key_file": "/etc/ardents-authority/store.key",
    "signer_file": "/etc/ardents-authority/signer.json",
    "checkpoint_repository_path": "/srv/ardents-checkpoints"
  }
}
```

`store_key_file` contains one standard-base64 encoded 32-byte key in a
protected regular file. `signer_file` is separately provisioned protected JSON:

```json
{
  "version": 1,
  "principal": "p1_<full-self-certifying-principal>",
  "private_key": "<raw-URL-base64 Ed25519 private key>"
}
```

The signer file is loaded and its Principal/public-key continuity is checked
before uninitialized authority readiness and again before genesis. It is never
copied into the encrypted Bolt authority database. Missing or unreadable
store-key/signer inputs produce stable unavailable readiness; they never cause
key generation.

The checkpoint repository must be outside the Node data directory and outside
all authority database/key/signer paths, on deletion-protected/WORM storage
administered with credentials independent from the authority runtime. The
daemon never creates its root and never falls back to a local directory. The
independent administrator must place this protected assertion in the mounted
root before startup:

```json
{"version":1,"retention":"worm","administration":"independent"}
```

The file name is `.ardents-worm-repository-v1.json`. Its presence is deployment
admission evidence, not a substitute for storage-enforced retention or
independent credentials. A missing mount, assertion, or protected directory
makes the authority unavailable. The repository contains only signed monotonic
freshness evidence; it cannot decide membership or sign a checkpoint.

## Transaction and recovery semantics

Genesis atomically commits RealmID, epoch `1`, sequence `1`, one operation ID,
the idempotency result, immutable audit-chain head, audit outbox record, and
the signed checkpoint reference to the encrypted ledger. The repository then
creates sequence `1` with create-if-absent. A final ledger CAS changes the
operation from `checkpointing` to `ready`.

The same request ID and payload returns the original result. Reusing the ID
with another payload or attempting a second genesis returns a stable conflict.
Crashes after ledger commit or after repository creation resume the same
operation and audit identity. Existing heads are never overwritten.

Startup validates the encrypted schema, RealmID, Authority Principal/public
key, epoch, sequence, audit head, checkpoint digest/signature, and the unique
repository chain. Missing or unreachable dependencies are `unavailable`.
Corrupt, lower, forked, non-monotonic, unexpected, or signer-mismatched truth
is `recovery_required` and blocks mutation. CGA-01 deliberately provides no
restore, repair, downgrade, migration, or authority-rotation command.

## Bounds and redaction

Schema v1 fixes one Realm, 256 Realm members, 1,024 active channels, 256
members per channel, one pending generation, one receive-only previous
generation, and four outstanding deliveries per member/channel. CGA-01 leaves
member and channel collections empty. Inputs, persisted containers, versions,
unknown protobuf/JSON fields, signatures, and compare-and-append sequences fail
closed when invalid or exhausted.

Inspect and CLI output expose only version, RealmID/class, epoch/sequence,
current generation (`0` in CGA-01), genesis-operation deadline, checkpoint
digest, phase, readiness/reason, and bounded counts. They do not
expose membership, Authority Principal/public key, private key material,
Channel Grants, selectors, receipts, endpoints, or plaintext payloads.
Authority metrics contain only bounded phase/readiness/reason labels and
numeric counts:

- `ardents_realm_authority_readiness`
- `ardents_realm_authority_phase`
- `ardents_realm_authority_audit_outbox_depth`
- `ardents_realm_authority_members`
- `ardents_realm_authority_channels`
- `ardents_realm_authority_pending_operations`

The genesis audit record and its Actor/Effective attribution are retained in an
immutable hash-chained section of the encrypted authority ledger. The same
transaction also places that record in the delivery outbox. Successful
delivery acknowledges only the outbox copy; it never deletes or truncates the
authority-owned audit record. The durable diagnostics adapter receives stable
audit/operation IDs, hash continuity, and timestamp, but no key, grant,
selector, checkpoint body, or Realm-specific metric label. Delivery is at
least once. Failure leaves the record pending and reports
`authority_audit_unavailable`.

## Review scope and retained evidence

Independent review should verify the authority module, protected Operator
mapping, encrypted store, external signer adapter, immutable checkpoint
repository, daemon composition, redacted CLI/metrics, and real-store
crash/restart tests. The canonical checkpoint serialization vector is
`internal/authority/testdata/checkpoint-genesis-v1.json`.

This document remains the CGA-01 genesis contract. CGA-02 initial-generation
delivery is specified separately in `realm-authority-cga02.md`; later
rotation/activation, membership, fencing, renewal, restore and qualification
remain outside this document.
