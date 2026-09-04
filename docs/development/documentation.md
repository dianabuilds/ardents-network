# Documentation ownership and promotion

This policy governs current technical and operational truth. It does not make a
stage brief, experiment, test, implementation plan, or generated receipt a
second product specification.

## Authorities and reader routes

Product scope, journeys, and honest promises belong in `docs/product/`.
Threats, protected information, adversaries, conditions, measurements, and
limitations belong in `docs/security/`. Accepted hard-to-reverse decisions live
in `docs/adr/`; decision evidence and falsification live in `docs/research/`.
`docs/development/` owns engineering policy, factual source maps, and developer
workflow. Implementation and operator facts belong in the smallest current
technical, operations, or reference document that has the responsible Module
or command as its source.

The normal routes are: contributor from `README.md` to product scope, threat
model, the affected current technical owner, and package map; operator from
the runbook to command and configuration reference; auditor from product/threat
contract to current technical/format/Qualification facts and then selected
ADR/research provenance. An ADR index, research queue, experiment directory,
receipt, compatibility tree, and Git history are not default implementation
routes. No reader should have to infer implemented behavior from stage
chronology.

## Promotion and retirement

A code, format, command, support, or operational change updates its current
owner in the same change. It states normal and failure behavior, boundaries,
compatibility/retirement conditions, and honest limitations where applicable.
An ADR or research record remains rationale/evidence after that promotion; it
does not become the operator manual.

Stage material is transitional. Before it is deleted, its unique current fact
is promoted to one canonical owner, inbound links are repaired, and any needed
historical source identity or claim evidence remains recoverable in Git or the
declared external evidence location. A document is not retained merely because
it is lengthy, stage-named, or previously reviewed.

New documentation earns a separate file only for a distinct authority,
audience/task, change cadence, or audit-retention need. Do not mirror package
file order or create empty future manuals. A target technical, operations, or
reference document first appears with real owned behavior; a package rename is
not a documentation boundary by itself.

The C0 delivery backlog and its live status belong in GitHub Issues under the
[`C0 Closed Alpha` milestone](https://github.com/dianabuilds/ardents-network/milestones),
not in an ADR, research record, experiment README, or product contract. Its
absence blocks a new C0 implementation slice. At most one C0 implementation
issue and one explicitly selected research question may be in progress. A
current document may link to a tracker item for operational status, but it
stays a stable contract rather than a second backlog.

## Checks

The package map and dependency register remain factual and are architecture-gate
inputs. A retained command/configuration/format compatibility promise has a
named current reference and a behavior or compatibility test. A security or
privacy claim links to its product/threat statement and the named Qualification
evidence instead of duplicating a stronger shorthand claim in implementation
documentation.
