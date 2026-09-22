---
id: R-156
title: How do old Name operator network commands retire without retiring Service Names?
status: completed
owner: Product Owner and Codex
started: 2026-09-22
reviewed: 2026-09-22
---

# R-156 — How do old Name operator network commands retire without retiring Service Names?

## Decision this unlocks

Decide the bounded disposition required by issue #145 before issue #99 may
change `ardents name resolve` and `ardents name control`. The decision must
separate the old operator HTTP/OHTTP adapters from the retained Service Name
product function, canonical encoding, Namespace lifecycle and proofs, and
custody consumers.

## Current contract

The [C0 product scope](../../product/scope.md) does not select canonical Service
Names or Namespace governance for C0, while retaining human-facing Service
Names as a product function over the future protected protocol. The
[Naming owner](../../technical/naming.md) keeps the canonical local modules and
states that no maintained production runtime composes a Gateway or Resolver.
The [threat model](../../security/threat-model.md) forbids treating encrypted
payloads as anonymity or silently falling back to an unselected path.

Today `cmd/ardents` still composes `name resolve` and `name control` through the
old private-resolution HTTP/OHTTP runtime. `name encode` is a separate local
operation. Product Owner direction in issue #140 retires old network
implementations while retaining Service Names as a product function.

This record selects no successor Naming wire, Resolver/Gateway topology,
authority or governance, migration, conversion, data deletion, AAI3 extension,
or public Namespace. Runtime refusal remains the separate issue #99.

## Hypotheses

- **H1:** Retire only the two recognized operator network commands before any
  input, State, transport, output, or Namespace effect, while retaining local
  encoding and the separately consumed Naming, Namespace, and custody modules.
- **H2:** Keep the HTTP/OHTTP commands until the successor protected Naming
  path exists so the Service Name product function remains demonstrable.
- **H3:** Remove the commands and all Naming, Namespace, resolution, and custody
  modules together because they belong to the same historical feature.
- **H0:** The current command and module boundaries cannot express a safe
  retirement without first choosing the successor Naming protocol.

## Evaluation criteria

- A recognized `name resolve` or `name control` request has one deterministic,
  operator-readable non-success outcome before its first observable effect.
- The decision does not imply that protected Service Name access is available.
- `name encode`, canonical bytes, Namespace proofs/lifecycle, custody behavior,
  existing roots, journals, floors, and records remain unchanged.
- Retained production consumers and evidence-only modules are distinguished
  from the two accepting command adapters.
- No fallback, protocol translation, automatic migration, or replacement
  authority is inferred.
- The later implementation has a finite oracle: zero transport attempts, no
  output, and unchanged State/Namespace paths for previously valid requests.

H1 is falsified if the command cannot refuse before reading its plan or
operation, opening State, constructing HTTP/OHTTP work, writing output, or
mutating Namespace state. H2 is falsified if retaining the command keeps an
unselected old network path without being required by any current product
journey. H3 is falsified if the same packages have separate current consumers
or retained compatibility obligations. H0 is selected only if command
composition cannot be separated from canonical local behavior.

## Evidence plan

### Primary sources

- Current product, threat-model, Naming, command-reference, and command-surface
  owners in this repository, accessed 2026-09-22.
- `cmd/ardents/name.go`, `name_resolution.go`,
  `name_resolution_input.go`, `name_control.go`, and their tests at the exact
  source revision recorded below, inspected 2026-09-22.
- Production import closure for `internal/naming`,
  `internal/naming/namespace`, and `internal/naming/resolution`, inspected
  2026-09-22.
- GitHub issues #99, #140, and #145, accessed 2026-09-22.

### Experiment

No runtime experiment is required for this boundary decision. The reproducible
check is an exact command-dispatch and production-import inspection at the
recorded revision. Issue #99 owns the executable early-refusal oracle and the
unchanged-byte checks.

### Failure scenarios

- Refusal follows context decoding, plan or operation reads, State recovery,
  HTTP/OHTTP construction, network activity, output, or Namespace mutation.
- Removing the commands is described as delivery of protected Service Name
  access or as deletion of the Naming product function.
- `name encode` or canonical wire bytes change as a side effect.
- Namespace roots, records, pending journals, floors, proofs, or custody state
  are deleted, rewritten, converted, or treated as successor authority.
- An old adapter silently falls back to a Target Link, AAI3, ambient DNS, or a
  newly invented Resolver/Gateway path.

