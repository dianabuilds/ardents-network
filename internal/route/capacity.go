package route

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"
)

const maximumSetupAttempts = 2 * maximumRoleAttachments

var errAttachmentCapacity = errors.New("authenticated Route Attachment capacity is full")

type capacityCarry func(context.Context, Actor, net.Conn, Evidence, func() bool) (Evidence, error)

type capacityResult struct {
	connection    net.Conn
	evidence      Evidence
	err           error
	authenticated bool
}

type capacityAdmission struct {
	mu        sync.Mutex
	active    uint16
	completed uint16
	maximum   uint16
	protected bool
}

func (value *capacityAdmission) admit() bool {
	value.mu.Lock()
	defer value.mu.Unlock()
	if value.protected || value.active+value.completed >= value.maximum {
		return false
	}
	value.active++
	return true
}

func (value *capacityAdmission) release() {
	value.mu.Lock()
	defer value.mu.Unlock()
	value.active--
}

func (value *capacityAdmission) complete() {
	value.mu.Lock()
	defer value.mu.Unlock()
	value.active--
	value.completed++
}

func (value *capacityAdmission) protect(enabled bool) {
	value.mu.Lock()
	defer value.mu.Unlock()
	value.protected = enabled
}

func serveCapacity(ctx context.Context, input Actor, emit func(Evidence), observation Evidence,
	carry capacityCarry) (Evidence, error) {
	maximum, target := input.MaximumAttachments, input.AttachmentTarget
	if maximum == 0 {
		maximum = 1
	}
	if target == 0 {
		target = maximum
	}
	listener, err := net.Listen("tcp", input.ListenAddress)
	if err != nil {
		return observation, fmt.Errorf("listen for %s: %w", input.Role, err)
	}
	defer listener.Close()
	if err := bindListenerLifetime(ctx, listener.(*net.TCPListener), input.Role); err != nil {
		return observation, err
	}
	stop := context.AfterFunc(ctx, func() { _ = listener.Close() })
	defer stop()
	interval := input.PressureInterval
	if interval == 0 {
		interval = 100 * time.Millisecond
	}
	resources := &capacityResourceRecord{}
	guard, initialState, guardErr := capacityGuard(input, &observation, resources, interval)
	if guardErr != nil {
		return observation, guardErr
	}
	observation.State = initialState
	if emit != nil {
		emit(capacityEvent(input, observation, "ready", initialState, observation.Resource))
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	acceptContext, stopAccept := context.WithCancel(ctx)
	defer stopAccept()
	accepted := make(chan net.Conn)
	acceptErrors := make(chan error, 1)
	go acceptCapacityConnections(acceptContext, listener, accepted, acceptErrors)
	results := make(chan capacityResult, maximumSetupAttempts)
	connections := make(map[net.Conn]struct{})
	admission := &capacityAdmission{maximum: maximum}
	protected := initialState == "PROTECT"
	admission.protect(protected)
	targetReached := false
	for !targetReached || len(connections) > 0 {
		select {
		case connection := <-accepted:
			if len(connections) >= maximumSetupAttempts || protected {
				observation.AttachmentsRefused++
				_ = connection.Close()
				continue
			}
			connections[connection] = struct{}{}
			observation.AttachmentsAbandoned++
			go runCapacityAttempt(ctx, input, observation, connection, admission, carry, results)
		case result := <-results:
			recordCapacityResult(connections, admission, &observation, result)
			if !targetReached && observation.AttachmentsCompleted >= target {
				targetReached = true
				stopAccept()
				_ = listener.Close()
				accepted = nil
			}
		case acceptErr := <-acceptErrors:
			if targetReached {
				acceptErrors = nil
				continue
			}
			return observation, contextError(ctx, acceptErr)
		case <-ticker.C:
			if guard == nil {
				continue
			}
			pressure, observeErr := guard.Observe(uint64(len(connections)+1), uint64(len(connections)), 0)
			sample := pressure.Sample
			observation.Resource = &sample
			resources.record(&observation, sample, interval)
			if observeErr != nil || pressure.Drain {
				observation.State = "DRAIN"
				if emit != nil {
					emit(capacityEvent(input, observation, "resource", "DRAIN", &pressure.Sample))
				}
				stopAccept()
				closeCapacityConnections(listener, connections)
				drainCapacityConnections(connections, results, admission, &observation)
				resources.finish(&observation)
				if emit != nil {
					emit(capacityEvent(input, observation, "resource", "EXIT", &pressure.Sample))
				}
				return observation, errors.Join(errors.New("route capacity drained under resource pressure"), observeErr)
			}
			if pressure.Protect != protected {
				protected = pressure.Protect
				admission.protect(protected)
				state := "NORMAL"
				if protected {
					state = "PROTECT"
				}
				observation.State = state
				if emit != nil {
					emit(capacityEvent(input, observation, "resource", state, &pressure.Sample))
				}
			}
		case <-ctx.Done():
			stopAccept()
			closeCapacityConnections(listener, connections)
			drainCapacityConnections(connections, results, admission, &observation)
			resources.finish(&observation)
			return observation, ctx.Err()
		}
	}
	resources.finish(&observation)
	observation.PeerAuthenticated = true
	return observation, nil
}

func acceptCapacityConnections(ctx context.Context, listener net.Listener, accepted chan<- net.Conn, failed chan<- error) {
	for {
		connection, err := listener.Accept()
		if err != nil {
			select {
			case failed <- err:
			case <-ctx.Done():
			}
			return
		}
		select {
		case accepted <- connection:
		case <-ctx.Done():
			_ = connection.Close()
			return
		}
	}
}

func runCapacityAttempt(ctx context.Context, input Actor, observation Evidence, connection net.Conn,
	admission *capacityAdmission, carry capacityCarry, results chan<- capacityResult) {
	authenticated := false
	value, err := carry(ctx, input, connection, observation, func() bool {
		authenticated = admission.admit()
		return authenticated
	})
	results <- capacityResult{connection: connection, evidence: value, err: err, authenticated: authenticated}
}

func aggregateCapacityEvidence(total *Evidence, value Evidence) {
	total.AttachmentsCompleted++
	total.OpaqueBytes += value.OpaqueBytes
	total.ReverseOpaqueBytes += value.ReverseOpaqueBytes
	total.CanaryLength += value.CanaryLength
}

func recordCapacityResult(connections map[net.Conn]struct{}, admission *capacityAdmission,
	observation *Evidence, result capacityResult) {
	delete(connections, result.connection)
	if result.authenticated {
		if result.err == nil {
			admission.complete()
		} else {
			admission.release()
		}
	}
	if result.err == nil && result.authenticated {
		observation.AttachmentsAbandoned--
		aggregateCapacityEvidence(observation, result.evidence)
	} else if errors.Is(result.err, errAttachmentCapacity) {
		observation.AttachmentsAbandoned--
		observation.AttachmentsRefused++
	}
}
