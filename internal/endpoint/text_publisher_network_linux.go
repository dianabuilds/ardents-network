//go:build linux

package endpoint

import (
	"context"
	"errors"
	"sync"

	"github.com/dianabuilds/ardents-network/internal/application/interfacev2/connection"
)

func (worker *qualifiedTextWorker) produceNetwork(lifetime context.Context, delivered chan<- connection.Stream) error {
	owner := worker.job.owner
	network, cancel := context.WithCancel(lifetime)
	var retired sync.WaitGroup
	owner.mu.Lock()
	drain := owner.publicationDrain
	owner.mu.Unlock()
	draining := false
	// Each joined Service retains an Introduction exchange. Reserve space
	// before receiving the next delivery, including its temporary exchange.
	slots := make(chan struct{}, 16)
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
		// This one in-flight stream remains producer-owned during backpressure.
		// Service lifetime and the context's exchange limit bound its resources.
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
