//go:build linux

package endpoint

import (
	"context"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/endpoint/tokenjournal"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/route/ardp"
)

type tokenTransferFailure struct {
	stage string
	cause error
}

func (failure *tokenTransferFailure) Error() string { return failure.cause.Error() }

func (failure *tokenTransferFailure) Unwrap() error { return failure.cause }

func tokenTransferFailureAt(stage string, cause error) error {
	return &tokenTransferFailure{stage: stage, cause: cause}
}

func tokenTransferFailureStage(cause error) string {
	var failure *tokenTransferFailure
	if errors.As(cause, &failure) && failure.stage != "" {
		return failure.stage
	}
	return "unknown"
}

// takeTokenLocked is shared only after the exact opening or issuance
// flight has independently authorized its role. It durably marks consumed stock
// and rechecks the surviving context before releasing bytes to Route.
func (owner *textContext) takeTokenLocked(profile state.ClosedProfileView, now time.Time, hello ardp.Hello, class uint8, attempt context.Context) ([]byte, error) {
	permission := owner.tokens.permission
	token, err := permission.consumeToken(profile, now, hello, class)
	if err != nil {
		return nil, err
	}
	journal, err := owner.endpoint.tokenJournal()
	if err == nil {
		err = journal.Mark(token, tokenjournal.Attempt{Profile: profile.Digest, Receiver: hello.RecipientNodeID, Duty: hello.RecipientDutyGeneration,
			Window: permission.accepted.NotBefore, Class: class, Nonce: hello.ChannelNonce})
	}
	if err != nil {
		clear(token)
		owner.closeErr = errors.Join(owner.closeErr, err)
		owner.closed = true
		owner.endpoint.failTextContexts(err)
		return nil, tokenTransferFailureAt("journal", err)
	}
	currentProfile, currentTime, currentErr := owner.permissionProfileLocked()
	if currentErr != nil || currentProfile != profile || !currentTime.Before(permission.accepted.NotAfter) || attempt.Err() != nil {
		clear(token)
		return nil, tokenTransferFailureAt("owner", errors.Join(currentErr, attempt.Err(), errors.New("text token owner changed after durable mark")))
	}
	return token, nil
}
