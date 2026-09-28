//go:build linux

package tokens

import (
	"errors"
	"slices"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/route/ardp"
	"github.com/dianabuilds/ardents-network/internal/route/client"
	"github.com/dianabuilds/ardents-network/internal/route/credential"
)

type Stock struct {
	Challenge credential.ClosedTokenContext
	Tokens    [][]byte
}

// StockCountFor inspects candidate stock before Route supplies the exact
// recipient duty. Presentation still matches and verifies the full challenge;
// this preflight grants no spending authority.
func (permission *Permission) StockCountFor(profileDigest, receiver [32]byte, class uint8) int {
	return permission.countStock(profileDigest, receiver, 0, class, false)
}

// StockCountForDuty uses a known recipient duty when the caller has one.
func (permission *Permission) StockCountForDuty(profileDigest, receiver [32]byte, duty uint64, class uint8) int {
	return permission.countStock(profileDigest, receiver, duty, class, true)
}

// MissingStockFor lists, in receiver order, the receivers that hold no stock
// of the given class for the profile digest. It runs under the shared context
// lock and only inspects; the caller decides whether and how to issue the
// missing stock, and must unlock before any issuance.
func (permission *Permission) MissingStockFor(profileDigest [32]byte, receivers [][32]byte, class uint8) [][32]byte {
	var missing [][32]byte
	for _, receiver := range receivers {
		if permission.StockCountFor(profileDigest, receiver, class) == 0 {
			missing = append(missing, receiver)
		}
	}
	return missing
}

func (permission *Permission) countStock(profileDigest, receiver [32]byte, duty uint64, class uint8, exactDuty bool) int {
	if permission == nil {
		return 0
	}
	ready := 0
	for _, stock := range permission.Stock {
		if stock.Challenge.ReceiverNodeID == receiver && stock.Challenge.ProfileDigest == profileDigest &&
			stock.Challenge.Class == class && stock.Challenge.WindowStart == permission.Accepted.NotBefore &&
			(!exactDuty || stock.Challenge.ReceiverDutyGeneration == duty) {
			ready += len(stock.Tokens)
		}
	}
	return ready
}

// Remaining owns the allocation arithmetic so malformed retained state cannot
// turn a spent grant into an apparently large unsigned balance.
func (permission *Permission) Remaining(class uint8) uint32 {
	if permission == nil || class < 1 || class > 3 || permission.Reserved[class-1] > permission.Accepted.Maxima[class-1] {
		return 0
	}
	return permission.Accepted.Maxima[class-1] - permission.Reserved[class-1]
}

// ReserveBatch owns exact retry matching and allocation reservation. The
// Context holds its admission lock while selecting the live Route prefix and
// State challenges; the permission alone changes its batch and quota state.
func (permission *Permission) ReserveBatch(profile state.ClosedProfileView, now time.Time,
	challenges []credential.ClosedTokenContext, selection client.ClosedBootstrapSelection, refill bool,
	current Prefix, joined bool, expected Prefix) (*Batch, error) {
	if batch := permission.Pending; batch != nil {
		if batch.Refill != refill || !slices.Equal(batch.Challenges, challenges) || batch.Selection != selection ||
			joined && batch.Prefix != expected ||
			batch.Prefix != nil && (batch.Prefix != current || !current.Loaded()) {
			return nil, errors.New("text issuance retry must retain the original batch")
		}
		return batch, nil
	}
	prefix := current
	if prefix == nil && permission.Batches >= 2 {
		return nil, errors.New("text bootstrap batch allowance is exhausted")
	}
	if len(challenges) == 0 {
		return nil, errors.New("text issuance allocation is empty")
	}
	class := challenges[0].Class
	if class < 1 || class > 3 || uint32(len(challenges)) > permission.Remaining(class) {
		return nil, errors.New("text issuance allocation is exhausted")
	}
	pending, err := credential.PrepareClosedTokenBatch(credential.ClosedTokenBatchConfig{Profile: profile, Contexts: challenges,
		Permission: permission.Accepted, HolderKey: permission.Holder, Now: now})
	if err != nil {
		return nil, err
	}
	batch := &Batch{Refill: refill, Prefix: prefix, Challenges: challenges, Selection: selection, Pending: pending}
	permission.Pending = batch
	permission.Reserved[class-1] += uint32(len(challenges))
	if prefix == nil {
		permission.Batches++
	}
	return batch, nil
}

// discardPendingBatch erases only the admitted batch. Its allocation remains
// reserved after cancellation, so a separate attempt cannot reuse that grant.
func (permission *Permission) discardPendingBatch(batch *Batch) {
	if permission == nil || batch == nil || permission.Pending != batch {
		return
	}
	batch.Pending.Discard()
	permission.Pending = nil
}

// acceptIssuedBatch finalizes the exact retained batch and deposits its tokens
// in the permission-owned stock. A failed finalization consumes the batch.
func (permission *Permission) acceptIssuedBatch(batch *Batch, nonce [32]byte, body []byte) error {
	if permission == nil || batch == nil || permission.Pending != batch {
		return errors.New("text issuance batch owner changed")
	}
	tokens, err := batch.Pending.FinalizeTerminalOperation(nonce, body)
	permission.Pending = nil
	if err != nil {
		return err
	}
	for index, token := range tokens {
		challenge := batch.Challenges[index]
		found := false
		for slot := range permission.Stock {
			if permission.Stock[slot].Challenge == challenge {
				permission.Stock[slot].Tokens = append(permission.Stock[slot].Tokens, token)
				found = true
				break
			}
		}
		if !found {
			permission.Stock = append(permission.Stock, Stock{Challenge: challenge, Tokens: [][]byte{token}})
		}
	}
	return nil
}

// consumeToken burns one exact stock entry under the shared context lock
// before verifying its signature. An invalid token is never returned or
// restored.
func (permission *Permission) consumeToken(profile state.ClosedProfileView, now time.Time, hello ardp.Hello, class uint8) ([]byte, error) {
	if !permission.CurrentFor(profile, now) {
		return nil, TransferFailureAt("permission", errors.New("text token permission expired"))
	}
	challenge := credential.ClosedTokenContext{NetworkID: profile.NetworkID, ProfileDigest: profile.Digest, IssuerNodeID: profile.IssuerNodeID,
		ReceiverNodeID: hello.RecipientNodeID, ReceiverDutyGeneration: hello.RecipientDutyGeneration, Class: class, WindowStart: permission.Accepted.NotBefore}
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
	for index := range permission.Stock {
		stock := &permission.Stock[index]
		if stock.Challenge != challenge || len(stock.Tokens) == 0 {
			continue
		}
		token := stock.Tokens[0]
		stock.Tokens[0] = nil
		stock.Tokens = stock.Tokens[1:]
		if err := credential.VerifyClosedToken(challenge, spki, token); err != nil {
			clear(token)
			return nil, TransferFailureAt("verification", err)
		}
		return token, nil
	}
	return nil, TransferFailureAt("stock", errors.New("text forwarding token stock unavailable"))
}
