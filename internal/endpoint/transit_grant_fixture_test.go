package endpoint

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"errors"

	"github.com/dianabuilds/ardents-network/internal/route"
)

func issueTransitGrantFixture(input route.TransitGrant, signer ed25519.PrivateKey) ([]byte, error) {
	if len(signer) != ed25519.PrivateKeySize || input.IssuerID != sha256.Sum256(signer.Public().(ed25519.PublicKey)) {
		return nil, errors.New("transit grant fixture signer is invalid")
	}
	body := make([]byte, 0, len("ardents-transit-grant-v1\x00")+2+7*32+8+1+8)
	body = append(body, "ardents-transit-grant-v1\x00"...)
	body = binary.BigEndian.AppendUint16(body, 1)
	for _, value := range [][32]byte{input.IssuerID, input.GrantID, input.NetworkID, input.Digest, input.AttachmentID,
		input.TransitNodeID, input.ClientKeyDigest} {
		body = append(body, value[:]...)
	}
	body = binary.BigEndian.AppendUint64(body, input.Epoch)
	body = append(body, input.TransitRole)
	body = binary.BigEndian.AppendUint64(body, uint64(input.NotAfter.Unix()))
	signature := ed25519.Sign(signer, append([]byte("ardents-transit-grant-signature-v1\x00"), body...))
	return append(body, signature...), nil
}
