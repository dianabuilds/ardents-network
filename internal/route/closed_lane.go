package route

import (
	"encoding/binary"
	"errors"
	"io"
	"time"
)

const (
	closedLaneMagic      = "ARDP"
	closedLaneGeneration = uint16(3)
	closedLaneHeaderSize = 16
	closedLaneMaximum    = 16 << 10

	closedFrameHello     = uint8(1)
	closedFrameAdmit     = uint8(2)
	closedFrameBootstrap = uint8(3)
	closedFrameOpen      = uint8(4)
	closedFrameAccept    = uint8(5)
	closedFrameBytes     = uint8(6)
	closedFrameCredit    = uint8(7)
	closedFrameEOF       = uint8(8)
	closedFrameClose     = uint8(9)
	closedFrameOperation = uint8(10)
	closedFrameResult    = uint8(11)
	closedFrameKeepalive = uint8(12)
)

// ClosedPurpose is the one current recipient role purpose of a v3 channel.
// It is public routing context, never a Target, Name, or holder identity.
type ClosedPurpose uint8

const (
	ClosedPurposeIssuer       ClosedPurpose = 1
	ClosedPurposeName         ClosedPurpose = 2
	ClosedPurposeReachability ClosedPurpose = 3
	ClosedPurposeIntroduction ClosedPurpose = 4
	ClosedPurposeSubmission   ClosedPurpose = 5
	ClosedPurposeDataJoin     ClosedPurpose = 6
	ClosedPurposeForwarding   ClosedPurpose = 7
)

// ClosedLaneFrame is one bounded generation-3 ARDP frame. Lane zero is used
// only for channel admission; lane ownership is enforced by the caller.
type ClosedLaneFrame struct {
	Kind uint8
	Lane uint32
	Body []byte
}

// ClosedHello binds one channel to exact State and recipient duty facts.
type ClosedHello struct {
	NetworkID, StateGeneration, StateDigest, ProfileDigest, RecipientNodeID [32]byte
	RecipientDutyGeneration                                                 uint64
	Purpose                                                                 ClosedPurpose
	ChannelNonce                                                            [32]byte
	Deadline                                                                time.Time
}

// EncodeClosedLaneFrame writes the one canonical ARDP frame representation.
func EncodeClosedLaneFrame(frame ClosedLaneFrame) ([]byte, error) {
	if !validClosedFrame(frame) {
		return nil, errors.New("closed lane frame is invalid")
	}
	raw := make([]byte, closedLaneHeaderSize+len(frame.Body))
	copy(raw[:4], closedLaneMagic)
	binary.BigEndian.PutUint16(raw[4:6], closedLaneGeneration)
	raw[6], raw[7] = frame.Kind, 0
	binary.BigEndian.PutUint32(raw[8:12], frame.Lane)
	binary.BigEndian.PutUint32(raw[12:16], uint32(len(frame.Body)))
	copy(raw[closedLaneHeaderSize:], frame.Body)
	return raw, nil
}

// DecodeClosedLaneFrame rejects unsupported header facts before allocating a
// body. It accepts exactly one whole frame and no trailing carrier bytes.
func DecodeClosedLaneFrame(raw []byte) (ClosedLaneFrame, error) {
	if len(raw) < closedLaneHeaderSize || string(raw[:4]) != closedLaneMagic || binary.BigEndian.Uint16(raw[4:6]) != closedLaneGeneration || raw[7] != 0 {
		return ClosedLaneFrame{}, errors.New("closed lane header is invalid")
	}
	length := int(binary.BigEndian.Uint32(raw[12:16]))
	if length < 0 || length > closedLaneMaximum || len(raw) != closedLaneHeaderSize+length {
		return ClosedLaneFrame{}, errors.New("closed lane length is invalid")
	}
	frame := ClosedLaneFrame{Kind: raw[6], Lane: binary.BigEndian.Uint32(raw[8:12]), Body: append([]byte(nil), raw[closedLaneHeaderSize:]...)}
	if !validClosedFrame(frame) {
		return ClosedLaneFrame{}, errors.New("closed lane frame is invalid")
	}
	return frame, nil
}

// ReadClosedLaneFrame reads one bounded frame without accepting a declared
// body before its header and maximum have been checked.
func ReadClosedLaneFrame(reader io.Reader) (ClosedLaneFrame, error) {
	if reader == nil {
		return ClosedLaneFrame{}, errors.New("closed lane reader is missing")
	}
	header := make([]byte, closedLaneHeaderSize)
	if _, err := io.ReadFull(reader, header); err != nil {
		return ClosedLaneFrame{}, err
	}
	if string(header[:4]) != closedLaneMagic || binary.BigEndian.Uint16(header[4:6]) != closedLaneGeneration || header[7] != 0 {
		return ClosedLaneFrame{}, errors.New("closed lane header is invalid")
	}
	length := int(binary.BigEndian.Uint32(header[12:16]))
	if length < 0 || length > closedLaneMaximum {
		return ClosedLaneFrame{}, errors.New("closed lane length is invalid")
	}
	body := make([]byte, length)
	if _, err := io.ReadFull(reader, body); err != nil {
		return ClosedLaneFrame{}, err
	}
	frame := ClosedLaneFrame{Kind: header[6], Lane: binary.BigEndian.Uint32(header[8:12]), Body: body}
	if !validClosedFrame(frame) {
		return ClosedLaneFrame{}, errors.New("closed lane frame is invalid")
	}
	return frame, nil
}

// WriteClosedLaneFrame writes one complete validated frame.
func WriteClosedLaneFrame(writer io.Writer, frame ClosedLaneFrame) error {
	raw, err := EncodeClosedLaneFrame(frame)
	if err != nil {
		return err
	}
	return writeAll(writer, raw)
}

