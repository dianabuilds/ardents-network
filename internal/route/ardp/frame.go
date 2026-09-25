package ardp

import (
	"encoding/binary"
	"errors"
	"io"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route/terminal"
)

const (
	closedLaneMagic      = "ARDP"
	closedLaneGeneration = uint16(3)
	HeaderSize           = 16
	MaximumBodySize      = 16 << 10

	KindHello     = uint8(1)
	KindAdmit     = uint8(2)
	KindBootstrap = uint8(3)
	KindOpen      = uint8(4)
	KindAccept    = uint8(5)
	KindBytes     = uint8(6)
	KindCredit    = uint8(7)
	KindEOF       = uint8(8)
	KindClose     = uint8(9)
	KindOperation = uint8(10)
	KindResult    = uint8(11)
	KindKeepalive = uint8(12)
)

// ValidHeader checks a complete header before a caller allocates or charges
// its declared body. It does not admit the frame's kind or lane.
func ValidHeader(header []byte) bool {
	return len(header) == HeaderSize && string(header[:4]) == closedLaneMagic &&
		binary.BigEndian.Uint16(header[4:6]) == closedLaneGeneration && header[7] == 0 &&
		binary.BigEndian.Uint32(header[12:16]) <= MaximumBodySize
}

// Purpose is the one current recipient role purpose of a v3 channel.
// It is public routing context, never a Target, Name, or holder identity.
type Purpose uint8

const (
	PurposeIssuer       Purpose = 1
	PurposeName         Purpose = 2
	PurposeReachability Purpose = 3
	PurposeIntroduction Purpose = 4
	PurposeSubmission   Purpose = 5
	PurposeDataJoin     Purpose = 6
	PurposeForwarding   Purpose = 7
)

// Frame is one bounded generation-3 ARDP frame. Lane zero is used
// for channel admission and ordinary terminal operations; JOIN uses lane one.
// Callers enforce the authenticated allocation and activation of each lane.
type Frame struct {
	Kind uint8
	Lane uint32
	Body []byte
}

// Hello binds one channel to exact State and recipient duty facts.
type Hello struct {
	NetworkID, StateGeneration, StateDigest, ProfileDigest, RecipientNodeID [32]byte
	RecipientDutyGeneration                                                 uint64
	Purpose                                                                 Purpose
	ChannelNonce                                                            [32]byte
	Deadline                                                                time.Time
}

// EncodeFrame writes the one canonical ARDP frame representation.
func EncodeFrame(frame Frame) ([]byte, error) {
	if !ValidFrame(frame) {
		return nil, errors.New("closed lane frame is invalid")
	}
	raw := make([]byte, HeaderSize+len(frame.Body))
	copy(raw[:4], closedLaneMagic)
	binary.BigEndian.PutUint16(raw[4:6], closedLaneGeneration)
	raw[6], raw[7] = frame.Kind, 0
	binary.BigEndian.PutUint32(raw[8:12], frame.Lane)
	binary.BigEndian.PutUint32(raw[12:16], uint32(len(frame.Body)))
	copy(raw[HeaderSize:], frame.Body)
	return raw, nil
}

// ReadFrame reads one bounded frame without accepting a declared
// body before its header and maximum have been checked.
func ReadFrame(reader io.Reader) (Frame, error) {
	if reader == nil {
		return Frame{}, errors.New("closed lane reader is missing")
	}
	header := make([]byte, HeaderSize)
	if _, err := io.ReadFull(reader, header); err != nil {
		return Frame{}, err
	}
	if string(header[:4]) != closedLaneMagic || binary.BigEndian.Uint16(header[4:6]) != closedLaneGeneration || header[7] != 0 {
		return Frame{}, errors.New("closed lane header is invalid")
	}
	length := int(binary.BigEndian.Uint32(header[12:16]))
	if length < 0 || length > MaximumBodySize {
		return Frame{}, errors.New("closed lane length is invalid")
	}
	body := make([]byte, length)
	if _, err := io.ReadFull(reader, body); err != nil {
		return Frame{}, err
	}
	frame := Frame{Kind: header[6], Lane: binary.BigEndian.Uint32(header[8:12]), Body: body}
	if !ValidFrame(frame) {
		return Frame{}, errors.New("closed lane frame is invalid")
	}
	return frame, nil
}

// WriteFrame writes one complete validated frame.
func WriteFrame(writer io.Writer, frame Frame) error {
	raw, err := EncodeFrame(frame)
	if err != nil {
		return err
	}
	return writeAll(writer, raw)
}

func writeAll(writer io.Writer, value []byte) error {
	for len(value) != 0 {
		count, err := writer.Write(value)
		if err != nil {
			return err
		}
		if count <= 0 || count > len(value) {
			return io.ErrShortWrite
		}
		value = value[count:]
	}
	return nil
}

// EncodeHello returns the exact lane-zero HELLO body.
func EncodeHello(hello Hello) ([]byte, error) {
	if !ValidHello(hello) {
		return nil, errors.New("closed HELLO is invalid")
	}
	body := make([]byte, 0, 209)
	for _, value := range [][32]byte{hello.NetworkID, hello.StateGeneration, hello.StateDigest, hello.ProfileDigest, hello.RecipientNodeID} {
		body = append(body, value[:]...)
	}
	body = binary.BigEndian.AppendUint64(body, hello.RecipientDutyGeneration)
	body = append(body, byte(hello.Purpose))
	body = append(body, hello.ChannelNonce[:]...)
	return binary.BigEndian.AppendUint64(body, uint64(hello.Deadline.Unix())), nil
}

