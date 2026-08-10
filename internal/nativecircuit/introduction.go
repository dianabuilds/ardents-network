package nativecircuit

import (
	"context"
	"errors"
	"io"
	"sync"
)

type introductionManager struct {
	mu            sync.Mutex
	registrations map[handle]*introductionRegistration
	consumed      map[handle]struct{}
}

type introductionRegistration struct {
	stream   io.ReadWriter
	delivery chan introductionDelivery
}

type introductionDelivery struct {
	sealed []byte
	done   chan error
}

func newIntroductionManager() *introductionManager {
	return &introductionManager{registrations: make(map[handle]*introductionRegistration), consumed: make(map[handle]struct{})}
}

func (manager *introductionManager) register(ctx context.Context, slot handle, stream io.ReadWriter) error {
	if slot == (handle{}) || stream == nil {
		return errors.New("introduction registration is incomplete")
	}
	registration := &introductionRegistration{stream: stream, delivery: make(chan introductionDelivery, 1)}
	manager.mu.Lock()
	if _, used := manager.consumed[slot]; used || manager.registrations[slot] != nil {
		manager.mu.Unlock()
		return errors.New("introduction slot is not available")
	}
	manager.registrations[slot] = registration
	manager.mu.Unlock()
	if err := writeFrame(stream, frame{Type: frameIntroductionAcknowledge, Payload: []byte("registered")}); err != nil {
		manager.removeRegistration(slot, registration)
		return err
	}
	select {
	case delivery := <-registration.delivery:
		err := writeFrame(stream, frame{Type: frameIntroductionDeliver, Payload: delivery.sealed})
		if err == nil {
			var acknowledgement frame
			acknowledgement, err = readFrame(stream)
			if err == nil && (acknowledgement.Type != frameIntroductionAcknowledge || string(acknowledgement.Payload) != "accepted") {
				err = errors.New("service did not acknowledge the Introduction invitation")
			}
		}
		delivery.done <- err
		return err
	case <-ctx.Done():
		manager.removeRegistration(slot, registration)
		return ctx.Err()
	}
}

func (manager *introductionManager) deliver(ctx context.Context, slot handle, sealed []byte) error {
	if len(sealed) == 0 || len(sealed) > maximumInvitation {
		return errors.New("introduction delivery is outside the fixed bound")
	}
	manager.mu.Lock()
	if _, used := manager.consumed[slot]; used {
		manager.mu.Unlock()
		return errors.New("introduction slot was already consumed")
	}
	registration := manager.registrations[slot]
	if registration == nil {
		manager.mu.Unlock()
		return errors.New("introduction slot is not registered")
	}
	delete(manager.registrations, slot)
	manager.consumed[slot] = struct{}{}
	manager.mu.Unlock()
	delivery := introductionDelivery{sealed: append([]byte(nil), sealed...), done: make(chan error, 1)}
	select {
	case registration.delivery <- delivery:
	case <-ctx.Done():
		return ctx.Err()
	}
	select {
	case err := <-delivery.done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (manager *introductionManager) removeRegistration(slot handle, registration *introductionRegistration) {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	if manager.registrations[slot] == registration {
		delete(manager.registrations, slot)
	}
}
