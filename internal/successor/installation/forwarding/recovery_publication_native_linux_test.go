//go:build network_signing_native

package forwarding

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// Physical same-directory sealing and actual writer phases only. These native
// fixtures grant no original cut, candidate, Release or installed permission.
func TestForwardingRecoveryPublicationNativeOriginalSeal(t *testing.T) {
	marker, err := os.ReadFile("/etc/ardents-forwarding-preparation-native-stand")
	if err != nil || string(marker) != "ardents-forwarding-preparation-native-v1\n" || os.Geteuid() != 0 || os.Getegid() != 0 {
		t.Fatal("invalid disposable root native stand", err)
	}
	for _, fault := range []string{"complete", "unknown-entry", "mode", "inode", "cancel-before", "cancel-after-sync", "late-sync-error"} {
		t.Run(fault, func(t *testing.T) {
			parent, err := os.MkdirTemp(os.TempDir(), "frseal-")
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := os.RemoveAll(parent); err != nil {
					t.Error(err)
				}
			})
			path := filepath.Join(parent, "generation")
			if err := os.Mkdir(path, 0700); err != nil {
				t.Fatal(err)
			}
			files := map[string][]byte{}
			for _, name := range forwardingGenerationNames() {
				body := []byte(name + " physical fixture\n")
				file, err := os.OpenFile(filepath.Join(path, name), os.O_RDWR|os.O_CREATE|os.O_EXCL|syscall.O_NOFOLLOW, 0600)
				if err != nil {
					t.Fatal(err)
				}
				n, writeErr := file.Write(body)
				mode := os.FileMode(0444)
				if name == programName {
					mode = 0555
				}
				if err := errors.Join(writeErr, file.Chmod(mode), file.Sync(), file.Close()); err != nil || n != len(body) {
					t.Fatal("original fixture file", name, err)
				}
				files[name] = body
			}
			if err := os.Chmod(path, 0755); err != nil {
				t.Fatal(err)
			}
			file, err := os.OpenFile(path, os.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW, 0)
			if err != nil {
				t.Fatal(err)
			}
			info, err := file.Stat()
			if err != nil || file.Sync() != nil || syncInitialParent(path) != nil {
				file.Close()
				t.Fatal("original fixture directory", err)
			}
			writer, err := openForwardingBirths(t.Context(), filepath.Join(parent, "seal-journal"), map[string]bool{path: true}, "forwarding-recovery-seal-v1")
			if err != nil {
				file.Close()
				writer.close()
				t.Fatal("actual seal writer opening", err)
			}
			defer func() {
				if err := errors.Join(file.Close(), writer.close()); err != nil {
					t.Error(err)
				}
			}()
			native, ok := infoSys(info)
			if !ok {
				t.Fatal("missing original native fixture metadata")
			}
			u := &initialSealContinuation{directory: &inputDirectory{path: path, file: file, info: info}, record: initialRecord{Sequence: 50, Phase: "directory-ready", Path: path, Device: uint64(native.Dev), Inode: native.Ino, Mode: 0755}}
			generation := digest([]byte("physical generation fixture"))
			if err := writer.record("candidate-retained", path, info, generation, 0755, 0, 0); err != nil {
				t.Fatal(err)
			}
			if err := writer.record("continued-directory-seal-intent", path, info, generation, 0555, 0, 0); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			late := errors.New("physical seal late sync control")
			calls := 0
			expectedMode := os.FileMode(0755)
			switch fault {
			case "unknown-entry":
				if err := os.WriteFile(filepath.Join(path, "unknown"), []byte("preserve\n"), 0600); err != nil {
					t.Fatal(err)
				}
			case "mode":
				expectedMode = 0700
				if err := file.Chmod(0700); err != nil {
					t.Fatal(err)
				}
			case "inode":
				if err := os.Rename(path, path+".original"); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(path, 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.Chmod(path, 0755); err != nil {
					t.Fatal(err)
				}
			case "cancel-before":
				cancel()
			case "cancel-after-sync", "late-sync-error":
				expectedMode = 0555
				u.afterSync = func() error {
					calls++
					if fault == "cancel-after-sync" {
						cancel()
						return nil
					}
					return late
				}
			case "complete":
				expectedMode = 0555
			}
			result := u.commit(ctx, files)
			if (fault == "complete") != (result == nil) {
				t.Fatal("physical sealing result", result)
			}
			if fault == "late-sync-error" && !errors.Is(result, late) || (fault == "cancel-before" || fault == "cancel-after-sync") && !errors.Is(result, context.Canceled) {
				t.Fatal("original late error lost", result)
			}
			retained, err := file.Stat()
			if err != nil || !os.SameFile(retained, info) || retained.Mode().Perm() != expectedMode {
				t.Fatal("original descriptor released or inode/access changed", err)
			}
			if err := u.commit(t.Context(), files); err == nil || fault == "late-sync-error" && !errors.Is(err, late) {
				t.Fatal("sealing retried or lost original uncertainty", err)
			}
			if u.afterSync != nil && calls != 1 {
				t.Fatal("physical sync effect repeated", calls)
			}
			if fault == "complete" {
				if !u.sealed || u.directory.info.Mode().Perm() != 0555 {
					t.Fatal("physical result not independently retained")
				}
				if err := writer.record("continued-directory-sealed", path, u.directory.info, generation, 0555, 0, 0); err != nil {
					t.Fatal(err)
				}
			} else if u.sealed {
				t.Fatal("uncertain effect became completed sealing")
			}
			if fault == "unknown-entry" {
				body, err := os.ReadFile(filepath.Join(path, "unknown"))
				if err != nil || string(body) != "preserve\n" {
					t.Fatal("unknown inventory was modified", err)
				}
			}
		})
	}
}

