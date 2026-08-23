//go:build ignore

package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"runtime"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
)

type roleCarriageMeasurement struct {
	Schema             string        `json:"schema"`
	Host               hostIdentity  `json:"host"`
	Capacity           int           `json:"capacity"`
	PayloadBytes       int           `json:"payload_bytes"`
	AdmittedLegs       int           `json:"admitted_legs"`
	RefusedDials       int           `json:"refused_dials"`
	Withdrawn          bool          `json:"withdrawn"`
	HoldNanoseconds    int64         `json:"hold_nanoseconds"`
	ElapsedNanoseconds int64         `json:"elapsed_nanoseconds"`
	GoroutinesBefore   int           `json:"goroutines_before"`
	GoroutinesAfter    int           `json:"goroutines_after"`
	LinuxSamples       []linuxSample `json:"linux_samples,omitempty"`
}

func runRoleCarriage(capacity, payloadSize int, hold, sampleInterval, timeout time.Duration) (roleCarriageMeasurement, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	host, err := currentHostIdentity()
	if err != nil {
		return roleCarriageMeasurement{}, err
	}
	client, clientKey, err := identity(10)
	if err != nil {
		return roleCarriageMeasurement{}, err
	}
	serverCertificate, serverKey, err := identity(11)
	if err != nil {
		return roleCarriageMeasurement{}, err
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return roleCarriageMeasurement{}, err
	}
	address := listener.Addr().String()
	payload := make([]byte, payloadSize)
	if _, err := rand.Read(payload); err != nil {
		_ = listener.Close()
		return roleCarriageMeasurement{}, err
	}
	notAfter := time.Now().UTC().Add(time.Minute).Truncate(time.Second)
	server := newRoleCarrier(ctx, listener, serverCertificate, clientKey, capacity, payload, notAfter)
	defer server.Close()
	samples := newLinuxSampler(ctx, sampleInterval)
	defer samples.Stop()
	started, goroutines := time.Now(), runtime.NumGoroutine()
	go server.Run()
	clientErrors := make(chan error, capacity)
	var clients sync.WaitGroup
	for range capacity {
		clients.Add(1)
		go func() {
			defer clients.Done()
			clientErrors <- carryHeld(ctx, address, client, serverKey, payload, notAfter)
		}()
	}
	if err := server.WaitForAdmissions(ctx); err != nil {
		return roleCarriageMeasurement{}, err
	}
	if err := server.WaitForWithdrawal(ctx); err != nil {
		return roleCarriageMeasurement{}, err
	}
	refused, err := refusedDial(address)
	if err != nil {
		return roleCarriageMeasurement{}, err
	}
	select {
	case <-time.After(hold):
	case <-ctx.Done():
		return roleCarriageMeasurement{}, ctx.Err()
	}
	server.BeginDrain()
	clients.Wait()
	close(clientErrors)
	for err := range clientErrors {
		if err != nil {
			return roleCarriageMeasurement{}, err
		}
	}
	if err := server.Wait(ctx); err != nil {
		return roleCarriageMeasurement{}, err
	}
	linuxSamples, err := samples.Stop()
	if err != nil {
		return roleCarriageMeasurement{}, err
	}
	return roleCarriageMeasurement{
		Schema: "ardents-r092-native-role-carriage-v1", Host: host, Capacity: capacity, PayloadBytes: payloadSize,
		AdmittedLegs: capacity, RefusedDials: refused, Withdrawn: true, HoldNanoseconds: hold.Nanoseconds(),
		ElapsedNanoseconds: time.Since(started).Nanoseconds(), GoroutinesBefore: goroutines,
		GoroutinesAfter: runtime.NumGoroutine(), LinuxSamples: linuxSamples,
	}, nil
}

type roleCarrier struct {
	ctx         context.Context
	listener    net.Listener
	certificate tls.Certificate
	clientKey   ed25519.PublicKey
	slots       chan struct{}
	payload     []byte
	notAfter    time.Time
	admitted    chan struct{}
	draining    chan struct{}
	withdrawn   chan struct{}
	done        chan struct{}

	workers      sync.WaitGroup
	withdrawOnce sync.Once
	drainOnce    sync.Once
	closeContext func() bool
	errMu        sync.Mutex
	err          error
	activeMu     sync.Mutex
	active       map[net.Conn]struct{}
}

func newRoleCarrier(ctx context.Context, listener net.Listener, certificate tls.Certificate, clientKey ed25519.PublicKey,
	capacity int, payload []byte, notAfter time.Time,
) *roleCarrier {
	result := &roleCarrier{ctx: ctx, listener: listener, certificate: certificate, clientKey: append(ed25519.PublicKey(nil), clientKey...),
		slots: make(chan struct{}, capacity), payload: append([]byte(nil), payload...), notAfter: notAfter,
		admitted: make(chan struct{}, capacity), draining: make(chan struct{}), withdrawn: make(chan struct{}), done: make(chan struct{}),
		active: make(map[net.Conn]struct{})}
	result.closeContext = context.AfterFunc(ctx, func() {
		result.Withdraw()
		result.BeginDrain()
		result.closeActive()
	})
	return result
}

