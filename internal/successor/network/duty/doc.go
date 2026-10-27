// Package duty owns bounded Endpoint-local Role Domain duty and exposure truth.
// Network supplies producer-scoped replacement, conflict, expiry and bound
// decisions; this adapter owns the exclusive root, journal and commit lifecycle.
// Its one current root format is version 2: ADR-0107 retired the historical
// receiving one-use Transit Grant spend ledger whose write operation died with
// the Route v2 execution closure (ADR-0093). A version-1 generation is still
// strictly decoded and converted in place - conflict duties, generation
// continuity, and watermark rollback protection are preserved while the spend
// records are dropped - but no writer emits version 1, and the first
// replacement commits version 2.
// Linux SourceInspection and NodeInspection independently retain existing
// read-only histories under their original leases. Source permits only retained
// exposure records; stopped Node inspection permits no producer record at all.
// Neither policy clears, repairs or initializes history, and native readers join
// before their original lease returns.
package duty
