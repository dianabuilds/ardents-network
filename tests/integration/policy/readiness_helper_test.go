//go:build integration

package policy_test

import (
	"testing"

	"ardents/tests/testkit"
)

var policyReadyHelper = testkit.ReadinessHelper{
	TestName:   "TestPolicyReadyHelper",
	EnabledEnv: "ARDENTS_POLICY_READY_HELPER",
	AddressEnv: "ARDENTS_POLICY_READY_ADDRESS",
}

func TestPolicyReadyHelper(t *testing.T) {
	policyReadyHelper.Run()
}

//goland:noinspection ALL
func policyReadyFixture(t *testing.T) (string, string, string) {
	return policyReadyHelper.Fixture(t)
}
