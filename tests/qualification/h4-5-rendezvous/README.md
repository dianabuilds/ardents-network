# H4-5 dedicated Rendezvous qualification

Status: **accepted and frozen for the dedicated-host Functional Alpha on
2026-08-29.** The retained composite disposition covers the declared-host
workload, fault, lifecycle, utility, and removal matrix. It preserves every
failed attempt and uses corrected-cell reruns plus one final smoke; the complete
campaign was not replayed.

## Decision question

Can one freshly installed `h4-5-rendezvous-alpha-v1` Contributor perform the
real Rendezvous position in the maintained Publisher-to-User C-2 topology,
stay inside its declared dedicated-host limits, fail and recover predictably,
and be operated and completely withdrawn by the Product Owner without hidden
repair?

The hypothesis is falsified by any unclassified loss of admitted work,
acceptance after drain or withdrawal, resource use beyond a declared ceiling,
unbounded residue, non-reproducible recovery, or an operator step not present
in the candidate runbook. A negative result rejects or retains the duty as
project-operated; it must not be softened into a supported Contributor claim.

## Fixed topology

- **Candidate host:** the Product Owner-declared existing native Ubuntu LTS
  `x86-64` project host, with systemd as PID 1 and cgroup v2 `cpu`, `memory`,
  and `pids` controllers. Its actual CPU, memory, disk, link, kernel, and
  co-resident project workload are captured rather than used as eligibility
  gates. The installed Contributor generation alone owns its managed roots,
  systemd service, cgroup placement, identity, and one public State-authorized
  TCP/TLS Rendezvous listener.
- **Topology fixtures:** available project-operated hosts run the two
  authenticated Sources and all remaining roles required to create one real
  Publisher-to-User C-2 path. The Sources have distinct identities, roots,
  families, keys, and ports. Temporary fixture co-location with the installed
  Contributor is accepted for this functional-alpha campaign only; it is not
  a supported Endpoint-plus-Contributor product profile and supplies no host,
  Source, availability, or operator independence.
- **Controller:** the Product Owner's workstation builds and hashes the exact
  repository revision, stages generation 1 and generation 2 bundles, controls
  both hosts over authenticated SSH, and retains evidence outside the
  repository. It is not an always-on production operator.

The candidate plan contains exactly those two literal Source addresses and
their authenticated names, roots, identities, families, and leaf-key digests.
There is no DNS fallback, discovery, alternate Source, or public admission.
The topology must prove that the installed systemd process, rather than a
fixture or co-hosted process, owns the Rendezvous role.

## Frozen workload and fault matrix

The campaign runs each deterministic decision-bearing cell once from fresh
fixture state and one bounded mixed workload of 260 paced cycles at 250 ms
intervals. That workload crosses the previously observed approximately
250-cycle failure boundary while keeping the installed workload near 65
seconds; it is an operation/lifecycle check, not an availability-duration
claim. The controller executes independent shards concurrently on both
Product Owner-declared existing Ubuntu VPS hosts and local isolated Docker
containers. The installed systemd Contributor and its real C-2 path remain on
one declared VPS; the second VPS and local containers run only supporting
fixture, link/fault, hostile-admission, and offline-oracle shards and do not
create another supported host profile.

The complete campaign has a hard 60-minute wall-clock ceiling: no new cell may
start after minute 50, leaving ten minutes for bounded collection and exact
cleanup. Each cell also has its own shorter deadline. Every cell records its
seed, UTC interval, exact binary and bundle digests, completed application
bytes, reservation/admission outcomes, Node lifecycle events, systemd/cgroup
samples, host link counters, and terminal classification. A failed attempt
remains in the denominator. After a product or qualifier correction, only the
affected cell is repeated, followed by one short ordinary installed-product
smoke; the complete campaign is not replayed and a later pass never erases the
earlier failure.

1. Idle `READY`, followed by one healthy full-duplex C-2 connection.
2. One continuously replaced healthy pair at the fixed one-pair capacity.
3. A second concurrent pair while the first pair is held; it must receive a
   bounded capacity refusal without evicting the admitted pair.
4. Four stalled TLS admissions, two authenticated unmatched legs, one slow
   reader/backpressured pair, reset, half-close, and bounded connection churn,
   each as a separately classified cell.
5. Sustained churn sufficient to cross a declared `PROTECT` boundary without
   crossing the systemd backstop; new work is refused while admitted work is
   preserved and recovery requires the complete low-watermark interval.
6. Terminal `DRAIN` at a declared emergency boundary; no new handshake may be
   accepted and every worker must join inside the Work Safety Lease.
7. Source/link loss, stale State, assignment successor, assignment expiry,
   listener failure, `SIGTERM`, abrupt process `SIGKILL`, host reboot, and
   network restoration, each injected only after the predeclared readiness
   marker.
8. Generation-2 update while idle and while a pair is held; ambiguous
   pre-switch stop failure must restore generation 1, and a successful update
   must run only authenticated generation 2.
9. Explicit drain, restart, withdrawal, attempted post-withdrawal connection,
   exact-ID removal, and residue inspection. The caller-owned bundles,
   provider snapshots, and journal policy are recorded as external residues.

