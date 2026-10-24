//go:build linux && amd64 && network_signing_native

package infrastructure

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"
)

func TestSourceRecoveryPhysicalProfileInspectionNative(t *testing.T) {
	program := os.Getenv("ARDENTS_OBSERVER_INSPECT_PROGRAM")
	raw, err := os.ReadFile(program)
	if err != nil || !sourcePath(program) {
		t.Fatal("actual canonical ELF required", err)
	}
	root := filepath.Join(t.TempDir(), "uncreated-source")
	zero, generation := strings.Repeat("0", 64), strings.Repeat("1", 64)
	for _, operation := range []string{"initial", "replacement"} {
		for _, full := range []bool{false, true} {
			for _, window := range []string{"repaired-body-before-record", "completion-before-close", "pending-before-unlink"} {
				if full && window == "repaired-body-before-record" {
					continue
				}
				name := operation + "/writing/" + window
				if full {
					name = operation + "/complete/" + window
				}
				t.Run(name, func(t *testing.T) {
					count, phase, journal := 13, "file-born", root+".source-replacement-"+generation+"-journal"
					if operation == "initial" {
						count, journal = 70, root+".source-initial-journal"
					}
					if full {
						count, phase = 55, "replacement-verified"
						if operation == "initial" {
							count, phase = 115, "stopped-verified"
						}
					}
					profile := sourceRecoveryPhysicalProfile{sourceCutObserverProfile: sourceCutObserverProfile{
						Schema: "source-recovery-physical-cut-observer-v1", ProgramSHA256: sourcePhysicalSHA(raw), PublicReceiptSHA256: zero,
						Arguments: []string{"installation", "recover-source", filepath.Dir(program), "/testtmp/history", "alpha", "one-network", "/testtmp/request", journal}, Journal: journal, PrefixRecords: count, LastPhase: phase},
						Window: window, Generation: generation, BindingSHA256: zero, JournalSHA256: zero, OriginalCutSHA256: zero}
					encode := func(p sourceRecoveryPhysicalProfile) []byte {
						t.Helper()
						raw, err := json.Marshal(p)
						if err != nil {
							t.Fatal(err)
						}
						return append(raw, '\n')
					}
					load := func(raw []byte) (*sourceRecoveryPhysicalProfile, error) {
						t.Helper()
						path := filepath.Join(t.TempDir(), "profile.json")
						if err := os.WriteFile(path, raw, 0600); err != nil {
							t.Fatal(err)
						}
						return readSourceRecoveryPhysicalProfile(path, sourcePhysicalSHA(raw), program, root)
					}
					admitted, err := load(encode(profile))
					if err != nil {
						t.Fatal(err)
					}
					result, err := observeSourceCut(program, root, "", "unsafe-root-cut", nil, true, 2, sourceRecoveryPhysicalWindow(admitted))
					if err != nil || !result.InspectionOnly || result.Entry == 0 || result.RootPID != 0 || result.InstalledExecution || result.RootKillRequested || result.RootKilledByObserver || result.RootCloseEntry != 0 {
						t.Fatal("offline profile executed or claimed root cut", result, err)
					}
					for name, change := range map[string]func(*sourceRecoveryPhysicalProfile){
						"other-purpose":    func(p *sourceRecoveryPhysicalProfile) { p.Schema = "source-physical-cut-observer-v1" },
						"other-command":    func(p *sourceRecoveryPhysicalProfile) { p.Arguments[1] = "replace-source" },
						"other-count":      func(p *sourceRecoveryPhysicalProfile) { p.PrefixRecords++ },
						"other-phase":      func(p *sourceRecoveryPhysicalProfile) { p.LastPhase = "unknown" },
						"other-window":     func(p *sourceRecoveryPhysicalProfile) { p.Window = "root-process-file-complete-before-close" },
						"no-generation":    func(p *sourceRecoveryPhysicalProfile) { p.Generation = "" },
						"no-binding":       func(p *sourceRecoveryPhysicalProfile) { p.BindingSHA256 = "" },
						"no-original-pin":  func(p *sourceRecoveryPhysicalProfile) { p.JournalSHA256 = "" },
						"no-cut-pin":       func(p *sourceRecoveryPhysicalProfile) { p.OriginalCutSHA256 = "" },
						"foreign-original": func(p *sourceRecoveryPhysicalProfile) { p.Arguments[7] += ".other" },
					} {
						t.Run(name, func(t *testing.T) {
							changed := profile
							changed.Arguments = slices.Clone(profile.Arguments)
							change(&changed)
							if _, err := load(encode(changed)); err == nil {
								t.Fatal("foreign root recovery declaration admitted")
							}
						})
					}
					encoded := encode(profile)
					for _, bad := range [][]byte{append(bytes.Clone(encoded), []byte("{}\n")...), bytes.Replace(encoded, []byte(`"schema":`), []byte(`"extra":true,"schema":`), 1), bytes.Replace(encoded, []byte(`"schema":`), []byte(`"schema":"other","schema":`), 1)} {
						if _, err := load(bad); err == nil {
							t.Fatal("ambiguous root recovery profile admitted")
						}
					}
					admitted.ProgramSHA256 = zero
					if _, err := observeSourceCut(program, root, "", "unsafe-root-cut", nil, true, 2, sourceRecoveryPhysicalWindow(admitted)); err == nil {
						t.Fatal("foreign program admitted")
					}
				})
			}
		}
	}
	if _, err := os.Lstat(root); !os.IsNotExist(err) {
		t.Fatal("offline inspection touched Source root", err)
	}
}

