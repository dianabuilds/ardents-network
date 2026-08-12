package state

import "crypto/sha256"

func networkIdentityDigest(network [32]byte) [32]byte {
	prefix := []byte("ardents-h3-network-id-v1\x00")
	return sha256.Sum256(append(prefix, network[:]...))
}

func isZero32(value [32]byte) bool { return value == [32]byte{} }