## Findings

The inspected source revision is
`0d5af59e02d2998430ff2978fc4c6013b45df87d`.

### Command adapters and effects

- **Sourced fact:** `cmd/ardents/name.go` dispatches `encode`, `resolve`, and
  `control`. `encode` calls only canonical `naming.Parse` and `EncodeWire`.
  `resolve` and `control` decode a context before calling their network
  adapters.
- **Sourced fact:** `name_resolution.go` reads an operator plan, opens an
  authenticated Network State resolution view, constructs the private
  resolution client with an `http.Transport`, performs the exchange, and
  writes a receipt.
- **Sourced fact:** `name_control.go` reads the same network input, opens State,
  constructs the private control client with an `http.Transport`, reads the
  operation, executes it, and writes a receipt.
- **Inference:** The correct retirement boundary is the subcommand dispatch.
  A recognized `resolve` or `control` verb can return before argument-derived
  context decoding and therefore before every named file, State, transport,
  output, or Namespace effect.

### Retained modules and consumers

- **Sourced fact:** `internal/naming` supplies canonical parsing and wire
  encoding to `name encode` and to Namespace record, claim, proof, recovery,
  admission, and retained compatibility packages.
- **Sourced fact:** production custody code imports Namespace authority,
  record, and epoch packages for bounded Name Authority operations. Namespace
  packages also own persisted lifecycle/proof behavior independent of the two
  command adapters.
- **Sourced fact:** `internal/naming/resolution` is imported by production code
  only through the three old `cmd/ardents` Name adapter files. The current
  Naming owner already states that its maintained module behavior has no
  maintained production Gateway/Resolver composition.
- **Inference:** Issue #99 may close the two command callers without deleting
  canonical Naming, Namespace, or custody. The resolution package becomes an
  uncomposed retained module/evidence surface; deleting or redesigning it
  requires a separate proven-consumer slice and is not part of command
  retirement.
- **Inference:** Existing State and Namespace bytes remain evidence and module
  state, not an automatic migration input or authority for a successor.
- **Inference:** No new product term is introduced. The existing `Service
  Name`, `Namespace`, `Resolver`, and `Private Resolution` glossary entries are
  sufficient, so `CONTEXT.md` needs no change.

## Options

- **H1 — retire the two command adapters and retain separate modules.** Meets
  every criterion and removes the accepting old network path without pretending
  to solve successor Naming.
- **H2 — keep the commands until a successor exists.** Rejected. No current C0
  journey or maintained production Gateway/Resolver requires them, and product
  continuity does not justify accepting the old network runtime.
- **H3 — delete the whole Naming closure.** Rejected. Canonical encoding,
  Namespace lifecycle/proofs, and custody have separate consumers and retained
  state obligations. Resolution-package deletion also needs its own exact
  closure proof after the command gate.
- **H0 — require the successor protocol first.** Rejected. The dispatch boundary
  already separates local canonical behavior from the two network commands.

## Recommendation

Select H1.

Keep `ardents name encode <name>` and its exact canonical output. For either
recognized `ardents name resolve ...` or `ardents name control ...` command,
return a deterministic retirement refusal at command dispatch before validating
the remaining arguments, decoding context, reading any input or operation file,
opening State, constructing or using HTTP/OHTTP transport, writing output, or
touching Namespace state. The operator explanation must say that the old Name
network command is retired and that protected Service Name access is not yet
selected; it must not advertise an automatic fallback.

Retain canonical Naming, Namespace lifecycle/proofs, and custody with their
existing consumers and bytes. Retain the uncomposed resolution module as
evidence/compatibility surface until a separately scoped closure decision.
Do not delete, convert, or reset State/Namespace roots, records, journals,
floors, or authority material. Do not select a wire, Resolver/Gateway topology,
authority, migration, Target-Link translation, ambient-DNS fallback, or AAI3
Name route.

Confidence is high because the command dispatch precedes every named effect and
the production import closure cleanly identifies the separate consumers. The
strongest objection is that removing the demonstrator leaves no operator Name
network route. That is an honest availability limit, not authority to preserve
an unselected old runtime.

## Disposition

Completed on 2026-09-22 and promoted to
[ADR-0090](../../adr/0090-retire-name-operator-network-adapters.md), the product
scope, Naming owner, command reference, and command-surface inventory. Runtime
refusal remains the separate bounded issue #99; this record does not make it
integrated or implement protected Service Name access.
