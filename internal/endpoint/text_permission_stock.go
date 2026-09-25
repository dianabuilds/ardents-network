//go:build linux

package endpoint

import (
	"errors"
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
	if permission.profile != profile || now.Before(permission.accepted.NotBefore) || !now.Before(permission.accepted.NotAfter) {
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
