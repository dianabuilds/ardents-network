# Component execution contract proposal

Draft first architecture slice, based on local dev commit
`63e3b4239d9a803ea0af3d74f07c3f24d9ed7303`, 2026-10-02. The Product Owner
authorized work in the existing branch and stopped other chats. This proposal
does not select an additional product command, package, network operation or
protection claim. GitHub CLI returned HTTP 401 when querying the C0 milestone;
That CLI authentication failure does not establish MCP access failure. The
Product Owner subsequently directed using GitHub MCP and treating the existing
backlog separately from this architecture work. No existing issue is selected
as the scope of this proposal. No new C0 code is started by this document.

## Acceptance boundary

Define one independently observable component invocation: validate its declared
inputs before effects, start its owned work, report actual readiness or failure,
stop admission, cancel and join all owned work, and return the retained terminal
result. This is an invocation contract, not a universal network controller.

The first implementation must pair this contract with a real selected component
and non-test consumer. No empty runtime framework or speculative package is
created. Admission remains the proposed pilot; its precise bounded responsibility
and dependencies must be selected before implementation.

## State and ownership

| State | Admission | Observation | Permitted next states |
|---|---|---|---|
| Validating | Closed | Inputs being checked; no runtime readiness | Starting, Failed |
| Starting | Closed | Owned startup is in progress | Ready, Stopping |
| Ready | Open only under live component authority | Declared component prerequisites hold | Stopping |
| Stopping | Closed immediately | Cancellation issued; cleanup not yet complete | Stopped |
| Stopped | Closed | All owned work joined; immutable terminal result available | None |
| Failed | Closed | Pre-effect validation failed; no runtime work exists | None |

Startup failure after effects transitions through Stopping. Readiness lost during
work closes admission and retires this invocation; this first contract has no
implicit repair or restart. Explicit restart creates a fresh invocation identity.
Failed validation and failed terminal outcomes remain distinguishable.

The invocation owns its child work and completion result. The selected domain
owner owns readiness predicates and authority. The command adapter owns parsing
and human-facing output. Platform mechanism owns process attachment and verified
cleanup where required. Telemetry owns no admission or accounting authority.

## Lifetime rules

- Cancellation requests stop; it does not prove completion.
- Stop closes admission before any waits and is safe under concurrent requests.
- Completion is published exactly once after all owned work is joined.
- Repeated completion reads return the same outcome, including cleanup failures.
- A caller wait deadline cannot turn incomplete cleanup into success. It reports
  incomplete termination; ownership and eventual completion remain explicit.
- Late startup or operation completion cannot revive a stopped invocation or
  attach to its replacement.
- No waits for child completion occur while holding a lock needed by that child.
- Durable state is neither deleted nor reset by generic shutdown or restart.

These rules must be reconciled with the selected component's actual locks,
durability and platform lifecycle before choosing Go method signatures.

## Configuration and operator contract

Configuration is explicit, bounded and owned by the selected component. Unknown,
contradictory or unauthorized input refuses before effects. Secrets are not
accepted in arguments or emitted in status. Status distinguishes component
readiness from complete Endpoint readiness and qualification. Output describes
an actionable local failure category without destinations, raw errors or keys.

No new launcher, arbitrary executable selection, weaker Route or default remote
telemetry destination is introduced. Installed identity and confinement remain
obligations of existing accepted contracts.

## Observations

Required semantic observations are startup outcome, readiness transition,
admission refusal category, in-flight work, stop requested, cleanup outcome and
duration. Categories and attributes have finite declared vocabularies. Sensitive
facts are removed at the producer boundary.

OpenTelemetry is the proposed instrumentation/export mechanism. Providers and
bounded exporters belong to composition; Collector and storage remain external.
SDK selection and dependencies require the repository's dependency acceptance
process before go.mod changes. No cross-role trace propagation is automatic.
Disabled or unavailable telemetry cannot change domain outcomes or block cleanup.

## Independent acceptance scenarios

1. Invalid configuration causes zero owned runtime effects.
2. Startup success reports readiness only after actual prerequisites hold.
3. Startup failure after acquiring resources joins their cleanup.
4. Stop during startup rejects late readiness.
5. Concurrent operation admission and Stop have a defined ordering; no operation
   begins after the admission boundary closes.
6. Repeated Stop joins the same invocation and retains the original outcome.
7. Cleanup failure remains visible and cannot become success through retry.
8. Caller timeout reports incomplete termination while child ownership persists.
9. Restart cannot receive an old invocation's completion or authority.
10. Telemetry saturation or outage bounds overhead and does not alter outcomes.

Tests must use production entry points and independent expectations. Isolated
tests establish component behavior only; actual installed lifecycle and network
qualification remain separate checks.

## Isolation and continuation

The pilot's temporary package name and exact allowed imports are recorded before
creation. Legacy dependencies are forbidden directly and transitively. Test
fixtures do not manufacture missing production authority. New state cannot be
mutated by old owners. Integration adapters, if selected, are outside the new
owner and their remaining legacy reliance is reported explicitly.

For each finished behavior slice retain its contract, acceptance tests, source
checkpoint and exact next step. GitHub Issues own execution status. This file
owns the proposed contract, not the task ledger. Before resuming after context
compression verify branch, HEAD, local changes, selected issue and actual callers.

Next admission step: use GitHub MCP for a dedicated architecture slice record,
without taking scope from the old backlog, and finalize the pilot component's
authority, state and permitted effects. Implementation
then proceeds in the existing dev checkout as explicitly authorized, preserving
unrelated changes and required quick/full verification gates.
