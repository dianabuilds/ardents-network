---
id: R-041
title: What exact canonical Service Name limits and schema_version freeze the parser, encoder, and Service Link in S6.0?
status: decided
owner: Product Owner
started: 2026-08-19
reviewed: 2026-08-20
---

# R-041 — Canonical Service Name limits and schema_version

## Decision this unlocks

Freeze the numeric profile (label length, total length, depth), canonical
encoding, and `schema_version` that the S6.1 encoding/lifecycle slice will
consume. Without this freeze, S6.1 implementation would either embed an
unresearched default or duplicate DNS heritage that R-039 explicitly
rejects. The accepted profile replaces the open `R-041` row in
`docs/research/questions.md` and the unchecked §B.1 item in
`stage-6-readiness-checklist.md`.

## Current contract

R-039 § Fixed product contract already fixes the rule:

- canonical V1 Service Names are lowercase ASCII dot hierarchies with the
  parent on the right;
- `ardents://<Service Name>` is the explicit shareable form, not DNS;
- Unicode, IDNA, Punycode, public-TLD lookup, and DNS fallback are absent;
- each accepted claim creates a new Name Generation, and revisions are
  monotonic within a generation.

R-003 (Service Name contract) fixes the relationship between a Service Name
and its bound Service Target. R-024 § security fixes that no administrator,
registrar, legal or trademark claimant, or manual panel may seize, block,
transfer, or reassign a canonical Name Lease. CONTEXT.md defines `Service
Name` and `Service Link`.

What remains open before S6.1 can start is the numeric profile and the wire
encoding (length-prefixed form and the frozen `schema_version`).

## Hypotheses

- **H1:** the rule (lowercase ASCII, dot hierarchy, parent on the right,
  no Unicode/IDNA/Punycode) is already decided at the product level; the
  S6.0 freeze needs only the numeric values, the canonical encoder, and
  `schema_version`.
- **H2:** a DNS-derived profile (RFC 1035) is acceptable because the
  charset and depth are identical, even if the heritage is rejected.
- **H0:** an existing namespace (DNS, Onion v3, ENS) must be reused to
  avoid inventing a new identifier.

## Evaluation criteria

1. **Charset:** lowercase ASCII letters, digits, hyphen. No case folding,
   no IDNA processing, no Punycode decoding.
2. **Label rule:** non-empty, length 1–63, regex
   `^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`, plus an explicit rejection of
   `--`. No leading or trailing hyphen. No all-numeric top label (numeric root
   disambiguation, no other reason).
3. **Total rule:** canonical representation ≤ 253 ASCII characters.
4. **Depth:** ≤ 127 labels.
5. **Canonical name wire encoding:** `schema_version` as one `uint16`
   big-endian value, followed by repeated `uint8` label length plus lowercase
   ASCII label bytes, from leaf to root, until the bounded name field ends.
6. **Service Link:** `ardents://<canonical textual Service Name>`. The link
   parser applies the same textual validation as the ordinary Service Name
   parser; binary wire bytes never appear in a Service Link.
7. **Schema version:** `schema_version = 1`, `uint16` big-endian in wire
   encoding. Any future change to the encoding requires a new
   `schema_version` and a new record.
8. **Rejection:** the parser rejects without allocation or state mutation
   every non-canonical form (Unicode, IDNA, Punycode, uppercase, leading or
   trailing hyphen, consecutive hyphens or dots, leading/trailing dot,
   all-numeric top label,
   empty label, length overflow).
9. **Falsification:** a label that is valid by these rules but the parser
   silently accepts in another form; a parent that outlives its child
   generation; a `ardents://<name>` that decodes to a name different from
   the canonical form; two distinct inputs that yield the same canonical
   output.

## Evidence plan

### Primary sources

Primary sources, accessed 2026-08-19:

- R-003 — Service Name contract (accepted).
- R-024 — operational product closure, § security and § public-launch
  gates (decided).
