package nativecircuit

import (
	"context"
	"errors"
	"net"
	"sync"
)

type rendezvousManager struct {
	mu       sync.Mutex
	pending  map[handle]*rendezvousParticipant
	consumed map[handle]struct{}
}

type rendezvousParticipant struct {
	side   string
	handle handle
	stream net.Conn
	paired chan *rendezvousPair
}

type rendezvousPair struct {
	user    net.Conn
	service net.Conn
	done    chan struct{}
	context context.Context
}

func newRendezvousManager() *rendezvousManager {
	return &rendezvousManager{pending: make(map[handle]*rendezvousParticipant), consumed: make(map[handle]struct{})}
}

func (manager *rendezvousManager) join(ctx context.Context, side string, token, attempt handle, stream net.Conn) error {
	if side != "user" && side != "service" || token == (handle{}) || attempt == (handle{}) {
		_ = stream.Close()
		return errors.New("rendezvous attachment is incomplete")
	}
	participant := &rendezvousParticipant{side: side, handle: attempt, stream: stream, paired: make(chan *rendezvousPair, 1)}
	manager.mu.Lock()
	if _, used := manager.consumed[token]; used {
		manager.mu.Unlock()
		_ = stream.Close()
		return errors.New("rendezvous join token was already consumed")
	}
	other, found := manager.pending[token]
	if !found {
		manager.pending[token] = participant
		manager.mu.Unlock()
		select {
		case pair := <-participant.paired:
			return runRendezvousPair(side, pair)
		case <-ctx.Done():
			manager.removePending(token, participant)
			_ = stream.Close()
			return ctx.Err()
		}
	}
	if other.side == side || other.handle == attempt {
		delete(manager.pending, token)
		manager.consumed[token] = struct{}{}
		manager.mu.Unlock()
		_ = other.stream.Close()
		_ = stream.Close()
		return errors.New("rendezvous rejected duplicate side or attempt handle")
	}
	delete(manager.pending, token)
	manager.consumed[token] = struct{}{}
	pair := &rendezvousPair{done: make(chan struct{}), context: ctx}
	if side == "user" {
		pair.user, pair.service = stream, other.stream
	} else {
		pair.user, pair.service = other.stream, stream
	}
	manager.mu.Unlock()
	other.paired <- pair
	return runRendezvousPair(side, pair)
}

func runRendezvousPair(side string, pair *rendezvousPair) error {
	if side == "user" {
		stop := context.AfterFunc(pair.context, func() {
			_ = pair.user.Close()
			_ = pair.service.Close()
		})
		defer stop()
		result := frame{Type: frameRendezvousResult, Payload: []byte("joined")}
		if err := writeFrame(pair.user, result); err != nil {
			_ = pair.user.Close()
			_ = pair.service.Close()
			close(pair.done)
			return err
		}
		if err := writeFrame(pair.service, result); err != nil {
			_ = pair.user.Close()
			_ = pair.service.Close()
			close(pair.done)
			return err
		}
		proxyOpaque(pair.user, pair.service)
		close(pair.done)
		return pair.context.Err()
	}
	<-pair.done
	return nil
}

func (manager *rendezvousManager) removePending(token handle, participant *rendezvousParticipant) {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	if manager.pending[token] == participant {
		delete(manager.pending, token)
	}
}
