package architecture

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"go/parser"
	"go/token"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// third_party/quic-go is the dependency source of ADR-0136: the published quic-go
// module with one retained patch. It is a dependency, not project code. The shared
// walker skips it, so the first-party rules do not reach it. This file replaces
// them with one rule, that the directory equals the published module with the
// patch applied, and with the registration of every importer of the library.
const (
	quicGoModule    = "github.com/quic-go/quic-go"
	quicGoVersion   = "v0.66.0"
	quicGoCommit    = "d08d86670a1b480bb81a216fb59f6a2b23e8d444"
	quicGoModuleSum = "h1:NCsBBGZrI2HZC6aKt4I89T5i4Crt39EYNqkh0w2Wgmg="
	quicGoSource    = "third_party/quic-go"
	quicGoPatch     = "experiments/r-172-quic-return-path/quic-go-v0.66.0-probe-budget.patch"
	quicGoReplace   = "replace " + quicGoModule + " => ./" + quicGoSource
)

// The source keeps the published Go files and build files at the module root, the
// license, and the packages the root module and their tests import.
var (
	quicGoRootFiles   = []string{"go.mod", "go.sum", "LICENSE"}
	quicGoDirectories = []string{"internal", "qlog", "qlogwriter", "quicvarint", "testutils"}
)

func quicGoSelected(name string) bool {
	directory, _, _ := strings.Cut(name, "/")
	if directory != name {
		return slices.Contains(quicGoDirectories, directory)
	}
	return strings.HasSuffix(name, ".go") || slices.Contains(quicGoRootFiles, name)
}

// publishedQuicGo returns the selected files of the published module.
func publishedQuicGo(t *testing.T) map[string][]byte {
	t.Helper()
	archive, err := zip.OpenReader(publishedQuicGoZip(t))
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()
	prefix := quicGoModule + "@" + quicGoVersion + "/"
	files := make(map[string][]byte)
	for _, entry := range archive.File {
		name := strings.TrimPrefix(entry.Name, prefix)
		if !quicGoSelected(name) {
			continue
		}
		content, err := readZipEntry(entry)
		if err != nil {
			t.Fatal(err)
		}
		files[name] = content
	}
	if len(files) == 0 {
		t.Fatal("the published module archive has no selected file")
	}
	return files
}

// publishedQuicGoZip returns the archive of the published module from the module
// cache, downloading it when the cache is cold. The Go checksum database sum and
// the source commit identify the module; a missing cache and no network is an
// invalid environment, not a passing skip.
func publishedQuicGoZip(t *testing.T) string {
	t.Helper()
	cache := os.Getenv("GOMODCACHE")
	if cache == "" {
		command := exec.Command("go", "env", "GOMODCACHE")
		command.Env = append(externalEnvironment(os.Environ()), "GOWORK=off")
		output, err := command.Output()
		if err != nil {
			t.Fatalf("locate the module cache: %v", err)
		}
		cache = strings.TrimSpace(string(output))
	}
	archive := filepath.Join(cache, "cache", "download", quicGoModule, "@v", quicGoVersion+".zip")
	if _, err := os.Stat(archive); err == nil {
		if sum := moduleZipHash(t, archive); sum != quicGoModuleSum {
			t.Fatalf("cached %s@%s archive has sum %s, want %s", quicGoModule, quicGoVersion, sum, quicGoModuleSum)
		}
		return archive
	} else if !errors.Is(err, fs.ErrNotExist) {
		t.Fatal(err)
	}

	// The download runs in a scratch module without a go.sum entry, so Go would
	// ask the checksum database and cache the answer under GOPATH. GOPATH can be
	// read-only here (the Linux stand runs as an unprivileged user), and this
	// test verifies the archive against the pinned sum itself.
	scratch, gopath := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(scratch, "go.mod"), []byte("module quic-go-source\n\ngo 1.21\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	command := exec.Command("go", "mod", "download", "-json", quicGoModule+"@"+quicGoVersion)
	command.Dir = scratch
	command.Env = append(externalEnvironment(os.Environ()), "GOWORK=off", "GOPATH="+gopath, "GOFLAGS=-mod=mod", "GONOSUMDB="+quicGoModule)
	output, err := command.Output()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			output = exit.Stderr
		}
		t.Fatalf("download %s@%s for the dependency source check: %v\n%s", quicGoModule, quicGoVersion, err, output)
	}
	var downloaded struct {
		Zip    string
		Sum    string
		Origin struct{ Hash string }
		Error  string
	}
	if err := json.Unmarshal(output, &downloaded); err != nil {
		t.Fatalf("read the module download result: %v\n%s", err, output)
	}
	if downloaded.Error != "" {
		t.Fatalf("download %s@%s: %s", quicGoModule, quicGoVersion, downloaded.Error)
	}
	if downloaded.Sum != quicGoModuleSum || downloaded.Origin.Hash != quicGoCommit {
		t.Fatalf("downloaded %s@%s is sum %s at commit %s, want %s at %s", quicGoModule, quicGoVersion, downloaded.Sum, downloaded.Origin.Hash, quicGoModuleSum, quicGoCommit)
	}
	return downloaded.Zip
}

