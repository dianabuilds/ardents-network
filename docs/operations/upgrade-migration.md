# Upgrade, Backup, And Recovery Guide

## First-Release Greenfield Identity Contract

Ardents has no released Operator/Application bearer authentication or truncated
`p_` Principal state. The first release creates canonical `p1_` Principals,
independent `d1_` device identities, Principal sessions, Access Grants, and
one-use enrollment tickets directly. There is no supported `p_ -> p1_` epoch,
dual-ID alias, credential coexistence state, bearer retirement window, or
identity-migration command.

Operator and Application protected calls use their distinct Principal session
schemes on their distinct protected Unix listeners. A failed or unknown scheme
is denied without fallback. Provisioning creates no permanent Operator or
Application token. The first Operator uses a short-lived one-use Bootstrap
Ticket; an Application installation uses its own short-lived one-use enrollment
ticket. These tickets and the short-lived sessions are current security
mechanisms, not compatibility credentials.

Runtime configuration comes from one canonical versioned configuration
document. Unknown versions and fields, obsolete token-file inputs, alternate
environment-only configuration, `p_` identifiers, and duplicate pre-release
wire/storage fields fail closed. Operators must correct the configuration or
state; the daemon does not translate or guess.

Accordingly, this guide's upgrade, rollback, and recovery procedures apply to
future released schemas and operational failures. They do not define an
identity cutover from bearer authentication or `p_` identifiers, and they must
not be used to justify retaining a dual runtime path.

## Release Identity

Use only a release artifact whose `ardentsctl version` or `ardentsd --version`
matches the version, commit, build date, OS, and architecture in its provenance
statement. Verify `SHA256SUMS` before installation and retain the previous
immutable image digest until rollback acceptance.

## Rolling Upgrade

1. Read the release notes, persisted-schema changes, dependency exceptions, and
   platform support.
2. Stop one Node and create a verified consistency-group backup.
3. Record the current image digest and cluster manifest.
4. Durably add that Node to the rollout transaction journal before recreation.
5. Recreate and start that Node with the new immutable image.
6. If recreation, start, or readiness fails, compensate the current Node and
   every previously changed Node to the journal's single fallback digest.
7. Require protected local API readiness, canonical network readiness,
   Diagnostics, and retained Principal/grant state before continuing.
8. Repeat for remaining Nodes; do not upgrade all bootstrap Nodes together.
9. Retain backups and the previous image until the observation window closes.

`./ardents.ps1 upgrade` automates the supported single-host Compose sequence.
If a compensation attempt is interrupted, the next `upgrade` or `rollback`
invocation with the same project and state directory finishes that compensation
and exits; run the intended command again only after the journal is cleared.
For a native systemd Node, use the release's documented
`scripts/install/linux.sh upgrade` command when present.

## Consistency-Group Backup

A whole-state backup is taken while the Node is stopped and drained. It includes
`ardents.db`, `identity-access.db`, key files, the canonical configuration, and
any released-schema upgrade marker from the same state directory. The databases
are independently transactional; no backup or restore procedure may claim one
transaction across both files.

The daemon owns the only live `identity-access.db` handle. Backup, upgrade, and
recovery helpers stop and drain the daemon before copying the whole group and do
not open that live file independently. A repository checkpoint made through the
daemon is one `identity-access.db` read-transaction boundary only; it is not a
whole-state backup.

Backups and manifests contain no session secret, raw Bootstrap/Application
Enrollment Ticket, private key, proof, or channel secret. Key files remain part
of the protected consistency group and retain their platform permissions.

### Realm Authority recovery-only restore

The native stopped backup includes the protected authority archive but never
the independently configured checkpoint repository. Before accepting a
backup, retain its immutable archive/manifest hashes and a redacted authority
status showing the same sequence/digest as the repository head.

Restore only into empty stopped targets:

1. run `scripts/install/linux.sh restore --archive PATH`; the service remains
   stopped after archive, path and per-component digest verification;
2. set `authority.recovery_only` to `true` in the reviewed configuration;
3. start the matching commit and inspect the exact Realm status;
4. require `recovery_only/degraded` with
   `authority_restore_verification_required`; any signer, store, schema,
   predecessor or head failure stops this procedure;
5. run
   `ardentsctl authority recovery verify --realm-id ID --authority-sequence N --checkpoint-digest DIGEST`
   using the exact redacted values from that startup;
6. retain the JSON response, archive/manifest digests, repository history/head
   evidence and binary commit together;
7. stop the daemon, set `authority.recovery_only` to `false`, restart and
   require the same ready sequence/digest.

Never copy the repository into the restore target, recreate its marker/head,
lower a sequence or retry against a different digest. If the unique latest
head or old signer cannot be proved, leave the old Realm stopped and create a
new Realm with new member enrollment.

### Local realm v2 migration and downgrade

