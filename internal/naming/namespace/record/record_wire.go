package record

import (
	"encoding/binary"
	"errors"

	"github.com/dianabuilds/ardents-network/internal/naming"
)

// Record V3 is decode-only migration input. Record V4 adds the signed Target
// validity boundary selected by ADR-0022.
const (
	legacyRecordSchemaVersion uint16 = 3
	recordSchemaVersion       uint16 = 4
)

// EncodeRecord deterministically encodes one validated lifecycle Record.
func EncodeRecord(record Record) ([]byte, error) {
	return encodeRecord(record, recordSchemaVersion)
}

func encodeRecord(record Record, schema uint16) ([]byte, error) {
	if schema != legacyRecordSchemaVersion && schema != recordSchemaVersion {
		return nil, errors.New("name record schema is unavailable")
	}
	if err := validateRecord(record); err != nil {
		return nil, err
	}
	out := make([]byte, 0, 128)
	out = binary.BigEndian.AppendUint16(out, schema)
	out = appendRecordString(out, record.Name)
	out = binary.BigEndian.AppendUint64(out, record.Generation)
	out = binary.BigEndian.AppendUint64(out, record.Revision)
	out = append(out, leaseCode(record.Lease), consistencyCode(record.Consistency), recoveryCode(record.Recovery))
	out = appendRecordString(out, record.Authority)
	out = append(out, record.Target[:]...)
	out = append(out, record.RecoveryPolicy[:]...)
	out = append(out, record.PendingPolicy[:]...)
	out = append(out, record.RecoveryOperation[:]...)
	out = append(out, record.RecoverySuccessor[:]...)
	out = appendRecordString(out, record.ParentName)
	for _, value := range []uint64{record.ParentGeneration, uint64(record.LeaseExpiresAt),
		uint64(record.GraceExpiresAt), uint64(record.RecoveryExpiresAt), uint64(record.RecoveryStartedAt),
		record.RecoveryPolicyRev, uint64(record.RecoveryPolicyDelay), record.PendingPolicyRev,
		uint64(record.PendingPolicyDelay), uint64(record.PolicyActivatesAt), record.Continuity} {
		out = binary.BigEndian.AppendUint64(out, value)
	}
	out = appendRecordString(out, record.ConflictIdentifier)
	if schema == recordSchemaVersion {
		out = binary.BigEndian.AppendUint64(out, uint64(record.RecordNotAfter))
	}
	return out, nil
}

// DecodeRecord decodes only the exact canonical Record encoding.
func DecodeRecord(raw []byte) (Record, error) {
	c := recordCursor{raw: raw}
	version, err := c.uint16()
	if err != nil || (version != legacyRecordSchemaVersion && version != recordSchemaVersion) {
		return Record{}, errors.New("name record has invalid schema version")
	}
	name, err := c.text()
	if err != nil {
		return Record{}, err
	}
	generation, err := c.uint64()
	if err != nil {
		return Record{}, err
	}
	revision, err := c.uint64()
	if err != nil {
		return Record{}, err
	}
	lease, consistency, recovery, err := c.states()
	if err != nil {
		return Record{}, err
	}
	authority, err := c.text()
	if err != nil {
		return Record{}, err
	}
	target, err := c.target()
	if err != nil {
		return Record{}, err
	}
	recoveryPolicy, err := c.target()
	if err != nil {
		return Record{}, err
	}
	pendingPolicy, err := c.target()
	if err != nil {
		return Record{}, err
	}
	recoveryOperation, err := c.target()
	if err != nil {
		return Record{}, err
	}
	recoverySuccessor, err := c.target()
	if err != nil {
		return Record{}, err
	}
	parent, err := c.text()
	if err != nil {
		return Record{}, err
	}
	numbers := make([]uint64, 11)
	for i := range numbers {
		numbers[i], err = c.uint64()
		if err != nil {
			return Record{}, err
		}
	}
	conflict, err := c.text()
	if err != nil {
		return Record{}, errors.New("name record has trailing or malformed bytes")
	}
	recordNotAfter := int64(0)
	if version == recordSchemaVersion {
		value, valueErr := c.uint64()
		if valueErr != nil || value > uint64(^uint64(0)>>1) {
			return Record{}, errors.New("name record validity is malformed")
		}
		recordNotAfter = int64(value)
	}
	if c.offset != len(raw) {
		return Record{}, errors.New("name record has trailing or malformed bytes")
	}
	record := Record{Name: name, Generation: generation, Revision: revision,
		Lease: lease, Consistency: consistency, Recovery: recovery,
		Authority: authority, Target: target, RecoveryPolicy: recoveryPolicy,
		PendingPolicy: pendingPolicy, RecoveryOperation: recoveryOperation,
		RecoverySuccessor: recoverySuccessor, ParentName: parent,
		ParentGeneration: numbers[0], LeaseExpiresAt: int64(numbers[1]),
		GraceExpiresAt: int64(numbers[2]), RecoveryExpiresAt: int64(numbers[3]), RecoveryStartedAt: int64(numbers[4]),
		RecoveryPolicyRev: numbers[5], RecoveryPolicyDelay: int64(numbers[6]),
		PendingPolicyRev: numbers[7], PendingPolicyDelay: int64(numbers[8]),
		PolicyActivatesAt: int64(numbers[9]), Continuity: numbers[10], ConflictIdentifier: conflict,
		RecordNotAfter: recordNotAfter}
	if err := validateRecord(record); err != nil {
		return Record{}, err
	}
	canonical, err := encodeRecord(record, version)
	if err != nil || string(canonical) != string(raw) {
		return Record{}, errors.New("name record is not canonical")
	}
	return record, nil
}