- R-039 — H3 private naming lifecycle, § Fixed product contract
  (accepted 2026-08-17).
- CONTEXT.md — glossary: `Service Name`, `Service Link`, `Namespace`.
- Stage 6 brief, plan, readiness checklist, evidence contract.
- IETF RFC 1035 § 2.3.4 (size limits) — referenced for comparison, not
  adopted.
- IETF RFC 5890 (IDNA) — referenced only to confirm the rejection
  contract.
- IETF RFC 9458 (OHTTP) — referenced indirectly via R-026 for the
  existing fixed-pad encoder experience.

### Experiment

No comparative experiment is required for this bounded syntax choice. S6.1
must provide behavior vectors that round-trip the exact textual and wire forms
and reject every non-canonical form without state mutation.

### Failure scenarios

- Uppercase, mixed-case, or full-width characters are silently accepted via
  case folding or width normalization.
- An IDNA-compatible form (`xn--…`) is silently accepted and translated.
- An all-numeric root is accepted. A mixed root such as `3d` remains valid.
- An empty label or a leading/trailing dot is accepted and produces a
  shorter or different canonical form.
- A `schema_version` change is silently introduced in code without
  incrementing the version and a new record.
- The canonical encoder disagrees with the textual form: two inputs that
  differ only in delimiter placement hash to the same identifier only
  when the canonical form is identical.

## Findings

- **Sourced fact:** R-003 fixes lowercase ASCII dot hierarchy, the parent on
  the right, and textual `ardents://<Service Name>` links.
- **Sourced fact:** RFC 1035 supplies a useful size comparison but does not
  make Ardents names DNS names or authorize DNS parsing or fallback.
- **Assumption:** 253 textual bytes and 127 labels are sufficient for the V1
  Developer journey; implementation evidence must expose usability problems.
- **Inference:** a separate versioned binary form prevents link syntax from
  being confused with protocol framing.

## Options

1. **Option A — DNS-derived (RFC 1035).** Label 1–63 octets, total ≤ 253
   octets, depth ≤ 127, binary labels. Familiar to engineers and 1:1 with
   RFC 1035. Rejected: binary label length overshoots ASCII; the heritage
   is exactly what R-039 § Fixed product contract rejects.
2. **Option B — ASCII strict (recommended).** Label 1–63 ASCII, total
   ≤ 253 ASCII, depth ≤ 127, charset `[a-z0-9-]`, delimiter `.`, parent
   on the right, no leading/trailing/empty label, no leading/trailing or
   consecutive hyphen, no all-numeric top label, wire encoding
   length-prefixed, `schema_version = 1`. Pros: matches R-039 exactly,
   easy to validate, no DNS heritage, no IDNA interaction, easily
   documented for developers. Cons: visually similar to DNS.
3. **Option C — Tighter (≤ 127 total).** Same rules as B with total ≤ 127
   and depth ≤ 63. Rejected: shorter than the accepted Onion v3 spec and
   no measured reason to tighten.

## Recommendation

Choose **Option B**, accepted by the Product Owner on 2026-08-19. Confidence is
medium: the form is deterministic and bounded, but the usability of the maximum
depth is not measured. The strongest counterargument is that smaller bounds
would simplify implementations; no current product requirement justifies that
additional restriction.

## Disposition

- R-041 becomes `decided`. The open row in `docs/research/questions.md` is
  updated to point at this record and the frozen values.
- §B.1 of `stage-6-readiness-checklist.md` is checked.
- S6.1 (encoding/lifecycle) may consume the frozen profile. Any future
  change requires a new `schema_version` and a new research record.
- This freeze does not authorize code; the Stage 6 coding gate remains
  closed until §B.2 through §B.6 of the readiness checklist are also
  checked and the corrected brief, plan, and evidence contract are
  accepted.
- No ADR is required: this is a configuration freeze, not a technology
  selection that creates lock-in.
