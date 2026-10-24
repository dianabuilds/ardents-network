//go:build linux && amd64 && network_signing_native

package infrastructure

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/successor/installation/fixedfile"
)

// Native file mechanisms only, with deliberately unsigned fixture bodies.
// Unit controls use a private ext4 leaf, never the host's systemd unit.
func TestSourceCanonicalPhysicalPublicationNative(t *testing.T) {
	for _, operation := range []string{"initial", "replacement"} {
		for _, window := range []string{"binding-before-record", "unit-before-record"} {
			t.Run(operation+"/"+window, func(t *testing.T) {
				plan, root, births, _ := prepareSourcePhysicalSealedNative(t)
				plan.window = window
				plan.canonical.Arguments = []string{"installation", "install-initial-stopped"}
				if operation == "replacement" {
					plan.canonical.Arguments[1] = "replace-source"
				}
				directory := filepath.Join(root, "generations", plan.canonical.Generation)
				if err := births.record("sealed", directory, births.directories[directory], plan.canonical.Generation, 0555, 0, 0); err != nil {
					t.Fatal(err)
				}
				target, name, predecessor, intent := filepath.Join(root, "binding.json"), "binding.json", "predecessor-binding", "selection-intent"
				if window == "unit-before-record" {
					target, name, predecessor, intent = filepath.Join(root, "fixed-unit"), "rendered-source.service", "predecessor-unit", "unit-intent"
				}
				body, err := os.ReadFile(filepath.Join(directory, name))
				if err != nil {
					t.Fatal(err)
				}
				ctx := context.Background()
				readRecords := func() []sourceRecord {
					t.Helper()
					raw, err := os.ReadFile(births.journal.path)
					if err != nil {
						t.Fatal(err)
					}
					r, err := decodeSourceRecoveryPrefix(raw, sourceRecoverableInitialRecords)
					if err != nil {
						t.Fatal(err)
					}
					return r
				}
				if operation == "initial" {
					births.allowed[target] = true
					if err := births.write(target, body, 0644, 0, 0, ctx.Err); err != nil {
						t.Fatal(err)
					}
				} else {
					previous := []byte("original native fixed preimage\n")
					if err := os.WriteFile(target, previous, 0644); err != nil {
						t.Fatal(err)
					}
					original, err := os.Stat(target)
					if err != nil {
						t.Fatal(err)
					}
					if err := births.record(predecessor, target, original, sourcePhysicalSHA(previous), 0644, 0, 0); err != nil {
						t.Fatal(err)
					}
					before := readRecords()
					pin, err := sourcePhysicalPrefixSHA(before)
					if err != nil {
						t.Fatal(err)
					}
					if err := plan.capturePublicationPreimage(before, target, pin); err != nil {
						t.Fatal(err)
					}
					parent, err := os.Stat(filepath.Dir(target))
					if err != nil {
						t.Fatal(err)
					}
					mutation, err := fixedfile.Replace(ctx, target, parent, original, previous, body, 0644, 0)
					if mutation != nil {
						t.Cleanup(func() {
							if err := mutation.Close(); err != nil {
								t.Error(err)
							}
						})
					}
					if err != nil {
						t.Fatal(err)
					}
					if err := births.record(intent, target, mutation.Identity(), sourcePhysicalSHA(body), 0644, 0, 0); err != nil {
						t.Fatal(err)
					}
					if _, err := mutation.Commit(); err != nil {
						t.Fatal(err)
					}
					plan.lastPhase = intent
				}
				records := readRecords()
				before, info, err := plan.canonicalPublication(records, root, target)
				if err != nil {
					t.Fatal("actual native published candidate/original inode", err)
				}
				again, same, err := plan.canonicalPublication(records, root, target)
				if err != nil || !bytes.Equal(before, again) || !sameSourceFile(info, same) {
					t.Fatal("stable original publication changed", err)
				}
				if operation == "replacement" {
					saved := plan.publicationBefore
					plan.publicationBefore = nil
					if _, _, err := plan.canonicalPublication(records, root, target); err == nil {
						t.Fatal("new bytes without actual original preimage observation admitted")
					}
					if err := plan.capturePublicationPreimage(records, target, saved.prefixSHA); err == nil {
						t.Fatal("post-Commit new bytes supplied old preimage")
					}
					plan.publicationBefore = saved
				}
				for _, change := range []func([]sourceRecord){
					func(r []sourceRecord) { r[len(r)-1].Inode++ },
					func(r []sourceRecord) { r[len(r)-1].Mode = 0600 },
					func(r []sourceRecord) { r[len(r)-1].SHA256 = sourcePhysicalSHA([]byte("foreign")) },
				} {
					altered := slices.Clone(records)
					change(altered)
					if _, _, err := plan.canonicalPublication(altered, root, target); err == nil {
						t.Fatal("foreign publication record admitted")
					}
				}
				if err := os.WriteFile(target, []byte("changed actual bytes\n"), 0644); err != nil {
					t.Fatal(err)
				}
				if _, _, err := plan.canonicalPublication(records, root, target); err == nil {
					t.Fatal("same original inode with wrong publication bytes admitted")
				}
			})
		}
	}
}

