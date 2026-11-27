//go:build linux && (source_installed_native || forwarding_platform_native || join_platform_native || descriptor_platform_native || introduction_repair_native)

package nodeinstalled_test

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
