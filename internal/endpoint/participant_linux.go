//go:build linux

package endpoint

import (
	"context"
	"crypto/ed25519"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/service/instance"
)

func withParticipant(ctx context.Context, config ClosedParticipantConfig, run func(*endpoint) error) error {
	return useParticipant(ctx, config, true, run)
}

func inspectParticipant(ctx context.Context, config ClosedParticipantConfig, run func(*endpoint) error) error {
	return useParticipant(ctx, config, false, run)
}

func useParticipant(ctx context.Context, config ClosedParticipantConfig, withdrawBinding bool, run func(*endpoint) error) (outcome error) {
	clock := config.Clock
	if clock == nil {
		clock = time.Now
	}
	config.Network.Clock = clock
	network, err := state.Open(config.Network)
	if err != nil {
		return err
	}
	defer func() { outcome = errors.Join(outcome, network.Close()) }()
	if config.RefreshNetwork {
		if _, err := network.Refresh(ctx); err != nil {
			return err
		}
	}
	if _, err := network.CurrentClosedRoute(); err != nil {
		return err
	}
	input := setup{NetworkID: config.Network.NetworkID, BrokerID: config.BrokerID, ConnectionPrincipal: config.ConnectionPrincipal, AdministrationPrincipal: config.AdministrationPrincipal, PublicationRoot: config.PublicationRoot, Clock: clock}
	var root *instance.Root
	if !config.ReaderOnly {
		root, err = instance.Open(config.ServiceInstanceRoot)
		if err != nil {
			return err
		}
		defer func() { outcome = errors.Join(outcome, root.Close()) }()
		credential, err := root.Credential()
		if err != nil || credential.NetworkID != config.Network.NetworkID {
			return errors.Join(errors.New("text participant Instance unavailable"), err)
		}
		input.AuthorityPublic = ed25519.PublicKey(credential.AuthorityPublic[:])
	}
	owner, err := newEndpoint(input)
	if err != nil {
		return err
	}
	defer func() { outcome = errors.Join(outcome, owner.Close()) }()
	owner.closedState = network
	owner.closedEntryRoot, owner.closedRoleRoot, owner.closedTokenRoot = config.EntryRoot, config.LocalRoleRoot, config.TokenRoot
	if root != nil {
		floor, err := owner.publications.Floor()
		if err != nil {
			return err
		}
		binding, err := root.OpenBinding(floor)
		if err != nil {
			return err
		}
		owner.publisherBinding = binding
		if !withdrawBinding {
			defer func() {
				owner.publisherMu.Lock()
				if owner.publisherBinding == binding {
					owner.publisherBinding = nil
				}
				owner.publisherMu.Unlock()
			}()
		}
	}
	if _, err := owner.tokenJournal(); err != nil {
		return err
	}
	if _, err := owner.dutyEntrySets(); err != nil {
		return err
	}
	return run(owner)
}
