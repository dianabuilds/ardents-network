# Rendezvous Contributor dedicated-host profile

Status: **retirement-only after the project-qualified dedicated-host Functional
Alpha accepted on 2026-08-29.** New `apply` and explicit `restart` are retired;
the remaining commands exist only to inspect and remove an authenticated owned
installation. This is not a public Contributor offer or a capacity/availability
claim.

The canonical profile identity is `ardents-rendezvous-dedicated-host-v1`.
Readers accept the historical `h4-5-rendezvous-alpha-v1` identity only for
already pinned bundles, Node plans, and installation records; runtime state,
new records, and reports use the canonical identity.

## Selected retirement transition

[ADR-0089](../adr/0089-retire-old-node-starts-preserve-owned-shutdown.md)
retires `apply` and `restart` for both canonical and historical profile inputs.
They return `old Contributor start is retired` after command-shape recognition
and before filesystem, systemd, supervisor, root, listener, or Network effects.

An exactly authenticated existing installation retains `diagnose`, `drain`,
`withdraw`, and confirmed `remove` solely to retire owned resources. Neither
these actions nor interrupted-update recovery may Start, Restart, Enable, or
complete an update by executing the current, previous, or next generation.
`diagnose` authenticates managed evidence and reads status. `drain` may Stop an
active owned unit and wait finitely for `WITHDRAWN`, but never starts an
inactive unit. `withdraw` additionally Disables it. `remove` retains the exact
deployment confirmation and inactive, disabled, `WITHDRAWN` preconditions and
deletes only the managed installation.

If an interrupted or ambiguous installation cannot be authenticated and
retired without execution, the command fails explicitly and retains evidence.
It does not choose a generation by convenience, rewrite the stored profile,
adopt foreign files, or direct manual systemd/filesystem mutation as a
supported recovery. Historical profile recognition authenticates retained
evidence only and never grants a new start.

Before any retained Control action, recovery verifies the current generation
or one exact authenticated predecessor under the root lease. A valid active
current generation may remain observable; a predecessor that must be restored
is stopped first and remains inactive and `WITHDRAWN`. A valid inactive
generation is never started. Exact `previous`/`next` residue and its transition
record are removed only after authentication and reconciliation; incomplete,
conflicting, or foreign evidence fails with no Start/Restart and remains for
operator-visible diagnosis.

This contract does not assert that a real owned installation exists or know
its host, profile, deployment/generation, active/enabled/update state, or
external bundle, journal, and snapshot residue. Those facts must come from the
bounded authenticated no-start observation of the installation being retired;
missing facts never authorize execution or foreign-state adoption.

## Exact supported shape

The profile runs one `rendezvous` duty and nothing else inside its dedicated
service boundary on a Product Owner-declared existing Ubuntu LTS `x86-64`
host with cgroup v2 and systemd as PID 1. Host CPU, RAM, disk, link, and other
project workloads are recorded observations, not eligibility thresholds. The
Contributor alone owns its managed roots, identity, systemd service, cgroup,
and public listener. Temporary campaign fixtures may share a project host but
do not create a supported co-resident Endpoint-plus-Contributor product mode.
One unprivileged public TCP port must reach the exact address in authenticated
Network State. Outbound TCP must reach the two literal-IP authenticated Source
addresses in the plan; there is no discovery, DNS fallback, alternate Source,
or public admission.

The service runs through systemd `DynamicUser` with these enforced process
limits: `CPUQuota=100%`, `MemoryHigh=192M`, `MemoryMax=256M`, `TasksMax=64`,
`LimitNOFILE=256`, `GOMAXPROCS=1`, and `GOMEMLIMIT=134217728`. Rendezvous admits
at most four concurrent handshakes, two waiting legs, one paired route, and
64 MiB for that pair. The observed host link is evidence, not an Ardents
throughput promise or a profile eligibility gate. The authenticated bundle is
bounded to one executable of at most 128 MiB and nine configuration inputs of
at most 64 KiB each. Update temporarily retains one previous generation.
The runtime has no application queue: its queue-item and queue-byte ceilings
are exactly zero because a leg is either in a finite handshake/waiting
reservation or an active direct pump. Network State plus local role state are
measured together on every resource observation: `320 MiB` enters `PROTECT`,
recovery requires less than `256 MiB`, and `384 MiB` or more enters terminal
`DRAIN`; more than 5,000 regular files, 5,000 directories, or 32 directory
levels fails closed. The two installed
generations and input bundle are a separate static inventory bounded by their
manifest sizes. The accepted declared-host evidence measured actual disk,
network, CPU, memory, socket, FD, task, and Go-runtime observations under these
values.

