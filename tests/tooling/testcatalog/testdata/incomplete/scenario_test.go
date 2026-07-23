package incomplete

import (
	"testing"

	"ardents/tests/testkit"
)

func TestIncompleteScenario(t *testing.T) {
	_ = testkit.BeginScenario(t, testkit.Spec{
		Layer:  testkit.LayerIntegration,
		Domain: "test-catalog",
		Suite:  "integration",
	})
}
