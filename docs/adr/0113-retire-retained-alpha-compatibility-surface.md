---
status: accepted
date: 2026-09-27
partially-supersedes: ADR-0088 (retained corpus parser and read-only floor reader compatibility obligation only), ADR-0098 (retained alpha-only Service Link grammar consequence only)
---

# ADR-0113 — The retained Alpha corpus/link compatibility surface and the local Name wire encoder are retired; the whole `name` command family refuses before effects

(Repository-cleanup card after the dev-merge qualification. ADR-0088 kept the
Alpha corpus parser and its read-only persistent-floor reader as a
compatibility obligation "until a separate bounded decision both proves that
no maintained consumer remains and defines migration or data retention". This
ADR is that bounded decision. The product owner's standing decisions apply:
legacy dies and no support or backward compatibility is owed.)

## Context

- `internal/naming/alpha` has zero production importers. Its entire exported
  surface (40 symbols across the deadcode allowlist groups "retained Alpha
  corpus/link compatibility" and "retained Alpha corpus floor/history
  reader") is reached only from two refusal-evidence tests that build
  realistic retired bytes: the Endpoint target-link retirement test and the
  e2e `accept-alpha-corpus` retirement test.
- The live ACA1 control-inspection path (`internal/alphacontrol`, retained by
  ADR-0110) is self-contained: it imports neither `internal/naming/alpha` nor
  reads persistent corpus floors. Retired `accept-alpha-corpus` and
  `inspect-alpha-corpus` refuse at dispatch before any file or floor touch
  (ADR-0110).
- `internal/naming` has exactly one production caller: `ardents name encode`,
  a local canonical Stage-6 wire encoder. Nothing maintained consumes its
  output — `DecodeWire` has been dead since the ADR-0100 resolution
  retirement, and protected text uses explicit Target Links.
- `enrollment.VerifyRunningCompanion` (ADR-0042-era companion proof for a
  bounded participant tool) and the exported `alphacontrol.VerifyComponent`
  wrapper have no production callers; the live inspect path calls the
  unexported `verifiedComponent` directly, and `Verify` itself already pins
  the running artifact through `exactExecutable`.

## Decision

1. `internal/naming/alpha` is deleted outright: the corpus grammar, the
   alpha-only Service Link grammar, and the persistent/session floor readers
   and writers. Existing corpus-floor and corpus bytes on disk stay
   byte-for-byte; typed incompatibility is the absence of any read path
   (the ADR-0105/ADR-0109 precedent). No migration, conversion, or grace
   reader is provided. The ADR-0088 compatibility-obligation clause and the
   ADR-0098 retained Service-Link-grammar consequence are superseded.
2. `ardents name encode` is retired. The whole `name` verb family
   (`encode`, `resolve`, `control`) returns the existing
   `errNameNetworkCommandRetired` refusal before remaining arguments or
   effects, and `internal/naming` is deleted with its last caller. The
   frozen Stage-6 wire grammar dies with its final consumer; canonical Name
   syntax remains a possible future stage only under an entirely new scoped
   design (unchanged ADR-0081 boundary).
3. `enrollment.VerifyRunningCompanion` is deleted; `exactExecutable` remains
   as `Verify`'s own running-artifact gate. The exported
   `alphacontrol.VerifyComponent` wrapper is deleted; the unexported
   `verifiedComponent` remains the live component verifier.
4. The two refusal-evidence tests keep their zero-effect proofs with
   test-local historic builders: the e2e reproduces the retired corpus wire
   layout in a fixture-local signed builder (the ADR-0110
   `historicAlphaCatalogV2` precedent) and both tests seed synthetic
   floor-shaped bytes. The "retained floor reopens and still resolves"
   assertion is retired together with the reader; the surviving evidence is
   refusal-before-effects plus byte-for-byte floor preservation.
5. The deadcode allowlist common scope shrinks to the "test fixture support"
   group; the four retained-compatibility groups are deleted with their
   symbols.

## Consequences

- No maintained code can read, convert, or delete retired Alpha corpus or
  floor bytes; they are inert evidence on disk.
- `ardents name` has no live verb; its usage error lists the retired family.
- The Windows re-execution enrollment test now proves the live
  running-artifact identity through `Verify` instead of the retired
  companion verifier.
- Registries contract: two deterministic packages, two package-map rows, two
  package-graph rows, two disposition rows (retargeted to the retirement),
  eighteen file-map rows, and 42 allowlist symbols disappear.

## Verification

- `internal/architecture` guards assert the absence of `internal/naming`
  (whole tree), the exact whole-family `name` refusal, the absence of the
  exported `VerifyComponent` and `VerifyRunningCompanion`, and the absence
  of the removed packages from the deterministic profile and allowlist.
- The e2e retirement test still proves `accept-alpha-corpus is retired` with
  realistic signed historic corpus bytes, no created roots, and unchanged
  floor bytes, under `-race` on Linux.
- Full Windows suite, `make quick-check`, staticcheck, exact bidirectional
  deadcode compare, and the docker Linux race battery.
