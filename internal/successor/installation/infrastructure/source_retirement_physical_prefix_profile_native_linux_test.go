//go:build linux && amd64 && network_signing_native

package infrastructure

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// Actual native files, socket transfer and outside append-only writer only.
// The unsigned framing fixture grants no Source, account, manager, semantic
// history, native-domain completion or successful stop authority. In particular
// its own writer Close is not sourceStopStorage.close or an installed cut.
func TestSourceRetirementPhysicalPrefixNative(t *testing.T) {
	marker, err := os.ReadFile("/etc/ardents-source-stopped-native-stand")
	if err != nil || string(marker) != "ardents-source-stopped-native-v1\n" || os.Geteuid() != 0 {
		t.Fatal("actual private native file stand required", err)
	}
	for number := sourceAttemptNumber(0); number < sourceAttemptLimit; number++ {
		t.Run(fmt.Sprintf("attempt-%02d", number), func(t *testing.T) {
			root, err := os.MkdirTemp(os.TempDir(), "rp-")
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := os.RemoveAll(root); err != nil {
					t.Error(err)
				}
			})
			directory := number.directory(root)
			if err := os.Mkdir(directory, 0750); err != nil {
				t.Fatal(err)
			}
			if err := os.Chown(directory, 0, 999); err != nil {
				t.Fatal(err)
			}
			journal := filepath.Join(directory, "source-retirement-journal")
			writer, err := os.OpenFile(journal, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0600)
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
			journalInfo, err := writer.Stat()
			if err != nil {
				t.Fatal(err)
			}
			if err := syscall.Flock(int(writer.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
				t.Fatal(err)
			}
			start := digest([]byte("unsigned original startup archive framing only\n"))
			pending := filepath.Join(directory, "source-retirement-pending")
			if err := os.WriteFile(pending, []byte("ardents-source-retirement-pending-v1\n"+start+"\n"), 0600); err != nil {
				t.Fatal(err)
			}
			var entries []sourceStartEntry
			appendRecord := func(phase, path, pin string, info os.FileInfo) {
				t.Helper()
				entry := sourceStartEntry{Schema: "ardents-source-retirement-record-v1", Sequence: uint32(len(entries)), Phase: phase, Path: path, SHA256: pin}
				if info != nil {
					native := info.Sys().(*syscall.Stat_t)
					entry.Device, entry.Inode, entry.Mode, entry.UID, entry.GID = uint64(native.Dev), native.Ino, uint32(info.Mode().Perm()), native.Uid, native.Gid
				}
				raw, err := json.Marshal(entry)
				if err != nil {
					t.Fatal(err)
				}
				raw = append(raw, '\n')
				count, written := writer.Write(raw)
				if err := errors.Join(written, writer.Sync()); err != nil || count != len(raw) {
					t.Fatal("actual outside original journal append/sync", err)
				}
				entries = append(entries, entry)
			}
			appendRecord("stop", journal, start, journalInfo)
			socketPath := filepath.Join(directory, "stop-completion.socket")
			appendRecord("socket-intent", socketPath, "", nil)
			listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: socketPath, Net: "unix"})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := listener.Close(); err != nil {
					t.Error(err)
				}
			})
			listener.SetUnlinkOnClose(false)
			if err := listener.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
				t.Fatal(err)
			}
			born, err := os.Lstat(socketPath)
			if err != nil {
				t.Fatal(err)
			}
			appendRecord("socket-born", socketPath, "", born)
			if err := os.Chown(socketPath, 0, 999); err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(socketPath, 0660); err != nil {
				t.Fatal(err)
			}
			socketInfo, err := os.Lstat(socketPath)
			if err != nil {
				t.Fatal(err)
			}
			appendRecord("socket-ready", socketPath, "", socketInfo)
			challenge := [32]byte{2}
			var startup [160]byte
			copy(startup[:], strings.Repeat("1", 32)+strings.Repeat("2", 64)+strings.Repeat("3", 64))
			frame := sourceRetirementFrame(startup, challenge)
			appendRecord("interrupt-attempt", directory, digest(challenge[:]), nil)
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
			deadline := time.Now().Add(5 * time.Second)
			if err := errors.Join(client.SetDeadline(deadline), server.SetDeadline(deadline)); err != nil {
				t.Fatal(err)
			}
			count, sent := io.Copy(client, bytes.NewReader(frame[:]))
			var observed [sourceRetirementFrameSize]byte
			_, received := io.ReadFull(server, observed[:])
			if count != int64(len(frame)) || sent != nil || received != nil || frame != observed {
				t.Fatal("actual native unsigned retirement frame transfer", sent, received)
			}
			appendRecord("domain-joined", directory, digest(observed[:]), nil)
			resultPath := filepath.Join(directory, "source-retirement.json")
			resultSHA := ""
			encodeEntries := func(changed []sourceStartEntry) []byte {
				t.Helper()
				var result []byte
				for _, entry := range changed {
					raw, err := json.Marshal(entry)
					if err != nil {
						t.Fatal(err)
					}
					result = append(append(result, raw...), '\n')
				}
				return result
			}
			check := func(t *testing.T, window string) {
				t.Helper()
				raw, err := os.ReadFile(journal)
				if err != nil || !bytes.Equal(raw, encodeEntries(entries)) {
					t.Fatal("outside append-only native journal differs", err)
				}
				decode := func(raw []byte) ([]sourceStartEntry, error) {
					return sourceRetirementPhysicalPrefix(raw, number, root, start, digest(challenge[:]), digest(observed[:]), resultSHA, 999, window)
				}
				decoded, err := decode(raw)
				if err != nil {
					t.Fatal("exact native retirement prefix", err)
				}
				if _, err := sourceRetirementPhysicalFiles(decoded, 999); err != nil {
					t.Fatal("actual native retirement image", err)
				}
				if err := probeSourceRefusalLease(journal, digest(raw), 0600, !closed); err != nil {
					t.Fatal("actual outside writer lease", err)
				}
				// Failure-only product consumer: no successful Source selection is
				// supplied. The actual pending guard must stop it before any proof.
				selection := &sourceSelection{binding: sourceBinding{Request: SourceRequest{InstallationRoot: root}}}
				if _, err := readSourceRetirement(selection, &sourceStartHistory{number: number}); !errors.Is(err, ErrBinding) {
					t.Fatal("blocked fixture supplied committed retirement", err)
				}
				if len(decoded) == 6 {
					if _, err := decodeSourceRetirementRecords(raw, directory, start, digest(challenge[:]), digest(observed[:]), strings.Repeat("4", 64), 999); err == nil {
						t.Fatal("product complete-only reader accepted six-record prefix")
					}
					if err := os.WriteFile(resultPath, []byte("unrecorded unsigned result\n"), 0600); err != nil {
						t.Fatal(err)
					}
					if _, err := sourceRetirementPhysicalFiles(decoded, 999); err == nil {
						t.Fatal("unrecorded native result accepted")
					}
					if err := os.Remove(resultPath); err != nil {
						t.Fatal(err)
					}
				}
				for index := range decoded {
					changed := append([]sourceStartEntry(nil), decoded...)
					changed[index].Path += ".foreign"
					if _, err := decode(encodeEntries(changed)); err == nil {
						t.Fatal("foreign original retirement path accepted", index)
					}
				}
				for _, changed := range [][]byte{raw[:len(raw)-1], append(bytes.Clone(raw), []byte("{}\n")...), bytes.Replace(raw, []byte(`"schema":`), []byte(`"extra":true,"schema":`), 1), bytes.Replace(raw, []byte(`"schema":`), []byte(`"schema":"other","schema":`), 1)} {
					if _, err := decode(changed); err == nil {
						t.Fatal("ambiguous or incomplete retirement prefix accepted")
					}
				}
				link := journal + ".linked"
				if err := os.Link(journal, link); err != nil {
					t.Fatal(err)
				}
				if _, err := sourceRetirementPhysicalFiles(decoded, 999); err == nil {
					t.Fatal("aliased original retirement journal accepted")
				}
				if err := os.Remove(link); err != nil {
					t.Fatal(err)
				}
			}
			t.Run("native-before-joined", func(t *testing.T) { check(t, "native-before-joined") })
			result := sourceRetirementResult{Schema: "ardents-source-retirement-v1", Attempt: uint32(number), Generation: strings.Repeat("2", 64), BindingSHA256: strings.Repeat("3", 64), StartSHA256: start, InvocationID: strings.Repeat("1", 32), PID: uint32(os.Getpid()), Manager: "unsigned-native-framing-only", Challenge: fmt.Sprintf("%x", challenge), CompletionSHA256: digest(observed[:]), StartedRealtime: 1, StartedMonotonic: 1, ExitedRealtime: 2, ExitedMonotonic: 2, Code: 1}
			body, err := json.Marshal(result)
			if err != nil {
				t.Fatal(err)
			}
			body = append(body, '\n')
			resultSHA = digest(body)
			appendRecord("native-joined", directory, resultSHA, nil)
			appendRecord("native-closed", directory, resultSHA, nil)
			appendRecord("file-intent", resultPath, resultSHA, nil)
			file, err := os.OpenFile(resultPath, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0600)
			if err != nil {
				t.Fatal(err)
			}
			fileClosed := false
			t.Cleanup(func() {
				if !fileClosed {
					if err := file.Close(); err != nil {
						t.Error(err)
					}
				}
			})
			birthInfo, err := file.Stat()
			if err != nil {
				t.Fatal(err)
			}
			appendRecord("file-born", resultPath, resultSHA, birthInfo)
			countWritten, written := file.Write(body)
			if err := errors.Join(written, file.Sync()); err != nil || countWritten != len(body) {
				t.Fatal("actual outside result write/sync", err)
			}
			resultInfo, err := file.Stat()
			if err != nil {
				t.Fatal(err)
			}
			appendRecord("file-ready", resultPath, resultSHA, resultInfo)
			appendRecord("retired", resultPath, resultSHA, resultInfo)
			t.Run("result-before-close", func(t *testing.T) { check(t, "result-before-close") })
			before, err := sourceRetirementPhysicalFiles(entries, 999)
			if err != nil {
				t.Fatal(err)
			}
			closed, fileClosed = true, true
			if err := errors.Join(file.Close(), writer.Close()); err != nil {
				t.Fatal(err)
			}
			after, err := sourceRetirementPhysicalFiles(entries, 999)
			if err != nil || !bytes.Equal(sourceRetirementImageJSON(t, before), sourceRetirementImageJSON(t, after)) {
				t.Fatal("outside writer Close changed retained native image", err)
			}
			t.Run("pending-before-unlink", func(t *testing.T) { check(t, "pending-before-unlink") })
			// Preserve the original result inode under a different fixture name;
			// a same-byte replacement must not match its recorded birth.
			original := resultPath + ".original"
			if err := os.Rename(resultPath, original); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(resultPath, body, 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := sourceRetirementPhysicalFiles(entries, 999); err == nil {
				t.Fatal("copied same-byte result inode accepted")
			}
		})
	}
}

func sourceRetirementImageJSON(t *testing.T, files []sourcePhysicalSealedFile) []byte {
	t.Helper()
	raw, err := json.Marshal(files)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