// EncodeClosedHello returns the exact lane-zero HELLO body.
func EncodeClosedHello(hello ClosedHello) ([]byte, error) {
	if !validClosedHello(hello) {
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

// DecodeClosedHello parses only the exact generation-3 fixed HELLO body.
func DecodeClosedHello(body []byte) (ClosedHello, error) {
	if len(body) != 209 {
		return ClosedHello{}, errors.New("closed HELLO length is invalid")
	}
	hello := ClosedHello{}
	offset := 0
	for _, value := range []*[32]byte{&hello.NetworkID, &hello.StateGeneration, &hello.StateDigest, &hello.ProfileDigest, &hello.RecipientNodeID} {
		copy(value[:], body[offset:offset+32])
		offset += 32
	}
	hello.RecipientDutyGeneration = binary.BigEndian.Uint64(body[offset : offset+8])
	offset += 8
	hello.Purpose = ClosedPurpose(body[offset])
	offset++
	copy(hello.ChannelNonce[:], body[offset:offset+32])
	offset += 32
	hello.Deadline = time.Unix(int64(binary.BigEndian.Uint64(body[offset:offset+8])), 0).UTC()
	if !validClosedHello(hello) {
		return ClosedHello{}, errors.New("closed HELLO facts are invalid")
	}
	return hello, nil
}

// EncodeClosedBootstrap returns the lane-zero bootstrap operation body.
func EncodeClosedBootstrap(issuer bool) []byte {
	if issuer {
		return []byte{2}
	}
	return []byte{1}
}

// DecodeClosedBootstrap accepts only public-evidence or issuer bootstrap.
func DecodeClosedBootstrap(body []byte) (issuer bool, err error) {
	if len(body) != 1 || body[0] < 1 || body[0] > 2 {
		return false, errors.New("closed bootstrap operation is invalid")
	}
	return body[0] == 2, nil
}

// ClosedAcceptFrame returns the exact fixed admission acknowledgement. A
// refusal has zero credit and exposes no detailed path-or-policy oracle.
func ClosedAcceptFrame(status uint8, credit uint32) (ClosedLaneFrame, error) {
	if status > 4 || status != 0 && credit != 0 {
		return ClosedLaneFrame{}, errors.New("closed acceptance is invalid")
	}
	body := make([]byte, 5)
	body[0] = status
	binary.BigEndian.PutUint32(body[1:], credit)
	return ClosedLaneFrame{Kind: closedFrameAccept, Lane: 0, Body: body}, nil
}

// DecodeClosedAcceptFrame accepts only a lane-zero fixed acknowledgement.
func DecodeClosedAcceptFrame(frame ClosedLaneFrame) (status uint8, credit uint32, err error) {
	if frame.Kind != closedFrameAccept || frame.Lane != 0 || len(frame.Body) != 5 || frame.Body[0] > 4 || frame.Body[0] != 0 && binary.BigEndian.Uint32(frame.Body[1:]) != 0 {
		return 0, 0, errors.New("closed acceptance is invalid")
	}
	return frame.Body[0], binary.BigEndian.Uint32(frame.Body[1:]), nil
}

func validClosedFrame(frame ClosedLaneFrame) bool {
	if frame.Kind == 0 || len(frame.Body) > closedLaneMaximum {
		return false
	}
	if frame.Kind == closedFrameHello {
		return frame.Lane == 0 && len(frame.Body) == 209
	}
	if frame.Kind == closedFrameAdmit {
		return len(frame.Body) == 355
	}
	if frame.Kind == closedFrameBootstrap {
		return frame.Lane == 0 && len(frame.Body) == 1
	}
	if frame.Kind == closedFrameOpen {
		return frame.Lane != 0 && len(frame.Body) == 49
	}
	if frame.Kind == closedFrameAccept {
		return frame.Lane == 0 && len(frame.Body) == 5
	}
	if frame.Kind == closedFrameBytes {
		return frame.Lane != 0 && len(frame.Body) >= 1
	}
	if frame.Kind == closedFrameCredit {
		return frame.Lane != 0 && len(frame.Body) == 4 && binary.BigEndian.Uint32(frame.Body) != 0
	}
	if frame.Kind == closedFrameEOF {
		return frame.Lane != 0 && len(frame.Body) == 0
	}
	if frame.Kind == closedFrameClose {
		return frame.Lane != 0 && len(frame.Body) == 1 && frame.Body[0] <= 6
	}
	if frame.Kind == closedFrameOperation {
		return frame.Lane != 0 && (len(frame.Body) == closedTerminalOperationSize || len(frame.Body) == closedSmallTerminalOperation)
	}
	if frame.Kind == closedFrameResult {
		return frame.Lane != 0 && len(frame.Body) == closedTerminalOperationSize
	}
	if frame.Kind == closedFrameKeepalive {
		return frame.Lane == 0 && len(frame.Body) == 0
	}
	return false
}

func validClosedHello(hello ClosedHello) bool {
	return hello.NetworkID != [32]byte{} && hello.StateGeneration != [32]byte{} && hello.StateDigest != [32]byte{} &&
		hello.ProfileDigest != [32]byte{} && hello.RecipientNodeID != [32]byte{} && hello.RecipientDutyGeneration != 0 &&
		validClosedPurpose(hello.Purpose) && hello.ChannelNonce != [32]byte{} && !hello.Deadline.IsZero() &&
		hello.Deadline == hello.Deadline.UTC().Truncate(time.Second)
}

func validClosedPurpose(purpose ClosedPurpose) bool {
	return purpose >= ClosedPurposeIssuer && purpose <= ClosedPurposeForwarding
}
