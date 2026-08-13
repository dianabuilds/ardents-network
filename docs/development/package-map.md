# Package map

This is the factual naming and dependency registry for maintained Go code. It
lists packages that exist, not future intentions. A new entry is an architecture
change: name the responsibility first, add the real Implementation and tests,
then update this table in the same change. The normative growth and dependency
rules are in [repository-layout.md](repository-layout.md).

Every package below `internal/lab/` is laboratory code. The path prefix is the
executable classification: product packages and commands cannot import it.
Only purpose-named `cmd/*-lab` adapters may start laboratory Modules.
`internal/lab` itself contains no Go files and is not a package.

| Directory | Go declaration | Responsibility | May import |
|---|---|---|---|
| `cmd/ardents` | `package main` | Parse bounded offline or Direct-Origin Source inputs, call Network State, and render the accepted generation. | `internal/network/state`, `internal/network/source`, `internal/planfile`, standard library |
| `cmd/ardents-node` | `package main` | Parse one bounded source or Node configuration and run one private distributor or separately keyed Node process. | `internal/network/state`, `internal/network/source`, `internal/node`, `internal/node/probe`, `internal/planfile`, standard library |
| `cmd/ardents-qualify` | `package main` | Start bounded black-box qualification and render its terminal machine result. | `internal/planfile`, `internal/qualification/state`, `internal/qualification/node`, `internal/qualification/node/fixture`, standard library |
| `cmd/ardents-route` | `package main` | Parse one role-local H3 Stage 2 tracer plan, obtain authenticated Network State for Client selection, run one Route actor, and render bounded evidence. | `internal/network/state`, `internal/route`, standard library |
| `cmd/ardents-route-qualify` | `package main` | Read one bounded Stage 2 manifest/evidence bundle, call the independent Route verifier, and render its terminal machine result. | `internal/qualification/route`, standard library |
| `cmd/carrier-lab` | `package main` | Parse fixed Carrier Lab commands, call the selected Module, and translate results to exit codes. | `internal/lab/directcontrol`, `internal/lab/carrier`, `internal/lab/tooling`, `internal/lab/nativecircuit`, `internal/lab/preflight`, `internal/lab/routecomparison`, standard library |
| `cmd/named-site-lab` | `package main` | Parse fixed Gate C commands, derive one experiment identity, call `namedsite`, and translate its result to an exit code. | `internal/lab/runlayout`, `internal/lab/namedsite`, standard library |
| `internal/lab/carrier` | `package carrier` | Own the fixed two-role Carrier Lab isolation scenario: role configuration, Compose lifecycle, bounded observations, fault injection, verdict, and cleanup. | `internal/lab/preflight`, standard library |
| `internal/lab/tooling` | `package tooling` | Verify the exact external tool supply and runnable image pair; own shaping, capture, tracer, fail-closed smoke, and evidence behind `VerifyInputs`, `VerifyNativeImages`, `RunSmoke`, `RunRole`, and `RunNativeRole`. | `internal/lab/sourceidentity`, `internal/lab/preflight`, standard library |
| `internal/lab/preflight` | `package preflight` | Orchestrate pinned Carrier Lab setup and write canonical preflight evidence and cleanup verdicts for a verified experiment run. | `internal/lab/sourceidentity`, `internal/lab/runlayout`, standard library |
| `internal/lab/runlayout` | `package runlayout` | Own and revalidate the filesystem identity and derived paths of one maintained laboratory run. | standard library |
| `internal/lab/sourceidentity` | `package sourceidentity` | Bind all maintained experiment code, tests, workflows, and container inputs into one shared source SHA-256. | standard library |
| `internal/lab/namedsite` | `package namedsite` | Own the bounded Gate C Named Unlisted Site scenario, role processes, security decisions, evidence, and cleanup. | `internal/lab/sourceidentity`, `internal/lab/runlayout`, `internal/lab/nativecircuit`, standard library; reviewed OHTTP closure |
| `internal/lab/directcontrol` | `package directcontrol` | Own the complete lab-only Direct TLS fixture, control lifecycle, roles, protected-record fault, evidence, and cleanup behind `RunControl`, `RunRole`, and `RunTamper`; never act as a Route or fallback. | `internal/lab/preflight`, standard library |
| `internal/lab/nativecircuit` | `package nativecircuit` | Own the fixed lab-only native C-5/C2 candidate: bounded wire protocol, HPKE Introduction, telescoped Node TLS, joined endpoint TLS, opaque UDS attachment, role-local runtime, Compose lifecycle, evidence, and cleanup behind its small run/role interface. | `internal/lab/runlayout`, `internal/lab/tooling`, `internal/lab/preflight`, standard library |
| `internal/lab/routecomparison` | `package routecomparison` | Own the frozen R-013 comparative sequence, immutable workload/seed manifest, coarse statistics, conjunctive C-5 verdict, retained evidence, and cleanup behind `Run`. | `internal/lab/sourceidentity`, `internal/lab/nativecircuit`, `internal/lab/preflight`, standard library |
| `internal/network/epoch` | `package epoch` | Verify bounded Network Epoch and Candidate View semantics, materializations, Merkle proofs, and deterministic assignments. | `internal/network/epoch/assignment`, `internal/network/epoch/merkle`, `internal/network/framing`, standard library |
| `internal/network/epoch/assignment` | `package assignment` | Own deterministic Epoch role-domain selection. | standard library |
| `internal/network/epoch/merkle` | `package merkle` | Own canonical Epoch Merkle commitments and inclusion proofs. | standard library |
| `internal/network/framing` | `package framing` | Read bounded canonical Network binary values through one cursor implementation. | standard library |
| `internal/network/source` | `package source` | Own the finite Direct-Origin Source plan, credential binding, private TLS transport, material selector, ordering, and exposure identity. | standard library |
| `internal/network/store` | `package store` | Own the exclusive state root, bounded durable files, immutable generations, control journal, and atomic pointers. | standard library |
| `internal/network/state` | `package state` | Orchestrate authenticated Network State acceptance, current and pending decisions, finite acquisition, clock confidence, and durable publication. Product tests own their canonical vector builders. | `internal/network/epoch`, `internal/network/epoch/assignment`, `internal/network/epoch/merkle`, `internal/network/framing`, `internal/network/source`, `internal/network/store`, `internal/resource`, standard library |
| `internal/route` | `package route` | Select one complete four-position Route from authenticated Network State and run one bounded role-local Stage 2 canary actor over a replaceable laboratory carrier. | `internal/network/state`, standard library |
| `internal/node` | `package node` | Own one local Node identity's assignment admission, readiness, duty, drain, withdrawal, and terminal cleanup from narrow authenticated facts. | `internal/node/probe`, `internal/resource`, standard library |
| `internal/node/probe` | `package probe` | Own the authenticated bounded role-probe listener, TLS trust, wire framing, replay rejection, admission pressure, and connection cleanup. | standard library |
| `internal/planfile` | `package planfile` | Own bounded operator-plan and credential reads, strict JSON decoding, exact hexadecimal fields, and TLS key-pair loading. | standard library |
| `internal/qualification/byteio` | `package byteio` | Enforce bounded file reads, directory enumeration, and child-process output capture for qualification adapters. | standard library |
| `internal/qualification/epochfixture` | `package epochfixture` | Build bounded canonical Epoch, Node Record, and materialization qualification vectors through one fixture contract. | `internal/network/epoch`, `internal/network/epoch/assignment`, `internal/network/epoch/merkle`, standard library |
| `internal/qualification/state` | `package state` | Independently read and recompute bounded persisted Network State framing, digests, signatures, Epoch chain, Candidate View, rejections, summaries, assignments, and materialization proofs, then return the terminal verdict. | `internal/qualification/byteio`, standard library |
| `internal/qualification/node` | `package node` | Run separate bounded short, two-hour churn, and twenty-four-hour unattended black-box Node Docker campaigns; bind source/image/binary identity; own hostile workloads, Linux fault injection, external observations, evidence, and terminal verdict. | `internal/qualification/byteio`, `internal/qualification/node/fixture`, `golang.org/x/sys/unix`, standard library |
| `internal/qualification/node/fixture` | `package fixture` | Create one immutable bounded Node qualification fixture with a sealed manifest and optionally seed its initial persisted Network State. | `internal/network/epoch/assignment`, `internal/qualification/byteio`, `internal/qualification/epochfixture`, standard library |
| `internal/qualification/route` | `package route` | Independently verify bounded Stage 2 manifest/evidence integrity, process separation, Route selection, role-local knowledge, canary agreement, cleanup inputs, and terminal pass/fail/invalid. | `internal/route` in compatibility tests only; standard library in production |
| `internal/resource` | `package resource` | Own shared OS/runtime measurement, fixed H3 placement, hysteresis, and the finite NORMAL/PROTECT/DRAIN pressure decision behind `Check` and `Observe`; consumers own readiness, admission, drain, and shutdown reactions. | standard library |
| `internal/architecture` | `package architecture` | Test repository structure, names, dependencies, formatting, and quality wiring. | standard library |

