---
status: accepted
date: 2026-09-14
---

# Consolidate the protected network and retire predecessor runtimes

The Product Owner accepts the architecture and atomic change decomposition
assessed in [R-155](../research/records/r-155-network-core-consolidation.md).
The maintained branch contains both predecessor runtime compositions and the
selected protected successor. Hiding both behind a new facade would preserve
duplicated maintenance and an unwanted accepting path.

## Decision

After acceptance of the complete installed successor and its explicit local
adoption under ADR-0081, subsequent adopted releases retain one protected
runtime. Retire the accepting Endpoint v1 plan and AAI2 Connection path,
legacy alpha-corpus intake/destination adapters, predecessor Node duties and
Transit Grant issuance, the Source native-profile selector, and the former
networked Name resolve/control command adapters. No failed successor operation
may select one of these paths. This is a conditional retirement decision;
accepting this ADR does not assert that migration or removal has happened.

The dedicated Rendezvous Contributor retains only authenticated diagnosis and
retirement control for its own installed deployment. New apply/restart is
removed. Diagnose/drain/withdraw/remove must not start or restart predecessor
work through interrupted-update recovery. Preserve the deployment checks,
root lease, necessary floors and explicit removal confirmation; introduce
neither arbitrary service control nor an automatic replacement Contributor.

Preserve authority, conflict, resource and adoption floors. Old roots are not
cleared or relabelled to admit a new runtime. Retain the minimum verification
and refusal machinery needed for migration and authenticated history, but no
old dial/listen/forward engine. Adoption remains voluntary; a new release does
not remotely stop an unadopted installation or force an update.

Keep the shared opaque ordered Application stream, AAI3, the existing
half-close pipe and native Service Connection authentication/recovery/terminal
mechanisms. In particular, a retained Service wire identity containing `v2`
is not evidence that it is exclusive to the retired Route generation.
Preserve Application Interface v1 Administration, canonical Namespace domain
verification and the immutable Browser compatibility evidence required by
ADR-0069. Name remains refused at the protected Application boundary until
its separately selected real producer and protected composition exist.

Application owns its content protocol. The selected text codec and trusted
presentation remain intact; the common stream receives trusted workload
bounds and opaque bytes. Reuse the selected confined qualification Application
to verify binary payloads. Select no universal content codec, generic worker,
DSL, datagram semantics, new Carrier, package or dependency.

Consolidate mutable Endpoint state into closed owners inside the current
package: issuance, Reader prefixes, Publisher prefixes, publication with
registration, and jobs. One publication owner commits the current pair and
its finite previous overlap. Context owns authorization and joined shutdown.
Route owns complete forwarding admission and Carrier-session lifecycles;
Node retains current State/duty, process policy and stop decisions. These
changes preserve wire, authority, budgets and recovery behavior.

## Consequences and verification

First preserve useful predecessor stream behavior in successor tests; then
retire accepting entrypoints before deleting their exclusive implementation.
Fix unsafe spend-journal continuation/recovery independently of the later
refactor, without opening a second active implementation slice.

Every atomic change includes its real caller, behavior/adversarial tests and
current-owner documentation, plus affected package, ownership, profile and
deadcode inventories. Retained historical evidence does not become an active
test or qualify changed artifacts. Final qualification remains tied to the
exact candidate and every applicable P1–P11 obligation on both Carriers.

The [common architecture owner](../technical/common-privacy-architecture.md#accepted-consolidation)
owns the selected composition and the [qualification owner](../development/privacy-qualification.md#consolidation-verification)
owns its verification boundary. GitHub Issues own execution and dependencies;
this decision does not start implementation, complete #50, waive a gate, or
change R-149/public naming, public operation or anonymity claims.
