//go:build ignore

// Command run-fuzz-targets verifies and mutation-fuzzes the selected targets.
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

type fuzzTarget struct {
	packagePath string
	name        string
	budget      time.Duration
}

var selectedFuzzTargets = []fuzzTarget{
	{packagePath: "./internal/network/state", name: "FuzzCanonicalParsers", budget: 30 * time.Second},
	{packagePath: "./internal/contributor", name: "FuzzContributorJSONDecoders", budget: 30 * time.Second},
}

func main() {
	list := flag.Bool("list", false, "print selected fuzz targets")
	verify := flag.Bool("verify", false, "verify selected fuzz targets are declared")
	commands := flag.Bool("commands", false, "print selected mutation-fuzz commands")
	fuzzTime := flag.String("fuzztime", "", "override the selected mutation-fuzz budget")
	flag.Parse()
	if flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "run-fuzz-targets does not accept positional arguments")
		os.Exit(2)
	}
	if *list {
		for _, target := range selectedFuzzTargets {
			fmt.Printf("%s %s %s\n", target.packagePath, target.name, target.budget)
		}
		return
	}
	targets, err := targetsWithBudget(*fuzzTime)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if *commands {
		for _, target := range targets {
			fmt.Println(strings.Join(fuzzCommandArguments(target), "\t"))
		}
		return
	}
	if err := verifySelectedTargets(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if *verify {
		return
	}
	for _, target := range targets {
		if err := runTarget(target); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
}

func targetsWithBudget(raw string) ([]fuzzTarget, error) {
	if raw == "" {
		return selectedFuzzTargets, nil
	}
	budget, err := time.ParseDuration(raw)
	if err != nil || budget <= 0 {
		return nil, fmt.Errorf("fuzztime must be a positive duration, got %q", raw)
	}
	targets := make([]fuzzTarget, len(selectedFuzzTargets))
	copy(targets, selectedFuzzTargets)
	for index := range targets {
		targets[index].budget = budget
	}
	return targets, nil
}

func verifySelectedTargets() error {
	for _, target := range selectedFuzzTargets {
		output, err := exec.Command("go", "test", target.packagePath, "-run", "^$", "-list", "^"+target.name+"$").CombinedOutput()
		if err != nil {
			return fmt.Errorf("list selected fuzz target %s in %s: %w\n%s", target.name, target.packagePath, err, output)
		}
		if !containsLine(string(output), target.name) {
			return fmt.Errorf("selected fuzz target %s is absent from %s", target.name, target.packagePath)
		}
	}
	return nil
}

func runTarget(target fuzzTarget) error {
	arguments := fuzzCommandArguments(target)
	command := exec.Command(arguments[0], arguments[1:]...)
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	if err := command.Run(); err != nil {
		return fmt.Errorf("mutation-fuzz selected target %s in %s: %w", target.name, target.packagePath, err)
	}
	return nil
}

func fuzzCommandArguments(target fuzzTarget) []string {
	return []string{"go", "test", target.packagePath, "-run", "^$", "-fuzz", "^" + target.name + "$", "-fuzztime=" + target.budget.String()}
}

func containsLine(output, want string) bool {
	for _, line := range strings.Split(output, "\n") {
		if strings.TrimSpace(line) == want {
			return true
		}
	}
	return false
}
