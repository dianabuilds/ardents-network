//go:build ignore

// Command check-deadcode verifies the reviewed deadcode result for each
// maintained production platform and requires no function to be test-only.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
)

const modulePath = "github.com/dianabuilds/ardents-network"

type reportPackage struct {
	Path  string
	Funcs []reportFunction
}

type reportFunction struct{ Name string }

type allowance struct {
	Symbols        []string
	Classification string
	Rationale      string
	Owner          string
	Retirement     string
}

type allowlist struct {
	Common     []allowance
	Production map[string][]allowance
}

func main() {
	allowed, err := readAllowlist("tests/profiles/deadcode-allowlist.json")
	if err != nil {
		fail(err)
	}
	platforms := []string{"windows-amd64", "linux-amd64"}
	for _, platform := range platforms {
		actual, runErr := dead(platform, false)
		if runErr != nil {
			fail(runErr)
		}
		if err := compare(platform+" production", actual, symbols(allowed.Common, allowed.Production[platform])); err != nil {
			fail(err)
		}
		testActual, runErr := dead(platform, true)
		if runErr != nil {
			fail(runErr)
		}
		if len(testActual) != 0 {
			fail(fmt.Errorf("%s test analysis found test-only functions:\n%s", platform, strings.Join(testActual, "\n")))
		}
	}
}

func readAllowlist(path string) (allowlist, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return allowlist{}, err
	}
	var result allowlist
	if err := json.Unmarshal(raw, &result); err != nil {
		return allowlist{}, fmt.Errorf("decode deadcode allowlist: %w", err)
	}
	if len(result.Production) != 2 || result.Production["windows-amd64"] == nil || result.Production["linux-amd64"] == nil {
		return allowlist{}, fmt.Errorf("deadcode allowlist must define exactly two production platforms")
	}
	for _, entries := range append([][]allowance{result.Common}, mapEntries(result.Production)...) {
		seen := map[string]bool{}
		for _, entry := range entries {
			if len(entry.Symbols) == 0 || entry.Classification == "" || entry.Rationale == "" || entry.Owner == "" || entry.Retirement == "" {
				return allowlist{}, fmt.Errorf("deadcode allowlist entry is incomplete")
			}
			for _, symbol := range entry.Symbols {
				if symbol == "" || seen[symbol] {
					return allowlist{}, fmt.Errorf("deadcode allowlist symbol is empty or duplicated")
				}
				seen[symbol] = true
			}
		}
	}
	for platform, entries := range result.Production {
		seen := map[string]bool{}
		for _, entry := range append(append([]allowance(nil), result.Common...), entries...) {
			for _, symbol := range entry.Symbols {
				if seen[symbol] {
					return allowlist{}, fmt.Errorf("deadcode allowlist symbol is duplicated for %s", platform)
				}
				seen[symbol] = true
			}
		}
	}
	return result, nil
}

func mapEntries(values map[string][]allowance) [][]allowance {
	result := make([][]allowance, 0, len(values))
	for _, entries := range values {
		result = append(result, entries)
	}
	return result
}

func dead(platform string, tests bool) ([]string, error) {
	parts := strings.Split(platform, "-")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid deadcode platform %q", platform)
	}
	args := []string{"-json"}
	if tests {
		args = append(args, "-test")
	}
	args = append(args, "./...")
	command := exec.Command("deadcode", args...)
	command.Env = platformEnvironment(parts[0], parts[1])
	output, err := command.Output()
	if err != nil {
		if message, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("deadcode %s failed: %s", platform, strings.TrimSpace(string(message.Stderr)))
		}
		return nil, err
	}
	var reports []reportPackage
	if err := json.Unmarshal(output, &reports); err != nil {
		return nil, fmt.Errorf("decode deadcode %s output: %w", platform, err)
	}
	set := map[string]bool{}
	for _, report := range reports {
		path := strings.TrimPrefix(report.Path, modulePath+"/")
		if path == report.Path {
			continue
		}
		for _, function := range report.Funcs {
			set[path+"."+function.Name] = true
		}
	}
	result := make([]string, 0, len(set))
	for symbol := range set {
		result = append(result, symbol)
	}
	sort.Strings(result)
	return result, nil
}

func platformEnvironment(goos, goarch string) []string {
	result := make([]string, 0, len(os.Environ())+2)
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "GOOS=") && !strings.HasPrefix(entry, "GOARCH=") {
			result = append(result, entry)
		}
	}
	return append(result, "GOOS="+goos, "GOARCH="+goarch)
}

func symbols(groups ...[]allowance) []string {
	var result []string
	for _, entries := range groups {
		for _, entry := range entries {
			result = append(result, entry.Symbols...)
		}
	}
	sort.Strings(result)
	return result
}

func compare(scope string, actual, expected []string) error {
	missing, unexpected := difference(expected, actual), difference(actual, expected)
	if len(missing) == 0 && len(unexpected) == 0 {
		return nil
	}
	var parts []string
	if len(missing) != 0 {
		parts = append(parts, "missing reviewed entries:\n"+strings.Join(missing, "\n"))
	}
	if len(unexpected) != 0 {
		parts = append(parts, "unreviewed dead functions:\n"+strings.Join(unexpected, "\n"))
	}
	return fmt.Errorf("%s deadcode differs from the exact allowlist:\n%s", scope, strings.Join(parts, "\n"))
}

func difference(left, right []string) []string {
	rightSet := make(map[string]bool, len(right))
	for _, value := range right {
		rightSet[value] = true
	}
	var result []string
	for _, value := range left {
		if !rightSet[value] {
			result = append(result, value)
		}
	}
	return result
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
