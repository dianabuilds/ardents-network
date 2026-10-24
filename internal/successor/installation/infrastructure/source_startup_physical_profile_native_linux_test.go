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

func TestSourceStartupPhysicalProfileInspectionNative(t *testing.T) {
	program := os.Getenv("ARDENTS_OBSERVER_INSPECT_PROGRAM")
	raw, err := os.ReadFile(program)
	if err != nil || !sourcePath(program) {
		t.Fatal("actual canonical startup ELF required", err)
	}
	root := filepath.Join(t.TempDir(), "uncreated-source")
	zero, generation := strings.Repeat("0", 64), strings.Repeat("1", 64)
	for number := sourceAttemptNumber(0); number < sourceAttemptLimit; number++ {
		for _, window := range []string{"archive-before-archived", "ack-before-written"} {
			t.Run(fmt.Sprintf("attempt-%02d/%s", number, window), func(t *testing.T) {
				count, phase, command := 19, "file-ready", "start-source"
				if number != 0 {
					command = "restart-source"
				}
				if window == "ack-before-written" {
					count, phase = 21, "ack-attempt"
				}
				profile := sourceStartupPhysicalProfile{sourceCutObserverProfile: sourceCutObserverProfile{Schema: "source-startup-physical-cut-observer-v1",
					ProgramSHA256: sourcePhysicalSHA(raw), PublicReceiptSHA256: zero,
					Arguments: []string{"installation", command, filepath.Dir(program), "/testtmp/history", "alpha", "one-network", filepath.Join(root, "binding.json")},
					Journal:   number.journal(root), PrefixRecords: count, LastPhase: phase}, Window: window, Attempt: uint32(number), Generation: generation, BindingSHA256: zero, GID: 999}
				encode := func(p sourceStartupPhysicalProfile) []byte {
					t.Helper()
					raw, err := json.Marshal(p)
					if err != nil {
						t.Fatal(err)
					}
					return append(raw, '\n')
				}
				load := func(raw []byte) (*sourceStartupPhysicalProfile, error) {
					t.Helper()
					path := filepath.Join(t.TempDir(), "profile.json")
					if err := os.WriteFile(path, raw, 0600); err != nil {
						t.Fatal(err)
					}
					return readSourceStartupPhysicalProfile(path, sourcePhysicalSHA(raw), program, root)
				}
				p, err := load(encode(profile))
				if err != nil {
					t.Fatal(err)
				}
				result, err := observeSourceCut(program, root, "", "unsafe-root-cut", nil, true, 2, sourceStartupPhysicalWindow(p))
				if err != nil || !result.InspectionOnly || result.Entry == 0 || result.RootPID != 0 || result.InstalledExecution || result.ExternalCutReady || result.ExternalCutRequested || result.StartupArchive != nil {
					t.Fatal("offline startup inspection executed or claimed readiness", result, err)
				}
				for name, change := range map[string]func(*sourceStartupPhysicalProfile){
					"other-purpose":     func(p *sourceStartupPhysicalProfile) { p.Schema = "source-recovery-physical-cut-observer-v1" },
					"other-command":     func(p *sourceStartupPhysicalProfile) { p.Arguments[1] = "recover-source" },
					"other-count":       func(p *sourceStartupPhysicalProfile) { p.PrefixRecords++ },
					"other-phase":       func(p *sourceStartupPhysicalProfile) { p.LastPhase = "ack-written" },
					"other-window":      func(p *sourceStartupPhysicalProfile) { p.Window = "pending-before-unlink" },
					"other-binding":     func(p *sourceStartupPhysicalProfile) { p.Arguments[6] += ".other" },
					"other-journal":     func(p *sourceStartupPhysicalProfile) { p.Journal += ".other" },
					"exhausted-attempt": func(p *sourceStartupPhysicalProfile) { p.Attempt = uint32(sourceAttemptLimit) },
					"wrong-gid":         func(p *sourceStartupPhysicalProfile) { p.GID = 0 },
					"no-generation":     func(p *sourceStartupPhysicalProfile) { p.Generation = "" },
					"no-binding":        func(p *sourceStartupPhysicalProfile) { p.BindingSHA256 = "" },
				} {
					t.Run(name, func(t *testing.T) {
						changed := profile
						changed.Arguments = slices.Clone(profile.Arguments)
						change(&changed)
						if _, err := load(encode(changed)); err == nil {
							t.Fatal("foreign startup profile admitted")
						}
					})
				}
				encoded := encode(profile)
				for _, raw := range [][]byte{append(bytes.Clone(encoded), []byte("{}\n")...), bytes.Replace(encoded, []byte(`"schema":`), []byte(`"extra":true,"schema":`), 1), bytes.Replace(encoded, []byte(`"schema":`), []byte(`"schema":"other","schema":`), 1)} {
					if _, err := load(raw); err == nil {
						t.Fatal("unknown or ambiguous startup profile admitted")
					}
				}
			})
		}
	}
	if _, err := os.Lstat(root); !os.IsNotExist(err) {
		t.Fatal("offline startup inspection touched installation", err)
	}
}
