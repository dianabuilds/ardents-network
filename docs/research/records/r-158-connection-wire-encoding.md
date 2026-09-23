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

**Sourced fact:** The candidate's §2 states bounded length, CBOR forms, arity and nesting; §8 gives predicted maxima. These are proposed limits, not an accepted wire contract.

**Arithmetic check (2026-09-23):** Recomputed the maximum CBOR body length from each candidate §5 fixed-array shape, using RFC 8949 §4.2.1 shortest definite-length heads, the declared maximum field lengths, and the *allowed* enum ranges; no maintained encoder or parser was run. The results match candidate §8: PROPOSE 340, C0 416, OFFER 518, CONFIRM/OPEN_READY 104, CONTINUE_REQUEST 175, CONTINUE_ACCEPT 72, SETTLED_RECEIPT 90, ASSIGN (two paths) 81, ASSIGNED 47, PROBE/PROBE_REPLY 37, ABORT/REFUSE 4, DATA 16,399, ACK 22, EOF 12 bytes. DATA leaves 49 body bytes under proposed L=16,448; OFFER is the largest non-DATA body at 518, below proposed control limit 1,024. This checks only the declared shapes and maxima, not emitted canonical bytes, negative decode behavior, semantic admission, framing allocation or actual callers.

**Sourced caller fact:** `internal/service/connection/record.go` owns the v2 prefix, two-byte length, version, fixed profile and six kinds. `initial_authentication.go`, `continuity_exchange.go` and the stream workers consume that codec. The protected Linux Endpoint's `text_service_stream.go` calls `NewAuthenticatedStream` for initial and recovered Attachments; `text_service_tls.go` sets no end-to-end ALPN. The separate `internal/endpoint/service_connection.go` path directly reads/writes v2 Challenge and Proof. An implementation split must account for both caller families; changing only a decoder or test caller would leave an accepting v2 path.

**Sourced identity fact:** `Profile` is encoded into `context.go:Context` under retained `ardents-service-connection-context-v1` and required exactly by `ValidateRecovery`; Endpoint sets it in `text_service_binding.go`. In the protected text path, the local v1 Context feeds the salted InitiatorBinding, which then enters `ProtectedContext`'s distinct v3 logical tuple. Therefore a blanket `Profile` replacement changes that tuple indirectly as well as recovery admission. The proposed Connection ALPN is a different identity from the existing authenticated Route Profile; reusing `Profile` for the new ALPN would silently change Route recovery checks. #214 must name the new wire identity separately, leaving the accepted Route Profile untouched unless its own owner is explicitly revised. The accepted [ADR-0075](../../adr/0075-service-connection-v2-terminal-receipt.md) explicitly selects the v2 envelope and terminal receipt markers; the current technical owner repeats the selection. A new grammar needs an explicit superseding decision. This does not assign new binding semantics: #215 owns those, while #214 must name the incompatible wire identity and caller cutover. No Connection-package disk reader/writer was found; retained assets and floors outside it still need an exact inventory before claiming a clean-dev cutover.

**Retained-state boundary from current owners:** The v1 `ConnectionContext` is derived per Connection, and the maintained Connection package has no disk reader/writer for a live Connection or its continuity key. This limited source observation does not license deleting state outside the package. The [C0 scope](../../product/scope.md) retains existing roots and floors as evidence, and the [Endpoint owner](../../technical/endpoint-service-runtime.md) expressly keeps the non-decreasing Publication generation floor and opened host Instance root; deleting either is not Service recovery. The [threat model](../../security/threat-model.md) requires persisted non-decreasing security watermarks and finite Work Safety. The candidate §10's clean deployment with new Network/trust context and roots is a proposal, not authorization to erase, migrate or reinterpret any existing bytes. Exact external asset ownership and any named migration need a separate decision before accepting a cutover claim; #214 can still select a distinct wire encoding without asserting old-state conversion.

