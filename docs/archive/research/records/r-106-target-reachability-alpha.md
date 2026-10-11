---
id: R-106
title: Target Link reachability acquisition for closed alpha
status: decided
owner: Product Owner and Codex
started: 2026-08-25
reviewed: 2026-08-26
---

# R-106 — How can a User who has only an exact Target Link obtain one current authenticated Service Publication and its C-2/Entry facts for the closed alpha without a direct Publisher origin, a hidden directory, a stale replay path, or a shadow Namespace authority?

## Decision this unlocks

Select the production acquisition boundary for H4-3A's Target Link open flow,
or narrow the alpha claim before any user-facing `open` command is built.

## Current contract

- A Target Link is exactly a network-bound opaque Target. It contains neither
  origin, mutable reachability, Node identity, nor a naming fallback
  ([CONTEXT](../../../CONTEXT.md)).
- Private Reachability Resolution is required for Target Links as well as
  name-derived Targets and has no direct public fallback. The maintained
  Descriptor v1, Gateway floor/conflict state, Entry-to-Initiator private
  carrier, and C-2 tracer now exercise that selected path; no participant
  `open` command or normal Endpoint acquisition profile exists yet.
- `publication.Decode` verifies a bounded Credential and publication against
  the target's Service Authority. `OpenUserReferenceSite` further compares the
  Target Link against that publication before emitting a browser origin.
- A User C-2 attempt additionally needs a State-selected Introduction,
  Initiator, and Rendezvous, a bounded Entry acquisition, and one opaque
  Introduction authorization. The maintained C-2 APIs deliberately require
  these typed inputs; they do not look up or choose them.
- `entry.Import` owns State-issued Entry Invite validation/replay state. A
  Publisher cannot manufacture a User's Entry Invite merely by signing a
  Publication.
- H4-4A name resolution remains deferred. A project-signed catalog cannot be
  passed off as current Namespace or current reachability authority.

## Hypotheses

- **H1:** One explicit, bounded, signed alpha invitation can carry an exact
  Target Link plus independently verifiable Publication and Entry facts while
  declaring that it is a short-lived invitation, not current private
  reachability resolution.
- **H2:** A minimal private reachability role can provide a current descriptor
  without joining H4-4 naming or exposing a direct Publisher origin.
- **H0:** Neither candidate retains target binding, freshness, and role
  separation with one-person alpha operations; H4-3 must defer its open flow.

## Evaluation criteria

- The User can start from an exact Target Link and receives a current signed
  Publication or an explicit unavailable/stale/conflicting outcome.
- No carrier Node receives a direct Publisher origin, Service private key,
  complete Route, or authority to choose a Target or a fallback.
- A stale, replayed, equivocated, withheld, substituted, or foreign-network
  artifact cannot silently create a Service Connection.
- A Publisher cannot mint an Entry Invite or an authorization for a transit
  Node; State/Entry retain their existing authority.
- The chosen form is bounded, inspectable, and usable by the actual one-person
  alpha team without becoming a hidden global directory or Namespace.

## Evidence plan

### Primary sources

- Current glossary definitions for Target Link, Private Reachability
  Resolution, Service Descriptor, and Destination Resolution Role, inspected
  2026-08-25.
- `internal/service/publication`, `internal/entry`, `internal/endpoint`, and
  ADR-0032 through ADR-0035, inspected 2026-08-25.

### Experiment

For each candidate, construct separate User, Publisher, and Node processes.
Record accepted origin-free inputs, exact Target/Network binding, expiry and
floor behavior, missing/current/stale/conflicting cases, and whether User can
complete one C-2 Reference Site request. The experiment must reject a changed
Publication, old valid Publication, wrong Target Link, substituted Entry
Invite, and direct Publisher endpoint before an HTTP request is possible.

### Failure scenarios

- A replayed invitation presents an old but still time-valid Publication.
- A project catalog or relay equivocates between two Publication generations.
- A User imports an Entry Invite for a different State generation or Node.
- The reachability service withholds, observes, or substitutes one Target
  request; a direct Publisher fallback is attempted.
- A static alpha invitation is accidentally described as DNS, Namespace, or a
  general private resolver.

## Findings

- **Sourced fact:** a Target Link deliberately cannot supply mutable
  reachability or a Publisher origin.
- **Sourced fact:** current C-2 composition consumes caller-supplied,
  State-selected transit facts and a signed Publication; its success in the
  six-process test is not an acquisition proof.
- **Sourced fact:** a Publication Credential authenticates its Authority,
  Target, Network, capabilities, generation, and validity interval, but
  `publication.Decode` has no User-held per-Target publication floor or
  authenticated source of a newer generation. It therefore accepts an older
  still-time-valid signed generation.
- **Sourced fact:** an Entry Invite is validated against a current State view
  at the Initiator. It cannot be made current merely by placing its bytes next
  to a Publication in a Publisher-signed artifact.
- **Inference:** a JSON plan that simply embeds the required bytes would make
  the product flow appear complete while leaving their producer, freshness,
  and authority unexplained. It is not an acceptable production adapter.
- **Inference:** option 1 cannot satisfy the stated `Target Link -> site`
  journey by itself: if a User needs a separately distributed invitation to
  learn the current facts, the invitation is the acquisition authority. It is
  potentially a useful closed-alpha test fixture, but must be explicitly
  described as such and cannot be presented as Target Link reachability.
