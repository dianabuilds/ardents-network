package main

import (
	"bytes"
	"context"
	"testing"
)

func TestForwardingTransitionRefusesForeignCommandsBeforeLaunch(t *testing.T) {
	t.Setenv("ARDENTS_FORWARDING_TRANSITION_CHILD", "")
	for _, args := range [][]string{
		{"supervise-transition"},
		{"supervise-transition", "--stop-after-record", "-1", "--timeout-ms", "1000", "--", "stop", "/absent/history", "/absent/binding"},
		{"supervise-transition", "--stop-after-record", "01", "--timeout-ms", "1000", "--", "stop", "/absent/history", "/absent/binding"},
		{"supervise-transition", "--stop-after-record", "0", "--timeout-ms", "0", "--", "stop", "/absent/history", "/absent/binding"},
		{"supervise-transition", "--stop-after-record", "0", "--timeout-ms", "120001", "--", "stop", "/absent/history", "/absent/binding"},
		{"supervise-transition", "--stop-after-record", "0", "--timeout-ms", "1000", "--", "/bin/sh", "-c", "exit 0"},
		{"supervise-transition", "--stop-after-record", "0", "--timeout-ms", "1000", "--", "start-installed", "/absent/binding"},
		{"supervise-transition", "--stop-after-record", "0", "--timeout-ms", "1000", "--", "inspect-inputs", "/absent/request"},
		{"supervise-transition", "--stop-after-record", "0", "--timeout-ms", "1000", "--", "supervise-transition"},
		{"supervise-transition", "--stop-after-record", "19", "--timeout-ms", "1000", "--", "stop", "/absent/history", "/absent/binding"},
	} {
		var output bytes.Buffer
		if err := runForwardingInstallation(context.Background(), args, &output); err == nil || output.Len() != 0 {
			t.Fatal("foreign or unbounded command admitted", args, err, output.String())
		}
	}
}
