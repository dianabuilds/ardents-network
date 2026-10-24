//go:build linux && amd64 && network_signing_native

package infrastructure

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func sourceRetirementNativeJSON(t *testing.T, value any) []byte {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return append(raw, '\n')
}

func sourceRetirementNativeSocket(t *testing.T, path string) (os.FileInfo, os.FileInfo) {
	t.Helper()
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := listener.Close(); err != nil {
			t.Error(err)
		}
	})
	listener.SetUnlinkOnClose(false)
	born, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chown(path, 0, 999); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0660); err != nil {
		t.Fatal(err)
	}
	ready, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	return born, ready
}

// Real startup storage/file mechanisms, with unsigned framing only. The
// dummy program, bindings and archive supply no Source/manager authority.
func sourceRetirementNativeStartup(t *testing.T, number sourceAttemptNumber) (*sourceRetirementPhysicalProfile, sourceStartArchive, [160]byte) {
	t.Helper()
	root, err := os.MkdirTemp(os.TempDir(), "ri-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(root); err != nil {
			t.Error(err)
		}
	})
	bindingBody, programBody := []byte("unsigned binding framing only\n"), []byte("unsigned program framing only\n")
	binding, generation := digest(bindingBody), strings.Repeat("2", 64)
	if err := os.WriteFile(filepath.Join(root, "binding.json"), bindingBody, 0644); err != nil {
		t.Fatal(err)
	}
	programDirectory := filepath.Join(root, "generations", generation)
	if err := os.MkdirAll(programDirectory, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(programDirectory, ProgramName), programBody, 0555); err != nil {
		t.Fatal(err)
	}
	journal, directory := number.journal(root), number.directory(root)
	file, err := os.OpenFile(journal, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0600)
	if err != nil {
		t.Fatal(err)
	}
	storage := &sourceStartStorage{ctx: context.Background(), path: directory, number: number, journal: &sourceBirth{path: journal, file: file}}
	t.Cleanup(func() {
		if err := storage.close(); err != nil {
			t.Error(err)
		}
	})
	info, err := file.Stat()
	if err != nil {
		t.Fatal(err)
	}
	storage.journal.info = info
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		t.Fatal(err)
	}
	_, first, intentSchema, archiveSchema := number.schema()
	if err := storage.record(first, journal, info, binding); err != nil {
		t.Fatal(err)
	}
	if err := storage.birthDirectory(999, storage.ctx.Err); err != nil {
		t.Fatal(err)
	}
	if err := storage.preparePending(binding); err != nil {
		t.Fatal(err)
	}
	intent := sourceStartIntent{Schema: intentSchema, BindingSHA256: binding, InitialHistorySHA256: strings.Repeat("3", 64), TargetsSHA256: strings.Repeat("4", 64), UID: 999, GID: 999, Attempt: uint32(number)}
	if number != 0 {
		intent.PredecessorSHA256 = strings.Repeat("5", 64)
	}
	guard, err := storage.write("start-guard.json", sourceRetirementNativeJSON(t, intent), storage.ctx.Err)
	if err != nil {
		t.Fatal(err)
	}
	socketPath := filepath.Join(directory, "start-completion.socket")
	if err := storage.record("socket-intent", socketPath, nil, ""); err != nil {
		t.Fatal(err)
	}
	born, ready := sourceRetirementNativeSocket(t, socketPath)
	if err := storage.record("socket-born", socketPath, born, ""); err != nil {
		t.Fatal(err)
	}
	if err := storage.record("socket-ready", socketPath, ready, ""); err != nil {
		t.Fatal(err)
	}
	native := ready.Sys().(*syscall.Stat_t)
	socketSchema := "ardents-source-first-start-socket-v1"
	if number != 0 {
		socketSchema = "ardents-source-invocation-socket-v1"
	}
	if _, err := storage.write("start-socket.json", sourceRetirementNativeJSON(t, sourceSocketRecord{socketSchema, digest(guard.body), uint64(native.Dev), native.Ino, 999}), storage.ctx.Err); err != nil {
		t.Fatal(err)
	}
	for _, phase := range []string{"start-attempt", "invocation-observed"} {
		if err := storage.record(phase, directory, nil, digest(guard.body)); err != nil {
			t.Fatal(err)
		}
	}
	invocation := "01000000000000000000000000000000"
	var frame [160]byte
	copy(frame[:], invocation+generation+binding)
	if err := storage.record("completion-received", directory, nil, digest(frame[:])); err != nil {
		t.Fatal(err)
	}
	archive := sourceStartArchive{Schema: archiveSchema, GuardSHA256: digest(guard.body), BindingSHA256: binding, Generation: generation, PID: uint32(os.Getpid()), InvocationID: invocation, ProcessDevice: 1, ProcessInode: 1, CgroupDevice: 1, CgroupInode: 1, Manager: "unsigned-native-framing-only", StartedRealtime: 1, StartedMonotonic: 1}
	archived, err := storage.write("start-archived.json", sourceRetirementNativeJSON(t, archive), storage.ctx.Err)
	if err != nil {
		t.Fatal(err)
	}
	startSHA := digest(archived.body)
	if err := storage.record("archived", archived.path, archived.info, startSHA); err != nil {
		t.Fatal(err)
	}
	for _, phase := range []string{"ack-attempt", "ack-written"} {
		if err := storage.record(phase, directory, nil, digest(frame[:])); err != nil {
			t.Fatal(err)
		}
	}
	if err := storage.close(); err != nil {
		t.Fatal(err)
	}
	if err := storage.publishStart(); err != nil {
		t.Fatal(err)
	}
	start, err := os.ReadFile(journal)
	if err != nil {
		t.Fatal(err)
	}
	p := &sourceRetirementPhysicalProfile{sourceCutObserverProfile: sourceCutObserverProfile{ProgramSHA256: digest(programBody), Journal: filepath.Join(directory, "source-retirement-journal")}, Attempt: uint32(number), Generation: generation, BindingSHA256: binding, StartSHA256: startSHA, StartJournalSHA256: digest(start), GID: 999, root: root}
	return p, archive, frame
}

