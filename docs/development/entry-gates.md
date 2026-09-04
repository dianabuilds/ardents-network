# Development entry gates

This document controls entry into maintained project work. Product scope,
accepted ADRs, the threat model, current technical documentation, and the
package map are the authority order. Historical stage material and closed
research do not authorize behavior.

For C0 work, begin with the product scope and threat model, then read the
smallest affected current technical and development owners. Consult an accepted
ADR only through a link from those owners; consult closed research, experiments,
audit receipts, compatibility material, or Git history only as named provenance.
Broad historical search is not a substitute for identifying the current owner.

## Research and experiments

An experiment starts only with an active question in
[the research queue](../research/questions.md), a falsifiable hypothesis,
declared inputs and environment, an evidence plan, and a disposal condition.
It belongs under `experiments/` and cannot create a maintained dependency,
protocol, or compatibility promise.

## Maintained code

A maintained change requires all of the following before integration:

- a current product or technical contract that grants the behavior;
- a package-map entry and a cohesive responsibility for every affected package;
- reviewed dependencies in [the dependency register](dependencies.md) before
  `go.mod` changes;
- behavior tests and at least one real non-test caller for a new package;
- `make quick-check` while writing and `make check` before integration.

A consequential, hard-to-reverse selection also needs an accepted ADR. A
temporary experiment, old implementation, popular library, or historical
campaign result cannot silently choose a route, storage engine, consensus
system, public wire protocol, or product runtime.

## Product and security claims

A release, privacy, security, availability, or platform claim requires the
specific conditions and measurements in the
[threat model](../security/threat-model.md). Passing local tests or a
project-controlled host is implementation evidence, not independent
Qualification. Missing native host, Docker, binary, privilege, or orchestration
prerequisites make an execution profile invalid rather than passing by skip.
