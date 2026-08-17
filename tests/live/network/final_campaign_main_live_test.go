//go:build live

package network_test

import (
	"os"
	"testing"
)

func TestMain(tests *testing.M) {
	if os.Getenv("ARDENTS_BLOCKED_CELL_HELPER") == "1" && os.Getenv("ARDENTS_BLOCKED_CELL_WORKER") != "1" {
		os.Exit(runFinalCampaignRunner())
	}
	os.Exit(tests.Run())
}
