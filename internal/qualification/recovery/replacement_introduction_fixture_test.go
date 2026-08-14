package recovery

import (
	"crypto/sha256"
	"encoding/binary"
	"time"
)

func s42IntroductionProof(value routeCase, manifest [32]byte,
	selected map[string]replacementCandidate) (introductionProof, [32]byte) {
	proof := introductionProof{ManifestDigest: manifest, NetworkID: value.NetworkID, EpochDigest: value.EpochDigest,
		ViewRoot: value.ViewRoot, ProfileDigest: sha256.Sum256([]byte(value.Profile)),
		CapabilitiesDigest: sha256.Sum256([]byte("ardents-h3-recovery-setup-capabilities-v1\x00tls13|single-use|no-application-data")),
		IntroductionNode:   selected["introduction"].NodeID, RendezvousNode: selected["rendezvous"].NodeID,
		RendezvousReachability: sha256.Sum256(append([]byte("ardents-h3-rendezvous-reachability-v1\x00"),
			selected["rendezvous"].Endpoint...)), JoinHandle: [32]byte{70}, EndpointHandshake: [32]byte{71},
		Reply: [32]byte{72}, ExpiresAtNanos: time.Unix(value.SelectionAt, 0).Add(time.Hour).UnixNano()}
	body := make([]byte, 397)
	copy(body[:5], "ASIS\x02")
	fields := [][32]byte{proof.ManifestDigest, proof.NetworkID, proof.EpochDigest, proof.ViewRoot, proof.ProfileDigest,
		proof.CapabilitiesDigest, proof.IntroductionNode, proof.RendezvousNode, proof.RendezvousReachability,
		proof.JoinHandle, proof.EndpointHandshake}
	for index, field := range fields {
		copy(body[5+index*32:5+(index+1)*32], field[:])
	}
	binary.BigEndian.PutUint64(body[357:365], uint64(proof.ExpiresAtNanos))
	proof.TranscriptContext = sha256.Sum256(append([]byte("ardents-h3-sealed-introduction-transcript-v2\x00"), body[:365]...))
	copy(body[365:], proof.TranscriptContext[:])
	receipt := append([]byte("ardents-h3-sealed-introduction-v2\x00"), body...)
	receipt = append(receipt, proof.Reply[:]...)
	return proof, sha256.Sum256(receipt)
}
