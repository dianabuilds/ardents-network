//go:build network_signing_native

package state_test

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/domain/network/source"
	"github.com/dianabuilds/ardents-network/internal/domain/network/source/servingdeclaration"
	"github.com/dianabuilds/ardents-network/internal/domain/network/state"
)

func sourceInspectionNativeFixture(t *testing.T) (state.Config, []byte) {
	t.Helper()
	if os.Geteuid() != 0 {
		t.Fatal("selected Source inspection profile requires Linux root and root-controlled native TMPDIR")
	}
	genesis := newFixture(t)
	cfg := installedServingSourceConfig(t, genesis)
	// t.TempDir creates 0755 on the current toolchain; the production State
	// root is owner-only (0700), so bind the fixture to that invariant rather
	// than the temp-directory default, as the Node inspection fixture does.
	if err := os.Chmod(cfg.Root, 0o700); err != nil {
		t.Fatal(err)
	}
	serving, err := state.Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := serving.Close(); err != nil {
		t.Fatal(err)
	}
	cfg.Source = source.Config{}
	cfg.ClockObservationFile = filepath.Join(t.TempDir(), "clock")
	if err := os.WriteFile(cfg.ClockObservationFile, []byte("independent clock observation"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(cfg.ClockObservationFile, cfg.Now, cfg.Now); err != nil {
		t.Fatal(err)
	}
	// Native numeric ownership is observed from actual retained roots, rather
	// than supplied by a successful neighbor. No runtime user is launched here.
	setSourceInspectionNativeOwner(t, cfg)
	n := &servingdeclaration.ServingNetwork{Root: cfg.Root, NetworkID: cfg.NetworkID, Threshold: cfg.Threshold, ClockObservationFile: cfg.ClockObservationFile}
	copy(n.ProfileAuthority[:], cfg.ClosedProfileAuthority)
	for _, public := range cfg.Authorities {
		var value [32]byte
		copy(value[:], public)
		n.Authorities = append(n.Authorities, value)
	}
	plan := servingdeclaration.ServingDeclaration{Network: n, ParticipationRoot: cfg.LocalRoleStateRoot, ProcessProfile: "source", Address: "127.0.0.1:12345", Certificate: "/certificate", PrivateKey: "/private-key", ClientRoot: "/client-root", ClientPins: [][32]byte{{1}}}
	raw, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	return cfg, raw
}

func setSourceInspectionNativeOwner(t *testing.T, cfg state.Config) {
	t.Helper()
	for _, root := range []string{cfg.Root, cfg.LocalRoleStateRoot} {
		if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			return os.Chown(path, 61100, 61100)
		}); err != nil {
			t.Fatal(err)
		}
	}
}

func TestSourceInspectionNativeStaleMirrorRefusesWithoutRepair(t *testing.T) {
	cfg, _ := sourceInspectionNativeFixture(t)
	genesis := newFixture(t)
	successor := nextFixture(t, genesis)
	owner, err := state.OpenRetained(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = owner.Close() })
	if _, err := owner.AcceptEpoch(t.Context(), successor.epoch, successor.inputs, successor.materializations); err != nil {
		t.Fatal(err)
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	setSourceInspectionNativeOwner(t, cfg)
	inspection, err := state.OpenSourceInspection(ctx, cfg)
	if err != nil {
		t.Fatalf("inspection of intact successor: %v", err)
	}
	if err := inspection.Close(); err != nil {
		t.Fatal(err)
	}

	stale := fmt.Sprintf("%x\n", genesis.epochDigest)
	currentPath := filepath.Join(cfg.Root, "current")
	if err := os.WriteFile(currentPath, []byte(stale), 0600); err != nil {
		t.Fatal(err)
	}
	before := sourceInspectionNativeImages(t, cfg)
	refused, err := state.OpenSourceInspection(ctx, cfg)
	if err == nil {
		_ = refused.Close()
	}
	if err == nil || !strings.Contains(err.Error(), "retained State security history requires explicit recovery") {
		t.Fatalf("stale mirror inspection: %v", err)
	}
	after := sourceInspectionNativeImages(t, cfg)
	if len(after) != len(before) {
		t.Fatal("stale mirror inspection changed retained inventory")
	}
	for path, original := range before {
		actual, ok := after[path]
		if !ok || original.digest != actual.digest || !os.SameFile(original.info, actual.info) {
			t.Fatalf("stale mirror inspection touched %s", path)
		}
	}
}

