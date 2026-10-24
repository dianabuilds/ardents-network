//go:build linux && amd64 && network_signing_native

package infrastructure

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

// Inspect the real canonical ELF only. These format pins supply no Release,
// Source provenance, accepting completion or human bootstrap acceptance.
func TestSourceCanonicalPhysicalProfileInspectionNative(t *testing.T) {
	program := os.Getenv("ARDENTS_OBSERVER_INSPECT_PROGRAM")
	if !sourcePath(program) {
		t.Fatal("actual offline canonical ELF required")
	}
	programRaw, err := os.ReadFile(program)
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(t.TempDir(), "uncreated-source")
	zero, generation := strings.Repeat("0", 64), strings.Repeat("1", 64)
	for _, operation := range []string{"initial", "replacement"} {
		windows := []string{"file-born-before-record", "file-ready-before-record", "file-born-before-journal-sync", "seal-before-sealed", "binding-before-record", "unit-before-record", "reload-before-verified", "journal-complete-before-close", "pending-before-unlink"}
		if operation == "initial" {
			windows = append(windows, "account-before-complete-birth", "directory-born-before-record")
		}
		for _, window := range windows {
			t.Run(operation+"/"+window, func(t *testing.T) {
				count, journal, arguments := 70, root+".source-initial-journal", []string{"installation", "install-initial-stopped", filepath.Dir(program), zero, "/testtmp/history", "alpha", "one-network", "/testtmp/request"}
				if operation == "replacement" {
					count, journal = 13, root+".source-replacement-"+generation+"-journal"
					arguments = []string{"installation", "replace-source", filepath.Dir(program), "/testtmp/history", "alpha", "one-network", filepath.Join(root, "binding.json"), "/testtmp/request"}
				}
				phase := "file-born"
				if window == "account-before-complete-birth" {
					count, phase = 24, "account-born"
				}
				if window == "directory-born-before-record" {
					count, phase = 42, "directory-intent"
				}
				if window == "file-born-before-record" {
					count, phase = count-1, "file-intent"
				}
				if window == "seal-before-sealed" {
					count, phase = 48, "seal-intent"
					if operation == "initial" {
						count = 105
					}
				}
				if window == "binding-before-record" || window == "unit-before-record" {
					count, phase = 50, "selection-intent"
					if window == "unit-before-record" {
						count, phase = 52, "unit-intent"
					}
					if operation == "initial" {
						count, phase = 109, "file-ready"
						if window == "unit-before-record" {
							count = 113
						}
					}
				}
				if window == "reload-before-verified" || window == "journal-complete-before-close" || window == "pending-before-unlink" {
					count, phase = 54, "reload-intent"
					if operation == "initial" {
						count = 114
					}
					if window != "reload-before-verified" {
						count, phase = count+1, "replacement-verified"
						if operation == "initial" {
							phase = "stopped-verified"
						}
					}
				}
				profile := sourceCanonicalPhysicalProfile{sourceCutObserverProfile: sourceCutObserverProfile{Schema: "source-physical-cut-observer-v1", ProgramSHA256: sourcePhysicalSHA(programRaw), PublicReceiptSHA256: zero,
					Arguments: arguments, Journal: journal, PrefixRecords: count, LastPhase: phase}, Window: window, Generation: generation}
				load := func(raw []byte) (*sourceCanonicalPhysicalProfile, error) {
					t.Helper()
					path := filepath.Join(t.TempDir(), "physical.json")
					if err := os.WriteFile(path, raw, 0600); err != nil {
						t.Fatal(err)
					}
					return readSourceCanonicalPhysicalProfile(path, sourcePhysicalSHA(raw), program, root)
				}
				encode := func(p sourceCanonicalPhysicalProfile) []byte {
					t.Helper()
					raw, err := json.Marshal(p)
					if err != nil {
						t.Fatal(err)
					}
					return append(raw, '\n')
				}
				admitted, err := load(encode(profile))
				if err != nil {
					t.Fatal(err)
				}
				result, err := observeSourceCut(program, root, "", "unsafe-external-cut", nil, true, 2, sourceCanonicalPhysicalWindow(admitted))
				if err != nil || !result.InspectionOnly || result.Entry == 0 || result.PhysicalWindow != window || result.RootPID != 0 || result.RootExit != -1 || result.InstalledExecution || result.HeldCoordinatorKilled || result.ExternalCancellationAcknowledged || result.PhysicalPreimageObserved {
					t.Fatal("offline physical inspection executed command or claimed original cut", result, err)
				}
				mutations := map[string]func(*sourceCanonicalPhysicalProfile){
					"ack-purpose":         func(p *sourceCanonicalPhysicalProfile) { p.Schema = "source-original-cut-observer-v1" },
					"recovery-command":    func(p *sourceCanonicalPhysicalProfile) { p.Arguments[1] = "recover-source" },
					"wrong-frontier":      func(p *sourceCanonicalPhysicalProfile) { p.PrefixRecords++ },
					"wrong-phase":         func(p *sourceCanonicalPhysicalProfile) { p.LastPhase = "unexpected-phase" },
					"unknown-window":      func(p *sourceCanonicalPhysicalProfile) { p.Window = "seal-before-record" },
					"root-window":         func(p *sourceCanonicalPhysicalProfile) { p.Window = "root-" + p.Window },
					"root-process-window": func(p *sourceCanonicalPhysicalProfile) { p.Window = "root-process-" + p.Window },
					"missing-generation":  func(p *sourceCanonicalPhysicalProfile) { p.Generation = "" },
					"foreign-journal":     func(p *sourceCanonicalPhysicalProfile) { p.Journal += ".other" },
				}
				for name, change := range mutations {
					t.Run(name, func(t *testing.T) {
						changed := profile
						changed.Arguments = append([]string(nil), profile.Arguments...)
						change(&changed)
						if _, err := load(encode(changed)); err == nil {
							t.Fatal("out-of-purpose physical profile admitted")
						}
					})
				}
				raw := encode(profile)
				for _, malformed := range [][]byte{append(bytes.Clone(raw), []byte("{}\n")...), bytes.Replace(raw, []byte(`"schema":`), []byte(`"extra":true,"schema":`), 1), bytes.Replace(raw, []byte(`"schema":`), []byte(`"schema":"source-physical-cut-observer-v1","schema":`), 1)} {
					if _, err := load(malformed); err == nil {
						t.Fatal("ambiguous physical declaration admitted")
					}
				}
				admitted.ProgramSHA256 = zero
				if _, err := observeSourceCut(program, root, "", "unsafe-external-cut", nil, true, 2, sourceCanonicalPhysicalWindow(admitted)); err == nil {
					t.Fatal("different original program accepted")
				}
			})
		}
	}
	if _, err := os.Lstat(root); !os.IsNotExist(err) {
		t.Fatal("offline physical inspection touched Source root", err)
	}
}

