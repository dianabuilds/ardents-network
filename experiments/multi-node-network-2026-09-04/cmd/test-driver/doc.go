//go:build ignore

// Command test-driver is the multi-node pilot's Go orchestrator. It owns
// three subcommands:
//
//   - prebake — produces a fresh closed-alpha State fixture (network id,
//     authority key, signed epoch, materialization, two source-server plans,
//     one shared source-client plan, x509 certs) into the evidence directory
//     the Docker compose network bind-mounts into every container.
//   - verify  — reads the per-node "source-wave-accepted" event each
//     consumer wrote and asserts that all six consumers converged on the
//     same State.
//   - self-test — checks the convergence verifier against six synthetic
//     successful consumer events.
//
// The binary is disposable and lives entirely under
// experiments/multi-node-network-2026-09-04/. It is not a maintained command
// and is not registered in docs/development/package-map.md.
package main
