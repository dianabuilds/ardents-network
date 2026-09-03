//go:build ignore

// Command test-driver is the multi-node pilot's Go orchestrator. It owns
// five subcommands split across three responsibility files (prebake.go,
// verify.go, selftest.go):
//
// Slice 1 (alpha_round_trip):
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
// Slice 2 (adversary_rejected, the current default scenario):
//
//   - prebake_adversary — like prebake, but also generates an attacker-
//     controlled authority key, re-signs the same epoch body with it,
//     and writes source-c (adversary) and client-probe plans. The forged
//     epoch is written next to the real one so accept-offline for the
//     adversary state root uses the forged signer.
//   - verify_adversary — like verify, but calls VerifyAdversaryScenario
//     which tightens the acceptance criteria to per-node generation
//     match, exact [4]string source_outcomes signature, specific node-id
//     assignment (node-1..5 honest, node-6 probe), and the same
//     distinct=1 convergence check. The self-test (above) also runs one
//     successful and eight rejecting synthetic cases (N1..N9) against this
//     verifier.
//
// The binary is disposable and lives entirely under
// experiments/multi-node-network-2026-09-04/. It is not a maintained command
// and is not registered in docs/development/package-map.md.
package main
