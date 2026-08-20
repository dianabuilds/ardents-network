# Stage 7 decision proposal — Separate release decisions from versioned local activation

Status: **accepted on 2026-08-20 and recorded in
[ADR-0015](../adr/0015-separate-release-decision-from-local-activation.md).**

## Context

ADR-0006 separates release safety from protocol transition, but does not choose
the local installation architecture. Stage 7 must accept identical authenticated
artifact bytes from untrusted distribution, preserve Authority and monotonic
state, survive interruption at every transition, and roll back only to an
authenticated compatible non-revoked build on both Ubuntu and Windows.

Letting an OS package manager, store, mirror, or one self-updating executable own
both trust and mutation would mix authorities and produce different behavior on
each platform. Mutating the active payload or durable schemas in place would
also make crash-safe rollback depend on application-specific repair code.

## Proposed decision

Adopt the R-048 O1 seam if the Product Owner accepts this proposal and R-049/R-050
validate the candidates:

1. A **Release Decision Module** verifies trusted release metadata, artifact
   identity, platform/environment binding, build-safety/protocol policy, and
   non-decreasing release watermarks. Distribution supplies bytes only.
2. An **Update Transaction Module** owns the Installed state machine
   `verify -> stage -> reserve rollback -> stop new work -> drain -> activate ->
   self-test -> commit | rollback | repair-required`. Portable stopped
   replacement reuses the same Release Decision, compatibility, self-test,
   copy-on-write migration, and rollback-floor rules without this activation
   Adapter.
3. An **Install Lifecycle Module** owns thin Ubuntu and Windows Installed
   Adapters for the stable bootstrap, package-owned paths, optional OS
   registration, repair, uninstall, and explicit purge. It has no
   release-signing or Authority Custody power. Portable needs no lifecycle
   Adapter.
4. Installed executable payloads are immutable versioned directories. A small
   system-owned bootstrap reads one bounded authenticated activation record;
   activation atomically replaces that record rather than rewriting a running
   payload. Portable is the same platform executable target plus only
   unavoidable declared companions and is updated by authenticated stopped
   replacement.
5. Mutable schema migration is copy-on-write until commit. Authority Vault,
   Recovery Bundle state, Network Epoch, Namespace, generation, freshness, and
   rollback watermarks are outside the payload tree and never roll back with it.
6. A failed forward start activates only a previously authenticated,
   schema-compatible, non-revoked retained payload. If neither direction is
   safe, networking stops and local repair/Authority export remains available.

The platform package signature is additional install-channel evidence. It is
not an Ardents release root and cannot authorize an executable omitted from
accepted release metadata.

## Consequences

- Two platform Installed Adapters make the lifecycle/activation seam real;
  common release and transaction behavior stays in deep Modules and is tested
  through their Interfaces. Portable adds no platform lifecycle Adapter.
- Installed and Portable have Client/Publisher feature, Application Interface,
  resource, state-compatibility, and claim parity. Installed adds managed
  package/registration/update conveniences; Portable is directly copied, run,
  stopped, replaced, or deleted without implicit system integration.
- Program portability does not imply Vault, Grant, root, watermark, network
  state, or Endpoint-identity portability. Profile coexistence is disjoint and
  any future state migration requires its own explicit contract.
- The Installed stable bootstrap becomes a small high-risk artifact. Its update and
  recovery path must be explicit, independently identified, and cannot silently
  self-replace through a payload transaction.
- Versioned payload and rollback reserve consume finite disk. Admission must
  reserve staging and one safe rollback before stopping work.
- OS-specific atomic replacement and crash durability are not assumed equivalent;
  R-050 must freeze exact filesystem/API conditions and fail unsupported volumes.
- No package path, Go package, dependency, serialization, or platform Adapter is
  selected by this proposal. Those require R-049/R-050 and package-map updates
  with real code.
- This is an H3 laboratory-release architecture. Project-controlled test keys
  and builds do not satisfy H4 independent-custodian or independent-builder gates.

## Compliance and acceptance

- [R-048](../research/records/r-048-h3-stage-7-contract.md) contains the scope,
  alternatives, sources, and recommendation.
- [Stage 7 lifecycle specification](../development/stage-7-lifecycle-spec.md)
  defines the normative transaction and state ownership.
- ADR-0006 remains authoritative for release/build/protocol semantics.
- The Product Owner accepted the proposal for Stage 7 development on
  2026-08-20. R-049 selected its bounded verifier profile; R-050 selected O1
  with Windows installation still requiring a separate explicit command and
  unavailable native durability evidence remaining deferred.
- R-049 and R-050 may refine the implementation or require this proposal to be
  rejected/replaced before S7.1; they may not weaken its authority separation or
  rollback invariants silently.
