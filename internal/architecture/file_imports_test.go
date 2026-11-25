package architecture

import (
	"path"
	"path/filepath"
	"strings"
	"testing"
)

type fileImportEdge struct {
	owner      string
	dependency string
}

// assertDocumentedFileImports compares the documented files with the
// intersection of the package registry and the existing isolation rules.
// Inspect each package's Go files, including tests and other platforms.
func assertDocumentedFileImports(t *testing.T, root string, packages map[string][]string, registry map[string]packageRegistration) {
	t.Helper()
	for edge, documented := range readDocumentedFileImports(t, root) {
		if !registry[edge.owner].allowedImports[edge.dependency] {
			t.Errorf("file-specific import %s -> %s has no package-level registration", edge.owner, edge.dependency)
			continue
		}

		dependency := modulePath + "/" + edge.dependency
		for _, filename := range packages[filepath.Join(root, filepath.FromSlash(edge.owner))] {
			name := filepath.Base(filename)
			source := path.Join(edge.owner, name)
			allowed := successorImportAllowed(source, dependency) && routeMigrationBoundaryPermits(source, dependency)
			if allowed != documented[name] {
				t.Errorf("file-specific import %s -> %s: package map allows=%t, isolation allows=%t", source, edge.dependency, documented[name], allowed)
			}
			delete(documented, name)
		}
		for name := range documented {
			t.Errorf("file-specific import %s -> %s names a missing Go file", path.Join(edge.owner, name), edge.dependency)
		}
	}
}

func readDocumentedFileImports(t *testing.T, root string) map[fileImportEdge]map[string]bool {
	t.Helper()
	const heading = "\n## File-specific project imports\n"
	const table = "\n| Importing package | Imported package | Allowed Go files |\n| --- | --- | --- |\n"
	contents := string(readProjectFile(t, root, "docs/development/package-map.md"))
	_, section, ok := strings.Cut(contents, heading)
	if !ok {
		t.Fatal("package map has no file-specific project imports section")
	}
	section, _, _ = strings.Cut(section, "\n## ")
	_, rows, ok := strings.Cut(section, table)
	if !ok || strings.TrimSpace(rows) == "" {
		t.Fatal("package map has no file-specific project imports table")
	}

	edges := make(map[fileImportEdge]map[string]bool)
	for line := range strings.SplitSeq(strings.TrimSpace(rows), "\n") {
		cells := strings.Split(line, "|")
		if len(cells) != 5 || cells[0] != "" || cells[4] != "" {
			t.Fatalf("invalid file-specific import row: %s", line)
		}
		edge := fileImportEdge{
			owner:      strings.TrimSpace(cells[1]),
			dependency: strings.TrimSpace(cells[2]),
		}
		if _, exists := edges[edge]; exists {
			t.Fatalf("duplicate file-specific import %s -> %s", edge.owner, edge.dependency)
		}
		files := make(map[string]bool)
		for item := range strings.SplitSeq(cells[3], ",") {
			name := strings.Trim(strings.TrimSpace(item), "`")
			if name != path.Base(name) || !strings.HasSuffix(name, ".go") || files[name] {
				t.Fatalf("invalid or duplicate file in file-specific import row: %s", line)
			}
			files[name] = true
		}
		edges[edge] = files
	}
	return edges
}
