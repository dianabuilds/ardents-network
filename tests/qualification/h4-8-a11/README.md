# H4-8 A11 Publisher-to-User soak and fault qualification

Status: **contract selected on 2026-08-28; implementation and first execution
are pending.** The Product Owner's H4-3/H4-8 closure goal authorizes this
bounded campaign. This file and its checked runner must be committed before
the first decision-bearing attempt. A later result may record success or
failure, but must not change the contract retroactively.

## Question

Can one exact functional-alpha source and release identity sustain the selected
Publisher-to-User HTTP/1.1 journey on the project Windows/Ubuntu topology, and
can the same journey fail honestly when its Application, Publisher Endpoint,
Carrier, or product Node fault domain is lost?

## Identity and topology

The runner requires, rather than discovers, all of these inputs:

- one clean committed candidate-source worktree, its source revision, and the
  immutable release tag resolving to it; and a separately clean committed
  runner worktree whose revision is retained as harness evidence;
- the exact closed-alpha archive, archive SHA-256, Alpha Enrollment Pin,
  Endpoint SHA-256, and control-companion SHA-256;
- the literal Ubuntu VPS address, SSH user, SSH key, and base port; and
- one previously absent external evidence directory outside Git.

Windows 11 is the User and campaign-observer host. Ubuntu 22.04 LTS `x86_64`
is the Publisher/transit host. The remote campaign container uses host
networking with an aggregate `1` vCPU, `1 GiB` memory, and `128` PID limit.
Every cell gets a new exact `/tmp/ardents-h4-8-a11-*` root and container, and
the runner refuses a pre-existing root, container, or occupied selected port.

Publisher and User are separate `reference-c2` qualification processes built
from the exact selected revision and bound to the maintained Endpoint, Route,
Service, reachability, and control Modules. They are not the released
`ardents-linux-amd64` process and do not create a Windows Endpoint-support
claim. Carrier and Node-loss cells use the exact `ardents-node` command built
from that revision; killing a fixture transit role cannot satisfy the product
Node cell. The release archive and its two program hashes remain mandatory
identity inputs and are independently checked, but are not misreported as the
processes that generate the soak workload.

The campaign uses short-lived qualification State containing the roles needed
by the journey. The published functional-alpha State is deliberately empty and
is not rewritten or described as available merely to run this qualification.

## Frozen workload

The normal cell opens one authenticated Target and one Service Connection. It
then performs exactly `1800` sequential cycles at one scheduled cycle per
second for a declared duration of `30 minutes`:

1. `POST /publish?draft=1` carries the fixed form body, content type, prior
   cookie, and a monotonically increasing anti-replay cycle identity;
2. the Publisher returns `302`, the exact `/timeline` location, and the
   selected session cookie;
3. `GET /timeline` returns the two preserved chunks `first-` and `second`; and
4. only after cycle `1800` does the User send `GET /close` and require `204`.

The same already-selected Service Connection carries every Publisher request
and response. Reconnect, Route regeneration, retry, replay, another Target,
and another Carrier profile are forbidden. A no-fallback probe runs after each
60 completed cycles and must show that an unregistered `.ard` name and an
ordinary Internet name did not become Publisher destinations. Every cycle has
a `5 second` terminal deadline. A missing, duplicate, out-of-order, late, or
changed cycle fails the attempt; a later retry cannot erase it.

The observer records the planned and actual start, latency, byte counts,
completed/expected cycle counts, no-fallback probes, Service Connection and
Route generations, and recovery count. It reports minimum, maximum, and
selected latency quantiles without using an average as a pass oracle.

## Frozen fault cells

Each fault cell uses a new topology, completes exactly `60` normal warm-up
cycles, records explicit readiness for the fault owner, injects one fault, and
requires the User's classified terminal result within `15 seconds`. No cell
may replay a request, select a fallback, reconnect, or leave a live listener,
socket, relay connection, process, or container. After cleanup, a second new
topology must pass an exact `10`-cycle normal canary before that cell passes.

| Cell | Injection | Required distinction |
|---|---|---|
| Application loss | reset the Publisher-local Application connection after the warm-up | the Publisher Endpoint and transit roles remain alive |
| Publisher Endpoint loss | hard-stop the Publisher Endpoint after the warm-up | the local Application observes loss; the transit path is not called the failed owner |
| Carrier loss | reset the active inter-Node TCP connection through a test-owned transparent relay | every product Node remains alive and the relay retains no connection |
| product Node loss | hard-stop the exact active `ardents-node` Rendezvous process | the Carrier relay, Publisher Endpoint, and Application remain alive until the Node fault |

