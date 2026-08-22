# Horizon 3 Stage 9 frozen product qualification and closure brief

Status: **replacement planning contract accepted by the Product Owner on
2026-08-22. Execution has not started.** This brief replaces both the original
Stage 9 cleanup plan and the coarse 2026-08-21 qualification brief. All planned
product, architecture, code, test, infrastructure, and documentation mutations
belong to Stage 8.

Authoritative inputs are:

- the Product Owner-accepted Stage 8 execution record and freeze disposition;
- the frozen product contract, threat model, applicable ADRs, limitations,
  journey/claim matrix, technical architecture, package/dependency maps,
  operations/reference documentation, and testing policy;
- the exact candidate, source, build, supply, toolchain, configuration, format,
  schema, fixture, Qualification/verifier, platform/stand, schedule, and active
  normative-document identities frozen at the Stage 8 exit; and
- applicable completed research and qualification protocols, including retained
  R-023 and R-037 obligations unless Stage 8 accepted a superseding decision.

## Purpose

Stage 9 proves one exact post-refactoring product candidate. It separates fast
deterministic regression, real platform/lifecycle behavior, integrated and
adversarial regression, long-running stability, and claim-level Qualification
so that a green result cannot conceal an unexecuted class of risk.

Stage 9 changes nothing about the candidate. It produces immutable raw evidence,
reproducible verdicts, honest limitations, and an explicit Product Owner Horizon
3 closure decision.

## Entry gate

Stage 9 starts only when:

1. Stage 8 has passed and the Product Owner has admitted one freeze proposal;
2. no planned product, Interface, Implementation, package, command, format,
   migration, dependency, test, infrastructure, or normative-document change
   remains;
3. the candidate builds from a clean checkout with no repository-local cache,
   generated evidence, secret, capture, database, mutable download, or hidden
   service supplying the result;
4. supported platforms, package formats, host capabilities, stand topology,
   resource allocation, clocks, fault schedule, observers, and external evidence
   roots are identified and available;
5. every required check/campaign has a frozen owner, input, duration, timeout,
   cleanup condition, result predicate, evidence schema, and failure effect;
6. unavailable external users or independent reviewers are recorded as honest
   unqualified gates rather than silently replaced with the Product Owner or
   another Codex pass; and
7. the complete ordered schedule and worst-case runtime are reviewable before
   the first attempt starts.

## Immutable-candidate rule

- Do not edit source, tests, build inputs, fixtures, configurations, protocols,
  acceptance predicates, or normative documents during an attempt.
- Do not update a dependency, container image, external binary, platform patch,
  stand topology, clock source, resource allocation, or verifier without ending
  the attempt or applying a predeclared environment-equivalence rule.
- Record every failure and invalid environment before diagnosis. A rerun is
  diagnostic evidence and never erases the original result.
- A candidate change ends Stage 9 and returns to the owning Stage 8 wave. The
  changed candidate receives a new freeze and a new affected qualification
  attempt.
- Evidence reuse is permitted only where the frozen impact model proves that
  changed identity cannot affect the claim; security/privacy, durable format,
  supply, platform, and integrated journey changes default to no reuse.
- Stage 9 may repair only external stand state without changing its frozen
  meaning. A stand repair is recorded, re-admitted, and reruns every affected
  observation.

## Evidence and result model

Every selected check produces `pass`, `fail`, or `invalid` for one candidate and
environment identity:

- `pass` means every conjunctive predicate was observed under its declared
  conditions;
- `fail` means candidate behavior violated a predicate;
- `invalid` means identity, environment, completeness, attribution, observer,
  cleanup, or evidence integrity could not support a verdict.

`invalid` is never pass. Missing platforms, tools, capabilities, actors, or raw
evidence do not reduce the denominator. There is no permanent quarantine, skip
allowlist, flake budget, or “pass after N retries.” Generated evidence remains
outside the repository; the repository retains only durable protocols, small
reviewed conclusions, digests/locations, and allowed limitations.

## S9.0 — Candidate, schedule, and stand admission

Before executing product checks:

1. record commit/tree, dirty-state refusal, reproducible build, binary and
   package digests, Go/tool/dependency/supply identities;
2. record all configuration, schema, format, fixture, verifier, active-document,
   supported-platform, host, topology, resource, clock, and fault identities;
3. verify the dedicated stand is non-overcommitted where the claim requires it
   and collectors observe the actual candidate process trees;
4. prove evidence roots are external, access-controlled, append/immutable as
   required, capacity-bounded, and mapped to retention/cleanup policy;
5. precompute the complete scenario matrix, ordering constraints, time budget,
   stop conditions, and required independent recomputation; and
6. reject admission on ambiguity, missing identity, mutable supply, hidden
   infrastructure, shared resources that invalidate attribution, or a changed
   normative contract.

