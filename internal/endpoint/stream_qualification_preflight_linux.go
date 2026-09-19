//go:build linux

package endpoint

import (
	"context"
	"errors"
	"os"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/streamqualification"
	"github.com/dianabuilds/ardents-network/internal/network/duty"
	"github.com/dianabuilds/ardents-network/internal/resource"
)

// StreamQualificationPreflight identifies the State-owned data path retained
// before any worker, admission request, or network effect begins.
type StreamQualificationPreflight struct {
	Role                        streamqualification.Role
	EntryNodeID, InteriorNodeID [32]byte
}

func prepareStreamQualificationParticipantRoots(config TextParticipantConfig) (outcome error) {
	clock := config.Clock
	if clock == nil {
		clock = time.Now
	}
	roles, err := duty.Open(duty.Config{Root: config.LocalRoleRoot, Clock: clock, Create: true})
	if err != nil {
		return err
	}
	defer func() { outcome = errors.Join(outcome, roles.Close()) }()
	if err := os.Mkdir(config.TokenRoot, 0o700); err != nil && !errors.Is(err, os.ErrExist) {
		return err
	}
	return nil
}

// PreflightStreamQualification validates one installed participant without
// starting a worker, issuing authority requests, or opening a network Route.
// It is the cheap fail-fast boundary used before a two-host measured attempt.
func PreflightStreamQualification(ctx context.Context, config StreamQualificationConfig) (result StreamQualificationPreflight, outcome error) {
	if ctx == nil || ctx.Err() != nil || config.HostingRoot == "" {
		return result, errors.New("qualification preflight configuration unavailable")
	}
	if _, err := config.Profile.Definition(config.Role); err != nil {
		return result, err
	}
	if _, err := config.Condition.CarrierRatioLimit(); err != nil {
		return result, err
	}
	if config.Participant.Observe == nil {
		config.Participant.Observe = func(context.Context, TextParticipantEvent) error { return nil }
	}
	if err := config.Participant.validate(); err != nil {
		return result, err
	}
	artifact, err := loadInstalledWorkerArtifact(streamInventory)
	if err != nil {
		return result, err
	}
	if err := artifact.verify(); err != nil {
		return result, err
	}
	hosting, err := resource.OpenHosting(config.HostingRoot)
	if err != nil {
		return result, err
	}
	defer func() { outcome = errors.Join(outcome, hosting.Close()) }()
	if _, err := hosting.Sample(ctx, 0); err != nil {
		return result, err
	}
	if err := prepareStreamQualificationParticipantRoots(config.Participant); err != nil {
		return result, err
	}
	// Preflight must not create a refresh wave. The already accepted State and
	// its live clock observation are checked by the normal participant owner.
	config.Participant.Network.AutomaticRefreshInterval = 0
	result.Role = config.Role
	return result, inspectTextParticipant(ctx, config.Participant, func(endpoint *endpoint) error {
		domain := uint8(1)
		if config.Role == streamqualification.PublisherRole {
			domain = 3
		}
		owner := &textContext{textContextState: textContextState{endpoint: endpoint}}
		owner.mu.Lock()
		selection, err := owner.selectTextAdjacentLocked(domain, &owner.sourceSet)
		owner.mu.Unlock()
		if err != nil {
			return err
		}
		result.EntryNodeID, result.InteriorNodeID = selection.EntryNodeID, selection.InteriorNodeID
		return nil
	})
}
