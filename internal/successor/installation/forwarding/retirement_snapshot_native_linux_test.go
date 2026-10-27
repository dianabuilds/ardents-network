//go:build network_signing_native

package forwarding

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/successor/installation/completion"
)

// Actual native files only. All process/manager/exit/frame values below are
// detached grammar fixtures, NEVER supplied to a live Stop, Startup or native
// process/scope owner. Recorded phase names do not claim those events occurred.
func nativeRetirementSnapshotFixture(t *testing.T) (*retirementStorage, binding, string, string, startArchive, [16]byte) {
	t.Helper()
	s := nativeRetirementStorage(t, t.Context())
	b := binding{UID: 988, GID: 988, Generation: strings.Repeat("a", 64), Request: Request{InstallationRoot: filepath.Dir(s.path)}}
	bh, sh := digest([]byte("physical binding fixture only")), digest([]byte("physical archive fixture only"))
	id, challenge := [16]byte{9}, [32]byte{7}
	a := startArchive{Schema: "ardents-forwarding-start-archive-v1", Generation: b.Generation, BindingSHA256: bh, PID: 7317, InvocationID: hex.EncodeToString(id[:]), Manager: "detached-fixture-no-manager", StartedRealtime: 30, StartedMonotonic: 40}
	if err := s.birthDirectory(988, s.check); err != nil {
		t.Fatal(err)
	}
	if _, err := s.write("retirement-pending", []byte("ardents-forwarding-retirement-pending-v1\n"+sh+"\n"), s.check); err != nil {
		t.Fatal(err)
	}
	if err := s.prepareSocket(s.check); err != nil {
		t.Fatal(err)
	}
	start, err := completion.Encode(id, b.Generation, bh)
	if err != nil {
		t.Fatal(err)
	}
	frame := retirementFrame(start, challenge)
	r := retirementResult{Schema: "ardents-forwarding-retirement-v1", Generation: b.Generation, BindingSHA256: bh, StartSHA256: sh, InvocationID: a.InvocationID, PID: a.PID, Manager: a.Manager, Challenge: hex.EncodeToString(challenge[:]), CompletionSHA256: digest(frame[:]), StartedRealtime: 30, StartedMonotonic: 40, ExitedRealtime: 50, ExitedMonotonic: 60, Code: 1, Status: 0}
	raw, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	raw = append(raw, '\n')
	for _, item := range []struct{ phase, hash string }{{"interrupt-attempt", digest(challenge[:])}, {"domain-joined", digest(frame[:])}, {"native-joined", digest(raw)}, {"native-closed", digest(raw)}} {
		if err := s.record(item.phase, s.path, nil, item.hash); err != nil {
			t.Fatal(err)
		}
	}
	result, err := s.write("retirement-result.json", raw, s.check)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.record("retired", result.path, result.info, digest(raw)); err != nil {
		t.Fatal(err)
	}
	return s, b, bh, sh, a, id
}

func TestForwardingRetirementSnapshotNativeReadbackAndCommittedLeaseAreDistinct(t *testing.T) {
	s, b, bh, sh, a, id := nativeRetirementSnapshotFixture(t)
	readback, err := readRetirementSnapshot(t.Context(), b, 0, bh, sh, a, id, retirementReadback)
	defer readback.close()
	if err != nil {
		t.Fatal("actual still-blocked file readback", err)
	}
	if readback.pending == nil || readback.result.PID != 7317 || readback.sha256 == "" {
		t.Fatal("physical snapshot facts differ")
	}
	other, err := readRetirementSnapshot(t.Context(), b, 0, bh, sh, a, id, retirementCommitted)
	if !errors.Is(err, syscall.EWOULDBLOCK) {
		t.Fatal("writer lease borrowed", err)
	}
	if err := other.close(); !errors.Is(err, syscall.EWOULDBLOCK) {
		t.Fatal("original exclusion failure lost", err)
	}
	if err := readback.close(); err != nil {
		t.Fatal(err)
	}
	if err := s.close(); err != nil {
		t.Fatal(err)
	}
	other, err = readRetirementSnapshot(t.Context(), b, 0, bh, sh, a, id, retirementCommitted)
	if !errors.Is(err, ErrRepairRequired) {
		t.Fatal("blocked proof admitted as committed", err)
	}
	_ = other.close()
	// Remove only our own detached fixture marker, never an installed blocker.
	if err := os.Remove(filepath.Join(s.path, "retirement-pending")); err != nil {
		t.Fatal(err)
	}
	h, err := readRetirementSnapshot(t.Context(), b, 0, bh, sh, a, id, retirementCommitted)
	defer h.close()
	if err != nil {
		t.Fatal("actual committed-file custody", err)
	}
	probe, err := os.Open(h.journal.path)
	if err != nil {
		t.Fatal(err)
	}
	defer probe.Close()
	if err := syscall.Flock(int(probe.Fd()), syscall.LOCK_SH|syscall.LOCK_NB); !errors.Is(err, syscall.EWOULDBLOCK) {
		t.Fatal("independent reader lease absent", err)
	}
	if err := h.close(); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Flock(int(probe.Fd()), syscall.LOCK_SH|syscall.LOCK_NB); err != nil {
		t.Fatal("joined reader retained lease", err)
	}
}

