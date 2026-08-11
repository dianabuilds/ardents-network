package networkstate

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"time"
)

type sourceResult struct {
	index        int
	slot         int
	decision     candidateDecision
	observations [4]byte
	err          error
}

// Refresh waits for the complete two-source wave and accepts its highest valid state.
func (s *store) Refresh(ctx context.Context) (Snapshot, error) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return Snapshot{}, errors.New("network state is closed")
	}
	if s.refreshing {
		s.mu.Unlock()
		return Snapshot{}, errors.New("network state refresh is already active")
	}
	if s.config.sources[0].address == "" {
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
		return Snapshot{}, errors.New("finite source retry is in durable backoff")
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

func (s *store) fetchAndVerify(ctx context.Context, index int, current *Snapshot, currentDecision *candidateDecision) sourceResult {
	observations := [4]byte{}
	resultIndex, outcomeIndex := index, index
	response, err := s.fetchSource(ctx, index, sourceRequest{
		opcode: sourceLatest, networkDigest: networkIdentityDigest(s.config.networkID),
	})
	if err != nil && !isZero32(response.objectDigest) {
		observations[index] = classifySourceOutcome(err)
		fallback := 1 - index
		if startErr := s.beginDigestAttempt(fallback, response.objectDigest); startErr != nil {
			return failedSourceResult(index, outcomeIndex, observations, startErr)
		}
		requestedDigest := response.objectDigest
		resultIndex, outcomeIndex = fallback, 2+fallback
		response, err = s.fetchSource(ctx, fallback, sourceRequest{
			opcode: sourceByDigest, networkDigest: networkIdentityDigest(s.config.networkID), objectDigest: response.objectDigest,
		})
		if terminalErr := s.finishDigestAttempt(fallback, err == nil); terminalErr != nil {
			return failedSourceResult(resultIndex, outcomeIndex, observations, terminalErr)
		}
		if err == nil {
			err = validateByDigestResponse(requestedDigest, response.objectDigest)
		}
	}
	if err != nil {
		return failedSourceResult(resultIndex, outcomeIndex, observations, err)
	}
	bundle, err := decodeSourceBundle(response.payload)
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
	if response.objectDigest != decision.epoch.digest {
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

func (s *store) fetchSource(ctx context.Context, index int, request sourceRequest) (sourceResponse, error) {
	source := s.config.sources[index]
	totalContext, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	connection, err := (&net.Dialer{Timeout: time.Second}).DialContext(totalContext, "tcp", source.address)
	if err != nil {
		return sourceResponse{}, fmt.Errorf("source unavailable: %w", err)
	}
	defer connection.Close()
	if err := connection.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		return sourceResponse{}, err
	}
	tlsConnection := tls.Client(connection, sourceTLSConfig(source))
	handshakeContext, stopHandshake := context.WithTimeout(totalContext, 2*time.Second)
	err = tlsConnection.HandshakeContext(handshakeContext)
	stopHandshake()
	if err != nil {
		return sourceResponse{}, fmt.Errorf("source authentication failed: %w", err)
	}
	if err := writeSourceRequest(tlsConnection, request); err != nil {
		return sourceResponse{}, fmt.Errorf("write source request: %w", err)
	}
	response, err := readSourceResponse(tlsConnection)
	if err != nil {
		return response, fmt.Errorf("read source response: %w", err)
	}
	if response.status != sourceOK {
		return sourceResponse{}, sourceStatusErrors[response.status]
	}
	var trailing [1]byte
	if count, trailingErr := tlsConnection.Read(trailing[:]); count != 0 || (trailingErr != nil && !errors.Is(trailingErr, io.EOF)) {
		return sourceResponse{}, errors.New("source response has trailing bytes or an unclean close")
	}
	return response, nil
}

func sourceTLSConfig(source sourceConfig) *tls.Config {
	return &tls.Config{
		MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13,
		RootCAs: source.roots, ServerName: source.serverName,
		Certificates:       []tls.Certificate{source.client},
		ClientSessionCache: nil, SessionTicketsDisabled: true,
		VerifyConnection: func(state tls.ConnectionState) error {
			if len(state.PeerCertificates) == 0 {
				return errors.New("source certificate is missing")
			}
			digest, err := transportKeyDigest(state.PeerCertificates[0].PublicKey)
			if err != nil || digest != source.leafDigest {
				return errors.New("source leaf key pin does not match")
			}
			return nil
		},
	}
}
