# Multi-agent real-network harness hardening

Status: **implemented; local qualification passed; owner acceptance pending.** This disposable
experiment repairs the invalid first run described in [POST-MORTEM.md](POST-MORTEM.md).
It does not qualify a VPS, a public network, independent operators, or a
direct-source-cap DoS claim.

## Question

Can goal-driven agents operate real `ardents refresh-sources` consumers
without owning shell commands or runtime paths, while one coordinator keeps a
stable consumer identity and publishes evidence that concurrent writers cannot
overwrite?

This is the R-139 question recorded in `docs/research/questions.md`. The
question registration is committed separately so the experiment commit remains
limited to this tree.

## Hypothesis and falsification

The harness should work when one run manifest fixes `--state-root`,
`local_role_state_root`, source plan, container, expected signature, and plan
hash for every persona. An agent may submit only a schema-tagged `refresh` or
`noop` decision.

The hypothesis is falsified if any honest/probe refresh uses a different path,
if an injected path or unknown field reaches Docker, if two writers can publish
the same persona sequence, if old evidence can be overwritten, or if the local
run produces a local-role lifecycle abort.

## Implementation

`cmd/wrapper` exposes five commands:

- `prepare EVIDENCE_ROOT` creates a cryptographically unique run ID, private
  persona plans, prompts, fixed paths, and a hashed manifest.
- `prompt MANIFEST PERSONA` prints the complete action-only LLM contract.
- `act MANIFEST PERSONA ACTION_FILE` validates one decision, obtains an
  exclusive persona writer lease, executes a wrapper-owned Docker command, and
  publishes one immutable event.
- `verify MANIFEST` is the only counter/aggregator. It rejects gaps, identity
  drift, unexpected signatures, and every retained runtime failure.
- `self-test` checks the narrow action and classifier contracts.

The Compose file runs three real mTLS Sources, the live clock owner, and one
inert `agent-executor`. Source containers are never reused as mutable consumer
state.

## Local run

Run this only on the local Docker host. All generated binaries, credentials,
State, plans, prompts, and event evidence must remain in a fresh system-temp
directory outside this repository.

First generate fresh Slice 2 artifacts and certificates:

```powershell
$env:ARDENTS_PILOT_EVIDENCE_DIR = Join-Path $env:TEMP ("ardents-multi-agent-{0}" -f [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $env:ARDENTS_PILOT_EVIDENCE_DIR
Push-Location experiments/multi-node-network-2026-09-04
docker compose --profile build up --detach builder prebake
$prebakeStatus = docker wait ardents-multi-node-pilot-prebake-1
docker compose down
Pop-Location
if ([int]$prebakeStatus -ne 0) { throw "prebake failed: $prebakeStatus" }
```

Build and prepare the wrapper outside the repository, then start the local
network:

```powershell
$wrapper = Join-Path $env:ARDENTS_PILOT_EVIDENCE_DIR 'artifacts\multi-agent-wrapper.exe'
& experiments/multi-agent-real-2026-09-04/cmd/wrapper/build.ps1 -OutputPath $wrapper
$manifest = (& $wrapper prepare $env:ARDENTS_PILOT_EVIDENCE_DIR).Trim()
Push-Location experiments/multi-agent-real-2026-09-04
docker compose up --detach --wait
Pop-Location
```

For each scheduled persona decision, give the output of `wrapper prompt` to
the agent and pipe its single JSON response to `wrapper act`; `-` means stdin.
An external action file is also accepted. For example:

```powershell
& $wrapper prompt $manifest honest_user
'{"schema":"ardents-agent-action-v1","action":"refresh"}' | & $wrapper act $manifest honest_user -
```

Run `wrapper verify $manifest` only after the tick counts in
[S3.6.5-CONTRACT.md](S3.6.5-CONTRACT.md) are met. Stop the local network with:

```powershell
Push-Location experiments/multi-agent-real-2026-09-04
docker compose down
Pop-Location
```

## Checks and evidence

The probe performs one evidence-producing refresh and then noops during its
three-minute observation window. This is required by production semantics:
the expected `invalid-state` outcome activates durable backoff even though the
valid Source satisfies the threshold. Hammering refresh during that backoff is
an invalid persona policy, not additional adversarial evidence.

`cmd/wrapper/test.ps1` runs the named-source unit tests, vet, build, and runtime
self-test. The unit suite covers action-field injection, plan drift, stable
commands, exact four-slot signatures, typed error classification, first-refresh
admission, no-overwrite publication, and concurrent writer exclusion.

Runtime evidence belongs under `EVIDENCE_ROOT/runs/<run-id>/`; none is checked
in. [S3.6.5-RESULTS.md](S3.6.5-RESULTS.md) records only commands, counts,
hashes, and a bounded verdict after qualification.

## Result and disposition

The original run remains inconclusive, but the replacement coordinator passed
the sequential local-Docker qualification recorded in
[S3.6.5-RESULTS.md](S3.6.5-RESULTS.md): 11 honest accepts, one battery accept
plus two noops, and one expected probe reject plus four noops, all on one
verified generation and with no retained runtime failure. Retain this tree as a
disposable falsification harness. Keep the direct-source cap question deferred
until a realistic attacker capability and caller path are specified.
