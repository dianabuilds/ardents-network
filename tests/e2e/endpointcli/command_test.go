package endpointcli_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dianabuilds/ardents-network/tests/e2e/node/processfixture"
)

var endpointCommandBuild = processfixture.CommandBuild{
	Repository:  filepath.Join("..", "..", ".."),
	Name:        "ardents-next",
	ArtifactEnv: "ARDENTS_E2E_COMMAND",
}

func TestMain(m *testing.M) {
	os.Exit(endpointCommandBuild.Run(m))
}

func buildArdentsNext(t *testing.T) string {
	t.Helper()
	return endpointCommandBuild.Binary(t)
}
