//go:build live

package network_test

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

const finalDirectBytes = uint32(256 << 20)

func runFinalDirectBaseline(t *testing.T, ctx context.Context, compose composeCall, toolImage,
	direction, fixtureRoot, pairID string, timeline time.Time,
) (float64, finalDirectRunEvidence) {
	t.Helper()
	sender, receiver := "direct-endpoint-sender", "direct-publisher-receiver"
	endpoint, publisher := sender, receiver
	if direction == "publisher-to-endpoint" {
		sender, receiver = "direct-publisher-sender", "direct-endpoint-receiver"
		endpoint, publisher = receiver, sender
	} else if direction != "endpoint-to-publisher" {
		t.Fatalf("unsupported final direct direction %q", direction)
	}
	startFile := filepath.Join(fixtureRoot, "sync", "direct", "direct.start")
	if err := os.Remove(startFile); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	startLiveContainer(t, ctx, compose, receiver)
	waitForJSON(t, ctx, compose, receiver, directReady)
	startLiveContainer(t, ctx, compose, sender)
	applyFinalDirectNetwork(t, ctx, compose, toolImage, endpoint, publisher)
	started := time.Now()
	writeLiveFile(t, startFile, []byte("start\n"))
	sent := waitForApplication(t, ctx, compose, sender)
	received := waitForApplication(t, ctx, compose, receiver)
	delivered := assertLiveDirectResults(t, sent, received)
	if output, err := compose(ctx, "rm", "-s", "-f", sender, receiver); err != nil {
		t.Fatalf("remove final direct pair: %v\n%s", err, output)
	}
	if err := os.Remove(startFile); err != nil {
		t.Fatal(err)
	}
	duration := sent.DurationMillis
	result := finalDirectRunEvidence{StartedOffsetMillis: uint64(started.Sub(timeline).Milliseconds()),
		FinishedOffsetMillis: uint64(time.Since(timeline).Milliseconds()),
		DurationMillis:       duration, DeliveredBytes: uint64(delivered),
		Digest: hex.EncodeToString(received.ReceivedDigest[:]), PairID: pairID,
		Complete: received.Terminal == "success" && duration >= 60_000}
	return float64(delivered) * 8 / (float64(duration) / 1_000) / 1e6, result
}

func directReady(line []byte) bool {
	var value struct{ Kind string }
	return json.Unmarshal(line, &value) == nil && value.Kind == "ready"
}
