package stock

import (
	"errors"

	"github.com/dianabuilds/ardents-network/internal/admission"
	admissiontoken "github.com/dianabuilds/ardents-network/internal/admission/token"

	"slices"
	"time"
)

type stockEntry struct {
	Challenge admissiontoken.ClosedTokenContext
	Tokens    [][]byte
}

func (perm *permission) StockCountForDuty(profileDigest, receiver [32]byte, duty uint64, class uint8) int {
	if perm == nil {
		return 0
	}
	ready := 0
	for _, stock := range perm.stock {
		if stock.Challenge.ReceiverNodeID == receiver && stock.Challenge.ProfileDigest == profileDigest &&
			stock.Challenge.Class == class && stock.Challenge.WindowStart == perm.accepted.NotBefore &&
			stock.Challenge.ReceiverDutyGeneration == duty {
			ready += len(stock.Tokens)
		}
	}
	return ready
}

// Remaining owns the allocation arithmetic so malformed retained state cannot
// turn a spent grant into an apparently large unsigned balance.
func (perm *permission) Remaining(class uint8) uint32 {
	if perm == nil || class < 1 || class > 3 || perm.reserved[class-1] > perm.accepted.Maxima[class-1] {
		return 0
	}
	return perm.accepted.Maxima[class-1] - perm.reserved[class-1]
}

// reserveBatch owns exact retry matching and allocation reservation under the
// Owner lock. Challenges and delivery identity are selected by the application.
func (perm *permission) reserveBatch(profile admission.AuthorityFacts, now time.Time,
	challenges []admissiontoken.ClosedTokenContext, selection ExchangeBinding, refill bool,
	bootstrap bool) (*batch, error) {
	if batch := perm.pending; batch != nil {
		if batch.Refill != refill || !slices.Equal(batch.Challenges, challenges) || batch.Selection != selection ||
			batch.Bootstrap != bootstrap {
			return nil, errors.New("admission issuance retry must retain the original batch")
		}
		return batch, nil
	}

	if bootstrap && perm.BootstrapAllowance() == 0 {
		return nil, errors.New("admission bootstrap batch allowance is exhausted")
	}
	if len(challenges) == 0 {
		return nil, errors.New("admission issuance allocation is empty")
	}
	class := challenges[0].Class
	if class < 1 || class > 3 || uint32(len(challenges)) > perm.Remaining(class) {
		return nil, errors.New("admission issuance allocation is exhausted")
	}
	pending, err := admissiontoken.PrepareClosedTokenBatch(admissiontoken.ClosedTokenBatchConfig{Profile: profile, Contexts: challenges,
		Permission: perm.accepted, HolderKey: perm.holder, Now: now})
	if err != nil {
		return nil, err
	}
	batch := &batch{Refill: refill, Bootstrap: bootstrap, Challenges: slices.Clone(challenges), Selection: selection, Pending: pending}
	perm.pending = batch
	perm.reserved[class-1] += uint32(len(challenges))
	if bootstrap {
		perm.batches++
	}
	return batch, nil
}

// discardPendingBatch erases only the admitted batch. Its allocation remains
// reserved after cancellation, so a separate attempt cannot reuse that grant.
func (perm *permission) discardPendingBatch(batch *batch) {
	if perm == nil || batch == nil || perm.pending != batch {
		return
	}
	batch.Pending.Discard()
	perm.pending = nil
}

// acceptIssuedBatch finalizes the exact retained batch and deposits its tokens
// in the permission-owned stock. A failed finalization consumes the batch.
func (perm *permission) acceptIssuedBatch(batch *batch, body []byte, check func() error) error {
	if perm == nil || batch == nil || perm.pending != batch {
		return errors.New("admission issuance batch owner changed")
	}
	tokens, err := batch.Pending.FinalizeEncoded(body)
	perm.pending = nil
	if err != nil {
		return err
	}
	if check != nil {
		if err := check(); err != nil {
			for _, raw := range tokens {
				clear(raw)
			}
			return err
		}
	}
	if perm.stock == nil {
		perm.stock = make(map[admissiontoken.ClosedTokenContext]*stockEntry)
	}
	for index, token := range tokens {
		challenge := batch.Challenges[index]
		if entry := perm.stock[challenge]; entry != nil {
			entry.Tokens = append(entry.Tokens, token)
		} else {
			perm.stock[challenge] = &stockEntry{Challenge: challenge, Tokens: [][]byte{token}}
		}
	}
	return nil
}

// consumeToken burns one exact stock entry under the holder owner lock
// before verifying its signature. An invalid token is never returned or
// restored.
func (perm *permission) consumeToken(profile admission.AuthorityFacts, now time.Time, hello Presentation, class uint8) ([]byte, error) {
	if !perm.CurrentFor(profile, now) {
		return nil, TransferFailureAt("permission", errors.New("admission permission is no longer current"))
	}
	challenge := admissiontoken.ClosedTokenContext{NetworkID: profile.NetworkID, ProfileDigest: profile.Digest, IssuerNodeID: profile.IssuerNodeID,
		ReceiverNodeID: hello.RecipientNodeID, ReceiverDutyGeneration: hello.RecipientDutyGeneration, Class: class, WindowStart: perm.accepted.NotBefore}
	if int(profile.TokenKeyCount) > len(profile.TokenKeys) {
		return nil, TransferFailureAt("key-inventory", errors.New("admission token key inventory unavailable"))
	}
	var spki []byte
	for _, key := range profile.TokenKeys[:profile.TokenKeyCount] {
		if key.Class == class && key.WindowStart == challenge.WindowStart {
			if spki != nil {
				return nil, TransferFailureAt("key-ambiguity", errors.New("admission token key ambiguous"))
			}
			spki = key.SPKI[:]
		}
	}
	if stock := perm.stock[challenge]; stock != nil && len(stock.Tokens) > 0 {
		token := stock.Tokens[0]
		stock.Tokens[0] = nil
		stock.Tokens = stock.Tokens[1:]
		if err := admissiontoken.VerifyClosedToken(challenge, spki, token); err != nil {
			clear(token)
			return nil, TransferFailureAt("verification", err)
		}
		return token, nil
	}
	return nil, TransferFailureAt("stock", errors.New("admission token stock unavailable"))
}
