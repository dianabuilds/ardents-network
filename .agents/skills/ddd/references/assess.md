# Bounded brownfield assessment

Use the accepted model in CONTEXT.md, the domain map and the affected technical
owner. Inspect actual code and callers before judging it.

| Question | Evidence to inspect |
|---|---|
| Is this a domain decision or a mechanism? | Invariants, refusal and who authorizes the effect |
| Where is consistency enforced? | State owner, durable commit, concurrency and original generation |
| Is the boundary real? | Non-test caller, exact imports and independent reasons to change |
| Can completion be trusted? | Denial, interruption, joined physical users and retained failures |
| Is repetition harmful? | Same rule/mechanism, divergence history and correction cost |
| Does integration preserve authority? | Real producer/consumer, fresh checks and no runtime bridge |

Choose the lightest sufficient modeling. Core rules merit an explicit model;
supporting coordination merits clear lifetimes. Codecs and native observers need
no repository, event or aggregate wrapper merely to look like DDD.

Report findings by consequence and evidence, not a total score. Distinguish a
confirmed defect from a design risk or unmeasured optimization. Give source,
owner contract, bounded correction and a causal positive/refusal check. Source
inspection proves neither race freedom nor installed/security qualification.

Do not require a new domain-context file. A useful diagram goes with its existing
owner and shows authority, effects, borrows and completion rather than every field.
