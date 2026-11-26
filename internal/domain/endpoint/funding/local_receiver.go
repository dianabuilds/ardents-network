package funding

import (
	"context"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/domain/admission/receiving"
	"github.com/dianabuilds/ardents-network/internal/domain/network/state"
	"github.com/dianabuilds/ardents-network/internal/pathcheck"
)

var (
	errInvalidLocalReceiver  = errors.New("invalid local receiver configuration")
	errLocalReceiverCanceled = errors.New("local receiver canceled before opening")
)

// localReceiver retains one Receiving owner and its original authority.
type localReceiver struct {
	allowance *receiving.LocalAllowance
	authority AdmissionAuthority
}

// openLocalReceiver validates and opens the original authority and allowance.
func openLocalReceiver(ctx context.Context, root, profile string, network *state.AuthorityDeclaration, receiver receiving.Receiver, end time.Time) (*localReceiver, error) {
	if ctx == nil || !pathcheck.IsCleanAbsolute(root) || !ValidAdmissionAuthority(profile, network, root) {
		return nil, errInvalidLocalReceiver
	}
	if err := ctx.Err(); err != nil {
		return nil, errors.Join(errLocalReceiverCanceled, err)
	}
	authority, err := OpenAdmissionAuthority(profile, network)
	if err != nil {
		return nil, err
	}
	allowance, err := receiving.OpenLocalAllowance(root, receiver, func() (receiving.Observation, error) {
		return authority.Receiver(receiver, end)
	})
	if err != nil {
		return nil, errors.Join(err, authority.Close())
	}
	return &localReceiver{allowance: allowance, authority: authority}, nil
}

// closeAuthority releases the original authority after Receiving has joined.
func (receiver *localReceiver) closeAuthority() error { return receiver.authority.Close() }
