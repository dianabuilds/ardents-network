# Development entry gates

The repository separates **research code**, **Reference Application code**, and
**production network code**. Passing time or accumulating code does not promote
one category into the next.

## Gate A — start an experiment

An experiment may be written when:

- it answers one named question from the network research queue;
- a falsifiable hypothesis and comparison criteria are written first;
- the network requirement, journey, and threat-model claim being tested are
  linked;
- inputs, environment, evidence, and cleanup behavior are defined;
- its directory is under `experiments/` and makes no compatibility promise.

An experiment may use any suitable language or maintained library. That choice
does not select the production stack.

## Gate B — start the Named Unlisted Site tracer

The Reference Application may gain a shared codebase only after review of the
minimum relevant contracts:

- accepted R-006 Service Target lifecycle;
- R-002 live Application Interface;
- R-001 Interactive Route adversary and claim;
- R-003 Service Name and recovery boundary;
- R-004 minimum Interactive Route evidence;
- R-007 connection and Service failure behavior;
- R-009 minimum authenticated bootstrap path;
- R-017 accepted Reference Application scope.

The tracer must use an ordinary local HTTP service over the generic Application
Interface. A browser adapter or simulated route may be part of the tracer, but
must be visibly marked. Replicated Site Bundles, offline delivery, and an
Ardents application runtime are not silently added to this gate.

## Gate C — select production language and foundations

A production language, runtime, routing foundation, storage component, or wire
format is selected only when:

- the same accepted network and tracer contract is evaluated fairly;
- security and maintenance evidence covers proposed dependencies;
- memory safety, concurrency, target-platform support, reproducible builds,
  fuzzing, interoperability, operability, and one-to-one project capacity are
  compared;
- migration and replacement boundaries are explicit;
- a research record recommends the choice;
- an ADR records meaningful lock-in and rejected alternatives.

## Gate D — claim a security property

A security or privacy property may be presented as implemented only when:

- it follows the claim format in the threat model;
- conformance and adversarial tests exercise the declared conditions;
- measurements or analysis are retained and reproducible;
- downgrade, route failure, key rotation, update, and recovery are covered;
- documentation exposes the honest limitation to Users and Developers.

## Gate E — call a network release usable

A release is usable only when complete journeys, not isolated primitives, pass
on supported platforms. For the first tracer this includes start/join, publish,
name, resolve, connect, exchange, close/fail, accepted rotation, alternate-path
recovery, and blocked-entry behavior.

An offline Service must be reported as unavailable. The release must not imply
that Application Data was retained, delivered, or semantically completed unless
a separate accepted Overlay contract provides and verifies that behavior.

## Repository promotion

When an experiment is promoted:

1. preserve its result and evidence in the research record;
2. design the production module from the accepted contract rather than copying
   the experiment directory wholesale;
3. delete or clearly archive obsolete experiment code;
4. add conformance, misuse, failure, and recovery tests with the first
   production slice;
5. update the functional map and decision state.
