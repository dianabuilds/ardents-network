package execution_test

import (
	"ardents/tests/testkit"
	"testing"
)

func helperProcessConfig(t *testing.T, mode string) string {
	t.Helper()
	return testkit.HelperProcessConfig(t, mode)
}