// DecodeHello parses only the exact generation-3 fixed HELLO body.
func DecodeHello(body []byte) (Hello, error) {
	if len(body) != 209 {
		return Hello{}, errors.New("closed HELLO length is invalid")
	}
	hello := Hello{}
	offset := 0
	for _, value := range []*[32]byte{&hello.NetworkID, &hello.StateGeneration, &hello.StateDigest, &hello.ProfileDigest, &hello.RecipientNodeID} {
		copy(value[:], body[offset:offset+32])
		offset += 32
	}
	hello.RecipientDutyGeneration = binary.BigEndian.Uint64(body[offset : offset+8])
	offset += 8
	hello.Purpose = Purpose(body[offset])
	offset++
	copy(hello.ChannelNonce[:], body[offset:offset+32])
	offset += 32
	hello.Deadline = time.Unix(int64(binary.BigEndian.Uint64(body[offset:offset+8])), 0).UTC()
	if !ValidHello(hello) {
		return Hello{}, errors.New("closed HELLO facts are invalid")
	}
	return hello, nil
}

// DecodeBootstrap accepts only public-evidence or issuer bootstrap.
func DecodeBootstrap(body []byte) (issuer bool, err error) {
	if len(body) != 1 || body[0] < 1 || body[0] > 2 {
		return false, errors.New("closed bootstrap operation is invalid")
	}
	return body[0] == 2, nil
}

// AcceptFrame returns the exact fixed admission acknowledgement. A
// refusal has zero credit and exposes no detailed path-or-policy oracle.
func AcceptFrame(status uint8, credit uint32) (Frame, error) {
	if status > 4 || status != 0 && credit != 0 {
		return Frame{}, errors.New("closed acceptance is invalid")
	}
	body := make([]byte, 5)
	body[0] = status
	binary.BigEndian.PutUint32(body[1:], credit)
	return Frame{Kind: KindAccept, Lane: 0, Body: body}, nil
}

// DecodeAcceptFrame accepts only a lane-zero fixed acknowledgement.
func DecodeAcceptFrame(frame Frame) (status uint8, credit uint32, err error) {
	if frame.Kind != KindAccept || frame.Lane != 0 || len(frame.Body) != 5 || frame.Body[0] > 4 || frame.Body[0] != 0 && binary.BigEndian.Uint32(frame.Body[1:]) != 0 {
		return 0, 0, errors.New("closed acceptance is invalid")
	}
	return frame.Body[0], binary.BigEndian.Uint32(frame.Body[1:]), nil
}

// ValidFrame checks a complete frame's kind, lane, and exact body size.
func ValidFrame(frame Frame) bool {
	if frame.Kind == 0 || len(frame.Body) > MaximumBodySize {
		return false
	}
	if frame.Kind == KindHello {
		return frame.Lane == 0 && len(frame.Body) == 209
	}
	if frame.Kind == KindAdmit {
		return len(frame.Body) == 355
	}
	if frame.Kind == KindBootstrap {
		return frame.Lane == 0 && len(frame.Body) == 1
	}
	if frame.Kind == KindOpen {
		return frame.Lane != 0 && (len(frame.Body) == 49 || len(frame.Body) == 50)
	}
	if frame.Kind == KindAccept {
		return frame.Lane == 0 && len(frame.Body) == 5
	}
	if frame.Kind == KindBytes {
		return frame.Lane != 0 && len(frame.Body) >= 1
	}
	if frame.Kind == KindCredit {
		return frame.Lane != 0 && len(frame.Body) == 4 && binary.BigEndian.Uint32(frame.Body) != 0
	}
	if frame.Kind == KindEOF {
		return frame.Lane != 0 && len(frame.Body) == 0
	}
	if frame.Kind == KindClose {
		return frame.Lane != 0 && len(frame.Body) == 1 && frame.Body[0] <= 6
	}
	if frame.Kind == KindOperation {
		return len(frame.Body) == terminal.BodySize || len(frame.Body) == terminal.SmallBodySize
	}
	if frame.Kind == KindResult {
		return len(frame.Body) == terminal.BodySize
	}
	if frame.Kind == KindKeepalive {
		return frame.Lane == 0 && len(frame.Body) == 0
	}
	return false
}

// ValidHello checks the fixed nonzero and canonical HELLO facts.
func ValidHello(hello Hello) bool {
	return hello.NetworkID != [32]byte{} && hello.StateGeneration != [32]byte{} && hello.StateDigest != [32]byte{} &&
		hello.ProfileDigest != [32]byte{} && hello.RecipientNodeID != [32]byte{} && hello.RecipientDutyGeneration != 0 &&
		ValidPurpose(hello.Purpose) && hello.ChannelNonce != [32]byte{} && !hello.Deadline.IsZero() &&
		hello.Deadline == hello.Deadline.UTC().Truncate(time.Second)
}

// ValidPurpose checks the selected generation-3 purpose vocabulary.
func ValidPurpose(purpose Purpose) bool {
	return purpose >= PurposeIssuer && purpose <= PurposeForwarding
}
