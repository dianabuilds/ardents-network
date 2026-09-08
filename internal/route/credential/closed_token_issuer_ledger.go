package credential

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"os"
	"path/filepath"
	"time"
)

const (
	closedTokenIssuerLedgerName       = "closed-token-ledger"
	closedTokenIssuerLedgerMagic      = "ARDILG01"
	closedTokenIssuerLedgerHeaderSize = 8 + 32 + 32 + 32
	closedTokenIssuerReservationSize  = 32 + 32 + 32 + 8 + 1 + 2 + 1
	maximumClosedTokenReservations    = 6 * 65536
)

type closedTokenIssuerLedger struct {
	root                         string
	network, node, profileDigest [32]byte
	reservations                 []closedTokenIssuerReservation
}

type closedTokenIssuerReservation struct {
	requestID, requestDigest, permissionID [32]byte
	window                                 time.Time
	class                                  uint8
	count                                  uint16
}

func openClosedTokenIssuerLedger(root string, network, node, profileDigest [32]byte) (*closedTokenIssuerLedger, error) {
	if network == [32]byte{} || node == [32]byte{} || profileDigest == [32]byte{} {
		return nil, errors.New("closed token issuer ledger binding is invalid")
	}
	path := filepath.Join(root, closedTokenIssuerLedgerName)
	raw, err := readIssuerFile(path, int64(closedTokenIssuerLedgerHeaderSize+maximumClosedTokenReservations*closedTokenIssuerReservationSize))
	if errors.Is(err, os.ErrNotExist) {
		ledger := &closedTokenIssuerLedger{root: root, network: network, node: node, profileDigest: profileDigest, reservations: []closedTokenIssuerReservation{}}
		if err := initializeClosedTokenIssuerLedger(path, ledger); err != nil {
			return nil, err
		}
		return ledger, nil
	}
	if err != nil {
		return nil, err
	}
	return decodeClosedTokenIssuerLedger(root, raw, network, node, profileDigest)
}

func initializeClosedTokenIssuerLedger(path string, ledger *closedTokenIssuerLedger) error {
	raw := make([]byte, 0, closedTokenIssuerLedgerHeaderSize)
	raw = append(raw, closedTokenIssuerLedgerMagic...)
	for _, value := range [][32]byte{ledger.network, ledger.node, ledger.profileDigest} {
		raw = append(raw, value[:]...)
	}
	if err := writeIssuerExclusive(path, raw); err != nil {
		return err
	}
	return syncIssuerDirectory(filepath.Dir(path))
}

func decodeClosedTokenIssuerLedger(root string, raw []byte, network, node, profileDigest [32]byte) (*closedTokenIssuerLedger, error) {
	if len(raw) < closedTokenIssuerLedgerHeaderSize || string(raw[:8]) != closedTokenIssuerLedgerMagic {
		return nil, errors.New("closed token issuer ledger framing is invalid")
	}
	ledger := &closedTokenIssuerLedger{root: root, network: network, node: node, profileDigest: profileDigest, reservations: []closedTokenIssuerReservation{}}
	offset := 8
	for _, value := range [][32]byte{network, node, profileDigest} {
		if !bytes.Equal(raw[offset:offset+32], value[:]) {
			return nil, errors.New("closed token issuer ledger cannot be rebound")
		}
		offset += 32
	}
	complete := len(raw) - (len(raw)-offset)%closedTokenIssuerReservationSize
	for offset < complete {
		record := raw[offset : offset+closedTokenIssuerReservationSize]
		if record[len(record)-1] == 0 {
			if err := truncateClosedTokenIssuerLedger(filepath.Join(root, closedTokenIssuerLedgerName), int64(offset)); err != nil {
				return nil, err
			}
			break
		}
		if record[len(record)-1] != 1 {
			return nil, errors.New("closed token issuer ledger commit is invalid")
		}
		reservation, err := decodeClosedTokenIssuerReservation(record)
		if err != nil {
			return nil, err
		}
		for _, prior := range ledger.reservations {
			if prior.requestID == reservation.requestID {
				return nil, errors.New("closed token issuer ledger request ID is duplicated")
			}
		}
		ledger.reservations = append(ledger.reservations, reservation)
		if len(ledger.reservations) > maximumClosedTokenReservations {
			return nil, errors.New("closed token issuer ledger exceeds reservation bound")
		}
		offset += closedTokenIssuerReservationSize
	}
	if complete != len(raw) {
		if err := truncateClosedTokenIssuerLedger(filepath.Join(root, closedTokenIssuerLedgerName), int64(complete)); err != nil {
			return nil, err
		}
	}
	return ledger, nil
}

