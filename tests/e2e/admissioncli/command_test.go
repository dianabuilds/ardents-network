package admissioncli_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dianabuilds/ardents-network/tests/e2e/node/processfixture"
)

var admissionCommandBuild = processfixture.CommandBuild{Repository: filepath.Join("..", "..", ".."), Name: "ardents"}

func TestMain(m *testing.M) {
	os.Exit(admissionCommandBuild.Run(m))
}

func buildNodeCommand(t *testing.T) string {
	t.Helper()
	return admissionCommandBuild.Binary(t)
}
