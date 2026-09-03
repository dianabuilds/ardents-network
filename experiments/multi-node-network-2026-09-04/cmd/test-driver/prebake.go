//go:build ignore

package main

import (
	"context"
	"crypto/ed25519"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// artifactsDir is the directory inside the compose containers that holds the
// pre-compiled ardents, ardents-node, and test-driver binaries. The
// bind-mount in docker-compose.yml maps it from $ARDENTS_PILOT_EVIDENCE_DIR/artifacts
// on the host.
const artifactsDir = "/workspace/artifacts"

// runPrebake is the entrypoint for both `test-driver prebake` (slice 1) and
// `test-driver prebake_adversary` (slice 2). It owns the full closed-alpha
// State fixture assembly: generation of network id, authority key, signed
// epoch, materialization, and the five on-disk plans (source-a, source-b,
// source-c, client, client-probe) that the compose containers open at start.
func runPrebake(ctx context.Context, arguments []string, withAdversary bool) error {
	if len(arguments) != 1 {
		return errors.New("usage: test-driver prebake EVIDENCE_DIR")
	}
	evidenceDir, err := filepath.Abs(arguments[0])
	if err != nil {
		return fmt.Errorf("evidence dir abs: %w", err)
	}
	if err := os.MkdirAll(evidenceDir, 0o700); err != nil {
		return fmt.Errorf("mkdir evidence: %w", err)
	}
	fixturesDir := filepath.Join(evidenceDir, "fixtures")
	if err := os.MkdirAll(fixturesDir, 0o700); err != nil {
		return fmt.Errorf("mkdir fixtures: %w", err)
	}
	sourceAState := filepath.Join(evidenceDir, "source-a-state")
	sourceBState := filepath.Join(evidenceDir, "source-b-state")
	sourceCState := filepath.Join(evidenceDir, "source-c-state")
	now := time.Now().UTC()
	var fixtures Fixtures
	if withAdversary {
		fixtures, err = PrebakeAdversary(fixturesDir, now)
	} else {
		fixtures, err = Prebake(fixturesDir, now)
	}
	if err != nil {
		return fmt.Errorf("prebake: %w", err)
	}
	clientPin, sourceAPin, sourceBPin, err := WriteCerts(
		filepath.Join(fixturesDir, "source-ca.pem"),
		filepath.Join(fixturesDir, "source-a.pem"), filepath.Join(fixturesDir, "source-a-key.pem"),
		filepath.Join(fixturesDir, "source-b.pem"), filepath.Join(fixturesDir, "source-b-key.pem"),
		filepath.Join(fixturesDir, "client-ca.pem"),
		filepath.Join(fixturesDir, "client.pem"), filepath.Join(fixturesDir, "client-key.pem"),
		now)
	if err != nil {
		return fmt.Errorf("prebake certs: %w", err)
	}
	if err := WritePlans(fixturesDir, sourceAState, sourceBState,
		ResolveSourceAddress("PILOT_SOURCE_A_ADDRESS", DefaultSourceAddressA),
		ResolveSourceAddress("PILOT_SOURCE_B_ADDRESS", DefaultSourceAddressB),
		fixtures, clientPin, sourceAPin, sourceBPin, now); err != nil {
		return fmt.Errorf("prebake plans: %w", err)
	}
	if err := acceptOffline(evidenceDir, fixturesDir, sourceAState, fixtures, now, fixtures.AuthorityPublic,
		filepath.Join(fixturesDir, "generations", fixtures.Generation, "epoch.bin")); err != nil {
		return fmt.Errorf("accept-offline source-a-state: %w", err)
	}
	if err := acceptOffline(evidenceDir, fixturesDir, sourceBState, fixtures, now, fixtures.AuthorityPublic,
		filepath.Join(fixturesDir, "generations", fixtures.Generation, "epoch.bin")); err != nil {
		return fmt.Errorf("accept-offline source-b-state: %w", err)
	}
	if withAdversary {
		if err := WriteAdversarySourcePlan(fixturesDir, sourceCState,
			fixtures, clientPin, now); err != nil {
			return fmt.Errorf("write adversary source plan: %w", err)
		}
		if err := acceptOffline(evidenceDir, fixturesDir, sourceCState, fixtures, now, fixtures.AdversaryAuthorityPublic,
			filepath.Join(fixturesDir, "generations", fixtures.Generation, "epoch-adversary.bin")); err != nil {
			return fmt.Errorf("accept-offline source-c-state: %w", err)
		}
		if err := WriteProbeClientPlan(fixturesDir,
			ResolveSourceAddress("PILOT_SOURCE_B_ADDRESS", DefaultSourceAddressB),
			ResolveSourceAddress("PILOT_ADVERSARY_ADDRESS", DefaultAdversaryAddress),
			fixtures, sourceBPin, sourceAPin, now); err != nil {
			return fmt.Errorf("write probe client plan: %w", err)
		}
	}
	fmt.Printf("test-driver: prebake complete generation=%s network_id=%s client_pin=%x source-a-state=%s source-b-state=%s adversary=%v\n",
		fixtures.Generation, fixtures.NetworkIDHex, clientPin, sourceAState, sourceBState, withAdversary)
	_ = ctx
	return nil
}

// acceptOffline runs the production `ardents accept-offline` binary against
// one state root. It is invoked once per source (a, b, and conditionally c)
// by runPrebake. The state root is created fresh; the production accept-
// offline path populates it from the supplied --epoch, --inputs, and
// --materialization. The slice 2 fix uses the forged epoch path for the
// adversary state root so that the produced state root contains the forged
// epoch and accept-offline claims it under the adversary authority.
func acceptOffline(evidenceDir, fixturesDir, stateRoot string, fixtures Fixtures, now time.Time,
	authority ed25519.PublicKey, epochPath string) error {
	if err := os.MkdirAll(stateRoot, 0o700); err != nil {
		return fmt.Errorf("mkdir state root: %w", err)
	}
	authorityHex := fmt.Sprintf("%x", authority)
	generation := fixtures.Generation
	materializationPath := filepath.Join(fixturesDir, "materialization.bin")
	if err := os.WriteFile(materializationPath, fixtures.Materialization, 0o600); err != nil {
		return fmt.Errorf("write materialization: %w", err)
	}
	args := []string{
		"accept-offline",
		"--state-root", stateRoot,
		"--network-id", fixtures.NetworkIDHex,
		"--authorities", authorityHex,
		"--threshold", "1",
		"--at", now.UTC().Format(time.RFC3339),
		"--epoch", epochPath,
		"--inputs", filepath.Join(fixturesDir, "generations", generation, "inputs"),
		"--materialization", materializationPath,
	}
	ardents := filepath.Join(artifactsDir, "ardents-linux-amd64")
	logPath := filepath.Join(evidenceDir, "logs", "accept-offline-"+filepath.Base(stateRoot)+".log")
	if err := os.MkdirAll(filepath.Dir(logPath), 0o700); err != nil {
		return fmt.Errorf("mkdir logs: %w", err)
	}
	logFile, err := os.Create(logPath)
	if err != nil {
		return fmt.Errorf("create log file: %w", err)
	}
	defer logFile.Close()
	command := exec.Command(ardents, args...)
	command.Stdout = logFile
	command.Stderr = logFile
	if err := command.Run(); err != nil {
		return fmt.Errorf("%s: %w (see %s)", ardents, err, logPath)
	}
	return nil
}
