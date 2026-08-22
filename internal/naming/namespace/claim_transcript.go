package namespace

import (
	"crypto/sha256"
	"encoding/binary"
)

// CommitmentFor binds one hidden reveal to its Network Epoch and authority.
func CommitmentFor(network [32]byte, epoch uint64, claim Claim) [32]byte {
	transcript := claimText(nil, "ardents-name-claim-commit-v1")
	transcript = append(transcript, network[:]...)
	transcript = binary.BigEndian.AppendUint64(transcript, epoch)
	transcript = claimText(transcript, claim.Name)
	transcript = append(transcript, claim.Authority[:]...)
	transcript = append(transcript, claim.Secret[:]...)
	return sha256.Sum256(transcript)
}

// RevealTranscript returns the canonical authority-signed reveal bytes.
func RevealTranscript(network [32]byte, epoch uint64, claim Claim) []byte {
	transcript := claimText(nil, "ardents-name-claim-reveal-v1")
	transcript = append(transcript, network[:]...)
	transcript = binary.BigEndian.AppendUint64(transcript, epoch)
	transcript = binary.BigEndian.AppendUint32(transcript, claim.Ordinal)
	transcript = append(transcript, claim.Commitment[:]...)
	return transcript
}

// StatementTranscript returns the exact threshold-signed Epoch close.
func StatementTranscript(proof ClaimProof) []byte {
	out := claimText(nil, "ardents-name-claim-epoch-close-v1")
	out = append(out, proof.Network[:]...)
	out = binary.BigEndian.AppendUint64(out, proof.Epoch)
	out = claimText(out, proof.Rule)
	out = binary.BigEndian.AppendUint64(out, uint64(proof.CutoffOffset))
	out = append(out, proof.InputRoot[:]...)
	out = binary.BigEndian.AppendUint32(out, proof.InputLength)
	out = append(out, proof.MaterializationRoot[:]...)
	out = binary.BigEndian.AppendUint32(out, proof.MaterializationLength)
	out = append(out, proof.RejectionRoot[:]...)
	return binary.BigEndian.AppendUint32(out, proof.RejectionLength)
}

func claimText(out []byte, value string) []byte {
	out = binary.BigEndian.AppendUint32(out, uint32(len(value)))
	return append(out, value...)
}
