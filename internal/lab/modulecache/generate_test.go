package modulecache

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestGeneratePublishesStableManifestBoundArchive(t *testing.T) {
	workspace := fixtureModule(t, "module example.com/stage5fixture\n\ngo 1.26.0\n")
	firstPath := filepath.Join(t.TempDir(), "first.tar.gz")
	secondPath := filepath.Join(t.TempDir(), "second.tar.gz")
	first, firstErr := Generate(Options{Workspace: workspace, Output: firstPath})
	second, secondErr := Generate(Options{Workspace: workspace, Output: secondPath})
	if firstErr != nil || secondErr != nil || first != second || first.Bytes == 0 {
		t.Fatalf("generated receipts differ: %+v/%v %+v/%v", first, firstErr, second, secondErr)
	}
	entries := archiveEntryNames(t, firstPath)
	for _, required := range []string{".ardents-stage5/source.sha256", ".ardents-stage5/modules.txt"} {
		if !entries[required] {
			t.Fatalf("generated archive omits %s", required)
		}
	}
	for name := range entries {
		if strings.Contains(name, "/sumdb/") || filepath.Base(name) == "list" ||
			strings.HasSuffix(name, ".lock") || strings.HasSuffix(name, ".tmp") {
			t.Fatalf("generated archive retained volatile Go lookup state %s", name)
		}
	}
}

func TestGenerateCacheBuildsDependencyOffline(t *testing.T) {
	proxyRoot := t.TempDir()
	writeProxyModule(t, proxyRoot)
	server := httptest.NewServer(http.FileServer(http.Dir(proxyRoot)))
	defer server.Close()
	workspace := fixtureModule(t, "module example.com/stage5app\n\ngo 1.26.0\n\nrequire example.com/dependency v1.0.0\n")
	if err := os.WriteFile(filepath.Join(workspace, "app.go"),
		[]byte("package app\n\nimport \"example.com/dependency\"\n\nvar Value = dependency.Value\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runFixtureGo(t, workspace, []string{"GOMODCACHE=" + t.TempDir()}, server.URL, "mod", "tidy")
	archive := filepath.Join(t.TempDir(), "gomodcache.tar.gz")
	if _, err := generate(Options{Workspace: workspace, Output: archive}, server.URL, "off"); err != nil {
		t.Fatal(err)
	}
	cache := t.TempDir()
	extractArchive(t, archive, cache)
	for _, arguments := range [][]string{{"list", "-m", "all"}, {"mod", "verify"}, {"test", "./..."}} {
		runFixtureGo(t, workspace, []string{"GOMODCACHE=" + cache}, "off", arguments...)
	}
}

func TestGenerateCommandFailureLeavesNoOutput(t *testing.T) {
	workspace := fixtureModule(t, "not a module\n")
	parent := t.TempDir()
	target := filepath.Join(parent, "gomodcache.tar.gz")
	if _, err := Generate(Options{Workspace: workspace, Output: target}); err == nil {
		t.Fatal("invalid module graph was accepted")
	}
	entries, err := os.ReadDir(parent)
	if err != nil || len(entries) != 0 {
		t.Fatalf("failed generation left files=%v err=%v", entries, err)
	}
}

func fixtureModule(t *testing.T, module string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte(module), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "go.sum"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	return root
}

func archiveEntryNames(t *testing.T, path string) map[string]bool {
	t.Helper()
	input, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer input.Close()
	compressed, err := gzip.NewReader(input)
	if err != nil {
		t.Fatal(err)
	}
	defer compressed.Close()
	reader := tar.NewReader(compressed)
	result := make(map[string]bool)
	for {
		header, err := reader.Next()
		if err == io.EOF {
			return result
		}
		if err != nil {
			t.Fatal(err)
		}
		result[header.Name] = true
	}
}

func writeProxyModule(t *testing.T, root string) {
	t.Helper()
	directory := filepath.Join(root, "example.com", "dependency", "@v")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{"list": "v1.0.0\n", "v1.0.0.info": "{\"Version\":\"v1.0.0\",\"Time\":\"2026-01-01T00:00:00Z\"}\n",
		"v1.0.0.mod": "module example.com/dependency\n\ngo 1.26.0\n"}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(directory, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	output, err := os.Create(filepath.Join(directory, "v1.0.0.zip"))
	if err != nil {
		t.Fatal(err)
	}
	archive := zip.NewWriter(output)
	for name, content := range map[string]string{
		"example.com/dependency@v1.0.0/go.mod":        files["v1.0.0.mod"],
		"example.com/dependency@v1.0.0/dependency.go": "package dependency\n\nconst Value = 7\n"} {
		entry, err := archive.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	if err := output.Close(); err != nil {
		t.Fatal(err)
	}
}

func runFixtureGo(t *testing.T, root string, extra []string, proxy string, arguments ...string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, "go", arguments...)
	command.Dir = root
	command.Env = append(os.Environ(), append(extra, "GOTOOLCHAIN=local", "GOWORK=off", "GOENV=off",
		"GOCACHE="+t.TempDir(), "GOPROXY="+proxy, "GOSUMDB=off")...)
	if output, err := command.CombinedOutput(); err != nil || ctx.Err() != nil {
		t.Fatalf("go %s failed: %v/%v: %s", strings.Join(arguments, " "), err, ctx.Err(), output)
	}
}

func extractArchive(t *testing.T, path, root string) {
	t.Helper()
	input, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer input.Close()
	compressed, err := gzip.NewReader(input)
	if err != nil {
		t.Fatal(err)
	}
	defer compressed.Close()
	reader := tar.NewReader(compressed)
	for {
		header, err := reader.Next()
		if err == io.EOF {
			return
		}
		if err != nil {
			t.Fatal(err)
		}
		target := filepath.Join(root, filepath.FromSlash(header.Name))
		if header.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o700); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			t.Fatal(err)
		}
		output, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.Copy(output, reader); err != nil {
			t.Fatal(err)
		}
		if err := output.Close(); err != nil {
			t.Fatal(err)
		}
	}
}
