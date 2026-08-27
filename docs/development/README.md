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
- [Deep audit campaign](deep-audit.md) defines the inactive whole-codebase
  review, proof, remediation, and requalification process for a future exact
  H4 release candidate.
- [Dependency register](dependencies.md) records reviewed runtime dependencies.
- [Scoped risk exceptions](scoped-risk-exceptions.md) records any accepted,
  bounded exception to the normal engineering rules.
- [Current package map](package-map.md) is the factual register of maintained
  Go packages and their permitted imports.

## Current technical references

- [Private naming and namespace](../technical/naming.md)
- [Release update and authority custody](../technical/release-update-custody.md)
- [Endpoint and Service runtime](../technical/endpoint-service-runtime.md)
- [Network State, Entry, Route, and Node](../technical/network-route-node.md)
- [Current command reference](../reference/commands.md)

## Historical provenance

Closed stage material, completed research records, and disposable experiments
are removed after their retained facts gain a current owner. Git history is the
provenance route; it is not a current command, package, or Qualification
contract. Start with the routes above unless a current ADR or active research
record explicitly needs that history.

Only an active experiment may remain under
[experiments](../../experiments/README.md).
