package architecture

import (
	"strings"
	"testing"
)

type makeProfileTarget struct {
	dependencies []string
	commands     []string
}

// makeProfileTargetCommands follows the Make targets that a checked profile
// actually invokes. A timeout elsewhere in the Makefile cannot satisfy the
// profile registry's promise for this target.
func makeProfileTargetCommands(makefile, target string) (string, bool) {
	targets := make(map[string]*makeProfileTarget)
	var current *makeProfileTarget
	for _, line := range strings.Split(strings.ReplaceAll(makefile, "\r\n", "\n"), "\n") {
		if strings.HasPrefix(line, "\t") {
			if current != nil {
				current.commands = append(current.commands, strings.TrimSpace(line))
			}
			continue
		}
		current = nil
		name, rest, found := strings.Cut(line, ":")
		if !found || name == "" || strings.ContainsAny(name, " \t$().") {
			continue
		}
		rest = strings.TrimSpace(rest)
		// Target-specific variable assignments do not define a recipe or
		// prerequisite edge. The actual target rule may follow later.
		if strings.HasPrefix(rest, "export ") || strings.Contains(rest, ":=") {
			continue
		}
		current = targets[name]
		if current == nil {
			current = &makeProfileTarget{}
			targets[name] = current
		}
		beforeComment, _, _ := strings.Cut(rest, "#")
		current.dependencies = append(current.dependencies, strings.Fields(beforeComment)...)
	}
	if targets[target] == nil {
		return "", false
	}
	visited := make(map[string]bool)
	var commands []string
	var collect func(string)
	collect = func(name string) {
		if visited[name] {
			return
		}
		visited[name] = true
		rule := targets[name]
		if rule == nil {
			return
		}
		commands = append(commands, rule.commands...)
		for _, dependency := range rule.dependencies {
			collect(dependency)
		}
	}
	collect(target)
	return strings.Join(commands, "\n"), true
}

func TestMakeProfileTargetCommandsDoesNotBorrowAnotherProfileTimeout(t *testing.T) {
	makefile := "profile: dependency\n\tgo test -timeout=4m\n" +
		"dependency:\n\tgo test -timeout=2m\n" +
		"unrelated:\n\tgo test -timeout=5m\n" +
		"# absent:\n\tgo test -timeout=8m\n"
	commands, found := makeProfileTargetCommands(makefile, "profile")
	if !found || !strings.Contains(commands, "-timeout=4m") || !strings.Contains(commands, "-timeout=2m") {
		t.Fatalf("profile commands = %q, found = %t", commands, found)
	}
	if strings.Contains(commands, "-timeout=5m") || strings.Contains(commands, "-timeout=8m") {
		t.Fatalf("profile borrowed another target's timeout: %q", commands)
	}
	if _, found := makeProfileTargetCommands(makefile, "absent"); found {
		t.Fatal("commented target was treated as active")
	}
}
