package enrollment

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

const companionVerificationChild = "ARDENTS_ENROLLMENT_COMPANION_CHILD"

func TestVerifyRunningCompanionUsesExactRunningArtifact(t *testing.T) {
	if os.Getenv(companionVerificationChild) == "1" {
		bundle, name := os.Getenv("ARDENTS_ENROLLMENT_COMPANION_BUNDLE"), os.Getenv("ARDENTS_ENROLLMENT_COMPANION_NAME")
		artifact, err := os.ReadFile(filepath.Join(bundle, name))
		if err != nil {
			t.Fatal(err)
		}
		if err := VerifyRunningCompanion(Request{BundleRoot: bundle}, name, artifact); err != nil {
			t.Fatal(err)
		}
		return
	}
	bundle := t.TempDir()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	name := "ardents-control-test" + filepath.Ext(executable)
	companion := filepath.Join(bundle, name)
	bytes, err := os.ReadFile(executable)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(companion, bytes, 0o700); err != nil {
		t.Fatal(err)
	}
	command := exec.Command(companion, "-test.run=^TestVerifyRunningCompanionUsesExactRunningArtifact$")
	command.Env = append(os.Environ(), companionVerificationChild+"=1", "ARDENTS_ENROLLMENT_COMPANION_BUNDLE="+bundle, "ARDENTS_ENROLLMENT_COMPANION_NAME="+name)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("running enrolled companion: %v\n%s", err, output)
	}
}
