package wire

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/tls"
	"errors"
)

const ExporterLabel = "EXPORTER-ardents-service-connection-v3"

// AttachmentSecrets derives only this Attachment's TLS exporter commitments.
// The initial key becomes the logical continuity secret at authenticated
// commit; replacement callers retain that original key and use only the new
// commitment here. No derived digest attests Publication/local authority.
func AttachmentSecrets(connection *tls.Conn, attachmentContext [32]byte) (key, commitment [32]byte, err error) {
	if connection == nil || connection.NetConn() == nil {
		return key, commitment, errors.New("service Attachment TLS absent")
	}
	state := connection.ConnectionState()
	if err := ValidateExporterState(state, attachmentContext); err != nil {
		return key, commitment, err
	}
	material, err := state.ExportKeyingMaterial(ExporterLabel, attachmentContext[:], 32)
	if err != nil {
		return key, commitment, err
	}
	defer clear(material)
	return DeriveExporterSecrets(material)
}

func ValidateExporterState(state tls.ConnectionState, attachmentContext [32]byte) error {
	if !state.HandshakeComplete || state.Version != tls.VersionTLS13 || state.DidResume || attachmentContext == [32]byte{} {
		return errors.New("service Attachment TLS exporter unavailable")
	}
	return nil
}

func DeriveExporterSecrets(material []byte) (key, commitment [32]byte, err error) {
	if len(material) != 32 {
		return key, commitment, errors.New("service Attachment TLS exporter length differs")
	}
	mac := hmac.New(sha256.New, material)
	mac.Write([]byte("ardents-service-connection-continuity-key-v3\x00"))
	secret := mac.Sum(nil)
	copy(key[:], secret)
	clear(secret)
	encoded := append([]byte("ardents-service-connection-exporter-v3\x00"), key[:]...)
	commitment = sha256.Sum256(encoded)
	clear(encoded)
	return key, commitment, nil
}

func ContinuityCommitment(key [32]byte) [32]byte {
	encoded := append([]byte("ardents-service-connection-continuity-commitment-v3\x00"), key[:]...)
	commitment := sha256.Sum256(encoded)
	clear(encoded)
	return commitment
}