// Only the physical recovery-owned directory birth and joined custody are
// exercised. There is no original process cut, Release or accepting recovery.
func TestForwardingRecoveryPublicationNativeNeverStartedDirectory(t *testing.T) {
	marker, err := os.ReadFile("/etc/ardents-forwarding-preparation-native-stand")
	if err != nil || string(marker) != "ardents-forwarding-preparation-native-v1\n" || os.Geteuid() != 0 || os.Getegid() != 0 {
		t.Fatal("invalid root native stand", err)
	}
	for _, mode := range []string{"complete", "cancel-before-promotion", "late-close"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			parent, err := os.MkdirTemp(os.TempDir(), "frd-")
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := os.RemoveAll(parent); err != nil {
					t.Error(err)
				}
			})
			path, journal := filepath.Join(parent, "attempt"), filepath.Join(parent, "recovery-journal")
			j, err := openForwardingBirths(ctx, journal, map[string]bool{path: true}, "forwarding-recovery-v1")
			if err != nil {
				t.Fatal(err)
			}
			defer j.close()
			calls := 0
			admit := func() error {
				calls++
				if mode == "cancel-before-promotion" && calls == 3 {
					cancel()
				}
				return ctx.Err()
			}
			err = j.directory(path, 0750, 0, 988, admit)
			if mode == "cancel-before-promotion" {
				if !errors.Is(err, context.Canceled) {
					t.Fatal("cancelled promotion accepted", err)
				}
				info, err := os.Lstat(path)
				if err != nil || !nodeAccess(info, 0700, 0, 0, true) {
					t.Fatal("partial original birth changed", err)
				}
				if !errors.Is(j.close(), context.Canceled) {
					t.Fatal("birth cancellation lost")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			info, err := os.Lstat(path)
			if err != nil || !nodeAccess(info, 0750, 0, 988, true) {
				t.Fatal("actual root:Node directory access", err)
			}
			var records []initialRecord
			for _, line := range bytes.Split(bytes.TrimSuffix(j.journal.body, []byte{'\n'}), []byte{'\n'}) {
				var row initialRecord
				if json.Unmarshal(line, &row) != nil {
					t.Fatal("record framing")
				}
				records = append(records, row)
			}
			if len(records) != 4 || records[1].Phase != "directory-intent" || records[1].Mode != 0750 || records[1].GID != 988 || records[2].Phase != "directory-born" || records[2].Mode != 0700 || records[2].GID != 0 || records[3].Phase != "directory-ready" || records[3].Mode != 0750 || records[3].GID != 988 || records[2].Inode != records[3].Inode || records[2].Device != records[3].Device {
				t.Fatal("own birth and same-inode promotion not retained")
			}
			held, resume := make(chan struct{}), make(chan struct{})
			j.beforeClose = func(file *os.File) error {
				if file == j.dirs[path] {
					close(held)
					<-resume
					if mode == "late-close" {
						return file.Close()
					}
				}
				return nil
			}
			done := make(chan error, 1)
			go func() { done <- j.close() }()
			<-held
			probe, err := os.Open(journal)
			if err != nil {
				close(resume)
				<-done
				t.Fatal(err)
			}
			leaseErr := syscall.Flock(int(probe.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
			probeClose := probe.Close()
			close(resume)
			closed := <-done
			if !errors.Is(leaseErr, syscall.EWOULDBLOCK) || probeClose != nil {
				t.Fatal("journal released before directory Close", leaseErr, probeClose)
			}
			if mode == "complete" && closed != nil || mode == "late-close" && !errors.Is(closed, os.ErrClosed) {
				t.Fatal("physical completion lost", closed)
			}
		})
	}
}