func (ledger *closedTokenIssuerLedger) find(requestID, digest [32]byte) (closedTokenIssuerReservation, bool, error) {
	for _, reservation := range ledger.reservations {
		if reservation.requestID == requestID {
			if reservation.requestDigest != digest {
				return closedTokenIssuerReservation{}, false, errors.New("closed token issuer request ID conflicts")
			}
			return reservation, true, nil
		}
	}
	return closedTokenIssuerReservation{}, false, nil
}

func (ledger *closedTokenIssuerLedger) reserve(request ClosedTokenBatchRequest, digest [32]byte) (bool, error) {
	if _, found, err := ledger.find(request.RequestID, digest); err != nil || found {
		return found, err
	}
	count := uint32(len(request.BlindedRequests))
	var dutyUsed, permissionUsed uint32
	for _, reservation := range ledger.reservations {
		if reservation.window == request.WindowStart {
			dutyUsed += uint32(reservation.count)
			if reservation.permissionID == request.Permission.PermissionID && reservation.class == request.Class {
				permissionUsed += uint32(reservation.count)
			}
		}
	}
	if dutyUsed+count > 65536 || permissionUsed+count > request.Permission.Maxima[request.Class-1] || len(ledger.reservations) == maximumClosedTokenReservations {
		return false, nil
	}
	reservation := closedTokenIssuerReservation{requestID: request.RequestID, requestDigest: digest, permissionID: request.Permission.PermissionID,
		window: request.WindowStart, class: request.Class, count: uint16(count)}
	if err := appendClosedTokenIssuerReservation(filepath.Join(ledger.root, closedTokenIssuerLedgerName), reservation); err != nil {
		return false, err
	}
	ledger.reservations = append(ledger.reservations, reservation)
	return true, nil
}

func encodeClosedTokenIssuerReservation(reservation closedTokenIssuerReservation) ([]byte, error) {
	if reservation.requestID == [32]byte{} || reservation.requestDigest == [32]byte{} || reservation.permissionID == [32]byte{} ||
		reservation.window.IsZero() || reservation.window != reservation.window.UTC() || reservation.window.Truncate(time.Hour) != reservation.window ||
		reservation.class < 1 || reservation.class > 3 || reservation.count == 0 || reservation.count > maximumClosedTokenBatch {
		return nil, errors.New("closed token issuer reservation is invalid")
	}
	raw := make([]byte, 0, closedTokenIssuerReservationSize)
	for _, value := range [][32]byte{reservation.requestID, reservation.requestDigest, reservation.permissionID} {
		raw = append(raw, value[:]...)
	}
	raw = binary.BigEndian.AppendUint64(raw, uint64(reservation.window.Unix()))
	raw = append(raw, reservation.class)
	raw = binary.BigEndian.AppendUint16(raw, reservation.count)
	return append(raw, 0), nil
}

func decodeClosedTokenIssuerReservation(raw []byte) (closedTokenIssuerReservation, error) {
	if len(raw) != closedTokenIssuerReservationSize || raw[len(raw)-1] != 1 {
		return closedTokenIssuerReservation{}, errors.New("closed token issuer reservation framing is invalid")
	}
	value := closedTokenIssuerReservation{}
	offset := 0
	for _, field := range []*[32]byte{&value.requestID, &value.requestDigest, &value.permissionID} {
		copy(field[:], raw[offset:offset+32])
		offset += 32
	}
	value.window = time.Unix(int64(binary.BigEndian.Uint64(raw[offset:offset+8])), 0).UTC()
	offset += 8
	value.class = raw[offset]
	offset++
	value.count = binary.BigEndian.Uint16(raw[offset : offset+2])
	committed, err := encodeClosedTokenIssuerReservation(value)
	if err != nil || !bytes.Equal(committed[:len(committed)-1], raw[:len(raw)-1]) {
		return closedTokenIssuerReservation{}, errors.New("closed token issuer reservation is noncanonical")
	}
	return value, nil
}

func appendClosedTokenIssuerReservation(path string, reservation closedTokenIssuerReservation) error {
	raw, err := encodeClosedTokenIssuerReservation(reservation)
	if err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	start, err := file.Seek(0, io.SeekEnd)
	if err == nil {
		_, err = file.Write(raw)
	}
	if err == nil {
		err = file.Sync()
	}
	if err == nil {
		_, err = file.WriteAt([]byte{1}, start+int64(len(raw)-1))
	}
	if err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err != nil {
		return err
	}
	return closeErr
}

func truncateClosedTokenIssuerLedger(path string, size int64) error {
	file, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	if err = file.Truncate(size); err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err != nil {
		return err
	}
	return closeErr
}