**ALPN refusal derivation (Go 1.26.8 source, not an end-to-end test):** Current `secureTextClient` and `secureTextPublisher` configure no `NextProtos`. Go `crypto/tls.negotiateALPN` returns an empty protocol without error if either side has no ALPN list; `checkALPN` also permits an empty server choice for ordinary TLS. Thus a new peer merely advertising `ardents-connection/1` can finish TLS against an old peer. The candidate must require `ConnectionState().NegotiatedProtocol == "ardents-connection/1"` on *both* new client and server before any new Connection frame or Application effect, and close on empty/mismatch. With two nonempty, disjoint ALPN lists Go rejects the handshake; a v2 peer's empty list instead requires the new-side exact post-handshake guard. This is a proposed refusal rule to verify with real old/new Endpoint pairs, not a claim that current runtime enforces it. No fallback to v2 is implied.

**Proposed old/new refusal matrix (still untested end to end):**

| Client / Publisher | TLS observation | Required new-profile outcome |
| --- | --- | --- |
| New / new, both offering only `ardents-connection/1` | Exact ALPN selected | Parse only the proposed bounded CBOR grammar after the exact-profile guard; ordinary semantic admission is still separate. |
| New / current v2 | Current Publisher offers no ALPN, so ordinary Go TLS can complete with empty negotiated protocol. | New client closes before sending a Connection frame or Application bytes. |
| Current v2 / new | Current client offers no ALPN, so ordinary Go TLS can complete with empty negotiated protocol. | New Publisher closes before reading any v2 frame or causing Connection/Application effects. |
| New / peer with a different nonempty ALPN list | No common ALPN; Go TLS refuses the handshake. | No fallback list or retry with an empty profile. |
| Current v2 / current v2 | Empty ALPN and v2 records remain a pair of old-runtime peers. | This is not a new-profile connection or an authorization to preserve an alternate new-runtime parser. Deployment cutover must retire old peers explicitly. |

For a framing negative vector, the old v2 prefix begins with ASCII `arde` (`61 72 64 65`), which the proposed four-byte big-endian length parser would read as `0x61726465` (>16,448); it must reject before allocating the body. Conversely, a candidate frame starts with a bounded four-byte length (`00 00 ...`) and cannot match the old fixed prefix. These are wire deductions; actual mixed-peer tests and a new-side ALPN guard remain required. The [protected Route owner](../../technical/protected-route-protocol.md) also explicitly retains current Service Connection wire records and the salted local ConnectionContext input; it must be changed in the same accepted contract cutover, without discarding retained root/floor bytes.

**Open evidence:** Encoder/decoder verification of the computed bounds, canonicality and all registry vectors, mixed-peer Endpoint tests, and a complete external retained-asset/floor inventory. The source caller and TLS/ADR inventory above does not establish those tests or a migration permission. No implementation or qualification result is inferred.

## Candidate syntax table for decision

This table transcribes candidate §3/§5 shapes and independently checked maximum *body* lengths; it is proposed, not the current technical contract. `b32`/`b64` mean exact-length CBOR byte strings; `u64` means an unsigned CBOR integer with the shortest form. Every array is definite, has exact listed arity, and has no optional or trailing fields. The four-byte prefix is additional to these body lengths. C0 is nested inside OFFER and is not its own wire kind.

