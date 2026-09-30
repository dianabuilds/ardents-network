//go:build ignore

// Command protected-generation-check validates supplied producer bytes without
// changing the descriptor that the operator's signed Release set must bind.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/dianabuilds/ardents-network/internal/enrollment"
)

func main() {
	if err := check(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}

func check() error {
	if len(os.Args) != 5 {
		return fmt.Errorf("usage: protected-generation-check <static-root> <endpoint> <text-worker> <release-identity>")
	}
	root := os.Args[1]
	paths := map[string]string{"ardents-linux-amd64": os.Args[2], "ardents-text-linux-amd64": os.Args[3]}
	for _, name := range []string{"ardents-text-reader@.service", "ardents-text-publisher@.service", "ardents-text-reader.socket", "ardents-text-publisher.socket", "50-ardents-text.rules", "ardents-text.conf", "ardents-endpoint.service"} {
		paths[name] = filepath.Join(root, name)
	}
	files := make(map[string][]byte, len(paths))
	for name, path := range paths {
		contents, err := read(path, 64<<20)
		if err != nil {
			return err
		}
		files[name] = contents
	}
	raw, err := read(filepath.Join(root, "protected-endpoint.json"), 16<<10)
	if err != nil {
		return err
	}
	return enrollment.ValidateProtectedGeneration(raw, files, os.Args[4])
}

func read(path string, maximum int64) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() < 1 || info.Size() > maximum {
		return nil, fmt.Errorf("protected producer input is not a bounded direct regular file: %s", path)
	}
	return os.ReadFile(path)
}
