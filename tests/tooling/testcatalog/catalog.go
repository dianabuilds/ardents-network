package main

import (
	"path/filepath"
	"slices"

	"ardents/tests/tooling/scenariocatalog"
)

type catalogEntry struct {
	Package     string   `json:"package"`
	TestName    string   `json:"test_name"`
	File        string   `json:"file"`
	Layer       string   `json:"layer"`
	Domain      string   `json:"domain"`
	ScenarioID  string   `json:"scenario_id"`
	Suite       string   `json:"suite"`
	Tags        []string `json:"tags,omitempty"`
	Speed       string   `json:"speed,omitempty"`
	Environment string   `json:"environment,omitempty"`
}

type goListPackage struct {
	Dir          string   `json:"Dir"`
	ImportPath   string   `json:"ImportPath"`
	TestGoFiles  []string `json:"TestGoFiles"`
	XTestGoFiles []string `json:"XTestGoFiles"`
}

func listPackages(tags string, patterns []string) ([]goListPackage, error) {
	selected, err := scenariocatalog.ListPackages("", tags, patterns)
	if err != nil {
		return nil, err
	}
	packages := make([]goListPackage, 0, len(selected))
	for _, item := range selected {
		packages = append(packages, goListPackage{
			Dir: item.Dir, ImportPath: item.ImportPath,
			TestGoFiles: item.TestGoFiles, XTestGoFiles: item.XTestGoFiles,
		})
	}
	return packages, nil
}

func buildCatalog(packages []goListPackage) ([]catalogEntry, error) {
	var entries []catalogEntry
	for _, pkg := range packages {
		files := append([]string{}, pkg.TestGoFiles...)
		files = append(files, pkg.XTestGoFiles...)
		for _, name := range files {
			path := filepath.Join(pkg.Dir, name)
			fileEntries, err := parseCatalogFile(path, pkg.ImportPath)
			if err != nil {
				return nil, err
			}
			entries = append(entries, fileEntries...)
		}
	}
	return entries, nil
}

func parseCatalogFile(path string, importPath string) ([]catalogEntry, error) {
	parsed, err := scenariocatalog.ParseFile(path)
	if err != nil {
		return nil, err
	}
	entries := make([]catalogEntry, 0, len(parsed))
	for _, entry := range parsed {
		entries = append(entries, catalogEntry{
			Package: importPath, TestName: entry.TestName, File: filepath.Base(path),
			Layer: entry.Layer, Domain: entry.Domain, ScenarioID: entry.ScenarioID,
			Suite: entry.Suite, Tags: entry.Tags, Speed: entry.Speed, Environment: entry.Environment,
		})
	}
	return entries, nil
}

func filterCatalog(entries []catalogEntry, layer string, domain string, scenario string, tag string, suite string) []catalogEntry {
	filtered := make([]catalogEntry, 0, len(entries))
	for _, entry := range entries {
		if layer != "" && entry.Layer != layer {
			continue
		}
		if domain != "" && entry.Domain != domain {
			continue
		}
		if scenario != "" && entry.ScenarioID != scenario {
			continue
		}
		if suite != "" && entry.Suite != suite {
			continue
		}
		if tag != "" && !contains(entry.Tags, tag) {
			continue
		}
		filtered = append(filtered, entry)
	}
	return filtered
}

func contains(values []string, want string) bool {
	return slices.Contains(values, want)
}