func TestSourceCanonicalPhysicalPayloadNative(t *testing.T) {
	root := t.TempDir()
	generation := strings.Repeat("1", 64)
	directory := filepath.Join(root, "generations", generation)
	if err := os.MkdirAll(directory, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, ProgramName)
	body := bytes.Repeat([]byte("physical original program bytes\n"), 300)
	if err := os.WriteFile(path, nil, 0600); err != nil {
		t.Fatal(err)
	}
	plan := &sourcePhysicalCut{nextPhase: "file-born", canonical: &sourceCanonicalPhysicalProfile{sourceCutObserverProfile: sourceCutObserverProfile{ProgramSHA256: sourcePhysicalSHA(body)}, Generation: generation}}
	intent := sourceRecord{Path: path, Mode: 0555, SHA256: sourcePhysicalSHA(body)}
	if _, _, err := plan.canonicalPayload(intent, root); err != nil {
		t.Fatal("actual empty unrecorded birth", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	native := info.Sys().(*syscall.Stat_t)
	born := intent
	born.Device, born.Inode = uint64(native.Dev), native.Ino
	plan.nextPhase, plan.window = "", "file-born-before-journal-sync"
	if raw, _, err := plan.canonicalPayload(born, root); err != nil || len(raw) != 0 {
		t.Fatal("actual original empty born inode before journal Sync", err)
	}
	foreign := born
	foreign.Inode++
	if _, _, err := plan.canonicalPayload(foreign, root); err == nil {
		t.Fatal("foreign born inode before journal Sync admitted")
	}
	if err := os.WriteFile(path, body, 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := plan.canonicalPayload(born, root); err == nil {
		t.Fatal("written payload admitted as pre-Sync empty original birth")
	}
	plan.nextPhase, plan.window = "file-born", ""
	if _, _, err := plan.canonicalPayload(intent, root); err == nil {
		t.Fatal("bytes before unrecorded birth claimed empty")
	}
	if err := os.Chmod(path, 0555); err != nil {
		t.Fatal(err)
	}
	plan.nextPhase = "file-ready"
	if raw, _, err := plan.canonicalPayload(born, root); err != nil || !bytes.Equal(raw, body) {
		t.Fatal("actual same original inode body before ready", err)
	}
	for name, changed := range map[string]sourceRecord{"foreign-inode": func() sourceRecord { r := born; r.Inode++; return r }(), "wrong-access": func() sourceRecord { r := born; r.Mode = 0444; return r }(), "foreign-hash": func() sourceRecord { r := born; r.SHA256 = strings.Repeat("0", 64); return r }()} {
		if _, _, err := plan.canonicalPayload(changed, root); err == nil {
			t.Fatal("substituted native payload accepted", name)
		}
	}
}
