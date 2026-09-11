//go:build linux

package endpoint

import (
	"context"
	"crypto/ed25519"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
	"github.com/dianabuilds/ardents-network/internal/application/interfacev1/administration"
	"github.com/dianabuilds/ardents-network/internal/application/interfacev2/connection"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/service/instance"
)

func runTextParticipant(ctx context.Context, config TextParticipantConfig) (outcome error) {
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
	root, err := instance.Open(config.ServiceInstanceRoot)
	if err != nil {
		return err
	}
	defer func() { outcome = errors.Join(outcome, root.Close()) }()
	credential, err := root.Credential()
	if err != nil || credential.NetworkID != config.Network.NetworkID {
		return errors.Join(errors.New("text participant Instance unavailable"), err)
	}
	owner, err := newEndpoint(setup{NetworkID: config.Network.NetworkID, BrokerID: config.BrokerID, ConnectionPrincipal: config.ConnectionPrincipal, AdministrationPrincipal: config.AdministrationPrincipal, PublicationRoot: config.PublicationRoot, AuthorityPublic: ed25519.PublicKey(credential.AuthorityPublic[:]), IntroductionPublic: ed25519.PublicKey(credential.IntroductionHPKEPublic[:]), Clock: clock})
	if err != nil {
		return err
	}
	defer func() { outcome = errors.Join(outcome, owner.Close()) }()
	owner.closedState = network
	owner.closedEntryRoot, owner.closedRoleRoot, owner.closedTokenRoot = config.EntryRoot, config.LocalRoleRoot, config.TokenRoot
	floor, err := owner.publications.Floor()
	if err != nil {
		return err
	}
	binding, err := root.OpenBinding(floor)
	if err != nil {
		return err
	}
	owner.publisherBinding = binding
	if _, err := owner.textTokenJournal(); err != nil {
		return err
	}
	if _, err := owner.textEntrySets(); err != nil {
		return err
	}
	return owner.runTextInterfaces(ctx, config)
}

func (endpoint *endpoint) runTextInterfaces(ctx context.Context, config TextParticipantConfig) (outcome error) {
	var contexts [2]*textContext
	for index, role := range []struct {
		principal [32]byte
		surface   broker.Surface
		files     TextPermissionFiles
	}{{config.ConnectionPrincipal, broker.Connection, config.ReaderPermission}, {config.AdministrationPrincipal, broker.Administration, config.PublisherPermission}} {
		capability, err := endpoint.Admit(role.principal, role.surface)
		if err != nil {
			return err
		}
		owner, err := endpoint.beginTextContext(ctx, capability, role.principal, role.surface)
		if err != nil {
			return err
		}
		defer func() { outcome = errors.Join(outcome, owner.Close()) }()
		contexts[index] = owner
		if err := owner.provisionTextPermission(ctx, role.files.RequestPath, role.files.ResponsePath, role.files.Maxima, func(reportCtx context.Context, digest [32]byte) error {
			return config.Observe(reportCtx, TextParticipantEvent{Kind: "permission-required", NetworkID: endpoint.network, Surface: string(role.surface), RequestDigest: digest})
		}); err != nil {
			return err
		}
		if role.surface == broker.Administration {
			owner.mu.Lock()
			owner.refreshFailure = func(failure string) {
				// A failed refresh is local operational state. Its fixed category
				// exposes neither a wrapped transport error nor private route data.
				_ = config.Observe(context.Background(), TextParticipantEvent{Kind: "publication-refresh-failed", NetworkID: endpoint.network, Failure: failure})
			}
			owner.mu.Unlock()
		}
	}
	for _, owner := range contexts {
		owner.mu.Lock()
		profile, now, err := owner.textPermissionProfileLocked()
		permission := owner.permission
		current := err == nil && permission != nil && permission.profile == profile && permission.accepted.Signature != [64]byte{} && !now.Before(permission.accepted.NotBefore) && now.Before(permission.accepted.NotAfter)
		owner.mu.Unlock()
		if !current {
			return errors.New("text participant permission expired before command exposure")
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	reader, err := contexts[0].openTextConnection()
	if err != nil {
		return err
	}
	defer func() { outcome = errors.Join(outcome, reader.Close()) }()
	publisher, err := contexts[1].openTextAdministration()
	if err != nil {
		return err
	}
	defer func() { outcome = errors.Join(outcome, publisher.Close()) }()
	readServer, err := connection.Listen(config.ApplicationAddress, reader)
	if err != nil {
		return err
	}
	defer func() { outcome = errors.Join(outcome, readServer.Close()) }()
	adminServer, err := administration.Listen(config.AdministrationAddress, publisher)
	if err != nil {
		return err
	}
	defer func() { outcome = errors.Join(outcome, adminServer.Close()) }()
	if err := config.Observe(ctx, TextParticipantEvent{Kind: "ready", NetworkID: endpoint.network, ApplicationAddress: config.ApplicationAddress, AdministrationAddress: config.AdministrationAddress}); err != nil {
		return err
	}
	<-ctx.Done()
	return nil
}