The runtime has no application queue. Queue items and bytes must remain zero;
handshake, waiting-leg, and active-pair reservations are separate evidence.
Network State plus local role state enter `PROTECT` at 320 MiB, recover only
below 256 MiB, and enter terminal `DRAIN` at 384 MiB or 5,000 regular files.
Measurement itself fails closed above 5,000 directories or 32 directory levels.
The campaign must distinguish these mutable roots from installed generations,
the transfer bundle, the system journal, and provider snapshots.

## Operator burden and evidence

The controller records wall time and its zero-prompt campaign interval;
per-action timings cover generation-1 apply, diagnosis, restart, generation-2
update, fault recovery, drain, withdrawal, removal, and residue review. Its
required Base64-encoded `ardents-h4-5-operator-history-v1` input lists every
pre-campaign manual command, failed attempt, clarification, repair,
provisioning action, and input-verification action. Preparation active-human
seconds are recorded when they were instrumented; otherwise they are `null`
with an explicit measurement limitation rather than a fabricated zero.

One immutable evidence directory must contain:

- host and link declarations plus captured host/systemd/cgroup/network facts;
- source revision/status and SHA-256 digests for every staged artifact;
- the closed bundle manifests and independent pins, with private-key bytes
  excluded from the retained report;
- one result record per matrix cell and the complete attempt denominator;
- bounded lifecycle/resource diagnostics, systemd state, cgroup samples, host
  counters, fault receipts, and exact process identities;
- lifecycle command reports and timings, removal inventory, external-residue
  declaration, and an overall accepted/rejected disposition.

Evidence stays outside the repository until private material has been removed
and the bounded, reviewed summary is promoted to R-092. Raw evidence is never
silently converted into a capacity, availability, anonymity, public-admission,
or independent-operation claim.

## Host preflight

On the untouched candidate host, from the exact clean source revision:

```sh
export ARDENTS_H4_5_EVIDENCE_DIR=/var/tmp/ardents-h4-5-preflight-20260829
export ARDENTS_H4_5_LISTEN_PORT=49152
sudo --preserve-env=ARDENTS_H4_5_EVIDENCE_DIR,ARDENTS_H4_5_LISTEN_PORT \
  make prepare-h4-5-rendezvous
```

The preflight captures the selected host's actual envelope. CPU count, memory,
and link speed are evidence rather than rejection criteria under the Product
Owner selection of 2026-08-29. A stronger or differently sized existing host
therefore does not fail preparation; the installed process must still enforce
the exact `h4-5-rendezvous-alpha-v1` cgroup and runtime limits.

The Windows controller is `make qualification-h4-5-rendezvous`. It requires
two distinct declared VPS literal IPv4 addresses, an existing OpenSSH-compatible
primary key, and exactly one existing PuTTY secondary key
or owner-only secondary password file,
the independently verified secondary SSH host-key fingerprint,
the required `H4_5_OPERATOR_HISTORY_BASE64` factual history input,
root access to the primary systemd host, Docker on all three hosts, the pinned
`golang:1.26.6` image, an absent absolute evidence directory, and a clean
committed worktree. It starts the primary installed workload/smoke, local
Go/Docker oracles, and second-VPS Linux/Docker oracles concurrently. It stops
all still-running shards at minute 50, reserves the remaining ten minutes for
bounded evidence collection and exact cleanup, and rejects any missing cell,
nonzero exit, missing host, missing Docker engine, timeout, or cleanup failure.

The controller's closed 16-cell denominator maps the frozen matrix as follows:

- primary host envelope; installed bounded 260-cycle mixed C-2 workload with
  cgroup and link samples before and after workload; installed automatic `SIGKILL`
  recovery plus one real host reboot and changed boot identity; and the final
  installed C-2 lifecycle/removal smoke;
- local exact capacity/hostile-admission, `PROTECT`/recovery/terminal `DRAIN`,
  update/rollback, Source/State failure, stream/backpressure, and isolated
  Linux listener/process cells; and
- second-VPS host envelope, exact artifact stage/upload, isolated Linux
  incomplete-TLS/expiry/`SIGTERM`/successor-reassignment process cell, and
  exact shard cleanup; and
- one controller-owned final cleanup cell that removes only the exact
  campaign-ID local/remote containers and staged roots, recovers an interrupted
  generation-1 installation when possible, withdraws and removes the exact
  authenticated deployment, and verifies the unit, managed roots, listener,
  and containers absent even when a shard failed or hit the minute-50 stop.

The primary installed-host Go oracle requires an OpenSSH-compatible private key
and uses the system `ssh` client. The secondary shard may instead receive a
declared `.ppk` key or caller-owned password file and then uses installed PuTTY
`plink`/`pscp` in non-interactive batch mode. The controller does not convert,
copy, print, or retain private-key or password bytes. The final disposition
accepts only one unique result per expected name whose schema, exact source
revision, owning shard, zero exit status, and `passed` outcome all match.