func readZipEntry(entry *zip.File) ([]byte, error) {
	reader, err := entry.Open()
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	return io.ReadAll(reader)
}

// moduleZipHash is the Go module checksum of an archive: SHA-256 over the
// sorted file names, each with the hash of its content.
func moduleZipHash(t *testing.T, archive string) string {
	t.Helper()
	reader, err := zip.OpenReader(archive)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	names := make([]string, 0, len(reader.File))
	for _, entry := range reader.File {
		names = append(names, entry.Name)
	}
	slices.Sort(names)
	entries := make(map[string]*zip.File, len(reader.File))
	for _, entry := range reader.File {
		entries[entry.Name] = entry
	}
	hash := sha256.New()
	for _, name := range names {
		content, err := readZipEntry(entries[name])
		if err != nil {
			t.Fatal(err)
		}
		fmt.Fprintf(hash, "%x  %s\n", sha256.Sum256(content), name)
	}
	return "h1:" + base64.StdEncoding.EncodeToString(hash.Sum(nil))
}

type unifiedHunk struct {
	start int // first old line the hunk refers to, counted from one
	count int // number of old lines
	lines []string
}

var unifiedHunkHeader = regexp.MustCompile(`^@@ -(\d+)(?:,(\d+))? \+\d+(?:,\d+)? @@`)

// applyUnifiedDiff applies a Git diff to the files. Every hunk must match its
// old lines exactly, so the diff also proves the identity of its base.
func applyUnifiedDiff(files map[string][]byte, diff []byte) ([]string, error) {
	type section struct {
		name  string
		hunks []unifiedHunk
	}
	var sections []*section
	var current *section
	var hunk *unifiedHunk
	for _, line := range strings.SplitAfter(string(diff), "\n") {
		if line == "" {
			continue
		}
		line = strings.TrimSuffix(line, "\n")
		switch {
		case strings.HasPrefix(line, "diff --git "):
			current = &section{}
			sections = append(sections, current)
			hunk = nil
		case strings.HasPrefix(line, "+++ "):
			if current == nil {
				return nil, errors.New("diff target without a file header")
			}
			if !strings.HasPrefix(line, "+++ b/") {
				return nil, fmt.Errorf("unexpected diff target %q", line)
			}
			current.name = strings.TrimSuffix(strings.TrimPrefix(line, "+++ b/"), "\t")
		case strings.HasPrefix(line, "@@"):
			if current == nil {
				return nil, errors.New("diff hunk without a file header")
			}
			header := unifiedHunkHeader.FindStringSubmatch(line)
			if header == nil {
				return nil, fmt.Errorf("unexpected hunk header %q", line)
			}
			start, err := strconv.Atoi(header[1])
			if err != nil {
				return nil, err
			}
			count := 1
			if header[2] != "" {
				count, err = strconv.Atoi(header[2])
				if err != nil {
					return nil, err
				}
			}
			hunk = &unifiedHunk{start: start, count: count}
			current.hunks = append(current.hunks, *hunk)
			hunk = &current.hunks[len(current.hunks)-1]
		case strings.HasPrefix(line, "\\"):
		case hunk != nil:
			hunk.lines = append(hunk.lines, line)
		}
	}
	var changed []string
	for _, item := range sections {
		content, present := files[item.name]
		if !present {
			return nil, fmt.Errorf("diff changes %s, which the module does not contain", item.name)
		}
		lines := splitLines(content)
		var result []string
		done := 0
		for _, hunk := range item.hunks {
			var old []string
			for _, line := range hunk.lines {
				if len(line) == 0 {
					return nil, fmt.Errorf("empty diff line in %s", item.name)
				}
				if line[0] == ' ' || line[0] == '-' {
					old = append(old, line[1:]+"\n")
				}
			}
			start := hunk.start - 1
			if hunk.count == 0 {
				start = hunk.start
			}
			if start < done || start+len(old) > len(lines) || !slices.Equal(lines[start:start+len(old)], old) {
				return nil, fmt.Errorf("the patch does not apply to %s at line %d", item.name, hunk.start)
			}
			result = append(result, lines[done:start]...)
			for _, line := range hunk.lines {
				if line[0] == ' ' || line[0] == '+' {
					result = append(result, line[1:]+"\n")
				}
			}
			done = start + len(old)
		}
		files[item.name] = []byte(strings.Join(append(result, lines[done:]...), ""))
		changed = append(changed, item.name)
	}
	if len(sections) == 0 {
		return nil, errors.New("the patch changes no file")
	}
	return changed, nil
}

