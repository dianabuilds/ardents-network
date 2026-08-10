package architecture

import "testing"

func TestRouteExperimentInterfaceStaysDeep(t *testing.T) {
	t.Parallel()
	assertPackageExports(t, "internal/routeexperiment", "Run")
}
