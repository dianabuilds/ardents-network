---
id: R-014
title: Which language and runtime candidates fit Ardents?
status: decided
owner: product research
started: 2026-08-08
reviewed: 2026-08-08
---

# R-014 — Language and runtime candidates

## Decision this unlocks

Choose the maintained project language and runtime foundation without selecting
any route, transport, storage, wire format, or later product horizon.

## Decision

- **Maintained project foundation:** Go 1.26.x, standard-library-first, in one
  root module; CI and Carrier Lab pin Go 1.26.5.
- **Project structure:** thin commands in `cmd`, cohesive deep modules in
  `internal`, bootstrap tooling in `scripts`, and no maintained Go code in
  `experiments`.
- **Quality contract:** executable architecture rules plus format, vet, test,
  build, module, race, Staticcheck, and vulnerability gates.
- **Reconsideration trigger:** replace Go only when measured evidence shows a
  hard accepted resource, safety, platform, or maintenance gate cannot be met
  and a bounded challenger can meet it within one-owner capacity.
- **No mixed core:** Ardents does not maintain a Go/Rust FFI split. A separately
  versioned external C Tor process is an Adapter, not part of the production core
  language decision.

No parallel full prototype is authorized. C#/.NET remains technically credible but
provides no unique benefit over this shortlist. C, C++, Zig, JVM, JavaScript,
and Python do not enter the network-core comparison. Python, shell, or
PowerShell may orchestrate disposable evidence tools only.

## Evaluation contract

The language choice is judged on the complete stack needed by the accepted
tracer, not on syntax preference or an isolated microbenchmark:

1. memory safety and a reviewable unsafe/native boundary;
2. finite queues, task/goroutine lifetimes, cancellation, deadlines, and
   backpressure;
3. async networking, TLS, cryptographic integration, and parser safety;
4. Ubuntu and Windows `x86-64` support;
5. deterministic dependency resolution, offline/repeatable builds, SBOM and
   vulnerability evidence;
6. fuzzing, race/concurrency testing, diagnostics, profiling, and failure
   evidence;
7. one-to-one project capacity: implementation time, cognitive load, build time,
   dependency review, release work, and long-term repair;
8. ability to keep the Route Module deep while changing an Adapter without
   exposing runtime details through its Interface.

A language passes only through a complete candidate built with its actual
runtime and crypto libraries. Memory safety alone does not prove bounded memory,
deadlock freedom, correct cancellation, protocol authenticity, anonymity, or
availability.

## Evidence classification

- **Sourced facts:** compatibility, module authentication, fuzzing, race and
  vulnerability tooling for Go, plus target support, locked/offline builds,
  vendoring, Tokio channels, and audit tooling for Rust, come from the official
  sources linked below. All external sources were accessed on 2026-08-09.
- **Measurements:** no bounded Go/Rust Route comparison has run. This record
  contains no comparative latency, CPU, RSS, build-time, or defect-rate result.
- **Assumptions:** one Product Owner plus Codex owns implementation and repair;
  standard-library-first Go and cgo-free builds reduce the maintained surface;
  a second runtime is justified only by a measured Go-specific failure.
- **Recommendations and decisions:** use Go as the maintained foundation under
  ADR-0009, retain Rust as a conditional challenger, and run the fixed
  comparison only after a viable Route gives both candidates a real subject.

## Candidate A — Go

### Candidate stack

- Go 1.26.5 for Carrier Lab; a production comparison pins the current reviewed
  stable toolchain in its own pre-run manifest;
- standard library first: `net`, `io`, `context`, `sync`, `crypto/tls`,
  `crypto/x509`, `crypto/rand`, `crypto/hpke`, `encoding/binary`,
  `encoding/json`, `log/slog`, pprof, and `runtime/metrics`;
- `go.mod` and `go.sum`, with module graph and checksums retained;
- built-in `go test`, coverage, fuzzing, race detector, `vet`, and
  `govulncheck`;
- release-style candidate built with `CGO_ENABLED=0` and no first-party
  `unsafe`; the separate race-test build may use its required instrumented
  toolchain;
- third-party packages admitted only after maintenance, license, advisory,
  transitive native code, replacement cost, and misuse review.

Go supplies authenticated module verification, built-in fuzzing and race
detection, strong cross-platform tooling, and a compatibility promise. Primary
sources (accessed 2026-08-09):