func (carrier *roleCarrier) Run() {
	defer close(carrier.done)
	defer carrier.workers.Wait()
	for {
		raw, err := carrier.listener.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) || carrier.ctx.Err() != nil {
				return
			}
			carrier.record(err)
			return
		}
		select {
		case carrier.slots <- struct{}{}:
			carrier.track(raw)
			carrier.workers.Add(1)
			go carrier.Carry(raw)
			if len(carrier.slots) == cap(carrier.slots) {
				carrier.Withdraw()
			}
		default:
			_ = raw.Close()
		}
	}
}

func (carrier *roleCarrier) Carry(raw net.Conn) {
	defer carrier.workers.Done()
	defer func() { <-carrier.slots }()
	defer carrier.untrack(raw)
	secured := tls.Server(raw, config(carrier.certificate, carrier.clientKey, true))
	defer secured.Close()
	if err := secured.HandshakeContext(carrier.ctx); err != nil {
		carrier.record(err)
		return
	}
	if err := route.AcceptNodeLegBinding(secured, binding(0, false, carrier.notAfter)); err != nil {
		carrier.record(err)
		return
	}
	got := make([]byte, len(carrier.payload))
	if _, err := io.ReadFull(secured, got); err != nil || !bytes.Equal(got, carrier.payload) {
		carrier.record(errors.New("role-carriage opaque payload is invalid"))
		return
	}
	select {
	case carrier.admitted <- struct{}{}:
	case <-carrier.ctx.Done():
		return
	}
	select {
	case <-carrier.draining:
	case <-carrier.ctx.Done():
		return
	}
	if err := writeAll(secured, got); err != nil {
		carrier.record(err)
	}
}

func (carrier *roleCarrier) WaitForAdmissions(ctx context.Context) error {
	for range cap(carrier.slots) {
		select {
		case <-carrier.admitted:
		case <-carrier.done:
			return carrier.Err()
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

func (carrier *roleCarrier) WaitForWithdrawal(ctx context.Context) error {
	select {
	case <-carrier.withdrawn:
		return nil
	case <-carrier.done:
		return carrier.Err()
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (carrier *roleCarrier) BeginDrain() {
	carrier.drainOnce.Do(func() { close(carrier.draining) })
}

func (carrier *roleCarrier) Withdraw() {
	carrier.withdrawOnce.Do(func() {
		_ = carrier.listener.Close()
		close(carrier.withdrawn)
	})
}

func (carrier *roleCarrier) Close() {
	carrier.closeContext()
	carrier.Withdraw()
	carrier.BeginDrain()
	carrier.closeActive()
}

func (carrier *roleCarrier) Wait(ctx context.Context) error {
	select {
	case <-carrier.done:
		return carrier.Err()
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (carrier *roleCarrier) Err() error {
	carrier.errMu.Lock()
	defer carrier.errMu.Unlock()
	return carrier.err
}

func (carrier *roleCarrier) record(err error) {
	if err == nil {
		return
	}
	carrier.errMu.Lock()
	defer carrier.errMu.Unlock()
	if carrier.err == nil {
		carrier.err = err
	}
}

func (carrier *roleCarrier) track(connection net.Conn) {
	carrier.activeMu.Lock()
	defer carrier.activeMu.Unlock()
	carrier.active[connection] = struct{}{}
}

func (carrier *roleCarrier) untrack(connection net.Conn) {
	carrier.activeMu.Lock()
	defer carrier.activeMu.Unlock()
	delete(carrier.active, connection)
}

func (carrier *roleCarrier) closeActive() {
	carrier.activeMu.Lock()
	defer carrier.activeMu.Unlock()
	for connection := range carrier.active {
		_ = connection.Close()
	}
}

func carryHeld(ctx context.Context, address string, certificate tls.Certificate, server ed25519.PublicKey, payload []byte,
	notAfter time.Time,
) error {
	raw, err := (&net.Dialer{}).DialContext(ctx, "tcp", address)
	if err != nil {
		return err
	}
	secured := tls.Client(raw, config(certificate, server, false))
	defer secured.Close()
	if err := secured.HandshakeContext(ctx); err != nil {
		return err
	}
	if err := route.ConfirmNodeLegBinding(secured, binding(0, true, notAfter)); err != nil {
		return err
	}
	if err := writeAll(secured, payload); err != nil {
		return err
	}
	echo := make([]byte, len(payload))
	if _, err := io.ReadFull(secured, echo); err != nil || !bytes.Equal(echo, payload) {
		return errors.New("role-carriage opaque payload echo is invalid")
	}
	return nil
}

func refusedDial(address string) (int, error) {
	connection, err := net.DialTimeout("tcp", address, 250*time.Millisecond)
	if err == nil {
		_ = connection.Close()
		return 0, errors.New("withdrawn role-carriage listener accepted a new connection")
	}
	return 1, nil
}
