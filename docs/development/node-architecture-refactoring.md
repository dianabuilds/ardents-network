# Node architecture refactoring plan

Status: **agreed refactoring direction; package candidates under source review**. The
[C0 component reconstruction](c0-component-reconstruction.md) defines the
cross-system ownership questions to settle before extracting Node packages.
This plan organizes maintained code. The product contract, technical Node
owner and GitHub issues remain authoritative for behavior and delivery status.
Initial baseline inspected: `codex/architecture-refactor` at `e1deba3e`
(2026-09-25). The cross-system [reconstruction](c0-component-reconstruction.md)
and [behavior map](repository-behavior-map.md) reconcile the maintained tree
to `53f02e64`; their later source findings control the candidate package
sequence below.

## Goal and completion condition

A reader should be able to locate Node admission, the selected duty, its
listener and admitted work, resource pressure, and terminal cleanup from the
package and file names. `internal/node` composes process lifecycle and duty
owners; a duty may continue to retain its transport sessions, recipient state
and durable spend lifetime inside that package until a smaller acyclic
boundary is proved. The maintained TCP/TLS and QUIC behavior,
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
| Current authority projection (package undecided) | Exact current Node duty, profile and recipient checks consumed by duty owners. | State authenticates the view; Node applies local duty checks. An `authority` package needs an actual narrow value/caller seam and must not copy or select State. |
| Shared outer lifetime (possible `internal/node/outer`) | Serve one accepted outer Carrier and join its inner lanes. | Its current two files use Route and standard-library types; a move still requires a production caller, owned cleanup result and tests in the same change. |
| Forwarding lifetime (currently `internal/node`) | Start and join one State-authorized forwarding duty. | The server retains listener, spend and host reservations, admitted producers, outgoing Carrier sessions and readers. Moving this cohort now would transfer private `runtimeConfig`, the copied duty value, spend-close and probeServer return contracts. Deepen it in place first; a later package requires a demonstrated smaller interface. |
| Private role probe (currently `internal/node`) | Start and join the selected probe duty under Node lifecycle. | Four implementation files use the common running-duty handle. Its name and navigation can improve in place; no independent package boundary has been shown. |
| Direct recipient owners | Issuer, Introduction, Resolution, and Data Join each retain their own listener/admission/work/drain. | Extract an individual package when its state and lifecycle form a deep module with a small API. A thin Route adapter stays as a clearly named file in `internal/node` if a package would add only forwarding methods. |

These are ownership responsibilities, not a directory plan. Create any
subpackage only with its implementation, `doc.go`, behavior tests, non-test
caller, and package-map entry. Exact exported names follow the first real
caller; no package is required merely to remove a filename prefix.

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

### 2. Prove the shared outer boundary

- Map `serveClosedOuter`, its writer, all five production callers and the
  accepted-connection close result. F-61 shows that only Data JOIN currently
  retains non-benign accepted-connection close errors; the other four duties
  discard them. Keep `Done` as the accept-loop result and `Drain` as the final
  joined cleanup result. Preserve the issuer root's late-close question after
  a timed-out `Drain`. Keep the close/interruption/children-join order and
  deadline behavior together.
- If that gives one small acyclic caller-facing operation, move the owner and
  its behavior tests to `internal/node/outer` in one slice. Otherwise keep
  the two responsibilities named in Node without duplicating Route's bridge.
- Acceptance: no parent-package import or copied writer/bridge logic, and
  tested cancellation, queued write, deadline, accepted-child close failure,
  and joined cleanup without releasing a still-owned root.

### 3. Narrow current-authority access

- Trace Node-local current duty facts, accepted closed-profile/recipient
  projection, and shared token verification at their actual callers. State
  still authenticates its read-only views; process admission and duty choice
  remain in `internal/node`.
- Define the exact data that each duty needs. A forwarding caller receives
  current receiver/peer/token decisions, not the whole process `Config` or
  mutable State runtime. Issuer and control recipients consume the same exact
  verification rules without copying them.
- Extract an `authority` package only if this yields a narrow real caller
  contract without exporting `runtimeConfig`, moving State authority, or
  duplicating its checks. Acceptance: successor State or profile loss makes
  every affected duty unavailable at its existing recheck points, and no
  role can substitute a plan value for State authority.

### 4. Deepen forwarding under its existing owner

- Keep the forwarding listener, receiving-resource group, admission, sessions,
  links, queue, bootstrap/peer logic, Carrier relay choice and shutdown under
  the existing Node duty owner. Name each file for its exact responsibility;
  the `closed_forwarding_` prefix still distinguishes this duty inside Node.
- Make the local start/stop/join map readable before exporting it. The
  forwarding server already owns spend root, pool, host reservations,
  accepted producers, outgoing readers, their join order and terminal result.
- Keep one explicit startup resource owner: failed construction closes only
  resources actually acquired; after successful start the running duty owns
  them until drain. Preserve the bounded drain result on repeated calls.
- Reassess a `forwarding` package only after a narrow authority/outer seam
  removes dependence on most of `runtimeConfig`, the copied duty value and the common
  running-duty handle. Acceptance for this step is a clear call and resource
  map with the existing admission, bootstrap, parent/child, relay, limit and
  shutdown behavior preserved for both Carriers.

### 5. Place the remaining duties and probe

- Keep the private probe in `internal/node` under the common duty lifecycle;
  rename only its private files or handle where that improves navigation.
  Extract it only if a later source audit shows a cohesive independent API.
- Review each direct recipient—Introduction, Resolution, Data Join, and
  Issuer—by its independent state transitions and cleanup. Extract a role
  package only when it owns meaningful behavior and can take current
  authority without a parent import. Otherwise retain a short, purpose-named
  Node adapter around Route's deep module. Record the decision in the Node
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
is the shared outer ownership decision and exact final-result trace. The
first major structural result is a readable forwarding duty with its existing
admission and cleanup behavior; a new package is conditional on a real seam.
Each result must be coherent and usable before another package move starts.

If a proposed package needs most of `runtimeConfig`, imports its parent, or
duplicates current State checks, repair the authority seam first. If a direct
role only delegates to Route, keep its adapter in Node. If a behavior check
fails, fix that specific owner before broadening the refactor. Changes to
wire/persisted identities, authority, protection policy, or product behavior
require their own accepted contract rather than being folded into this plan.