// Real native births, immutable prefix, separate locked completion and hostile
// readback only. Repeated framing records and unsigned bodies are deliberately
// NOT Source/account/Release/manager authority; no recovery command is invoked.
func TestSourceRecoveryPhysicalImageNative(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	generation := strings.Repeat("1", 64)
	journal := root + ".source-initial-journal"
	parent := filepath.Join(root, "generations")
	directory := filepath.Join(parent, generation)
	if err := os.Mkdir(parent, 0700); err != nil {
		t.Fatal(err)
	}
	binding, unit, pending := filepath.Join(root, "binding.json"), filepath.Join(root, "native-unit"), sourceInitialPendingPath(root)
	allowed := map[string]bool{directory: true, binding: true, unit: true, pending: true}
	for _, name := range sourceGenerationNames() {
		allowed[filepath.Join(directory, name)] = true
	}
	births, err := openSourceRecoverableInitialBirths(ctx, journal, allowed)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := births.close(); err != nil {
			t.Error(err)
		}
	})
	if err := births.directory(directory, 0755, 0, 0, ctx.Err); err != nil {
		t.Fatal(err)
	}
	for _, name := range sourceGenerationNames() {
		mode := os.FileMode(0444)
		if name == ProgramName {
			mode = 0555
		}
		if err := births.write(filepath.Join(directory, name), []byte("unsigned native only: "+name+"\n"), mode, 0, 0, ctx.Err); err != nil {
			t.Fatal(err)
		}
	}
	for _, phase := range []string{"seal-intent", "sealed"} {
		if phase == "sealed" {
			if err := sealSourceDirectory(births, directory, ctx.Err); err != nil {
				t.Fatal(err)
			}
		}
		if err := births.record(phase, directory, births.directories[directory], generation, 0555, 0, 0); err != nil {
			t.Fatal(err)
		}
	}
	bindingBody := []byte("unsigned native only: binding.json\n")
	unitBody := []byte("unsigned native only: rendered-source.service\n")
	bindingPin := sourcePhysicalSHA(bindingBody)
	for _, file := range []struct {
		path string
		body []byte
		mode os.FileMode
	}{{binding, bindingBody, 0644}, {unit, unitBody, 0644}, {pending, sourceInitialPending(bindingPin), 0600}} {
		if err := births.write(file.path, file.body, file.mode, 0, 0, ctx.Err); err != nil {
			t.Fatal(err)
		}
	}
	for births.sequence < 113 {
		if err := births.record("admitted", root, nil, strings.Repeat("2", 64), 0, 999, 999); err != nil {
			t.Fatal(err)
		}
	}
	if err := births.record("reload-intent", unit, nil, sourcePhysicalSHA(unitBody), 0644, 0, 0); err != nil {
		t.Fatal(err)
	}
	if err := births.record("stopped-verified", binding, nil, bindingPin, 0644, 0, 0); err != nil {
		t.Fatal(err)
	}
	if err := births.close(); err != nil {
		t.Fatal(err)
	}
	prefix, err := openSourceRecoveryPrefix(ctx, journal, "initial")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := prefix.close(); err != nil {
			t.Error(err)
		}
	})
	writer, err := openSourceRecoveryBirths(prefix, generation, bindingPin, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := writer.close(); err != nil {
			t.Error(err)
		}
	})
	if err := writer.appendJournal([]byte(`],"transcript_sha256":"` + sourcePhysicalSHA(prefix.journal.raw) + "\"}\n")); err != nil {
		t.Fatal(err)
	}
	for _, suffix := range []string{".original-cut-journal", ".original-cut-result"} {
		if err := os.WriteFile(journal+suffix, []byte("native framing fixture only\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	p := &sourceRecoveryPhysicalProfile{sourceCutObserverProfile: sourceCutObserverProfile{Journal: journal, ProgramSHA256: sourcePhysicalSHA([]byte("unsigned native only: " + ProgramName + "\n")), PrefixRecords: 115, LastPhase: "stopped-verified"}, Generation: generation, BindingSHA256: bindingPin, JournalSHA256: sourcePhysicalSHA(prefix.journal.raw), OriginalCutSHA256: sourcePhysicalSHA([]byte("native framing fixture only\n")), root: root}
	before, err := p.image(unit)
	if err != nil {
		t.Fatal("actual native complete envelope/image", err)
	}
	again, err := p.image(unit)
	if err != nil || before.payloadSHA != again.payloadSHA {
		t.Fatal("stable native image changed", err)
	}
	for _, change := range []func(*sourceRecoveryPhysicalProfile){func(p *sourceRecoveryPhysicalProfile) { p.JournalSHA256 = strings.Repeat("0", 64) }, func(p *sourceRecoveryPhysicalProfile) { p.BindingSHA256 = strings.Repeat("0", 64) }, func(p *sourceRecoveryPhysicalProfile) { p.OriginalCutSHA256 = strings.Repeat("0", 64) }} {
		foreign := *p
		change(&foreign)
		if _, err := foreign.image(unit); err == nil {
			t.Fatal("foreign original/envelope pin admitted")
		}
	}
	if err := os.WriteFile(journal+".original-cut-pending", []byte("blocked\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := p.image(unit); err == nil {
		t.Fatal("guarded original cut admitted")
	}
	if err := os.Remove(journal + ".original-cut-pending"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(unit, []byte("foreign same-inode unit\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := p.image(unit); err == nil {
		t.Fatal("foreign fixed-file bytes admitted")
	}
}

// Actual native incomplete envelopes and original repaired inodes, without
// signed Source/Release/account/manager authority or a recovery invocation.
func TestSourceRecoveryRepairedImageNative(t *testing.T) {
	for _, operation := range []string{"initial", "replacement"} {
		t.Run(operation, func(t *testing.T) {
			ctx := context.Background()
			root := t.TempDir()
			generation, binding := strings.Repeat("1", 64), strings.Repeat("2", 64)
			count, phase := 13, "initial"
			journal := root + ".source-replacement-" + generation + "-journal"
			pending := filepath.Join(root, "source-replacement-pending")
			pendingBody := sourceReplacementPending(binding)
			if operation == "initial" {
				count, phase, journal = 70, sourceRecoverableInitialPhase, root+".source-initial-journal"
				pending, pendingBody = sourceInitialPendingPath(root), sourceInitialPending(binding)
			}
			directory := filepath.Join(root, "generations", generation)
			if err := os.MkdirAll(directory, 0700); err != nil {
				t.Fatal(err)
			}
			payload := filepath.Join(directory, ProgramName)
			body := []byte("unsigned native repaired inode only\n")
			births, err := openSourceBirthsWithPhase(ctx, journal, map[string]bool{pending: true, payload: true}, phase)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := births.close(); err != nil {
					t.Error(err)
				}
			})
			if err := births.write(pending, pendingBody, 0600, 0, 0, ctx.Err); err != nil {
				t.Fatal(err)
			}
			for births.sequence < count-2 {
				if err := births.record("admitted", root, nil, strings.Repeat("3", 64), 0, 999, 999); err != nil {
					t.Fatal(err)
				}
			}
			if err := births.record("file-intent", payload, nil, sourcePhysicalSHA(body), 0555, 0, 0); err != nil {
				t.Fatal(err)
			}
			file, err := os.OpenFile(payload, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0600)
			if err != nil {
				t.Fatal(err)
			}
			info, err := file.Stat()
			if err != nil {
				t.Fatal(err)
			}
			if err := births.record("file-born", payload, info, sourcePhysicalSHA(body), 0555, 0, 0); err != nil {
				t.Fatal(err)
			}
			if err := file.Close(); err != nil {
				t.Fatal(err)
			}
			if err := births.close(); err != nil {
				t.Fatal(err)
			}
			prefix, err := openSourceRecoveryPrefix(ctx, journal, operation)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := prefix.close(); err != nil {
					t.Error(err)
				}
			})
			writer, err := openSourceRecoveryBirths(prefix, generation, binding, nil)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := writer.close(); err != nil {
					t.Error(err)
				}
			})
			if err := os.WriteFile(payload, body, 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(payload, 0555); err != nil {
				t.Fatal(err)
			}
			for _, suffix := range []string{".original-cut-journal", ".original-cut-result"} {
				if err := os.WriteFile(journal+suffix, []byte("unsigned framing only\n"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			p := &sourceRecoveryPhysicalProfile{sourceCutObserverProfile: sourceCutObserverProfile{Journal: journal, PrefixRecords: count, LastPhase: "file-born", ProgramSHA256: sourcePhysicalSHA(body)}, Window: "repaired-body-before-record", Generation: generation, BindingSHA256: binding, JournalSHA256: sourcePhysicalSHA(prefix.journal.raw), OriginalCutSHA256: sourcePhysicalSHA([]byte("unsigned framing only\n")), root: root}
			before, err := p.image("")
			if err != nil {
				t.Fatal("actual repaired incomplete image", err)
			}
			again, err := p.image("")
			if err != nil || before.payloadSHA != again.payloadSHA {
				t.Fatal("unstable repaired image", err)
			}
			if err := writer.close(); err != nil {
				t.Fatal(err)
			}
			after, err := p.image("")
			if err != nil || after.payloadSHA != before.payloadSHA {
				t.Fatal("after-close image changed", err)
			}
			native := info.Sys().(*syscall.Stat_t)
			actual, err := os.Stat(payload)
			if err != nil || actual.Sys().(*syscall.Stat_t).Ino != native.Ino {
				t.Fatal("original inode lost", err)
			}
			foreign := *p
			foreign.ProgramSHA256 = strings.Repeat("0", 64)
			if _, err := foreign.image(""); err == nil {
				t.Fatal("foreign repaired bytes admitted")
			}
			if err := os.WriteFile(journal+".original-cut-pending", []byte("blocked\n"), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := p.image(""); err == nil {
				t.Fatal("guarded cut admitted")
			}
			if err := os.Remove(journal + ".original-cut-pending"); err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(payload, 0755); err != nil {
				t.Fatal(err)
			}
			if _, err := p.image(""); err == nil {
				t.Fatal("changed recorded access admitted")
			}
			if err := os.Chmod(payload, 0555); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(journal+".recovery-completion", []byte("foreign or appended completion\n"), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := p.image(""); err == nil {
				t.Fatal("substituted incomplete envelope admitted")
			}
		})
	}
}
