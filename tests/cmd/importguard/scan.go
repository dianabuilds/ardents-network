package main

import (
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type finding struct {
	File   string `json:"file"`
	Import string `json:"import"`
}

var guardedImports = []string{
	"github.com/waku-org/go-waku",
	"github.com/libp2p/go-libp2p",
}

func scanRepository(root string) ([]finding, error) {
	root = filepath.Clean(root)
	var findings []finding
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if skipDir(root, path) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, ".pb.go") {
			return nil
		}
		if allowedImportBoundary(root, path) {
			return nil
		}

		fileFindings, err := scanFile(root, path)
		if err != nil {
			return err
		}
		findings = append(findings, fileFindings...)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return findings, nil
}

func skipDir(root string, path string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	rel = filepath.ToSlash(rel)
	switch rel {
	case ".git", ".artifacts", "third_party", "aim-core":
		return true
	default:
		return false
	}
}

func allowedImportBoundary(root string, path string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	rel = filepath.ToSlash(rel)
	return rel == "internal/network" ||
		strings.HasPrefix(rel, "internal/network/")
}

func scanFile(root string, path string) ([]finding, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}

	rel, err := filepath.Rel(root, path)
	if err != nil {
		return nil, err
	}
	rel = filepath.ToSlash(rel)

	var findings []finding
	for _, spec := range file.Imports {
		value, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			return nil, fmt.Errorf("decode import %s: %w", path, err)
		}
		if !matchesGuardedImport(value) {
			continue
		}
		findings = append(findings, finding{
			File:   rel,
			Import: value,
		})
	}
	return findings, nil
}

func matchesGuardedImport(value string) bool {
	for _, prefix := range guardedImports {
		if value == prefix || strings.HasPrefix(value, prefix+"/") {
			return true
		}
	}
	return false
}
