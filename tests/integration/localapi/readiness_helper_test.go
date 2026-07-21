//go:build integration

package localapi_test

import (
	"testing"

	"ardents/tests/testkit"
)

var localControlReadyHelper = testkit.ReadinessHelper{
	TestName:   "TestLocalControlReadyHelper",
	EnabledEnv: "ARDENTS_LOCAL_CONTROL_READY_HELPER",
	AddressEnv: "ARDENTS_LOCAL_CONTROL_READY_ADDRESS",
}

func TestLocalControlReadyHelper(t *testing.T) {
	localControlReadyHelper.Run()
}

//goland:noinspection ALL
func localControlReadyFixture(t *testing.T) (string, string, string) {
	return localControlReadyHelper.Fixture(t)
}
