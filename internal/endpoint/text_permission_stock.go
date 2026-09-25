//go:build linux

package endpoint

import (
	"errors"
	"slices"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/route/credential"
)

type textTokenStock struct {
	challenge credential.ClosedTokenContext
	tokens    [][]byte
}

// stockCountFor inspects candidate stock before Route supplies the exact
// recipient duty. Presentation still matches and verifies the full challenge;
// this preflight grants no spending authority.
func (permission *textPermission) stockCountFor(profileDigest, receiver [32]byte, class uint8) int {
	return permission.countStock(profileDigest, receiver, 0, class, false)
}

// stockCountForDuty uses a known recipient duty when the caller has one.
func (permission *textPermission) stockCountForDuty(profileDigest, receiver [32]byte, duty uint64, class uint8) int {
	return permission.countStock(profileDigest, receiver, duty, class, true)
}

func (permission *textPermission) countStock(profileDigest, receiver [32]byte, duty uint64, class uint8, exactDuty bool) int {
	if permission == nil {
		return 0
	}
	ready := 0
	for _, stock := range permission.stock {
		if stock.challenge.ReceiverNodeID == receiver && stock.challenge.ProfileDigest == profileDigest &&
			stock.challenge.Class == class && stock.challenge.WindowStart == permission.accepted.NotBefore &&
			(!exactDuty || stock.challenge.ReceiverDutyGeneration == duty) {
			ready += len(stock.tokens)
		}
	}
	return ready
}

// remaining owns the allocation arithmetic so malformed retained state cannot
// turn a spent grant into an apparently large unsigned balance.
func (permission *textPermission) remaining(class uint8) uint32 {
	if permission == nil || class < 1 || class > 3 || permission.reserved[class-1] > permission.accepted.Maxima[class-1] {
		return 0
	}
	return permission.accepted.Maxima[class-1] - permission.reserved[class-1]
}

// reserveBatchLocked owns exact retry matching and allocation reservation. The
// Context holds its admission lock while selecting the live Route prefix and
// State challenges; the permission alone changes its batch and quota state.
func (permission *textPermission) reserveBatchLocked(profile state.ClosedProfileView, now time.Time,
	challenges []credential.ClosedTokenContext, selection route.ClosedBootstrapSelection, refill bool,
	current *textSourceHandle, joined bool, expected *textSourceHandle) (*textTokenBatch, error) {
	if batch := permission.pending; batch != nil {
		if batch.refill != refill || !slices.Equal(batch.challenges, challenges) || batch.selection != selection ||
			joined && batch.prefix != expected ||
			batch.prefix != nil && (batch.prefix != current || current.prefix.Load() == nil) {
			return nil, errors.New("text issuance retry must retain the original batch")
		}
		return batch, nil
	}
	prefix := current
	if prefix == nil && permission.batches >= 2 {
		return nil, errors.New("text bootstrap batch allowance is exhausted")
	}
	if len(challenges) == 0 {
		return nil, errors.New("text issuance allocation is empty")
	}
	class := challenges[0].Class
	if class < 1 || class > 3 || uint32(len(challenges)) > permission.remaining(class) {
		return nil, errors.New("text issuance allocation is exhausted")
	}
	pending, err := credential.PrepareClosedTokenBatch(credential.ClosedTokenBatchConfig{Profile: profile, Contexts: challenges,
		Permission: permission.accepted, HolderKey: permission.holder, Now: now})
	if err != nil {
		return nil, err
	}
	batch := &textTokenBatch{refill: refill, prefix: prefix, challenges: challenges, selection: selection, pending: pending}
	permission.pending = batch
	permission.reserved[class-1] += uint32(len(challenges))
	if prefix == nil {
		permission.batches++
	}
	return batch, nil
}

// discardPendingBatch erases only the admitted batch. Its allocation remains
// reserved after cancellation, so a separate attempt cannot reuse that grant.
func (permission *textPermission) discardPendingBatch(batch *textTokenBatch) {
	if permission == nil || batch == nil || permission.pending != batch {
		return
	}
	batch.pending.Discard()
	permission.pending = nil
}

// acceptIssuedBatch finalizes the exact retained batch and deposits its tokens
// in the permission-owned stock. A failed finalization consumes the batch.
func (permission *textPermission) acceptIssuedBatch(batch *textTokenBatch, nonce [32]byte, body []byte) error {
	if permission == nil || batch == nil || permission.pending != batch {
		return errors.New("text issuance batch owner changed")
	}
	tokens, err := batch.pending.FinalizeTerminalOperation(nonce, body)
	permission.pending = nil
	if err != nil {
		return err
	}
	for index, token := range tokens {
		challenge := batch.challenges[index]
		found := false
		for slot := range permission.stock {
			if permission.stock[slot].challenge == challenge {
				permission.stock[slot].tokens = append(permission.stock[slot].tokens, token)
				found = true
				break
			}
		}
		if !found {
			permission.stock = append(permission.stock, textTokenStock{challenge: challenge, tokens: [][]byte{token}})
		}
	}
	return nil
}

// consumeTextToken burns one exact stock entry under textContext.mu before
// verifying its signature. An invalid token is never returned or restored.
func (permission *textPermission) consumeTextToken(profile state.ClosedProfileView, now time.Time, hello route.ClosedHello, class uint8) ([]byte, error) {
	if !permission.currentFor(profile, now) {
		return nil, textTokenTransferFailureAt("permission", errors.New("text token permission expired"))
	}
	challenge := credential.ClosedTokenContext{NetworkID: profile.NetworkID, ProfileDigest: profile.Digest, IssuerNodeID: profile.IssuerNodeID,
		ReceiverNodeID: hello.RecipientNodeID, ReceiverDutyGeneration: hello.RecipientDutyGeneration, Class: class, WindowStart: permission.accepted.NotBefore}
	if int(profile.TokenKeyCount) > len(profile.TokenKeys) {
		return nil, textTokenTransferFailureAt("key-inventory", errors.New("text token key inventory unavailable"))
	}
	var spki []byte
	for _, key := range profile.TokenKeys[:profile.TokenKeyCount] {
		if key.Class == class && key.WindowStart == challenge.WindowStart {
			if spki != nil {
				return nil, textTokenTransferFailureAt("key-ambiguity", errors.New("text token key ambiguous"))
			}
			spki = key.SPKI[:]
		}
	}
	for index := range permission.stock {
		stock := &permission.stock[index]
		if stock.challenge != challenge || len(stock.tokens) == 0 {
			continue
		}
		token := stock.tokens[0]
		stock.tokens[0] = nil
		stock.tokens = stock.tokens[1:]
		if err := credential.VerifyClosedToken(challenge, spki, token); err != nil {
			clear(token)
			return nil, textTokenTransferFailureAt("verification", err)
		}
		return token, nil
	}
	return nil, textTokenTransferFailureAt("stock", errors.New("text forwarding token stock unavailable"))
}
