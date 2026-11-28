---
status: accepted
date: 2026-11-28
supersedes: ADR-0122 only for the Carrier set of the nearest closed release
---

# ADR-0136 — Return QUIC to the nearest closed release from a patched official source

The Product Owner selects QUIC together with TCP/TLS as the Carrier set of the
nearest closed release, which ADR-0122 had reduced to TCP/TLS alone. The blocker
ADR-0122 named, the server reply budget of R-172, is removed at the library:
official quic-go v0.66.0 with the retained correction
`quic-go-v0.66.0-probe-budget.patch` answers a small packet from an unvalidated
address with at most three times the received payload. This was measured on
macOS arm64 and the Linux x86-64 stands with both server writers and the race
detector. Every unpatched official release through v0.66.0, and `master` on
this date, answers with 1200 bytes. No official release carries the correction,
and the timing of the upstream project is not ours, so the release takes the
correction from a source copy inside the repository. The multipath fork
`qoke/mp-quic-go` stays rejected: it fails the maintenance and vulnerability
acceptance rule of the dependency register.

This decision supersedes the Carrier-set clauses of ADR-0122 for this release,
including its selection of no upstream submission and no private fork for that
purpose. The dual-Carrier selection and qualification clauses of ADR-0048 and
ADR-0081 apply again. Every other requirement of those decisions stays:
canonical signed identities, no fallback between Carriers, genuine authority,
and the mandatory gates.

## Source of the dependency

- The dependency stays `github.com/quic-go/quic-go v0.66.0` with its published
  module checksum. A `replace` points it at `third_party/quic-go`: the files of
  that release that the root module and its tests import, unchanged, plus the
  one retained patch. The patch file is kept in
  `experiments/r-172-quic-return-path/` next to the runner that measures it.
- A repository test proves that the directory equals the published module with
  the patch applied. No other local change is permitted there.
- A new upstream release replaces the directory wholesale. The patch is then
  applied again with its exact context and fails loudly where upstream changed
  the same code. Upstream's own unit tests run inside the directory as a
  required repository check.
- The directory leaves the repository on the first official release that
  carries the correction, verified by the session test of R-172.
- The official module path and version stay visible to `govulncheck`. Each
  dependency review still maps the advisories of the release by hand.
- This is a time-bounded exception to the register's rule that no vendor tree
  is committed and to the repository's one root Go module, which still covers
  all maintained code. The directory is a dependency source, not project code,
  and the first-party rules for `panic`, implicit `init`, `unsafe` and file
  ownership do not apply inside it. It covers one dependency and one patch.

## Required behavior

- Signed QUIC profile identifiers keep their canonical bytes and meaning. The
  Carrier selection admits QUIC again only through the registered listeners and
  clients, after the same pre-output checks as TCP/TLS. There is no fallback
  between Carriers.
- The qualification of the changed release covers both Carriers: installed Node
  and Service execution on the Linux stands, Windows execution in the
  continuous integration lane, race, cancellation and joined termination. The
  open rows of R-172 name what is missing until each is measured.
- The correction is offered to the upstream project through its private
  vulnerability reporting. Its acceptance and timing do not change this
  decision.

## Delivery

The return is delivered in stages, and GitHub owns their status. Until the
Carrier selection admits QUIC and the both-Carrier qualification is accepted,
the product keeps refusing QUIC before any effect, as ADR-0122 required.
Selection alone is neither implementation nor Route acceptance.
