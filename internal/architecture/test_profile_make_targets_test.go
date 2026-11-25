package architecture

import (
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"
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
	for line := range strings.SplitSeq(strings.ReplaceAll(makefile, "\r\n", "\n"), "\n") {
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

// This inventory check covers literal package arguments and top-level
// selectors. Runners still validate runtime prerequisites and execute tests.
func TestMakeProfileSelectorsHaveTests(t *testing.T) {
	root := repositoryRoot(t)
	makefile := string(readProjectFile(t, root, "Makefile"))
	selection := regexp.MustCompile(`^\t(?:sudo .* )?go test (.+) -run '([^'/]+)'`)
	for index, line := range strings.Split(makefile, "\n") {
		fields := selection.FindStringSubmatch(line)
		if fields == nil {
			continue
		}
		pattern := strings.ReplaceAll(fields[2], "$$", "$")
		if pattern == "^$" { // Fuzz profiles deliberately select no ordinary tests.
			continue
		}
		filter, err := regexp.Compile(pattern)
		if err != nil {
			t.Errorf("Makefile:%d: invalid test selector %q: %v", index+1, pattern, err)
			continue
		}
		args := strings.Fields(fields[1])
		var tags []string
		for i, arg := range args {
			if arg == "-tags" && i+1 < len(args) {
				tags = strings.Split(args[i+1], ",")
			}
		}
		for _, arg := range args {
			if !strings.HasPrefix(arg, "./") || strings.Contains(arg, "...") || strings.HasSuffix(arg, ".go") {
				continue
			}
			if !packageHasProfileTest(t, filepath.Join(root, arg), tags, filter) {
				t.Errorf("Makefile:%d: %s has no test matching %q with tags %v", index+1, arg, pattern, tags)
			}
		}
	}
}

func packageHasProfileTest(t *testing.T, directory string, tags []string, filter *regexp.Regexp) bool {
	t.Helper()
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	context := build.Default
	context.BuildTags = tags
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, "_test.go") {
			continue
		}
		selected := false
		for _, platform := range [][2]string{{"linux", "amd64"}, {"windows", "amd64"}, {"darwin", "arm64"}} {
			context.GOOS, context.GOARCH = platform[0], platform[1]
			match, err := context.MatchFile(directory, name)
			if err != nil {
				t.Fatal(err)
			}
			if match {
				selected = true
				break
			}
		}
		if !selected {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), filepath.Join(directory, name), nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatal(err)
		}
		for _, decl := range file.Decls {
			function, ok := decl.(*ast.FuncDecl)
			if !ok || function.Recv != nil || function.Name.Name == "TestMain" {
				continue
			}
			suffix, ok := strings.CutPrefix(function.Name.Name, "Test")
			first, _ := utf8.DecodeRuneInString(suffix)
			if ok && !unicode.IsLower(first) && filter.MatchString(function.Name.Name) {
				return true
			}
		}
	}
	return false
}

func TestMakeProfileSelectorRequiresAnEligibleDeclaration(t *testing.T) {
	directory := t.TempDir()
	source := `//go:build fixture_native

package inventory

import "testing"

// func TestComment(t *testing.T) {}
func TestSelected(t *testing.T) {}
func TestMain(m *testing.M) {}
func (structReceiver) TestMethod(t *testing.T) {}

type structReceiver struct{}
`
	if err := os.WriteFile(filepath.Join(directory, "selection_linux_test.go"), []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		selector string
		tags     []string
		want     bool
	}{
		{selector: "^TestSelected$", tags: []string{"fixture_native"}, want: true},
		{selector: "^TestSelected$"},
		{selector: "^Test(Comment|Method|Main|Moved)$", tags: []string{"fixture_native"}},
	} {
		if got := packageHasProfileTest(t, directory, tc.tags, regexp.MustCompile(tc.selector)); got != tc.want {
			t.Errorf("selector %s with tags %v matches = %t, want %t", tc.selector, tc.tags, got, tc.want)
		}
	}
}
