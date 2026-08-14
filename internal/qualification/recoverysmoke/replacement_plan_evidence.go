package recoverysmoke

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
)

var replacementPlanRoles = [...]string{
	"client", "initiator", "introduction", "rendezvous", "responder", "publisher",
}

func (observer dockerObserver) bindReplacementPlanTimings(ctx context.Context, cell *replacementCell,
	clientRaw []byte) error {
	logs := map[string][]byte{"client": clientRaw}
	for proposalIndex := range cell.Proposals {
		proposal := &cell.Proposals[proposalIndex]
		proposal.PlanTimings = make(map[string]routePlanTiming, len(replacementPlanRoles))
		for _, role := range replacementPlanRoles {
			service, process, err := replacementPlanProcess(cell, proposal, role)
			if err != nil {
				return err
			}
			if !proposal.Committed && role == "rendezvous" {
				prior, ok := cell.Proposals[proposalIndex-1].PlanTimings[role]
				if !ok || prior.Process != process {
					return errors.New("stopped Rendezvous plan timing evidence is unbound")
				}
				proposal.PlanTimings[role] = prior
				continue
			}
			raw, ok := logs[service]
			if !ok {
				raw, err = observer.compose(ctx, time.Minute, "logs", "--no-color", "--no-log-prefix", service)
				if err != nil {
					return fmt.Errorf("read %s Route plan evidence: %w", role, err)
				}
				logs[service] = raw
			}
			localAttachment := replacementLocalAttachment(cell.Proposals, proposalIndex, role, process)
			timing, err := parseRoutePlanTiming(raw, role, localAttachment)
			if err != nil {
				return err
			}
			timing.Process = process
			proposal.PlanTimings[role] = timing
		}
	}
	return nil
}

func replacementLocalAttachment(proposals []replacementProposal, current int, role string,
	process processEvidenceRef) uint32 {
	if role == "client" || role == "publisher" {
		return proposals[current].Attachment
	}
	var result uint32
	for proposalIndex := 0; proposalIndex <= current; proposalIndex++ {
		_, candidate, err := replacementPlanProcess(nil, &proposals[proposalIndex], role)
		if err == nil && candidate == process {
			result++
		}
	}
	return result
}

func replacementPlanProcess(cell *replacementCell, proposal *replacementProposal,
	role string) (string, processEvidenceRef, error) {
	if role == "client" || role == "publisher" {
		if cell == nil {
			return "", processEvidenceRef{}, errors.New(role + " Route process evidence is missing")
		}
		process, ok := cell.HostProcesses[role]
		if !ok {
			return "", processEvidenceRef{}, errors.New(role + " Route process evidence is missing")
		}
		return role, process.Host, nil
	}
	for index, candidateRole := range replacementRoles {
		if role == candidateRole {
			process := proposal.Processes[index]
			return process.Service, process.Host, nil
		}
	}
	return "", processEvidenceRef{}, errors.New("replacement Route plan role is invalid")
}

func parseRoutePlanTiming(raw []byte, role string, attachment uint32) (routePlanTiming, error) {
	var result routePlanTiming
	seen := make(map[string]bool, 2)
	for _, line := range splitLines(raw) {
		var value route.Evidence
		if err := json.Unmarshal(line, &value); err != nil {
			return routePlanTiming{}, fmt.Errorf("decode %s Route plan evidence: %w", role, err)
		}
		if value.Kind != "complete" && value.Kind != "ready" || value.Role != role || value.Attachment != attachment {
			continue
		}
		if seen[value.Kind] {
			return routePlanTiming{}, errors.New(role + " Route plan timing evidence is duplicated")
		}
		seen[value.Kind] = true
		observed := routePlanTiming{Attachment: value.Attachment,
			DeadlineMillis: value.DeadlineMillis, LifetimeMillis: value.LifetimeMillis}
		if result.Attachment != 0 && result != observed {
			return routePlanTiming{}, errors.New(role + " Route plan timing observations disagree")
		}
		result = observed
	}
	if result.Attachment == 0 || result.DeadlineMillis == 0 || result.LifetimeMillis == 0 {
		return routePlanTiming{}, errors.New(role + " Route plan timing evidence is incomplete or duplicated")
	}
	return result, nil
}
