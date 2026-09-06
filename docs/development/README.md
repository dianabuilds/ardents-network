# Development documentation

This directory owns current engineering policy, factual source maps, and
developer workflow. It is the maintained route for contributors; it does not
make stage briefs, experiments, plans, or generated receipts a second
specification.

## Current engineering route

- [Documentation ownership and promotion](documentation.md) defines which
  document owns a fact and how stage material is retired.
- [Contributing](../../CONTRIBUTING.md) defines change prerequisites, research
  entry, local setup, and the integration workflow.
- [Repository layout and growth rules](repository-layout.md) define Go code
  rules, architecture review, the factual tree, and permitted growth.
- [Testing model](testing.md) defines the selected execution profiles.
- [Deep audit campaign](deep-audit.md) defines the whole-codebase review,
  proof, remediation, and requalification method for an exact frozen C0
  candidate.
- [Dependency register](dependencies.md) records reviewed runtime dependencies.
- [Scoped risk exceptions](scoped-risk-exceptions.md) records any accepted,
  bounded exception to the normal engineering rules.
- [Current package map](package-map.md) is the factual register of maintained
  Go packages and their permitted imports.
- [Command surface inventory](command-surface.md) records the process, artifact,
  route, retirement, and future-decision boundaries for every maintained binary.

## Current technical references

- [Private naming and namespace](../technical/naming.md)
- [Release update and authority custody](../technical/release-update-custody.md)
- [Endpoint and Service runtime](../technical/endpoint-service-runtime.md)
- [Closed-alpha enrollment verification](../technical/enrollment-verification.md)
- [Network State, Entry, Route, and Node](../technical/network-route-node.md)
- [Current command reference](../reference/commands.md)

## Retained audit receipts

- [C0 run-2 Track A receipt](audit-receipts/c0-run-2-track-a.md) retains the
  exact blocked Gate A identity and external evidence digest; it is historical
  evidence, not a current product or Qualification claim.
- [C0 run-2 Gate A remediation receipt](audit-receipts/c0-run-2-gate-a-remediation.md)
  records the exact accepted source-artifact candidate, all 15 finding
  dispositions, repeated affected-cell evidence, and reproducible artifact
  digests. Track B remains unstarted.

## Historical provenance

Completed research records and accepted ADRs remain decision evidence outside
the normal reading route. Superseded stage procedures and disposable
experiments are retired to Git after their unique current facts gain an owner,
as defined by [documentation policy](documentation.md#promotion-and-retirement).
Neither a retained record nor Git history is a current command, package, or
Qualification contract.

Completed experiment source is available from Git history and its accepted
research record; no experiment is part of the current C0 tree.
