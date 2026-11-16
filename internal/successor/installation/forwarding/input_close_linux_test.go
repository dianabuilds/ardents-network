package forwarding

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestDeliveryCloseRetainsLateFileReplacement(t *testing.T) {
	for _, replace := range []bool{false, true} {
		t.Run(map[bool]string{false: "unchanged", true: "replaced"}[replace], func(t *testing.T) {
			parentPath := t.TempDir()
			path := filepath.Join(parentPath, "input")
			if err := os.WriteFile(path, []byte("original"), 0600); err != nil {
				t.Fatal(err)
			}
			dir, err := os.Open(parentPath)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = dir.Close() })
			dirInfo, err := dir.Stat()
			if err != nil {
				t.Fatal(err)
			}
			file, err := os.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			fileInfo, err := file.Stat()
			if err != nil {
				t.Fatal(err)
			}
			parent := &inputDirectory{path: parentPath, file: dir, info: dirInfo}
			original := &inputFile{path: path, name: "input", file: file, info: fileInfo, parent: parent, raw: []byte("original")}
			input := Inputs{original: &inputs{ctx: context.Background(), path: path, files: []*inputFile{original}}}
			if replace {
				if err := os.Rename(path, path+".old"); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte("original"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			err = input.Close()
			if replace && !errors.Is(err, ErrBinding) {
				t.Fatal("replacement was not retained as a failure", err)
			}
			if !replace && err != nil {
				t.Fatal("unchanged input refused", err)
			}
			if repeated := input.Close(); (repeated == nil) != (err == nil) {
				t.Fatal("repeated close changed the result", repeated)
			}
		})
	}
}
