---
id: R-103
title: Canonical Target Link v1 grammar
status: decided
owner: Product Owner and Codex
started: 2026-08-24
reviewed: 2026-08-24
---

# R-103 — What exact textual Target Link permits a User to share one network-bound opaque Service Target without origin, reachability, or naming ambiguity?

## Decision this unlocks

Define the first public, copyable H4-3 destination grammar without selecting
reachability, browser integration, a Service Name, or a new identity system.

## Current contract

Target Links are the complete alpha destination path while private naming is
deferred. They must be tagged, versioned, network-bound, and distinct from
Service Links; they carry neither an ordinary origin nor mutable reachability.
The Endpoint must reject a link for another network before connection work.

## Hypotheses

- **H1:** a fixed-width binary payload under a fixed textual prefix is concise,
  unambiguous, and can reject truncation, another network, and unknown target
  algorithms before any lookup.
- **H2:** a URI with independently parsed fields improves readability without
  increasing ambiguity or future parser divergence.
- **H0:** neither option can avoid making a reachability locator appear to be
  stable Service identity.

## Evaluation criteria

- one exact Target and network binding are preserved during copy/paste;
- text is disjoint from Service Links and ordinary URLs;
- malformed, padded, truncated, zero, foreign-network, and unknown-algorithm
  forms fail before resolution;
- no DNS, origin, path, capability, Local Grant, Service Name, or mutable
  reachability is encoded; and
- standard-library implementation is bounded and has one canonical spelling.

## Evidence plan

### Primary sources

- Existing Ardents product contracts, glossary, and H4-3 journey, accessed
  2026-08-24.

### Experiment

Use deterministic vectors to prove canonical encode/decode, rejection of
foreign syntax and padding, exact-length validation, zero-value rejection, and
Endpoint network binding before a connection is attempted.

### Failure scenarios

- a pasted ordinary URL or Service Link reaches a resolver;
- a bit-flipped, padded, or truncated value is accepted;
- a link intended for another network initiates local network work;
- an unrecognized future target algorithm is silently treated as v1; or
- a caller mistakes a Target Link for remote reachability or browser authority.

## Findings

- **Product decision:** the Product Owner selected
  `ardents-target:v1:<base64url(algorithm-id | network-id[32] | target[32])>`
  on 2026-08-24.
- **Inference:** an unpadded base64url encoding of exactly 65 bytes has one
  compact spelling and accepts no separator, host, query, or path that could be
  interpreted as ordinary web routing. Target algorithm id `0x01` is the only
  v1 identifier. All-zero network and Target values are invalid.
- **Measurement:** maintained deterministic tests exercise round-trip vector,
  foreign prefix, padding, malformed length, unknown algorithm, whitespace,
  zero values, and mismatched Endpoint network. They do not prove remote
  resolution, TLS identity, Route operation, or browser usability.

## Options

1. **Tagged fixed-width payload — selected.** `ardents-target:v1:` plus
   unpadded base64url of one byte algorithm id, 32-byte network id, and
   32-byte Target. It has no other fields.
2. **Structured URI fields.** Rejected: would introduce another grammar for
   escaping, order, and duplicate-field handling without adding an alpha user
   outcome.
3. **Reuse an ordinary URL or DNS name.** Rejected: it confuses Service
   identity with origin/reachability and violates the H4-3 boundary.

## Recommendation

Adopt option 1 as a closed v1 grammar. Decode strictly, re-encode to verify
canonicality, and compare its network identifier with the active Endpoint
before resolution. Keep target algorithm migration as an explicit future wire
decision.

**Confidence:** high for syntax and binding; low for the unfinished live
Target-to-browser experience, which this grammar deliberately does not supply.

## Disposition

Decided. ADR-0032 records the hard-to-reverse public grammar. The maintained
`service/targetlink` codec and Endpoint binding tests are retained; browser
presentation and Service Connection remain separate H4-3 work.
