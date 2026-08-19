# S6.0 preparation summary

Status: **all six S6.0 freezes are decided.** Stage 6 coding gate remains
closed only because the corrected brief, plan, readiness checklist, and
evidence contract still require Product Owner acceptance, and the
Product Owner has not yet recorded the coding start decision. Once
those document-level acceptances land, the S6.1–S6.5 coding gate opens
and Stage 6 production work can begin.

This document is a preparation pointer, not a decision record. It maps
the six S6.0 freezes to research questions, identifies which freezes
needed an ADR, and lists the order in which the gate can be opened. It
does not select primitives, storage, ordering rules, or interfaces
beyond the decisions already recorded.

## Gate

Stage 6 entry gate from `horizon-3-stage-6-brief.md`:

| # | Condition | Status |
|---|---|---|
| 1 | R-033 through R-037 decided and Product Owner has recorded the maintained Stage 5 `advance` for S5.4 and S5.5 | **Satisfied 2026-08-19.** Final R-037 qualification remains S9.6 and is not a Stage 6 predecessor. |
| 2 | Corrected Stage 6 brief, plan, readiness checklist, and evidence contract accepted by Product Owner after review | **Not accepted.** Document-level acceptance pending. |
| 3 | R-003 remains authoritative without a contradictory interpretation | To be confirmed by Product Owner during document review. |
| 4 | S6.0 decision profile accepted (six freezes) | **All six decided 2026-08-19.** See table below. |
| 5 | Package and command ownership factual in `package-map.md`; any new verifier package or command is added only with its implementation, tests, non-test caller, `doc.go`, and exact permitted imports | Foundation packages (`internal/naming`, `internal/namelease`, `internal/nameauthority`, `internal/nameresolver`) are factual work in progress; a verifier package is not yet registered. |

Existing Stage 6 foundation packages are work in progress, not evidence
that the entry or completion gate has passed.

## Six S6.0 freezes — status

| ID | Freeze | Blocks slice(s) | ADR | Status | Decision |
|---|---|---|---|---|---|
| R-041 | Canonical name limits and `schema_version` | S6.1 encoding/lifecycle | — | decided | label 1–63 ASCII, total ≤ 253, depth ≤ 127, `[a-z0-9-]`, parent on the right, no leading/trailing/empty label, no leading/trailing or consecutive hyphen, no all-numeric top label, length-prefixed wire encoding, `schema_version = 1` |
| R-046 | Field-level role matrix | S6.2 role separation | — | decided | five roles with per-role concrete types (`endpoint-adjacent`, `naming-rendezvous`, `local-resolver`, `authority-operation`, `observer`), forbidden combined view `User/Endpoint location ∧ exact Service Name / lookup value`, stable identifier scope, Role Domain per ADR-0005, identity/known-family exclusion per ADR-0005 + R-005 + R-035, Isolation Context per CONTEXT.md, fail-closed on missing/invalid role proof per R-002 |
| R-044 | Cryptographic suite | S6.2, S6.4, S6.5 | **ADR-0013** | decided | Ed25519 (Name Authority, RFC 8032, Go `crypto/ed25519`), BLS12-381 (Recovery Policy threshold, `golang.org/x/crypto/bls12381`), OHTTP (resolver query hiding, R-026 supply `openpcc/ohttp v0.0.80` commit `79bec89d8042`), SHA-256 + HKDF-SHA-256, replaceable Go interface boundary (`Signer`, `ThresholdVerifier`, `QueryHider` in `internal/nameauthority/sig.go`), no local primitive |
| R-043 | Persistence, restart, rollback, cache-proof | S6.4 authority/delegation/recovery | — | decided | Go `Storage` interface in `internal/namelease/store.go`, default implementation `internal/network/store`; durable set (Name Lease, Generation, Record signed Ed25519, Recovery Policy signed BLS12-381, capability counter); restart-derived set (ephemeral handles, in-flight admission, query state, uncommitted buffers); cache-bounded set (resolved Target with `epoch_number` + `epoch_digest` freshness proof, activation countdown, order-key index); tamper rule `state-tampered`; replay-bound `state-stale`; atomic write semantics |
| R-042 | Claim ordering and Conflict/Fork classification | S6.5 concurrency/fork/abuse | — | decided | order key `(network_id, epoch_number, SHA-256(canonical claim payload))` reusing R-029 Network Epoch; proof structure (signed payload, authenticated epoch, parent proof, R-041 schema, R-046 role); five-state classification (`ordered`, `conflict`, `fork`, `unavailable`, `partition`) with explicit fail-closed and Lease-mutation rules; coverage map for the eight brief scenarios |
| R-045 | Anonymous Cost and local admission | S6.5 concurrency/fork/abuse | — | decided | Hashcash-style SHA-256 PoW (k leading zero bits) + per-endpoint per-epoch rate limit + scoped short-lived capability (TTL ≤ 60 s, scoped to `(endpoint_pubkey, surface, target_epoch)`); per-surface limits (claim 1/20/100, renewal 10/16/1000, resolution 100/8/10000, policy 1/18/10, recovery 0/22/1 per generation) calibrated to R-023 reference host; admission order `epoch → role → schema → counter → PoW`; fail-closed `admission-denied` per R-002 |

## ADR list

- **ADR-0013** — Stage 6 cryptographic suite (Ed25519 + BLS12-381 + OHTTP-reused + SHA-256 + HKDF-SHA-256 + replaceable Go interface boundary). The other five freezes were configuration / contract decisions and did not require an ADR.

## Next step

1. Product Owner reviews the corrected Stage 6 brief, plan, readiness
   checklist, and evidence contract for document-level acceptance.
2. Product Owner records the coding start decision after §A through §D
   of `stage-6-readiness-checklist.md` pass.
3. Stage 6 coding gate opens. S6.1 (encoding/lifecycle), S6.2 (role
   separation), S6.3 (Target continuity), S6.4 (authority/delegation/
   recovery), S6.5 (concurrency/fork/abuse), and S6.6 (verifier)
   proceed in order.

No implementation slice may treat an open S6.0 freeze as an implicit
default. All six freezes are now closed, so a Stage 6 slice that needs
a value the freeze did not provide must return to S6.0 rather than
embedding an unresearched choice in code. A future replacement of any
frozen value (limits, suite, role matrix, storage engine, cost
mechanism) requires a new research record and, where the replacement
creates technology lock-in, a new ADR.