The earlier passing idle and held-pair generation-2 update attempts remain
separate installed-product evidence and are not repeated by this campaign.
Earlier failures remain in their retained attempt directories and in the
complete denominator even though only their corrected cells were rerun.

The accepted retained preflight used commit
`bdb9a66523c26558a09c063aa06399b49c8fa4cf` at
`2026-08-29T07:40:49Z`. The declared existing project VPS reported Ubuntu
22.04.5 LTS, Linux 5.15.0-185 `x86_64`, four online CPUs, 8,109,136 KiB
`MemTotal`, running systemd 249, cgroup v2 with `cpu`, `memory`, and `pids`, an
unused port 49152, and no Contributor unit or managed paths. Its outcome is
`eligible-for-h4-5-campaign; no qualification result`. Retained external
evidence is
`C:\Users\vitek\Ardents-Release\evidence\ardents-h4-5-preflight-bdb9a665`;
the captured `input.sha256` verifies the host facts, observation, and exact
runner bytes. The earlier `1b810813` attempt is retained but superseded: review
found that it checked `ardents-contributor.service` rather than the exact
`ardents-rendezvous-contributor.service`; the accepted rerun corrected that
oracle and again found the exact unit absent.

## Installed-product smoke evidence

The first complete installed-product smoke passed on 2026-08-29 at commit
`174283d5`. On the selected existing VPS it installed generation 1, populated
an initially empty Network State root from the two pinned authenticated
Sources, reached `READY`, diagnosed and restarted the exact systemd service,
carried the maintained Publisher-to-User C-2 path, drained, refused a new TCP
connection, withdrew, removed the confirmed deployment, and left the unit,
managed roots, runtime root, and selected ports absent. The test completed in
43.62 seconds. Its retained external evidence is
`C:\Users\vitek\Ardents-Release\evidence\h4-5-smoke-174283d5-attempt4-20260829`.

Three earlier attempts remain in the denominator. Attempt 1 found that the
Windows-to-Linux fixture transfer had removed the staged executable bit.
Attempt 2 reached the real service and found that an empty State root exited
before its first Source refresh. Attempt 3 then passed bootstrap, lifecycle,
C-2 utility, and drain, but found that disabling an already drained service
treated a non-failed `systemctl reset-failed` result as an error. The retained
attempt directories respectively identify commits `adef2464`, `0d86d974`, and
`a8839270`; the product and qualifier corrections are committed separately.

This smoke evidence preceded the accepted campaign disposition below and is
retained as provenance rather than counted as a substitute for a matrix cell.

## Accepted qualification disposition

H4-5A/B accepted the dedicated profile on 2026-08-29. The original controller
campaign ran once in less than ten minutes. Nine cells passed, four produced
classified failures, and three downstream cells were absent after the
controller path stopped. Per the Product Owner's fixed budget, the campaign was
not rerun. Every failed attempt remains under
`C:\Users\vitek\Ardents-Release\evidence\h4-5-campaign-87bc4ab3-20260829` and
`C:\Users\vitek\Ardents-Release\evidence\h4-5-repair-9fcf33c3-20260829`.

The accepted composite denominator is:

- original passes: local capacity/hostile admission, listener pressure,
  pressure/storage, Source/State, stream semantics, update/rollback, primary
  kill/reboot recovery, secondary stage, and secondary cleanup;
- corrected-cell passes: both host envelopes, secondary upload and Linux
  process behavior, and exact secondary cleanup;
- accepted installed workload whose source content was later committed as
  `e3ff7ba7`: 260/260 cycles in
  65.129399 seconds, four no-fallback probes, one proxy TCP dial and zero
  redials, 107,559 accepted and acknowledged bytes, 51,786 received bytes,
  P95 cycle latency 136.883 ms, maximum start lag 246.373 ms, and a clean
  service-connection close;
- final installed lifecycle/resource smoke: apply, State bootstrap, readiness,
  C-2 carriage, update, restart, `PROTECT` recovery, drain, withdrawal and
  exact removal passed; and
- final cleanup inspection found the primary unit, managed executable root,
  private state root, selected listener, secondary shard roots, and named
  containers absent.

The classified failures found and retained were controller quoting and SSH
transport selection errors, a released-capacity race, atomic State replacement
during sampling, sampler shutdown behavior, and the oversized eight-minute
gate. The product/qualifier defects were corrected. The eight-minute gate was
not rerun: it was rejected as disproportionate to the narrow claim and replaced
by the bounded workload above. A final short regression also crossed 300 cycles
locally. No failure was removed from the historical denominator.

The accepted workload evidence is retained under
`primary-bounded-mixed-workload-accepted` and has remote-capture SHA-256
`D2054621CEDFB1BC5BBE95A3746D8069D204590254A1A54D288838039562285B`. The final
smoke is under `primary-final-smoke-accepted` and has remote-capture SHA-256
`26A2BF149A42625138FE1F43EC67E7135D09048577E5D30982543379B06D0765`.

## Claim boundary

The accepted disposition selects only one project-qualified dedicated
Rendezvous functional-alpha operating profile. It does not support co-resident
Endpoint contribution, permissionless admission, incentives, public capacity
or availability, Source independence, or independent-operator language.
