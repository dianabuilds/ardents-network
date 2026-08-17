package blockedentry

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFinalSupplyLockRejectsCallerSelectedImages(t *testing.T) {
	root := t.TempDir()
	toolLockPath := filepath.Join(root, "lab", "carrier", "tools.lock")
	if err := os.MkdirAll(filepath.Dir(toolLockPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(toolLockPath, []byte("locked tooling\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	toolLockHash, _, err := hashFile(toolLockPath)
	if err != nil {
		t.Fatal(err)
	}
	recipePath := filepath.Join(root, "tests", "live", "stage5-final", "go-builder.Dockerfile")
	if err := os.MkdirAll(filepath.Dir(recipePath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(recipePath, []byte("accepted recipe\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	recipeHash, _, err := hashFile(recipePath)
	if err != nil {
		t.Fatal(err)
	}
	lockPath := filepath.Join(root, filepath.FromSlash(finalSupplyLockSourcePath))
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o700); err != nil {
		t.Fatal(err)
	}
	value := finalSupplyLock{Schema: "ardents-h3-s5-supply-lock-v1",
		GoBuilderImageID: "sha256:" + strings.Repeat("a", 64), GoBuilderVersion: finalGoBuilderVersion,
		GoArchiveSHA256: finalGoArchiveHash, GoRecipeSHA256: recipeHash,
		GoModuleSHA256: strings.Repeat("d", 64),
		ToolImageID:    "sha256:" + strings.Repeat("b", 64),
		ToolLockSHA256: toolLockHash, CarrierLabSHA256: strings.Repeat("c", 64)}
	if err := writeJSON(lockPath, value); err != nil {
		t.Fatal(err)
	}
	observed, err := loadFinalSupplyLock(root)
	if err != nil || observed != value {
		t.Fatalf("supply lock=%+v err=%v", observed, err)
	}
	spec := finalSpec{GoBuilderImageID: value.GoBuilderImageID, GoBuilderVersion: value.GoBuilderVersion,
		ToolImageID: value.ToolImageID,
		ProductReceipt: finalProductReceipt{GoArchiveSHA256: value.GoArchiveSHA256,
			GoRecipeSHA256: value.GoRecipeSHA256, GoModuleSHA256: value.GoModuleSHA256},
		ToolReceipt: finalToolReceipt{ToolLockSHA256: value.ToolLockSHA256,
			CarrierSHA256: value.CarrierLabSHA256}}
	if !finalSupplyLockMatchesSpec(value, spec) {
		t.Fatal("exact Go supply receipt was rejected")
	}
	for name, mutate := range map[string]func(*finalSpec){
		"archive": func(value *finalSpec) { value.ProductReceipt.GoArchiveSHA256 = strings.Repeat("1", 64) },
		"recipe":  func(value *finalSpec) { value.ProductReceipt.GoRecipeSHA256 = strings.Repeat("2", 64) },
		"modules": func(value *finalSpec) { value.ProductReceipt.GoModuleSHA256 = strings.Repeat("3", 64) },
	} {
		t.Run(name, func(t *testing.T) {
			changed := spec
			mutate(&changed)
			if finalSupplyLockMatchesSpec(value, changed) {
				t.Fatal("modified Go supply receipt was accepted")
			}
		})
	}
	value.ToolImageID = "pending-qualifying-stand"
	if err := os.Remove(lockPath); err != nil {
		t.Fatal(err)
	}
	if err := writeJSON(lockPath, value); err != nil {
		t.Fatal(err)
	}
	if _, err := loadFinalSupplyLock(root); err == nil {
		t.Fatal("caller-selected pending tooling identity was accepted")
	}
	if err := os.WriteFile(lockPath, make([]byte, (4<<10)+1), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadFinalSupplyLock(root); err == nil {
		t.Fatal("oversized supply lock was read before rejection")
	}
}
