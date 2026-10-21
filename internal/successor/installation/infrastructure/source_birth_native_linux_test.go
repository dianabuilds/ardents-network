//go:build linux && network_signing_native

package infrastructure

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
)

// This isolates filesystem/manager mechanisms, never Enrollment/Release rights.
// The compiled consumer separately proves the full genuine transition.
func TestSourceStoppedNativeReloadKeepsInputAncestorsOriginal(t *testing.T) {
	births, path := nativeSourceBirths(t, t.Context())
	defer births.close()
	base := filepath.Dir(path)
	inputInfo, e := os.Lstat(base)
	if e != nil {
		t.Fatal(e)
	}
	const generator = "/run/systemd/generator"
	old, e := os.Open(generator)
	if e != nil {
		t.Fatal("invalid environment: actual manager generator output required", e)
	}
	defer old.Close()
	before, e := old.Stat()
	if e != nil {
		t.Fatal(e)
	}
	p := &sourceInputs{ctx: t.Context(), ancestors: map[string]os.FileInfo{base: inputInfo}, namespaceAncestors: map[string]os.FileInfo{generator: before}}
	input := SourceInputs{original: p}
	if e := input.Check(); e != nil {
		t.Fatal(e)
	}
	if out, e := exec.CommandContext(t.Context(), "/usr/bin/systemctl", "daemon-reload").CombinedOutput(); e != nil {
		t.Fatal("actual private manager reload", string(out), e)
	}
	phase, e := input.observeReloadedNamespace()
	if e != nil || os.SameFile(before, phase[generator]) {
		t.Fatal("actual reload did not replace the pinned generator output", e)
	}
	if e := input.checkSourcePhase(nil, phase); e != nil {
		t.Fatal("separate post-reload namespace refused", e)
	}
	strict := SourceInputs{original: &sourceInputs{ctx: t.Context(), namespaceAncestors: map[string]os.FileInfo{generator: before}}}
	if strict.Check() == nil || strict.checkSourcePhase(nil, phase) == nil {
		t.Fatal("read-only original namespace or retained failure rebound")
	}
	if e := os.Rename(base, base+".original"); e != nil {
		t.Fatal(e)
	}
	if e := os.Mkdir(base, 0700); e != nil {
		t.Fatal(e)
	}
	if input.checkSourcePhase(nil, phase) == nil {
		t.Fatal("reload phase adopted a substituted original input ancestor")
	}
}

func nativeSourceBirths(t *testing.T, ctx context.Context) (*sourceBirths, string) {
	t.Helper()
	marker, e := os.ReadFile("/etc/ardents-source-stopped-native-stand")
	if e != nil || string(marker) != "ardents-source-stopped-native-v1\n" || os.Geteuid() != 0 {
		t.Fatal("invalid environment: own private native stopped stand required", e)
	}
	base, e := os.MkdirTemp(os.Getenv("ARDENTS_SOURCE_STOPPED_FUTURE_PARENT"), "retained-birth-")
	if e != nil {
		t.Fatal(e)
	}
	var fs syscall.Statfs_t
	if e = syscall.Statfs(base, &fs); e != nil || fs.Type != 0xef53 {
		t.Fatal("invalid environment: actual native ext4 required", e)
	}
	path := filepath.Join(base, "credential")
	s, e := openSourceBirths(ctx, filepath.Join(base, "journal"), map[string]bool{path: true, filepath.Join(base, "late"): true})
	if e != nil {
		t.Fatal(e)
	}
	t.Log("retained private native birth fixture", base)
	return s, path
}

func TestSourceStoppedNativeBornInodeSubstitutionAndLateClose(t *testing.T) {
	s, path := nativeSourceBirths(t, t.Context())
	substituted := false
	e := s.write(path, []byte("never disclose this runtime key"), 0600, 123, 123, func() error {
		if _, e := os.Lstat(path); e == nil && !substituted {
			substituted = true
			if e := os.Rename(path, path+".original"); e != nil {
				return e
			}
			return os.WriteFile(path, []byte("foreign"), 0600)
		}
		return nil
	})
	if e == nil {
		t.Fatal("journal-born inode substitution accepted")
	}
	if raw, e := os.ReadFile(path + ".original"); e != nil || len(raw) != 0 {
		t.Fatal("private bytes written to denied original", e)
	}
	if raw, e := os.ReadFile(path); e != nil || string(raw) != "foreign" {
		t.Fatal("foreign replacement overwritten", e)
	}
	if e = s.files[0].file.Close(); e != nil {
		t.Fatal(e)
	}
	closed := s.close()
	if !errors.Is(closed, ErrBinding) || !errors.Is(closed, os.ErrClosed) {
		t.Fatal("first/physical close failure lost", closed)
	}
	if s.close() != closed {
		t.Fatal("repeat close changed retained outcome")
	}
	if e = s.write(filepath.Join(filepath.Dir(path), "late"), []byte("late"), 0444, 0, 0, func() error { return nil }); e == nil {
		t.Fatal("failed/closed birth custody revived")
	}
}

