# Development documentation

This directory owns current engineering policy, factual source maps, and
developer workflow. It is the maintained route for contributors; it does not
make stage briefs, experiments, plans, or generated receipts a second
specification.

## Current engineering route

- [Documentation ownership and promotion](documentation.md) defines which
  document owns a fact and how stage material is retired.
- [Development entry gates](entry-gates.md) defines the required evidence before
  a change enters a maintained product path.
- [Go engineering rules](go-engineering.md) define package, file, dependency,
  and test expectations.
- [Repository layout and growth rules](repository-layout.md) define the factual
  tree and permitted growth.
- [Testing model](testing.md) defines the selected execution profiles.
- [Dependency register](dependencies.md) records reviewed runtime dependencies.
- [Scoped risk exceptions](scoped-risk-exceptions.md) records any accepted,
  bounded exception to the normal engineering rules.

## Current Stage 8 controls

- [Target architecture](stage-8-target-architecture.md) is the intended
  maintained shape after the active migration.
- [Refactoring and retirement plan](refactoring-plan.md) is the active work
  control while Stage 8 is in progress.
- [Current package map](package-map.md) is the factual register of maintained
  Go packages and their permitted imports.
- [Compatibility observer inventory](stage-8-compatibility-observer-inventory.md)
  and [preservation ledger](stage-8-preservation-ledger.md) record the bounded
  compatibility work that remains during the migration.

## Current technical references

- [Private naming and namespace](../technical/naming.md)
- [Release update and authority custody](../technical/release-update-custody.md)
- [Endpoint and Service runtime](../technical/endpoint-service-runtime.md)

## Historical provenance

The Horizon 3 briefs, Stage 5--7 evidence and specifications, completed
Stage 8 inventories and reviews, stopped campaign material, and completed
research records explain how current decisions were reached. They are not
current commands, package contracts, or qualification evidence. Use them only
when a current owner, ADR, or research record needs provenance; otherwise start
with the routes above.

The repository preserves their Git history. A completed disposable experiment
is removed when its result is promoted, rejected, or superseded; only an active
experiment may remain under [experiments](../../experiments/README.md).