- **Measurement (2026-08-26):** the maintained separate-process C-2 tracer
  starts distinct Publisher, User, Destination Resolution Gateway,
  Introduction, Initiator, Rendezvous, and Responder duties. It takes the exact
  Target only through the User's private Entry-to-Initiator OHTTP lookup, then
  verifies the returned Descriptor before it spends separate C-2 Entry work.
  The static, unavailable-Publisher, untrusted-Application, and dynamic HTTP/1.1
  scenarios pass; the User never receives a direct Publisher origin or a plan
  that embeds the Descriptor. This is bounded tracer evidence, not a normal
  participant open flow, multi-host operation, public deployment, or a
  freshness proof for a distribution source.
- **Implementation audit (2026-08-26):** `AlphaBrowserResolution` proves
  `name.ard -> accepted alpha floor -> exact C-2 Service` in a maintained
  Endpoint integration test and Windows Firefox qualification. Its only
  callers are those tests: no `ardents endpoint` command yet supplies the
  alpha floor plus typed State/Entry/Private-Reachability/C-2 opener. A command
  that copied e2e JSON facts would be rejected option 1 in a different form,
  not a participant acquisition path. The runtime must be built from selected
  option-2 inputs before H4-4 can call the browser route participant-operable.
- **Runtime-boundary audit (2026-08-26):** an imported `entry` owner can
  already revalidate and open a current Initiator candidate. A returned
  Descriptor already binds Introduction and Rendezvous identities, and the
  existing State `ResolutionView.Candidate` can verify those identities. The
  missing signed fact is narrower but consequential: State has no
  purpose-selected Destination Resolution Gateway record containing the
  Gateway identity/family and its signed OHTTP `GatewayProfile`. Choosing the
  first Rendezvous-domain candidate or carrying that profile in an operator
  plan would create an unselected discovery/authority rule. A participant
  runtime needs that State projection before it can invoke the existing typed
  private-reachability and C-2 APIs.
- **Implementation measurement (2026-08-26):** the accepted State projection
  is now Epoch v2: it binds one `destination-resolution` candidate to opaque
  signed GatewayProfile bytes. State rejects a missing, oversized, mismatched,
  or ambiguous projection and exposes no candidate ordering or alternate
  profile. `OpenAlphaBrowserRuntime` then proves in one Endpoint integration
  test that a browser `reference.ard` request obtains its binding solely from
  the retained alpha floor, its Gateway/C-2 facts solely from State and Entry,
  performs OHTTP through the selected Initiator, and reaches the Publisher's
  ordinary HTTP application. The test is a composition proof, not a
  participant credential-acquisition or installed-process proof.

## Options

1. **Explicit short-lived alpha invitation.** Carry an exact Target Link,
   signed Publication, and independently verifiable State/Entry references
   through a new bounded format. It may be a closed-alpha bootstrap only and
   must have explicit expiry/floor/replay behavior. It cannot be called the
   Target Link itself or Namespace resolution.
2. **Minimal Private Reachability Resolution.** Add a target-descriptor
   publication/lookup role with its own authenticated freshness and privacy
   contract. This fits the glossary but is a material protocol and operations
   slice.
3. **Project catalog lookup.** Reject as a sole authority: R-098's catalog is
   an inspectable index, not Release, State, Namespace, or reachability
   authority.
4. **Direct Publisher endpoint.** Reject: it bypasses C-2 and reveals a
   prohibited origin-like ingress.

## Recommendation

Choose option 2: implement a minimal Private Reachability Resolution role for
the accepted H4-3 journey. The Product Owner accepted this choice on
2026-08-25. Its currentness basis is deliberately narrow: Target-derived
Authority binding, Authority-issued non-overlapping Credential lifetimes,
Gateway-held durable per-Target floor/conflict state, and short-lived
Instance-signed live slot evidence. It is not Namespace proof or a catalog.

**Confidence:** high that a direct plan or catalog would violate the existing
contract, and that a signed invitation alone cannot establish currentness for a
Target Link. The strongest argument against the chosen role is its operational
and protocol scope before one alpha user can open one site; the counterweight is
that it is the smallest selected path which preserves the stated H4-3 journey.

## Disposition

Decided. ADR-0036 and `docs/technical/private-reachability.md` own the selected
boundary. Descriptor v1, Gateway-local durable generation/conflict state, the
private carrier, and its Endpoint-to-C-2 Reference Site composition are
implemented. The separate-process tracer now adopts the Descriptor and carrier;
run `go test ./tests/e2e/service -run '^TestReferenceC2' -count=1` for its
bounded behavior evidence. It does not make the path participant-ready: a
normal Endpoint still lacks a first-run/acquisition profile and a user-facing
`open` command. R-106 no longer blocks the selected tracer; it continues to
govern participant composition, publication-withdrawal/failure qualification,
and any future production claim. The Product Owner accepted the missing State
projection on 2026-08-26; ADR-0046 owns its source-of-truth boundary and the
projection/runtime composition is implemented. The remaining participant
blocker is not another resolver or route plan: the maintained command has no
enrolled owner for the current State view, Entry set, and one-use Transit TLS
credentials. That ownership/distribution boundary must be selected before a
normal Endpoint command may expose Browser Entry readiness.
