package recoverysmoke

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"time"

	"github.com/dianabuilds/ardents-network/internal/qualification/byteio"
)

const replacementManifestSchema = "ardents-h3-replacement-cell-manifest-v1"

type replacementCellManifest struct {
	Schema, Direction, Mode, FaultFamily               string
	Seed                                               [32]byte
	Bytes, ChunkBytes, CanaryBytes                     uint32
	ChunkDelayNanos, SetupDeadlineNanos, LifetimeNanos int64
	FailureRoles                                       []string
	FaultOffsets                                       []uint32
	Digest                                             string
}

func prepareReplacementManifest(root, direction, mode string, seed [32]byte, failures []string,
	offsets []uint32, lifetime, delay string) (replacementCellManifest, error) {
	lifetimeValue, err := time.ParseDuration(lifetime)
	if err != nil {
		return replacementCellManifest{}, fmt.Errorf("parse replacement lifetime: %w", err)
	}
	delayValue, err := time.ParseDuration(delay)
	if err != nil {
		return replacementCellManifest{}, fmt.Errorf("parse replacement pacing: %w", err)
	}
	deadlineValue, err := time.ParseDuration(replacementSetupDeadline)
	if err != nil {
		return replacementCellManifest{}, fmt.Errorf("parse replacement setup deadline: %w", err)
	}
	value := replacementCellManifest{Schema: replacementManifestSchema, Direction: direction, Mode: mode,
		FaultFamily: "route-process", Seed: seed, Bytes: 4 << 20, ChunkBytes: 16_381, CanaryBytes: 32,
		ChunkDelayNanos: delayValue.Nanoseconds(), SetupDeadlineNanos: deadlineValue.Nanoseconds(),
		LifetimeNanos: lifetimeValue.Nanoseconds(),
		FailureRoles:  append([]string(nil), failures...), FaultOffsets: append([]uint32(nil), offsets...)}
	value.Digest = replacementManifestDigest(value)
	path := filepath.Join(root, "replacement-cell-manifest.json")
	if err := byteio.WriteJSON(path, value, 64<<10); err != nil {
		return replacementCellManifest{}, fmt.Errorf("write replacement cell manifest: %w", err)
	}
	return value, nil
}

func replacementManifestDigest(value replacementCellManifest) string {
	hash := sha256.New()
	_, _ = hash.Write([]byte(replacementManifestSchema + "\x00"))
	writeManifestText(hash.Write, value.Direction)
	writeManifestText(hash.Write, value.Mode)
	writeManifestText(hash.Write, value.FaultFamily)
	_, _ = hash.Write(value.Seed[:])
	var numeric [36]byte
	binary.BigEndian.PutUint32(numeric[0:4], value.Bytes)
	binary.BigEndian.PutUint32(numeric[4:8], value.ChunkBytes)
	binary.BigEndian.PutUint32(numeric[8:12], value.CanaryBytes)
	binary.BigEndian.PutUint64(numeric[12:20], uint64(value.ChunkDelayNanos))
	binary.BigEndian.PutUint64(numeric[20:28], uint64(value.SetupDeadlineNanos))
	binary.BigEndian.PutUint64(numeric[28:36], uint64(value.LifetimeNanos))
	_, _ = hash.Write(numeric[:])
	writeManifestList(hash.Write, value.FailureRoles, value.FaultOffsets)
	return hex.EncodeToString(hash.Sum(nil))
}

func writeManifestText(write func([]byte) (int, error), value string) {
	var size [4]byte
	binary.BigEndian.PutUint32(size[:], uint32(len(value)))
	_, _ = write(size[:])
	_, _ = write([]byte(value))
}

func writeManifestList(write func([]byte) (int, error), roles []string, offsets []uint32) {
	var size [4]byte
	binary.BigEndian.PutUint32(size[:], uint32(len(roles)))
	_, _ = write(size[:])
	for _, role := range roles {
		writeManifestText(write, role)
	}
	binary.BigEndian.PutUint32(size[:], uint32(len(offsets)))
	_, _ = write(size[:])
	for _, offset := range offsets {
		binary.BigEndian.PutUint32(size[:], offset)
		_, _ = write(size[:])
	}
}