- [Go compatibility](https://go.dev/doc/go1compat);
- [module authentication](https://go.dev/ref/mod#authenticating);
- [fuzzing](https://go.dev/doc/security/fuzz/);
- [race detector](https://go.dev/doc/articles/race_detector);
- [vulnerability management](https://go.dev/doc/security/vuln/).

### Why it leads

- the language and standard library are small enough for one owner to inspect;
- goroutines, `context`, sockets, TLS, profiling, and deployment have a direct
  operational model;
- a cgo-free binary reduces native build and packaging surface;
- the accepted throughput and latency goals do not inherently require manual
  memory management or zero GC;
- existing project experience is reusable without reusing the old architecture.

The old code failed as a product decomposition, not because Go made the product
impossible. Reusing Go does not authorize copying old modules or contracts.

### Go failure conditions

Go loses leading status when any of the following occurs under the same
candidate contract:

- GC behavior or retained allocations miss a hard RSS/CPU/latency budget after
  bounded profiling and one redesign;
- slow readers, cancellation storms, partial writes, or nested channels produce
  goroutine, timer, socket, handle, or memory growth after quiescence;
- the Implementation cannot enforce byte-bounded queues without silent loss,
  deadlock, or unreviewable scheduling behavior;
- parser input controls allocation above the declared frame or connection cap;
- the required maintained stack introduces cgo, first-party `unsafe`, an
  unacceptable native dependency, or unresolved high/critical vulnerability;
- offline/repeatable builds and complete dependency evidence cannot be produced;
- the Rust candidate passes the same contract while Go does not.

## Candidate B — Rust with Tokio

### Candidate stack

- pinned stable Rust, edition 2024, with Windows and Linux Tier-1 targets;
- Tokio, rustls/tokio-rustls, `bytes`, `tokio-util`, `tracing`, `serde`, and
  `serde_json` for the bounded comparison;
- `#![forbid(unsafe_code)]` in first-party crates;
- an inventory and review disposition for every transitive crate containing
  `unsafe`, a build script, proc macro, or native dependency;
- `Cargo.lock`, vendoring/offline build evidence, RustSec, cargo-deny, and
  cargo-vet;
- unit/integration tests, proptest, cargo-fuzz/libFuzzer, Loom for selected
  concurrency state machines, and sanitizer/Miri runs where applicable.

Rust supports both required target families as Tier 1, and Cargo supports
locked, offline, and vendored dependency resolution. Primary sources (accessed
2026-08-09):

- [Rust platform support](https://doc.rust-lang.org/rustc/platform-support.html);
- [Cargo locked/offline](https://doc.rust-lang.org/cargo/commands/cargo-build.html);
- [Cargo vendor](https://doc.rust-lang.org/cargo/commands/cargo-vendor.html);
- [Tokio bounded channels](https://tokio.rs/tokio/tutorial/channels);
- [RustSec](https://rustsec.org/) and
  [cargo-vet](https://mozilla.github.io/cargo-vet/).

### Why it remains serious

- safe Rust provides the strongest compile-time memory and ownership boundary in
  the shortlist;
- absence of a tracing GC can make RSS and tail behavior more predictable;
- Tokio, rustls, Quinn, Arti, and rust-libp2p provide a broad networking
  ecosystem if a later selected Adapter genuinely needs one of them;
- ownership can localize buffer, connection, and state-machine lifetimes.

### Rust failure conditions

Rust is rejected for the first production core when:

- the bounded comparison cannot be completed inside its timebox by the same
  one-to-one team;
- Tokio cancellation, pinning, trait/generic structure, or lifetime plumbing
  makes the Route Module Interface harder to understand than its behavior;
- the dependency feature graph, compile time, MSRV policy, build scripts, or
  transitive `unsafe` inventory is too large for continuous review;
- safe Rust still misses the same resource, lifecycle, protocol, or evidence
  gates;
- its measurable benefit is smaller than the additional implementation and
  maintenance cost after both candidates already pass.

## Conditional bounded comparison

Do not implement the complete C-5 laboratory twice. If the Go Carrier Lab
retains a viable Route, extract one protocol-compatible critical slice and give
each language at most `24` focused engineering hours for candidate-specific
work. Existing Go Carrier Lab code may be adapted, but both results use the same
manifest, tracer data, limits, and verdict calculator.

The slice includes only:

1. the same bounded frame parser and state-transition corpus;
2. one nested authenticated relay path carrying the exact canary and a
   60-second incompressible stream in both directions;
3. the same `256 KiB` byte-queue cap and a slow-reader backpressure case;
4. `10,000` open/cancel/close lifecycles followed by quiescence checks for
   task/goroutine, timer, socket, handle, allocation, and RSS growth;
5. one hour of coverage-guided parser/state fuzzing with the same seed corpus and
   maximum input;
6. repeatable locked offline builds, binary/import inventory, dependency graph,
   license report, and vulnerability report;
7. source-level review of cancellation, ownership, error propagation, and the
   resulting Route Module Interface.

This comparison cannot earn Route Qualification or substitute for the full
candidate matrix. It exists only to decide which implementation foundation is
safer to own.

## Selection rule

The Product Owner promoted Go from the experiment hypothesis to the maintained
project foundation on 2026-08-08 because a coherent project and preventive code
quality rules are required before further feature work. ADR-0009 records the
lock-in and replacement rule. A Route failure stops or redesigns that Route; it
does not dissolve the project structure or automatically authorize another
language. A challenger is considered only after a measured Go-specific hard
failure, using the bounded comparison above and a superseding ADR.

## Alternatives not shortlisted

| Alternative | Disposition |
|---|---|
| C / C++ | Reject for the first core: unnecessary memory-corruption and native supply-chain surface for hostile protocol input. External mature tools may remain process-isolated Adapters. |
| Zig | Reject for now: no unique maintained anonymity/networking stack and a less mature ecosystem than the two finalists. |
| Java/Kotlin | Reject for the core: importing I2P does not justify inheriting a second large runtime and ecosystem; its GC/runtime advantage over Go is not established. |
| C#/.NET | Reserve only: credible async, IPC, diagnostics, and Native AOT, but no unique Route dependency or ownership advantage warrants a third prototype. |
| JavaScript/TypeScript/Node | Reject for the network core: runtime, resource, parser, packaging, and native crypto dependency costs do not improve the accepted contract. |
| Python | Harness/analysis only; reject for the long-running route data path. |
| Go plus Rust FFI | Reject: two dependency graphs, two build systems, unsafe FFI ownership, cross-runtime diagnostics, and duplicated release work exceed one-to-one capacity. |

## Disposition

- State: `decided`; ADR-0009 selects Go and the one-module project structure.
- The decision creates no Route, compatibility, privacy, anonymity, or public
  network claim.
- Rust is a conditional challenger after a measured Go-specific hard failure,
  not a parallel implementation obligation.
- Runtime dependencies and all protocol-bound foundations remain separately
  gated and unselected.
