package recoverysmoke

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/dianabuilds/ardents-network/internal/qualification/byteio"
)

type stressShapers struct {
	observer dockerObserver
	clock    time.Time
	services [2]string
	values   [2]shaperEvidence
}

type stressShaperConfig struct {
	SchemaVersion string `json:"schema_version"`
	RunID         string `json:"run_id"`
	Role          string `json:"role"`
	Mode          string `json:"mode"`
	Profile       string `json:"profile"`
	Seed          uint32 `json:"seed"`
}

func prepareStressShapers(observer dockerObserver, phase string, direct bool,
	targets map[string]string, clock time.Time) (stressShapers, error) {
	var nonce [4]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return stressShapers{}, err
	}
	root := filepath.Join(observer.input.FixtureRoot, "shapers", phase+"-"+hex.EncodeToString(nonce[:]))
	observer.shaperRoot = root
	result := stressShapers{observer: observer, clock: clock}
	prefix := "candidate"
	if direct {
		prefix = "direct"
	}
	result.services = [2]string{prefix + "-shape-client", prefix + "-shape-publisher"}
	for index, role := range []string{"shape-client", "shape-publisher"} {
		targetRole := []string{"client", "publisher"}[index]
		if targets[targetRole] == "" {
			return stressShapers{}, errors.New("S4.3 shaper target identity is missing")
		}
		seed, err := randomShaperSeed()
		if err != nil {
			return stressShapers{}, err
		}
		config := stressShaperConfig{SchemaVersion: "carrier-lab-native-tool-role/v1",
			RunID: "s43-" + phase + "-" + role, Role: role, Mode: "shape",
			Profile: "h3-s43-impaired-v1", Seed: seed}
		raw, err := json.Marshal(config)
		if err != nil {
			return stressShapers{}, err
		}
		roleRoot := filepath.Join(root, targetRole)
		for _, name := range []string{"config", "evidence", "capture", "control"} {
			if err := os.MkdirAll(filepath.Join(roleRoot, name), 0o777); err != nil {
				return stressShapers{}, err
			}
		}
		if err := os.WriteFile(filepath.Join(roleRoot, "config", "config.json"), raw, 0o644); err != nil {
			return stressShapers{}, err
		}
		digest := sha256.Sum256(raw)
		result.values[index] = shaperEvidence{Role: role, TargetContainer: targets[targetRole],
			ToolImageID: observer.input.ToolImage, ConfigDigest: hex.EncodeToString(digest[:]), Config: raw}
	}
	return result, nil
}

func (run *stressShapers) start(ctx context.Context) error {
	profile := "s43-candidate"
	if run.services[0] == "direct-shape-client" {
		profile = "s43-direct"
	}
	arguments := append([]string{"--profile", profile, "up", "-d"}, run.services[:]...)
	if _, err := run.observer.compose(ctx, time.Minute, arguments...); err != nil {
		return err
	}
	for index, service := range run.services {
		identity, err := run.observer.serviceID(ctx, service)
		if err != nil {
			return err
		}
		projection, err := run.observer.inspectReplacementObserver(ctx, identity)
		if err != nil {
			return err
		}
		run.values[index].ContainerID, run.values[index].Observer = identity, projection
		ready := filepath.Join(run.observer.shaperRoot, []string{"client", "publisher"}[index], "evidence", "ready.json")
		if err := waitStressFile(ctx, ready, 20*time.Second); err != nil {
			return fmt.Errorf("wait for %s readiness: %w", service, err)
		}
		run.values[index].ReadyObservedAtNanos = max(int64(1), time.Since(run.clock).Nanoseconds())
	}
	return nil
}

func (run *stressShapers) finish(ctx context.Context) ([]shaperEvidence, error) {
	for _, role := range []string{"client", "publisher"} {
		path := filepath.Join(run.observer.shaperRoot, role, "control", "stop")
		if err := os.WriteFile(path, []byte("stop\n"), 0o666); err != nil {
			return nil, err
		}
	}
	for index, service := range run.services {
		if err := run.observer.waitContainerFor(ctx, run.values[index].ContainerID, true, time.Minute); err != nil {
			return nil, err
		}
		run.values[index].CompletedAtNanos = max(run.values[index].ReadyObservedAtNanos+1,
			time.Since(run.clock).Nanoseconds())
		path := filepath.Join(run.observer.shaperRoot, []string{"client", "publisher"}[index], "evidence", "result.json")
		raw, err := byteio.ReadFile(path, 64<<10)
		if err != nil {
			return nil, err
		}
		run.values[index].Result = raw
		if _, err := run.observer.compose(ctx, time.Minute, "rm", "-f", service); err != nil {
			return nil, err
		}
		if _, err := run.observer.docker(ctx, 10*time.Second, "inspect", run.values[index].ContainerID); err == nil {
			return nil, errors.New("S4.3 shaper remained after removal")
		}
		run.values[index].Removed = true
	}
	return append([]shaperEvidence(nil), run.values[:]...), nil
}

func randomShaperSeed() (uint32, error) {
	var raw [4]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return 0, err
	}
	value := binary.BigEndian.Uint32(raw[:])
	if value == 0 {
		value = 1
	}
	return value, nil
}

func waitStressFile(ctx context.Context, path string, limit time.Duration) error {
	deadline := time.Now().Add(limit)
	for time.Now().Before(deadline) {
		if info, err := os.Stat(path); err == nil && info.Size() > 0 {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
	return errors.New("bounded evidence file did not appear")
}
