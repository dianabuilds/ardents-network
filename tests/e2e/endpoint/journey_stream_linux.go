//go:build linux && document_worker_installed

package endpoint

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"hash"
	"io"
	"math/rand/v2"
	"time"
)

// JourneyStream specifies deterministic binary input generated on a participant.
type JourneyStream struct {
	Bytes          int64
	Seed           uint64
	BytesPerSecond int64
}

func (input JourneyStream) Reader() *io.LimitedReader {
	var seed [32]byte
	binary.BigEndian.PutUint64(seed[:], input.Seed)
	return &io.LimitedReader{R: rand.NewChaCha8(seed), N: input.Bytes}
}

// PacedReader generates the same bytes while bounding its production rate.
func (input JourneyStream) PacedReader(ctx context.Context) io.Reader {
	source := input.Reader()
	if input.BytesPerSecond == 0 {
		return source
	}
	return &journeyPacedReader{ctx: ctx, source: source, rate: input.BytesPerSecond}
}

type journeyPacedReader struct {
	ctx    context.Context
	source *io.LimitedReader
	rate   int64
}

func (input *journeyPacedReader) Read(dst []byte) (int, error) {
	size := min(int64(len(dst)), input.source.N, 32<<10)
	if size == 0 {
		return input.source.Read(dst)
	}
	timer := time.NewTimer(time.Duration(size) * time.Second / time.Duration(input.rate))
	defer timer.Stop()
	select {
	case <-input.ctx.Done():
		return 0, input.ctx.Err()
	case <-timer.C:
		return input.source.Read(dst[:size])
	}
}

// JourneyStreamProof summarizes bytes received through the public CLI.
type JourneyStreamProof struct {
	Bytes  int64
	SHA256 [sha256.Size]byte
}

// JourneyStreamOutput records an output proof with constant storage.
type JourneyStreamOutput struct {
	bytes  int64
	digest hash.Hash
}

func NewJourneyStreamOutput() *JourneyStreamOutput {
	return &JourneyStreamOutput{digest: sha256.New()}
}

func (output *JourneyStreamOutput) Write(data []byte) (int, error) {
	size, err := output.digest.Write(data)
	output.bytes += int64(size)
	return size, err
}

func (output *JourneyStreamOutput) Proof() JourneyStreamProof {
	proof := JourneyStreamProof{Bytes: output.bytes}
	_ = output.digest.Sum(proof.SHA256[:0])
	return proof
}
