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
| `cmd/ardents-route` | `package main` | Load one role-local Route sequence, run its actors, and render bounded carrier or raw-attachment observations. | `internal/route`, `internal/routeplan`, standard library |
| `cmd/ardents-service` | `package main` | Adapt one role-local Endpoint process invocation to bounded JSON readiness and terminal output. | `internal/serviceendpoint`, standard library |
| `cmd/ardents-stream-app` | `package main` | Act as one external opaque-stream tracer Application using only its supplied local byte stream and classified Connection Result. | `internal/applicationipc`, `internal/streamworkload`, standard library |
| `cmd/ardents-publish-app` | `package main` | Act as one separately granted external publication operator using only its supplied Service Administration socket. | standard library |
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
| `internal/route` | `package route` | Select one complete four-position Route from authenticated Network State, perform bounded mutually authenticated sealed Introduction setup, and carry a bounded canary, caller-owned stream, or endpoint-secured raw Route Attachment through role-local actors with separate setup and active lifetimes, without receiving Service Instance material. | `internal/network/state`, standard library |
| `internal/routeplan` | `package routeplan` | Strictly load, validate, and construct one bounded role-local sequence of Route actors without widening role knowledge. | `internal/network/state`, `internal/planfile`, `internal/route`, standard library |
| `internal/serviceconn` | `package serviceconn` | Own bounded local admission, current Service publication, exact Target/Instance authentication, endpoint TLS, immutable recovery binding, volatile continuity, ordered/acknowledged Application bytes, finite replay state, Route Attachment cutover, and one honest terminal Service Connection result. | standard library |
| `internal/serviceendpoint` | `package serviceendpoint` | Own role-local Endpoint process composition, strict bounded plan loading, scoped Application/administration IPC, reusable Route Attachment acceptance, and resource-observed cleanup behind `Run`. | `internal/applicationipc`, `internal/planfile`, `internal/serviceconn`, standard library |
| `internal/streamworkload` | `package streamworkload` | Generate, stream, pace, and validate one deterministic bounded opaque workload for external Application tracers without payload-sized allocation. | standard library |
| `internal/node` | `package node` | Own one local Node identity's assignment admission, readiness, duty, drain, withdrawal, and terminal cleanup from narrow authenticated facts. | `internal/node/probe`, `internal/resource`, standard library |
| `internal/node/probe` | `package probe` | Own the authenticated bounded role-probe listener, TLS trust, wire framing, replay rejection, admission pressure, and connection cleanup. | standard library |
| `internal/planfile` | `package planfile` | Own bounded operator-plan and credential reads, strict JSON decoding, exact hexadecimal fields, and TLS key-pair loading. | standard library |
| `internal/resource` | `package resource` | Own shared OS/runtime measurement, fixed H3 placement, hysteresis, and the finite NORMAL/PROTECT/DRAIN pressure decision behind `Check` and `Observe`; consumers own readiness, admission, drain, and shutdown reactions. | standard library |
| `internal/architecture` | `package architecture` | Test repository structure, names, dependencies, formatting, and quality wiring. | standard library |
| `internal/applicationipc` | `package applicationipc` | Encode and decode bounded local Application Interface control and Route-opaque Connection Result framing. | standard library |
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