// This native image can validate unsigned framing; it does not observe an
// accepting Source, original domain/native join, live writer receiver, actual
// sourceStopStorage.close or a canonical installed process cut.
func TestSourceRetirementPhysicalImageNative(t *testing.T) {
	marker, err := os.ReadFile("/etc/ardents-source-stopped-native-stand")
	if err != nil || string(marker) != "ardents-source-stopped-native-v1\n" || os.Geteuid() != 0 {
		t.Fatal("actual private native file stand required", err)
	}
	for number := sourceAttemptNumber(0); number < sourceAttemptLimit; number++ {
		t.Run(fmt.Sprintf("attempt-%02d", number), func(t *testing.T) {
			p, archive, frame := sourceRetirementNativeStartup(t, number)
			directory := number.directory(p.root)
			writer, err := os.OpenFile(p.Journal, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0600)
			if err != nil {
				t.Fatal(err)
			}
			closed := false
			t.Cleanup(func() {
				if !closed {
					if err := writer.Close(); err != nil {
						t.Error(err)
					}
				}
			})
			if err := syscall.Flock(int(writer.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
				t.Fatal(err)
			}
			journalInfo, err := writer.Stat()
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(directory, "source-retirement-pending"), []byte("ardents-source-retirement-pending-v1\n"+p.StartSHA256+"\n"), 0600); err != nil {
				t.Fatal(err)
			}
			var entries []sourceStartEntry
			record := func(phase, path, pin string, info os.FileInfo) {
				t.Helper()
				entry := sourceStartEntry{Schema: "ardents-source-retirement-record-v1", Sequence: uint32(len(entries)), Phase: phase, Path: path, SHA256: pin}
				if info != nil {
					native := info.Sys().(*syscall.Stat_t)
					entry.Device, entry.Inode, entry.Mode, entry.UID, entry.GID = uint64(native.Dev), native.Ino, uint32(info.Mode().Perm()), native.Uid, native.Gid
				}
				raw := sourceRetirementNativeJSON(t, entry)
				count, written := writer.Write(raw)
				if err := errors.Join(written, writer.Sync()); err != nil || count != len(raw) {
					t.Fatal("outside native journal append", err)
				}
				entries = append(entries, entry)
			}
			record("stop", p.Journal, p.StartSHA256, journalInfo)
			socketPath := filepath.Join(directory, "stop-completion.socket")
			record("socket-intent", socketPath, "", nil)
			born, ready := sourceRetirementNativeSocket(t, socketPath)
			record("socket-born", socketPath, "", born)
			record("socket-ready", socketPath, "", ready)
			challenge := [32]byte{2}
			completed := sourceRetirementFrame(frame, challenge)
			record("interrupt-attempt", directory, digest(challenge[:]), nil)
			record("domain-joined", directory, digest(completed[:]), nil)
			check := func(window string) *sourcePhysicalImage {
				t.Helper()
				p.Window = window
				snapshot, err := p.image()
				if err != nil || snapshot.archive == nil || *snapshot.archive != archive || (snapshot.result != nil) != (window != "native-before-joined") {
					t.Fatal("actual linked native retirement image", snapshot, err)
				}
				again, err := p.image()
				if err != nil || !sameSourcePhysicalImage(snapshot.image, again.image) {
					t.Fatal("unstable native linked image", err)
				}
				return snapshot.image
			}
			check("native-before-joined")
			for _, path := range []string{filepath.Join(p.root, "binding.json"), filepath.Join(p.root, "generations", p.Generation, ProgramName)} {
				original, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				before, err := os.Lstat(path)
				if err != nil {
					t.Fatal(err)
				}
				changed := bytes.Clone(original)
				changed[0] ^= 1
				if err := os.WriteFile(path, changed, before.Mode().Perm()); err != nil {
					t.Fatal(err)
				}
				after, err := os.Lstat(path)
				if err != nil || !os.SameFile(before, after) || before.Mode() != after.Mode() {
					t.Fatal("actual same-inode selected-resource mutation required", err)
				}
				if _, err := p.image(); err == nil {
					t.Fatal("changed same-inode selected resource admitted", path)
				}
				if err := os.WriteFile(path, original, before.Mode().Perm()); err != nil {
					t.Fatal(err)
				}
			}
			for _, field := range []string{"start", "start-journal", "binding", "program"} {
				foreign := *p
				switch field {
				case "start":
					foreign.StartSHA256 = strings.Repeat("0", 64)
				case "start-journal":
					foreign.StartJournalSHA256 = strings.Repeat("0", 64)
				case "binding":
					foreign.BindingSHA256 = strings.Repeat("0", 64)
				case "program":
					foreign.ProgramSHA256 = strings.Repeat("0", 64)
				}
				if _, err := foreign.image(); err == nil {
					t.Fatal("foreign original image pin admitted", field)
				}
			}
			result := sourceRetirementResult{Schema: "ardents-source-retirement-v1", Attempt: uint32(number), Generation: p.Generation, BindingSHA256: p.BindingSHA256, StartSHA256: p.StartSHA256, InvocationID: archive.InvocationID, PID: archive.PID, Manager: archive.Manager, Challenge: fmt.Sprintf("%x", challenge), CompletionSHA256: digest(completed[:]), StartedRealtime: 1, StartedMonotonic: 1, ExitedRealtime: 2, ExitedMonotonic: 2, Code: 1}
			body := sourceRetirementNativeJSON(t, result)
			resultPath := filepath.Join(directory, "source-retirement.json")
			record("native-joined", directory, digest(body), nil)
			record("native-closed", directory, digest(body), nil)
			record("file-intent", resultPath, digest(body), nil)
			if err := os.WriteFile(resultPath, body, 0600); err != nil {
				t.Fatal(err)
			}
			resultInfo, err := os.Lstat(resultPath)
			if err != nil {
				t.Fatal(err)
			}
			for _, phase := range []string{"file-born", "file-ready", "retired"} {
				record(phase, resultPath, digest(body), resultInfo)
			}
			before := check("result-before-close")
			closed = true
			if err := writer.Close(); err != nil {
				t.Fatal(err)
			}
			after := check("pending-before-unlink")
			if !sameSourcePhysicalImage(before, after) {
				t.Fatal("outside writer Close changed original image")
			}
			pending := filepath.Join(directory, "source-start-pending")
			if err := os.WriteFile(pending, []byte("blocked\n"), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := p.image(); err == nil {
				t.Fatal("guarded startup admitted as complete retirement predecessor")
			}
			if err := os.Remove(pending); err != nil {
				t.Fatal(err)
			}
			archived := filepath.Join(directory, "start-archived.json")
			raw, err := os.ReadFile(archived)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.Rename(archived, archived+".original"); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(archived, bytes.Clone(raw), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := p.image(); err == nil {
				t.Fatal("same-byte copied startup archive supplied original retirement image")
			}
		})
	}
}