Exit: one signed/reviewable admission manifest and zero executed result claimed
before admission.

## S9.1 — Clean deterministic regression

Run from a fresh checkout and clean external fixture roots:

- formatting, architecture/import/dependency/artifact/documentation consistency,
  build and static analysis gates;
- every Module behavior suite, including normal, negative, fault, restart,
  cancellation, deadline, resource, cleanup, corruption, and compatibility
  outcomes through the target Interface;
- deterministic Adapter-contract suites with admitted local stand-ins;
- command composition and bounded local process journeys;
- race profiles for every concurrency-owning Module on supported race-capable
  platforms;
- frozen fuzz/property corpus for every untrusted decoder, canonicalizer, and
  state transition, plus the scheduled bounded fuzz duration; and
- exact CLI/configuration/format reference and technical-document checks.

Selection manifests are explicit and positive. `-short`, path exclusions, or a
skipped external dependency cannot silently remove a required scenario. Any
selected failure, panic, race, hang, residue, missing cleanup observation, or
invalid environment stops the attempt before more expensive stages unless the
schedule explicitly requires collecting additional non-mutating diagnostics.

Exit: complete deterministic regression evidence for the frozen identity.

## S9.2 — Platform, installation, and lifecycle regression

On every supported platform and package/profile combination, prove the retained
real Adapters and full operator lifecycle:

- install/first start/readiness, normal start/stop/drain/restart, repair,
  upgrade, failed activation, rollback/forward repair, uninstall and purge;
- protected durable state, release floors, Namespace/Network/Publication state,
  Custody material, backup/restore/reconcile, permissions, path identity,
  locking, replacement/reparse resistance, and post-crash reopen;
- command/configuration versioning, supported automation, stable results and
  documented diagnostic/troubleshooting paths;
- Application peer identity, Grant enforcement/revocation, IPC, Isolation,
  child/process-tree lifetime, signal/job behavior, and escape/substitution
  negatives;
- external WebTunnel/Carrier/source/installer binaries with frozen provenance,
  failure mapping, cleanup, and platform parity;
- resource measurement, admission/refusal, pressure, process placement,
  oversubscription defense, and zero unintended residue; and
- exact operator runbook execution with every documented postcondition.

Unsupported capability is a typed documented product refusal, not a skipped
test for a claimed platform. Platform evidence records actual filesystem,
runtime, package, privilege, resource, process, and cleanup observations.

Exit: complete supported-platform lifecycle and real-Adapter regression.

## S9.3 — Integrated journey, recovery, and adversarial regression

Run the complete retained J00-J08 product journey matrix against real composed
Endpoint, Node, Application, naming, publication, connection, entry, route,
release/update, and custody responsibilities. Cover at minimum:

- clean bootstrap and authenticated current-state refresh;
- exact-name private resolution, publication, connection and opaque bidirectional
  Application Data;
- Service migration, generation cutover, concurrent acquire, revocation,
  unpublish/drain, crash and restart;
- Route/Entry replacement, blocked-entry behavior, stale/conflicting state,
  replay, unavailable candidate, capacity/refusal, and explicit no-fallback;
- contributor admission, quarantine, duty conflicts, resource pressure, drain,
  withdrawal and residue;
- update/rollback/repair without rollback of protected authority/freshness state;
- corruption, cancellation, deadline, child death, network impairment, partial
  durable transition, host restart, and repeated recovery; and
- role-local knowledge, diagnostics redaction, secret absence, and every
  applicable threat-model limitation/negative oracle.

Tests observe through product commands and Module Interfaces. A runner cannot
author its own success verdict from internal state. Every failure remains
classified, bounded, recoverable where promised, and leaves only explicitly
retained state.

Exit: complete integrated functional, failure, recovery, adversarial, and
cleanup regression for the same frozen identity.

## S9.4 — Sustained, soak, and stability campaigns

Run the predeclared longer campaigns after shorter regressions are green:

- repeated start/stop/restart/update/rollback/restore and network-state churn;
- sustained Application Data in both directions across sequential and
  concurrent connections, replacement attachments, and service generations;
- resource pressure, capacity/refusal hysteresis, slow peers, partial failures,
  timeouts, cancellation storms, child failures, and recovery cycles;
- unattended operation with scheduled fault injection and bounded operator
  intervention assumptions;
- long-running memory, goroutine, handle/descriptor, process, disk, queue,
  latency, throughput, error, reconnect, and retained-state trends; and
- final drain, shutdown, removal/cleanup and external residue inventory.

Duration and load come from accepted claim and resource budgets, not a generic
“run for a long time.” The schedule defines warm-up, steady-state, fault,
recovery and cooldown windows; sampling cadence; baselines; allowed envelopes;
and early-stop predicates before execution. A trend anomaly is a failure or
named investigation, never averaged away by the final sample.