| Kind | Exact typed body array | Arity | Max body bytes |
| --- | --- | ---: | ---: |
| 1 DATA | `[1,direction:u64,offset:u64,payload:bstr(1..16384)]` | 4 | 16,399 |
| 2 ACK | `[2,direction:u64,acceptedOffset:u64,receiveLimit:u64,finalAccepted:bool]` | 5 | 22 |
| 3 EOF | `[3,direction:u64,finalOffset:u64]` | 3 | 12 |
| 16 PROPOSE | `[16,1,network:b32,target:b32,instanceKey:b32,instanceGeneration:u64,publicationDigest:b32,profileDigest:b32,initiatorBinding:b32,nonceA:b32,routeBinding:b32,limitsA:[u64,u64,u64],bounds:[u64,u64,u64]]` | 13 | 340 |
| 17 OFFER | `[17,C0,instanceSignature:b64,proofB:b32]` | 4 | 518 |
| nested C0 | `[1,network:b32,target:b32,instanceKey:b32,instanceGeneration:u64,publicationDigest:b32,profileDigest:b32,initiatorBinding:b32,nonceA:b32,nonceB:b32,handle:b32,limits:[u64,u64,u64,u64],bounds:[u64,u64,u64],proposeDigest:b32]` | 14 | 416 |
| 18 CONFIRM | `[18,handle:b32,H0:b32,proofA:b32]` | 4 | 104 |
| 19 OPEN_READY | `[19,handle:b32,H0:b32,proofB:b32]` | 4 | 104 |
| 32 CONTINUE_REQUEST | `[32,role:u64,handle:b32,H0:b32,intent:u64,attemptNonce:b32,routeBinding:b32,proof:b32]` | 8 | 175 |
| 33 CONTINUE_ACCEPT | `[33,role:u64,requestDigest:b32,proof:b32]` | 4 | 72 |
| 34 SETTLED_RECEIPT | `[34,role:u64,requestDigest:b32,finalA:u64,finalB:u64,proof:b32]` | 6 | 90 |
| 40 ASSIGN | `[40,revision:u64,paths:[1..2 × b32]]` | 3 | 81 |
| 41 ASSIGNED | `[41,revision:u64,setDigest:b32,retainedPresent:bool]` | 4 | 47 |
| 48 PROBE / 49 PROBE_REPLY | `[kind,nonce:b32]` | 2 | 37 each |
| 62 ABORT / 63 REFUSE | `[kind,code:u64]` | 2 | 4 each |

The listed maxima depend on enforcing those enum ranges, not merely decoding their CBOR major type as `u64`. Recomputing with unrestricted full-width u64 in enum positions gives DATA 16,407 instead of 16,399 bytes, ACK 30 instead of 22, CONTINUE_REQUEST 191 instead of 175, and ABORT 12 instead of 4. Those encodings remain below the outer 16,448-byte frame cap but must fail the per-kind value/range gate before effects. This is an independent arithmetic check of the table, not a passing runtime parser test.

Candidate field ranges additionally fix direction and role to 0/1, intent to 0/1, ABORT code to 0..3 and REFUSE code to 0..2; generation/revision are positive and do not wrap. ASSIGN carries one or two distinct b32 paths in lexicographic order. Time values are positive Unix seconds through 2^63-1 and still require the separate time/authority owner. A syntactically valid value does not authorize a phase or effect.

Candidate envelope `L=1..16448`; before data-ready, reject `L>1024` from the prefix before body allocation. All non-DATA kinds are limited to 1,024 body bytes even after data-ready. DATA alone may use the larger envelope, with its own 16,384-byte payload bound. A decoder must reject non-minimal integers/lengths, indefinite items, maps, tags, floats, negative integers, null/undefined, unknown kinds, wrong type/arity, partial frames and trailing bytes before any semantic effect. The maximum 16,448-byte read allocation is a framing ceiling, not a per-peer work or throughput budget. Authorization and phase admission remain separate.

## Candidate framing vectors (schema only)

Each row is the entire proposed `uint32be(L) || CBOR body` in hex. These short examples exercise syntax only; they convey no valid Instance proof, phase, authority or workload acceptance. The expected result applies to a prospective strict new-profile parser after exact ALPN selection. No parser has been run against these vectors.

