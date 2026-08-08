---
status: accepted
date: 2026-08-08
---

# Authenticate shared epochs and separate Control Plane roots

This is a closed-test/Public Beta Control Plane boundary, not Carrier Lab scope.
Carrier Lab uses explicit project-owned synthetic state and makes no public
decentralization, freshness, completeness, or auditor-independence claim.

Ardents accepts one expiring threshold-authenticated Network Epoch committing a
logical complete Candidate View, its canonical length, global summaries, and a
precommitted append-only Node-publication input root/cutoff. Clients may fetch
deterministic Candidate Materializations with indexed inclusion proofs instead
of an unbounded monolith. A distributor cannot produce a valid personalized
subset or bias selection through resampling; at least two full auditors,
independent from each other, the epoch signer threshold, and audited Candidate
operator families, detect signer omission, invalid rejection, and false global
summaries and publish control-independence evidence. Exact convergent
log and proof machinery remain implementation research. Distribution never
grants state authority. Release, Network Epoch, Namespace,
qualification, and emergency powers use separate roles and keys, with expiry,
transparent transitions, rollback protection, and explicit forks. The public
baseline is `3-of-5` for epoch, offline release-root transitions, and every new
executable package digest, and `4-of-5` for narrow expiring emergency
incompatibility or revocation. Online snapshot/timestamp delegates cannot
introduce an executable.

Every mandatory pre-Route public artifact class has three beta/five stable
effective authenticated direct-source-only operator families under `40%`/`25%`
concentration caps; external/CDN delivery without authenticated family evidence
does not count. A source-only identity/family is incompatible with Route and
Destination Resolution assignment, while an ordinary candidate contacted
directly is quarantined locally by the Endpoint.

This explicit Control Plane is preferred to source-local reseed truth or one
project server because bootstrap cannot be trust-free and hidden authority is
harder to inspect or replace. A one-to-one project necessarily uses centralized
test keys, so its network remains visibly provisional and cannot inherit a
decentralization claim. Mirrors cannot change an epoch; threshold capture can,
and transparency makes that capture visible rather than impossible.
