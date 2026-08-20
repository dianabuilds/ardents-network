package stage6verify_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/lab/stage6evidence"
)

var (
	evidenceBuildOnce sync.Once
	evidenceBinary    string
	evidenceBuildRoot string
	evidenceBuildErr  error
)

func TestMain(main *testing.M) {
	code := main.Run()
	if evidenceBuildRoot != "" {
		_ = os.RemoveAll(evidenceBuildRoot)
	}
	os.Exit(code)
}

func writeEvidenceCampaign(t *testing.T, root, source, dirty string) {
	t.Helper()
	evidenceBuildOnce.Do(func() {
		evidenceBuildRoot, evidenceBuildErr = os.MkdirTemp("", "ardents-stage6-evidence-command-")
		if evidenceBuildErr != nil {
			return
		}
		evidenceBinary = filepath.Join(evidenceBuildRoot, "stage6-evidence-lab")
		if os.PathSeparator == '\\' {
			evidenceBinary += ".exe"
		}
		command := exec.Command("go", "build", "-o", evidenceBinary, "./cmd/stage6-evidence-lab")
		command.Dir = filepath.Clean(filepath.Join("..", "..", ".."))
		if output, err := command.CombinedOutput(); err != nil {
			evidenceBuildErr = &buildEvidenceError{err: err, output: string(output)}
		}
	})
	if evidenceBuildErr != nil {
		t.Fatal(evidenceBuildErr)
	}
	if err := stage6evidence.Run(root, source, dirty, evidenceBinary); err != nil {
		t.Fatal(err)
	}
}

type buildEvidenceError struct {
	err    error
	output string
}

func (value *buildEvidenceError) Error() string {
	return "build stage6-evidence-lab: " + value.err.Error() + "\n" + value.output
}
