//go:build linux

package endpoint

import (
	"context"
	"errors"
	"sync"

	"github.com/dianabuilds/ardents-network/internal/application/interfacev2/connection"
)

// The qualification Publisher keeps one slot below the dispatch hard cap and
// explicitly reserves the matching forwarding stock before releasing each
// opening. The shared Reader admission owner enforces the same bound.
const qualificationPublisherOpeningParallelism = streamQualificationSetupLimit
const qualificationPublisherOpeningBatch = 16

func (worker *qualifiedTextWorker) produceNetwork(lifetime context.Context, delivered chan<- connection.Stream) error {
	if worker == nil || worker.job == nil {
		return errors.New("text Publisher producer unavailable")
	}
	if worker.job.qualification == nil {
		return worker.produceNetworkSequential(lifetime, delivered)
	}
	return worker.produceQualificationNetwork(lifetime, delivered)
}

func (worker *qualifiedTextWorker) produceNetworkSequential(lifetime context.Context, delivered chan<- connection.Stream) error {
	owner := worker.job.owner
	network, cancel := context.WithCancel(lifetime)
	var retired sync.WaitGroup
	owner.mu.Lock()
	drain := owner.publicationDrain
	slots := make(chan struct{}, owner.streamConnectionLimitLocked())
	owner.mu.Unlock()
	draining := false
	defer func() {
		if !draining {
			cancel()
		}
		retired.Wait()
		cancel()
	}()
	for {
		select {
		case <-drain:
			draining = true
			return nil
		case slots <- struct{}{}:
		case <-network.Done():
			return network.Err()
		}
		attempt, err := owner.receiveTextIntroduction(network, worker.job)
		if onlyTextPublicationDraining(err) {
			draining = true
			return nil
		}
		if onlyTextIntroductionRefusal(err) {
			<-slots
			continue
		}
		if err != nil {
			return err
		}
		stream, err := owner.openTextJoinedService(network, worker.job, attempt)
		if err != nil {
			return err
		}
		if err := network.Err(); err != nil {
			return errors.Join(err, stream.Close())
		}
		select {
		case delivered <- stream:
			retired.Add(1)
			go func() {
				defer retired.Done()
				<-stream.finished
				<-slots
			}()
		case <-network.Done():
			return errors.Join(network.Err(), stream.Close())
		}
	}
}

func (worker *qualifiedTextWorker) ensureQualificationPublisherJoinReserve(ctx context.Context, minimum int) error {
	owner := worker.job.owner
	owner.mu.Lock()
	prefix := owner.responder.prefix
	owner.mu.Unlock()
	if prefix == nil {
		var err error
		prefix, err = owner.openTextPublisherPrefix(ctx, &owner.responder, 3)
		if err != nil {
			return errors.Join(err, errors.New("qualification Publisher JOIN reserve prefix unavailable"))
		}
	}
	receiver, _, _, err := prefix.DataJoinRecipient()
	if err != nil {
		return err
	}
	return owner.ensureQualificationTokenReserve(ctx, receiver, 2, minimum)
}

func (worker *qualifiedTextWorker) produceQualificationNetwork(lifetime context.Context, delivered chan<- connection.Stream) error {
	owner := worker.job.owner
	network, cancel := context.WithCancel(lifetime)
	if err := worker.ensureQualificationPublisherJoinReserve(network, 32); err != nil {
		cancel()
		return err
	}
	var retired sync.WaitGroup
	owner.mu.Lock()
	drain := owner.publicationDrain
	connectionLimit := owner.streamConnectionLimitLocked()
	owner.mu.Unlock()
	draining := false
	// Each pending or joined Service owns one connection slot. Concurrent
	// receivers register the waiters that the Introduction dispatch already
	// bounds; without them, a second honest submission can arrive before the
	// sequential producer registers its matching owner and be refused.
	slots := make(chan struct{}, connectionLimit)
	type openingResult struct {
		stream *textServiceStream
		err    error
	}
	opened := make(chan openingResult, qualificationPublisherOpeningParallelism)
	var openings sync.WaitGroup
	inFlight := 0
	batchStarted := 0
	stopping := false
	var failure error
	networkDone := network.Done()
	startOpening := func() bool {
		select {
		case slots <- struct{}{}:
		default:
			return false
		}
		inFlight++
		batchStarted++
		openings.Add(1)
		go func() {
			defer openings.Done()
			attempt, err := owner.receiveTextIntroduction(network, worker.job)
			if err != nil {
				opened <- openingResult{err: err}
				return
			}
			stream, err := owner.openTextJoinedService(network, worker.job, attempt)
			opened <- openingResult{stream: stream, err: err}
		}()
		return true
	}
	defer func() {
		if !draining {
			cancel()
		}
		openings.Wait()
		retired.Wait()
		cancel()
	}()
	for {
		if !stopping && batchStarted == qualificationPublisherOpeningBatch && inFlight == 0 {
			// No accepted capsule is waiting while the slow issuer replenishes the
			// next batch. Thirty-two tokens leave a full batch in reserve after
			// every sixteen-opening window.
			if err := worker.ensureQualificationPublisherJoinReserve(network, 32); err != nil {
				failure = errors.Join(failure, err)
				stopping = true
				cancel()
				networkDone = nil
			} else {
				batchStarted = 0
			}
		}
		for !stopping && batchStarted < qualificationPublisherOpeningBatch && inFlight < qualificationPublisherOpeningParallelism && startOpening() {
		}
		if stopping && inFlight == 0 {
			return failure
		}
		select {
		case <-drain:
			draining = true
			stopping = true
			drain = nil
		case result := <-opened:
			inFlight--
			if result.err != nil {
				<-slots
				if onlyTextPublicationDraining(result.err) {
					draining = true
					stopping = true
					drain = nil
					continue
				}
				if onlyTextIntroductionRefusal(result.err) {
					continue
				}
				if !stopping || !qualificationCancellationOnly(result.err) {
					failure = errors.Join(failure, result.err)
				}
				if !stopping {
					stopping = true
					cancel()
					networkDone = nil
				}
				continue
			}
			if stopping {
				failure = errors.Join(failure, result.stream.Close())
				<-slots
				continue
			}
			if err := network.Err(); err != nil {
				failure = errors.Join(failure, err, result.stream.Close())
				<-slots
				stopping = true
				cancel()
				networkDone = nil
				continue
			}
			select {
			case delivered <- result.stream:
				retired.Add(1)
				go func(stream *textServiceStream) {
					defer retired.Done()
					<-stream.finished
					<-slots
				}(result.stream)
			case <-network.Done():
				failure = errors.Join(failure, network.Err(), result.stream.Close())
				<-slots
				stopping = true
				cancel()
				networkDone = nil
			}
		case <-networkDone:
			if !stopping {
				failure = errors.Join(failure, network.Err())
				stopping = true
				networkDone = nil
			}
		}
	}
}
