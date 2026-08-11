package nativecircuit

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"hash"
	"strconv"
	"time"
)

const (
	streamUpload       = "user-to-service"
	streamDownload     = "service-to-user"
	streamCorpusChunks = 256
)

type streamSpec struct {
	Direction string
	Seed      string
	Duration  time.Duration
}

type streamResult struct {
	Bytes   uint64
	Elapsed time.Duration
	Digest  [sha256.Size]byte
}

func validateStreamSpec(spec streamSpec, qualification bool) error {
	if spec.Direction != streamUpload && spec.Direction != streamDownload {
		return errors.New("stream direction is outside the fixed contract")
	}
	if spec.Seed == "" || spec.Duration <= 0 {
		return errors.New("stream seed or duration is missing")
	}
	if qualification && spec.Duration != 60*time.Second {
		return errors.New("qualification streams must run for exactly 60 seconds")
	}
	return nil
}

func sendTimedStream(ctx context.Context, connection interface{ Write([]byte) (int, error) }, spec streamSpec) (streamResult, error) {
	if err := validateStreamSpec(spec, false); err != nil {
		return streamResult{}, err
	}
	corpus := newStreamCorpus(spec.Seed)
	started := time.Now()
	deadline := started.Add(spec.Duration)
	digest := sha256.New()
	var result streamResult
	for counter := uint64(0); time.Now().Before(deadline); counter++ {
		select {
		case <-ctx.Done():
			return result, ctx.Err()
		default:
		}
		chunk := corpus.chunk(counter)
		if err := writeFrame(connection, frame{Type: frameProtectedData, Payload: chunk}); err != nil {
			return result, err
		}
		accumulateStream(&result, digest, chunk)
	}
	if err := writeFrame(connection, frame{Type: frameClose}); err != nil {
		return result, err
	}
	result.Elapsed = time.Since(started)
	copy(result.Digest[:], digest.Sum(nil))
	return result, nil
}

func receiveTimedStream(ctx context.Context, connection interface{ Read([]byte) (int, error) }, spec streamSpec) (streamResult, error) {
	if err := validateStreamSpec(spec, false); err != nil {
		return streamResult{}, err
	}
	corpus := newStreamCorpus(spec.Seed)
	started := time.Now()
	digest := sha256.New()
	var result streamResult
	for counter := uint64(0); ; counter++ {
		select {
		case <-ctx.Done():
			return result, ctx.Err()
		default:
		}
		value, err := readFrame(connection)
		if err != nil {
			return result, err
		}
		if value.Type == frameClose {
			result.Elapsed = time.Since(started)
			copy(result.Digest[:], digest.Sum(nil))
			return result, nil
		}
		if value.Type != frameProtectedData || !corpus.matches(value.Payload, counter) {
			return result, errors.New("timed Application stream contains unverified or out-of-order bytes")
		}
		accumulateStream(&result, digest, value.Payload)
	}
}

func accumulateStream(result *streamResult, digest hash.Hash, chunk []byte) {
	result.Bytes += uint64(len(chunk))
	_, _ = digest.Write(chunk)
}

type streamCorpus [][]byte

func newStreamCorpus(seed string) streamCorpus {
	result := make(streamCorpus, streamCorpusChunks)
	for index := range result {
		result[index] = seededStreamTemplate(seed, uint64(index))
	}
	return result
}

func (corpus streamCorpus) chunk(counter uint64) []byte {
	chunk := corpus[counter%uint64(len(corpus))]
	binary.BigEndian.PutUint64(chunk[len(chunk)-8:], counter)
	return chunk
}

func (corpus streamCorpus) matches(payload []byte, counter uint64) bool {
	if len(payload) != maximumApplicationPayload || binary.BigEndian.Uint64(payload[len(payload)-8:]) != counter {
		return false
	}
	template := corpus[counter%uint64(len(corpus))]
	return bytes.Equal(payload[:len(payload)-8], template[:len(template)-8])
}

func seededStreamChunk(seed string, counter uint64) []byte {
	chunk := seededStreamTemplate(seed, counter%streamCorpusChunks)
	binary.BigEndian.PutUint64(chunk[len(chunk)-8:], counter)
	return chunk
}

func seededStreamTemplate(seed string, corpusIndex uint64) []byte {
	result := make([]byte, maximumApplicationPayload)
	var block uint64
	for offset := 0; offset < len(result); block++ {
		value := sha256.Sum256([]byte(seed + ":" + strconv.FormatUint(corpusIndex, 10) + ":" + strconv.FormatUint(block, 10)))
		offset += copy(result[offset:], value[:])
	}
	return result
}

func encodeStreamReceipt(result streamResult) []byte {
	receipt := make([]byte, 8+sha256.Size)
	binary.BigEndian.PutUint64(receipt, result.Bytes)
	copy(receipt[8:], result.Digest[:])
	return receipt
}

func verifyStreamReceipt(payload []byte, expected streamResult) error {
	if len(payload) != 8+sha256.Size || binary.BigEndian.Uint64(payload[:8]) != expected.Bytes || !bytes.Equal(payload[8:], expected.Digest[:]) {
		return errors.New("timed Application stream receipt does not match verified bytes")
	}
	return nil
}