// Completed fixed images use real original births or fixedfile Commit on private
// native leaves. No fixture manager result, signed Source authority or observed
// original pre-Commit claim is supplied by this control.
func TestSourceCanonicalPhysicalCompletedImageNative(t *testing.T) {
	for _, operation := range []string{"initial", "replacement"} {
		for _, window := range []string{"reload-before-verified", "journal-complete-before-close", "pending-before-unlink"} {
			t.Run(operation+"/"+window, func(t *testing.T) {
				plan, root, births, _ := prepareSourcePhysicalSealedNative(t)
				plan.window = window
				plan.canonical.Arguments = []string{"installation", "install-initial-stopped"}
				if operation == "replacement" {
					plan.canonical.Arguments[1] = "replace-source"
				}
				directory := filepath.Join(root, "generations", plan.canonical.Generation)
				if err := births.record("sealed", directory, births.directories[directory], plan.canonical.Generation, 0555, 0, 0); err != nil {
					t.Fatal(err)
				}
				binding, unit := filepath.Join(root, "binding.json"), filepath.Join(root, "fixed-unit")
				ctx := context.Background()
				var bindingBody, unitBody []byte
				for _, fixed := range []struct{ target, name, predecessor, intent, written string }{
					{binding, "binding.json", "predecessor-binding", "selection-intent", "selection-written"},
					{unit, "rendered-source.service", "predecessor-unit", "unit-intent", "unit-written"},
				} {
					body, err := os.ReadFile(filepath.Join(directory, fixed.name))
					if err != nil {
						t.Fatal(err)
					}
					if fixed.target == binding {
						bindingBody = body
					} else {
						unitBody = body
					}
					if operation == "initial" {
						births.allowed[fixed.target] = true
						if err := births.write(fixed.target, body, 0644, 0, 0, ctx.Err); err != nil {
							t.Fatal(err)
						}
					} else {
						previous := []byte("original completed-image preimage\n")
						if err := os.WriteFile(fixed.target, previous, 0644); err != nil {
							t.Fatal(err)
						}
						original, err := os.Stat(fixed.target)
						if err != nil {
							t.Fatal(err)
						}
						if err := births.record(fixed.predecessor, fixed.target, original, sourcePhysicalSHA(previous), 0644, 0, 0); err != nil {
							t.Fatal(err)
						}
						parent, err := os.Stat(root)
						if err != nil {
							t.Fatal(err)
						}
						mutation, err := fixedfile.Replace(ctx, fixed.target, parent, original, previous, body, 0644, 0)
						if mutation != nil {
							t.Cleanup(func() {
								if err := mutation.Close(); err != nil {
									t.Error(err)
								}
							})
						}
						if err != nil {
							t.Fatal(err)
						}
						if err := births.record(fixed.intent, fixed.target, mutation.Identity(), sourcePhysicalSHA(body), 0644, 0, 0); err != nil {
							t.Fatal(err)
						}
						info, err := mutation.Commit()
						if err != nil {
							t.Fatal(err)
						}
						if err := births.record(fixed.written, fixed.target, info, sourcePhysicalSHA(body), 0644, 0, 0); err != nil {
							t.Fatal(err)
						}
					}
				}
				if err := births.record("reload-intent", unit, nil, sourcePhysicalSHA(unitBody), 0644, 0, 0); err != nil {
					t.Fatal(err)
				}
				if window != "reload-before-verified" {
					phase := "replacement-verified"
					if operation == "initial" {
						phase = "stopped-verified"
					}
					if err := births.record(phase, binding, nil, sourcePhysicalSHA(bindingBody), 0644, 0, 0); err != nil {
						t.Fatal(err)
					}
				}
				raw, err := os.ReadFile(births.journal.path)
				if err != nil {
					t.Fatal(err)
				}
				records, err := decodeSourceRecoveryPrefix(raw, sourceRecoverableInitialRecords)
				if err != nil {
					t.Fatal(err)
				}
				before, info, err := plan.canonicalCompletedImage(records, root, unit)
				if err != nil {
					t.Fatal("actual completed original native image", err)
				}
				again, same, err := plan.canonicalCompletedImage(records, root, unit)
				if err != nil || !bytes.Equal(before, again) || !sameSourceFile(info, same) {
					t.Fatal("unchanged completed image differs", err)
				}
				for _, change := range []func([]sourceRecord){
					func(r []sourceRecord) { r[len(r)-1].Device++ },
					func(r []sourceRecord) { r[len(r)-1].SHA256 = sourcePhysicalSHA([]byte("foreign")) },
					func(r []sourceRecord) { r[len(r)-2].Inode++ },
				} {
					altered := slices.Clone(records)
					change(altered)
					if _, _, err := plan.canonicalCompletedImage(altered, root, unit); err == nil {
						t.Fatal("substituted completed record admitted")
					}
				}
				if err := os.WriteFile(unit, []byte("foreign same-inode bytes\n"), 0644); err != nil {
					t.Fatal(err)
				}
				if _, _, err := plan.canonicalCompletedImage(records, root, unit); err == nil {
					t.Fatal("foreign completed unit admitted")
				}
			})
		}
	}
}
