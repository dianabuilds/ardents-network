package serviceconn

import (
	"crypto/rand"
	"errors"
	"time"
)

type localSession struct {
	principal [32]byte
	surface   string
	broker    [32]byte
	expires   int64
}

func (endpoint *Endpoint) admit(input Request) (Result, error) {
	var expected [32]byte
	switch input.Surface {
	case "connection":
		expected = endpoint.connectionPrincipal
	case "administration":
		expected = endpoint.adminPrincipal
	default:
		return denied("local interface surface is not granted")
	}
	if expected == [32]byte{} || input.Principal != expected {
		return denied("Application Principal does not match Local Grant")
	}
	endpoint.mu.Lock()
	defer endpoint.mu.Unlock()
	if len(endpoint.sessions) >= maximumSessions {
		return failed("local authorization or policy denial", "local session budget is exhausted", errors.New("session budget exhausted"))
	}
	var capability [32]byte
	if _, err := rand.Read(capability[:]); err != nil {
		return failed("indeterminate failure", "fresh local session could not be created", err)
	}
	if capability == [32]byte{} {
		return failed("indeterminate failure", "fresh local session is invalid", errors.New("zero session capability"))
	}
	endpoint.sessions[capability] = localSession{principal: input.Principal, surface: input.Surface,
		broker: endpoint.broker, expires: endpoint.clock().Add(15 * time.Second).UnixNano()}
	return Result{Class: "authorized", Session: capability}, nil
}

func (endpoint *Endpoint) consume(capability, principal [32]byte, surface string) error {
	endpoint.mu.Lock()
	defer endpoint.mu.Unlock()
	session, ok := endpoint.sessions[capability]
	if ok {
		delete(endpoint.sessions, capability)
	}
	if !ok || capability == [32]byte{} || session.principal != principal || session.surface != surface ||
		session.broker != endpoint.broker || endpoint.clock().UnixNano() > session.expires {
		return errors.New("ephemeral session is absent, replayed, or bound to another principal")
	}
	return nil
}
