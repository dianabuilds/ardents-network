//go:build linux

package endpoint

import (
	"context"
	"testing"
	"time"
)

func TestTextWorkerLaunchGateSerializesEndpointInstances(t *testing.T) {
	first, second := &endpoint{}, &endpoint{}
	releaseFirst, err := first.acquireTextLaunch(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	firstHeld := true
	defer func() {
		if firstHeld {
			releaseFirst()
		}
	}()

	blocked, cancel := context.WithTimeout(t.Context(), 25*time.Millisecond)
	defer cancel()
	if release, err := second.acquireTextLaunch(blocked); err == nil {
		release()
		t.Fatal("distinct Endpoint bypassed the process worker activation gate")
	}

	releaseFirst()
	firstHeld = false
	releaseSecond, err := second.acquireTextLaunch(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	releaseSecond()
}