func TestSourceInspectionNativeReadOnlyOriginalLifetime(t *testing.T) {
	cfg, raw := sourceInspectionNativeFixture(t)
	before := sourceInspectionNativeImages(t, cfg)
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	inspection, err := state.OpenSourceInspection(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	copy := inspection
	if err := copy.Check(ctx, raw, 61100, 61100); err != nil {
		t.Fatal(err)
	}
	if owner, err := state.OpenRetained(cfg); err == nil {
		_ = owner.Close()
		t.Fatal("read-only inspection did not retain its own existing State lease")
	}
	var users sync.WaitGroup
	for range 4 {
		users.Add(1)
		go func() {
			defer users.Done()
			for range 4 {
				_ = copy.Check(ctx, raw, 61100, 61100)
			}
		}()
	}
	if err := inspection.Close(); err != nil {
		t.Fatal(err)
	}
	users.Wait()
	if err := copy.Check(ctx, raw, 61100, 61100); err == nil {
		t.Fatal("copied inspection survived original Close")
	}
	if err := copy.Close(); err != nil {
		t.Fatal(err)
	}
	// Reopen without GC proves synchronous native lease release. Its semantic
	// currentness remains State-owned; opening supplies no runtime witness.
	next, err := state.OpenSourceInspection(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := next.Close(); err != nil {
		t.Fatal(err)
	}
	after := sourceInspectionNativeImages(t, cfg)
	if len(before) != len(after) {
		t.Fatal("read-only inspection changed retained inventory")
	}
	for path, original := range before {
		actual, ok := after[path]
		if !ok || original.digest != actual.digest || !os.SameFile(original.info, actual.info) || original.info.Mode() != actual.info.Mode() || original.info.Sys().(*syscall.Stat_t).Ctim != actual.info.Sys().(*syscall.Stat_t).Ctim {
			t.Fatal("read-only inspection changed retained native history", path)
		}
	}
}

// Cancellation must deny new observations immediately while the exact
// original clock reader and its leases remain held until physical completion.
func TestSourceInspectionNativeCancellationJoinsOriginalClockReader(t *testing.T) {
	cfg, raw := sourceInspectionNativeFixture(t)
	now := cfg.Now
	cfg.Now = time.Time{}
	entered, resume := make(chan struct{}), make(chan struct{})
	var hold atomic.Bool
	var release sync.Once
	defer release.Do(func() { close(resume) })
	cfg.Clock = func() time.Time {
		if hold.CompareAndSwap(true, false) {
			close(entered)
			<-resume
		}
		return now
	}
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	inspection, err := state.OpenSourceInspection(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = inspection.Close() })
	hold.Store(true)
	checked, closed := make(chan error, 1), make(chan error, 1)
	go func() { checked <- inspection.Check(ctx, raw, 61100, 61100) }()
	select {
	case <-entered:
	case err := <-checked:
		t.Fatal("signed check did not reach original clock read", err)
	case <-ctx.Done():
		t.Fatal("original clock boundary not reached", ctx.Err())
	}
	cancel()
	if err := inspection.Check(ctx, raw, 61100, 61100); !errors.Is(err, context.Canceled) {
		t.Fatal("cancellation did not seal fresh observation", err)
	}
	go func() { closed <- inspection.Close() }()
	fresh, finish := context.WithTimeout(t.Context(), 5*time.Second)
	defer finish()
	foreign, err := state.OpenSourceInspection(fresh, cfg)
	if err == nil {
		_ = foreign.Close()
		t.Fatal("held original reader returned State lease before join")
	}
	select {
	case err := <-closed:
		t.Fatal("Close returned before original clock callback joined", err)
	default:
	}
	release.Do(func() { close(resume) })
	if err := <-checked; !errors.Is(err, context.Canceled) {
		t.Fatal("late observation lost original cancellation", err)
	}
	terminal := <-closed
	if !errors.Is(terminal, context.Canceled) {
		t.Fatal("Close lost original cancellation", terminal)
	}
	if err := inspection.Close(); err != terminal {
		t.Fatal("copied Close changed original terminal result", err)
	}
	next, err := state.OpenSourceInspection(fresh, cfg)
	if err != nil {
		t.Fatal("actual joined readers did not return original leases", err)
	}
	if err := next.Close(); err != nil {
		t.Fatal(err)
	}
}

type sourceInspectionNativeImage struct {
	info   os.FileInfo
	digest [32]byte
}

func sourceInspectionNativeImages(t *testing.T, cfg state.Config) map[string]sourceInspectionNativeImage {
	t.Helper()
	images := map[string]sourceInspectionNativeImage{}
	for _, root := range []string{cfg.Root, cfg.LocalRoleStateRoot} {
		if err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			value := sourceInspectionNativeImage{info: info}
			if !info.IsDir() {
				raw, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				value.digest = sha256.Sum256(raw)
			}
			images[path] = value
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	return images
}

func TestSourceInspectionNativeFailedAcquisitionJoinsWithoutGC(t *testing.T) {
	cfg, _ := sourceInspectionNativeFixture(t)
	path := filepath.Join(cfg.LocalRoleStateRoot, "watermark")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	before := sourceInspectionNativeImages(t, cfg)
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	for range 3 {
		inspection, err := state.OpenSourceInspection(ctx, cfg)
		if err == nil {
			_ = inspection.Close()
			t.Fatal("missing participation floor accepted")
		}
		entries, err := os.ReadDir("/proc/self/fd")
		if err != nil {
			t.Fatal(err)
		}
		for _, entry := range entries {
			target, err := os.Readlink(filepath.Join("/proc/self/fd", entry.Name()))
			if os.IsNotExist(err) {
				continue
			}
			if err != nil {
				t.Fatal(err)
			}
			for _, root := range []string{cfg.Root, cfg.LocalRoleStateRoot} {
				if target == root || strings.HasPrefix(target, root+"/") {
					t.Fatal("failed acquisition retained original descriptor", target)
				}
			}
		}
	}
	after := sourceInspectionNativeImages(t, cfg)
	if len(before) != len(after) {
		t.Fatal("refusal recreated semantic history")
	}
	for path, original := range before {
		actual, ok := after[path]
		if !ok || actual.digest != original.digest || !os.SameFile(actual.info, original.info) {
			t.Fatal("refusal changed original retained history")
		}
	}
}

func TestSourceInspectionNativeLiveOriginalAndExpiredStateRefuse(t *testing.T) {
	cfg, _ := sourceInspectionNativeFixture(t)
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	original, err := state.OpenRetained(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if inspection, err := state.OpenSourceInspection(ctx, cfg); err == nil {
		_ = inspection.Close()
		t.Fatal("live original State lease was borrowed")
	}
	if err := original.Close(); err != nil {
		t.Fatal(err)
	}
	cfg.Now = cfg.Now.Add(48 * time.Hour)
	if err := os.Chtimes(cfg.ClockObservationFile, cfg.Now, cfg.Now); err != nil {
		t.Fatal(err)
	}
	if inspection, err := state.OpenSourceInspection(ctx, cfg); err == nil {
		_ = inspection.Close()
		t.Fatal("expired signed State accepted")
	}
}

func TestSourceInspectionNativeMissingOrChangedHistoryRefuses(t *testing.T) {
	for _, which := range []string{"state-marker", "participation-current", "state-current", "clock", "principal", "same-inode"} {
		t.Run(which, func(t *testing.T) {
			cfg, raw := sourceInspectionNativeFixture(t)
			ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
			defer cancel()
			inspection, err := state.OpenSourceInspection(ctx, cfg)
			if err != nil {
				t.Fatal(err)
			}
			defer inspection.Close()
			uid := uint32(61100)
			switch which {
			case "state-marker":
				if err := os.Remove(filepath.Join(cfg.Root, ".ardents-network-state-v1")); err != nil {
					t.Fatal(err)
				}
			case "participation-current":
				if err := os.Remove(filepath.Join(cfg.LocalRoleStateRoot, "current")); err != nil {
					t.Fatal(err)
				}
			case "state-current":
				if err := os.Remove(filepath.Join(cfg.Root, "current")); err != nil {
					t.Fatal(err)
				}
			case "clock":
				if err := os.Remove(cfg.ClockObservationFile); err != nil {
					t.Fatal(err)
				}
			case "principal":
				uid++
			case "same-inode":
				path := filepath.Join(cfg.Root, "current")
				body, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, body, 0600); err != nil {
					t.Fatal(err)
				}
			}
			if err := inspection.Check(ctx, raw, uid, 61100); err == nil {
				t.Fatal("changed semantic history/principal accepted")
			}
			if err := inspection.Check(ctx, raw, 61100, 61100); err == nil {
				t.Fatal("original refusal was erased")
			}
		})
	}
}
