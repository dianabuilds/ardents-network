---
id: R-159
title: Which signed EndpointSet format and Node profile preserve atomic endpoint authority?
status: open
owner: network-core
started: 2026-09-23
reviewed: 2026-09-23
---

# R-159 — Signed Node EndpointSet and profile incompatibility

## Decision this unlocks

Issue [#218](https://github.com/dianabuilds/ardents-network/issues/218) needs one bounded Node Record format in which a Node advertises atomic (Carrier, IP, port) tuples, and an explicit incompatibility rule for old and successor State consumers. The operator producer (#226), destination policy (#227), State implementation and public authority are separate decisions. This record is a proposal for review, not an accepted owner or ADR.

## Current contract

[Product scope](../../product/scope.md) and [threat model](../../security/threat-model.md) retain historical roots, keys and floors while requiring a complete committed Candidate View and deterministic rejection. [ADR-0048](../../adr/0048-maintain-tcp-and-quic-carriers.md) selected a single Carrier per signed schema-2 record; [ADR-0081](../../adr/0081-select-closed-protected-text-service.md) selected the protected successor's two Carrier-v2 values and exact generation-3 binding. The [Network/Node owner](../../technical/network-route-node.md) and [protected Route owner](../../technical/protected-route-protocol.md) currently select one authenticated Carrier per attempt, one v3 listener per literal endpoint, no race or fallback, and a schema-2 exact record digest in the ClosedProfile. The dev source at 7e381928 (2026-09-23) confirms that State accepts ARNR schema 1/2, signs exact bytes before the 64-byte Ed25519 signature, selects one endpoint and Carrier, then commits accepted/rejected View roots. Changing only the parser would change the View and its materializations.

The prepared [network transition map](../../development/network-core-transition.md) and external EndpointSet probe are candidate provenance only. [#212](https://github.com/dianabuilds/ardents-network/issues/212) integrated the map as a draft, without accepting its format.

## Hypotheses and falsification

- H1: one bounded, deterministically encoded EndpointSet inside a new signed ARNR schema binds each tuple to the same Node/key/generation/capacity, while an explicit successor State profile rejects old schema and old verifiers reject new schema.
- H2: reuse schema 2 with a textual or separately signed endpoint list is sufficient. A tuple whose Carrier, key or record digest can be combined with another signed record falsifies H2.
- H0: the set is insufficient until an exact authority/profile boundary and pre-packet destination owner are selected.

H1 is falsified if two conforming decoders disagree on the accepted tuple set, if a changed tuple retains a valid record signature, if old/new verifiers can commit different Views under one profile identity, or if one Node gains extra capacity from extra tuples.

## Evaluation criteria and evidence plan

Require exact field order, types and bounds; one encoding per set; signature over the complete record; no repair/sorting before verification; deterministic rejected-View classification; no old-profile fallback; at most eight tuples and 139 set bytes before decode. Count a Node once in capacity and collision accounting. Preserve Network, Node, key, generation, validity, family, capability and capacity semantics. Protect the authorization of network contacts against a malicious record supplier and a mismatched State consumer. A signature proves authorship of bytes, not State membership, endpoint safety, listener readiness, operator independence or privacy.

Primary sources reviewed 2026-09-23: [RFC 8949 §4.2.1](https://www.rfc-editor.org/rfc/rfc8949.html#section-4.2.1) for shortest deterministic CBOR encodings; the accepted ADRs and owners above; internal/network/state/epoch_record.go, epoch_candidate_view.go, closed_profile_accept.go and epoch_profile.go; internal/route/closed_node_carrier.go. The external 2026-09-21 proposal and 42-case disposable field probe informed the candidate. That probe did not test Node signatures, State roots or runtime use. The deterministic signed record vectors in [R-159 vectors](r-159-endpointset-vectors.md) are independently generated from Go's standard Ed25519 implementation and are syntax/decision fixtures, not successful State acceptance.

Failure cases: malformed/overlong/duplicate/out-of-order tuples, non-shortest CBOR, unknown schema/version/Carrier/family, mutated tuple with unchanged signature, validly signed wrong Network, reused Node/key or endpoint across records, mismatched ClosedProfile digest, and old/new profile mismatch. A signed local/private address still needs #227's pre-packet policy. A State successor must stop/join its old duty and retain historical floors; this format alone gives no currentness or live transfer guarantee.

## Findings

**Sourced fact:** Current ARNR schema 1/2 has one endpoint text field; schema 2 signs one explicit Carrier string. State rejects malformed records before signature checks, then checks Network, signature, time, capability, capacity and Carrier, and detects Node/key/endpoint collisions before View commitment. Its closed Epoch profile is exactly ardents-route-v3; closed Route also uses that ALPN. The accepted protected owner specifies schema 2 for generation-3 duty.

**Inference:** Accepting schema 3 under the unchanged ardents-route-v3 State profile permits old and new verifiers to compute different accepted/rejected roots for the same input log. Therefore a new profile identity and explicit old/new refusal must precede implementation; changing ARNR version alone is insufficient. A new identifier must be approved with the affected State and protected Route owners. This record does not silently redefine their v3 bytes, ALPN or binding.

**Measurement:** The prepared field probe reports 42 passing grammar cases, including the 12-byte minimum and 139-byte maximum. Recalculation: outer [version, array] occupies 3 bytes; four maximum IPv4 tuples use 4×11 and four IPv6 tuples use 4×23, totaling 139. The proposed full ARNR maximum with a 32-byte family is 366 bytes, below the existing 32 KiB record framing ceiling. The attached deterministic Ed25519 vectors provide one canonical signed positive candidate, a tampered-byte negative, a validly signed noncanonical negative and a validly signed wrong-Network negative. They have not been run through a successor State verifier.

**Assumption:** Eight tuples, at most two per (Carrier, family), are enough for a current/successor address overlap. This is an operational bound, not a measured availability guarantee. More addresses need a new version, not silent truncation.

## Options

1. **New schema and incompatible successor profile (recommended for decision).** Retain ARNR's identity/authority fields and exact Ed25519 signature; replace endpoint text plus Carrier text with a length-delimited canonical EndpointSet. Select an explicit new State/Node profile and update the accepted owners/ADR before runtime. Old schema 1/2 remain historical inputs and are rejected for new duty; schema 3 is rejected by old verifiers. This avoids one profile name denoting two View algorithms. Cost: coordinated State/ClosedProfile/Route transition and no automatic conversion.
2. **Widen ardents-route-v3 in place.** Fewer profile names, but incompatible View results and changed meaning of the accepted v3 owner. Reject.
3. **Independent signed endpoint supplements or multiple Node records.** Adds authority joins and permits key/Carrier mixing or duplicate capacity without a new lifecycle contract. Reject for #218.

## Recommended candidate bytes and authority

Proposed complete record, all integers big-endian as in current ARNR:

    ARNR[4] | schema:u8=3 | Network[32] | Node[32] | generation:u64 |
    notBefore:i64 | notAfter:i64 | family:existing length-u8 text <=32 |
    capability:u8 | endpointSetLength:u16 | EndpointSet[that length] |
    capacity:u16 | NodePublic[32] | Ed25519Signature[64]

Sign the exact prefix from ARNR through NodePublic, including schema, length and tuple order. No second set signature. The State authority must still authenticate Network membership, key, time, role/profile, committed View and exact record digest; self-signature alone grants none of them. One Node/key/generation contributes one capacity, regardless of tuple count. New cross-Node endpoint collision checks compare canonical IP/port across each set, conservatively preserving current endpoint uniqueness even across TCP and QUIC; one Node may use the same IP/port for its own TCP and QUIC tuples.

EndpointSet v1 is one core-deterministic CBOR item [1, [Endpoint...]]. Endpoint is exactly [carrierID:uint, family:uint, address:bstr, port:uint]. Carrier IDs 1 and 2 mean the exact successor tcp-tls-v2 and quic-v2 profiles only inside schema 3; family 4 requires four address bytes, family 6 sixteen; port is 1..65535. Require 1..8 tuples, at most two per (Carrier, family), ascending by (Carrier ID, family, address bytes, numeric port), with no duplicates. Require shortest definite arrays/strings/integers, exact arity and one consumed item; no maps, tags, text, floating values or aliases. Reject mapped IPv6, unspecified, multicast, IPv4 limited broadcast and link-local unicast at grammar validation. The set must be <=139 bytes before decode; full record retains the existing 32 KiB bound. Neither tuple order nor tuple count grants retry priority, more Route weight or more capacity.

The exact successor Epoch/Carrier profile identifier and interaction with the selected ardents-route-v3 ALPN/HELLO are consequential owner decisions. The suggested name ardents-route-v4 in the external proposal is **not selected here**. A safe acceptance matrix requires: old v3 consumer + schema 1/2 according to current contract; old v3 consumer + schema 3 rejects; new consumer + schema 3 may accept after all authority checks; new consumer + schema 1/2 rejects for new duty; unknown mapping/schema/version rejects before dial. Distinct Network/trust context for a clean dev successor is a deployment proposal, not permission to erase retained roots, keys or floors. No automatic conversion or active Attachment mutation follows.

## Recommendation and disposition

Recommend option 1 with conditional confidence: field grammar and signature boundary are precise, but the chosen profile identifier, ALPN/HELLO relationship and exact ClosedProfile acceptance rule need an accepted owner/ADR decision. Strongest counterargument is the coordination cost of a new profile for a small address grammar; widening v3 nevertheless changes what an authenticated Epoch means. Keep #218 open and this record as research until those owners are updated and signed vectors become exact State acceptance tests. #226 owns the ordinary record producer; #227 owns destination policy. The external probe stays disposable provenance; no runtime/dependency/ADR change is made by this record.
