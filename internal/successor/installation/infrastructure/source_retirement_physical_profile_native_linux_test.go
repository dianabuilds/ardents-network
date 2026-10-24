//go:build linux && amd64 && network_signing_native

package infrastructure

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestSourceRetirementPhysicalProfileInspectionNative(t *testing.T) {
	program := os.Getenv("ARDENTS_OBSERVER_INSPECT_PROGRAM")
	raw, err := os.ReadFile(program)
	if err != nil || !sourcePath(program) {
		t.Fatal("actual canonical retirement ELF required", err)
	}
	root := filepath.Join(t.TempDir(), "uncreated-source")
	zero, generation := strings.Repeat("0", 64), strings.Repeat("1", 64)
	for number := sourceAttemptNumber(0); number < sourceAttemptLimit; number++ {
		for _, window := range []string{"native-before-joined", "result-before-close", "pending-before-unlink"} {
			t.Run(fmt.Sprintf("attempt-%02d/%s", number, window), func(t *testing.T) {
				count, last := sourceRetirementPhysicalCount(window), "retired"
				if count == 6 {
					last = "domain-joined"
				}
				profile := sourceRetirementPhysicalProfile{sourceCutObserverProfile: sourceCutObserverProfile{Schema: "source-retirement-physical-cut-observer-v1", ProgramSHA256: sourcePhysicalSHA(raw), PublicReceiptSHA256: zero,
					Arguments: []string{"installation", "stop-source", "/testtmp/history", filepath.Join(root, "binding.json")},
					Journal:   filepath.Join(number.directory(root), "source-retirement-journal"), PrefixRecords: count, LastPhase: last},
					Window: window, Attempt: uint32(number), Generation: generation, BindingSHA256: zero, StartSHA256: zero, StartJournalSHA256: zero, GID: 999}
				encode := func(p sourceRetirementPhysicalProfile) []byte {
					t.Helper()
					raw, err := json.Marshal(p)
					if err != nil {
						t.Fatal(err)
					}
					return append(raw, '\n')
				}
				load := func(raw []byte) (*sourceRetirementPhysicalProfile, error) {
					t.Helper()
					path := filepath.Join(t.TempDir(), "profile.json")
					if err := os.WriteFile(path, raw, 0600); err != nil {
						t.Fatal(err)
					}
					return readSourceRetirementPhysicalProfile(path, sourcePhysicalSHA(raw), program, root)
				}
				p, err := load(encode(profile))
				if err != nil {
					t.Fatal(err)
				}
				result, err := observeSourceCut(program, root, "", "unsafe-root-cut", nil, true, 2, sourceRetirementPhysicalWindow(p))
				if err != nil || !result.InspectionOnly || result.Entry == 0 || result.RootPID != 0 || result.InstalledExecution || result.ExternalCutReady || result.ExternalCutRequested || result.StartupArchive != nil || result.RetirementResult != nil {
					t.Fatal("offline retirement inspection executed or claimed actual facts", result, err)
				}
				p.ProgramSHA256 = zero
				if _, err := observeSourceCut(program, root, "", "unsafe-root-cut", nil, true, 2, sourceRetirementPhysicalWindow(p)); err == nil {
					t.Fatal("different actual retirement program accepted")
				}
				for name, change := range map[string]func(*sourceRetirementPhysicalProfile){
					"other-purpose":         func(p *sourceRetirementPhysicalProfile) { p.Schema = "source-startup-physical-cut-observer-v1" },
					"other-command":         func(p *sourceRetirementPhysicalProfile) { p.Arguments[1] = "recover-source" },
					"relative-history":      func(p *sourceRetirementPhysicalProfile) { p.Arguments[2] = "history" },
					"foreign-binding":       func(p *sourceRetirementPhysicalProfile) { p.Arguments[3] += ".foreign" },
					"foreign-journal":       func(p *sourceRetirementPhysicalProfile) { p.Journal += ".foreign" },
					"other-count":           func(p *sourceRetirementPhysicalProfile) { p.PrefixRecords++ },
					"other-phase":           func(p *sourceRetirementPhysicalProfile) { p.LastPhase = "native-closed" },
					"other-window":          func(p *sourceRetirementPhysicalProfile) { p.Window = "completion-before-close" },
					"exhausted-attempt":     func(p *sourceRetirementPhysicalProfile) { p.Attempt = sourceAttemptLimit },
					"foreign-attempt":       func(p *sourceRetirementPhysicalProfile) { p.Attempt = (p.Attempt + 1) % sourceAttemptLimit },
					"wrong-gid":             func(p *sourceRetirementPhysicalProfile) { p.GID = 0 },
					"missing-start":         func(p *sourceRetirementPhysicalProfile) { p.StartSHA256 = "" },
					"missing-start-journal": func(p *sourceRetirementPhysicalProfile) { p.StartJournalSHA256 = "" },
					"missing-generation":    func(p *sourceRetirementPhysicalProfile) { p.Generation = "" },
				} {
					changed := profile
					changed.Arguments = slices.Clone(profile.Arguments)
					change(&changed)
					if _, err := load(encode(changed)); err == nil {
						t.Fatal("foreign retirement declaration admitted", name)
					}
				}
				encoded := encode(profile)
				for _, raw := range [][]byte{append(bytes.Clone(encoded), []byte("{}\n")...), bytes.Replace(encoded, []byte(`"schema":`), []byte(`"extra":true,"schema":`), 1), bytes.Replace(encoded, []byte(`"schema":`), []byte(`"schema":"other","schema":`), 1)} {
					if _, err := load(raw); err == nil {
						t.Fatal("ambiguous/unknown retirement declaration admitted")
					}
				}
			})
		}
	}
	if _, err := os.Lstat(root); !os.IsNotExist(err) {
		t.Fatal("offline retirement inspection touched Source root", err)
	}
}