func TestForwardingRetirementSnapshotNativePartialAndSubstitutedResourcesRefuse(t *testing.T) {
	for _, mode := range []string{"partial journal", "substituted result", "substituted directory", "missing blocker"} {
		t.Run(mode, func(t *testing.T) {
			s, b, bh, sh, a, id := nativeRetirementSnapshotFixture(t)
			if err := s.close(); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(s.path, "retirement-result.json")
			switch mode {
			case "partial journal":
				info, err := os.Stat(s.journal.path)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.Truncate(s.journal.path, info.Size()-1); err != nil {
					t.Fatal(err)
				}
			case "substituted result":
				raw, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.Rename(path, path+".original"); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, raw, 0600); err != nil {
					t.Fatal(err)
				}
				if err := os.Remove(path + ".original"); err != nil {
					t.Fatal(err)
				}
			case "substituted directory":
				if err := os.Rename(s.path, s.path+".original"); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(s.path, 0750); err != nil {
					t.Fatal(err)
				}
				if err := os.Chown(s.path, 0, 988); err != nil {
					t.Fatal(err)
				}
			case "missing blocker":
				if err := os.Remove(filepath.Join(s.path, "retirement-pending")); err != nil {
					t.Fatal(err)
				}
			}
			h, err := readRetirementSnapshot(t.Context(), b, 0, bh, sh, a, id, retirementReadback)
			if err == nil {
				t.Fatal("partial/substituted native proof read", mode)
			}
			if h.close() == nil {
				t.Fatal("original refusal lost", mode)
			}
		})
	}
}

func TestForwardingRetirementSnapshotNativeCloseRetainsLeaseAndLateFailure(t *testing.T) {
	s, b, bh, sh, a, id := nativeRetirementSnapshotFixture(t)
	if err := s.close(); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(s.path, "retirement-pending")); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	h, err := readRetirementSnapshot(ctx, b, 0, bh, sh, a, id, retirementCommitted)
	if err != nil {
		t.Fatal(err)
	}
	defer h.close()
	probe, err := os.Open(h.journal.path)
	if err != nil {
		t.Fatal(err)
	}
	defer probe.Close()
	held, release := make(chan struct{}), make(chan struct{})
	h.observe = func(phase string, file *os.File) {
		if phase == "before-close" && file != h.journal.file {
			close(held)
			<-release
			_ = file.Close()
		}
	}
	done := make(chan error, 1)
	go func() { done <- h.close() }()
	<-held
	cancel()
	if err := syscall.Flock(int(probe.Fd()), syscall.LOCK_SH|syscall.LOCK_NB); !errors.Is(err, syscall.EWOULDBLOCK) {
		t.Fatal("held original Close returned lease", err)
	}
	close(release)
	actual := <-done
	if !errors.Is(actual, context.Canceled) || !errors.Is(actual, os.ErrClosed) {
		t.Fatal("cancellation hid late physical close failure", actual)
	}
	if err := h.close(); !errors.Is(err, context.Canceled) || !errors.Is(err, os.ErrClosed) {
		t.Fatal("repeated close lost terminal cause", err)
	}
	if err := syscall.Flock(int(probe.Fd()), syscall.LOCK_SH|syscall.LOCK_NB); err != nil {
		t.Fatal("joined close retained lease", err)
	}
}