func TestSourceStoppedNativeSyncUncertaintyRetainsBytesAndDenial(t *testing.T) {
	s, path := nativeSourceBirths(t, t.Context())
	defer s.close()
	uncertain := errors.New("injected uncertainty after actual fsync")
	s.afterSync = func(f *os.File) error {
		info, e := f.Stat()
		if e != nil {
			return e
		}
		if f.Name() == path && info.Size() > 0 {
			return uncertain
		}
		return nil
	}
	body := []byte("exact protected runtime bytes")
	if e := s.write(path, body, 0600, 123, 123, func() error { return nil }); !errors.Is(e, uncertain) {
		t.Fatal("uncertain sync became successful", e)
	}
	raw, e := os.ReadFile(path)
	if e != nil || !bytes.Equal(raw, body) {
		t.Fatal("uncertain bytes were removed/reset", e)
	}
	before, e := os.ReadFile(s.journal.path)
	if e != nil {
		t.Fatal(e)
	}
	if bytes.Contains(before, []byte(`"phase":"file-ready"`)) {
		t.Fatal("uncertain sync advertised ready")
	}
	s.afterSync = nil
	if !errors.Is(s.check(), uncertain) {
		t.Fatal("restored native counterpart revived failed owner")
	}
	if !errors.Is(s.close(), uncertain) {
		t.Fatal("close lost original uncertainty")
	}
	after, e := os.ReadFile(s.journal.path)
	if e != nil || !bytes.Equal(before, after) {
		t.Fatal("Close rewrote uncertain prefix", e)
	}
}

func TestSourceStoppedNativeActualPartialWriteAndOriginalCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	s, path := nativeSourceBirths(t, ctx)
	defer s.close()
	var limit syscall.Rlimit
	if e := syscall.Getrlimit(syscall.RLIMIT_FSIZE, &limit); e != nil {
		t.Fatal(e)
	}
	limited := false
	e := s.write(path, []byte("longer than the admitted native file-size limit"), 0600, 123, 123, func() error {
		if _, e := os.Lstat(path); e == nil && !limited {
			limited = true
			return syscall.Setrlimit(syscall.RLIMIT_FSIZE, &syscall.Rlimit{Cur: 4, Max: limit.Max})
		}
		return nil
	})
	if restore := syscall.Setrlimit(syscall.RLIMIT_FSIZE, &limit); restore != nil {
		t.Fatal(restore)
	}
	if e == nil {
		t.Fatal("actual partial file write accepted")
	}
	info, statErr := os.Lstat(path)
	if statErr != nil || info.Size() != 4 {
		t.Fatal("actual partial bytes not retained", statErr)
	}
	cancel()
	closed := s.close()
	if !errors.Is(closed, context.Canceled) {
		t.Fatal("original cancellation lost alongside physical failure", closed)
	}
	if _, e = os.Lstat(path); e != nil {
		t.Fatal("partial bytes deleted", e)
	}
}

func TestSourceStoppedNativeAdmissionAfterJournalAndBeforePromotion(t *testing.T) {
	for _, phase := range []string{"file-intent", "directory-intent", "directory-born"} {
		t.Run(phase, func(t *testing.T) {
			s, path := nativeSourceBirths(t, t.Context())
			defer s.close()
			denied := false
			refusal := errors.New("original authority retired during real journal sync")
			s.afterSync = func(f *os.File) error {
				if f == s.journal.file && bytes.Contains(s.journal.body, []byte(`"phase":"`+phase+`"`)) {
					denied = true
				}
				return nil
			}
			admit := func() error {
				if denied {
					return refusal
				}
				return nil
			}
			var e error
			if phase == "file-intent" {
				e = s.write(path, []byte("runtime bytes"), 0444, 123, 123, admit)
			} else {
				e = s.directory(path, 0700, 123, 123, admit)
			}
			if !errors.Is(e, refusal) {
				t.Fatal("post-journal refusal lost", e)
			}
			info, e := os.Lstat(path)
			if phase == "directory-born" {
				if e != nil || !sourceAccess(info, 0700, 0, 0, true) {
					t.Fatal("denied birth promoted access", e)
				}
			} else if !os.IsNotExist(e) {
				t.Fatal("denied intent performed an exclusive birth", e)
			}
		})
	}
	t.Run("written-bytes", func(t *testing.T) {
		s, path := nativeSourceBirths(t, t.Context())
		defer s.close()
		refusal := errors.New("original authority retired after body I/O")
		e := s.write(path, []byte("exact private bytes"), 0444, 123, 123, func() error {
			if info, e := os.Lstat(path); e == nil && info.Size() != 0 {
				return refusal
			}
			return nil
		})
		info, statErr := os.Lstat(path)
		if !errors.Is(e, refusal) || statErr != nil || !sourceAccess(info, 0600, 0, 0, false) || bytes.Contains(s.journal.body, []byte(`"phase":"file-ready"`)) {
			t.Fatal("post-write refusal permitted access promotion or ready record", e, statErr)
		}
	})
}
