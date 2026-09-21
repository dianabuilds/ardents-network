package architecture

import (
	"os/exec"
	"runtime"
	"testing"
)

func TestSelectedApplicationSeamsMatchTheirAdapters(t *testing.T) {
	root := repositoryRoot(t)
	retiredConnection := "github.com/dianabuilds/ardents-network/internal/application/interfacev1/connection"
	if listedDirectImports(t, root, "./cmd/ardents")[retiredConnection] {
		t.Error("./cmd/ardents directly imports the retired generic AAI2 Connection client")
	}
	selectedConnection := "github.com/dianabuilds/ardents-network/internal/application/interfacev2/connection"
	selectedAdapters := []string{"./cmd/ardents-text"}
	if runtime.GOOS == "linux" {
		selectedAdapters = append(selectedAdapters, "./internal/endpoint")
	}
	for _, packagePath := range selectedAdapters {
		if !listedDependencies(t, root, packagePath)[selectedConnection] {
			t.Errorf("%s does not use the selected protected text Connection Module", packagePath)
		}
	}
	administration := "github.com/dianabuilds/ardents-network/internal/application/interfacev1/administration"
	for _, packagePath := range []string{"./cmd/ardents", "./internal/endpoint"} {
		if !listedDependencies(t, root, packagePath)[administration] {
			t.Errorf("%s does not use the shared Application Administration Module", packagePath)
		}
	}
}

func listedDirectImports(t *testing.T, root, packagePath string) map[string]bool {
	t.Helper()
	command := exec.Command("go", "list", "-f", `{{join .Imports "\n"}}`, packagePath)
	command.Dir = root
	output, err := command.Output()
	if err != nil {
		t.Fatalf("list direct imports for %s: %v", packagePath, err)
	}
	return packageSet(t, string(output))
}
