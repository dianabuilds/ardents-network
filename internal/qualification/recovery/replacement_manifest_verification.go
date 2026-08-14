package recovery

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"strings"
	"time"
)

const replacementManifestSchema = "ardents-h3-replacement-cell-manifest-v1"

func verifyReplacementManifest(cell replacementCell) Result {
	if len(cell.FailureRoles) != len(cell.FaultOffsets) || len(cell.FaultOffsets) != len(cell.Events) {
		return invalid("S4.2 workload and fault manifest cardinality is invalid")
	}
	expectedRoles := []string{strings.TrimPrefix(cell.Mode, "isolated-")}
	expectedOffsets := []uint32{17 * 16_381}
	expectedDelay, expectedDeadline := int64(20*time.Millisecond), int64(2*time.Second)
	expectedLifetime := int64(time.Minute)
	if cell.Mode == "sequential-three" {
		expectedRoles = []string{"initiator", "rendezvous", "responder"}
		expectedOffsets = []uint32{64 * 16_381, 128 * 16_381, 192 * 16_381}
		expectedDelay, expectedLifetime = int64(2350*time.Millisecond), int64(12*time.Minute)
	} else if !strings.HasPrefix(cell.Mode, "isolated-") || !replacementRole(expectedRoles[0]) {
		return invalid("S4.2 workload and fault manifest mode is invalid")
	}
	if cell.FaultFamily != "route-process" || cell.ChunkBytes != 16_381 || cell.CanaryBytes != 32 ||
		cell.ChunkDelayNanos != expectedDelay || cell.SetupDeadlineNanos != expectedDeadline ||
		cell.LifetimeNanos != expectedLifetime ||
		!equalManifestRoles(cell.FailureRoles, expectedRoles) ||
		!equalManifestOffsets(cell.FaultOffsets, expectedOffsets) ||
		cell.CellManifestDigest != replacementManifestDigest(cell) {
		return invalid("S4.2 workload and fault manifest commitment is invalid")
	}
	return Result{Verdict: "pass"}
}

func replacementManifestDigest(cell replacementCell) string {
	hash := sha256.New()
	_, _ = hash.Write([]byte(replacementManifestSchema + "\x00"))
	writeReplacementManifestText(hash.Write, cell.Direction)
	writeReplacementManifestText(hash.Write, cell.Mode)
	writeReplacementManifestText(hash.Write, cell.FaultFamily)
	_, _ = hash.Write(cell.Seed[:])
	var numeric [36]byte
	binary.BigEndian.PutUint32(numeric[0:4], cell.Bytes)
	binary.BigEndian.PutUint32(numeric[4:8], cell.ChunkBytes)
	binary.BigEndian.PutUint32(numeric[8:12], cell.CanaryBytes)
	binary.BigEndian.PutUint64(numeric[12:20], uint64(cell.ChunkDelayNanos))
	binary.BigEndian.PutUint64(numeric[20:28], uint64(cell.SetupDeadlineNanos))
	binary.BigEndian.PutUint64(numeric[28:36], uint64(cell.LifetimeNanos))
	_, _ = hash.Write(numeric[:])
	writeReplacementManifestList(hash.Write, cell.FailureRoles, cell.FaultOffsets)
	return hex.EncodeToString(hash.Sum(nil))
}

func writeReplacementManifestText(write func([]byte) (int, error), value string) {
	var size [4]byte
	binary.BigEndian.PutUint32(size[:], uint32(len(value)))
	_, _ = write(size[:])
	_, _ = write([]byte(value))
}

func writeReplacementManifestList(write func([]byte) (int, error), roles []string, offsets []uint32) {
	var size [4]byte
	binary.BigEndian.PutUint32(size[:], uint32(len(roles)))
	_, _ = write(size[:])
	for _, role := range roles {
		writeReplacementManifestText(write, role)
	}
	binary.BigEndian.PutUint32(size[:], uint32(len(offsets)))
	_, _ = write(size[:])
	for _, offset := range offsets {
		binary.BigEndian.PutUint32(size[:], offset)
		_, _ = write(size[:])
	}
}

func equalManifestRoles(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func equalManifestOffsets(left, right []uint32) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
