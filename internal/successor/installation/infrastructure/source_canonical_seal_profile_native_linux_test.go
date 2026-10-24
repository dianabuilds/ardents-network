//go:build linux && amd64 && network_signing_native

package infrastructure

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// Actual native births/sealing and hostile file observations only. The twelve
// fixture bodies are deliberately not signed Source/Release/State authority.
func prepareSourcePhysicalSealedNative(t *testing.T) (*sourcePhysicalCut, string, *sourceBirths, []sourceRecord) {
	t.Helper()
	root := t.TempDir()
	generation := strings.Repeat("1", 64)
	parent := filepath.Join(root, "generations")
	if err := os.Mkdir(parent, 0700); err != nil {
		t.Fatal(err)
	}
	directory := filepath.Join(parent, generation)
	names := sourceGenerationNames()
	slices.Sort(names)
	allowed := map[string]bool{directory: true}
	for _, name := range names {
		allowed[filepath.Join(directory, name)] = true
	}
	journal := filepath.Join(root, "original-journal")
	ctx := context.Background()
	births, err := openSourceBirths(ctx, journal, allowed)
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
	for _, name := range names {
		mode := os.FileMode(0444)
		if name == ProgramName {
			mode = 0555
		}
		if err := births.write(filepath.Join(directory, name), []byte("native fixture only: "+name+"\n"), mode, 0, 0, ctx.Err); err != nil {
			t.Fatal(err)
		}
	}
	if err := births.record("seal-intent", directory, births.directories[directory], generation, 0555, 0, 0); err != nil {
		t.Fatal(err)
	}
	if err := sealSourceDirectory(births, directory, ctx.Err); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(journal)
	if err != nil {
		t.Fatal(err)
	}
	records, err := decodeSourceRecoveryPrefix(raw, sourceRecoverableInitialRecords)
	if err != nil {
		t.Fatal(err)
	}
	plan := &sourcePhysicalCut{window: "seal-before-sealed", canonical: &sourceCanonicalPhysicalProfile{
		sourceCutObserverProfile: sourceCutObserverProfile{ProgramSHA256: sourcePhysicalSHA([]byte("native fixture only: " + ProgramName + "\n"))}, Generation: generation}}
	return plan, root, births, records
}

func TestSourceCanonicalPhysicalSealNative(t *testing.T) {
	plan, root, _, records := prepareSourcePhysicalSealedNative(t)
	directory := filepath.Join(root, "generations", plan.canonical.Generation)
	names := sourceGenerationNames()
	slices.Sort(names)
	before, info, err := plan.canonicalImage(records, root)
	if err != nil {
		t.Fatal("actual sealed original native inventory", err)
	}
	after, again, err := plan.canonicalImage(records, root)
	if err != nil || !bytes.Equal(before, after) || !sameSourceFile(info, again) {
		t.Fatal("unchanged native inventory not stable", err)
	}
	for _, change := range []func([]sourceRecord){
		func(r []sourceRecord) { r[len(r)-1].Inode++ },
		func(r []sourceRecord) { r[len(r)-1].SHA256 = strings.Repeat("2", 64) },
		func(r []sourceRecord) { r[len(r)-1].Phase = "sealed" },
		func(r []sourceRecord) { r[3].Inode++ },
		func(r []sourceRecord) { r[5].Inode++ },
		func(r []sourceRecord) { r[6].Mode = 0600 },
	} {
		altered := slices.Clone(records)
		change(altered)
		if _, _, err := plan.canonicalImage(altered, root); err == nil {
			t.Fatal("substituted seal/birth lineage admitted")
		}
	}
	if err := os.Chmod(directory, 0755); err != nil {
		t.Fatal(err)
	}
	if _, _, err := plan.canonicalImage(records, root); err == nil {
		t.Fatal("unsealed directory claimed sealed")
	}
	extra := filepath.Join(directory, "unknown-extra")
	if err := os.WriteFile(extra, []byte("unknown"), 0444); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(directory, 0555); err != nil {
		t.Fatal(err)
	}
	if _, _, err := plan.canonicalImage(records, root); err == nil {
		t.Fatal("unknown extra generation file admitted")
	}
	if err := os.Remove(extra); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, names[len(names)-1])
	original := filepath.Join(root, "retained-original")
	if err := os.Rename(path, original); err != nil {
		t.Fatal(err)
	}
	if _, _, err := plan.canonicalImage(records, root); err == nil {
		t.Fatal("missing generation file admitted")
	}
	body, err := os.ReadFile(original)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, body, 0444); err != nil {
		t.Fatal(err)
	}
	if _, _, err := plan.canonicalImage(records, root); err == nil {
		t.Fatal("same-byte copied generation inode admitted")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(original, path); err != nil {
		t.Fatal(err)
	}
	stable, _, err := plan.canonicalImage(records, root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("substituted bytes\n"), 0444); err != nil {
		t.Fatal(err)
	}
	if _, _, err := plan.canonicalImage(records, root); err == nil {
		t.Fatal("same-inode changed generation body admitted")
	}
	if err := os.WriteFile(path, body, 0444); err != nil {
		t.Fatal(err)
	}
	fileInfo, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	changedTime := fileInfo.ModTime().Add(time.Second)
	if err := os.Chtimes(path, changedTime, changedTime); err != nil {
		t.Fatal(err)
	}
	changed, _, err := plan.canonicalImage(records, root)
	if err != nil || bytes.Equal(stable, changed) {
		t.Fatal("restored same-inode bytes concealed native file metadata change", err)
	}
}
