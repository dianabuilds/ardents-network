package architecture

import "testing"

func TestRouteComparisonInterfaceStaysDeep(t *testing.T) {
	t.Parallel()
	assertPackageExports(t, "internal/lab/routecomparison", "Run")
}
