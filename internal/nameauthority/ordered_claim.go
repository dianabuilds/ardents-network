package nameauthority

import (
	"encoding/hex"
	"errors"

	"github.com/dianabuilds/ardents-network/internal/nameclaim"
	"github.com/dianabuilds/ardents-network/internal/namelease"
)

// ApplyOrderedClaim verifies the authenticated Epoch close and materializes
// only its input-ordinal winner to an empty or Released root Name. Anonymous
// admission was consumed before the input entered the signed Epoch log.
func ApplyOrderedClaim(order nameclaim.ClaimOrder, proof nameclaim.Proof,
	current *namelease.Record, now int64, op namelease.Op, policy namelease.Policy,
) (namelease.Record, error) {
	result, err := order.Verify(proof)
	if err != nil || result.Outcome != "accepted" || result.WinnerOrdinal != op.ClaimOrdinal ||
		len(op.Parents) != 0 {
		return namelease.Record{}, errors.New("root claim ordering is not accepted")
	}
	var winner *nameclaim.Claim
	for index := range proof.Claims {
		if proof.Claims[index].Ordinal == result.WinnerOrdinal {
			winner = &proof.Claims[index]
			break
		}
	}
	if winner == nil || winner.Name != op.Name || hex.EncodeToString(winner.Authority[:]) != op.Authority {
		return namelease.Record{}, errors.New("root claim operation does not match the authenticated winner")
	}
	return namelease.Apply(current, now, op, policy)
}