## Authenticated input

The configuration authority supplies exactly two things by separate channels:

1. one directory containing `manifest.json` plus the closed inventory named by
   `ardents-contributor-bundle-v1`; and
2. the lowercase SHA-256 digest of the exact `manifest.json` bytes.

The operator compares the second value with the independently received pin;
the command verifies that pin before parsing the manifest, verifies every
listed file digest, rejects extra/missing inventory, and accepts only the fixed
Rendezvous plan and resource reservations. That format is now compatibility
evidence for the retained implementation and its tests, not authority to
transfer or apply a new bundle. Any pre-existing caller-owned transfer copy is
external residue: retirement commands neither adopt nor delete it.

The historical command shape remains recognized so it can fail with the stable
retirement result rather than being reinterpreted as another action:

```sh
./ardents-node contributor apply --bundle /absolute/owner-only/bundle --manifest-pin MANIFEST_SHA256
```

It does not open or validate the bundle and cannot create or update an
installation. The internal Apply and restart implementations remain temporarily
pending a separate consumer/deletion audit; only their internal behavior tests
call them, and neither has an accepting command caller. Retaining them is not
authority to start an installation.

## Diagnose, drain, withdrawal, and retired restart

Each successful command emits one `ardents-contributor-report-v1` JSON object
containing only profile, deployment/generation and digest facts, lifecycle
state, and active/enabled state.

```sh
/usr/lib/ardents-contributor/ardents-node contributor diagnose
/usr/lib/ardents-contributor/ardents-node contributor drain
/usr/lib/ardents-contributor/ardents-node contributor withdraw
```

`diagnose` re-authenticates every managed file and the fixed unit, reads the
bounded last lifecycle diagnostic, and asks systemd for current state.
The recognized `restart` syntax returns the same stable retirement result
before opening the installation or invoking systemd. `drain` asks the Node to
stop accepting handshakes, finish its finite drain, and reach `WITHDRAWN`; the
service remains enabled only for retirement handling, not for restart.
`withdraw` performs the same finite stop and then disables the unit. A command
fails instead of reporting a partial transition as success.

The two bounded local diagnostics are:

- `/var/lib/private/ardents-contributor/diagnostics/lifecycle.json` — the last
  Node lifecycle event; and
- `/var/lib/private/ardents-contributor/diagnostics/resource.json` — the last
  resource observation or pressure transition.

They contain no private key or Application payload. Routine process stdout is
discarded only after each event is durably reduced to the appropriate bounded
diagnostic file; stderr remains in the system journal for terminal failures.
Journal retention, provider snapshots, and the caller-owned bundle are host
storage outside the managed runtime-state ceiling and must not be published as
network-capacity evidence.

## Complete managed removal

Removal is deliberately a two-step transition. First run `withdraw` and retain
its exact 32-byte deployment ID. Then run:

```sh
/usr/lib/ardents-contributor/ardents-node contributor remove --confirm DEPLOYMENT_ID
```

Removal is refused unless the service is inactive, disabled, and
`WITHDRAWN`, and unless the confirmation names the installed deployment. It
removes the managed executable generations, configuration and private keys,
Network State, local role state, diagnostics, installation record, and the
exact systemd unit, then reloads systemd and reports `REMOVED`. It does not
delete the caller-owned input bundle, unrelated host files, journal retention,
or provider snapshots. The operator must remove the owner-only transfer copy
after each apply and apply the provider's documented snapshot-retention policy;
those are declared external residues rather than hidden product repair steps.

## Honest failure boundary

If `diagnose` cannot authenticate the current installation, if READY or
WITHDRAWN is not reached within 15 seconds, or if the exact cgroup placement is
unavailable, do not edit managed files or invoke systemctl manually. Retain the
command error, bounded diagnostics, journal slice, and host observations as a
failed qualification attempt. This profile was accepted by the frozen
declared-host matrix; ordinary lifecycle success alone would not have qualified
it.