// Only physical publication ordering is exercised here. The test has no genuine
// Release, original-cut, manager, installation or domain-completion authority.
func TestForwardingRecoveryPublicationNativeGuardsSpanPhysicalClose(t *testing.T) {
	marker, err := os.ReadFile("/etc/ardents-forwarding-preparation-native-stand")
	if err != nil || string(marker) != "ardents-forwarding-preparation-native-v1\n" || os.Geteuid() != 0 || os.Getegid() != 0 {
		t.Fatal("invalid environment: disposable root native stand required", err)
	}
	for _, group := range []int{0, 988} {
		for _, mode := range []string{"complete", "late-close", "cancel-close", "wrong-parent"} {
			name := mode
			if group != 0 {
				name += "-retirement-parent"
			}
			t.Run(name, func(t *testing.T) {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				parent, err := os.MkdirTemp(os.TempDir(), "frp-")
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					if err := os.RemoveAll(parent); err != nil {
						t.Error(err)
					}
				})
				originalParent := filepath.Join(parent, "installation-root")
				if err := os.Mkdir(originalParent, 0700); err != nil {
					t.Fatal(err)
				}
				if group != 0 {
					if err := os.Chown(originalParent, 0, group); err != nil {
						t.Fatal(err)
					}
					if err := os.Chmod(originalParent, 0750); err != nil {
						t.Fatal(err)
					}
				}
				originalParentInfo, err := os.Lstat(originalParent)
				if err != nil {
					t.Fatal(err)
				}
				originalPath := filepath.Join(originalParent, "original-pending")
				guardPath := filepath.Join(parent, "recovery-pending")
				journal := filepath.Join(parent, "recovery-journal")
				writer, err := openForwardingBirths(ctx, journal, map[string]bool{guardPath: true}, "forwarding-recovery-v1")
				if err != nil {
					t.Fatal(err)
				}
				defer func() { _ = writer.close() }()
				// The original owner has already closed its own guard. Recovery
				// creates only its separate suffix resources in the root parent.
				originalBody := []byte("original blocker fixture\n")
				if err := os.WriteFile(originalPath, originalBody, 0600); err != nil {
					t.Fatal(err)
				}
				originalInfo, err := os.Lstat(originalPath)
				if err != nil {
					t.Fatal(err)
				}
				original := &initialFile{path: originalPath, info: originalInfo, body: originalBody}
				if err := writer.write(guardPath, []byte("recovery blocker fixture\n"), 0600, 0, 0, writer.check); err != nil {
					t.Fatal(err)
				}
				guard := writer.files[len(writer.files)-1]
				parentInfo, err := os.Lstat(parent)
				if err != nil {
					t.Fatal(err)
				}
				publication := &recoveryPublication{births: writer, pending: guard, parent: parentInfo, originalPending: original, originalParent: originalParentInfo}
				if err := publishRecoveryOriginalPending(publication); !errors.Is(err, ErrRepairRequired) {
					t.Fatal("open writer permitted publication", err)
				}
				held, release := make(chan struct{}), make(chan struct{})
				late := errors.New("independent late recovery writer Close failure")
				writer.beforeClose = func(file *os.File) error {
					if file.Name() != journal {
						return nil
					}
					close(held)
					<-release
					if mode == "late-close" {
						return late
					}
					return nil
				}
				done := make(chan error, 1)
				go func() { done <- writer.close() }()
				select {
				case <-held:
				case err := <-done:
					t.Fatal("actual writer Close was not held", err)
				}
				_, originalHeld := os.Lstat(originalPath)
				_, guardHeld := os.Lstat(guardPath)
				if mode == "cancel-close" {
					cancel()
				}
				close(release)
				closed := <-done
				if originalHeld != nil || guardHeld != nil {
					t.Fatal("guard returned before actual Close", originalHeld, guardHeld)
				}
				if mode == "late-close" || mode == "cancel-close" {
					cause := late
					if mode == "cancel-close" {
						cause = context.Canceled
					}
					if !errors.Is(closed, cause) {
						t.Fatal("late original Close/cancellation lost", closed)
					}
					if err := publishRecoveryOriginalPending(publication); err == nil {
						t.Fatal("uncertain suffix released original blocker")
					}
					if _, err := os.Lstat(originalPath); err != nil {
						t.Fatal("original guard lost after refusal", err)
					}
					if _, err := os.Lstat(guardPath); err != nil {
						t.Fatal("recovery guard lost after refusal", err)
					}
					return
				}
				if closed != nil {
					t.Fatal(closed)
				}
				if mode == "wrong-parent" {
					publication.originalParent = parentInfo
					if err := publishRecoveryOriginalPending(publication); err == nil {
						t.Fatal("suffix parent substituted for original blocker parent")
					}
					if _, err := os.Lstat(originalPath); err != nil {
						t.Fatal("original blocker lost", err)
					}
					if _, err := os.Lstat(guardPath); err != nil {
						t.Fatal("suffix blocker lost", err)
					}
					return
				}
				if err := publishRecoveryOriginalPending(publication); err != nil {
					t.Fatal(err)
				}
				if _, err := os.Lstat(originalPath); !errors.Is(err, os.ErrNotExist) {
					t.Fatal("original publication unavailable", err)
				}
				if _, err := os.Lstat(guardPath); err != nil {
					t.Fatal("own guard did not span original publication", err)
				}
				if err := publishInitialPending(writer, guard, parentInfo); err != nil {
					t.Fatal("last own publication unavailable", err)
				}
				if _, err := os.Lstat(guardPath); !errors.Is(err, os.ErrNotExist) {
					t.Fatal("own final guard retained", err)
				}
			})
		}
	}
}

