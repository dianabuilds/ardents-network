//go:build linux

package endpoint

import (
	"context"
	"errors"
	"path/filepath"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/route"
)

// TextPermissionFiles is a trusted local offline handover, never Application
// input. Its signed response is checked against this process's exact request.
type TextPermissionFiles struct {
	RequestPath  string
	ResponsePath string
	Maxima       [3]uint32
}

// TextParticipantConfig owns one closed text participant generation. It has
// no legacy Invite, alternate protection mode, Target, or private signing key.
type TextParticipantConfig struct {
	Network                                                state.Config
	RefreshNetwork                                         bool
	EntryRoot, LocalRoleRoot, TokenRoot                    string
	PublicationRoot, ServiceInstanceRoot                   string
	ApplicationAddress, AdministrationAddress              string
	BrokerID, ConnectionPrincipal, AdministrationPrincipal [32]byte
	ReaderPermission, PublisherPermission                  TextPermissionFiles
	Clock                                                  func() time.Time
	Observe                                                func(context.Context, TextParticipantEvent) error
}

// TextParticipantEvent exposes only local lifecycle or the public commitment
// for an offline approval. Observers must honor their context and join output.
type TextParticipantEvent struct {
	Kind                                      string
	NetworkID                                 [32]byte
	Surface                                   string
	Failure                                   string
	RequestDigest                             [32]byte
	ApplicationAddress, AdministrationAddress string
}

func (config TextParticipantConfig) validate() error {
	if config.Network.AcceptedProfile != route.ClosedRouteProfile || config.Network.NetworkID == [32]byte{} || config.BrokerID == [32]byte{} || config.ConnectionPrincipal == [32]byte{} || config.AdministrationPrincipal == [32]byte{} || config.Observe == nil {
		return errors.New("text participant configuration incomplete")
	}
	seen := make(map[string]bool)
	for _, path := range []string{config.Network.Root, config.EntryRoot, config.LocalRoleRoot, config.TokenRoot, config.PublicationRoot, config.ServiceInstanceRoot, config.ApplicationAddress, config.AdministrationAddress, config.ReaderPermission.RequestPath, config.ReaderPermission.ResponsePath, config.PublisherPermission.RequestPath, config.PublisherPermission.ResponsePath} {
		if !filepath.IsAbs(path) || filepath.Clean(path) != path || seen[path] {
			return errors.New("text participant paths must be distinct, absolute and canonical")
		}
		seen[path] = true
	}
	for index, permission := range []TextPermissionFiles{config.ReaderPermission, config.PublisherPermission} {
		maximum := uint64(4096)
		if index == 1 {
			maximum = 16384
		}
		total := uint64(permission.Maxima[0]) + uint64(permission.Maxima[1]) + uint64(permission.Maxima[2])
		if total == 0 || total > maximum {
			return errors.New("text participant allocation unavailable")
		}
	}
	return nil
}

// RunTextParticipant qualifies and provisions both contexts before exposing
// local commands, retaining the State, Instance and worker cleanup owners.
func RunTextParticipant(ctx context.Context, config TextParticipantConfig) error {
	if ctx == nil {
		return errors.New("text participant context unavailable")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := config.validate(); err != nil {
		return err
	}
	return runTextParticipant(ctx, config)
}
