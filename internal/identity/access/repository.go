package access

import (
	"context"
	"encoding/binary"
	"fmt"

	identityprincipal "ardents/internal/identity/principal"
	"ardents/internal/storage"
)

const deviceRevocationsBucket = "identity-device-revocations-v1"
const (
	enrollmentsBucket                  = "identity-enrollments-v1"
	grantsBucket                       = "identity-access-grants-v1"
	grantIndexBucket                   = "identity-access-grant-index-v1"
	grantRevocationsBucket             = "identity-access-grant-revocations-v1"
	bootstrapTicketsBucket             = "identity-bootstrap-tickets-v1"
	enrollmentCredentialsBucket        = "identity-enrollment-credentials-v1"
	adminCommandsBucket                = "identity-admin-commands-v1"
	applicationEnrollmentTicketsBucket = "identity-application-enrollment-tickets-v1"
)

func StorageSchema() storage.Schema {
	return storage.Schema{Version: 6, Migrations: []storage.Migration{
		{Version: 1},
		{Version: 2, Buckets: []string{deviceRevocationsBucket}},
		{Version: 3, Buckets: []string{enrollmentsBucket, grantsBucket, grantIndexBucket, grantRevocationsBucket}},
		{Version: 4, Buckets: []string{bootstrapTicketsBucket}},
		{Version: 5, Buckets: []string{enrollmentCredentialsBucket, adminCommandsBucket}},
		{Version: 6, Buckets: []string{applicationEnrollmentTicketsBucket}},
	}}
}

type deviceRevocations struct{ database storage.Database }

func (r deviceRevocations) revoked(ctx context.Context, audience Audience, subject, deviceID string) (bool, error) {
	key, err := deviceRevocationKey(audience.Node, subject, deviceID)
	if err != nil {
		return false, err
	}
	var found bool
	err = r.database.View(ctx, func(tx storage.ReadTransaction) error {
		found, err = deviceRevoked(tx, key)
		return err
	})
	return found, err
}

func deviceRevoked(tx storage.ReadTransaction, key []byte) (bool, error) {
	_, found, err := tx.Get(deviceRevocationsBucket, key)
	return found, err
}

func (r deviceRevocations) record(ctx context.Context, artifact *Artifact) error {
	payload := artifact.DeviceRevocationPayload()
	if payload == nil {
		return errInvalid
	}
	key, err := deviceRevocationKey(payload.Audience.Node, payload.Subject, payload.TargetDeviceId)
	if err != nil {
		return err
	}
	raw, err := artifact.MarshalBinary()
	if err != nil {
		return err
	}
	return r.database.Update(ctx, func(tx storage.WriteTransaction) error {
		return recordDeviceRevocation(tx, key, raw)
	})
}

func recordDeviceRevocation(tx storage.WriteTransaction, key, raw []byte) error {
	existing, found, err := tx.Get(deviceRevocationsBucket, key)
	if err != nil {
		return err
	}
	if found {
		if string(existing) != string(raw) {
			return fmt.Errorf("conflicting device revocation")
		}
		return nil
	}
	return tx.Put(deviceRevocationsBucket, key, raw)
}

func deviceRevocationKey(node, subject, deviceID string) ([]byte, error) {
	if _, err := identityprincipal.Parse(node); err != nil {
		return nil, errInvalid
	}
	if _, err := identityprincipal.Parse(subject); err != nil {
		return nil, errInvalid
	}
	if _, err := identityprincipal.ParseDeviceID(deviceID); err != nil {
		return nil, errInvalid
	}
	parts := []string{node, subject, deviceID}
	length := 0
	for _, part := range parts {
		length += 2 + len(part)
	}
	key := make([]byte, 0, length)
	for _, part := range parts {
		var size [2]byte
		binary.BigEndian.PutUint16(size[:], uint16(len(part)))
		key = append(key, size[:]...)
		key = append(key, part...)
	}
	return key, nil
}
