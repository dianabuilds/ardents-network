---
status: accepted
date: 2026-08-08
---

# Adopt Go as the maintained project foundation

Ardents uses one root Go module for maintained project code. Executables are
thin adapters under `cmd`; cohesive implementation modules are internal;
disposable experiments remain evidence and cannot become a second project
tree. Go 1.26.x is the language family, while CI pins Go 1.26.6. Carrier Lab is
historical provenance, not a current toolchain owner.

Every change must pass executable architecture, format, vet, test, build, module
tidiness, race, Staticcheck, and vulnerability gates. Quality tools are pinned
and installed only by an explicit command. Runtime dependencies require a
recorded review. First-party cgo, `unsafe`, implicit initialization, speculative
interfaces, and generic package dumping grounds are prohibited unless a later
accepted ADR justifies a specific exception.

This selects the project foundation, not a route, transport, storage system,
cryptographic suite, public wire protocol, application runtime, or production
network claim. ADR-0008 still limits the current behavior slice to Carrier Lab.
Go may be reconsidered only with measured failure against an accepted contract
and a superseding ADR; an untested parallel implementation is not maintained.

## Lifecycle note — 2026-08-11

The final Carrier Lab sentence was temporal. Gate B and Gate C later completed,
so Go now also underpins whichever **single bounded Horizon 3 slice** is
explicitly promoted under the accepted product scope. This note changes neither
the language decision nor any still-open transport, storage, routing, wire,
cryptographic, or deployment-foundation choice.
