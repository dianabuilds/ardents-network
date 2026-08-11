package nativecircuit

import (
	"context"
	"errors"
	"slices"
	"testing"
)

func TestParseDockerResourceLine(t *testing.T) {
	t.Parallel()
	sample, err := parseDockerResourceLine("abc123\tproject-user-1\t12.50%\t64.5MiB / 512MiB\t1.2MB / 3.4MB\t7")
	if err != nil {
		t.Fatal(err)
	}
	if sample.CPUCores != .125 || sample.RSSBytes != 67633152 || sample.RXBytes != 1_200_000 || sample.TXBytes != 3_400_000 || sample.PIDs != 7 {
		t.Fatalf("unexpected sample: %+v", sample)
	}
}

func TestParseDockerSizeRejectsUnknownUnit(t *testing.T) {
	t.Parallel()
	if _, err := parseDockerSize("1watts"); err == nil {
		t.Fatal("unknown Docker size unit was accepted")
	}
}

func TestDockerStatsUsesFullContainerIDs(t *testing.T) {
	arguments := dockerStatsArguments([]string{"full-b", "full-a"})
	if !slices.Contains(arguments, "--no-trunc") {
		t.Fatal("resource sampling may compare truncated Docker IDs with inspected full IDs")
	}
	if arguments[len(arguments)-2] != "full-a" || arguments[len(arguments)-1] != "full-b" {
		t.Fatalf("container IDs are not deterministic: %v", arguments)
	}
}

func TestCanceledSamplerCommandIsNotAResourceFailure(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if !resourceSamplingCanceled(ctx, errors.New("signal: killed")) {
		t.Fatal("command termination caused by sampler cancellation was treated as a resource failure")
	}
	if resourceSamplingCanceled(context.Background(), errors.New("docker stats failed")) {
		t.Fatal("an active sampler hid a real Docker stats failure")
	}
}