func splitLines(content []byte) []string {
	lines := strings.SplitAfter(string(content), "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func directoryFiles(t *testing.T, directory string) map[string][]byte {
	t.Helper()
	files := make(map[string][]byte)
	err := filepath.WalkDir(directory, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("%s is not a regular file", path)
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		name, err := filepath.Rel(directory, path)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(name)] = content
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

// sourceDifferences lists every file that differs between two file sets.
func sourceDifferences(expected, actual map[string][]byte) []string {
	var differences []string
	for name, content := range expected {
		other, present := actual[name]
		switch {
		case !present:
			differences = append(differences, name+" is missing")
		case !bytes.Equal(content, other):
			differences = append(differences, name+" differs")
		}
	}
	for name := range actual {
		if _, present := expected[name]; !present {
			differences = append(differences, name+" is not part of the published module with the patch")
		}
	}
	slices.Sort(differences)
	return differences
}

func TestQuicGoSourceIsPublishedModuleWithOnePatch(t *testing.T) {
	root := repositoryRoot(t)
	manifest, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(strings.Split(string(manifest), "\n"), quicGoReplace) {
		t.Errorf("go.mod lacks the line %q", quicGoReplace)
	}
	expected := publishedQuicGo(t)
	patch, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(quicGoPatch)))
	if err != nil {
		t.Fatal(err)
	}
	changed, err := applyUnifiedDiff(expected, patch)
	if err != nil {
		t.Fatal(err)
	}
	// ADR-0136 admits the patch for the server path probe. Its reach into the
	// module is exactly the server's connection, its path manager and the packer;
	// the other changed files carry the module's own tests.
	var production []string
	for _, name := range changed {
		if !strings.HasSuffix(name, "_test.go") {
			production = append(production, name)
		}
	}
	slices.Sort(production)
	if want := []string{"connection.go", "packet_packer.go", "path_manager.go"}; !slices.Equal(production, want) {
		t.Errorf("the patch changes the production files %v, want %v", production, want)
	}
	if declaration, present := expected["go.mod"]; !present || !strings.Contains(string(declaration), "module "+quicGoModule+"\n") {
		t.Errorf("the dependency source does not declare module %s", quicGoModule)
	}
	if differences := sourceDifferences(expected, directoryFiles(t, filepath.Join(root, filepath.FromSlash(quicGoSource)))); len(differences) > 0 {
		t.Errorf("third_party/quic-go differs from the published module with the patch:\n%s", strings.Join(differences, "\n"))
	}
}

// The comparison must report every kind of drift, or the check above is blind.
func TestQuicGoSourceCheckDetectsChanges(t *testing.T) {
	expected := map[string][]byte{"a.go": []byte("one\n"), "d/b.go": []byte("two\n")}
	actual := map[string][]byte{"a.go": []byte("one\n"), "d/b.go": []byte("two\n")}
	if differences := sourceDifferences(expected, actual); len(differences) > 0 {
		t.Fatalf("equal file sets differ: %v", differences)
	}
	actual["a.go"] = []byte("changed\n")
	actual["extra.go"] = nil
	delete(actual, "d/b.go")
	differences := sourceDifferences(expected, actual)
	slices.Sort(differences)
	want := []string{"a.go differs", "d/b.go is missing", "extra.go is not part of the published module with the patch"}
	if !slices.Equal(differences, want) {
		t.Fatalf("differences %v, want %v", differences, want)
	}
}

// Only the dependency source may import quic-go, until a registered adapter
// returns with #1081. The multipath fork that ADR-0136 rejected may not be
// imported anywhere. Scans every maintained Go source, tests and build-tagged
// files included.
func TestQuicGoImportsAreRegistered(t *testing.T) {
	if err := rejectUnregisteredQUICImports(repositoryRoot(t)); err != nil {
		t.Fatal(err)
	}
	for _, dependency := range []string{quicGoModule, quicGoModule + "/http3", "github.com/qoke/mp-quic-go"} {
		path := filepath.Join(t.TempDir(), "consumer.go")
		if err := os.WriteFile(path, []byte("package fixture\nimport _ "+strconv.Quote(dependency)+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := rejectUnregisteredQUICImports(filepath.Dir(path)); err == nil {
			t.Fatalf("architecture guard accepted %s", dependency)
		}
	}
}

func rejectUnregisteredQUICImports(root string) error {
	return filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			switch {
			case entry.Name() == ".git" || entry.Name() == ".codex" || entry.Name() == ".agents":
				return filepath.SkipDir
			case filepath.ToSlash(strings.TrimPrefix(path, root+string(filepath.Separator))) == quicGoSource:
				// the dependency source imports its own module
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, item := range file.Imports {
			dependency, err := strconv.Unquote(item.Path.Value)
			if err != nil {
				return err
			}
			for _, forbidden := range []string{quicGoModule, "github.com/qoke"} {
				if dependency == forbidden || strings.HasPrefix(dependency, forbidden+"/") {
					return fmt.Errorf("unregistered QUIC import in %s: %s", path, dependency)
				}
			}
		}
		return nil
	})
}
