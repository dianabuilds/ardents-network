package epoch

import (
	"crypto/ed25519"
	"encoding/binary"
	"errors"
	"net"
	"strconv"
	"time"
)

// InitialClosedRecord describes one fresh closed Node Record. NodeID remains
// distinct from its signing key, and preparation grants neither duty nor State.
type InitialClosedRecord struct {
	NetworkID, NodeID         [32]byte
	ValidFrom, ValidUntil     time.Time
	Family, Endpoint, Carrier string
	Capacity                  uint16
	PublicKey                 ed25519.PublicKey
}

// PrepareInitialClosedRecord returns the existing ARNR version-two unsigned
// signing message, fixed to generation one and closed Route capability two.
func PrepareInitialClosedRecord(input InitialClosedRecord) ([]byte, error) {
	if input.NetworkID == [32]byte{} || input.NodeID == [32]byte{} || len(input.PublicKey) != ed25519.PublicKeySize ||
		input.Capacity == 0 || input.Capacity > 1024 || !initialPreparationInterval(input.ValidFrom, input.ValidUntil) ||
		!candidateCarrierEligible(closedRouteProfile, input.Carrier) {
		return nil, errors.New("initial closed Node Record facts are invalid")
	}
	host, port, err := net.SplitHostPort(input.Endpoint)
	portNumber, portErr := strconv.ParseUint(port, 10, 16)
	if err != nil || portErr != nil || host == "" || portNumber == 0 {
		return nil, errors.New("initial closed Node endpoint is invalid")
	}
	message := append([]byte("ARNR"), 2)
	message = append(message, input.NetworkID[:]...)
	message = append(message, input.NodeID[:]...)
	message = binary.BigEndian.AppendUint64(message, 1)
	message = binary.BigEndian.AppendUint64(message, uint64(input.ValidFrom.Unix()))
	message = binary.BigEndian.AppendUint64(message, uint64(input.ValidUntil.Unix()))
	message, err = appendInitialPreparationText(message, input.Family, 32)
	if err != nil {
		return nil, err
	}
	message = append(message, 2)
	message, err = appendInitialPreparationText(message, input.Endpoint, 96)
	if err != nil {
		return nil, err
	}
	message, err = appendInitialPreparationText(message, input.Carrier, 48)
	if err != nil {
		return nil, err
	}
	message = binary.BigEndian.AppendUint16(message, input.Capacity)
	message = append(message, input.PublicKey...)
	return message, nil
}

func initialPreparationInterval(from, until time.Time) bool {
	return !from.IsZero() && !until.IsZero() && from.Unix() >= 0 && until.Unix() >= 0 && from.Before(until) && from.Equal(from.UTC().Truncate(time.Second)) && until.Equal(until.UTC().Truncate(time.Second))
}

func appendInitialPreparationText(message []byte, value string, maximum int) ([]byte, error) {
	if len(value) == 0 || len(value) > maximum {
		return nil, errors.New("initial Network preparation text length is invalid")
	}
	for _, character := range value {
		if character < 0x21 || character > 0x7e {
			return nil, errors.New("initial Network preparation text is not canonical ASCII")
		}
	}
	message = append(message, byte(len(value)))
	return append(message, value...), nil
}