## Carrier Lab command registry

| Command | Owning Module | Fixed responsibility |
|---|---|---|
| `bootstrap` | `internal/lab/preflight` | Orchestrate pinned host/container setup while reserving canonical evidence and verdicts for the pinned verifier. |
| `evaluate` | `internal/lab/preflight` | Verify prepared pinned inputs and write intermediate preflight evidence. |
| `finalize-cleanup` | `internal/lab/preflight` | Verify owned-resource removal and publish the final preflight verdict. |
| `compose-smoke` | `internal/lab/carrier` | Run the fixed two-role isolated scenario and own its lifecycle and summary. |
| `smoke-role` | `internal/lab/carrier` | Run one data-only role whose config names only its allowed peer. |
| `tooling-verify` | `internal/lab/tooling` | Verify the exact external `.deb` set against the committed tool lock. |
| `tooling-smoke` | `internal/lab/tooling` | Bind the runnable image to base/lock/source/binary identity, then own real shaping/capture, exact topology evidence, fail-closed verdict, and cleanup. |
| `tooling-role` | `internal/lab/tooling` | Run one fixed synthetic tracer, shaper, or capture role. |
| `direct-control` | `internal/lab/directcontrol` | Generate one ephemeral Target/Instance fixture and run the positive and fixed Direct TLS negative cases. |
| `direct-role` | `internal/lab/directcontrol` | Run one User or Service tracer role for the fixed Direct TLS control. |
| `direct-tamper` | `internal/lab/directcontrol` | Modify one TLS-protected record without receiving endpoint or Application knowledge. |
| `native-run` | `internal/lab/nativecircuit` | Run the fixed isolated native C-5/C2 development smoke and retain its bounded verdict. |
| `native-role` | `internal/lab/nativecircuit` | Run one role-local native User, Service, relay, Rendezvous, or Introduction process. |
| `native-tool-role` | `internal/lab/tooling` | Apply real link shaping or capture for one native role namespace with one exact capability. |
| `native-negative` | `internal/lab/nativecircuit` | Execute one fixed fail-closed R-013 negative inside the immutable application image. |
| `route-experiment` | `internal/lab/routecomparison` | Run the frozen Direct/C-3/C-5 comparison, required negatives, conditional Tor reference, and canonical verdict without building or downloading. |

The architecture gate parses this table and rejects an unregistered or stale
package, a mismatched Go declaration, or a current project import absent from
the owning row's `May import` column.
