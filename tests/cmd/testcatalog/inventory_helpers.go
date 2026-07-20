package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func findModulePath() (string, error) {
	data, err := os.ReadFile("go.mod")
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module ")), nil
		}
	}
	return "", fmt.Errorf("module path not found in go.mod")
}

func importPathForFile(path string, modulePath string) (string, error) {
	dir := filepath.Dir(path)
	rel, err := filepath.Rel(".", dir)
	if err != nil {
		return filepath.ToSlash(filepath.Join(modulePath, filepath.Base(dir))), nil
	}
	return filepath.ToSlash(filepath.Join(modulePath, rel)), nil
}

func packagePathFromImport(importPath string, modulePath string) string {
	prefix := modulePath + "/"
	if strings.HasPrefix(importPath, prefix) {
		return strings.TrimPrefix(importPath, prefix)
	}
	return importPath
}

func inferLayerFromPath(path string) string {
	slashPath := filepath.ToSlash(path)
	switch {
	case strings.Contains(slashPath, "/tests/integration/") || strings.HasPrefix(slashPath, "tests/integration/"):
		return "integration"
	case strings.Contains(slashPath, "/tests/e2e/") || strings.HasPrefix(slashPath, "tests/e2e/"):
		return "e2e"
	default:
		return ""
	}
}

func uniqueStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func uniqueScenarioDocs(values []scenarioDoc) []scenarioDoc {
	if len(values) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	result := make([]scenarioDoc, 0, len(values))
	for _, value := range values {
		if _, exists := seen[value.ScenarioID]; exists {
			continue
		}
		seen[value.ScenarioID] = struct{}{}
		result = append(result, value)
	}
	return result
}

func countTestsBySource(tests []inventoryTest, source string) int {
	count := 0
	for _, test := range tests {
		if test.BindingSource == source {
			count++
		}
	}
	return count
}

func countTestsWithIssue(tests []inventoryTest, issue string) int {
	count := 0
	for _, test := range tests {
		if contains(test.Issues, issue) {
			count++
		}
	}
	return count
}

func countScenariosWithIssue(scenarios []inventoryScenario, issue string) int {
	count := 0
	for _, scenario := range scenarios {
		if contains(scenario.Issues, issue) {
			count++
		}
	}
	return count
}

func countTestIssues(tests []inventoryTest) int {
	count := 0
	for _, test := range tests {
		count += len(test.Issues)
	}
	return count
}

func countScenarioIssues(scenarios []inventoryScenario) int {
	count := 0
	for _, scenario := range scenarios {
		count += len(scenario.Issues)
	}
	return count
}
