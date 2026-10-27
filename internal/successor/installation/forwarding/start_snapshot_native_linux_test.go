//go:build network_signing_native

package forwarding

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/successor/installation/completion"
)

// readStartSnapshot selects the first attempt for native physical custody tests.
func readStartSnapshot(ctx context.Context, b binding, bindingSHA, initialSHA string) (h *startSnapshot, returned error) {
	return readStartSnapshotAt(ctx, b, 0, bindingSHA, initialSHA, "")
}

// This fixture produces real native files/socket/journal only. Its detached
// archive contains no actual process/manager/domain observation. It is never
// passed to ReadStarted, StartInitial or a live process/Stop/retirement owner.
// Tests establish physical file custody and grammar, not installed acceptance.
func nativeStartSnapshotFixture(t *testing.T) (binding, string, string) {
	return nativeStartSnapshotAtFixture(t, 0, "")
}

func nativeStartSnapshotAtFixture(t *testing.T, number attemptNumber, predecessor string) (binding, string, string) {
	t.Helper()
	_ = prepareInputFixture(t)                        // actual dedicated native stand and ext4 prerequisite
	parent, err := os.MkdirTemp(os.TempDir(), "fss-") // keep the genuine Unix address below its native bound
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(parent); err != nil {
			t.Error(err)
		}
	})
	root := filepath.Join(parent, "installation")
	if err := os.Mkdir(root, 0755); err != nil {
		t.Fatal(err)
	}
	s, err := createStartStorage(t.Context(), root, number, digest([]byte("independent physical fixture, no installed authority")))
	if err != nil {
		_ = s.close()
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.close() })
	if err := s.birthDirectory(988, s.check); err != nil {
		t.Fatal(err)
	}
	bind, initial := digest([]byte("independent physical fixture, no installed authority")), digest([]byte("physical initial history fixture"))
	b := binding{UID: 988, GID: 988, Generation: strings.Repeat("a", 64), Request: Request{InstallationRoot: filepath.Dir(s.path)}}
	write := func(name string, value any) *initialFile {
		raw, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		file, err := s.write(name, append(raw, '\n'), s.check)
		if err != nil {
			t.Fatal(err)
		}
		return file
	}
	if _, err := s.write("forwarding-start-pending", []byte("ardents-forwarding-start-pending-v1\n"+bind+"\n"), s.check); err != nil {
		t.Fatal(err)
	}
	var intent any = startIntent{Schema: "ardents-forwarding-start-intent-v1", BindingSHA256: bind, InitialHistorySHA256: initial, TargetsSHA256: digest([]byte("detached physical targets fixture")), UID: 988, GID: 988}
	if number != 0 {
		intent = invocationIntent{"ardents-forwarding-invocation-intent-v1", bind, initial, digest([]byte("detached physical targets fixture")), 988, 988, uint32(number), predecessor}
	}
	guard := write("start-guard.json", intent)
	socketPath := filepath.Join(s.path, "start-completion.socket")
	if err := s.record("socket-intent", socketPath, nil, ""); err != nil {
		t.Fatal(err)
	}
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: socketPath, Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}
	listener.SetUnlinkOnClose(false)
	t.Cleanup(func() { _ = listener.Close() })
	born, err := os.Lstat(socketPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.record("socket-born", socketPath, born, ""); err != nil {
		t.Fatal(err)
	}
	if err := os.Lchown(socketPath, 0, 988); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(socketPath, 0660); err != nil {
		t.Fatal(err)
	}
	socket, err := os.Lstat(socketPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.dir.Sync(); err != nil {
		t.Fatal(err)
	}
	if err := s.record("socket-ready", socketPath, socket, ""); err != nil {
		t.Fatal(err)
	}
	native := socket.Sys().(*syscall.Stat_t)
	write("start-socket.json", startSocketRecord{"ardents-forwarding-start-socket-v1", digest(guard.body), uint64(native.Dev), native.Ino, 988})
	id := [16]byte{9, byte(number)}
	frame, err := completion.Encode(id, b.Generation, bind)
	if err != nil {
		t.Fatal(err)
	}
	for _, phase := range []string{"start-attempt", "invocation-observed", "completion-received"} {
		hash := digest(guard.body)
		if phase == "completion-received" {
			hash = digest(frame[:])
		}
		if err := s.record(phase, s.path, nil, hash); err != nil {
			t.Fatal(err)
		}
	}
	archive := write("start-archived.json", startArchive{Schema: "ardents-forwarding-start-archive-v1", GuardSHA256: digest(guard.body), BindingSHA256: bind, Generation: b.Generation, PID: 7317, InvocationID: hex.EncodeToString(id[:]), ProcessDevice: 17, ProcessInode: 31, CgroupDevice: 19, CgroupInode: 37, Manager: "physical-fixture-no-live-manager", StartedRealtime: 30, StartedMonotonic: 40})
	if err := s.record("archived", archive.path, archive.info, digest(archive.body)); err != nil {
		t.Fatal(err)
	}
	for _, phase := range []string{"ack-attempt", "ack-written"} {
		if err := s.record(phase, s.path, nil, digest(frame[:])); err != nil {
			t.Fatal(err)
		}
	}
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	if err := s.close(); err != nil {
		t.Fatal(err)
	}
	// Physical fixture publication only; no root producer or domain/ACK result
	// is manufactured by this explicit removal of our own fixture marker.
	if err := os.Remove(filepath.Join(s.path, "forwarding-start-pending")); err != nil {
		t.Fatal(err)
	}
	return b, bind, initial
}

func TestForwardingStartSnapshotNativeNonzeroRequiresOriginalPredecessor(t *testing.T) {
	for _, number := range []attemptNumber{1, attemptLimit - 1} {
		predecessor := digest([]byte("detached predecessor retirement fixture only"))
		b, bh, ih := nativeStartSnapshotAtFixture(t, number, predecessor)
		h, err := readStartSnapshotAt(t.Context(), b, number, bh, ih, predecessor)
		if err != nil {
			_ = h.close()
			t.Fatal("independent nonzero physical grammar", number, err)
		}
		if h.number != number || h.predecessor != predecessor || h.id != ([16]byte{9, byte(number)}) {
			t.Fatal("nonzero snapshot facts differ")
		}
		if err := h.close(); err != nil {
			t.Fatal(err)
		}
		wrong, err := readStartSnapshotAt(t.Context(), b, number, bh, ih, bh)
		if !errors.Is(err, ErrBinding) {
			_ = wrong.close()
			t.Fatal("substituted predecessor admitted", err)
		}
		if !errors.Is(wrong.close(), ErrBinding) {
			t.Fatal("original predecessor refusal lost")
		}
		first, err := readStartSnapshot(t.Context(), b, bh, ih)
		if !errors.Is(err, os.ErrNotExist) {
			_ = first.close()
			t.Fatal("nonzero files supplied missing first history", err)
		}
		_ = first.close()
	}
}

func TestForwardingStartSnapshotNativeFullRetainedDirectoryBound(t *testing.T) {
	_ = prepareInputFixture(t)
	root, err := os.MkdirTemp(os.TempDir(), "fchain-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(root); err != nil {
			t.Error(err)
		}
	})
	for n := attemptNumber(0); n < 64; n++ {
		for _, path := range []string{n.directory(root), n.retirement(root)} {
			if err := os.Mkdir(path, 0750); err != nil {
				t.Fatal(err)
			}
		}
	}
	for _, name := range []string{"binding.json", "generations"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("detached inventory fixture"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	file, err := os.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	entries, err := historyDirectoryEntries(&inputDirectory{file: file})
	if err != nil || len(entries) != 130 {
		t.Fatal("all64 retained starts and retirements exceeded discovery bound", len(entries), err)
	}
	var names []string
	for _, entry := range entries {
		if entry.Name() == "forwarding-startup" || strings.HasPrefix(entry.Name(), "forwarding-invocation-") {
			names = append(names, entry.Name())
		}
	}
	last, err := attemptDirectories(names)
	if err != nil || last != 63 {
		t.Fatal("finite complete directory discovery", last, err)
	}
}

func TestForwardingStartSnapshotNativeOriginalLeasesAndMetadata(t *testing.T) {
	b, bind, initial := nativeStartSnapshotFixture(t)
	h, err := readStartSnapshot(t.Context(), b, bind, initial)
	defer h.close()
	if err != nil {
		t.Fatal("independent physical transcript snapshot", err)
	}
	if err := h.check(); err != nil {
		t.Fatal(err)
	}
	if h.archive.PID != 7317 || h.id != [16]byte{9} || len(h.order) != 4 || h.sha256 == "" {
		t.Fatal("detached transcript facts differ")
	}
	probe, err := os.Open(h.journal.path)
	if err != nil {
		t.Fatal(err)
	}
	defer probe.Close()
	if err := syscall.Flock(int(probe.Fd()), syscall.LOCK_SH|syscall.LOCK_NB); !errors.Is(err, syscall.EWOULDBLOCK) {
		t.Fatal("independent original journal not held", err)
	}
	other, err := readStartSnapshot(t.Context(), b, bind, initial)
	if !errors.Is(err, syscall.EWOULDBLOCK) {
		_ = other.close()
		t.Fatal("second reader borrowed original lease", err)
	}
	if err := other.close(); !errors.Is(err, syscall.EWOULDBLOCK) {
		t.Fatal("failed independent lease acquisition lost", err)
	}
	if err := h.close(); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Flock(int(probe.Fd()), syscall.LOCK_SH|syscall.LOCK_NB); err != nil {
		t.Fatal("joined reader retained lease", err)
	}
	if err := h.check(); err == nil {
		t.Fatal("closed original snapshot revived")
	}
	if err := h.close(); err != nil {
		t.Fatal("late check changed clean retained close", err)
	}
}

func TestForwardingStartSnapshotNativePendingAndPartialHistoryRefuse(t *testing.T) {
	for _, mode := range []string{"pending", "partial"} {
		t.Run(mode, func(t *testing.T) {
			b, bind, initial := nativeStartSnapshotFixture(t)
			if mode == "pending" {
				if err := os.WriteFile(filepath.Join(attemptNumber(0).directory(b.Request.InstallationRoot), "forwarding-start-pending"), []byte("uncertain fixture publication\n"), 0600); err != nil {
					t.Fatal(err)
				}
			} else {
				path := attemptNumber(0).journal(b.Request.InstallationRoot)
				raw, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, raw[:len(raw)-1], 0600); err != nil {
					t.Fatal(err)
				}
			}
			h, err := readStartSnapshot(t.Context(), b, bind, initial)
			if err == nil {
				_ = h.close()
				t.Fatal("uncertain/partial start became completed history")
			}
			if closeErr := h.close(); closeErr == nil {
				t.Fatal("original snapshot refusal lost")
			}
		})
	}
}

