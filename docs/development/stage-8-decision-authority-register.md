# Stage 8 decision-authority register

Status: **S8.1 decision route, accepted with the preservation ledger.** This
temporary register turns every S8.1 `Decide first` item that can constrain
S8.3 into a named question, authority path, and stop condition. It does not
select an algorithm, format, package, or dependency. The Product Owner's
2026-08-22 `continue` disposition remains the product-scope authority.

For a consequential or hard-to-reverse outcome, the route is: a question in
`docs/research/questions.md` (or a newly added one), a source-bound research
record with alternatives and falsification criteria, then a Product Owner
acceptance and a superseding ADR where the outcome creates technology, protocol,
format, platform, or security lock-in. No Code/format mutation is the evidence
that makes such a decision.

| ID | Question that must be decided | Current source conflict/constraint | Required authority before S8.3 design freezes it | Stop condition |
|---|---|---|---|---|
| DA-01 | Does verified release-root preservation occur before or only after executable metadata is accepted? | F003 contradicts R-049's stated transaction meaning; D06/D07 recovery and floor format depend on it. | Focused R-049 reconciliation research record; Product Owner acceptance; ADR-0015 supersession only if lifecycle/format semantics change. | Do not mutate Release/Update state, recovery, or codec. |
| DA-02 | What is the single Network State missing-current recovery rule? | F014 contradicted the accepted R-027/R-029 recovery interpretation; D01/W01 repairs had no safe meaning until reconciled. | **Closed:** accepted [R-059](../research/records/r-059-network-state-missing-current-recovery.md) selects fail-closed recovery-required without automatic pointer reconstruction. ADR is unnecessary because it confirms R-027/R-029. | M3 may alter the recovery oracle, but not the pointer format or unrelated representation without DA-05. |
| DA-03 | What signed-time/freshness boundary governs naming policy and recovery? | F027 has impossible gateway-millisecond equality and a second time-only recovery advance. | Naming transcript/freshness research with skew, delay, replay, restart, and cancellation falsification; Product Owner acceptance; ADR if signed transcript changes. | Do not change Name/Recovery transcript, state, or wire bytes. |
| DA-04 | What Namespace scale and update/query resource envelope is actually supported? | F032's arbitrary 4,096-record cap and whole-corpus lookup have no accepted capacity claim. | New research question/record with declared cardinality, latency, memory, concurrency, proof-size, and restart measurements; Product Owner acceptance. | Do not increase the cap or choose an index/cache as a refactor. |
| DA-05 | Which Network/Namespace persistence and commitment representation is shared, if any? | F020/F021 expose cross-domain Store import and `network/epoch/merkle` placement without a selected shared foundation. | **Closed:** accepted [R-060](../research/records/r-060-domain-owned-persistence-and-commitments.md) selects domain-owned representations and accepted [R-061](../research/records/r-061-domain-ownership-transfer-order.md) selects the Namespace-first prerequisite transfer. No ADR is required because no shared foundation, engine, or format is selected. | Before M3 deletion, remove Namespace's Network imports through the R-061 prerequisite; do not introduce a generic shared package. |
| DA-06 | Which H3 Route protocol/topology and transport representation survives? | W01/W02/W04 preserve information-flow semantics but not current Route bytes, topology, or WebTunnel selection. | Route/transport research tied to P06/S01/S03, mixed-version/downgrade behavior, and qualification impact; Product Owner acceptance; ADR for retained protocol/transport choice. | Do not retain, replace, or publish a protocol/transport by package migration alone. |
| DA-07 | Which naming wire/profile choices remain authoritative? | W03 must retain P05/S01/S02 while ADR-0013 is withdrawn and current implementations do not prove all global constraints. | Relevant accepted R-041--R-047/R-057 review; a new record for any unresolved algorithm/crypto/profile choice; Product Owner/ADR route as applicable. | Do not change signed records, proof semantics, OHTTP profile, or recovery representation without its authority. |
| DA-08 | What qualified Application isolation and platform operating profile, if any, is in scope? | S04/L05 and F041--F044/F062 preserve a claim condition while no Broker/Isolation exists and Windows activation is unsupported. | Product Owner horizon/supported-profile decision, platform threat/Adapter design, and platform evidence; ADR for a selected native confinement model. | Do not claim isolation, Windows activation, install, or supported host behavior. |
| DA-09 | What real release/update/custody product lifecycle, if any, is in scope beyond H3 technical inputs? | D07/D08 and F010/F012 lack real activation and Custody ownership; H3 does not ship them. | Product Owner scope promotion, lifecycle/custody design, and required research/ADR-0015/0021 compatibility analysis. | Do not turn tracer command results into installer, update, or Custody product behavior. |
| DA-10 | Which external compatibility observers require bounded support? | A01--A06/L02 have no established public window, but possible Application/operator/evidence consumers cannot be guessed away. | S8.3 caller inventory and Product Owner approval of each support/break/export rule. | Do not break a discovered external caller or retain an unbounded legacy reader. |
| DA-11 | Which lab/evidence artifacts retain a named reproduction or Qualification duty? | Q01/F064--F067 show six commands and historical suites, but history alone is neither runtime scope nor deletion authority. | S8.2 evidence-profile manifest plus Product Owner acceptance of claim/source/expiry ownership. | Do not remove a linked reproducer or make it a product-runtime dependency. |

## S8.3 admission rule

A proposed S8.3 target row may proceed only when it either has no matching
`DA-*` row or names the accepted decision identity that closes that row. A
target architecture must keep an unresolved row visible as a stop condition;
it cannot select a convenient current implementation as a default. This rule
applies equally to package additions, imports, formats, adapters, commands,
tests, and documentation that would otherwise encode the undecided outcome.
