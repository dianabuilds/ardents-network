package access

import (
	"context"
	"crypto/ed25519"
	"encoding/binary"
	"fmt"
	"time"

	identitycontract "ardents/api/ardents/identity/v1"
	identityprincipal "ardents/internal/identity/principal"
	"ardents/internal/storage"
)

const enrollmentRecordBytes = ed25519.PublicKeySize + 8

type enrollmentRepository struct{ database storage.Database }

func (r enrollmentRepository) record(ctx context.Context, enrollment EnrollmentRecord) error {
	key, record, err := encodeEnrollment(enrollment)
	if err != nil {
		return err
	}
	return r.database.Update(ctx, func(tx storage.WriteTransaction) error {
		return recordEnrollment(tx, key, record)
	})
}

func recordEnrollment(tx storage.WriteTransaction, key, record []byte) error {
	existing, found, err := tx.Get(enrollmentsBucket, key)
	if err != nil {
		return err
	}
	if found && string(existing) != string(record) {
		return fmt.Errorf("conflicting Principal enrollment")
	}
	if found {
		return nil
	}
	return tx.Put(enrollmentsBucket, key, record)
}

func (r enrollmentRepository) load(ctx context.Context, node, principal string) (EnrollmentRecord, error) {
	key, err := enrollmentKey(node, principal)
	if err != nil {
		return EnrollmentRecord{}, err
	}
	var result EnrollmentRecord
	err = r.database.View(ctx, func(tx storage.ReadTransaction) error {
		record, found, err := tx.Get(enrollmentsBucket, key)
		if err != nil {
			return err
		}
		if !found {
			return fmt.Errorf("Principal enrollment is missing")
		}
		result, err = decodeEnrollment(node, principal, record)
		return err
	})
	return result, err
}

func encodeEnrollment(enrollment EnrollmentRecord) ([]byte, []byte, error) {
	key, err := enrollmentKey(enrollment.Node, enrollment.Principal)
	if err != nil {
		return nil, nil, err
	}
	derived, err := identityprincipal.FromEd25519PublicKey(enrollment.RootPublicKey[:])
	if err != nil || derived.String() != enrollment.Principal || enrollment.EnrolledAt.Nanosecond() != 0 ||
		enrollment.EnrolledAt.Unix() < identitycontract.LowerTimestampUnix || enrollment.EnrolledAt.Unix() >= identitycontract.UpperTimestampUnix {
		return nil, nil, errInvalid
	}
	record := make([]byte, enrollmentRecordBytes)
	copy(record, enrollment.RootPublicKey[:])
	binary.BigEndian.PutUint64(record[ed25519.PublicKeySize:], uint64(enrollment.EnrolledAt.Unix()))
	return key, record, nil
}

func decodeEnrollment(node, principal string, record []byte) (EnrollmentRecord, error) {
	if len(record) != enrollmentRecordBytes {
		return EnrollmentRecord{}, fmt.Errorf("Principal enrollment record is corrupt")
	}
	var root [ed25519.PublicKeySize]byte
	copy(root[:], record[:ed25519.PublicKeySize])
	result := EnrollmentRecord{
		Node:          node,
		Principal:     principal,
		RootPublicKey: root,
		EnrolledAt:    time.Unix(int64(binary.BigEndian.Uint64(record[ed25519.PublicKeySize:])), 0).UTC(),
	}
	if _, _, err := encodeEnrollment(result); err != nil {
		return EnrollmentRecord{}, fmt.Errorf("Principal enrollment record is corrupt")
	}
	return result, nil
}

func enrollmentKey(node, principal string) ([]byte, error) {
	if _, err := identityprincipal.Parse(node); err != nil {
		return nil, errInvalid
	}
	if _, err := identityprincipal.Parse(principal); err != nil {
		return nil, errInvalid
	}
	return tuple([]byte(node), []byte(principal)), nil
}
