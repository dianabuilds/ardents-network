package stock

import (
	"errors"

	"github.com/dianabuilds/ardents-network/internal/successor/admission"
	admissiontoken "github.com/dianabuilds/ardents-network/internal/successor/admission/token"

	"slices"
	"time"
)

type stockEntry struct {
	Challenge admissiontoken.ClosedTokenContext
	Tokens    [][]byte
}

func (permission *permission) StockCountForDuty(profileDigest, receiver [32]byte, duty uint64, class uint8) int {
	if permission == nil {
		return 0
	}
	ready := 0
	for _, stock := range permission.stock {
		if stock.Challenge.ReceiverNodeID == receiver && stock.Challenge.ProfileDigest == profileDigest &&
			stock.Challenge.Class == class && stock.Challenge.WindowStart == permission.accepted.NotBefore &&
			stock.Challenge.ReceiverDutyGeneration == duty {
			ready += len(stock.Tokens)
		}
	}
	return ready
}

// Remaining owns the allocation arithmetic so malformed retained state cannot
// turn a spent grant into an apparently large unsigned balance.
func (permission *permission) Remaining(class uint8) uint32 {
	if permission == nil || class < 1 || class > 3 || permission.reserved[class-1] > permission.accepted.Maxima[class-1] {
		return 0
	}
	return permission.accepted.Maxima[class-1] - permission.reserved[class-1]
}

// reserveBatch owns exact retry matching and allocation reservation under the
// Owner lock. Challenges and delivery identity are selected by the application.
func (permission *permission) reserveBatch(profile admission.AuthorityFacts, now time.Time,
	challenges []admissiontoken.ClosedTokenContext, selection ExchangeBinding, refill bool,
	bootstrap bool) (*batch, error) {
	if batch := permission.pending; batch != nil {
		if batch.Refill != refill || !slices.Equal(batch.Challenges, challenges) || batch.Selection != selection ||
			batch.Bootstrap != bootstrap {
			return nil, errors.New("text issuance retry must retain the original batch")
		}
		return batch, nil
	}

	if bootstrap && permission.BootstrapAllowance() == 0 {
		return nil, errors.New("text bootstrap batch allowance is exhausted")
	}
	if len(challenges) == 0 {
		return nil, errors.New("text issuance allocation is empty")
	}
	class := challenges[0].Class
	if class < 1 || class > 3 || uint32(len(challenges)) > permission.Remaining(class) {
		return nil, errors.New("text issuance allocation is exhausted")
	}
	pending, err := admissiontoken.PrepareClosedTokenBatch(admissiontoken.ClosedTokenBatchConfig{Profile: profile, Contexts: challenges,
		Permission: permission.accepted, HolderKey: permission.holder, Now: now})
	if err != nil {
		return nil, err
	}
	batch := &batch{Refill: refill, Bootstrap: bootstrap, Challenges: slices.Clone(challenges), Selection: selection, Pending: pending}
	permission.pending = batch
	permission.reserved[class-1] += uint32(len(challenges))
	if bootstrap {
		permission.batches++
	}
	return batch, nil
}

// discardPendingBatch erases only the admitted batch. Its allocation remains
// reserved after cancellation, so a separate attempt cannot reuse that grant.
func (permission *permission) discardPendingBatch(batch *batch) {
	if permission == nil || batch == nil || permission.pending != batch {
		return
	}
	batch.Pending.Discard()
	permission.pending = nil
}

// acceptIssuedBatch finalizes the exact retained batch and deposits its tokens
// in the permission-owned stock. A failed finalization consumes the batch.
func (permission *permission) acceptIssuedBatch(batch *batch, body []byte, check func() error) error {
	if permission == nil || batch == nil || permission.pending != batch {
		return errors.New("text issuance batch owner changed")
	}
	tokens, err := batch.Pending.FinalizeEncoded(body)
	permission.pending = nil
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
	for index, token := range tokens {
		challenge := batch.Challenges[index]
		found := false
		for slot := range permission.stock {
			if permission.stock[slot].Challenge == challenge {
				permission.stock[slot].Tokens = append(permission.stock[slot].Tokens, token)
				found = true
				break
			}
		}
		if !found {
			permission.stock = append(permission.stock, stockEntry{Challenge: challenge, Tokens: [][]byte{token}})
		}
	}
	return nil
}

// consumeToken burns one exact stock entry under the holder owner lock
// before verifying its signature. An invalid token is never returned or
// restored.
func (permission *permission) consumeToken(profile admission.AuthorityFacts, now time.Time, hello Presentation, class uint8) ([]byte, error) {
	if !permission.CurrentFor(profile, now) {
		return nil, TransferFailureAt("permission", errors.New("text token permission expired"))
	}
	challenge := admissiontoken.ClosedTokenContext{NetworkID: profile.NetworkID, ProfileDigest: profile.Digest, IssuerNodeID: profile.IssuerNodeID,
		ReceiverNodeID: hello.RecipientNodeID, ReceiverDutyGeneration: hello.RecipientDutyGeneration, Class: class, WindowStart: permission.accepted.NotBefore}
	if int(profile.TokenKeyCount) > len(profile.TokenKeys) {
		return nil, TransferFailureAt("key-inventory", errors.New("text token key inventory unavailable"))
	}
	var spki []byte
	for _, key := range profile.TokenKeys[:profile.TokenKeyCount] {
		if key.Class == class && key.WindowStart == challenge.WindowStart {
			if spki != nil {
				return nil, TransferFailureAt("key-ambiguity", errors.New("text token key ambiguous"))
			}
			spki = key.SPKI[:]
		}
	}
	for index := range permission.stock {
		stock := &permission.stock[index]
		if stock.Challenge != challenge || len(stock.Tokens) == 0 {
			continue
		}
		token := stock.Tokens[0]
		stock.Tokens[0] = nil
		stock.Tokens = stock.Tokens[1:]
		if err := admissiontoken.VerifyClosedToken(challenge, spki, token); err != nil {
			clear(token)
			return nil, TransferFailureAt("verification", err)
		}
		return token, nil
	}
	return nil, TransferFailureAt("stock", errors.New("text forwarding token stock unavailable"))
}
