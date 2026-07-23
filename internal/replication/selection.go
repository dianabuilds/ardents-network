package replication

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	model "ardents/internal/content/catalog"
	identityprincipal "ardents/internal/identity/principal"
	"ardents/internal/replication/placement"
)

const (
	capacityQueryTimeout  = 3 * time.Second
	capacityQueryAttempts = 2
	capacityRetryDelay    = 250 * time.Millisecond
	maxCapacityCandidates = 16
	capacityQueryWorkers  = 4
)

type PlacementOutcome struct {
	Decision    placement.SelectionDecision
	Commitments []placement.Commitment
}

type placementUnsatisfied struct {
	decision            placement.SelectionDecision
	committed, required int
}

func (e *placementUnsatisfied) Error() string {
	return placementUnsatisfiedMessage(e.decision, e.committed, e.required)
}

func (s *Service) PlaceAvailable(ctx context.Context, blobID string, count int, intentVersion uint64) (PlacementOutcome, error) {
	blob, ok := s.cfg.Data.GetBlob(blobID)
	if !ok {
		return PlacementOutcome{}, fmt.Errorf("blob not found")
	}
	if !blob.Encrypted {
		return PlacementOutcome{}, fmt.Errorf("plaintext replica placement is forbidden")
	}
	excluded := s.existingReplicaPeers(blobID, intentVersion)
	candidates := s.observeCandidates(ctx, blob, excluded)
	decision := placement.SelectTargets(placement.SelectionRequest{
		OwnerPrincipal: s.cfg.LocalNodePrincipal, EncryptedSize: blob.Size, Count: count, Now: s.cfg.Now().UTC(),
		ExcludedNodes: excluded,
	}, candidates)
	outcome := PlacementOutcome{Decision: decision, Commitments: make([]placement.Commitment, 0, len(decision.Selected))}
	for _, candidate := range decision.Selected {
		commitment, err := s.PlaceBlob(ctx, blobID, candidate.NodePrincipal, intentVersion)
		if err != nil {
			outcome.Decision.Denials = append(outcome.Decision.Denials, placement.Denial{NodePrincipal: candidate.NodePrincipal, Reason: safeReason(err)})
			continue
		}
		outcome.Commitments = append(outcome.Commitments, commitment)
	}
	if len(outcome.Commitments) < count {
		return outcome, &placementUnsatisfied{decision: outcome.Decision, committed: len(outcome.Commitments), required: count}
	}
	return outcome, nil
}

func placementUnsatisfiedError(decision placement.SelectionDecision, committed, required int) error {
	return &placementUnsatisfied{decision: decision, committed: committed, required: required}
}

func placementUnsatisfiedMessage(decision placement.SelectionDecision, committed, required int) string {
	counts := make(map[string]int, len(decision.Denials))
	for _, denial := range decision.Denials {
		reason := denial.Reason
		if reason == "" {
			reason = "unspecified"
		}
		counts[reason]++
	}
	reasons := make([]string, 0, len(counts))
	for reason, count := range counts {
		reasons = append(reasons, fmt.Sprintf("%s:%d", reason, count))
	}
	sort.Strings(reasons)
	return fmt.Sprintf("replica placement target count is unsatisfied: committed=%d required=%d denials=%s", committed, required, strings.Join(reasons, ","))
}

func (s *Service) existingReplicaPeers(blobID string, intentVersion uint64) map[identityprincipal.ID]bool {
	excluded := map[identityprincipal.ID]bool{}
	for _, commitment := range s.cfg.Data.ReplicaPlacementState().Commitments {
		if commitment.ContentReference.String() == blobID && commitment.IntentVersion == intentVersion && commitment.TargetNode.String() != "" {
			excluded[commitment.TargetNode] = true
		}
	}
	return excluded
}

func (s *Service) observeCandidates(ctx context.Context, blob model.Blob, excluded map[identityprincipal.ID]bool) []placement.Candidate {
	targets := s.candidateTargets()
	pending := make([]identityprincipal.ID, 0, len(targets))
	candidates := make([]placement.Candidate, 0, len(targets))
	results := make(chan placement.Candidate, len(targets))
	for _, target := range targets {
		if excluded[target] {
			candidates = append(candidates, placement.Candidate{NodePrincipal: target})
			continue
		}
		pending = append(pending, target)
	}
	jobs := make(chan identityprincipal.ID, len(pending))
	for _, target := range pending {
		jobs <- target
	}
	close(jobs)
	workers := min(len(pending), capacityQueryWorkers)
	var group sync.WaitGroup
	group.Add(workers)
	for range workers {
		go func() {
			defer group.Done()
			for target := range jobs {
				results <- s.observeCandidate(ctx, target, blob)
			}
		}()
	}
	group.Wait()
	close(results)
	for candidate := range results {
		candidates = append(candidates, candidate)
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].NodePrincipal.String() < candidates[j].NodePrincipal.String()
	})
	return candidates
}

