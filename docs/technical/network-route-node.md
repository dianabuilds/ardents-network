# Network State, Entry, Route, and Node

Status: **current maintained technical contract.** This document describes the
implemented closed-test-network Modules and their current Interfaces. It does
not claim public network operation, independent operators, public discovery,
supported Node hosting, or Route qualification.

## Module ownership

| Module | Interface responsibility | Excluded responsibility |
|---|---|---|
| internal/network/state | Authenticate source input, verify Epoch/View material, publish one immutable current or pending View through its exclusive durable root, and supply narrow read-only views. | Source authority, public wire selection, Node lifecycle, Route selection, or private naming control. |
| internal/network/source | Obtain one finite selected Direct-Origin source input with its credential, TLS transport, material selector, ordering, and exposure identity. | Accepting State or selecting a peer protocol. |
| internal/network/duty | Persist the Endpoint-local Role Domain generation, watermark, expiry, and conflict truth. | Network State publication, assignment creation, Route ownership, or Node process lifecycle. |
| internal/resource | Measure selected Linux process limits and make the finite NORMAL, PROTECT, or DRAIN pressure decision. | Admission, listener shutdown, or a claim for unsupported platforms. |
| internal/entry | Import and admit a signed State-referenced Entry Invite, maintain its bounded durable replay/replacement set, and open an adjacent contact lifecycle. | Complete Route selection, carrier choice, or User identity. |
| internal/route | Select and hold one native Interactive Route attachment over authenticated State, Entry, and caller-owned resource facts. | H3 compatibility, peer runtime, Node profile, or durable State/Duty writing. |
| internal/node | Run one bounded Contributor duty from authenticated admission through listener readiness, pressure reaction, drain, withdrawal, and joined cleanup. | State-root authority, assignment creation, or a separate probe runtime. |

Each Module exposes one consumer-relevant Interface while retaining codec,
storage, replay, socket, and cleanup details privately. State readers receive
immutable snapshots only after durable publication. A source, clock, or
resource uncertainty prevents fresh State publication rather than creating a
fallback truth.

## Native Route profile

The selected Route profile is ardents-interactive-route-v1. EntryBinding binds
one signed Invite to a fresh User-to-Initiator TLS attempt key. Node-to-Node
LegBinding and SealedIntroduction have fixed binary records; State/publication
select supported generations, not a Node or peer value. The profile has no H3
reader, direct fallback, generic record map, or version-negotiation path.

Route owns volatile attachment selection and cleanup. It receives a
caller-owned resource reservation, verifies State-pinned TLS and Entry facts,
and returns an opaque attachment carrier to Service Connection. Service
Connection, not Route, decides whether an attachment must be replaced. Entry
may retain replay and adjacent-contact state but cannot construct a complete
Route from that state.

## Node and Resource lifecycle

Node consumes narrow authenticated State and Duty facts, then moves a local
role through admission, readiness, pressure protection or drain, withdrawal,
and terminal cleanup. The former standalone probe package is intentionally
private Node implementation; the command does not compose an independent
probe runtime.

Resource measurement is Linux-only until another native Adapter is selected
and measured. Unsupported platforms refuse rather than silently reporting
capacity. Resource has no authority over a consumer's lifecycle: Endpoint,
Node, and Route own their respective readiness, admission, drain, and shutdown
reaction.

## Current limits and limitations

The implemented system is a project-controlled Closed Test Network. A local
test or development-host process does not establish independently operated
capacity, anonymity, availability, censorship resistance, public deployment,
or a supported platform profile. Private source and Route bytes do not create
a public protocol promise.

The current direct-origin source and native Route code are selected technical
tracers. Any new source transport, peer announcement, public bootstrap,
directory, carrier fallback, or supported Node operating profile requires its
own decision, compatibility rule, and Qualification evidence.

## Verification and decisions

- Focused Network State, Duty, Resource, Entry, Route, and Node behavior tests
  cover durable reopen, corruption, replay, invitation replacement, attachment
  cancellation, pressure, listener drain, withdrawal, and cleanup.
- Process tests cover authenticated source-to-State and Node lifecycles; they
  do not qualify a public network or native host profile.
- One disposable mixed closed-network run completed Windows Initiator -> local
  Docker Introduction -> VPS Docker Rendezvous -> VPS-private Docker
  Responder. Every leg selected TLS 1.3 and the exact Route ALPN; the
  65,536-byte opaque payload had the same reported SHA-256
  `c3eb7cad...74076247` at Initiator and Responder, and all four roles exited
  successfully. Introduction was loopback-only; the temporary VPS exposure
  was Rendezvous only; containers, networks, images, tarball, keys, port, and
  isolated worktree were removed afterwards. This is functional integration
  evidence only: it does not establish State/Entry, Service Connection,
  Route topology, privacy, independent operation, public deployment, or a
  Node profile.
- [ADR-0024](../adr/0024-native-interactive-route-foundation.md),
  [ADR-0025](../adr/0025-state-referenced-entry-invites.md),
  [ADR-0026](../adr/0026-interactive-route-v1-wire.md), and
  [ADR-0027](../adr/0027-entry-binding-v1.md) define the selected native Route
  facts. R-092 remains the open measurement for a native Node operating
  profile.
