package stage6verify

import (
	"bytes"
	"encoding/binary"
	"errors"
	"strings"
)

type decodedRecord struct {
	Schema                                                          uint16
	Name, Lease, Consistency, Recovery, Authority, Parent, Conflict string
	Generation, Revision, ParentGeneration, RecoveryPolicyRev       uint64
	PendingPolicyRev, Continuity                                    uint64
	Target, RecoveryPolicy, PendingPolicy                           [32]byte
	RecoveryOperation, RecoverySuccessor                            [32]byte
	LeaseExpires, GraceExpires, RecoveryExpires, RecoveryStarted    int64
	RecoveryPolicyDelay, PendingPolicyDelay, PolicyActivates        int64
	RecordNotAfter                                                  int64
}

func decodeRecords(raw []byte) ([]decodedRecord, error) {
	if len(raw) < 2 {
		return nil, errors.New("record sequence is truncated")
	}
	count, offset := int(binary.BigEndian.Uint16(raw)), 2
	records := make([]decodedRecord, count)
	for index := range records {
		if len(raw)-offset < 4 {
			return nil, errors.New("record sequence length is truncated")
		}
		size := int(binary.BigEndian.Uint32(raw[offset:]))
		offset += 4
		if size <= 0 || len(raw)-offset < size {
			return nil, errors.New("record sequence entry is malformed")
		}
		var err error
		records[index], err = decodeRecord(raw[offset : offset+size])
		if err != nil {
			return nil, err
		}
		offset += size
	}
	if offset != len(raw) {
		return nil, errors.New("record sequence has trailing bytes")
	}
	return records, nil
}

func decodeRecord(raw []byte) (decodedRecord, error) {
	cursor := wireCursor{raw: raw}
	version, err := cursor.u16()
	if err != nil || (version != 3 && version != 4) {
		return decodedRecord{}, errors.New("record schema is invalid")
	}
	record := decodedRecord{Schema: version}
	if record.Name, err = cursor.text(); err != nil {
		return record, err
	}
	if record.Generation, err = cursor.u64(); err != nil {
		return record, err
	}
	if record.Revision, err = cursor.u64(); err != nil {
		return record, err
	}
	if len(raw)-cursor.offset < 3 {
		return record, errors.New("record states are truncated")
	}
	lease := []string{"", "active", "grace", "released"}
	consistency := []string{"", "current", "conflict", "fork", "unavailable"}
	recovery := []string{"", "stable", "recovery-pending"}
	a, b, c := int(raw[cursor.offset]), int(raw[cursor.offset+1]), int(raw[cursor.offset+2])
	cursor.offset += 3
	if a <= 0 || a >= len(lease) || b <= 0 || b >= len(consistency) || c <= 0 || c >= len(recovery) {
		return record, errors.New("record state is unknown")
	}
	record.Lease, record.Consistency, record.Recovery = lease[a], consistency[b], recovery[c]
	if record.Authority, err = cursor.text(); err != nil {
		return record, err
	}
	for _, target := range []*[32]byte{&record.Target, &record.RecoveryPolicy, &record.PendingPolicy,
		&record.RecoveryOperation, &record.RecoverySuccessor} {
		if err := cursor.fixed(target[:]); err != nil {
			return record, err
		}
	}
	if record.Parent, err = cursor.text(); err != nil {
		return record, err
	}
	values := make([]uint64, 11)
	for index := range values {
		if values[index], err = cursor.u64(); err != nil {
			return record, err
		}
	}
	record.ParentGeneration = values[0]
	record.LeaseExpires, record.GraceExpires = int64(values[1]), int64(values[2])
	record.RecoveryExpires, record.RecoveryStarted = int64(values[3]), int64(values[4])
	record.RecoveryPolicyRev, record.RecoveryPolicyDelay = values[5], int64(values[6])
	record.PendingPolicyRev, record.PendingPolicyDelay = values[7], int64(values[8])
	record.PolicyActivates, record.Continuity = int64(values[9]), values[10]
	if record.Conflict, err = cursor.text(); err != nil {
		return decodedRecord{}, errors.New("record is malformed")
	}
	if version == 4 {
		value, valueErr := cursor.u64()
		if valueErr != nil || value > uint64(^uint64(0)>>1) {
			return decodedRecord{}, errors.New("record validity is malformed")
		}
		record.RecordNotAfter = int64(value)
	}
	if cursor.offset != len(raw) || !validDecodedRecord(record) {
		return decodedRecord{}, errors.New("record is malformed")
	}
	canonical := encodeRecord(record)
	if !bytes.Equal(canonical, raw) {
		return decodedRecord{}, errors.New("record is non-canonical")
	}
	return record, nil
}

