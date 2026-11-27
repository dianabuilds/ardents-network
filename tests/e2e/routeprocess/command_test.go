package routeprocess_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dianabuilds/ardents-network/tests/e2e/node/processfixture"
)

var nodeCommandBuild = processfixture.CommandBuild{Repository: filepath.Join("..", "..", ".."), Name: "ardents"}

func TestMain(m *testing.M) {
	os.Exit(nodeCommandBuild.Run(m))
}

func buildNodeCommand(t *testing.T) string {
	t.Helper()
	return nodeCommandBuild.Binary(t)
}
