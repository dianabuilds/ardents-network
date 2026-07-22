package content

import (
	"errors"

	identitycontract "ardents/api/ardents/identity/v1"
	datapayload "ardents/internal/content/payload"
)

const MaxAccessPayloadBytes = identitycontract.MaxOperatorPublishBlobPayloadBytes

func AccessResourceID(id string) (string, error) {
	if len(id) == 0 || len(id) > identitycontract.MaxCanonicalResourceIDBytes {
		return "", errors.New("content resource identifier is invalid")
	}
	for _, b := range []byte(id) {
		if b < 0x21 || b > 0x7e {
			return "", errors.New("content resource identifier is invalid")
		}
	}
	return id, nil
}

// PublishBlobAccessResourceID derives the exact content reference from the
// bounded payload and rejects every conflicting client-declared identity.
func PublishBlobAccessResourceID(command PublishBlobCommand) (string, error) {
	if len(command.Payload) == 0 || len(command.Payload) > MaxAccessPayloadBytes {
		return "", errors.New("content payload is invalid")
	}
	hash, blobCID, err := datapayload.DeriveIdentity(command.Payload)
	if err != nil {
		return "", errors.New("content payload identity is invalid")
	}
	if err := datapayload.ApplyDerivedIdentity(&command.Blob, hash, blobCID); err != nil {
		return "", errors.New("content payload identity is invalid")
	}
	return AccessResourceID(blobCID)
}
