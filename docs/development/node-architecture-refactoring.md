# Node architecture refactoring plan

Status: **agreed refactoring direction; provisional package sequence**. The
[C0 component reconstruction](c0-component-reconstruction.md) defines the
cross-system ownership questions to settle before extracting Node packages.
This plan organizes maintained code. The product contract, technical Node
owner and GitHub issues remain authoritative for behavior and delivery status.
Baseline inspected: `codex/architecture-refactor` at `e1deba3e` (2026-09-25).

## Goal and completion condition

A reader should be able to locate Node admission, the selected duty, its
listener and admitted work, resource pressure, and terminal cleanup from the
package and file names. `internal/node` should expose the process lifecycle and
compose duty owners without retaining their transport sessions, recipient
state, or durable spend lifetimes. The maintained TCP/TLS and QUIC behavior,
State authority, wire and persisted identities, finite limits, and joined
cleanup remain the acceptance contract.

The work is complete when the selected duty has a clear owner and call path,
every extracted package has a small caller-facing API and non-test caller,
tests cover the moved behavior at its owner, the package map matches imports,
and the integrated installed Ubuntu journey works with both selected Carriers.

## Current shape and sources of friction

`internal/node` has 45 production and 68 test Go files in one package. The
forwarding-specific work spans more than ten production files, in addition to
shared outer-lane code. Its listener and link
files have 490 and 454 lines respectively. The following responsibilities are
present but not visible as package boundaries:

| Current location | Responsibility and coupling |
| --- | --- |
| `contract.go`, `admission.go`, `lifecycle.go`, `duty_server.go` | Public configuration and event contract, immutable duty facts, State admission, process lifecycle, and dispatch of five closed duties plus the private probe. `runtimeConfig` embeds the whole `Config`; duty implementations can read unrelated role settings. |
| `closed_route_receiver.go`, `closed_forwarding_admission.go`, `closed_hosting.go` | Current State/profile projection, exact recipient and token checks, and host reservation. Forwarding and direct recipients share these checks, so moving a role by filename would import the parent package or duplicate authority. |
| `closed_outer_lifetime.go`, `closed_outer_writer.go` | Outer Carrier lane lifetime and serialized writes shared by forwarding, issuer, resolution, Introduction, and JOIN. This is a genuine shared Node operation above Route's wire/Carrier mechanics. |
| `closed_forwarding_*`, `closed_bootstrap_forwarding.go`, `closed_forward_recipient.go`, `closed_carrier_relay.go` | Forwarding listener, receiving resources, sessions, links, queue, bootstrap, peer choice, and shutdown. `closedForwardingServer` retains the group and its terminal result. |
| `closed_issuer_*`, `closed_introduction_*`, `closed_resolution_*`, `closed_join_*` | Four distinct direct recipient duties and their own admission, work, and drain. |
| `probe_*`, `resource_pressure.go`, `event_writer*`, `local_roles.go`, `terminal_cleanup.go` | Private probe, process-wide pressure/evidence, local role retention, and terminal outcome. The shared running-duty handle is currently named `probeServer`. |

`internal/route` owns physical TCP/TLS and QUIC Carrier mechanics, protected
Route admission/wire, and shared outer-bridge primitives. Node owns the selected
listener, authenticated duty, admitted work, local session lifetime, pressure,
and terminal result. The refactoring keeps this direction of dependency.

## Target ownership

| Package | Owned interface | Internal work |
| --- | --- | --- |
| `internal/node` | `Run`, process `Config`/`Result`/`Event`, one selected-duty dispatch and running-duty handle. | State admission and process lifecycle, role retention, pressure response, evidence emission, and final cleanup ordering. |
| `internal/node/authority` | Current immutable Node duty projection and exact role/token checks consumed by Node duty owners. | Read the accepted State views; reject stale, conflicting, changed or mismatched authority. It never writes State or invents assignment material. The implementation must keep the caller-facing projection narrow enough that a duty cannot read unrelated fields. |
| `internal/node/outer` | Serve one accepted outer Carrier and join its inner lanes. | Serialized frame writes, deadlines, interruption, child join, and terminal transport cleanup. Route still owns its outer handshake, bridge, and codec. |
| `internal/node/forwarding` | Start one State-authorized forwarding duty and return its bounded running handle. | Listener, spend and host reservations, admitted links, outgoing Carrier sessions, queues, bootstrap and peer selection, drain, and retained cleanup failures. It consumes current authority through the shared authority owner. |
| `internal/node/probe` | Start the private role probe within the Node lifecycle. | Its listener, credentials, wire, admission, and drain; it cannot run as a second Node duty. |
| Direct recipient owners | Issuer, Introduction, Resolution, and Data Join each retain their own listener/admission/work/drain. | Extract an individual package when its state and lifecycle form a deep module with a small API. A thin Route adapter stays as a clearly named file in `internal/node` if a package would add only forwarding methods. |

These are target responsibilities, not permission to create placeholder
directories. `authority`, `outer`, `forwarding`, and `probe` are created only
with their implementation, `doc.go`, behavior tests, non-test callers, and
package-map entries. Exact exported names are chosen from the first real caller.

## Execution order

### 0. Preserve the baseline and coordinate intake

- Work in the architecture worktree. Record HEAD and existing unstaged/staged
  Endpoint changes before each Node slice; preserve them.
- Compare the active network task's completed commits and touched Node files
  before starting a slice. Incorporate completed network fixes into the
  architecture branch at a coherent checkpoint, then resolve overlap once.
