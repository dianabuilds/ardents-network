package node_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestDomainMeasurementsReachTimelineWithoutPrivateArguments(t *testing.T) {
	binary := buildNodeCommand(t)
	run := func(enabled bool) ([]byte, []byte) {
		t.Helper()
		command := exec.CommandContext(t.Context(), binary, "enrollment", "verify", "PRIVATE-missing-bundle", strings.Repeat("0", 64))
		command.Env = append(os.Environ(), "ARDENTS_LOCAL_METRICS=", "ARDENTS_METRICS_OTLP_ENDPOINT=")
		if enabled {
			command.Env = append(command.Env, "ARDENTS_LOCAL_METRICS=1")
		}
		var out, diagnostic bytes.Buffer
		command.Stdout, command.Stderr = &out, &diagnostic
		var exit *exec.ExitError
		if err := command.Run(); !errors.As(err, &exit) || exit.ExitCode() != 1 {
			t.Fatalf("missing bundle behavior changed: %v, %q", err, &out)
		}
		return out.Bytes(), diagnostic.Bytes()
	}
	original, quiet := run(false)
	receipt, captured := run(true)
	if !bytes.Equal(original, receipt) || bytes.Contains(quiet, []byte("ardents-domain-measurements-v1")) {
		t.Fatal("default instrumentation or stdout changed")
	}
	found := false
	for _, line := range bytes.Split(captured, []byte("\n")) {
		var point struct {
			Schema string
			Values []struct {
				Name           string
				Available      bool
				Current, Count uint64
				Seconds        *float64
			} `json:"measurements"`
		}
		if json.Unmarshal(line, &point) != nil || point.Schema != "ardents-domain-measurements-v1" {
			continue
		}
		for _, value := range point.Values {
			if value.Name == "enrollment.verify" && value.Available && value.Current == 0 && value.Count == 1 && value.Seconds != nil && *value.Seconds >= 0 {
				found = true
			}
		}
		projected := projectCommandTimeline(t, binary, line)
		if !bytes.Contains(projected, []byte("enrollment.verify")) || !bytes.Contains(projected, []byte("seconds")) {
			t.Fatalf("internal measurement filtered: %q", projected)
		}
	}
	if !found || bytes.Contains(captured, []byte("PRIVATE")) {
		t.Fatalf("real internal result missing or argument exposed: %q", captured)
	}
}
