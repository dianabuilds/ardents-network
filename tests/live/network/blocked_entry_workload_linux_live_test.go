//go:build linux && live

package network_test

import (
	"os"
	"strconv"
	"testing"
)

func blockedStreamArguments(t *testing.T, role string) []string {
	t.Helper()
	socket, send, receive := "/run/ardents/client-app/app.sock", "512", "65536"
	if role == "publisher" {
		socket, send, receive = "/run/ardents/publisher-app/app.sock", "65536", "512"
	} else if role != "client" {
		t.Fatalf("unknown blocked Application role %q", role)
	}
	mode := "run-short"
	if os.Getenv("ARDENTS_BLOCKED_WORKLOAD") == "sustained" {
		mode = "run"
		send, receive = os.Getenv("ARDENTS_BLOCKED_SEND_BYTES"), os.Getenv("ARDENTS_BLOCKED_RECEIVE_BYTES")
		for _, value := range []string{send, receive} {
			parsed, err := strconv.ParseUint(value, 10, 32)
			if err != nil || parsed > 768<<20 {
				t.Fatalf("invalid sustained blocked Application byte count %q", value)
			}
		}
	}
	return []string{mode, role, socket, "/run/secure/own.hex", "/run/secure/peer.hex", send, receive}
}

func TestBlockedStreamArgumentsKeepShortAndSustainedCorporaDistinct(t *testing.T) {
	t.Setenv("ARDENTS_BLOCKED_WORKLOAD", "")
	if got := blockedStreamArguments(t, "client"); got[0] != "run-short" || got[5] != "512" || got[6] != "65536" {
		t.Fatalf("short arguments = %v", got)
	}
	t.Setenv("ARDENTS_BLOCKED_WORKLOAD", "sustained")
	t.Setenv("ARDENTS_BLOCKED_SEND_BYTES", "750000000")
	t.Setenv("ARDENTS_BLOCKED_RECEIVE_BYTES", "0")
	if got := blockedStreamArguments(t, "client"); got[0] != "run" || got[5] != "750000000" || got[6] != "0" {
		t.Fatalf("sustained arguments = %v", got)
	}
}
