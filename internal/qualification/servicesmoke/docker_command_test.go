package servicesmoke

import (
	"path/filepath"
	"reflect"
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

func TestServiceTopologyExcludesLaterRecoveryProfiles(t *testing.T) {
	want := []string{"--profile", "negative", "--profile", "verify", "--profile", "setup", "config"}
	if got := serviceTopologyArguments(); !reflect.DeepEqual(got, want) {
		t.Fatalf("service topology arguments=%v; want %v", got, want)
	}
}
