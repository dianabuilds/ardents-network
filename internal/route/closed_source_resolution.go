//go:build linux

package route

import (
	"context"
	"crypto/rand"
	"errors"

	"github.com/dianabuilds/ardents-network/internal/route/ardp"
	"github.com/dianabuilds/ardents-network/internal/route/terminal"
)

// ResolutionRecipient derives the sole private resolution duty from the live
// opened State. It returns no address or transport key to its Endpoint caller.
func (prefix *ClosedSourcePrefix) ResolutionRecipient() ([32]byte, error) {
	peer, err := prefix.resolutionPeer()
	return peer.node, err
}

func (prefix *ClosedSourcePrefix) resolutionPeer() (closedBootstrapPeer, error) {
	if prefix == nil || prefix.plan.domain != 1 {
		return closedBootstrapPeer{}, errors.New("resolution requires the Source role")
	}
	return prefix.terminalPeer(ardp.PurposeReachability)
}
func (prefix *ClosedSourcePrefix) currentControl(purpose ardp.Purpose, peer closedBootstrapPeer) error {
	if err := prefix.plan.current(prefix.source, prefix.selection); err != nil {
		return err
	}
	switch purpose {
	case ardp.PurposeIssuer:
		if prefix.plan.domain == 1 && peer == prefix.plan.peers[2] {
			return nil
		}
	case ardp.PurposeSubmission:
		current, err := prefix.submissionPeer()
		if err == nil && current == peer {
			return nil
		}
	case ardp.PurposeReachability:
		current, err := prefix.resolutionPeer()
		if err == nil && current == peer {
			return nil
		}
	}
	return errors.New("closed Control recipient changed")
}

// ExchangeDescriptor uses a fresh confidential Control lane for one exact
// lookup or publication. Endpoint still verifies the returned signed proof.
func (prefix *ClosedSourcePrefix) ExchangeDescriptor(ctx context.Context, present ClosedTokenPresenter, target [32]byte, descriptor []byte) (uint8, []byte, error) {
	if (target == [32]byte{}) == (len(descriptor) == 0) {
		return 0, nil, errors.New("closed Descriptor operation is ambiguous")
	}
	peer, err := prefix.resolutionPeer()
	if err != nil {
		return 0, nil, err
	}
	var nonce [32]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return 0, nil, err
	}
	var operation []byte
	if len(descriptor) == 0 {
		operation, err = terminal.EncodeDescriptorLookup(nonce, target)
	} else {
		operation, err = terminal.EncodeDescriptorPublication(nonce, descriptor)
	}
	if err != nil {
		return 0, nil, err
	}
	defer clear(operation)
	body, err := prefix.exchangeControl(ctx, ardp.PurposeReachability, peer, present, operation)
	if err != nil {
		return 0, nil, err
	}
	defer clear(body)
	status, proof, err := terminal.DecodeDescriptorResult(body, nonce)
	if err == nil && status == 0 && (len(descriptor) == 0) != (len(proof) != 0) {
		return 0, nil, errors.New("closed Descriptor result has wrong operation payload")
	}
	return status, proof, err
}
