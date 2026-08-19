//go:build linux && live

package network_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func runBlockedCarrierCollector(t *testing.T) {
	t.Helper()
	root := blockedSync()
	output, err := os.OpenFile(filepath.Join(root, "carrier.jsonl"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer output.Close()
	writeLiveFile(t, filepath.Join(root, "carrier-ready"), []byte("ready\n"))
	waitBlockedFile(t, filepath.Join(root, "carrier-start"), 2*time.Minute)
	started, encoder := time.Now(), json.NewEncoder(output)
	ordinal := uint16(0)
	emit := func(boundary string) {
		value := blockedCarrierSample{Schema: "ardents-h3-carrier-counter-v1", Ordinal: ordinal,
			OffsetMillis: blockedTimelineOffsetMillis(t), Bytes: readBlockedCarrierBytes(t),
			Boundary: boundary}
		if err := encoder.Encode(value); err != nil {
			t.Fatal(err)
		}
		if err := output.Sync(); err != nil {
			t.Fatal(err)
		}
		ordinal++
	}
	emit("before")
	writeLiveFile(t, filepath.Join(root, "carrier-started"), []byte("started\n"))
	next := started.Add(time.Second)
	for {
		if fileExists(filepath.Join(root, "carrier-stop")) {
			emit("after")
			return
		}
		if !time.Now().Before(next) {
			emit("")
			next = next.Add(time.Second)
			continue
		}
		time.Sleep(25 * time.Millisecond)
	}
}

func readBlockedCarrierBytes(t *testing.T) uint64 {
	t.Helper()
	raw, err := os.ReadFile("/proc/net/dev")
	if err != nil {
		t.Fatal(err)
	}
	var total uint64
	for _, line := range strings.Split(string(raw), "\n") {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 || strings.TrimSpace(parts[0]) == "lo" {
			continue
		}
		fields := strings.Fields(parts[1])
		if len(fields) != 16 {
			t.Fatalf("invalid network counter line %q", line)
		}
		received, receiveErr := strconv.ParseUint(fields[0], 10, 64)
		sent, sendErr := strconv.ParseUint(fields[8], 10, 64)
		if receiveErr != nil || sendErr != nil {
			t.Fatal(errors.Join(receiveErr, sendErr))
		}
		total += received + sent
	}
	return total
}
