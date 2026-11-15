package systemdbus

import (
	"encoding/binary"
	"strconv"
	"strings"
)

// FramePolicy contains detached restrictions from the original connection owner.
// It does not select a method or grant manager access.
type FramePolicy struct {
	Owner          string
	Properties     bool
	AllowObject    bool
	ExpectedObject string
	AllowJob       bool
}

// ByteOrder identifies a supported wire byte order.
func ByteOrder(marker byte) (binary.ByteOrder, bool) {
	if marker == 'l' {
		return binary.LittleEndian, true
	}
	if marker == 'B' {
		return binary.BigEndian, true
	}
	return nil, false
}

// ValidateFrame verifies a bounded reply against the caller’s fixed connection policy.
func ValidateFrame(frame []byte, policy FramePolicy) error {
	owner, properties := policy.Owner, policy.Properties
	if policy.AllowObject && policy.ExpectedObject == "" || policy.AllowJob && !policy.AllowObject {
		return ErrObservation
	}
	if len(frame) < 16 || len(frame) > 65536 {
		return ErrObservation
	}
	order, ok := ByteOrder(frame[0])
	if !ok || frame[3] != 1 || frame[1] < 2 || frame[1] > 4 {
		return ErrObservation
	}
	hlen, blen := uint64(order.Uint32(frame[12:16])), uint64(order.Uint32(frame[4:8]))
	end := uint64(16) + hlen
	bodyStart := (end + 7) &^ uint64(7)
	if end > uint64(len(frame)) || bodyStart+blen != uint64(len(frame)) {
		return ErrObservation
	}
	fields := make(map[byte]string)
	for pos := 16; pos < int(end); {
		pos = (pos + 7) &^ 7
		if pos+4 > int(end) {
			return ErrObservation
		}
		code := frame[pos]
		if code < 1 || code > 9 {
			return ErrObservation
		}
		if _, exists := fields[code]; exists {
			return ErrObservation
		}
		kind := byte('s')
		switch code {
		case 1:
			kind = 'o'
		case 5, 9:
			kind = 'u'
		case 8:
			kind = 'g'
		}
		if frame[pos+1] != 1 || frame[pos+2] != kind || frame[pos+3] != 0 {
			return ErrObservation
		}
		pos += 4
		if kind == 'u' {
			if pos+4 > int(end) {
				return ErrObservation
			}
			value := order.Uint32(frame[pos : pos+4])
			if code == 9 && value != 0 {
				return ErrObservation
			}
			fields[code] = "u"
			pos += 4
			continue
		}
		var value string
		var err error
		value, pos, err = ReadScalar(frame, pos, int(end), kind, order)
		if err != nil {
			return err
		}
		fields[code] = value
	}
	sender := fields[7]
	if sender != "org.freedesktop.DBus" && (owner == "" || sender != owner) {
		return ErrObservation
	}
	if frame[1] == 4 && (sender != "org.freedesktop.DBus" || fields[2] != "org.freedesktop.DBus" || (fields[3] != "NameAcquired" && fields[3] != "NameLost")) {
		return ErrObservation
	}
	if frame[1] != 4 && fields[5] != "u" {
		return ErrObservation
	}
	sig := fields[8]
	if properties && frame[1] == 2 && owner != "" && sender == owner && (sig == "v" || sig == "a{sv}") {
		return ValidateValues(frame, int(bodyStart), sig, order)
	}
	// Each connection admits only its fixed unit's native LoadUnit reply. Job
	// replies belong only to that purpose's original root control. This grammar
	// grants no method selector, loaded policy or accepting startup fact.
	// An absence-only connection refuses all unit and job objects before decode.
	if properties && frame[1] == 2 && owner != "" && sender == owner && sig == "o" {
		if !policy.AllowObject {
			return ErrObservation
		}
		object, pos, err := ReadScalar(frame, int(bodyStart), len(frame), 'o', order)
		if err != nil || pos != len(frame) || (object != policy.ExpectedObject && (!policy.AllowJob || !ValidJobPath(object))) {
			return ErrObservation
		}
		return nil
	}
	if sig == "" {
		if blen != 0 || frame[1] != 2 {
			return ErrObservation
		}
		return nil
	}
	if sig != "s" {
		return ErrObservation
	}
	_, pos, err := ReadScalar(frame, int(bodyStart), len(frame), 's', order)
	if err != nil || pos != len(frame) {
		return ErrObservation
	}
	return nil
}

// ValidJobPath accepts only a canonical systemd job object path.
func ValidJobPath(object string) bool {
	const prefix = "/org/freedesktop/systemd1/job/"
	if !strings.HasPrefix(object, prefix) {
		return false
	}
	value := strings.TrimPrefix(object, prefix)
	number, err := strconv.ParseUint(value, 10, 32)
	return err == nil && number != 0 && strconv.FormatUint(number, 10) == value
}