| Case | Exact framed bytes | Expected schema result |
| --- | --- | --- |
| DATA: direction A→B, offset 0, one byte `A` | `00 00 00 06 84 01 00 00 41 41` | Accept shape. |
| ACK: direction A→B, accepted/limit 1, final false | `00 00 00 06 85 02 00 01 01 f4` | Accept shape. |
| EOF: direction A→B, final offset 1 | `00 00 00 04 83 03 00 01` | Accept shape. |
| ABORT: code 2 | `00 00 00 04 82 18 3e 02` | Accept shape. |
| REFUSE: code 1 | `00 00 00 04 82 18 3f 01` | Accept shape. |
| Non-minimal kind 62 | `00 00 00 05 82 19 00 3e 02` | Reject non-canonical integer. |
| Indefinite array | `00 00 00 05 9f 18 3e 02 ff` | Reject indefinite form. |
| Unknown kind 64 | `00 00 00 04 82 18 40 00` | Reject unknown kind. |
| Trailing CBOR null | `00 00 00 05 82 18 3e 02 f6` | Reject trailing item. |
| ABORT wrong arity | `00 00 00 03 81 18 3e` | Reject wrong arity. |
| ACK final flag encoded as integer 0 | `00 00 00 06 85 02 00 01 01 00` | Reject wrong field type. |
| DATA empty payload | `00 00 00 05 84 01 00 00 40` | Reject payload length zero. |
| L=0 | `00 00 00 00` | Reject before body allocation. |
| L=16,449 | `00 00 40 41` | Reject before body allocation. |
| Truncated ABORT at EOF | `00 00 00 04 82 18 3e` then EOF | Reject incomplete body, no effects. |

The [companion registry vectors](r-158-connection-wire-vectors.md) now specify exact complete syntax-only positive frames for all 16 candidate message kinds, one exact trailing-item rejection derivative per kind, and a maximum-size DATA frame recipe. They do not prove parser, encoder, canonical-byte or authority behavior. RFC 8949 core deterministic restrictions and type/range rejection must be tested across the shapes, not inferred from these examples.

## Options

1. Select the candidate's one incompatible Connection profile after the exact table, old/new refusal, retained-state inventory and consequential ADR/owner change are accepted. Advantage: the proposed messages share one bounded grammar. Risk: the current v2 owner and callers require coordinated replacement; parser bounds alone cannot authorize new phases.
2. Retain v2 records for the selected successor and defer a different Connection profile. Advantage: preserves the current maintained owner. Risk: leaves the candidate's new opening/continuation schemas unavailable and may block dependent #215/#216 work; no hybrid parser is implied.
3. Change the product or transition contract if a named retained identity prevents clean cutover. This requires a separately bounded migration decision, not permissive fallback.

## Recommendation

**Recommend option 1 for Product Owner decision, not acceptance by this record:** reserve one separate closed C0 Connection wire identity `ardents-connection/1` with the candidate's four-byte length, RFC 8949 core-deterministic fixed-array registry and stated per-kind limits. Require an exact negotiated ALPN check at both new endpoints, old/new refusal without v2 parser or fallback, and a coordinated caller cutover. Keep the already authenticated Route Profile distinct; do not edit `connection.Profile` as a shortcut. Preserve all existing roots, publication/security floors and historical bytes as evidence; selecting wire bytes does not choose a fresh Network/trust-root deployment or migrate live work. Update the current Endpoint and protected Route owners and supersede ADR-0075's v2 wire selection only in an explicit accepted decision. New opening, binding, DATA/ACK, assignment and terminal effects remain with their separate cards; no runtime enablement follows #214 alone.

The source inventory and all 16 syntax vectors support a bounded *format proposal*. They do not prove canonical parser behavior, real mixed-peer refusal, safe rollout or qualification. Option 2 preserves the present v2 grammar but cannot carry the proposed opening and continuation kinds without another format decision, so it does not unblock the stated #214 direction. Confidence is high in the v2/new incompatibility and the Go ALPN gap, moderate in the calculated schema bounds, and low in rollout feasibility until external retained assets and real mixed-peer tests are checked. A named retained asset requiring in-place migration would falsify the assumed clean cutover and trigger a separate migration decision, not a permissive parser.

## Disposition

Open. R-158/#214 is the sole selected C0 research question while R-157/#78 awaits the Product Owner's retained-maxima choice. No runtime code, wire profile, ADR or authority rule changes through this record.