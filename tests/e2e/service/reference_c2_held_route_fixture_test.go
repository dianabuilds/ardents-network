package service_test

import (
	"path"
	"testing"
)

func TestReferenceC2HeldRouteFixtureUsesOneHostLocalControlRoot(t *testing.T) {
	fixture := map[string]any{}
	root := path.Join("qualification", "update-held")

	referenceC2ConfigureHeldRoute(fixture, root, true)

	want := map[string]string{
		"HeldRouteReady":     path.Join(root, "held-route-ready"),
		"HeldRouteUserReady": path.Join(root, "held-route-user-ready"),
		"HeldRouteRelease":   path.Join(root, "held-route-release"),
	}
	for name, expected := range want {
		if observed, ok := fixture[name].(string); !ok || observed != expected {
			t.Fatalf("%s = %#v, want %q", name, fixture[name], expected)
		}
	}
}

func referenceC2ConfigureHeldRoute(fixture map[string]any, root string, enabled bool) {
	if !enabled {
		return
	}
	fixture["HeldRouteReady"] = path.Join(root, "held-route-ready")
	fixture["HeldRouteUserReady"] = path.Join(root, "held-route-user-ready")
	fixture["HeldRouteRelease"] = path.Join(root, "held-route-release")
}

func TestReferenceC2OrdinaryFixtureHasNoHeldRouteControls(t *testing.T) {
	fixture := map[string]any{}
	referenceC2ConfigureHeldRoute(fixture, "unused", false)
	if len(fixture) != 0 {
		t.Fatalf("ordinary fixture controls = %#v, want none", fixture)
	}
}

func TestReferenceC2KeepsOneRouteHeldUntilControllerRelease(t *testing.T) {
	runReferenceC2(t, referenceC2Scenario{transparentApplication: true, heldRoute: true})
}