Migration is fully stopped. Retain complete pre-migration authority and Node
backups, reconcile every `ardents.local-realm-node/v2` record and protected
receiver grant against the one `ardents.local-realm/v2` authority, remove the
shared old-manager path, and record its fencing evidence digest. Use
`provision.BuildLocalV2MigrationEvidence` while the new authority store is
empty; keep the returned evidence handle open through `MigrateLocalV2` so its
OS state-directory lock proves exclusive stopped-manager ownership. The
adapter reads and authenticates the protected Node capability databases rather
than accepting an operator-authored grant list. Unknown
fields/versions, a missing or extra member, any grant mismatch, signer
mismatch or occupied new repository head aborts before the new store is
created.

After import, the new authority remains
`authority_migration_rotation_required`. Complete fresh discovery and data
delivery/activation workflows for every member (or reviewed fencing), require
each repository compare-and-append, and retain both terminal operation and
checkpoint results. Do not enable production multi-host operation until
readiness is `ready`.

For a planned authority transition, preprovision the successor signer before
the dual-signed operation and distribute the proof through
`AdoptMemberAuthorityTransition` before successor-signed channel delivery.
Keep the successor available as `SuccessorSigner` until its checkpoint is
visible in the independent repository. A temporary repository outage is
resumed with the same exact request or by restart; never start a different
transition or discard the locally committed checkpoint.

Downgrade stops the new software and restores the complete verified
pre-migration backup with the exact old binary. Do not edit the new schema,
copy individual grants, reattach the shared old manager or run old and new
managers together.

For deterministic local evidence, run
`tests/ci/cga06-recovery-migration-gate.ps1 -OutputDir PATH` from a clean
matching commit. Retain its JSONL files and `manifest.json`; the manifest binds
all three drill hashes to the full commit and records that qualification
remains unchanged. CGA-07 still owns real-host and release qualification.

## Persisted-Schema Upgrade

Released schema changes are versioned, transactional, and fail closed on
unknown versions. A release that changes a persisted schema must document:

- the exact old/new schema versions and changed buckets/records;
- preflight and corruption checks;
- every durable interruption point and resume behavior;
- whether an older binary can read the new schema;
- the required stopped-Node backup and restore drill;
- redaction checks for backup manifests, logs, and errors.

An older binary must reject a newer incompatible schema. Never lower a schema
marker, delete buckets, copy individual grants between versions, or attach an
old binary to a partially upgraded database. Transaction rollback on an update
error and whole-state restore after a failed release are both required safety
mechanisms.

### Principal access schema version 1

The first release creates one greenfield `identity-access.db` schema version 1.
It includes the enrollment, Credential, grant, revocation, ticket-digest,
administration-idempotency, Delegation-revocation, and
`identity-audit-outbox-v1` buckets. There is no supported earlier identity
schema, bearer-state importer, `p_` identifier migrator, bucket alias, or
dual-reader mode. A database missing any required version-1 bucket, containing
an unknown schema marker, or containing a malformed outbox/grant record fails
startup closed.

An administration or enrollment mutation and its audit-outbox record commit in
one bbolt transaction. Callback error, panic, cancellation, or process loss
before commit leaves neither change. Process loss after commit leaves both; the
next daemon start validates and drains the outbox. Diagnostics persistence
failure keeps the record pending and the idempotent command reports
`Unavailable`. Operators must not edit or delete the bucket to force startup.

Rollback from the first release means restoring the complete stopped-Node
consistency group into an empty directory and running the matching released
binary. A pre-release binary without the audit-outbox invariant is not a
rollback target even if it happens to open a development database.

### Content catalogue version 3

The current `ardents.db` content snapshot has top-level `version: 3` and a
required `blob_ownership` section with `version: 1`. Object and Manifest map
keys encode the canonical `(Owner Principal, local ID)` tuple. Object and
Manifest owner fields contain the same canonical typed `p1_` Principal.
Nested Manifest references resolve only within the containing Manifest's
Owner. Each Blob binding contains one canonical typed `p1_` owner, one content
reference, and its creation time.
Payloads and Blob metadata remain global and content-addressed; the owner is not
encoded into the CID and identical bytes are not copied per Principal. Unknown
versions, missing/malformed/untyped owners, duplicate `(owner, reference)`
pairs, and bindings to missing Blob metadata fail startup closed.

Startup supports one upgrade edge, content schema v2 to v3. Preflight requires
every legacy Object/Manifest map key to equal its embedded local ID, every
embedded Owner to be valid, and every nested Manifest reference to resolve
under the same Owner. The complete snapshot is validated before one atomic v3
catalogue write. Payload cleanup and staging reconciliation are deliberately
deferred until the next ordinary v3 startup, so a failed migration does not
delete files required by the rollback source. Malformed keys, cross-owner
references, and owner-qualified collisions fail closed without replacing the
v2 snapshot.

There is no importer for a version-1 snapshot, duplicate Blob `id`/`cid`
fields, or state that lacks `blob_ownership`. Such state is rejected rather
than assigned an inferred owner or content reference.

Application Put writes, hashes, and fsyncs a private temporary payload, installs
the content-addressed file atomically, and then commits Blob metadata plus the
owner binding in one bbolt update. A failed catalogue update rolls back the
binding and metadata and removes a payload only when that payload did not exist
before the operation. On restart, an installed `.blob` file with no catalogue
metadata is reclaimable and is removed; recovery never creates an ownership
binding from file or CID knowledge.

