package recoverysmoke

import (
	"os"
	"os/exec"
	"testing"
)

func TestVerifierExitMatchesVerdict(t *testing.T) {
	if os.Getenv("ARDENTS_VERIFIER_EXIT_HELPER") == "1" {
		os.Exit(1)
	}
	command := exec.Command(os.Args[0], "-test.run=TestVerifierExitMatchesVerdict")
	command.Env = append(os.Environ(), "ARDENTS_VERIFIER_EXIT_HELPER=1")
	err := command.Run()
	if !verifierExitMatches(err, "fail") || verifierExitMatches(err, "pass") ||
		!verifierExitMatches(nil, "pass") || verifierExitMatches(nil, "fail") {
		t.Fatal("verifier subprocess exit did not remain bound to its typed verdict")
	}
}