// Native inodes and independent literal control rows test the control codec
// only. No original-cut or Release authority enters this component case.
func TestForwardingRecoveryPublicationNativeControlBindsActualGuard(t *testing.T) {
	marker, err := os.ReadFile("/etc/ardents-forwarding-preparation-native-stand")
	if err != nil || string(marker) != "ardents-forwarding-preparation-native-v1\n" || os.Geteuid() != 0 || os.Getegid() != 0 {
		t.Fatal("invalid environment: disposable root native stand required", err)
	}
	parent, err := os.MkdirTemp(os.TempDir(), "frc-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(parent); err != nil {
			t.Error(err)
		}
	})
	selected, err := decodeSelection(forwardingBindingVector())
	if err != nil {
		t.Fatal(err)
	}
	journal := selected.binding.Request.InstallationRoot + ".forwarding-initial-journal"
	control, result, pending := recoveryPaths(journal)
	sealed := filepath.Join(selected.binding.Request.InstallationRoot, "generations", selected.binding.Generation)
	completion := recoveryCompletion{Journal: journal, Generation: selected.binding.Generation, BindingSHA256: digest(selected.raw), OriginalCutSHA256: digest([]byte("control-only cut fixture"))}
	completionRaw := []byte("control-only completion fixture\n")
	guardRaw := []byte("ardents-forwarding-recovery-pending-v1\n" + completion.OriginalCutSHA256 + "\n")
	fileInfo := func(name string, raw []byte, mode os.FileMode) os.FileInfo {
		t.Helper()
		path := filepath.Join(parent, name)
		if err := os.WriteFile(path, raw, mode); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(path, mode); err != nil {
			t.Fatal(err)
		}
		info, err := os.Lstat(path)
		if err != nil {
			t.Fatal(err)
		}
		return info
	}
	originalInfo := fileInfo("original", []byte("immutable prefix fixture\n"), 0600)
	for _, header := range []string{"forwarding-recovery-unit-create-v1", "forwarding-recovery-selection-create-v1"} {
		t.Run("creation-writer-"+header, func(t *testing.T) {
			// Actual writer and actual synced native leaf only. This is not an
			// original cut, fixed-unit admission, Release or successful recovery.
			path := filepath.Join(parent, "created-writer-fixture-"+header)
			writer, err := openForwardingBirths(t.Context(), filepath.Join(parent, "creation-writer-journal-"+header), map[string]bool{path: true}, header)
			if writer != nil {
				defer writer.close()
			}
			if err != nil {
				t.Fatal("actual creation writer opening", err)
			}
			body := []byte("physical writer fixture\n")
			if err := writer.record("continued-file-intent", path, nil, digest(body), 0644, 0, 0); err != nil {
				t.Fatal("actual creation intent", err)
			}
			file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_EXCL|syscall.O_NOFOLLOW, 0600)
			if err != nil {
				t.Fatal(err)
			}
			defer file.Close()
			born, err := file.Stat()
			if err != nil || file.Sync() != nil || syncInitialParent(path) != nil {
				t.Fatal("actual empty birth", err)
			}
			if err := writer.record("continued-file-born", path, born, digest(nil), 0600, 0, 0); err != nil {
				t.Fatal("actual creation birth", err)
			}
			if _, err := file.Write(body); err != nil {
				t.Fatal(err)
			}
			if err := errors.Join(file.Chmod(0644), file.Sync()); err != nil {
				t.Fatal(err)
			}
			written, err := file.Stat()
			if err != nil || !os.SameFile(born, written) {
				t.Fatal("same physical birth", err)
			}
			if err := writer.record("continued-file-written", path, written, digest(body), 0644, 0, 0); err != nil {
				t.Fatal("actual creation written", err)
			}
			if err := writer.close(); err != nil {
				t.Fatal(err)
			}
			raw, err := os.ReadFile(writer.journal.path)
			if err != nil || !bytes.Equal(raw, writer.journal.body) || len(bytes.Split(bytes.TrimSuffix(raw, []byte{'\n'}), []byte{'\n'})) != 4 {
				t.Fatal("physical writer readback", err)
			}
		})
	}
	resultInfo := fileInfo("completion", completionRaw, 0600)
	t.Run("selection-continuation-writer", func(t *testing.T) {
		// Original empty native fixture plus actual journal append; no cut,
		// fixed installed path, Release or recovery authority is supplied.
		path := filepath.Join(parent, "selection-writer-empty")
		fileInfo("selection-writer-empty", nil, 0600)
		file, err := os.OpenFile(path, os.O_RDWR|syscall.O_NOFOLLOW, 0)
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()
		birth, err := file.Stat()
		if err != nil || errors.Join(file.Sync(), syncInitialParent(path)) != nil {
			t.Fatal("sync original empty fixture", err)
		}
		writer, err := openForwardingBirths(t.Context(), filepath.Join(parent, "selection-writer-journal"), map[string]bool{path: true}, "forwarding-recovery-selection-v1")
		if writer != nil {
			defer writer.close()
		}
		if err != nil {
			t.Fatal("actual selection writer opening", err)
		}
		if err := writer.record("continued-file-retained", path, birth, digest(nil), 0600, 0, 0); err != nil {
			t.Fatal(err)
		}
		if _, err := file.Write(selected.raw); err != nil {
			t.Fatal(err)
		}
		if err := errors.Join(file.Chmod(0644), file.Sync()); err != nil {
			t.Fatal(err)
		}
		written, err := file.Stat()
		if err != nil || !os.SameFile(birth, written) {
			t.Fatal("selection original fixture inode", err)
		}
		if err := writer.record("continued-file-written", path, written, digest(selected.raw), 0644, 0, 0); err != nil {
			t.Fatal(err)
		}
		if err := errors.Join(file.Close(), writer.close()); err != nil {
			t.Fatal(err)
		}
		raw, err := os.ReadFile(writer.journal.path)
		if err != nil || !bytes.Equal(raw, writer.journal.body) {
			t.Fatal("selection physical journal readback", err)
		}
	})
	unitInfo := fileInfo("unit", []byte("unit byte fixture\n"), 0644)
	guardInfo := fileInfo("pending", guardRaw, 0600)
	foreignGuard := fileInfo("foreign-pending", guardRaw, 0600)
	_ = fileInfo("control", []byte("not yet complete\n"), 0600)
	if err := os.Mkdir(filepath.Join(parent, "candidate"), 0555); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(parent, "candidate"), 0555); err != nil {
		t.Fatal(err)
	}
	candidateInfo, err := os.Lstat(filepath.Join(parent, "candidate"))
	if err != nil {
		t.Fatal(err)
	}
	cInfo, err := os.Lstat(filepath.Join(parent, "control"))
	if err != nil {
		t.Fatal(err)
	}
	c, _ := infoSys(cInfo)
	r, _ := infoSys(resultInfo)
	o, _ := infoSys(originalInfo)
	d, _ := infoSys(candidateInfo)
	u, _ := infoSys(unitInfo)
	g, _ := infoSys(guardInfo)
	rows := []initialRecord{
		{Phase: "forwarding-recovery-v1", Path: control, Device: uint64(c.Dev), Inode: c.Ino, Mode: 0600},
		{Phase: "file-intent", Path: pending, SHA256: digest(guardRaw), Mode: 0600},
		{Phase: "file-born", Path: pending, Device: uint64(g.Dev), Inode: g.Ino, Mode: 0600},
		{Phase: "file-ready", Path: pending, Device: uint64(g.Dev), Inode: g.Ino, SHA256: digest(guardRaw), Mode: 0600},
		{Phase: "original-verified", Path: journal, Device: uint64(o.Dev), Inode: o.Ino, SHA256: completion.OriginalCutSHA256, Mode: 0600},
		{Phase: "candidate-verified", Path: sealed, Device: uint64(d.Dev), Inode: d.Ino, SHA256: completion.BindingSHA256, Mode: 0555},
		{Phase: "stopped-verified", Path: "/etc/systemd/system/ardents-forwarding.service", Device: uint64(u.Dev), Inode: u.Ino, SHA256: selected.binding.UnitSHA256, Mode: 0644},
		{Phase: "file-intent", Path: result, SHA256: digest(completionRaw), Mode: 0600},
		{Phase: "file-born", Path: result, Device: uint64(r.Dev), Inode: r.Ino, Mode: 0600},
		{Phase: "file-ready", Path: result, Device: uint64(r.Dev), Inode: r.Ino, SHA256: digest(completionRaw), Mode: 0600},
		{Phase: "recovery-completed", Path: result, Device: uint64(r.Dev), Inode: r.Ino, SHA256: digest(completionRaw), Mode: 0600},
	}
	encode := func(rows []initialRecord) ([]byte, os.FileInfo) {
		t.Helper()
		var raw []byte
		for n, row := range rows {
			row.Sequence = n
			line, err := json.Marshal(row)
			if err != nil {
				t.Fatal(err)
			}
			raw = append(raw, line...)
			raw = append(raw, '\n')
		}
		info := fileInfo("control", raw, 0600)
		return raw, info
	}
	raw, controlInfo := encode(rows)
	check := func(raw []byte, controlInfo, guard os.FileInfo) error {
		return decodeRecoveryControl(raw, completionRaw, completion, selected, controlInfo, resultInfo, originalInfo, candidateInfo, unitInfo, guard)
	}
	if err := check(raw, controlInfo, guardInfo); err != nil {
		t.Fatal("independent exact native control", err)
	}
	if err := check(raw, controlInfo, nil); err != nil {
		t.Fatal("published historical control", err)
	}
	if err := check(raw, controlInfo, foreignGuard); err == nil {
		t.Fatal("same bytes in another guard inode admitted")
	}
	bad := append([]initialRecord(nil), rows...)
	bad[2].Inode++
	bad[3].Inode++
	changed, changedInfo := encode(bad)
	if err := check(changed, changedInfo, guardInfo); err == nil {
		t.Fatal("internally consistent invented guard birth admitted")
	}
	t.Run("initial-reload-control", func(t *testing.T) {
		beforeRaw := nativeContextByteVector()
		afterRaw := bytes.Replace(beforeRaw, []byte(`"inode":101,`), []byte(`"inode":102,`), 1)
		if bytes.Equal(beforeRaw, afterRaw) {
			t.Fatal("missing generator change")
		}
		beforePath, afterPath := recoveryReloadPaths(journal)
		proof := &recoveryReloadProof{before: &initialFile{path: beforePath, body: beforeRaw, info: fileInfo("before", beforeRaw, 0600)}, after: &initialFile{path: afterPath, body: afterRaw, info: fileInfo("after", afterRaw, 0600)}}
		triple := func(file *initialFile) []initialRecord {
			native, _ := infoSys(file.info)
			return []initialRecord{{Phase: "file-intent", Path: file.path, SHA256: digest(file.body), Mode: 0600}, {Phase: "file-born", Path: file.path, Device: uint64(native.Dev), Inode: native.Ino, Mode: 0600}, {Phase: "file-ready", Path: file.path, Device: uint64(native.Dev), Inode: native.Ino, SHA256: digest(file.body), Mode: 0600}}
		}
		reloadRows := append([]initialRecord(nil), rows[:6]...)
		reloadRows[0].Phase = "forwarding-recovery-reload-v1"
		reloadRows = append(reloadRows, triple(proof.before)...)
		reloadRows = append(reloadRows, initialRecord{Phase: "recovery-reload-intent", Path: "/etc/systemd/system/ardents-forwarding.service", SHA256: selected.binding.UnitSHA256, Mode: 0644})
		reloadRows = append(reloadRows, triple(proof.after)...)
		reloadRows = append(reloadRows, rows[6:]...)
		reloaded := completion
		reloaded.Schema = "ardents-forwarding-recovery-completion-v2"
		reloaded.Operation = "initial"
		reloaded.NativeContextSHA256 = digest(afterRaw)
		checkReload := func(rows []initialRecord, completion recoveryCompletion, proof *recoveryReloadProof) error {
			raw, info := encode(rows)
			return decodeRecoveryControlProof(raw, completionRaw, completion, selected, info, resultInfo, originalInfo, candidateInfo, unitInfo, guardInfo, nil, proof)
		}
		if len(reloadRows) != 18 {
			t.Fatal("independent reload grammar count")
		}
		if err := checkReload(reloadRows, reloaded, proof); err != nil {
			t.Fatal("independent complete native control", err)
		}
		for count := 0; count < 18; count++ {
			if err := checkReload(reloadRows[:count], reloaded, proof); err == nil {
				t.Fatal("partial recovery supplied Reload completion", count)
			}
		}
		if err := checkReload(reloadRows, reloaded, nil); err == nil {
			t.Fatal("missing native files supplied Reload proof")
		}
		wrong := reloaded
		wrong.NativeContextSHA256 = digest(beforeRaw)
		if err := checkReload(reloadRows, wrong, proof); err == nil {
			t.Fatal("after context not committed by completion")
		}
		for _, position := range []int{6, 7, 8, 9, 10, 11, 12} {
			bad := append([]initialRecord(nil), reloadRows...)
			bad[position].Path += "-foreign"
			if err := checkReload(bad, reloaded, proof); err == nil {
				t.Fatal("foreign reload birth or effect", position)
			}
		}
		t.Run("replacement-canonical-after", func(t *testing.T) {
			replacement := reloaded
			replacement.Operation = "replacement"
			if err := checkReload(reloadRows, replacement, proof); err == nil {
				t.Fatal("Initial after path supplied Replacement birth")
			}
			_, canonicalAfter := replacementNativePaths(journal)
			afterFile := *proof.after
			afterFile.path = canonicalAfter
			own := &recoveryReloadProof{before: proof.before, after: &afterFile}
			canonical := append([]initialRecord(nil), reloadRows...)
			for _, position := range []int{10, 11, 12} {
				canonical[position].Path = canonicalAfter
			}
			if err := checkReload(canonical, replacement, own); err != nil {
				t.Fatal("separately recorded canonical recovery birth", err)
			}
			canonical[11].Inode++
			if err := checkReload(canonical, replacement, own); err == nil {
				t.Fatal("invented canonical recovery birth inode")
			}
		})
		t.Run("initial-recorded-unit-birth", func(t *testing.T) {
			native, _ := infoSys(unitInfo)
			birth := initialRecord{Sequence: 115, Phase: "file-born", Path: "/etc/systemd/system/ardents-forwarding.service", Device: uint64(native.Dev), Inode: native.Ino, Mode: 0600}
			unitProof := &recoveryReloadProof{before: proof.before, after: proof.after, unitBirth: &birth}
			unitCompletion := reloaded
			unitCompletion.PrefixRecords = 116
			unitRows := append([]initialRecord(nil), reloadRows[:6]...)
			unitRows[0].Phase = "forwarding-recovery-unit-v1"
			unitRows = append(unitRows, initialRecord{Phase: "continued-file-retained", Path: birth.Path, Device: birth.Device, Inode: birth.Inode, SHA256: digest(nil), Mode: 0600}, initialRecord{Phase: "continued-file-written", Path: birth.Path, Device: birth.Device, Inode: birth.Inode, SHA256: selected.binding.UnitSHA256, Mode: 0644})
			unitRows = append(unitRows, reloadRows[6:]...)
			if len(unitRows) != 20 {
				t.Fatal("independent unit continuation count")
			}
			if err := checkReload(unitRows, unitCompletion, unitProof); err != nil {
				t.Fatal("original born unit continuation", err)
			}
			if err := checkReload(unitRows, unitCompletion, proof); err == nil {
				t.Fatal("missing original birth waived")
			}
			for _, position := range []int{6, 7} {
				bad := append([]initialRecord(nil), unitRows...)
				bad[position].Inode++
				if err := checkReload(bad, unitCompletion, unitProof); err == nil {
					t.Fatal("substituted unit inode", position)
				}
			}
			badBirth := birth
			badBirth.Inode++
			badProof := *unitProof
			badProof.unitBirth = &badBirth
			if err := checkReload(unitRows, unitCompletion, &badProof); err == nil {
				t.Fatal("foreign original unit birth accepted")
			}
		})
		t.Run("initial-recovery-created-unit", func(t *testing.T) {
			native, _ := infoSys(unitInfo)
			birth := initialRecord{Sequence: 115, Phase: "file-born", Path: "/etc/systemd/system/ardents-forwarding.service", Device: uint64(native.Dev), Inode: native.Ino, Mode: 0600}
			createdProof := &recoveryReloadProof{before: proof.before, after: proof.after, unitCreation: &birth}
			for count := uint32(72); count <= 115; count++ {
				created := reloaded
				ownProof := *createdProof
				created.PrefixRecords = count
				created.Records = []json.RawMessage{}
				var generationRows []initialRecord
				if count <= 104 {
					for i, name := range []string{"ardents-forwarding.service", "ardents-node-linux-amd64", "binding.json", "forwarding-plan.json", "forwarding-request.json", "infrastructure-enrollment.json", "infrastructure-forwarding.json", "rendered-forwarding.service", "root.json", "snapshot.json", "targets.json", "timestamp.json"} {
						sequence := 73 + 3*i
						if int(count) >= sequence+2 {
							continue
						}
						body := []byte(name + " original artifact byte fixture\n")
						mode := os.FileMode(0444)
						if name == "ardents-node-linux-amd64" {
							mode = 0555
						}
						info := fileInfo("generation-"+name, body, mode)
						native, _ := infoSys(info)
						path := filepath.Join(sealed, name)
						born := initialRecord{Sequence: sequence, Phase: "file-born", Path: path, Device: uint64(native.Dev), Inode: native.Ino, Mode: 0600}
						retained := int(count) == sequence+1
						ownProof.artifacts = append(ownProof.artifacts, initialArtifactProof{birth: born, file: &initialFile{path: path, info: info, body: body}, retained: retained})
						if !retained {
							if int(count) <= sequence-1 {
								intent, _ := json.Marshal(initialRecord{Sequence: sequence - 1, Phase: "file-intent", Path: path, SHA256: digest(body), Mode: uint32(mode)})
								created.Records = append(created.Records, intent)
							}
							raw, _ := json.Marshal(born)
							created.Records = append(created.Records, raw)
							generationRows = append(generationRows, initialRecord{Phase: "continued-artifact-intent", Path: path, SHA256: digest(body), Mode: uint32(mode)}, initialRecord{Phase: "continued-artifact-born", Path: path, Device: born.Device, Inode: born.Inode, SHA256: digest(nil), Mode: 0600})
						} else {
							generationRows = append(generationRows, initialRecord{Phase: "continued-artifact-retained", Path: path, Device: born.Device, Inode: born.Inode, SHA256: digest(nil), Mode: 0600})
						}
						ready, _ := json.Marshal(initialRecord{Sequence: sequence + 1, Phase: "file-ready", Path: path, Device: born.Device, Inode: born.Inode, SHA256: digest(body), Mode: uint32(mode)})
						created.Records = append(created.Records, ready)
						generationRows = append(generationRows, initialRecord{Phase: "continued-artifact-written", Path: path, Device: born.Device, Inode: born.Inode, SHA256: digest(body), Mode: uint32(mode)})
					}
				}
				var artifactBirth initialRecord
				if count >= 105 && count <= 107 {
					path := filepath.Join(sealed, "timestamp.json")
					body := []byte("original timestamp public byte fixture\n")
					info := fileInfo("timestamp107", body, 0444)
					native, _ := infoSys(info)
					artifactBirth = initialRecord{Sequence: 106, Phase: "file-born", Path: path, Device: uint64(native.Dev), Inode: native.Ino, Mode: 0600}
					ownProof.artifactFile = &initialFile{path: path, info: info, body: body}
					if count == 107 {
						ownProof.artifactBirth = &artifactBirth
					} else {
						ownProof.artifactCreation = &artifactBirth
						if count == 105 {
							raw, _ := json.Marshal(initialRecord{Sequence: 105, Phase: "file-intent", Path: path, SHA256: digest(body), Mode: 0444})
							created.Records = append(created.Records, raw)
						}
						raw, _ := json.Marshal(artifactBirth)
						created.Records = append(created.Records, raw)
					}
					raw, _ := json.Marshal(initialRecord{Sequence: 107, Phase: "file-ready", Path: path, Device: artifactBirth.Device, Inode: artifactBirth.Inode, SHA256: digest(body), Mode: 0444})
					created.Records = append(created.Records, raw)
				}
				var originalDirectory initialRecord
				if count <= 109 {
					native, _ := infoSys(candidateInfo)
					originalDirectory = initialRecord{Sequence: 50, Phase: "directory-ready", Path: sealed, Device: uint64(native.Dev), Inode: native.Ino, Mode: 0755}
					ownProof.generationDirectory = &originalDirectory
					if count <= 108 {
						intent, _ := json.Marshal(initialRecord{Sequence: 108, Phase: "seal-intent", Path: sealed, Device: originalDirectory.Device, Inode: originalDirectory.Inode, SHA256: selected.binding.Generation, Mode: 0555})
						created.Records = append(created.Records, intent)
					}
					sealedRow, _ := json.Marshal(initialRecord{Sequence: 109, Phase: "sealed", Path: sealed, Device: originalDirectory.Device, Inode: originalDirectory.Inode, SHA256: selected.binding.Generation, Mode: 0555})
					created.Records = append(created.Records, sealedRow)
				}
				var selectionBirth initialRecord
				if count <= 112 {
					path := filepath.Join(selected.binding.Request.InstallationRoot, "binding.json")
					info := fileInfo("selection112", selected.raw, 0644)
					native, _ := infoSys(info)
					selectionBirth = initialRecord{Sequence: 111, Phase: "file-born", Path: path, Device: uint64(native.Dev), Inode: native.Ino, Mode: 0600}
					if count <= 111 {
						ownProof.selectionCreation = &selectionBirth
						if count <= 110 {
							intent, _ := json.Marshal(initialRecord{Sequence: 110, Phase: "file-intent", Path: path, SHA256: digest(selected.raw), Mode: 0644})
							created.Records = append(created.Records, intent)
						}
						born, _ := json.Marshal(selectionBirth)
						created.Records = append(created.Records, born)
					} else {
						ownProof.selectionBirth = &selectionBirth
					}
					ownProof.selectionFile = &initialFile{path: path, info: info, body: selected.raw}
					ready, _ := json.Marshal(initialRecord{Sequence: 112, Phase: "file-ready", Path: path, Device: selectionBirth.Device, Inode: selectionBirth.Inode, SHA256: digest(selected.raw), Mode: 0644})
					created.Records = append(created.Records, ready)
				}
				if count <= 113 {
					selectedRow, _ := json.Marshal(initialRecord{Sequence: 113, Phase: "selected", Path: filepath.Join(selected.binding.Request.InstallationRoot, "binding.json"), SHA256: digest(selected.raw), Mode: 0644})
					created.Records = append(created.Records, selectedRow)
				}
				if count <= 114 {
					intent, _ := json.Marshal(initialRecord{Sequence: 114, Phase: "file-intent", Path: birth.Path, SHA256: selected.binding.UnitSHA256, Mode: 0644})
					created.Records = append(created.Records, intent)
				}
				rawBirth, _ := json.Marshal(birth)
				created.Records = append(created.Records, rawBirth)
				createdRows := append([]initialRecord(nil), reloadRows[:6]...)
				createdRows[0].Phase = "forwarding-recovery-unit-create-v1"
				createdRows = append(createdRows, initialRecord{Phase: "continued-file-intent", Path: birth.Path, SHA256: selected.binding.UnitSHA256, Mode: 0644}, initialRecord{Phase: "continued-file-born", Path: birth.Path, Device: birth.Device, Inode: birth.Inode, SHA256: digest(nil), Mode: 0600}, initialRecord{Phase: "continued-file-written", Path: birth.Path, Device: birth.Device, Inode: birth.Inode, SHA256: selected.binding.UnitSHA256, Mode: 0644})
				createdRows = append(createdRows, reloadRows[6:]...)
				wantedCount := 21
				if count <= 112 {
					wantedCount = 23
					prefix := append([]initialRecord(nil), createdRows[:6]...)
					prefix[0].Phase = "forwarding-recovery-selection-v1"
					if count <= 111 {
						wantedCount = 24
						prefix[0].Phase = "forwarding-recovery-selection-create-v1"
						prefix = append(prefix, initialRecord{Phase: "continued-file-intent", Path: selectionBirth.Path, SHA256: digest(selected.raw), Mode: 0644}, initialRecord{Phase: "continued-file-born", Path: selectionBirth.Path, Device: selectionBirth.Device, Inode: selectionBirth.Inode, SHA256: digest(nil), Mode: 0600})
					} else {
						prefix = append(prefix, initialRecord{Phase: "continued-file-retained", Path: selectionBirth.Path, Device: selectionBirth.Device, Inode: selectionBirth.Inode, SHA256: digest(nil), Mode: 0600})
					}
					prefix = append(prefix, initialRecord{Phase: "continued-file-written", Path: selectionBirth.Path, Device: selectionBirth.Device, Inode: selectionBirth.Inode, SHA256: digest(selected.raw), Mode: 0644})
					createdRows = append(prefix, createdRows[6:]...)
				}
				if count <= 109 {
					wantedCount = 26
					prefix := append([]initialRecord(nil), createdRows[:6]...)
					prefix[0].Phase = "forwarding-recovery-seal-v1"
					prefix[5].Phase, prefix[5].Mode = "candidate-retained", 0755
					prefix = append(prefix, initialRecord{Phase: "continued-directory-seal-intent", Path: sealed, Device: originalDirectory.Device, Inode: originalDirectory.Inode, SHA256: selected.binding.Generation, Mode: 0555}, initialRecord{Phase: "continued-directory-sealed", Path: sealed, Device: originalDirectory.Device, Inode: originalDirectory.Inode, SHA256: selected.binding.Generation, Mode: 0555})
					createdRows = append(prefix, createdRows[6:]...)
				}
				if count >= 105 && count <= 107 {
					file := ownProof.artifactFile
					prefix := append([]initialRecord(nil), createdRows[:6]...)
					prefix[0].Phase = "forwarding-recovery-artifact-v1"
					if count == 107 {
						wantedCount = 28
						prefix = append(prefix, initialRecord{Phase: "continued-artifact-retained", Path: file.path, Device: artifactBirth.Device, Inode: artifactBirth.Inode, SHA256: digest(nil), Mode: 0600})
					} else {
						wantedCount = 29
						prefix[0].Phase = "forwarding-recovery-artifact-create-v1"
						prefix = append(prefix, initialRecord{Phase: "continued-artifact-intent", Path: file.path, SHA256: digest(file.body), Mode: 0444}, initialRecord{Phase: "continued-artifact-born", Path: file.path, Device: artifactBirth.Device, Inode: artifactBirth.Inode, SHA256: digest(nil), Mode: 0600})
					}
					prefix = append(prefix, initialRecord{Phase: "continued-artifact-written", Path: file.path, Device: artifactBirth.Device, Inode: artifactBirth.Inode, SHA256: digest(file.body), Mode: 0444})
					createdRows = append(prefix, createdRows[6:]...)
				}
				if count <= 104 {
					prefix := append([]initialRecord(nil), createdRows[:6]...)
					prefix[0].Phase = "forwarding-recovery-generation-v1"
					prefix = append(prefix, generationRows...)
					createdRows = append(prefix, createdRows[6:]...)
					wantedCount += len(generationRows)
				}
				if len(createdRows) != wantedCount {
					t.Fatal("independent created-unit grammar count")
				}
				if err := checkReload(createdRows, created, &ownProof); err != nil {
					t.Fatal("separate actual unit birth", count, err)
				}
				if count <= 104 {
					for i := range ownProof.artifacts {
						for _, fault := range []string{"missing-file", "inode", "origin", "bytes"} {
							bad := ownProof
							bad.artifacts = append([]initialArtifactProof(nil), ownProof.artifacts...)
							switch fault {
							case "missing-file":
								bad.artifacts[i].file = nil
							case "inode":
								bad.artifacts[i].birth.Inode++
							case "origin":
								bad.artifacts[i].retained = !bad.artifacts[i].retained
							case "bytes":
								file := *bad.artifacts[i].file
								file.body = append(bytes.Clone(file.body), '!')
								bad.artifacts[i].file = &file
							}
							if err := checkReload(createdRows, created, &bad); err == nil {
								t.Fatal("unbound generation artifact accepted", count, i, fault)
							}
						}
					}
				}
				if count >= 105 && count <= 107 {
					for _, fault := range []string{"missing-timestamp", "timestamp-inode", "timestamp-bytes", "timestamp-mode", "missing-timestamp-birth"} {
						badProof := ownProof
						changedFile := *ownProof.artifactFile
						badBirth := artifactBirth
						switch fault {
						case "missing-timestamp":
							badProof.artifactFile = nil
						case "timestamp-inode":
							badBirth.Inode++
							if count == 107 {
								badProof.artifactBirth = &badBirth
							} else {
								badProof.artifactCreation = &badBirth
							}
						case "timestamp-bytes":
							changedFile.body = []byte("foreign timestamp\n")
							badProof.artifactFile = &changedFile
						case "timestamp-mode":
							changedFile.info = fileInfo("wrong-timestamp-mode", changedFile.body, 0644)
							badProof.artifactFile = &changedFile
						case "missing-timestamp-birth":
							badProof.artifactBirth, badProof.artifactCreation = nil, nil
						}
						if err := checkReload(createdRows, created, &badProof); err == nil {
							t.Fatal("unproved timestamp continuation", count, fault)
						}
					}
				}
				if count <= 109 {
					for _, fault := range []string{"missing-directory", "directory-inode", "synthetic-sealed-before-state"} {
						badProof := ownProof
						wrong := originalDirectory
						switch fault {
						case "missing-directory":
							badProof.generationDirectory = nil
						case "directory-inode":
							wrong.Inode++
							badProof.generationDirectory = &wrong
						case "synthetic-sealed-before-state":
							wrong.Mode = 0555
							badProof.generationDirectory = &wrong
						}
						if err := checkReload(createdRows, created, &badProof); err == nil {
							t.Fatal("unproved original directory", fault)
						}
					}
					bad := append([]initialRecord(nil), createdRows...)
					bad[5].Mode = 0555
					if err := checkReload(bad, created, &ownProof); err == nil {
						t.Fatal("synthetic already-sealed candidate before state")
					}
				}
				if count <= 112 {
					for _, fault := range []string{"missing-selection", "selection-birth", "selection-bytes"} {
						badProof := ownProof
						switch fault {
						case "missing-selection":
							badProof.selectionBirth = nil
							badProof.selectionCreation = nil
						case "selection-birth":
							wrong := selectionBirth
							wrong.Inode++
							if count <= 111 {
								badProof.selectionCreation = &wrong
							} else {
								badProof.selectionBirth = &wrong
							}
						case "selection-bytes":
							wrong := *ownProof.selectionFile
							wrong.body = []byte("foreign selection\n")
							badProof.selectionFile = &wrong
						}
						if err := checkReload(createdRows, created, &badProof); err == nil {
							t.Fatal("unproved original selection continuation", fault)
						}
					}
				}
				for n := range createdRows {
					bad := append([]initialRecord(nil), createdRows...)
					bad[n].Path += "-foreign"
					if err := checkReload(bad, created, &ownProof); err == nil {
						t.Fatal("foreign creation/effect row", count, n)
					}
				}
				bad := created
				bad.Records = nil
				if err := checkReload(createdRows, bad, &ownProof); err == nil {
					t.Fatal("completion failed to commit created birth", count)
				}
				wrong := birth
				wrong.Inode++
				wrongProof := ownProof
				wrongProof.unitCreation = &wrong
				if err := checkReload(createdRows, created, &wrongProof); err == nil {
					t.Fatal("different physical created unit", count)
				}
				if err := checkReload(createdRows, created, proof); err == nil {
					t.Fatal("missing actual creation proof", count)
				}
			}
		})
	})
}
