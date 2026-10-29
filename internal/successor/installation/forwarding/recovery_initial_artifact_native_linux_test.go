//go:build network_signing_native

package forwarding

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/successor/installation/fixedfile"
)

// Physical custody and the complete finite suffix inventory, without a signed
// Release, original process cut, manager or accepting recovery permission.
func TestForwardingRecoveryPublicationNativeOriginalGeneration(t *testing.T) {
	marker, err := os.ReadFile("/etc/ardents-forwarding-preparation-native-stand")
	if err != nil || string(marker) != "ardents-forwarding-preparation-native-v1\n" || os.Geteuid() != 0 || os.Getegid() != 0 {
		t.Fatal("invalid root native stand", err)
	}
	names := []string{"ardents-forwarding.service", "ardents-node-linux-amd64", "binding.json", "forwarding-plan.json", "forwarding-request.json", "infrastructure-enrollment.json", "infrastructure-forwarding.json", "rendered-forwarding.service", "root.json", "snapshot.json", "targets.json", "timestamp.json"}
	for count := uint32(72); count <= 104; count++ {
		for _, fault := range []string{"complete", "cancel-after-birth", "foreign-inode", "unknown-entry"} {
			t.Run(strconv.Itoa(int(count))+"/"+fault, func(t *testing.T) {
				parent, err := os.MkdirTemp(os.TempDir(), "frgeneration-")
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					if err := os.RemoveAll(parent); err != nil {
						t.Error(err)
					}
				})
				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()
				b := binding{Generation: digest([]byte("physical generation fixture")), Request: Request{InstallationRoot: filepath.Join(parent, "installation")}}
				candidate := &recoveryCandidate{selection: generationSelection{binding: b}, files: map[string][]byte{}}
				directory := filepath.Join(b.Request.InstallationRoot, "generations", b.Generation)
				if err := os.MkdirAll(directory, 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.Chmod(directory, 0755); err != nil {
					t.Fatal(err)
				}
				files := &completedFiles{ctx: t.Context(), files: map[string]*initialFile{}, dirs: map[string]*inputDirectory{}}
				defer files.close()
				if err := files.directory(directory, 0755, 0, 0); err != nil {
					t.Fatal(err)
				}
				g := &initialGenerationContinuation{records: count}
				defer g.close()
				identities := map[string]os.FileInfo{}
				for i, name := range names {
					path := filepath.Join(directory, name)
					body := []byte(name + " physical byte fixture\n")
					candidate.files[name] = body
					sequence := 73 + 3*i
					ready, retained := int(count) >= sequence+2, int(count) == sequence+1
					var file *initialFile
					var birth initialRecord
					mode := os.FileMode(0444)
					if name == "ardents-node-linux-amd64" {
						mode = 0555
					}
					if ready || retained {
						f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0600)
						if err != nil {
							t.Fatal(err)
						}
						if ready {
							n, writeErr := f.Write(body)
							if err := errors.Join(writeErr, f.Chmod(mode)); err != nil || n != len(body) {
								f.Close()
								t.Fatal(err)
							}
						}
						info, statErr := f.Stat()
						if err := errors.Join(statErr, f.Sync(), f.Close()); err != nil {
							t.Fatal(err)
						}
						identities[name] = info
						if ready {
							if _, err := files.read(path, 64<<20, mode, 0, 0); err != nil {
								t.Fatal(err)
							}
						} else {
							native, _ := infoSys(info)
							birth = initialRecord{Sequence: sequence, Phase: "file-born", Path: path, Device: uint64(native.Dev), Inode: native.Ino, Mode: 0600}
							if err := readInitialEmptyBirth(files, path, birth); err != nil {
								t.Fatal(err)
							}
							file = files.files[path]
						}
					}
					if !ready {
						g.artifacts = append(g.artifacts, &initialArtifactContinuation{name: name, birthSequence: sequence, retained: retained, initialFileContinuation: initialFileContinuation{original: file, birth: birth}})
					}
				}
				allowed := map[string]bool{}
				for _, u := range g.artifacts {
					allowed[filepath.Join(directory, u.name)] = true
				}
				writer, err := openForwardingBirths(ctx, filepath.Join(parent, "control"), allowed, "forwarding-recovery-generation-v1")
				if err != nil {
					writer.close()
					t.Fatal("actual early generation writer opening", err)
				}
				defer writer.close()
				if err := g.check(candidate, files); err != nil {
					t.Fatal("original finite physical inventory", err)
				}
				if fault == "unknown-entry" {
					if err := os.WriteFile(filepath.Join(directory, "foreign"), []byte("preserve\n"), 0600); err != nil {
						t.Fatal(err)
					}
					if err := g.check(candidate, files); err == nil {
						t.Fatal("unknown artifact admitted")
					}
					return
				}
				for _, u := range g.artifacts {
					path := filepath.Join(directory, u.name)
					mode := os.FileMode(0444)
					if u.name == "ardents-node-linux-amd64" {
						mode = 0555
					}
					parentInfo := files.dirs[directory].info
					if u.retained {
						u.mutation, err = fixedfile.ContinueBirth(ctx, path, parentInfo, u.original.info, candidate.files[u.name], mode, 0)
					} else {
						if err := writer.record("continued-artifact-intent", path, nil, digest(candidate.files[u.name]), mode, 0, 0); err != nil {
							t.Fatal(err)
						}
						u.mutation, err = fixedfile.Create(ctx, path, parentInfo, candidate.files[u.name], mode, 0)
					}
					if err != nil {
						t.Fatal(err)
					}
					if !u.retained {
						native, _ := infoSys(u.mutation.Identity())
						u.birth = initialRecord{Sequence: u.birthSequence, Phase: "file-born", Path: path, Device: uint64(native.Dev), Inode: native.Ino, Mode: 0600}
						if err := readInitialEmptyBirth(files, path, u.birth); err != nil {
							t.Fatal(err)
						}
						u.original = files.files[path]
						identities[u.name] = u.original.info
						if err := writer.record("continued-artifact-born", path, u.original.info, digest(nil), 0600, 0, 0); err != nil {
							t.Fatal(err)
						}
					} else if err := writer.record("continued-artifact-retained", path, u.original.info, digest(nil), 0600, 0, 0); err != nil {
						t.Fatal(err)
					}
					if err := g.check(candidate, files); err != nil {
						t.Fatal("recorded empty physical inventory", err)
					}
					if fault == "cancel-after-birth" {
						cancel()
					}
					if fault == "foreign-inode" {
						if err := os.Rename(path, path+"-original"); err != nil {
							t.Fatal(err)
						}
						if err := os.WriteFile(path, nil, 0600); err != nil {
							t.Fatal(err)
						}
					}
					err = u.commit(candidate.files[u.name], mode)
					if fault != "complete" {
						if err == nil || u.ready || fault == "cancel-after-birth" && !errors.Is(err, context.Canceled) {
							t.Fatal("uncertain artifact accepted or cause lost", err)
						}
						var probe [1]byte
						n, readErr := u.original.file.ReadAt(probe[:], 0)
						info, statErr := u.original.file.Stat()
						if n != 0 || readErr != io.EOF || statErr != nil || !nodeAccess(info, 0600, 0, 0, false) || g.close() == nil {
							t.Fatal("refusal changed original empty file or lost close failure", readErr, statErr)
						}
						return
					}
					if err != nil || g.check(candidate, files) != nil {
						t.Fatal("completed physical generation suffix", err)
					}
					if err := writer.record("continued-artifact-written", path, u.original.info, digest(candidate.files[u.name]), mode, 0, 0); err != nil {
						t.Fatal(err)
					}
				}
				if err := errors.Join(g.close(), writer.close(), files.close()); err != nil {
					t.Fatal("original writers/readers did not join", err)
				}
				lines := bytes.Split(bytes.TrimSuffix(writer.journal.body, []byte{'\n'}), []byte{'\n'})
				wantedRows := 1 + 3*len(g.artifacts)
				if count%3 == 2 {
					wantedRows--
				}
				var first, last initialRecord
				if len(lines) != wantedRows || json.Unmarshal(lines[0], &first) != nil || json.Unmarshal(lines[len(lines)-1], &last) != nil || first.Phase != "forwarding-recovery-generation-v1" || last.Phase != "continued-artifact-written" {
					t.Fatal("actual generation writer phases/count", len(lines), wantedRows)
				}
				for _, name := range names {
					mode := os.FileMode(0444)
					if name == "ardents-node-linux-amd64" {
						mode = 0555
					}
					body, info, err := readInitialOriginal(t.Context(), filepath.Join(directory, name), 64<<20, mode, 0, 0, false)
					if err != nil || !bytes.Equal(body, candidate.files[name]) || !os.SameFile(identities[name], info) {
						t.Fatal("fresh readback changed original/new birth", name, err)
					}
				}
			})
		}
	}
}