func validDecodedRecord(record decodedRecord) bool {
	if !canonicalName(record.Name) || record.Generation == 0 || record.Revision == 0 || record.Authority == "" ||
		record.Continuity == 0 || (record.Parent == "") != (record.ParentGeneration == 0) {
		return false
	}
	if record.Parent != "" && (!canonicalName(record.Parent) || !strings.HasSuffix(record.Name, "."+record.Parent)) {
		return false
	}
	if record.Lease == "released" {
		if record.LeaseExpires != 0 || record.GraceExpires != 0 {
			return false
		}
	} else if record.LeaseExpires <= 0 || record.GraceExpires < record.LeaseExpires {
		return false
	}
	if record.Consistency == "current" && record.Conflict != "" ||
		record.Consistency == "conflict" && record.Conflict == "" {
		return false
	}
	return true
}

func encodeRecord(record decodedRecord) []byte {
	schema := record.Schema
	if schema == 0 {
		schema = 4
	}
	out := binary.BigEndian.AppendUint16(nil, schema)
	out = appendText64(out, record.Name)
	out = binary.BigEndian.AppendUint64(out, record.Generation)
	out = binary.BigEndian.AppendUint64(out, record.Revision)
	out = append(out, map[string]byte{"active": 1, "grace": 2, "released": 3}[record.Lease])
	out = append(out, map[string]byte{"current": 1, "conflict": 2, "fork": 3, "unavailable": 4}[record.Consistency])
	out = append(out, map[string]byte{"stable": 1, "recovery-pending": 2}[record.Recovery])
	out = appendText64(out, record.Authority)
	for _, target := range [][32]byte{record.Target, record.RecoveryPolicy, record.PendingPolicy,
		record.RecoveryOperation, record.RecoverySuccessor} {
		out = append(out, target[:]...)
	}
	out = appendText64(out, record.Parent)
	for _, value := range []uint64{record.ParentGeneration, uint64(record.LeaseExpires), uint64(record.GraceExpires),
		uint64(record.RecoveryExpires), uint64(record.RecoveryStarted), record.RecoveryPolicyRev,
		uint64(record.RecoveryPolicyDelay), record.PendingPolicyRev, uint64(record.PendingPolicyDelay),
		uint64(record.PolicyActivates), record.Continuity} {
		out = binary.BigEndian.AppendUint64(out, value)
	}
	out = appendText64(out, record.Conflict)
	if schema == 4 {
		out = binary.BigEndian.AppendUint64(out, uint64(record.RecordNotAfter))
	}
	return out
}

type wireCursor struct {
	raw    []byte
	offset int
}

func (cursor *wireCursor) u16() (uint16, error) {
	if len(cursor.raw)-cursor.offset < 2 {
		return 0, errors.New("wire is truncated")
	}
	value := binary.BigEndian.Uint16(cursor.raw[cursor.offset:])
	cursor.offset += 2
	return value, nil
}

func (cursor *wireCursor) u64() (uint64, error) {
	if len(cursor.raw)-cursor.offset < 8 {
		return 0, errors.New("wire is truncated")
	}
	value := binary.BigEndian.Uint64(cursor.raw[cursor.offset:])
	cursor.offset += 8
	return value, nil
}

func (cursor *wireCursor) text() (string, error) {
	size, err := cursor.u64()
	if err != nil || size > uint64(len(cursor.raw)-cursor.offset) {
		return "", errors.New("wire text is malformed")
	}
	value := string(cursor.raw[cursor.offset : cursor.offset+int(size)])
	cursor.offset += int(size)
	return value, nil
}

func (cursor *wireCursor) fixed(target []byte) error {
	if len(cursor.raw)-cursor.offset < len(target) {
		return errors.New("wire fixed field is truncated")
	}
	copy(target, cursor.raw[cursor.offset:cursor.offset+len(target)])
	cursor.offset += len(target)
	return nil
}

func appendText64(out []byte, value string) []byte {
	out = binary.BigEndian.AppendUint64(out, uint64(len(value)))
	return append(out, value...)
}

func canonicalName(value string) bool {
	if value == "" || len(value) > 253 || strings.HasPrefix(value, ".") || strings.HasSuffix(value, ".") ||
		strings.Contains(value, "..") {
		return false
	}
	parts := strings.Split(value, ".")
	if len(parts) > 127 {
		return false
	}
	for index, part := range parts {
		if part == "" || len(part) > 63 || strings.HasPrefix(part, "-") || strings.HasSuffix(part, "-") ||
			strings.Contains(part, "--") {
			return false
		}
		allDigits := true
		for _, value := range []byte(part) {
			if !(value >= 'a' && value <= 'z' || value >= '0' && value <= '9' || value == '-') {
				return false
			}
			allDigits = allDigits && value >= '0' && value <= '9'
		}
		if index == len(parts)-1 && allDigits {
			return false
		}
	}
	return true
}