Exit: source-bound sustained/soak evidence and a complete resource/residue time
series for the candidate.

## S9.5 — Deferred claim Qualification and independent recomputation

Execute every accepted claim-level protocol deliberately deferred to terminal
Qualification. This includes, unless Stage 8 accepted a superseding protocol:

- Stage 1 churn and unattended-operation obligations;
- remaining R-023 live, performance, and cross-platform evidence;
- Route, privacy-boundary, install/update/isolation/custody Qualification mapped
  by the Stage 8 claim matrix; and
- R-037 `h3-s5-b1-v1`: 564 candidate cells plus six five-episode evidence
  campaigns for reference, strong-capacity, and refusal cases; ten-minute
  observations in both directions; P0-P4; C0-C6; all nine hostile groups;
  recovery, repeated shutdown, and final cleanup.

The stand collector derives CPU allocation, cgroup or equivalent identity,
memory limits, network isolation, clocks, candidate process attribution, and
cleanup from actual runtime state. A separately built verifier rejects missing,
shared, out-of-range, unattributable, candidate-mismatched, or internally
inconsistent evidence.

Independent recomputation is required only for claims whose acceptance contract
needs an independent observer/verifier. Ordinary Module tests remain ordinary
tests and do not gain a duplicate verdict protocol. Where no actual independent
human/security reviewer is available, record the limitation; tool separation
does not manufacture organizational independence.

Exit: raw immutable claim evidence, reproducible verifier inputs/verdicts, and
an exception ledger with no waiver hidden as pass.

## S9.6 — Final integrated campaign and Horizon 3 handoff

Existing Horizon 3 references to S9.6 continue to mean this terminal campaign.
Against the unchanged candidate and admitted stand:

1. rerun the frozen final journey matrix and short adversarial sentinel set;
2. execute the terminal multi-day integrated/unattended schedule that combines
   the accepted sustained workload, churn, recovery, pressure, and cleanup
   predicates without changing earlier protocols;
3. verify clean install through final purge and protected retained-state rules;
4. recompute every required verdict from raw evidence and reconcile it with
   S9.1-S9.5 without discarding failures or invalid attempts;
5. produce the external Qualification report, source/supply/evidence digest
   index, known limitations, exception and cleanup ledgers; and
6. hand the immutable package to the Product Owner for `advance`, `return`, or
   `stop`.

S9.6 is not a place to add a missing scenario or repair a flaky test. A missing
predicate returns to Stage 8, produces a new freeze, and re-enters the applicable
Stage 9 scope.

## Attempt failure and resumption

For every `fail` or `invalid` result:

1. preserve candidate/environment identity, raw observation, failure category,
   cleanup result, and completed unaffected cells;
2. diagnose without modifying the admitted candidate;
3. if only external stand state violated its frozen setup, repair, re-admit, and
   rerun the affected and dependency-successor cells;
4. if source, test, protocol, fixture, configuration, documentation, dependency,
   supply, supported-platform claim, or acceptance logic changes, close the
   attempt and return to Stage 8; and
5. accept evidence reuse only through the predeclared impact graph.

Stage 9 has no “continue with known failure” disposition. The Product Owner may
narrow/redesign the product only back in Stage 8, followed by a new freeze.

## Required outputs

1. immutable candidate, build, supply, configuration, normative-document,
   schedule, stand, observer, verifier, and evidence identities;
2. complete S9.1 deterministic regression evidence;
3. complete S9.2 supported-platform and operator-lifecycle evidence;
4. complete S9.3 journey/failure/recovery/adversarial evidence;
5. complete S9.4 sustained/soak/resource/residue evidence;
6. complete S9.5 claim Qualification and required independent verdicts;
7. S9.6 terminal integrated evidence and reconciled result;
8. exception, invalid-attempt, known-limitation, cleanup, retained-state, and
   evidence-retention ledgers; and
9. the Product Owner's explicit Horizon 3 closure disposition.

## Pass, return, and stop

- **Pass:** every applicable frozen predicate is `pass` for the same candidate,
  required independent verdicts reproduce, cleanup/retained state are correct,
  limitations remain honest, and the Product Owner accepts the Horizon 3
  product baseline.
- **Return:** any candidate/normative input must change, a required predicate is
  missing or fails, evidence is invalid, or the product/claim must narrow;
  Stage 9 ends and work returns to the owning Stage 8 decision/wave.
- **Stop:** evidence invalidates the product direction, safe Qualification is
  infeasible within the actual one-Product-Owner-plus-Codex capacity, or a
  required external/independent gate cannot be obtained. Record the limitation
  instead of manufacturing a passing claim.
