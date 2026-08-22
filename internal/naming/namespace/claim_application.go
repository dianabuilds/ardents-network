package namespace

import (
	"encoding/hex"
	"errors"
)

// ApplyOrderedClaim verifies the authenticated Epoch close and materializes
// only its input-ordinal winner to an empty or Released root Name. Anonymous
// admission was consumed before the input entered the signed Epoch log.
func ApplyOrderedClaim(order ClaimOrder, proof ClaimProof,
	current *Record, now int64, op Op, policy Policy,
) (Record, error) {
	result, err := order.Verify(proof)
	if err != nil || result.Outcome != "accepted" || result.WinnerOrdinal != op.ClaimOrdinal ||
		len(op.Parents) != 0 {
		return Record{}, errors.New("root claim ordering is not accepted")
	}
	var winner *Claim
	for index := range proof.Claims {
		if proof.Claims[index].Ordinal == result.WinnerOrdinal {
			winner = &proof.Claims[index]
			break
		}
	}
	if winner == nil || winner.Name != op.Name || hex.EncodeToString(winner.Authority[:]) != op.Authority {
		return Record{}, errors.New("root claim operation does not match the authenticated winner")
	}
	return Apply(current, now, op, policy)
}
