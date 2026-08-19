//go:build linux && live

package network_test

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net"
	"os"
	"strconv"
	"testing"
	"time"
)

type blockedPressureInjection struct {
	Schema                string `json:"schema"`
	Ordinal               uint16 `json:"ordinal"`
	CorpusSHA256          string `json:"corpus_sha256"`
	ScheduledOffsetMillis uint64 `json:"scheduled_offset_millis"`
	StartedOffsetMillis   uint64 `json:"started_offset_millis"`
	Bytes                 uint16 `json:"bytes"`
	Connected             bool   `json:"connected"`
}

func runBlockedPressure(t *testing.T) {
	t.Helper()
	count, err := strconv.Atoi(os.Getenv("ARDENTS_PRESSURE_CONNECTIONS"))
	if err != nil || count < 1 || count > 23 {
		t.Fatalf("invalid partial-handshake count %q", os.Getenv("ARDENTS_PRESSURE_CONNECTIONS"))
	}
	connections := make([]net.Conn, 0, count)
	defer func() {
		for _, connection := range connections {
			_ = connection.Close()
		}
	}()
	seed, readErr := os.ReadFile("/run/input/corpus-seed.bin")
	if readErr != nil || len(seed) != 32 {
		t.Fatalf("partial-handshake corpus seed is invalid: %v", readErr)
	}
	evidence, err := os.OpenFile("/run/evidence/pressure-injection.jsonl", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	encoder := json.NewEncoder(evidence)
	origin := time.Now()
	originOffset := blockedTimelineOffsetMillis(t)
	for index := range count {
		at := origin.Add(time.Duration(index) * 500 * time.Millisecond)
		if wait := time.Until(at); wait > 0 {
			time.Sleep(wait)
		}
		prefix := blockedPressureCorpus(seed, index)
		started := time.Now()
		connection, dialErr := net.DialTimeout("tcp4", "203.0.113.8:8480", time.Second)
		if dialErr != nil {
			t.Fatal(dialErr)
		}
		if _, writeErr := connection.Write(prefix); writeErr != nil {
			_ = connection.Close()
			t.Fatal(writeErr)
		}
		connections = append(connections, connection)
		digest := sha256.Sum256(prefix)
		observation := blockedPressureInjection{Schema: "ardents-h3-pressure-injection-v1",
			Ordinal: uint16(index), CorpusSHA256: hex.EncodeToString(digest[:]),
			ScheduledOffsetMillis: originOffset + uint64(index*500),
			StartedOffsetMillis:   originOffset + uint64(max(int64(0), started.Sub(origin).Milliseconds())),
			Bytes:                 uint16(len(prefix)), Connected: true}
		if encodeErr := encoder.Encode(observation); encodeErr != nil {
			t.Fatal(encodeErr)
		}
	}
	if err := errors.Join(evidence.Sync(), evidence.Close()); err != nil {
		t.Fatal(err)
	}
	writeBlockedSignal(t, "/run/evidence/pressure-ready")
	waitBlockedFile(t, "/run/evidence/pressure-release", 4*time.Minute)
	var closeErr error
	for _, connection := range connections {
		closeErr = errors.Join(closeErr, connection.Close())
	}
	connections = nil
	if closeErr != nil {
		t.Fatal(closeErr)
	}
	writeBlockedSignal(t, "/run/evidence/pressure-closed")
}

func blockedPressureCorpus(seed []byte, index int) []byte {
	prefix := make([]byte, 128)
	prefix[0], prefix[1], prefix[2], prefix[3], prefix[4] = 22, 3, 3, 0x10, 0
	ordinal := make([]byte, 8)
	binary.BigEndian.PutUint64(ordinal, uint64(index))
	for offset, blockIndex := 5, byte(0); offset < len(prefix); blockIndex++ {
		input := append(append(append([]byte(nil), seed...), ordinal...), blockIndex)
		block := sha256.Sum256(input)
		offset += copy(prefix[offset:], block[:])
	}
	return prefix
}
