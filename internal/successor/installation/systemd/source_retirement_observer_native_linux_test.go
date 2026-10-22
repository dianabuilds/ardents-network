//go:build network_signing_native

package systemd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

// This selected profile helper retains an independent read-only reference over
// the real command's stop/restart sequence. Collection must not erase the native
// exit oracle. Nothing reported here enters product admission or its journals.
func TestSourceRetirementObserverNative(t *testing.T) {
	marker, err := os.ReadFile("/etc/ardents-source-startup-native-stand")
	if os.Geteuid() != 0 || err != nil || string(marker) != "ardents-source-startup-native-v1\n" || os.Getenv("ARDENTS_SOURCE_STARTUP_READ_ONLY_OBSERVER") != "1" {
		t.Fatal("invalid selected private Source retirement observer environment", err)
	}
	control, err := OpenSourceControl(t.Context())
	closed := false
	if control != nil {
		defer func() {
			if !closed {
				if err := control.Close(); err != nil {
					t.Error("original observer close", err)
				}
			}
		}()
	}
	if err != nil {
		t.Fatal("original observer acquisition", err)
	}
	report := func() {
		t.Helper()
		facts, err := control.Observe()
		if err != nil || control.original.startAttempted || control.original.stopAttempted {
			t.Fatal("read-only original observation failed or performed a native effect", err)
		}
		raw, err := json.Marshal(facts)
		if err != nil || len(raw) > 64<<10 {
			t.Fatal("bounded original observation encoding", err)
		}
		if _, err = fmt.Fprintf(os.Stdout, "SOURCE_OBSERVATION %s\n", raw); err != nil {
			t.Fatal("original observation output", err)
		}
	}
	report()
	commands := bufio.NewScanner(os.Stdin)
	commands.Buffer(make([]byte, 64), 64)
	for count := 0; commands.Scan(); count++ {
		if count >= 32 {
			t.Fatal("finite original observer command bound exceeded")
		}
		switch commands.Text() {
		case "observe":
			report()
		case "finish":
			if err = control.Close(); err != nil {
				t.Fatal("original observer physical close", err)
			}
			closed = true
			if _, err = fmt.Fprintln(os.Stdout, "SOURCE_OBSERVER_CLOSED"); err != nil {
				t.Fatal("original observer completion output", err)
			}
			return
		default:
			t.Fatal("unknown original observer command")
		}
	}
	// Lost controller input is a failed profile, never a native join result.
	t.Fatal("original observer input ended before explicit finish", commands.Err())
}
