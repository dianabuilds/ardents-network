---
status: accepted
date: 2026-09-09
---

# Bind issuer-bootstrap restriction to each authenticated Node child

## Context

The Product Owner approved the authenticated child restriction proposed during
implementation on 2026-09-09. [R-153](../research/records/r-153-bootstrap-child-restriction.md)
explains why ADR-0081's49-byte outer OPEN cannot preserve an Entry's issuer-only
bootstrap restriction through opaque inner TLS. An otherwise valid Interior
ADMIT does not prove that the Entry admitted private forwarding.

## Decision

Amend only the Node-outer child allocation and its admission propagation in
[the selected protocol](../technical/protected-route-protocol.md):

- Endpoint-role OPEN remains exactly 49 bytes. On an authenticated Node Carrier,
  OPEN is exactly 50 bytes: the same 49-byte recipient/duty/purpose/deadline fields,
  followed by restriction u8. Value 0 means no additional restriction, not a grant;
  value 1 means issuer-bootstrap only. Unknown/missing values are unavailable.
  The authenticated channel state selects the grammar; there is no optional
  field, negotiation or Endpoint-supplied switch.
- The forwarding owner derives restriction from its actual incoming admission
  reservation. A receiving Node binds it before inner TLS allocation. A
  restricted forwarding child refuses private ADMIT even when valid; it may
  continue only the selected issuer bootstrap route and propagates restriction.
  The current issuer can terminate that restricted child. Existing exact
  State/record/duty/family, budget, and independent receiver checks still apply.
- Restriction is immutable through child closure. Issuance success alone does
  not upgrade any hop. Private work requires actual admission at each required
  hop and fresh children under admitted parents; no relabelling old children.
- Keep mixed children in the same existing Node/profile/Carrier pool. This is
  a negative constraint, not a new identity, Endpoint label, authority, token
  field, protection mode, transport or dependency.

Explicitly retire the former 49-byte Node-outer allocation. A revised receiver
rejects it before child allocation; a former receiver rejects the 50-byte form.
Do not fall back, infer value 0 from omission, or treat old receipts as acceptance
of the amendment. Generation3 role framing and generation 2 compatibility
identities otherwise retain their meaning. Before adoption, quiesce any use of
the experimental former form and follow the existing atomic artifact/profile
migration with fresh signed profile/duty bindings, preserved floors and old
spend tombstones. An old executable cannot resume network work after adoption.
This is a declared incompatible pre-qualification contract correction, not a
claim that already deployed candidates interoperate.

## Consequences

Each Node child allocation costs one additional accounted byte and no round
trip. Required acceptance includes a valid private token inside a restricted
child, missing/unknown restriction, Endpoint injection, attempted relabelling,
mixed pooled children, both Carriers, cancellation, withdrawal and aggregate
budgets. The amendment selects implementation work; it does not qualify issuance,
forwarding, Application confinement or the complete closed product.
