package servicesmoke

import (
	"path/filepath"
	"testing"
)

func TestComposeEnvironmentProvidesSharedRecoveryGateRoot(t *testing.T) {
	root := t.TempDir()
	observer := dockerObserver{input: Config{FixtureRoot: root}}
	want := "ARDENTS_RECOVERY_GATE_HOST=" + filepath.Join(root, "gate")
	for _, value := range observer.composeEnvironment() {
		if value == want {
			return
		}
	}
	t.Fatalf("Compose environment does not contain %q", want)
}
