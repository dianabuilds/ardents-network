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
4. Recreate that Node with the new immutable image.
5. Require protected local API readiness, canonical network readiness,
   Diagnostics, and retained Principal/grant state before continuing.
6. Repeat for remaining Nodes; do not upgrade all bootstrap Nodes together.
7. Retain backups and the previous image until the observation window closes.

`./ardents.ps1 upgrade` automates the supported single-host Compose sequence.
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

### Content catalogue version 1

The first-release `ardents.db` content snapshot has top-level `version: 1` and a
required `blob_ownership` section with `version: 1`. Object and Manifest owner
fields contain one canonical typed `p1_` Principal. Each Blob binding contains
one canonical typed `p1_` owner, one content reference, and its creation time.
Payloads and Blob metadata remain global and content-addressed; the owner is not
encoded into the CID and identical bytes are not copied per Principal. Unknown
versions, missing/malformed/untyped owners, duplicate `(owner, reference)`
pairs, and bindings to missing Blob metadata fail startup closed.

This is a greenfield first-release schema, so there is no importer for a
pre-release snapshot that lacks `blob_ownership`. Such state is rejected rather
than assigned an inferred owner. Create fresh state or restore a complete
same-version stopped-Node backup.

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

An older pre-PIA-014A binary does not understand this authority fact and is not
a supported in-place rollback target. Stop the Node and restore the complete
matching consistency-group backup before starting that binary. Never delete the
`blob_ownership` section or copy `ardents.db` independently from the rest of the
backup group to force rollback.

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
