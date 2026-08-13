# H3 Stage 2 local Docker smoke

This profile repeatedly runs one Client, four selected Route Nodes, and one
Publisher in six separate containers on an internal Docker network. Every
attempt uses current authenticated `h3-route-tracer-v1` Network State, a fresh
random canary, exact peer pins, external container-exit checks, cleanup, and the
independent `ardents-route-qualify` command.

It is local development evidence only. It is not official Stage 1
qualification, a production transport result, operator independence, or an
anonymity claim.

Run from a clean committed repository with Docker Compose:

```text
ardents-qualify route-smoke \
  --fixture /absolute/new/fixture \
  --evidence /absolute/new/evidence \
  --compose /absolute/repository/tests/qualification/h3-route-v1/compose.yaml \
  --source /absolute/repository \
  --duration 20m
```

Duration must be between 10 and 30 minutes. The credential/state fixture is
ephemeral and is removed before the terminal verdict. Only the externally
owned evidence bundle remains outside Git: preflight identity, per-attempt
actor evidence and verifier verdicts, cleanup and terminal records, and the
aggregate summary. Raw private keys are never retained.