Carrier loss is not simulated by killing the alpha OHTTP Relay, and product
Node loss is not simulated by stopping a `reference-c2` transit fixture.

## Expiry boundary cell

The sixth denominator member is a deterministic boundary companion. It does
not collapse the exact candidate's authenticated Release no-new-work bound and
terminal bound into one `NotAfter`. At `reference_at` and one second before
`BuildSafetyNoNewWorkAfter`, the full candidate is accepted and authorizes
work. At exact no-new-work and at terminal minus one second, the catalog and
Network remain current while Release is `update-required` and grants no
authorization. At exact terminal, fresh and persisted ACA1 inspection refuses
the expired catalog, all three exact components are expired, and direct
Release inspection classifies the authenticated build as `release-revoked`.
At terminal plus one second, direct TUF evaluation is `release-expired`.

The candidate oracle requires terminal to equal the authenticated catalog and
component `NotAfter` values, Network `ValidUntil`, Release terminate-after,
and the executable TUF timestamp, snapshot, and targets expiries. Separate
State/control and Service fixtures also check their own before/at boundaries;
at expiry fresh work is refused, no scoped browser listener remains, and no
lower generation, stale descriptor, cached control result, persisted floor,
or wall-clock retry restores authorization. The runner records every exact
injected UTC instant and maintained test/command entrypoint. This cell does not
wait for the public bundle to expire and does not alter an earlier accepted
floor.

## Observation and resource contract

The Windows runner samples once per second while each cell is live. It records
the campaign/test and qualification-process PIDs, CPU time, working set,
private bytes, handle count, and thread count. The Ubuntu observer records the
container ID/image, cgroup CPU and memory facts, `memory.events`, current and
maximum PIDs, process/FD counts, network byte counters, and Docker state. Both
host envelopes include OS/kernel, architecture, logical CPU count, memory,
Docker and Go versions where applicable, and the selected image ID.

No consecutive observer timestamps may be more than `2 seconds` apart. A
missing series, sampling error, container OOM/OOM-kill, PID-limit event,
container restart, memory use above `1 GiB`, or more than `128` remote PIDs
fails the attempt. CPU saturation is retained as an observation; its behavior
effect is judged by the fixed workload deadlines rather than by inventing an
unmeasured capacity claim.

## Denominator and evidence

The fixed denominator is `6/6`: normal soak, Application loss, Publisher
Endpoint loss, Carrier loss, product Node loss, and expiry boundaries. A cell
is green only when its primary run, required canary, resource observations,
and cleanup all pass. An unavailable prerequisite is an invalid environment,
not a skip or pass. There is no automatic retry. Every failed, invalid, and
successful attempt receives a distinct retained directory and disposition.

Each attempt retains outside Git:

- exact source/tag/archive/program/fixture/runner/config digests;
- complete stdout and stderr for every role and the campaign entrypoint;
- Windows and Ubuntu host envelopes and one-second resource series;
- workload/fault timing, cycle and route/recovery metrics;
- remote container/root/port inventory before and after the cell;
- exit statuses, cleanup result, and a SHA-256 inventory of the evidence; and
- the verdict plus any later defect or invalid-environment disposition.

The checked Make target calls `invoke-windows.ps1`, not the runner directly.
It captures the runner's complete independent stdout, stderr, and exit status
as `entrypoint.*` evidence, records both entrypoint/runner digests, and
rebuilds the root inventory after adding those files. A zero process exit is
not accepted unless the runner also retained an accepted A11 campaign receipt.

## Pass claim and limitations

A `6/6` result establishes only a bounded low-resource functional envelope for
this exact project-operated revision, fixture, topology, workload, and fault
set. It is not a capacity, availability, recovery, hostile-network,
censorship-resistance, anonymity, independent-operator, independent-custody,
public-State, Windows Endpoint-support, browser-isolation, or Public Beta
claim. A changed executable, Carrier/profile, State/control input, topology,
workload, limit, or oracle is a new campaign identity and reopens the affected
evidence.