func validateRecord(record Record) error {
	if _, err := naming.Parse(record.Name); err != nil || record.Generation == 0 || record.Revision == 0 {
		return errors.New("name record identity is invalid")
	}
	if !validStates(record) || !validRecordLifetimes(record) || !hasRequiredParent(record) || record.RecordNotAfter < 0 ||
		record.Authority == "" {
		return errors.New("name record state or binding is invalid")
	}
	if (record.ParentName == "") != (record.ParentGeneration == 0) {
		return errors.New("name record parent binding is incomplete")
	}
	if record.ParentName != "" {
		child, childErr := naming.Parse(record.Name)
		parent, parentErr := naming.Parse(record.ParentName)
		if childErr != nil || parentErr != nil || !naming.IsDescendant(child, parent) {
			return errors.New("name record parent is not an ancestor")
		}
	}
	if record.Consistency == consistencyCurrent && record.ConflictIdentifier != "" {
		return errors.New("current record contains conflict evidence")
	}
	if record.Consistency == consistencyConflict && record.ConflictIdentifier == "" {
		return errors.New("conflict record is missing evidence identifier")
	}
	if !validRecoveryBindings(record) {
		return errors.New("name record recovery binding is invalid")
	}
	return nil
}

func appendRecordString(out []byte, value string) []byte {
	out = binary.BigEndian.AppendUint64(out, uint64(len(value)))
	return append(out, value...)
}

type recordCursor struct {
	raw    []byte
	offset int
}

func (c *recordCursor) uint16() (uint16, error) {
	if len(c.raw)-c.offset < 2 {
		return 0, errors.New("name record is truncated")
	}
	value := binary.BigEndian.Uint16(c.raw[c.offset:])
	c.offset += 2
	return value, nil
}

func (c *recordCursor) uint64() (uint64, error) {
	if len(c.raw)-c.offset < 8 {
		return 0, errors.New("name record is truncated")
	}
	value := binary.BigEndian.Uint64(c.raw[c.offset:])
	c.offset += 8
	return value, nil
}

func (c *recordCursor) text() (string, error) {
	size, err := c.uint64()
	if err != nil || size > uint64(len(c.raw)-c.offset) {
		return "", errors.New("name record string is malformed")
	}
	value := string(c.raw[c.offset : c.offset+int(size)])
	c.offset += int(size)
	return value, nil
}

func (c *recordCursor) target() ([32]byte, error) {
	var target [32]byte
	if len(c.raw)-c.offset < len(target) {
		return target, errors.New("name record Target is truncated")
	}
	copy(target[:], c.raw[c.offset:c.offset+len(target)])
	c.offset += len(target)
	return target, nil
}

func (c *recordCursor) states() (string, string, string, error) {
	if len(c.raw)-c.offset < 3 {
		return "", "", "", errors.New("name record states are truncated")
	}
	lease := []string{"", leaseActive, leaseGrace, leaseReleased}
	consistency := []string{"", consistencyCurrent, consistencyConflict, consistencyFork, consistencyUnavailable}
	recovery := []string{"", recoveryStable, recoveryPending}
	a, b, d := int(c.raw[c.offset]), int(c.raw[c.offset+1]), int(c.raw[c.offset+2])
	c.offset += 3
	if a == 0 || a >= len(lease) || b == 0 || b >= len(consistency) || d == 0 || d >= len(recovery) {
		return "", "", "", errors.New("name record contains an unknown state")
	}
	return lease[a], consistency[b], recovery[d], nil
}

func leaseCode(value string) byte {
	return map[string]byte{leaseActive: 1, leaseGrace: 2, leaseReleased: 3}[value]
}

func consistencyCode(value string) byte {
	return map[string]byte{consistencyCurrent: 1, consistencyConflict: 2,
		consistencyFork: 3, consistencyUnavailable: 4}[value]
}

func recoveryCode(value string) byte {
	return map[string]byte{recoveryStable: 1, recoveryPending: 2}[value]
}
