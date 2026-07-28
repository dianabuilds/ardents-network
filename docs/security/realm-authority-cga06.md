# Realm Authority CGA-06 restore, transition and migration contract

CGA-06 adds a narrow same-Realm restore proof, a dual-signed authority
transition and a stopped `ardents.local-realm/v2` importer. It does
not permit repository repair, lost-key recovery under the old Realm ID,
in-place downgrade, mixed old/new management, deployment qualification or a
change to `realm.channel-grant-authority` qualification.

## Same-Realm restore

The authority archive and monotonic checkpoint repository remain independent
failure domains. The stopped native archive contains the encrypted authority
ledger, its store key, signer material and configuration. It never contains
the repository. Production repository admission still requires the exact
preprovisioned WORM assertion, and configuration rejects a repository path
inside the Node, authority store, store-key or signer path.

A restore targets empty, stopped directories and starts once with
`authority.recovery_only=true`. Recovery-only startup is read-only. It
validates:

- the strict ledger schema, audit chain, retained membership/fencing truth and
  signed local checkpoint;
- the retained signer Principal and public-key binding;
- the complete independently retained repository predecessor chain and its
  unique head;
- exact Realm ID, authority sequence and checkpoint digest equality.

It never calls store `Save`, repository create/append, audit drain or key
generation. Missing, stale, rolled-back, forked, ambiguous, corrupt or
unavailable repository truth fails closed. An old signed checkpoint is
authenticated history, not freshness evidence.

While recovery-only is active, create, issue, acknowledge, rotate, renew,
membership, fence and activation mutations return recovery-required. The
protected `realm.channel.recovery.execute` operation requires direct Operator
authority on the exact Realm and an exact sequence/digest acknowledgement.
Successful verification changes only process-local admission state. The
operator then stops the daemon, removes `recovery_only` from the reviewed
configuration and restarts normally. A restart that leaves the flag enabled
intentionally requires verification again.

Signer loss, repository loss or freshness ambiguity leaves the old Realm
stopped. A replacement uses a new empty store, new independently provisioned
repository and new signer, and therefore creates a different random Realm ID.
No code path recreates an old head, signer, sequence, membership or audit
history from the archive.

## Planned signer transition

`PlanAuthorityTransition` commits a versioned transition body bound to the
exact current Realm ID, old and new authority Principals/public keys, adjacent
epochs, current authority sequence, current checkpoint digest and canonical
creation second. The old and new signers both sign the same domain-separated
digest. Any field or either signature change invalidates the artifact.

The operation first revalidates the live ledger, retained old signer and exact
repository head. It is denied by Product Policy and fails closed when either
signer is unavailable, the proposed signer equals the old signer or repository
truth is not exact. It atomically advances the ledger and repository checkpoint
to the successor Principal and adjacent epoch; the successor signs that
checkpoint, which embeds both signatures and the exact predecessor binding.
The authority then remains degraded and admits only one fresh rotation for
each channel present at transition. Readiness becomes ready only after every
required channel completes. Members call the production
`ChannelDeliveryService.AdoptAuthorityTransition` boundary, which verifies
both signatures, requires the predecessor to be their current trusted channel
issuer, and durably adds the successor in the encrypted capability store before
accepting successor-signed deliveries. Once authority truth records every
required channel complete, `FinalizeAuthorityTransition` durably retires the
predecessor's channel-issuance purpose. Both states survive restart.

The successor signer is provisioned before transition as protected
`authority.successor_signer_file`; daemon composition supplies it through the
single `WithSuccessorSigner` continuity seam during restart. If repository
storage is temporarily
unavailable after the successor checkpoint is committed locally, the ledger
stays in checkpointing state; it is neither rewritten nor marked corrupt.
Retry with the same exact request or restart with the preprovisioned successor
rolls the same checkpoint forward idempotently. A conflicting or corrupt
repository CAS still enters recovery-required. Lost-key recovery cannot
manufacture the required old signature and must create a new Realm.

## Local-v2 migration

The importer is a stopped maintenance boundary. The production adapter holds
the old manager's shared OS state-directory lock through commit, observes that
the shared authority path is absent, reads the retained protected
authority/Node files, decrypts every Node capability store with its protected
key, and hashes those actual inputs into the fence evidence. Legacy
`OpenOrCreate` holds that same lock for its whole management lifetime,
so a running or restarted old manager cannot recreate authority state while
migration evidence is active. Its strict JSON decoders
accept only `ardents.local-realm/v2` authority state and
`ardents.local-realm-node/v2` member state and reject unknown fields, trailing
values and every other version. Before writing an empty new store it requires:

- the configured signer to match the legacy issuer exactly;
- exactly one Node record for every authority member and no extra member;
- exact Node/authority record equality;
- exactly the discovery and data receiver grants from each protected Node
  capability store;
- exact channel ID, secret, generation, grant ID, validity, permissions,
  scope, issuer, subject and signature equality for every grant;
- a stopped old process, removed shared-authority control path and a retained
  SHA-256 fencing evidence digest;
- no head for the newly generated Realm ID in the independent repository.

The imported ledger retains only the legacy discovery/data channel material
and issuer continuity. Its genesis audit commits a domain-separated digest of
the full migration input and old-manager fence; the signed genesis checkpoint
commits that audit hash. Imported member state is migration evidence, not an
HPKE installed receipt.

The authority remains
`degraded/authority_migration_rotation_required`. All ordinary mutations are
fenced. Only fresh discovery/data rotations and their existing
installed/activation/fencing continuations are admitted. Each transition must
append its exact signed checkpoint through repository CAS. Readiness becomes
ready only after both channel rotations complete. The migration record retains
both completed Channel IDs and the old-manager/commit evidence digests.

Downgrade never edits the new ledger or reattaches the old manager. It stops
the new control plane and restores the complete verified pre-migration
authority and Node backup with the exact old software. Mixed management is
unsupported.

## Evidence and qualification boundary

Restore evidence retains the immutable archive and manifest digests, exact
pre/post status JSON, verify response, repository predecessor/head export,
software commit and test result. Migration evidence additionally retains the
strict source manifests, every member reconciliation result, the commit
evidence digest, both rotation operation/checkpoint results and proof that the
shared old-manager path is absent. Evidence must contain no private key,
channel secret, envelope, receipt key or member capability payload.

The runnable `tests/ci/cga06-recovery-migration-gate.ps1` drill retains
commit-bound JSON test streams and a SHA-256 manifest for restore, migration
and downgrade gates. Unit, adapter and protected-interface evidence proves the
fail-closed paths and exact contracts. Real-host backup, WORM administration,
multi-host fencing and release qualification remain CGA-07 work.
`realm.channel-grant-authority` therefore remains `Q=no`.