func (s *Service) candidateTargets() []identityprincipal.ID {
	seen := map[identityprincipal.ID]bool{}
	targets := make([]identityprincipal.ID, 0)
	for _, entry := range s.cfg.Discovery.Entries() {
		target, err := identityprincipal.Parse(entry.Record.Subject())
		if entry.Record.Kind() != "node" || err != nil || target.Equal(s.cfg.LocalNodePrincipal) || seen[target] {
			continue
		}
		seen[target] = true
		targets = append(targets, target)
	}
	sort.Slice(targets, func(i, j int) bool { return targets[i].String() < targets[j].String() })
	if len(targets) > maxCapacityCandidates {
		targets = targets[:maxCapacityCandidates]
	}
	return targets
}

func (s *Service) observeCandidate(ctx context.Context, target identityprincipal.ID, blob model.Blob) placement.Candidate {
	candidate := placement.Candidate{NodePrincipal: target, PolicyAllowed: s.cfg.Policy.AllowPeerBlobReserving(blobView(blob)) == nil}
	entry, outcome, ok := s.cfg.Discovery.Resolve(target.String(), "node")
	if !ok || outcome != "found" {
		return candidate
	}
	trust := s.cfg.Trust.Evaluate(entry.Record)
	candidate.Trusted, candidate.Usable = trust.Valid && trust.Trusted, trust.Usable
	if !candidate.Trusted || !candidate.Usable || !candidate.PolicyAllowed {
		return candidate
	}
	observation, err := s.queryCapacityWithRetry(ctx, target, blob)
	if err != nil {
		candidate.DenialReason = placement.ReasonObservation
		return candidate
	}
	candidate.CapabilityValid = true
	candidate.DenialReason = observation.DenialReason
	candidate.CapacityBytes = observation.Capacity.FreeBytes
	candidate.ObservedAt = observation.Capacity.ObservedAt
	return candidate
}

func (s *Service) queryCapacityWithRetry(ctx context.Context, target identityprincipal.ID, blob model.Blob) (capacityObservation, error) {
	return retryCapacityQuery(ctx, func(attemptCtx context.Context) (capacityObservation, error) {
		return s.queryCapacity(attemptCtx, target, blob)
	})
}

func retryCapacityQuery(ctx context.Context, query func(context.Context) (capacityObservation, error)) (capacityObservation, error) {
	var lastErr error
	for attempt := range capacityQueryAttempts {
		observation, err := query(ctx)
		if err == nil {
			return observation, nil
		}
		lastErr = err
		if attempt+1 == capacityQueryAttempts {
			break
		}
		timer := time.NewTimer(capacityRetryDelay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return capacityObservation{}, errors.Join(lastErr, ctx.Err())
		case <-timer.C:
		}
	}
	return capacityObservation{}, lastErr
}

type capacityObservation struct {
	Capacity     placement.Capacity
	DenialReason string
}

func (s *Service) queryCapacity(ctx context.Context, target identityprincipal.ID, blob model.Blob) (capacityObservation, error) {
	operationID, _, err := operationIdentity()
	if err != nil {
		return capacityObservation{}, err
	}
	responses, unregister, err := s.cfg.Exchange.RegisterReplicaResponses(operationID)
	if err != nil {
		return capacityObservation{}, err
	}
	defer unregister()
	queryCtx, cancel := context.WithTimeout(ctx, capacityQueryTimeout)
	defer cancel()
	if err := s.publishControl(queryCtx, actionCapacityQuery, operationID, target, capacityQueryBody{Blob: blob}); err != nil {
		return capacityObservation{}, err
	}
	wire, err := s.awaitControl(queryCtx, responses, actionCapacityResult, target)
	if err != nil {
		return capacityObservation{}, err
	}
	var result capacityResultBody
	if err := decodeControlBody(wire.Body, &result); err != nil {
		return capacityObservation{}, err
	}
	if result.Status == "rejected" && validCapacityDenial(result.Reason) {
		return capacityObservation{DenialReason: result.Reason}, nil
	}
	if result.Status != "available" || result.Capacity == nil || !result.Capacity.NodePrincipal.Equal(target) || result.Capacity.ObservedAt.IsZero() {
		return capacityObservation{}, fmt.Errorf("replica capacity response binding is invalid")
	}
	return capacityObservation{Capacity: *result.Capacity}, nil
}

func validCapacityDenial(reason string) bool {
	return reason == placement.ReasonUntrusted || reason == placement.ReasonCapability ||
		reason == placement.ReasonPolicy || reason == placement.ReasonUnsupported
}
