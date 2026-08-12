package state

import (
	"context"
	"errors"
	"fmt"

	"github.com/dianabuilds/ardents-network/internal/network/source"
)

type sourceResult struct {
	index        int
	slot         int
	decision     candidateDecision
	observations [4]byte
	err          error
}

// Refresh waits for the complete two-source wave and accepts its highest valid state.
func (s *networkState) Refresh(ctx context.Context) (Snapshot, error) {
	if s.resourceGuard != nil {
		if err := s.resourceGuard.Check(); err != nil {
			return Snapshot{}, err
		}
	}
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return Snapshot{}, errors.New("network state is closed")
	}
	if s.refreshing {
		s.mu.Unlock()
		return Snapshot{}, errors.New("network state refresh is already active")
	}
	if !s.config.sourceInfo.Configured {
		s.mu.Unlock()
		return Snapshot{}, errors.New("finite source plan is not configured")
	}
	if s.distribution.conflicting {
		s.mu.Unlock()
		return Snapshot{}, errors.New("network state has a persistent source conflict")
	}
	now, err := trustedNow(s.config, s.distribution)
	if err != nil {
		s.mu.Unlock()
		return Snapshot{}, err
	}
	if err := s.activatePending(now); err != nil {
		s.mu.Unlock()
		return Snapshot{}, err
	}
	if s.distribution.nextAutomatic > now.Unix() {
		s.mu.Unlock()
		return Snapshot{}, fmt.Errorf("%w: retry is in durable backoff", errRefreshUnavailable)
	}
	if err := s.rejectSourceCollisions(); err != nil {
		s.mu.Unlock()
		return Snapshot{}, err
	}
	s.refreshing = true
	s.work.Add(1)
	defer s.work.Done()
	current, currentDecision := s.current, s.currentDecision
	order, deadline, err := s.startSourceWave(now)
	s.mu.Unlock()
	if err != nil {
		s.finishRefresh()
		return Snapshot{}, err
	}

	waveContext, cancel := context.WithDeadline(ctx, deadline)
	defer cancel()
	stopOwnedCancellation := context.AfterFunc(s.workContext, cancel)
	defer stopOwnedCancellation()
	results := make(chan sourceResult, 2)
	observed := make([]sourceResult, 0, 2)
	launched := 0
	for _, index := range order {
		started, outcome, beginErr := s.beginLatestAttempt(index)
		if beginErr != nil {
			observed = append(observed, failedSourceResult(index, index, [4]byte{}, beginErr))
			continue
		}
		if !started {
			observations := [4]byte{}
			observations[index] = outcome
			observed = append(observed, sourceResult{index: index, observations: observations, err: errors.New("durable LATEST attempt was not repeated")})
			continue
		}
		launched++
		go func(sourceIndex int) {
			results <- s.fetchAndVerify(waveContext, sourceIndex, current, currentDecision)
		}(index)
	}
	for range launched {
		observed = append(observed, <-results)
	}
	return s.completeSourceWave(now, current, observed)
}

func (s *networkState) fetchAndVerify(ctx context.Context, index int, current *Snapshot, currentDecision *candidateDecision) sourceResult {
	observations := [4]byte{}
	resultIndex, outcomeIndex := index, index
	response, err := s.fetchSource(ctx, index, source.Message{
		Operation: "latest", NetworkDigest: networkIdentityDigest(s.config.networkID),
		MaterialIndex: s.config.sourceInfo.MaterialIndex,
	})
	if err != nil && !isZero32(response.ObjectDigest) {
		observations[index] = classifySourceOutcome(err)
		fallback := 1 - index
		if startErr := s.beginDigestAttempt(fallback, response.ObjectDigest); startErr != nil {
			return failedSourceResult(index, outcomeIndex, observations, startErr)
		}
		requestedDigest := response.ObjectDigest
		resultIndex, outcomeIndex = fallback, 2+fallback
		response, err = s.fetchSource(ctx, fallback, source.Message{
			Operation: "by-digest", NetworkDigest: networkIdentityDigest(s.config.networkID), ObjectDigest: response.ObjectDigest,
			MaterialIndex: s.config.sourceInfo.MaterialIndex,
		})
		if terminalErr := s.finishDigestAttempt(fallback, err == nil); terminalErr != nil {
			return failedSourceResult(resultIndex, outcomeIndex, observations, terminalErr)
		}
		if err == nil {
			err = validateByDigestResponse(requestedDigest, response.ObjectDigest)
		}
	}
	if err != nil {
		return failedSourceResult(resultIndex, outcomeIndex, observations, err)
	}
	bundle, err := decodeSourceBundle(response.Payload)
	if err != nil {
		return failedSourceResult(resultIndex, outcomeIndex, observations, err)
	}
	decision, err := s.verifySourceBundle(bundle, current, currentDecision)
	if err != nil {
		return failedSourceResult(resultIndex, outcomeIndex, observations, err)
	}
	if err := s.rejectDecisionSourceCollisions(decision); err != nil {
		return failedSourceResult(resultIndex, outcomeIndex, observations, err)
	}
	if response.ObjectDigest != decision.epoch.digest {
		return failedSourceResult(resultIndex, outcomeIndex, observations, errors.New("source header digest disagrees with its authenticated Epoch"))
	}
	observations[outcomeIndex] = sourceOutcomeValid
	return sourceResult{index: resultIndex, slot: outcomeIndex, decision: decision, observations: observations}
}

func validateByDigestResponse(requested, returned [32]byte) error {
	if requested != returned {
		return errors.New("BY_DIGEST source returned a different object")
	}
	return nil
}

func failedSourceResult(index, outcomeIndex int, observations [4]byte, err error) sourceResult {
	observations[outcomeIndex] = classifySourceOutcome(err)
	return sourceResult{index: index, slot: outcomeIndex, observations: observations, err: err}
}

func (s *networkState) fetchSource(ctx context.Context, index int, request source.Message) (source.Message, error) {
	response, err := s.config.source.Fetch(ctx, index, request)
	if err != nil {
		return response, err
	}
	if response.Status != "ok" {
		return response, sourceStatusError(response.Status)
	}
	return response, nil
}
