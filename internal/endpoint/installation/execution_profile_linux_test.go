//go:build linux

package installation

import (
	"os"
	"testing"
)

func TestInstallationComponentProfilePrerequisite(t *testing.T) {
	switch os.Getenv("ARDENTS_INSTALLATION_COMPONENT_PROFILE") {
	case "":
		// Ordinary unprivileged checks do not claim Root filesystem coverage.
	case "root":
		if os.Geteuid() != 0 {
			t.Fatal("invalid environment: selected Root installation component profile requires UID 0")
		}
	default:
		t.Fatal("invalid environment: unknown installation component profile")
	}
}
