package recoverysmoke

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
)

func (observer dockerObserver) observeCandidateProcess(ctx context.Context, service string,
	candidate route.Position) (candidateProcess, error) {
	container, err := observer.serviceID(ctx, service)
	if err != nil {
		return candidateProcess{}, err
	}
	raw, err := observer.docker(ctx, 10*time.Second, "inspect", "--format",
		`{"id":"{{.Id}}","pid":{{.State.Pid}},"started":"{{.State.StartedAt}}","running":{{.State.Running}}}`, container)
	if err != nil {
		return candidateProcess{}, err
	}
	var value struct {
		ID, Started string
		PID         uint32
		Running     bool
	}
	if err := json.Unmarshal(raw, &value); err != nil {
		return candidateProcess{}, errors.Join(err, errors.New("candidate process inspection is invalid"))
	}
	incarnation, err := parseProcessIdentity(container, []byte(value.ID+" "+value.Started))
	if err != nil || !value.Running || value.PID == 0 || value.ID != container {
		return candidateProcess{}, errors.Join(err, errors.New("candidate process is not live and exact"))
	}
	return candidateProcess{Service: service, ContainerID: container, Incarnation: incarnation,
		PID: value.PID, NodeID: candidate.NodeID, PublicKey: candidate.PublicKey}, nil
}

func (observer dockerObserver) observeRouteGeneration(ctx context.Context, fixture prepared,
	generation uint64, selection selectedRoute) (routeGeneration, error) {
	result := routeGeneration{Generation: generation,
		Processes: make(map[string]candidateProcess, len(replacementRoles))}
	for _, role := range replacementRoles {
		service, err := candidateService(fixture.candidates, selection[role])
		if err != nil {
			return routeGeneration{}, err
		}
		process, err := observer.observeCandidateProcess(ctx, service, selection[role])
		if err != nil {
			return routeGeneration{}, err
		}
		result.Processes[role] = process
	}
	return result, nil
}

func (observer dockerObserver) stopCandidate(ctx context.Context,
	process candidateProcess) error {
	if !validContainerID(process.ContainerID) {
		return errors.New("failed candidate container identity is invalid")
	}
	if _, err := observer.docker(ctx, 10*time.Second, "stop", "-t", "0", process.ContainerID); err != nil {
		return err
	}
	raw, err := observer.docker(ctx, 10*time.Second, "inspect", "--format", "{{.Id}} {{.State.Running}}", process.ContainerID)
	if err != nil || strings.TrimSpace(string(raw)) != process.ContainerID+" false" {
		return errors.Join(err, errors.New("failed candidate did not remain stopped"))
	}
	return nil
}

func (observer dockerObserver) candidateUnavailable(ctx context.Context,
	process candidateProcess, cellClock time.Time) (failedResourceReceipt, error) {
	if !validContainerID(process.ContainerID) {
		return failedResourceReceipt{}, errors.New("failed candidate container identity is invalid")
	}
	raw, err := observer.docker(ctx, 10*time.Second, "inspect", "--format", "{{.Id}} {{.State.Running}}", process.ContainerID)
	if err != nil {
		return failedResourceReceipt{}, fmt.Errorf("inspect failed Route candidate availability: %w", err)
	}
	if strings.TrimSpace(string(raw)) != process.ContainerID+" false" {
		return failedResourceReceipt{}, errors.New("failed Route candidate became available again")
	}
	return failedResourceReceipt{ContainerID: process.ContainerID,
		ObservedAtNanos: time.Since(cellClock).Nanoseconds()}, nil
}

func parseAttachmentCount(raw []byte, role string) (uint32, error) {
	var highest uint32
	seen := map[uint32]bool{}
	for _, line := range splitLines(raw) {
		var value struct {
			Kind, Role, Terminal string
			Attachment           uint32
			PeerAuthenticated    bool `json:"peer_authenticated"`
		}
		if err := json.Unmarshal(line, &value); err != nil {
			return 0, errors.Join(err, errors.New("decode "+role+" attachment evidence"))
		}
		if value.Kind == "complete" && value.Role == role && value.Terminal == "success" && value.PeerAuthenticated {
			if value.Attachment == 0 || seen[value.Attachment] {
				return 0, errors.New(role + " attachment evidence is duplicated or unnumbered")
			}
			seen[value.Attachment] = true
			highest = max(highest, value.Attachment)
		}
	}
	if highest == 0 {
		return 0, errors.New(role + " authenticated attachment evidence is missing")
	}
	return highest, nil
}

func (observer dockerObserver) waitContainerStopped(ctx context.Context, identity string, limit time.Duration) error {
	if !validContainerID(identity) {
		return errors.New("replacement process identity is invalid")
	}
	deadline := time.Now().Add(limit)
	for time.Now().Before(deadline) {
		raw, err := observer.docker(ctx, 10*time.Second, "inspect", "--format", "{{.Id}} {{.State.Running}}", identity)
		if err == nil && strings.TrimSpace(string(raw)) == identity+" false" {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
	return errors.New("replacement process did not stop within its bounded lifetime")
}
