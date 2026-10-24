//go:build linux && amd64 && network_signing_native

package infrastructure

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/successor/installation/completion"
)

// Every finite first/subsequent path uses actual native storage, births and a
// real Unix frame write/read. Unsigned guard/archive bodies deliberately supply
// no account, Release, manager, Source process or installed startup authority.
// There is no Start, recovery invocation or process-cut qualification here.
func TestSourceStartupPhysicalPrefixNative(t *testing.T) {
	marker, err := os.ReadFile("/etc/ardents-source-stopped-native-stand")
	if err != nil || string(marker) != "ardents-source-stopped-native-v1\n" || os.Geteuid() != 0 {
		t.Fatal("actual private native file stand required", err)
	}
	for number := sourceAttemptNumber(0); number < sourceAttemptLimit; number++ {
		t.Run(fmt.Sprintf("attempt-%02d", number), func(t *testing.T) {
			ctx := context.Background()
			root, err := os.MkdirTemp(os.TempDir(), "sp-")
			if err != nil {
				t.Fatal(err)
			}
			journal, directory := number.journal(root), number.directory(root)
			t.Cleanup(func() {
				if err := os.RemoveAll(root); err != nil {
					t.Error(err)
				}
				if err := os.Remove(journal); err != nil && !os.IsNotExist(err) {
					t.Error(err)
				}
			})
			file, err := os.OpenFile(journal, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0600)
			if err != nil {
				t.Fatal(err)
			}
			storage := &sourceStartStorage{ctx: ctx, path: directory, number: number, journal: &sourceBirth{path: journal, file: file}}
			t.Cleanup(func() {
				if err := storage.close(); err != nil {
					t.Error(err)
				}
			})
			info, err := file.Stat()
			if err != nil {
				t.Fatal(err)
			}
			if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
				t.Fatal(err)
			}
			storage.journal.info = info
			bindingBody := []byte("unsigned native binding framing only\n")
			binding, generation := digest(bindingBody), strings.Repeat("2", 64)
			if err := os.WriteFile(filepath.Join(root, "binding.json"), bindingBody, 0644); err != nil {
				t.Fatal(err)
			}
			programDirectory := filepath.Join(root, "generations", generation)
			if err := os.MkdirAll(programDirectory, 0755); err != nil {
				t.Fatal(err)
			}
			programBody := []byte("unsigned native program framing only\n")
			if err := os.WriteFile(filepath.Join(programDirectory, ProgramName), programBody, 0555); err != nil {
				t.Fatal(err)
			}
			_, first, _, _ := number.schema()
			if err := storage.record(first, journal, info, binding); err != nil {
				t.Fatal(err)
			}
			if err := storage.birthDirectory(999, ctx.Err); err != nil {
				t.Fatal(err)
			}
			if err := storage.preparePending(binding); err != nil {
				t.Fatal(err)
			}
			encode := func(value any) []byte {
				t.Helper()
				raw, err := json.Marshal(value)
				if err != nil {
					t.Fatal(err)
				}
				return append(raw, '\n')
			}
			_, _, intentSchema, archiveSchema := number.schema()
			intent := sourceStartIntent{Schema: intentSchema, BindingSHA256: binding, InitialHistorySHA256: strings.Repeat("3", 64), TargetsSHA256: strings.Repeat("4", 64), UID: 999, GID: 999, Attempt: uint32(number)}
			if number != 0 {
				intent.PredecessorSHA256 = strings.Repeat("5", 64)
			}
			guard, err := storage.write("start-guard.json", encode(intent), ctx.Err)
			if err != nil {
				t.Fatal(err)
			}
			socketPath := filepath.Join(directory, "start-completion.socket")
			if err := storage.record("socket-intent", socketPath, nil, ""); err != nil {
				t.Fatal(err)
			}
			listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: socketPath, Net: "unix"})
			if err != nil {
				t.Fatal(err)
			}
			listener.SetUnlinkOnClose(false)
			t.Cleanup(func() {
				if err := listener.Close(); err != nil {
					t.Error(err)
				}
			})
			socketInfo, err := os.Lstat(socketPath)
			if err != nil {
				t.Fatal(err)
			}
			if err := storage.record("socket-born", socketPath, socketInfo, ""); err != nil {
				t.Fatal(err)
			}
			if err := os.Chown(socketPath, 0, 999); err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(socketPath, 0660); err != nil {
				t.Fatal(err)
			}
			socketInfo, err = os.Lstat(socketPath)
			if err != nil {
				t.Fatal(err)
			}
			if err := storage.record("socket-ready", socketPath, socketInfo, ""); err != nil {
				t.Fatal(err)
			}
			native := socketInfo.Sys().(*syscall.Stat_t)
			socketSchema := "ardents-source-first-start-socket-v1"
			if number != 0 {
				socketSchema = "ardents-source-invocation-socket-v1"
			}
			socketRecord, err := storage.write("start-socket.json", encode(sourceSocketRecord{socketSchema, digest(guard.body), uint64(native.Dev), native.Ino, 999}), ctx.Err)
			if err != nil {
				t.Fatal(err)
			}
			for _, phase := range []string{"start-attempt", "invocation-observed"} {
				if err := storage.record(phase, directory, nil, digest(guard.body)); err != nil {
					t.Fatal(err)
				}
			}
			frame, err := completion.Encode([16]byte{1}, generation, binding)
			if err != nil {
				t.Fatal(err)
			}
			if err := storage.record("completion-received", directory, nil, digest(frame[:])); err != nil {
				t.Fatal(err)
			}
			// This PID is the test process, whose executable deliberately differs
			// from the unsigned selected program. Live Source matching must refuse.
			archiveBody := sourceStartArchive{Schema: archiveSchema, GuardSHA256: digest(guard.body), BindingSHA256: binding, Generation: generation, PID: uint32(os.Getpid()), InvocationID: "01000000000000000000000000000000", ProcessDevice: 1, ProcessInode: 1, CgroupDevice: 1, CgroupInode: 1, Manager: "unsigned-native-framing-only", StartedRealtime: 1, StartedMonotonic: 1}
			archive, err := storage.write("start-archived.json", encode(archiveBody), ctx.Err)
			if err != nil {
				t.Fatal(err)
			}
			check := func(window string) []sourceStartEntry {
				t.Helper()
				raw, err := os.ReadFile(journal)
				if err != nil || !bytes.Equal(raw, storage.journal.body) {
					t.Fatal("actual native writer readback differs", err)
				}
				decode := func(raw []byte) ([]sourceStartEntry, error) {
					return sourceStartupPhysicalPrefix(raw, number, root, binding, digest(guard.body), digest(socketRecord.body), digest(archive.body), digest(frame[:]), 999, window)
				}
				entries, err := decode(raw)
				if err != nil {
					t.Fatal("actual partial native prefix", err)
				}
				if _, err := sourceStartupPhysicalFiles(entries); err != nil {
					t.Fatal("actual native birth/pending/socket image", err)
				}
				p := &sourceStartupPhysicalProfile{sourceCutObserverProfile: sourceCutObserverProfile{Journal: journal, ProgramSHA256: digest(programBody)}, Window: window, Attempt: uint32(number), Generation: generation, BindingSHA256: binding, GID: 999, root: root}
				before, _, err := p.image(false)
				if err != nil {
					t.Fatal("actual native snapshot only", err)
				}
				after, _, err := p.image(false)
				if err != nil || !sameSourcePhysicalImage(before, after) {
					t.Fatal("native startup snapshot changed", err)
				}
				if _, _, err := p.image(true); err == nil {
					t.Fatal("unsigned native snapshot supplied a live Source process")
				}
				if _, err := decodeSourceStartRecords(raw, number, journal, directory, binding, digest(guard.body), digest(socketRecord.body), digest(archive.body), digest(frame[:]), 999); err == nil {
					t.Fatal("product complete-only reader accepted partial startup")
				}
				for index := range entries {
					changed := append([]sourceStartEntry(nil), entries...)
					changed[index].Path += ".foreign"
					var altered []byte
					for _, entry := range changed {
						encoded, err := json.Marshal(entry)
						if err != nil {
							t.Fatal(err)
						}
						altered = append(append(altered, encoded...), '\n')
					}
					if _, err := decode(altered); err == nil {
						t.Fatal("foreign native resource path admitted", index)
					}
				}
				for _, altered := range [][]byte{append(bytes.Clone(raw), []byte("{}\n")...), bytes.Replace(raw, []byte(`"schema":`), []byte(`"extra":true,"schema":`), 1), bytes.Replace(raw, []byte(`"schema":`), []byte(`"schema":"other","schema":`), 1), raw[:len(raw)-1]} {
					if _, err := decode(altered); err == nil {
						t.Fatal("ambiguous, unknown or incomplete physical prefix admitted")
					}
				}
				return entries
			}
			check("archive-before-archived")
			if err := storage.record("archived", archive.path, archive.info, digest(archive.body)); err != nil {
				t.Fatal(err)
			}
			if err := storage.record("ack-attempt", directory, nil, digest(frame[:])); err != nil {
				t.Fatal(err)
			}
			client, err := net.DialUnix("unix", nil, &net.UnixAddr{Name: socketPath, Net: "unix"})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := client.Close(); err != nil {
					t.Error(err)
				}
			})
			server, err := listener.AcceptUnix()
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := server.Close(); err != nil {
					t.Error(err)
				}
			})
			count, err := io.Copy(server, bytes.NewReader(frame[:]))
			if err != nil || count != int64(len(frame)) {
				t.Fatal("actual full native frame write", count, err)
			}
			var received [len(frame)]byte
			if _, err := io.ReadFull(client, received[:]); err != nil || received != frame {
				t.Fatal("actual native frame peer read", err)
			}
			entries := check("ack-before-written")
			// Retain the old native inode so replacement cannot recycle it.
			retained, err := os.Open(archive.path)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := retained.Close(); err != nil {
					t.Error(err)
				}
			})
			if err := os.Rename(archive.path, archive.path+".retained"); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(archive.path, archive.body, 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := sourceStartupPhysicalFiles(entries); err == nil {
				t.Fatal("copied same-byte native archive admitted")
			}
		})
	}
}
