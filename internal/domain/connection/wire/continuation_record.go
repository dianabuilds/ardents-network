package wire

import (
	"encoding/binary"
	"errors"
)

// SegmentBoundary uses transfer-wide positions, measured in each named
// direction. Bit 0 marks Publisher input EOF and bit 1 marks Reader input EOF.
type SegmentBoundary struct {
	Next, ToPublisher, ToReader uint64
	EOF                         byte
}

func (boundary SegmentBoundary) Valid() bool {
	return boundary.Next > 1 && boundary.EOF&^byte(3) == 0
}

// ContinuationKind identifies one endpoint control inside authenticated Service
// TLS. None of these records conveys Application data or an admission grant.
type ContinuationKind byte

const (
	DrainSegment      ContinuationKind = 7
	ProposeBoundary   ContinuationKind = 8
	OfferContinuation ContinuationKind = 9
	ConfirmBoundary   ContinuationKind = 10
	ResumeTransfer    ContinuationKind = 11
	AcceptResume      ContinuationKind = 12
	AcceptBoundary    ContinuationKind = 13
)

// Continuation carries exact segment controls. Its fields become usable only
// after the owner checks their direction, authentication, lifetime and phase.
type Continuation struct {
	Kind                 ContinuationKind
	AttachmentGeneration uint64
	DrainOffset          uint64
	Boundary             SegmentBoundary
	Ticket               [32]byte
	LogicalContext       [32]byte
}

const (
	attachmentGenerationSize = 8
	drainPayloadSize         = attachmentGenerationSize + 8
	boundaryPayloadSize      = attachmentGenerationSize + 8*3 + 1
	offerPayloadSize         = boundaryPayloadSize + 32
	resumePayloadSize        = offerPayloadSize + 32
)

var errContinuation = errors.New("native Service continuation record is invalid")

func continuationPayloadSize(kind ContinuationKind) int {
	switch kind {
	case DrainSegment:
		return drainPayloadSize
	case ProposeBoundary:
		return boundaryPayloadSize
	case OfferContinuation, ConfirmBoundary, AcceptBoundary:
		return offerPayloadSize
	case ResumeTransfer, AcceptResume:
		return resumePayloadSize
	default:
		return 0
	}
}

func (record Continuation) payloadSize() (int, error) {
	size := continuationPayloadSize(record.Kind)
	if size == 0 || record.AttachmentGeneration == 0 {
		return 0, errContinuation
	}
	if record.Kind == DrainSegment && record.Boundary != (SegmentBoundary{}) ||
		record.Kind != DrainSegment && (record.DrainOffset != 0 || !record.Boundary.Valid()) {
		return 0, errContinuation
	}
	if (size >= offerPayloadSize) != (record.Ticket != [32]byte{}) ||
		(size == resumePayloadSize) != (record.LogicalContext != [32]byte{}) {
		return 0, errContinuation
	}
	return size, nil
}

func (record Continuation) encodePayload(dst []byte) {
	binary.BigEndian.PutUint64(dst[:attachmentGenerationSize], record.AttachmentGeneration)
	if record.Kind == DrainSegment {
		binary.BigEndian.PutUint64(dst[attachmentGenerationSize:], record.DrainOffset)
		return
	}
	if len(dst) >= boundaryPayloadSize {
		binary.BigEndian.PutUint64(dst[8:16], record.Boundary.Next)
		binary.BigEndian.PutUint64(dst[16:24], record.Boundary.ToPublisher)
		binary.BigEndian.PutUint64(dst[24:32], record.Boundary.ToReader)
		dst[32] = record.Boundary.EOF
	}
	if len(dst) >= offerPayloadSize {
		copy(dst[boundaryPayloadSize:offerPayloadSize], record.Ticket[:])
	}
	if len(dst) == resumePayloadSize {
		copy(dst[offerPayloadSize:], record.LogicalContext[:])
	}
}

func decodeContinuation(kind ContinuationKind, payload []byte) (Record, error) {
	size := continuationPayloadSize(kind)
	if size == 0 || len(payload) != size {
		return Record{}, errContinuation
	}
	record := &Continuation{
		Kind: kind, AttachmentGeneration: binary.BigEndian.Uint64(payload[:attachmentGenerationSize]),
	}
	if kind == DrainSegment {
		record.DrainOffset = binary.BigEndian.Uint64(payload[attachmentGenerationSize:])
	} else {
		record.Boundary = SegmentBoundary{
			Next:        binary.BigEndian.Uint64(payload[8:16]),
			ToPublisher: binary.BigEndian.Uint64(payload[16:24]),
			ToReader:    binary.BigEndian.Uint64(payload[24:32]),
			EOF:         payload[32],
		}
	}
	if size >= offerPayloadSize {
		copy(record.Ticket[:], payload[boundaryPayloadSize:offerPayloadSize])
	}
	if size == resumePayloadSize {
		copy(record.LogicalContext[:], payload[offerPayloadSize:])
	}
	if _, err := record.payloadSize(); err != nil {
		return Record{}, err
	}
	return Record{Continuation: record}, nil
}
