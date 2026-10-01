//go:build ignore

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type prCheckRegistry struct {
	Mappings []prCheckMapping `json:"pr_check_mappings"`
}
type prCheckMapping struct {
	PathPrefix string         `json:"path_prefix"`
	Owners     []prCheckOwner `json:"owners"`
	Race       bool           `json:"race"`
}
type prCheckOwner struct {
	Package     string `json:"package"`
	TestPattern string `json:"test_pattern"`
}

func loadPRCheckMappings(root string) ([]prCheckMapping, error) {
	body, err := os.ReadFile(filepath.Join(root, "docs", "development", "ownership.json"))
	if err != nil {
		return nil, err
	}
	var registry prCheckRegistry
	if err := json.Unmarshal(body, &registry); err != nil {
		return nil, err
	}
	for _, mapping := range registry.Mappings {
		if mapping.PathPrefix == "" || strings.HasPrefix(mapping.PathPrefix, "/") || strings.Contains(mapping.PathPrefix, "..") || len(mapping.Owners) == 0 {
			return nil, errors.New("PR check ownership mapping is invalid")
		}
		for _, owner := range mapping.Owners {
			if owner.Package == "" || strings.Contains(owner.Package, "..") {
				return nil, errors.New("PR check package owner is invalid")
			}
			if info, err := os.Stat(filepath.Join(root, filepath.FromSlash(owner.Package))); err != nil || !info.IsDir() {
				return nil, fmt.Errorf("PR check owner package %s is unavailable", owner.Package)
			}
			if _, err := regexp.Compile(owner.TestPattern); err != nil {
				return nil, fmt.Errorf("PR check owner %s: %w", owner.Package, err)
			}
		}
	}
	return registry.Mappings, nil
}
func applyPRCheckMappings(root, path string, mappings []prCheckMapping, directories map[string]*owner) error {
	for _, mapping := range mappings {
		if !strings.HasPrefix(path, mapping.PathPrefix) {
			continue
		}
		for _, selected := range mapping.Owners {
			directory := filepath.Join(root, filepath.FromSlash(selected.Package))
			current := directories[directory]
			if current == nil {
				return fmt.Errorf("PR check owner package %s is unavailable", selected.Package)
			}
			pattern, err := regexp.Compile(selected.TestPattern)
			if err != nil {
				return err
			}
			current.compile = true
			current.race = current.race || mapping.Race
			current.reasons[path] = true
			matched := false
			for _, decl := range current.declarations {
				if decl.test && pattern.MatchString(decl.name) {
					current.changed[decl.name] = true
					matched = true
				}
			}
			if !matched {
				matched, err = selectMappedTests(directory, pattern, current)
				if err != nil {
					return fmt.Errorf("inspect PR check owner %s: %w", selected.Package, err)
				}
			}
			if !matched {
				return fmt.Errorf("PR check owner %s pattern %q matches no test", selected.Package, selected.TestPattern)
			}
		}
	}
	return nil
}
func selectMappedTests(directory string, pattern *regexp.Regexp, current *owner) (bool, error) {
	files, err := filepath.Glob(filepath.Join(directory, "*_test.go"))
	if err != nil {
		return false, err
	}
	matched := false
	for _, name := range files {
		file, err := parser.ParseFile(token.NewFileSet(), name, nil, 0)
		if err != nil {
			return false, err
		}
		for _, node := range file.Decls {
			function, ok := node.(*ast.FuncDecl)
			if ok && function.Recv == nil && pattern.MatchString(function.Name.Name) {
				current.changed[function.Name.Name] = true
				current.declarations = append(current.declarations, declaration{name: function.Name.Name, test: true})
				matched = true
			}
		}
	}
	return matched, nil
}
