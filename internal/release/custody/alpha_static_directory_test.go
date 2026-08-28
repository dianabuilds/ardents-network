package custody

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestReadAlphaStaticDirectoryAcceptsOneCompleteGeneration(t *testing.T) {
	root := t.TempDir()
	for _, name := range alphaStaticFileNames(2) {
		if err := os.WriteFile(filepath.Join(root, name), []byte(name), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	got, err := readAlphaStaticDirectory(root)
	if err != nil {
		t.Fatalf("read static directory: %v", err)
	}
	if got.Generation != 2 || len(got.Files) != len(alphaInputFileNames) {
		t.Fatalf("static directory = generation %d with %d files, want generation 2 with %d files", got.Generation, len(got.Files), len(alphaInputFileNames))
	}
}

func TestReadAlphaStaticDirectoryRejectsMismatchedMetadataPair(t *testing.T) {
	root := t.TempDir()
	for _, name := range alphaStaticFileNames(2) {
		if name == "2.targets.json" {
			name = "3.targets.json"
		}
		if err := os.WriteFile(filepath.Join(root, name), []byte(name), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	_, err := readAlphaStaticDirectory(root)
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("read mismatched static directory error = %v, want ErrInvalid", err)
	}
}
