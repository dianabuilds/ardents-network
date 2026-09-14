//go:build ignore

// Command select-pr-checks selects and runs only tests affected by a pull-request
// diff. It prints the merge base, changed owners and the rationale for every
// selected package so the CI decision is reviewable.
package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

type check struct {
	pkg    string
	run    string
	race   bool
	reason string
	paths  []string
}

func main() {
	base := flag.String("base", "origin/main", "merge-base ref")
	head := flag.String("head", "HEAD", "head ref")
	execute := flag.Bool("execute", false, "run selected checks")
	flag.Parse()
	paths, err := changedPaths(*base, *head)
	if err != nil {
		fail(err)
	}
	checks := selectChecks(paths)
	fmt.Printf("pr-check-selection base=%s head=%s\n", *base, *head)
	for _, path := range paths {
		fmt.Printf("changed %s\n", path)
	}
	for _, selected := range checks {
		fmt.Printf("selected package=%s race=%t run=%q reason=%s paths=%s\n", selected.pkg, selected.race, selected.run, selected.reason, strings.Join(selected.paths, ","))
	}
	if !*execute {
		return
	}
	for _, selected := range checks {
		args := []string{"test", "-count=1"}
		if selected.race {
			args = append(args, "-race")
		}
		args = append(args, selected.pkg)
		if selected.run != "" {
			args = append(args, "-run", selected.run)
		}
		command := exec.Command("go", args...)
		command.Stdout, command.Stderr = os.Stdout, os.Stderr
		fmt.Printf("run go %s\n", strings.Join(args, " "))
		if err := command.Run(); err != nil {
			fail(fmt.Errorf("%s: %w", selected.pkg, err))
		}
	}
}

func changedPaths(base, head string) ([]string, error) {
	if base == "" || head == "" {
		return nil, errors.New("base and head are required")
	}
	output, err := exec.Command("git", "diff", "--name-only", base, head).Output()
	if err != nil {
		return nil, fmt.Errorf("read pull-request diff: %w", err)
	}
	var paths []string
	for _, line := range bytes.Split(output, []byte{'\n'}) {
		path := strings.TrimSpace(string(line))
		if path != "" {
			paths = append(paths, filepath.ToSlash(path))
		}
	}
	sort.Strings(paths)
	return paths, nil
}

func selectChecks(paths []string) []check {
	byKey := map[string]*check{}
	add := func(next check, path string) {
		key := fmt.Sprintf("%s|%s|%t", next.pkg, next.run, next.race)
		current := byKey[key]
		if current == nil {
			current = &next
			byKey[key] = current
		}
		current.paths = append(current.paths, path)
	}
	for _, path := range paths {
		switch {
		case strings.HasPrefix(path, "internal/application/streamqualification/"):
			add(check{pkg: "./internal/application/streamqualification", race: true, reason: "changed fixed qualification worker owner"}, path)
		case path == "internal/endpoint/text_issuance_network_test.go":
			add(check{pkg: "./internal/endpoint", race: true, run: "^(TestTextDataJoinIsolatedRoleObservations|TestTextPublicationIsolatedRoleObservations|TestTextNetworkFixtureWindowAvoidsExpiredPermissionImport)$", reason: "changed shared text-network fixture and its role-observation consumers"}, path)
		case path == "internal/endpoint/text_data_role_observation_test.go":
			add(check{pkg: "./internal/endpoint", race: true, run: "^TestTextDataJoinIsolatedRoleObservations$", reason: "changed isolated DataJoin observation"}, path)
		case path == "internal/endpoint/text_network_fixture_window_test.go":
			add(check{pkg: "./internal/endpoint", race: true, run: "^TestTextNetworkFixtureWindowAvoidsExpiredPermissionImport$", reason: "changed Permission-window fixture regression"}, path)
		case path == "internal/architecture/test_profiles_test.go":
			add(check{pkg: "./internal/architecture", run: "^(TestPackageProfileMembershipIsComplete|TestEndToEndPackageProfileMembershipIsComplete|TestProcessProfileSerializesPackagesSharingLoopbackResources|TestProfilePackageEntriesAreCurrent|TestTestProfileRegistryIsFactualAndWired|TestSuiteRootsBelongToOneExecutionProfile)$", reason: "changed test-profile registry owner"}, path)
		case path == "Makefile" || path == ".github/workflows/quality.yml" || path == "internal/architecture/quality_wiring_test.go" || strings.HasPrefix(path, "scripts/") || strings.HasPrefix(path, "tests/profiles/"):
			add(check{pkg: "./internal/architecture", run: "^TestRepositoryArchitecture$", reason: "changed quality selection or repository quality contract"}, path)
		case strings.HasSuffix(path, ".go") && (strings.HasPrefix(path, "internal/") || strings.HasPrefix(path, "cmd/") || strings.HasPrefix(path, "tests/")):
			directory := "./" + filepath.ToSlash(filepath.Dir(path))
			add(check{pkg: directory, reason: "changed Go package owner"}, path)
		}
	}
	checks := make([]check, 0, len(byKey))
	for _, selected := range byKey {
		sort.Strings(selected.paths)
		checks = append(checks, *selected)
	}
	sort.Slice(checks, func(left, right int) bool {
		return checks[left].pkg+checks[left].run < checks[right].pkg+checks[right].run
	})
	return checks
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "pr-check-selection:", err)
	os.Exit(1)
}