func TestForwardingStartSnapshotNativeSubstitutionLatchesAndReturnsOriginalLease(t *testing.T) {
	b, bind, initial := nativeStartSnapshotFixture(t)
	h, err := readStartSnapshot(t.Context(), b, bind, initial)
	if err != nil {
		t.Fatal(err)
	}
	defer h.close()
	path := filepath.Join(h.path, "start-guard.json")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(path, path+"-original"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, before, 0600); err != nil {
		t.Fatal(err)
	}
	if err := h.check(); err == nil {
		t.Fatal("equal bytes at a substitute inode admitted")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(path+"-original", path); err != nil {
		t.Fatal(err)
	}
	if err := h.check(); err == nil {
		t.Fatal("restoration revived failed original snapshot")
	}
	if err := h.close(); err == nil {
		t.Fatal("original native loss erased at close")
	}
	probe, err := os.Open(h.journal.path)
	if err != nil {
		t.Fatal(err)
	}
	defer probe.Close()
	if err := syscall.Flock(int(probe.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		t.Fatal("actual failed snapshot did not return its own lease after close", err)
	}
}

func TestForwardingStartSnapshotNativePhysicalCloseKeepsLeaseUntilLast(t *testing.T) {
	b, bind, initial := nativeStartSnapshotFixture(t)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	h, err := readStartSnapshot(ctx, b, bind, initial)
	if err != nil {
		t.Fatal(err)
	}
	defer h.close()
	entered, release := make(chan struct{}), make(chan struct{})
	once := false
	h.observe = func(phase string, file *os.File) {
		if phase == "before-close" && file != h.journal.file && !once {
			once = true
			close(entered)
			<-release
			_ = file.Close()
		}
	}
	probe, err := os.Open(h.journal.path)
	if err != nil {
		t.Fatal(err)
	}
	defer probe.Close()
	done := make(chan error, 1)
	go func() { done <- h.close() }()
	<-entered
	cancel()
	if err := syscall.Flock(int(probe.Fd()), syscall.LOCK_SH|syscall.LOCK_NB); !errors.Is(err, syscall.EWOULDBLOCK) {
		close(release)
		<-done
		t.Fatal("lease released before original descriptor close joined", err)
	}
	close(release)
	if err := <-done; !errors.Is(err, context.Canceled) || !errors.Is(err, os.ErrClosed) {
		t.Fatal("late physical/caller close causes lost", err)
	}
	if err := syscall.Flock(int(probe.Fd()), syscall.LOCK_SH|syscall.LOCK_NB); err != nil {
		t.Fatal("original journal lease unavailable after actual join", err)
	}
	if err := h.close(); !errors.Is(err, context.Canceled) || !errors.Is(err, os.ErrClosed) {
		t.Fatal("retained close outcome changed", err)
	}
}
