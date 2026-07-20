package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	ardentsv1 "ardents/proto/ardents/v1"
	"google.golang.org/protobuf/proto"
)

func TestWatchSnapshotsHumanShowsRetryRecoveryAndUpdatedTruth(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	app := &app{
		cfg: Config{
			Output:   "human",
			Interval: time.Millisecond,
			Timeout:  50 * time.Millisecond,
		},
		stdout: &stdout,
		stderr: &stderr,
	}

	count := 0
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Millisecond)
	defer cancel()

	code := app.watchSnapshots(ctx, "diagnostics health", func(context.Context) (proto.Message, error) {
		count++
		switch count {
		case 1:
			return healthSummary("ready", "steady"), nil
		case 2, 3:
			return nil, errors.New("temporary local api failure")
		default:
			return healthSummary("degraded", "waiting for recovery"), nil
		}
	}, func(w io.Writer, msg proto.Message) {
		renderDiagnosticsHealthHuman(w, msg.(*ardentsv1.HealthSummaryResponse))
	})
	if code != 0 {
		t.Fatalf("code = %d, stderr = %s", code, stderr.String())
	}

	text := stdout.String()
	for _, want := range []string{
		"watch retry: diagnostics health attempt=1 error=temporary local api failure",
		"watch retry: diagnostics health attempt=2 error=temporary local api failure",
		"watch recovered: diagnostics health after=2",
		"diagnostics health",
		"state: degraded",
		"reason: waiting for recovery",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in output:\n%s", want, text)
		}
	}
}

func TestWatchSnapshotsBudgetExhaustionFailsExplicitly(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	app := &app{
		cfg: Config{
			Output:   "human",
			Interval: time.Millisecond,
			Timeout:  50 * time.Millisecond,
		},
		stdout: &stdout,
		stderr: &stderr,
	}

	count := 0
	code := app.watchSnapshots(context.Background(), "network status", func(context.Context) (proto.Message, error) {
		count++
		if count == 1 {
			return networkStatus("ready", "steady"), nil
		}
		return nil, errors.New("transport unavailable")
	}, func(w io.Writer, msg proto.Message) {
		renderNetworkStatusHuman(w, msg.(*ardentsv1.NetworkStatusResponse))
	})
	if code == 0 {
		t.Fatalf("code = %d, want non-zero", code)
	}
	if !strings.Contains(stdout.String(), "watch retry: network status attempt=5 error=transport unavailable") {
		t.Fatalf("stdout = %s", stdout.String())
	}
	if !strings.Contains(stderr.String(), "network status watch exhausted retry budget: transport unavailable") {
		t.Fatalf("stderr = %s", stderr.String())
	}
}

func TestWatchSnapshotsDegradedTruthDoesNotLookLikeTransportRetry(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	app := &app{
		cfg: Config{
			Output:   "human",
			Interval: time.Millisecond,
			Timeout:  50 * time.Millisecond,
		},
		stdout: &stdout,
		stderr: &stderr,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Millisecond)
	defer cancel()

	code := app.watchSnapshots(ctx, "network status", func(context.Context) (proto.Message, error) {
		return networkStatus("degraded", "reduced transport profile"), nil
	}, func(w io.Writer, msg proto.Message) {
		renderNetworkStatusHuman(w, msg.(*ardentsv1.NetworkStatusResponse))
	})
	if code != 0 {
		t.Fatalf("code = %d, stderr = %s", code, stderr.String())
	}

	text := stdout.String()
	if !strings.Contains(text, "state: degraded") {
		t.Fatalf("stdout = %s", text)
	}
	if !strings.Contains(text, "reason: reduced transport profile") {
		t.Fatalf("stdout = %s", text)
	}
	if strings.Contains(text, "watch retry:") {
		t.Fatalf("degraded snapshot rendered as retry episode:\n%s", text)
	}
}

func healthSummary(state, reason string) *ardentsv1.HealthSummaryResponse {
	return &ardentsv1.HealthSummaryResponse{
		Status: &ardentsv1.OperationStatus{State: state, Reason: reason},
		Health: &ardentsv1.HealthSnapshot{
			State:                  state,
			PrimaryReason:          &ardentsv1.ReasonSnapshot{Summary: reason},
			OperatorActionRequired: true,
		},
	}
}

func networkStatus(state, reason string) *ardentsv1.NetworkStatusResponse {
	return &ardentsv1.NetworkStatusResponse{
		Status: &ardentsv1.OperationStatus{State: state, Reason: reason},
		Network: &ardentsv1.NetworkStatusSnapshot{
			State:         state,
			Reason:        reason,
			Joined:        state != "failed",
			ActiveProfile: "standard",
			ActiveMode:    "steady",
		},
	}
}
