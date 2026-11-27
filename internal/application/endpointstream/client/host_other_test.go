//go:build !linux

package client

import (
	"context"
	"testing"
)

func TestConnectRefusesUnsupportedHostBeforeReadingLink(t *testing.T) {
	err := ConnectLocalStream(context.Background(), "missing-socket", "missing-link")
	if err == nil || err.Error() != "application stream requires the selected Linux host" {
		t.Fatalf("unsupported host did not refuse before private input: %v", err)
	}
}
