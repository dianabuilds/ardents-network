package authority

import (
	"context"
	"time"

	identityapi "ardents/internal/identity"
)

type InspectChannelRequest struct {
	Version   uint32
	RealmID   string
	ChannelID [16]byte
}

type ChannelStatus struct {
	Version           uint32                      `json:"version"`
	RealmID           string                      `json:"realm_id"`
	ChannelClass      identityapi.CapabilityScope `json:"channel_class"`
	CurrentGeneration uint32                      `json:"current_generation"`
	MemberCount       uint32                      `json:"member_count"`
	Readiness         string                      `json:"readiness"`
	Reason            string                      `json:"reason,omitempty"`
	GrantNotAfter     time.Time                   `json:"grant_not_after"`
	RenewBy           time.Time                   `json:"renew_by"`
	PendingGeneration uint32                      `json:"pending_generation,omitempty"`
}

func (s *Service) InspectChannel(
	ctx context.Context,
	command Command,
	request InspectChannelRequest,
) (ChannelStatus, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if request.Version != ContractVersion ||
		!ValidRealmID(request.RealmID) || zeroFixedID(request.ChannelID) {
		return ChannelStatus{}, ErrInvalidArgument
	}
	if command.Actor == "" || command.Actor != command.Effective ||
		command.Action != ActionInspect ||
		command.ResourceKind != ResourceKindChannel ||
		command.ResourceID != ChannelResource(request.RealmID, request.ChannelID) {
		return ChannelStatus{}, ErrPermissionDenied
	}
	if s.store == nil || s.repository == nil {
		return ChannelStatus{}, ErrUnavailable
	}
	state, found, err := s.store.Load(ctx)
	if err != nil || !found {
		return ChannelStatus{}, ErrUnavailable
	}
	if err := validateLedger(state); err != nil {
		return ChannelStatus{}, ErrRecoveryRequired
	}
	if state.RealmID != request.RealmID {
		return ChannelStatus{}, ErrPermissionDenied
	}
	if state.Phase == PhaseCheckpointing {
		if err := s.reconcileLoaded(ctx, &state); err != nil {
			return ChannelStatus{}, err
		}
	}
	index := channelRecordIndex(state, request.ChannelID)
	if index < 0 {
		return ChannelStatus{}, ErrInvalidArgument
	}
	return channelStatusAt(state, state.Channels[index], s.clock().UTC().Truncate(time.Second)), nil
}

func channelStatusAt(state Ledger, channel ChannelRecord, now time.Time) ChannelStatus {
	notAfter := channelCurrentGrants(channel)[0].NotAfter
	for _, grant := range channelCurrentGrants(channel)[1:] {
		if grant.NotAfter.Before(notAfter) {
			notAfter = grant.NotAfter
		}
	}
	status := ChannelStatus{
		Version: ContractVersion, RealmID: state.RealmID,
		ChannelClass:      identityapi.CapabilityScope(channel.Class),
		CurrentGeneration: channel.CurrentGeneration,
		MemberCount:       channel.MemberCount, Readiness: ReadinessReady,
		GrantNotAfter: notAfter, RenewBy: notAfter.Add(-GrantRenewalThreshold),
	}
	switch {
	case !now.Before(notAfter):
		status.Readiness, status.Reason = ReadinessUnavailable, ReasonChannelGrantExpired
	case channel.PendingGenerationCount != 0:
		status.Readiness, status.Reason = ReadinessDegraded, ReasonChannelGrantPending
		status.PendingGeneration = channel.CurrentGeneration + 1
	case !now.Before(status.RenewBy):
		status.Readiness, status.Reason = ReadinessDegraded, ReasonChannelGrantRenewalDue
	}
	return status
}
