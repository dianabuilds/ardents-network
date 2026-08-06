# Development entry gates

The repository intentionally separates **research code**, **tracer code**, and
**production code**. Passing time or accumulating code does not promote one
category into the next.

## Gate A — start an experiment

An experiment may be written when:

- it answers one named question from the research queue;
- a falsifiable hypothesis and comparison criteria are written first;
- the product journey and threat-model claim being tested are linked;
- inputs, environment, evidence, and cleanup behavior are defined;
- its directory is under `experiments/` and makes no compatibility promise.

An experiment may use any suitable language or maintained library. That choice
does not select the production stack.

## Gate B — start the tracer implementation

The Named Private Site + Anonymous Mailbox tracer may gain a shared codebase only
after review of:

- R-001 Route Profile adversaries and claims;
- R-002 Site Bundle/application boundary;
- R-003 naming and recovery constitution;
- R-006 recovery, Device, Persona, and messaging state model;
- R-007 minimum Replica/Mailbox semantics;
- R-017 validation that the tracer demonstrates differentiated user value.

The tracer must declare which components are simulations, central test fixtures,
or deliberately insecure placeholders. Those declarations must be visible in
the user journey and test output.

## Gate C — select production language and foundations

A production language, runtime, transport, storage foundation, or wire format is
selected only when:

- the same accepted tracer contract has been prototyped or evaluated fairly;
- security and maintenance evidence covers proposed dependencies;
- memory-safety, concurrency, mobile/desktop support, reproducible builds,
  fuzzing, interoperability, and contributor learning cost are compared;
- migration and replacement boundaries are explicit;
- a research record recommends the choice;
- an ADR records meaningful lock-in and rejected alternatives.

## Gate D — claim a security property

A security or privacy property may be presented as implemented only when:

- it follows the claim format in the threat model;
- conformance and adversarial tests exercise the declared conditions;
- measurements or analysis are retained and reproducible;
- downgrade, recovery, update, and failure behavior are covered;
- documentation exposes the honest limitation to users and developers.

## Gate E — call a release usable

A release is usable only when complete journeys, not isolated primitives, pass
on supported platforms. For the first tracer this includes publish, resolve,
open, permission, asynchronous reply, update, recovery, Replica loss, and blocked
entry behavior.

## Repository promotion

When an experiment is promoted:

1. preserve its result and evidence in the research record;
2. design the production module from the accepted contract rather than copying
   the experiment directory wholesale;
3. delete or clearly archive obsolete experiment code;
4. add conformance, misuse, failure, and recovery tests with the first production
   slice;
5. update the functional map and decision state.
