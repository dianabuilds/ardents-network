---
status: accepted
date: 2026-09-27
supersedes: ADR-0041 (maintained ACA2 inspection); ADR-0088 (retained supplied-bytes ACA2 diagnostic); ADR-0091 (retained inspect-alpha-corpus command)
---

# ADR-0110 — Keep ACA1 as the sole maintained control inspection format

## Context

The standalone `ardents-control inspect-bundle` and `inspect-transitions`
commands verify the enrollment-pinned ACA1 catalog and its Release, Network,
and Compatibility components. Their inspection root retains a catalog floor
and separate Release and Network floors. Inspection does not authorize an
Endpoint action.

`inspect-alpha-corpus` instead verifies supplied ACA2 and Alpha Name Corpus
bytes. It opens no inspection root and retains no floor. ADR-0088 retired fresh
corpus intake and Alpha Service Link resolution, leaving ACA2 as a historical
diagnostic for a component absent from the supported text-Service journey.
F-49 identifies these as two maintained command formats, not two versions of
one accepting Endpoint protocol. The Product Owner selected one supported
version and no backward-compatibility obligation for retired paths.

## Decision

1. ACA1 remains the sole maintained alpha-control disclosure-inspection
   format. Its exact three-component grammar, enrollment pins, catalog floor,
   and independent Release and Network inspection floors remain in their
   existing owners. `inspect-bundle` and `inspect-transitions` continue to use
   one ACA1 reader; neither gains Endpoint authority.
2. Retire the accepting `inspect-alpha-corpus` command and the production ACA2
   catalog/corpus inspection code in one bounded implementation change. An
   explicit invocation of the retired command refuses before parsing its
   arguments or opening a file or root. Historical ACA2 bytes remain available
   through Git provenance, not a maintained parser or second operator format.
3. There is no ACA2 inspection floor to migrate. Existing ACA1 inspection
   roots retain their floors and reopen unchanged. Do not convert ACA2 bytes
   to ACA1: the fourth corpus component has no ACA1 authority or class.
4. Existing Alpha Corpus floor bytes and their separately retained read-only
   compatibility reader are outside this decision. The retired ACA2 command
   never owned that root. Enrollment v2/v3 parsing and its `corpus.pub`
   companion are also outside this change; their acceptance and retirement
   belong to the separate enrollment-format decision.
5. ADR-0038 remains authoritative for ACA1. This decision supersedes only
   ADR-0041's and ADR-0088's requirement to maintain ACA2 supplied-bytes
   inspection, and ADR-0091's explicit command-retention statement. Historical
   research and accepted evidence are not rewritten.

## Implementation boundary

Remove the `inspect-alpha-corpus` accepting branch and its command-only
fixtures, `inspection.VerifyACA2Corpus`/`VerifyCorpusComponent`, and the ACA2
catalog verifier only after confirming their exact remaining callers. Preserve
ACA1 inspection and its restart, replay, conflict, and separate-root tests.
The retired command must have a before-effect refusal test using nonexistent
or hostile paths and unchanged ACA1 and Alpha Corpus roots. Update the command
reference, package map, command-route map, and technical owner in the same
implementation slice; do not claim the command has already been removed.
