//go:build linux && amd64 && network_signing_native

package infrastructure

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
)

// A real native journal inode with a deliberately unsupported grammar vector.
// It constructs no account, Source, Release, manager or installed admission.
func prepareSourceEarlyPrefixNative(t *testing.T, root string, count int) string {
	t.Helper()
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "original-journal")
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL|syscall.O_NOFOLLOW, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if file != nil {
			if err := file.Close(); err != nil {
				t.Error(err)
			}
		}
	}()
	raw, _, _ := sourceInitialRecordVector(t, true)
	records, err := decodeSourceRecoveryPrefix(raw, sourceRecoverableInitialRecords)
	if err != nil {
		t.Fatal(err)
	}
	info, err := file.Stat()
	if err != nil {
		t.Fatal(err)
	}
	native := info.Sys().(*syscall.Stat_t)
	records[0] = sourceRecord{Phase: sourceRecoverableInitialPhase, Path: path, Device: uint64(native.Dev), Inode: native.Ino, Mode: 0600}
	var body []byte
	for _, record := range records[:count] {
		line, err := json.Marshal(record)
		if err != nil {
			t.Fatal(err)
		}
		body = append(body, append(line, '\n')...)
	}
	n, writeErr := file.Write(body)
	joined := errors.Join(writeErr, file.Sync(), file.Close())
	file = nil
	if joined != nil || n != len(body) {
		t.Fatal("real native prefix fixture write/join", joined)
	}
	if err := os.WriteFile(filepath.Join(root, "release-lock"), []byte("native file fixture only\n"), 0600); err != nil {
		t.Fatal(err)
	}
	return sourcePhysicalSHA(body)
}

func runSourceEarlyPrefixNative(t *testing.T, root, kind string) {
	t.Helper()
	path := filepath.Join(root, "original-journal")
	switch kind {
	case "read":
		prefix, err := openSourceRecoveryPrefix(context.Background(), path, "initial")
		if err != nil {
			t.Fatal(err)
		}
		if sourceInitialRecoveryPrefixSupported(prefix.records) {
			t.Fatal("unsupported native fixture became continuation")
		}
		if err := prefix.close(); err != nil {
			t.Fatal(err)
		}
	case "unattempted":
		return
	case "unlocked":
		if _, err := os.ReadFile(path); err != nil {
			t.Fatal(err)
		}
	case "partial", "foreign", "released", "shared":
		if kind == "foreign" {
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			path += ".foreign"
			if err := os.WriteFile(path, raw, 0600); err != nil {
				t.Fatal(err)
			}
		}
		file, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
		if err != nil {
			t.Fatal(err)
		}
		if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
			t.Fatal(errors.Join(err, file.Close()))
		}
		if kind == "released" || kind == "shared" {
			operation := syscall.LOCK_UN
			if kind == "shared" {
				operation = syscall.LOCK_SH | syscall.LOCK_NB
			}
			if err := syscall.Flock(int(file.Fd()), operation); err != nil {
				t.Fatal(errors.Join(err, file.Close()))
			}
		}
		var readErr error
		if kind == "partial" {
			_, readErr = file.Read(make([]byte, 1))
		} else {
			_, readErr = io.ReadAll(file)
		}
		if err := errors.Join(readErr, file.Close()); err != nil {
			t.Fatal(err)
		}
	default:
		t.Fatal("closed native early prefix fixture required")
	}
}

// This inspects a real canonical ELF. The format-only pins and absent Source
// root deliberately provide neither installed execution nor Release authority.
func TestSourceEarlyPrefixProfileInspectionNative(t *testing.T) {
	program := os.Getenv("ARDENTS_OBSERVER_INSPECT_PROGRAM")
	if !sourcePath(program) {
		t.Fatal("actual offline canonical ELF required")
	}
	programRaw, err := os.ReadFile(program)
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(t.TempDir(), "uncreated-source")
	zero := strings.Repeat("0", 64)
	load := func(p sourceRefusalObserverProfile) (*sourceRefusalObserverProfile, error) {
		t.Helper()
		raw, err := json.Marshal(p)
		if err != nil {
			t.Fatal(err)
		}
		raw = append(raw, '\n')
		path := filepath.Join(t.TempDir(), "early-prefix.json")
		if err := os.WriteFile(path, raw, 0600); err != nil {
			t.Fatal(err)
		}
		return readSourceRefusalObserverProfile(path, sourcePhysicalSHA(raw), program, root)
	}
	for _, count := range []int{24, 42} {
		t.Run(strconv.Itoa(count), func(t *testing.T) {
			p := sourceRefusalObserverProfile{Schema: "source-early-prefix-recovery-refusal-observer-v1", ProgramSHA256: sourcePhysicalSHA(programRaw), PublicReceiptSHA256: zero,
				Arguments: []string{"installation", "recover-source", filepath.Dir(program), "/testtmp/history", "alpha", "one-network", "/testtmp/request", root + ".source-initial-journal"}, JournalSHA256: zero, JournalMode: "0600", EarlyInitialPrefix: count}
			admitted, err := load(p)
			if err != nil {
				t.Fatal(err)
			}
			result, err := observeSourceProcess(program, root, "", "refusal-closed", nil, true, 2, nil, admitted)
			if err != nil || !result.InspectionOnly || result.RootPID != 0 || result.InstalledExecution || result.Entry == 0 || result.RefusalAdmissionEntry == 0 || result.RefusalEarlyPrefixObserved || result.RefusalEarlyPrefixClosed || result.RefusalOwnershipReleasedBeforeExit {
				t.Fatal("offline prefix inspection executed or claimed native custody", result, err)
			}
			for name, mutate := range map[string]func(*sourceRefusalObserverProfile){
				"other-count":        func(p *sourceRefusalObserverProfile) { p.EarlyInitialPrefix++ },
				"missing-count":      func(p *sourceRefusalObserverProfile) { p.EarlyInitialPrefix = 0 },
				"base-purpose":       func(p *sourceRefusalObserverProfile) { p.Schema = "source-recovery-refusal-observer-v1" },
				"missing-cut":        func(p *sourceRefusalObserverProfile) { p.MissingOriginalCut = true },
				"held-journal":       func(p *sourceRefusalObserverProfile) { p.ExternalJournalLease = true },
				"held-release":       func(p *sourceRefusalObserverProfile) { p.ExternalReleaseLease = true },
				"startup-pending":    func(p *sourceRefusalObserverProfile) { p.StartupPending = &sourcePendingGuardProfile{} },
				"retirement-pending": func(p *sourceRefusalObserverProfile) { p.RetirementPending = &sourcePendingGuardProfile{} },
				"recovery-hold":      func(p *sourceRefusalObserverProfile) { p.RecoveryHold = &sourceRecoveryHoldProfile{} },
				"damaged-access":     func(p *sourceRefusalObserverProfile) { p.JournalMode = "0640" },
				"replacement": func(p *sourceRefusalObserverProfile) {
					p.Arguments[7] = root + ".source-replacement-" + zero + "-journal"
				},
			} {
				changed := p
				changed.Arguments = append([]string(nil), p.Arguments...)
				mutate(&changed)
				if _, err := load(changed); err == nil {
					t.Fatal("mixed early-prefix purpose accepted", name)
				}
			}
		})
	}
	if _, err := os.Lstat(root); !os.IsNotExist(err) {
		t.Fatal("offline early-prefix inspection touched Source root", err)
	}
}
