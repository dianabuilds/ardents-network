---
status: accepted
date: 2026-09-27
partially-supersedes: ADR-0089 (the retained bounded no-start Contributor diagnose/drain/withdraw/remove obligation only)
---

# ADR-0114 — The dedicated-host Contributor retirement mechanism is removed entirely; the ADR-0089 owned-shutdown obligation closes for absence of object

(Retirement-completion card. ADR-0089/R-155 selected H1 — refuse every new
old start while preserving bounded no-start diagnose/drain/withdraw/remove for
an *exactly authenticated existing* dedicated-host installation — and rejected
H2 (remove everything) solely because removing the lifecycle commands would
orphan owner-held resources. This ADR records that the preservation premise no
longer holds and executes the deletion path that finding F-56 already named:
"removing the command, package, runbook and historical decoder together".)

## Context

- The only installations that could ever exist were created by the project's
  own H4-5 dedicated-host Functional Alpha campaign (bounded campaign stopped
  2026-08-22, alpha accepted 2026-08-29, `h4-alpha-1-rc*` candidates). The
  runbook stated explicitly: "This is not a public Contributor offer or a
  capacity/availability claim." No third party ever received an apply path,
  an enrollment, or a bundle capable of creating the installation.
- The Product Owner confirmed on 2026-09-27 that no live
  `ardents-rendezvous-contributor` installation remains: the H4-5
  qualification environments are discarded. The ADR-0089 owned-shutdown
  obligation therefore has no object, and keeping the mechanism would
  preserve an authenticated de-installer for zero installations.
- The mechanism was already contracting: the Apply installation/update engine
  was removed (`c9b63af0`), leaving only persisted-format readers, no-start
  interrupted-update recovery, the four retirement actions, their root lease,
  the systemd supervisor adapter, the retirement oracles, and one selected
  mutation-fuzz target.
- The future product role "Network Contributor" (NET-01D, vision, journeys,
  operating model) is a concept for the closed Node duties under an entirely
  new design. It is not served by this mechanism and its documentation is
  unaffected by this removal.

## Decision

1. `internal/contributor` is deleted outright: host paths, authenticated
   installation/update-record readers, no-start recovery, lifecycle
   diagnostics, drain/withdraw/remove, root lease, and retained fixtures.
   Any residual installation root or record, if it ever surfaced, stays on
   disk byte-for-byte with no reader, converter, or deleter; typed
   incompatibility is the absence of any read path (the
   ADR-0105/ADR-0109/ADR-0113 precedent). No migration or grace reader is
   provided.
2. The `ardents-node contributor` subcommand is deleted. `contributor` is no
   longer a recognized command shape: like any unknown argument it fails with
   the standard usage error before any platform, bundle, installation,
   supervisor, output, or Network effect. The former typed
   `old Contributor start is retired` refusal is superseded by that generic
   zero-effect refusal; no profile or deployment identity can restore a route.
3. The `Rendezvous Contributor` runbook is deleted and its links are removed
   from the maintained command, scope, transition, testing, command-surface,
   package-map, and Network/Node owner documents. Historical records — R-155,
   the ADR-0089 text, reconstruction findings, research records, and audit
   receipts — remain unchanged.
4. The ADR-0089 clause retaining bounded no-start diagnose/drain/withdraw/
   remove for an authenticated owned installation is superseded. The rest of
   ADR-0089 is unchanged: the old Node-reservation, Source-profile, and
   Transit-issuer start refusals remain integrated.
5. Retained adjacent surfaces are explicitly out of scope: the
   `ardents-rendezvous-dedicated-host-v1` resource-placement identity in
   `internal/resource` remains the selected dedicated-host profile for the
   closed Node plans, and the joined installed clock observation in
   `internal/node` remains a live mechanism (its historical
   `Contributor`-named symbols await a separate Node-owner rename slice).

## Consequences

- Registries contract by one package-map row, one deterministic-package
  entry, one selected fuzz target with its architecture-profile rows, one
  reference document, and four `cmd/ardents-node` files (mode adapter, its
  test, and the two systemd supervisor adapters).
- No maintained code can read, reconcile, stop, or remove an old dedicated-host
  installation; discovering one is an ordinary operator filesystem matter
  outside the product surface.
- The command usage test now pins the retired `contributor` shapes
  (diagnose, restart, apply, remove) to the zero-effect usage refusal.

## Verification

- `go build ./...` and `go vet` pass with no reference to
  `internal/contributor` remaining in any Go source.
- The extended `cmd/ardents-node` usage test proves every retired contributor
  command shape fails with the usage error before effects.
- The architecture fuzz-profile test asserts the single-target State
  inventory, its declaration, and the exact mutation command.
- Windows `make quick-check` passes, repeated by the pre-commit hook at
  commit time.
