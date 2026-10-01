//go:build linux

package endpoint

import (
	"context"
	"errors"
	"path/filepath"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/route/carrier"
)

// PermissionFiles is a trusted local offline handover, never Application
// input. Its signed response is checked against this process's exact request.
type PermissionFiles struct {
	RequestPath  string
	ResponsePath string
	Maxima       [3]uint32
}

// ClosedParticipantConfig owns one closed text participant generation. It has
// no legacy Invite, alternate protection mode, Target, or private signing key.
type ClosedParticipantConfig struct {
	ReaderOnly                                             bool
	Network                                                state.Config
	RefreshNetwork                                         bool
	EntryRoot, LocalRoleRoot, TokenRoot                    string
	PublicationRoot, ServiceInstanceRoot                   string
	ApplicationAddress, AdministrationAddress              string
	BrokerID, ConnectionPrincipal, AdministrationPrincipal [32]byte
	ReaderPermission, PublisherPermission                  PermissionFiles
	Clock                                                  func() time.Time
	Observe                                                func(context.Context, ClosedParticipantEvent) error
}

// ClosedParticipantEvent exposes only local lifecycle or the public commitment
// for an offline approval. Observers must honor their context and join output.
type ClosedParticipantEvent struct {
	At                                        time.Time
	Kind                                      string
	NetworkID                                 [32]byte
	Surface                                   string
	Failure                                   string
	RequestDigest                             [32]byte
	ApplicationAddress, AdministrationAddress string
}

func (config ClosedParticipantConfig) validate() error {
	if config.Network.AcceptedProfile != carrier.ClosedRouteProfile || config.Network.NetworkID == [32]byte{} || config.BrokerID == [32]byte{} || config.ConnectionPrincipal == [32]byte{} || !config.ReaderOnly && config.AdministrationPrincipal == [32]byte{} || config.Observe == nil {
		return errors.New("text participant configuration incomplete")
	}
	paths := []string{config.Network.Root, config.EntryRoot, config.LocalRoleRoot, config.TokenRoot, config.ApplicationAddress, config.ReaderPermission.RequestPath, config.ReaderPermission.ResponsePath}
	permissions := []PermissionFiles{config.ReaderPermission}
	if config.ReaderOnly {
		if config.PublicationRoot != "" || config.ServiceInstanceRoot != "" || config.AdministrationAddress != "" || config.AdministrationPrincipal != [32]byte{} || config.PublisherPermission != (PermissionFiles{}) {
			return errors.New("reader participant cannot select Publisher inputs")
		}
	} else {
		paths = append(paths, config.PublicationRoot, config.ServiceInstanceRoot, config.AdministrationAddress, config.PublisherPermission.RequestPath, config.PublisherPermission.ResponsePath)
		permissions = append(permissions, config.PublisherPermission)
	}
	seen := make(map[string]bool)
	for _, path := range paths {
		if !filepath.IsAbs(path) || filepath.Clean(path) != path || seen[path] {
			return errors.New("text participant paths must be distinct, absolute and canonical")
		}
		seen[path] = true
	}
	for index, permission := range permissions {
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

// RunClosedParticipant qualifies and provisions each selected context before
// exposing local commands, retaining State and worker cleanup owners. A Reader
// owns no Instance or Publication; the default also retains those owners.
func RunClosedParticipant(ctx context.Context, config ClosedParticipantConfig) error {
	if ctx == nil {
		return errors.New("text participant context unavailable")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := config.validate(); err != nil {
		return err
	}
	return runClosedParticipant(ctx, config)
}