// Actual exclusive/original-empty physical custody, wrapper Commit and writer
// phases only. These fixtures provide no original cut, signed Release or
// accepting recovery authority.
func TestForwardingRecoveryPublicationNativeOriginalTimestamp(t *testing.T) {
	marker, err := os.ReadFile("/etc/ardents-forwarding-preparation-native-stand")
	if err != nil || string(marker) != "ardents-forwarding-preparation-native-v1\n" || os.Geteuid() != 0 || os.Getegid() != 0 {
		t.Fatal("invalid root native stand", err)
	}
	for _, kind := range []string{"create", "original"} {
		for _, fault := range []string{"complete", "cancel-after-birth", "foreign-inode"} {
			t.Run(kind+"-"+fault, func(t *testing.T) {
				parent, err := os.MkdirTemp(os.TempDir(), "frartifact-")
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					if err := os.RemoveAll(parent); err != nil {
						t.Error(err)
					}
				})
				path := filepath.Join(parent, "timestamp.json")
				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()
				wanted := []byte("timestamp physical byte fixture\n")
				first := "forwarding-recovery-artifact-create-v1"
				if kind == "original" {
					first = "forwarding-recovery-artifact-v1"
				}
				writer, err := openForwardingBirths(ctx, filepath.Join(parent, "control"), map[string]bool{path: true}, first)
				if err != nil {
					t.Fatal(err)
				}
				defer writer.close()
				parentInfo, err := os.Lstat(parent)
				if err != nil {
					t.Fatal(err)
				}
				files := &completedFiles{ctx: t.Context(), files: map[string]*initialFile{}, dirs: map[string]*inputDirectory{}}
				defer files.close()
				u := &initialFileContinuation{}
				defer u.close()
				if kind == "create" {
					if err := writer.record("continued-artifact-intent", path, nil, digest(wanted), 0444, 0, 0); err != nil {
						t.Fatal(err)
					}
					u.mutation, err = fixedfile.Create(ctx, path, parentInfo, wanted, 0444, 0)
					if err != nil {
						t.Fatal(err)
					}
					native, _ := infoSys(u.mutation.Identity())
					u.birth = initialRecord{Sequence: 106, Phase: "file-born", Path: path, Device: uint64(native.Dev), Inode: native.Ino, Mode: 0600}
					if err := writer.record("continued-artifact-born", path, u.mutation.Identity(), digest(nil), 0600, 0, 0); err != nil {
						t.Fatal(err)
					}
				} else {
					f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0600)
					if err != nil {
						t.Fatal(err)
					}
					info, statErr := f.Stat()
					if err := errors.Join(statErr, f.Sync(), f.Close()); err != nil {
						t.Fatal(err)
					}
					native, _ := infoSys(info)
					u.birth = initialRecord{Sequence: 106, Phase: "file-born", Path: path, Device: uint64(native.Dev), Inode: native.Ino, Mode: 0600}
				}
				if !validInitialTimestampBirth(&u.birth, path) {
					t.Fatal("actual timestamp birth missing")
				}
				if err := readInitialEmptyBirth(files, path, u.birth); err != nil {
					t.Fatal(err)
				}
				u.original = files.files[path]
				if kind == "original" {
					u.mutation, err = fixedfile.ContinueBirth(ctx, path, parentInfo, u.original.info, wanted, 0444, 0)
					if err != nil {
						t.Fatal(err)
					}
					if err := writer.record("continued-artifact-retained", path, u.original.info, digest(nil), 0600, 0, 0); err != nil {
						t.Fatal(err)
					}
				}
				if fault == "cancel-after-birth" {
					cancel()
				}
				if fault == "foreign-inode" {
					if err := os.Rename(path, path+"-original"); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(path, nil, 0600); err != nil {
						t.Fatal(err)
					}
				}
				err = u.commit(wanted, 0444)
				if fault == "complete" {
					if err != nil || !u.ready {
						t.Fatal("actual timestamp continuation", err)
					}
					if err := writer.record("continued-artifact-written", path, u.original.info, digest(wanted), 0444, 0, 0); err != nil {
						t.Fatal(err)
					}
					current, err := os.Lstat(path)
					body, readErr := os.ReadFile(path)
					native, ok := infoSys(current)
					if err != nil || readErr != nil || !ok || !nodeAccess(current, 0444, 0, 0, false) || native.Ino != u.birth.Inode || uint64(native.Dev) != u.birth.Device || !bytes.Equal(body, wanted) {
						t.Fatal("timestamp changed original inode, access or bytes", err, readErr)
					}
					var rows []initialRecord
					for _, line := range bytes.Split(bytes.TrimSuffix(writer.journal.body, []byte{'\n'}), []byte{'\n'}) {
						var row initialRecord
						if json.Unmarshal(line, &row) != nil {
							t.Fatal("writer record framing")
						}
						rows = append(rows, row)
					}
					count := 4
					if kind == "original" {
						count = 3
					}
					if len(rows) != count || rows[0].Phase != first || rows[len(rows)-1].Phase != "continued-artifact-written" || rows[len(rows)-1].Mode != 0444 {
						t.Fatal("actual artifact writer opening/phases")
					}
					if err := u.close(); err != nil {
						t.Fatal("joined successful mutation", err)
					}
				} else {
					if err == nil || u.ready {
						t.Fatal("uncertain timestamp accepted")
					}
					if fault == "cancel-after-birth" && !errors.Is(err, context.Canceled) {
						t.Fatal("cancellation cause lost", err)
					}
					var probe [1]byte
					n, readErr := u.original.file.ReadAt(probe[:], 0)
					info, statErr := u.original.file.Stat()
					if n != 0 || readErr != io.EOF || statErr != nil || !nodeAccess(info, 0600, 0, 0, false) {
						t.Fatal("refusal changed original empty timestamp", readErr, statErr)
					}
					if closeErr := u.close(); closeErr == nil {
						t.Fatal("mutation lost retained failure")
					}
				}
				if err := files.close(); err != nil {
					t.Fatal("original reader close", err)
				}
			})
		}
	}
}
