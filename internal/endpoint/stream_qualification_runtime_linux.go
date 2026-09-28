//go:build linux

package endpoint

import (
	"context"
	"errors"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
	"github.com/dianabuilds/ardents-network/internal/application/streamqualification"
	"github.com/dianabuilds/ardents-network/internal/qualification"
)

// StreamQualificationConfig belongs only to the qualification runner. It
// contains normal trusted participant configuration, never a worker selector,
// privilege receipt, network bypass or caller-provided worker identity.
type StreamQualificationConfig struct {
	Participant  ClosedParticipantConfig
	Role         streamqualification.Role
	Profile      streamqualification.Profile
	Condition    streamqualification.NetworkCondition
	Seed         [32]byte
	ReaderIndex  int
	Link         string
	HostingRoot  string
	Measurements *qualification.Measurements
	Observe      func(context.Context, StreamQualificationEvent) error
}

// StreamQualificationEvent is the scenario observation owned by
// internal/qualification.
type StreamQualificationEvent = qualification.Event

// RunStreamQualification exercises the real installed-worker and Service
// boundary for one fixed workload. Its only command caller is the separate
// qualification runner; ordinary text commands cannot reach this operation.
// Endpoint composes and validates the authorized participant and binds launch
// authority to one Session; the scenario owner in internal/qualification
// drives the measurement through it.
func RunStreamQualification(ctx context.Context, config StreamQualificationConfig) (streamqualification.Report, error) {
	scenario := qualification.Scenario{
		Role:                config.Role,
		Profile:             config.Profile,
		Condition:           config.Condition,
		Seed:                config.Seed,
		ReaderIndex:         config.ReaderIndex,
		Link:                config.Link,
		NetworkID:           config.Participant.Network.NetworkID,
		HostingRoot:         config.HostingRoot,
		Measurements:        config.Measurements,
		Observe:             config.Observe,
		ValidateParticipant: config.Participant.validate,
	}
	scenario.WithSession = func(lifetime context.Context, run *qualification.Run, use func(qualification.Session) error) error {
		return withParticipant(lifetime, config.Participant, func(endpoint *endpoint) (operationErr error) {
			principal, surface, permission := config.Participant.ConnectionPrincipal, broker.Connection, config.Participant.ReaderPermission
			if config.Role == streamqualification.PublisherRole {
				principal, surface, permission = config.Participant.AdministrationPrincipal, broker.Administration, config.Participant.PublisherPermission
			}
			capability, err := endpoint.Admit(principal, surface)
			if err != nil {
				return err
			}
			owner, err := endpoint.beginDutyContext(lifetime, capability, principal, surface)
			if err != nil {
				return err
			}
			defer func() { operationErr = errors.Join(operationErr, owner.Close()) }()
			worker, err := owner.launchStreamQualificationWorker(lifetime, run)
			if err != nil {
				return err
			}
			defer func() { operationErr = errors.Join(operationErr, worker.Close()) }()
			session := &qualificationSession{
				qualifiedWorker: worker,
				owner:           owner,
				surface:         surface,
				permission:      permission,
				reportPermission: func(reportCtx context.Context, digest [32]byte) error {
					return config.Participant.Observe(reportCtx, ClosedParticipantEvent{Kind: "permission-required", NetworkID: endpoint.network, Surface: string(surface), RequestDigest: digest})
				},
			}
			return use(session)
		})
	}
	return qualification.RunScenario(ctx, scenario)
}
