package stage6verify

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
)

func verifyNamespaceTransitionCorpus(evidence resolutionCellEvidence, statement namespaceStatement) bool {
	if len(evidence.NamespaceTransitions) != int(statement.transitions) || len(evidence.NamespaceTransitions) != 2 ||
		namespaceRawRoot(evidence.NamespaceTransitions, 0x63) != statement.transition ||
		len(evidence.NamespaceClaimAuthorityIDs) != len(evidence.NamespaceClaimPublicKeys) ||
		int(evidence.NamespaceClaimThreshold) < 2 || evidence.NamespaceClaimMaximum == 0 ||
		len(evidence.NamespaceClaimAuthorityIDs) > 16 {
		return false
	}
	policy := claimPolicy{network: statement.network, minimumEpoch: statement.epoch,
		maximumClaims: evidence.NamespaceClaimMaximum, threshold: int(evidence.NamespaceClaimThreshold),
		authorities: make(map[[32]byte]ed25519.PublicKey, len(evidence.NamespaceClaimAuthorityIDs))}
	for index, id := range evidence.NamespaceClaimAuthorityIDs {
		public := evidence.NamespaceClaimPublicKeys[index]
		if sha256.Sum256(public[:]) != id || index > 0 &&
			bytes.Compare(evidence.NamespaceClaimAuthorityIDs[index-1][:], id[:]) >= 0 {
			return false
		}
		policy.authorities[id] = append(ed25519.PublicKey(nil), public[:]...)
	}
	proof := evidence.NamespaceClaim
	if proof.Network != statement.network || proof.Epoch != statement.epoch ||
		proof.CutoffOffset != int64(statement.cutoff) || proof.RejectionRoot != statement.rejected ||
		proof.RejectionLength != statement.rejections ||
		!verifyClaimCorpus(proof, evidence.NamespaceClaimInputs, evidence.NamespaceClaimRejections) {
		return false
	}
	outcome, winnerOrdinal, losers := evaluateClaim(policy, proof)
	if outcome != "accepted" || len(losers) != 1 || winnerOrdinal != 0 {
		return false
	}
	claimRecord, claimErr := verifySignedNamespaceRecord(evidence.NamespaceTransitions[0], statement.network)
	currentRecord, currentErr := verifySignedNamespaceRecord(evidence.NamespaceTransitions[1], statement.network)
	if claimErr != nil || currentErr != nil || claimRecord.Name != currentRecord.Name ||
		claimRecord.Generation != currentRecord.Generation || claimRecord.Revision != 1 || currentRecord.Revision != 2 ||
		claimRecord.Authority != currentRecord.Authority || claimRecord.Target != [32]byte{} ||
		currentRecord.Target == [32]byte{} || len(evidence.NamespaceRecords) != 1 ||
		!bytes.Equal(evidence.NamespaceTransitions[1], evidence.NamespaceRecords[0]) {
		return false
	}
	for _, claim := range proof.Claims {
		if claim.Ordinal == winnerOrdinal {
			return claim.Name == claimRecord.Name && hex.EncodeToString(claim.Authority[:]) == claimRecord.Authority
		}
	}
	return false
}