- Capture the current package list, caller graph, selected Node tests, and any
  known failing checks. A known failing unrelated C0 scenario is tracked as
  such; it is not repeatedly rerun to validate a local file move.

### 1. Make the root contract readable

- Split `contract.go` by responsibility into named files for process config,
  authenticated duty view/facts, and event/result contract. Keep the same Go
  package and behavior during this step.
- Rename the common `probeServer` handle to describe a running duty. Align
  `startDuty`, `Run`, and each adapter with that name. Keep the private probe's
  own types under probe names.
- Give `internal/node/doc.go` a concise navigation map: admission, authority,
  outer lifetime, each duty, pressure, and terminal cleanup. Avoid duplicating
  the technical contract.
- Acceptance: the five duty dispatch paths and probe still compile; focused
  lifecycle and dispatch tests pass; a reader can find the common API without
  opening duty implementations.

### 2. Extract shared outer lifetime

- Move `serveClosedOuter` and its writer into `internal/node/outer`; expose one
  operation for an accepted outer handshake and inner-lane callback. Keep the
  close/interruption/children-join order and deadline behavior together.
- Switch its five production callers (forwarding and four direct recipients)
  in the same slice. Move owner behavior tests with the implementation and
  retain cross-duty integration tests at Node.
- Acceptance: no parent-package import, no duplicated writer/bridge logic,
  and tested cancellation, queued write, deadline, and joined cleanup.

### 3. Give current authority one owner

- Extract Node-local current duty facts, accepted closed-profile/recipient
  projection, and shared token verification behind `internal/node/authority`.
  This package consumes read-only State views; process admission and the choice
  of local duty remain in `internal/node`.
- Define the exact data that each duty needs. A forwarding caller receives
  current receiver/peer/token decisions, not the whole process `Config` or
  mutable State runtime. Issuer and control recipients consume the same exact
  verification rules without copying them.
- Acceptance: successor State or profile loss makes every affected duty
  unavailable at its existing recheck points; no role can substitute a plan
  value for State authority; import direction is `node` and role packages to
  `authority`, never back to `node`.

### 4. Extract forwarding as one bounded owner

- Move the forwarding listener, receiving-resource group, admission, sessions,
  links, queue, bootstrap/peer logic, Carrier relay choice, and shutdown into
  `internal/node/forwarding`. Rename files to their responsibility inside the
  package; the package name replaces the repeated `closed_forwarding_` prefix.
- `node` supplies local roots/certificate/limits and the current-authority
  reader, then retains only the returned running handle. Forwarding owns its
  spend root, pool, host reservations, accepted producers, outgoing readers,
  their join order, and terminal cleanup result.
- Keep one explicit startup resource owner: failed construction closes only
  resources actually acquired; after successful start the running duty owns
  them until drain. Preserve the bounded drain result on repeated calls.
- Acceptance: forwarding has no `node` import or reach into `runtimeConfig`;
  existing admission, bootstrap, parent/child, relay, limit, and shutdown tests
  follow their owner; focused TCP/TLS and QUIC paths remain green.

### 5. Place the remaining duties and probe

- Move the private probe into `internal/node/probe` with its credentials, wire,
  listener and tests. It remains started and stopped by `node.Run`.
- For each direct recipient in this order—Introduction, Resolution, Data Join,
  Issuer—move its independent state transitions and cleanup together. Extract
  a role package only when it owns meaningful behavior and can take current
  authority without a parent import. Otherwise retain a short, purpose-named
  root adapter around Route's deep module. Record the decision in the Node
  navigation map rather than introducing a shallow package.
- Before the Issuer move, give the accepted token listener one owner for its
  issuer key root and replay ledger through worker join, including a `Drain`
  timeout. The current Node adapter closes those roots only when its bounded
  listener drain succeeds, whereas the other receiver servers continue their
  own close attempts after a caller timeout. Preserve Node's failed terminal
  outcome when cleanup is unproven and cover eventual release and close error
  propagation in the focused owner tests.
- After each move, remove only prefixes made redundant by the new package or
  role owner. Preserve names that encode a real protocol or compatibility
  distinction.
- Acceptance: every role's start, admitted work, and drain can be found from
  the root dispatch and its owner; no shared State/outer/hosting logic is
  copied into a role.

### 6. Integrate and verify the result

- Update `docs/development/package-map.md` with exact permitted imports in
  the same changes that create packages. Update the Node section of the
  technical owner when the real ownership changes; keep this plan separate
  from the issue ledger.
- For each coherent slice, run focused owner tests and the required
  `make quick-check` once at the commit boundary. Use targeted Linux and
  Windows compilation when platform-specific code moves. Preserve failures
  and fix the affected owner before advancing.
- After intake of completed network changes, run `make check` and the
  installed Ubuntu C0 scenario with TCP/TLS and QUIC on the combined tree.
  Distinguish a locally verified refactor from final integration and C0
  acceptance. Integrate only the reviewed, passing combined result.

## Review points and stopping rules

The first reviewable result is the root contract/navigation cleanup. The next
is the shared outer owner. The first major structural result is forwarding in
its own package with the same admission and cleanup behavior. Each result
must be coherent and usable before another package move starts.

If a proposed package needs most of `runtimeConfig`, imports its parent, or
duplicates current State checks, repair the authority seam first. If a direct
role only delegates to Route, keep its adapter in Node. If a behavior check
fails, fix that specific owner before broadening the refactor. Changes to
wire/persisted identities, authority, protection policy, or product behavior
require their own accepted contract rather than being folded into this plan.
