package processfixture

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
)

// SignedInstanceResponse encodes the public credential fixture independently
// of the product codec, using the supplied test Authority.
func SignedInstanceResponse(authority ed25519.PrivateKey, network, publicKey, commitment [32]byte, generation uint64, notBefore, notAfter int64) ([]byte, [32]byte) {
	public := authority.Public().(ed25519.PublicKey)
	target := sha256.Sum256(append([]byte("ardents-service-target-v3\x00"), public...))
	credential := append([]byte{0, 3}, public...)
	credential = append(credential, target[:]...)
	credential = append(credential, publicKey[:]...)
	for _, number := range []uint64{generation, uint64(notBefore), uint64(notAfter)} {
		credential = binary.BigEndian.AppendUint64(credential, number)
	}
	credential = append(credential, network[:]...)
	credential = binary.BigEndian.AppendUint32(credential, 3)
	credential = append(credential, ed25519.Sign(authority, credential)...)
	response := append([]byte("ardents-service-instance-response-v3\x00"), commitment[:]...)
	return append(response, credential...), target
}
