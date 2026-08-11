package nodelifecycle

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"io"

	"github.com/dianabuilds/ardents-network/internal/networkstate"
)

const (
	probeHeaderBytes  = 4 + 1 + 6*32 + 2
	probePayloadBytes = 32
)

var probeProfile = sha256.Sum256([]byte("h3-role-probe-v1"))

type probeRequest struct {
	network    [32]byte
	profile    [32]byte
	epoch      [32]byte
	node       [32]byte
	assignment [32]byte
	nonce      [32]byte
	payload    []byte
}

func readProbeRequest(reader io.Reader) (probeRequest, error) {
	header := make([]byte, probeHeaderBytes)
	if _, err := io.ReadFull(reader, header); err != nil {
		return probeRequest{}, err
	}
	if string(header[:4]) != "ARNP" || header[4] != 1 {
		return probeRequest{}, errors.New("role probe header is invalid")
	}
	request := probeRequest{}
	offset := 5
	for _, target := range []*[32]byte{&request.network, &request.profile, &request.epoch, &request.node, &request.assignment, &request.nonce} {
		copy(target[:], header[offset:offset+32])
		offset += 32
	}
	length := int(binary.BigEndian.Uint16(header[offset:]))
	if length != probePayloadBytes {
		return probeRequest{}, errors.New("role probe payload length is invalid")
	}
	request.payload = make([]byte, length)
	if _, err := io.ReadFull(reader, request.payload); err != nil {
		return probeRequest{}, err
	}
	return request, nil
}

func requestMatches(request probeRequest, snapshot networkstate.Snapshot) bool {
	return request.network == snapshot.NetworkID && request.profile == probeProfile && request.epoch == snapshot.Digest &&
		request.node == snapshot.NodeID && request.assignment == snapshot.AssignmentDigest &&
		!bytes.Equal(request.nonce[:], make([]byte, 32))
}

func writeProbeResponse(writer io.Writer, request probeRequest) error {
	response := make([]byte, probeHeaderBytes+sha256.Size)
	copy(response, "ARNS")
	response[4] = 1
	offset := 5
	for _, value := range [][32]byte{request.network, request.profile, request.epoch, request.node, request.assignment, request.nonce} {
		copy(response[offset:offset+32], value[:])
		offset += 32
	}
	binary.BigEndian.PutUint16(response[offset:], sha256.Size)
	digest := sha256.Sum256(request.payload)
	copy(response[probeHeaderBytes:], digest[:])
	_, err := writer.Write(response)
	return err
}
