---
status: accepted
date: 2026-09-22
supersedes: ADR-0040 (live Alpha Service Link and corpus-replacement selection); ADR-0041 (maintained corpus-floor acceptance consequence only); ADR-0042 (maintained accepting-command consequence only)
---

# Retire Alpha Service Links and fresh corpus intake

## Context

The bounded Alpha Name Corpus and Alpha Service Link supplied a non-canonical
named journey before the protected Target-Link text Service existed. The
maintained C0 journey now uses that protected Target-Link path. Keeping corpus
intake or a grace-period resolver would retain an otherwise uncalled legacy
network path and its authorities solely for old links.

Existing corpus floors also contain signed serial, digest, withdrawal,
rollback, and conflict evidence. Retiring the live path does not by itself
define a safe migration or deletion rule for those bytes.

## Decision

Retire fresh Alpha corpus intake, then retire every remaining Alpha Service
Link accepting path with an explicit refusal before any corpus-floor read,
resolver, Network, Route, dial, fallback, or conversion effect. Keep the
supplied-bytes corpus inspection route read-only. Do not provide a grace period
and do not convert an Alpha Link or old Target into a Target Link or Name.

Leave existing corpus floor bytes unchanged. Their parser and read-only reader
remain a compatibility obligation until a separate bounded decision both
proves that no maintained consumer remains and defines migration or data
retention. Retained bytes are evidence, not authority for continued Alpha
resolution.

## Consequences

- Human-facing Service Names remain a product function to be designed over the
  protected protocol; the legacy Alpha network path does not remain on their
  behalf.
- ADR-0040 remains authoritative for the non-Namespace and no-fallback
  character of retained Alpha identities, but not for live Alpha resolution.
- ADR-0041 retains the ACA2 and supplied-bytes inspection separation, but no
  longer requires a maintained accepting corpus-floor route.
- ADR-0042 retains its enrollment-v3 grammar as an accepted compatibility
  contract, but no longer requires the corpus command to remain accepting.

## Compliance

The [Endpoint Alpha destination retirement contract](../technical/endpoint-service-runtime.md#alpha-destination-retirement)
owns the exact observable effect boundary. The product scope and command
reference link to that owner rather than repeating the transition state
machine.