Remote Application Get is attempted only when its Effective Principal already
has a binding, and the binding is rechecked after fetch. Successful payload
verification alone never creates ownership. Removing one owner binding keeps
the payload while another binding, Object/Manifest reference, pin, durable
retention, relay retention, or staging fact remains; catalogue failure restores
the binding and any staged payload removal.

Replication state upgrades from schema v2 to v3 in the same stopped-Node
upgrade. Legacy root Manifest IDs acquire an Owner only when the migrated
content catalogue has exactly one matching Manifest. Missing or ambiguous
matches abort startup. Runtime intent, reconciliation, transfer, and repair APIs
never use this inference path and always require `(Owner, Manifest ID)`.

An older binary cannot read content or replication schema v3 and is not a
supported in-place rollback target. Before upgrading, stop the Node and retain a
verified full consistency-group backup plus the exact old immutable image. To
roll back, stop the new binary, preserve the failed state for diagnosis,
restore the complete pre-upgrade group into an empty state directory, verify
its manifest, and only then start the old image. Never lower a version marker,
delete `blob_ownership`, or restore only `ardents.db`.

### Node identity state version 1

The first-release `node-runtime/state` identity payload contains only
`principal` and `public_key`. It never contains a Node `device`: the Node root
key establishes the Node Principal, while Device IDs belong only to independent
root-authorized Credentials used for normal authentication. The Operator
`IdentitySnapshot` likewise exposes only state, Principal, and public key.

Strict loading rejects a pre-release `identity.device` member, missing or
duplicate schema markers, unknown fields, and unsupported versions. There is no
in-place importer for fake same-seed Device state. Before the first release,
discard that pre-release state or restore a complete canonical stopped-Node
backup; never copy only the identity record or synthesize a Device from the Node
key. Rollback to a pre-PIA-015A binary requires restoring its entire matching
consistency group rather than editing the version-1 record.

### Discovery retained state version 2

The PIA-015B `ardents.db` value at bucket/key `discovery/records` introduced a
strict JSON snapshot with `schema_version: 1`. The supported first-release
snapshot is `schema_version: 2`; each entry additionally requires local
verification evidence bound to the record's canonical bytes, signature, signer
Principal, and full trusted-Principal registry generation. Each signed record
remains `version: 1` and has exactly one NodeFacts or ServiceFacts
body. NodeFacts identify one Node Principal. ServiceFacts contain one Service
ID, service type, owning Node Principal, Workload ID, mode, public key, and
endpoints. Entry `source` and `seen_at` are local observations and are not part
of the signed record.

Startup rejects missing/unknown/duplicate schema fields, trailing JSON,
malformed unions, invalid signatures or Principal/public-key bindings, invalid
or missing verification evidence, invalid entry metadata, and duplicate record
IDs or kind/subject pairs before changing
runtime state. A valid record that expired while the Node was stopped is loaded
for retained-state continuity but remains non-routable. Import and restore save
failures roll in-memory discovery state back atomically.

Startup never treats persisted evidence as authority: it reverifies every
retained signature once. If the configured trust generation changed, startup
re-evaluates the exact `discovery.publish` purpose and atomically rewrites the
refreshed non-secret evidence before publishing runtime state. A schema-version
1 snapshot is rejected; there is no dual reader or in-place compatibility path.

This is the only supported first-release shape. There is no flat-record
importer, field precedence rule, or dual-format alias. Discard pre-release state
or restore a complete canonical stopped-Node backup. Rollback to a binary with
the flat record format requires restoring its complete matching consistency
group; never edit the bucket, remove the schema marker, or copy only
`discovery/records`.

## Rollback And Recovery

Recreate one Node at a time with the previous immutable image and re-prove
readiness. Restore data only when release notes say the newer release changed a
persisted format incompatibly or state verification failed.

To restore, stop and drain the Node, preserve the failed state and redacted
logs, verify the backup manifest, and restore the entire matching consistency
group into an empty target. Do not restore only `identity-access.db` or only the
configuration. Start the retained binary only after all files, hashes,
permissions, and schema versions verify.

Sessions and challenges are memory-only and intentionally disappear on restart;
clients authenticate again. Durable Principal enrollment, Access Grants, and
revocations must survive a normal restart and a verified same-version restore.
The recovery procedure never recreates a reusable bearer credential.

If automatic rollback or restore fails, keep the Node stopped, preserve the
complete state directory, and follow the incident runbook. Do not report the
Node healthy merely because another peer remains available.

## Release Acceptance

Before accepting an identity-affecting release, prove:

- a clean install produces only canonical `p1_`/`d1_` state and no permanent
  Operator/Application token;
- first Operator and first Application enrollment work and tickets are one-use,
  short-lived, and redacted;
- Operator/Application sessions reject cross-Node and cross-surface use;
- obsolete identifiers, credential schemes, configuration inputs, duplicate
  fields, and unknown versions fail closed;
- restart invalidates sessions but retains grants/revocations;
- transaction failure is atomic and stopped-Node backup/restore is verified;
- the repository-supported generation, unit, integration, e2e, and
  `go test ./...` checks pass or have a precise documented external exception.
