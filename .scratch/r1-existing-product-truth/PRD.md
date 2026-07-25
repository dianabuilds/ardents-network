# R1 Existing Product Truth

Status: completed

## Outcome

Replace inferred product maturity with bounded, commit-bound research and
machine-verifiable supported-interface evidence without claiming production
release qualification.

## Scope

- Application installation journey research and accepted implementation slices.
- Operator command smoke research and accepted implementation slices.
- Feature/evidence catalogue research and the accepted FEC-001/FEC-002 slices.

Application Discovery and AD-01 through AD-04 remain a separate workstream.
R3 environment execution and production release qualification remain out of
scope.

## Current baseline

The research packets are bound to
`180decc1b03f94a6115b59a4046b4795308ec235`. The implementation series ends at
`8b9f8ad87fb78fccd7a73d445f2d72dbf2e51b4c`.

## Completed issue order

```text
AIJ-01 Protected Application Ticket Handoff [completed]
  -> AIJ-02 Installation recovery and revocation [completed]

OCS-01 Fail-closed Operator command contract [completed]
  +--> OCS-02 Node/Network/Diagnostics procedures [completed]
  +--> OCS-03 Workload/Hosted-service procedures [completed]
  +--> OCS-04 Content/Transfer procedures [completed]
  +--> OCS-05 Principal access procedures [completed]

FEC-001 Canonical capability truth [completed]
  -> FEC-002 Evidence promotion and active claims [completed]
```

R3 environment execution and qualification snapshots remain separate.
