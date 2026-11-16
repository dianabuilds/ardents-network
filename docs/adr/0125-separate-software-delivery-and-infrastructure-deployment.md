---
status: accepted
date: 2026-11-16
---

# ADR-0125 — Separate software delivery and infrastructure deployment

## Context

The Product Owner directed removal of Installation as a standalone domain and
separation of user application installation from infrastructure deployment.
The existing `internal/successor/installation` tree combines both with Release
composition, native service management and role-specific runtime lifecycles.
That tree describes existing implementation, not a required product domain.

## Decision

1. Installation is not a standalone product domain. User application delivery,
   installation and update are platform-specific product delivery concerns.
   Infrastructure deployment is a separate operator concern. Neither owns
   Network, Route, Admission, Hosting, Service or Application behavior.
2. Enrollment retains independent initial-pin and inventory provenance; Release
   retains authenticated exact-target decisions and monotonic trust floors.
   Runtime domains retain their own admission, process lifetime, stop and joined
   completion; their data and durable histories remain with their existing
   owners. Composition may order these owners without acquiring their rights.
3. A replacement implementation must retain exact authorized artifact and
   configuration binding, predecessor completion before incompatible transition,
   no mixed-generation acceptance, original failure and joined cleanup, and
   existing durable state and floors. Interrupted or uncertain transitions must
   refuse unsafe continuation. Existing user data is never deleted or reset as a
   consequence of retiring an implementation. Retained wire and persisted
   identities keep their existing compatibility obligations.
4. Linux, macOS and Windows are product directions. Platform mechanisms may
   provide packaging, process management and confinement only when their actual
   behavior satisfies the required protection and failure contract. This
   decision selects no package manager, weaker accepting path, new supported
   platform, or qualification result.
5. Do not build a general Ardents installer or a separate lifecycle manager.
   Application delivery uses the platform's delivery mechanism, with explicit
   authenticated replacement and user data retained. Infrastructure operators
   supply verified artifacts and role configuration; platform service management
   starts and stops processes where it meets the required lifecycle and
   confinement contract. Ardents validates its own configuration, completes its
   own runtime work and retains its durable state. This selects no specific
   platform mechanism and grants no automatic update or weaker startup path.
6. Retire role-specific installation, stop, restart, replacement and recovery
   orchestration, including journals that exist solely for those operations,
   after their required guarantees are provided by the replacement. Keep
   authenticated Release and Enrollment, necessary configuration validation,
   and durable runtime state with their actual owners. Existing installed roots
   and their bytes are preserved: no automatic migration, deletion, reset or
   adoption of an uncertain process follows from removing the orchestration.
7. ADR-0123 decision 1 is amended only in its classification of Installation
   as a separate domain. ADR-0119, ADR-0123 and ADR-0124 retain their artifact,
   authority, lifecycle, state and refusal guarantees. Their existing Linux
   implementation and consumers remain until equivalent ownership and evidence
   are established. A source move, unit test or platform refusal is not a
   completed migration or installed acceptance.

## Consequences

Eliminating a directory or relabeling an owner alone does not realize this
boundary. Each transfer requires a real caller, exact dependency registration,
behavioral evidence and the original ownership/failure ordering. Historical
qualification receipts apply only to their original artifacts. The current
closed Ubuntu profile remains distinct from the broader product direction.
