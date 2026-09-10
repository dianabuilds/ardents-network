//go:build linux

package endpoint

import (
	"context"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/route/credential"
)

// takeTextTokenLocked is shared only after the exact opening or issuance
// flight has independently authorized its role. It burns stock before journal
// append and rechecks the surviving owner before releasing bytes to Route.
func (owner *textContext) takeTextTokenLocked(profile state.ClosedProfileView, now time.Time, hello route.ClosedHello, class uint8, attempt context.Context) ([]byte, error) {
	permission := owner.permission
	if permission.profile != profile || now.Before(permission.accepted.NotBefore) || !now.Before(permission.accepted.NotAfter) {
		return nil, errors.New("text token permission expired")
	}
	challenge := credential.ClosedTokenContext{NetworkID: profile.NetworkID, ProfileDigest: profile.Digest, IssuerNodeID: profile.IssuerNodeID,
		ReceiverNodeID: hello.RecipientNodeID, ReceiverDutyGeneration: hello.RecipientDutyGeneration, Class: class, WindowStart: permission.accepted.NotBefore}
	if int(profile.TokenKeyCount) > len(profile.TokenKeys) {
		return nil, errors.New("text token key inventory unavailable")
	}
	var spki []byte
	for _, key := range profile.TokenKeys[:profile.TokenKeyCount] {
		if key.Class == class && key.WindowStart == challenge.WindowStart {
			if spki != nil {
				return nil, errors.New("text token key ambiguous")
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
			return nil, err
		}
		journal, err := owner.endpoint.textTokenJournal()
		if err == nil {
			err = journal.mark(token, textTokenAttempt{profile: profile.Digest, receiver: hello.RecipientNodeID, duty: hello.RecipientDutyGeneration,
				window: challenge.WindowStart, class: class, attempt: hello.ChannelNonce})
		}
		if err != nil {
			clear(token)
			owner.closeErr = errors.Join(owner.closeErr, err)
			owner.closed = true
			owner.endpoint.failTextContexts(err)
			return nil, err
		}
		currentProfile, currentTime, currentErr := owner.textPermissionProfileLocked()
		if currentErr != nil || currentProfile != profile || !currentTime.Before(permission.accepted.NotAfter) || attempt.Err() != nil {
			clear(token)
			return nil, errors.New("text token owner changed after durable mark")
		}
		return token, nil
	}
	return nil, errors.New("text forwarding token stock unavailable")
}
