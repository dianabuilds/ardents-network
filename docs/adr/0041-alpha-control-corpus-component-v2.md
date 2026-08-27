---
status: accepted
date: 2026-08-25
---

# ADR-0041 — Add a separate signed corpus component through alpha-control v2

## Context

ADR-0038 deliberately fixes `ACA1` to three components and excludes H4-4.
H4-4A needs an alpha participant to inspect the exact finite Alpha Name Corpus
and preserve a local anti-rollback floor without making the catalog, release,
or Network State key naming authority.

## Decision

Keep `ACA1` and its reader unchanged. A versioned `ACA2` successor has exactly
four fixed components: Release, Network, Compatibility, and Alpha Corpus. The
new Alpha Corpus component contains no location, mirror, or replacement key.
It binds only one exact authority-signed Alpha Name Corpus byte sequence, its
digest/length, cohort, Network, serial, expiry, and identifier of the already
enrollment-pinned corpus public key.

The corpus signing key is independent of the `ACA2` disclosure, Release,
Network, and Compatibility keys. A valid `ACA2` catalog cannot make altered
corpus bytes valid; a valid corpus key cannot authorize a build, Node duty, or
canonical Namespace claim. An `ACA2` reader remains inspection-only. A distinct
Endpoint-owned corpus state root retains the accepted serial/digest and corpus
bytes after the corpus component has passed both verifiers; it never shares an
H4-6 reader floor or accepts `ACA1` as an alpha corpus source.

## Consequences

- H4-4A can gain a transparent named-alpha input without silently changing a
  closed alpha-control parser or collocating unrelated authorities.
- A fresh bundle needs one additional independently pinned `corpus.pub`
  companion; ordinary replacement stays explicit under its durable floor.
- The initially maintained reader can report the component without authorizing
  Endpoint startup; active Endpoint composition and live delivery still require
  separate evidence.
- Canonical `ardents://` Namespace work remains unchanged.

## Compliance

- [R-113](../research/records/r-113-alpha-corpus-distribution-floor.md) owns
  the artifact/floor experiment and promotion evidence.
- [ADR-0038](0038-alpha-control-disclosure-reader-v1.md) remains authoritative
  for `ACA1` and is not modified by this successor.
- [ADR-0040](0040-bounded-alpha-name-overlay.md) owns the alpha naming and
  browser boundary.
