package modulecache

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOutputParentMustBeExternalAndUnaliased(t *testing.T) {
	workspace := t.TempDir()
	if _, _, err := resolveLocations(Options{Workspace: workspace,
		Output: filepath.Join(workspace, "gomodcache.tar.gz")}); err == nil {
		t.Fatal("repository-local module cache was accepted")
	}
	external := t.TempDir()
	alias := filepath.Join(external, "workspace-alias")
	if err := os.Symlink(workspace, alias); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if _, _, err := resolveLocations(Options{Workspace: workspace,
		Output: filepath.Join(alias, "gomodcache.tar.gz")}); err == nil {
		t.Fatal("aliased repository-local module cache was accepted")
	}
}

func TestModuleEnvironmentReplacesAmbientSupplyAuthority(t *testing.T) {
	for _, name := range []string{"GOENV", "GOPROXY", "GOSUMDB", "GONOSUMDB", "GOPRIVATE",
		"GONOPROXY", "GOINSECURE", "GOAUTH", "HTTP_PROXY", "HTTPS_PROXY", "NO_PROXY"} {
		t.Setenv(name, "ambient-value")
	}
	environment := strings.Join(moduleEnvironment("fixed-cache", "https://proxy.golang.org", "sum.golang.org"), "\n")
	if strings.Contains(environment, "ambient-value") ||
		!strings.Contains(environment, "GOMODCACHE=fixed-cache") ||
		!strings.Contains(environment, "GOPROXY=https://proxy.golang.org") ||
		!strings.Contains(environment, "GOSUMDB=sum.golang.org") {
		t.Fatalf("module environment is not frozen: %s", environment)
	}
}

func TestBoundedBufferRetainsPrefixAndRejectsOverflow(t *testing.T) {
	var value boundedBuffer
	input := make([]byte, commandOutputLimit+1)
	written, err := value.Write(input)
	if err != nil || written != len(input) || !value.overflow || value.Len() != commandOutputLimit {
		t.Fatalf("bounded buffer=(written=%d bytes=%d overflow=%v err=%v)",
			written, value.Len(), value.overflow, err)
	}
}
