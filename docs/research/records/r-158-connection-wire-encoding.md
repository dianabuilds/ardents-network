---
id: R-158
title: Which bounded Service Connection frame encoding and incompatible profile replaces v2?
status: open
owner: Product Owner and design assistant
started: 2026-09-23
reviewed: 2026-09-23
---

# R-158 — Bounded Connection encoding and incompatible profile

## Decision this unlocks

Issue [#214](https://github.com/dianabuilds/ardents-network/issues/214) needs one accepted wire encoding, kind registry, parser limits and old/new profile refusal before #215 and dependent Connection work. This is a question about bytes and compatibility, not semantic authority, handoff or codec implementation. The [network-core wire proposal](../../development/network-core-wire-proposal.md) is input, not an accepted contract.

## Current contract

The [C0 product scope](../../product/scope.md), [threat model](../../security/threat-model.md), [Endpoint/Service owner](../../technical/endpoint-service-runtime.md) and [ADR-0081](../../adr/0081-select-closed-protected-service-contract.md) remain authoritative. The current owner explicitly retains `ardents-interactive-route-v2` Service Connection records in the selected successor, including the exact Instance and Continuity checks before Application effects. The native [record codec](../../../internal/service/connection/record.go) writes `ardents-service-connection-v2\x00`, a two-byte big-endian body length, version 2, a fixed profile string and one of six closed kinds. The current [Service TLS adapter](../../../internal/endpoint/text_service_tls.go) does not set an end-to-end Connection ALPN.

The candidate instead proposes exact end-to-end ALPN `ardents-connection/1`, a four-byte big-endian length and core-deterministic CBOR fixed arrays with new kind IDs. It proposes body length 1..16,448, control and pre-data-ready body limits of 1,024, DATA payload 1..16,384, arity at most 16 and nesting at most four. Those values and the clean-dev cutover are proposals. No public protocol is selected.

## Hypotheses and falsification

- **H1, candidate can be accepted as one incompatible profile:** every proposed message has one finite, canonical encoded form; length and depth are checked before allocation/effects; the selected old and new peers fail closed without fallback; no current persisted identity or floor is silently reused. A counterexample in any schema or an actual caller requiring mixed v2/new records under one identity falsifies H1.
- **H2, current v2 record grammar must remain for this bounded successor:** the existing owner and its real callers cannot change the record profile without a separately accepted migration or a change of product scope. A complete exact-caller and retained-state audit that permits clean separation falsifies H2.
- **H0:** neither direction preserves the accepted authority, privacy, workload and compatibility obligations; #214 must return a narrower proposal before implementation.

## Evaluation criteria

All message kinds need an exact registry, arity, field type and byte range; prefix/body limits must bound parser allocation before effects. Reject non-minimal integers, indefinite forms, maps/tags/floats, unknown or duplicate kinds, wrong arity, trailing bytes, partial/oversized frames and incompatible ALPN/profile without downgrade. Syntax alone grants no Instance, Target, Application or Attachment authority. Preserve the current Target/Instance authentication and retained roots/floors until an explicit accepted transition names their disposition. The chosen profile must carry the selected bounded text workload through both Route Carriers; a frame-size claim is not throughput or qualification evidence.

## Evidence plan

### Primary sources

Read on 2026-09-23 at dev@7e381928dd4bd906a1d623ca78a2dedf3738829c: the current product/security/technical owners above; `internal/service/connection/{contract,record,initial_authentication,continuity_exchange}.go`; `internal/endpoint/text_service_tls.go`; and the candidate proposal. Check [RFC 8949 §4.2.1](https://www.rfc-editor.org/rfc/rfc8949.html#section-4.2.1) for deterministic CBOR and the accepted ADRs linked by current owners, not external proposal files as contracts.

### Experiment

First produce a schema-by-schema encoded-size and canonicality table without implementation. Verify one maximum-size encoder representation and negative vectors for every kind against the candidate's stated limits. Trace current wire identities through actual callers and persisted roots before choosing clean-dev cutover. A disposable codec probe may test a falsifiable ambiguity, but a passing probe cannot accept the protocol or replace actual caller tests.

### Failure scenarios

Old peer/new peer mismatch, missing or different ALPN, hostile length prefix, short read, extra CBOR item, non-minimal integer, wrong bstr length, unknown kind, control over 1,024 bytes, DATA beyond 16,384 bytes, a syntactically valid but unauthorized phase, partial write followed by another frame, and retained old history presented under a new profile.

## Initial findings

**Sourced fact:** `docs/technical/endpoint-service-runtime.md` says the successor retains Service Connection v2 record bytes. `record.go` and `contract.go` implement that closed grammar, while `text_service_tls.go` has no new ALPN. The proposed CBOR/ALPN grammar would therefore replace maintained bytes and needs an explicit owner/ADR decision; publication of #212 only made the draft available.

**Sourced fact:** The candidate's §2 states bounded length, CBOR forms, arity and nesting; §8 gives predicted maxima, including 16,399 bytes for DATA body, 518 for OFFER and 340 for PROPOSE. Those are candidate calculations and have not been independently verified against every schema or current caller.

**Sourced caller fact:** The current `Profile` constant is also encoded into `internal/service/connection/context.go:Context` under a retained v1 ConnectionContext domain, and `ValidateRecovery` requires that exact value. Endpoint sets it in `text_service_binding.go`; `text_service_stream.go` uses `NewAuthenticatedStream`, while `service_connection.go` has direct v2 Challenge/Proof read/write callers. A blanket string replacement would therefore change the immutable logical digest and recovery admission, beyond #214's parser decision. The wire profile identity and any new context/binding identity must be assigned to their respective accepted owners; #215 handles binding semantics, and an exact caller cutover must reject mixed records before Application effects.

**Open evidence:** Exact v2 record and TLS caller inventory, any retained persisted identity/floor requiring compatibility, independent CBOR shape/maximum proof, and the old/new peer refusal matrix. No implementation or qualification result is inferred.

## Options

1. Select the candidate's one incompatible Connection profile after the exact table, old/new refusal, retained-state inventory and consequential ADR/owner change are accepted. Advantage: the proposed messages share one bounded grammar. Risk: the current v2 owner and callers require coordinated replacement; parser bounds alone cannot authorize new phases.
2. Retain v2 records for the selected successor and defer a different Connection profile. Advantage: preserves the current maintained owner. Risk: leaves the candidate's new opening/continuation schemas unavailable and may block dependent #215/#216 work; no hybrid parser is implied.
3. Change the product or transition contract if a named retained identity prevents clean cutover. This requires a separately bounded migration decision, not permissive fallback.

## Recommendation

No option accepted yet. Compare the candidate against every schema and exact caller, then ask the Product Owner to choose an explicit superseding profile/ADR or retain v2. Confidence is high that the two grammars are incompatible and low in the candidate's complete size/compatibility proof. The strongest objection to option 1 is a hidden retained identity or caller that cannot undergo the assumed clean-dev cutover.

## Disposition

Open. R-158/#214 is the sole selected C0 research question while R-157/#78 awaits the Product Owner's retained-maxima choice. No runtime code, wire profile, ADR or authority rule changes through this record.