package node

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route/credential"
)

type transitIssuerListener struct {
	listener net.Listener
	server   *http.Server
	issuer   *credential.Issuer
	limit    chan struct{}
	active   atomic.Uint32
	protect  atomic.Bool
	stopOnce sync.Once
	done     chan error
}

func startTransitIssuer(config runtimeConfig, snapshot dutyFacts) (*probeServer, error) {
	local := config.TransitIssuer
	if err := validateTransitIssuerProfile(local, snapshot, config.now()); err != nil {
		return nil, err
	}
	current := func() (credential.StateDuty, bool) {
		updated, err := currentFacts(config)
		if err != nil {
			return credential.StateDuty{}, false
		}
		return transitIssuerStateDuty(updated, config.now())
	}
	issuer, err := credential.OpenIssuerFromRoot(credential.RootIssuerConfig{Root: local.Root, NetworkID: snapshot.NetworkID,
		NodeID: snapshot.NodeID, IdentityKey: config.IdentityKey, CurrentDuty: current, Clock: config.now})
	if err != nil {
		return nil, err
	}
	certificate := local.Certificate
	if certificate.Leaf == nil && len(certificate.Certificate) > 0 {
		certificate.Leaf, err = x509.ParseCertificate(certificate.Certificate[0])
	}
	tlsConfig, tlsErr := issuer.TLSConfig(certificate)
	if err != nil || tlsErr != nil {
		return nil, errors.Join(err, tlsErr, issuer.Close())
	}
	listener, err := net.Listen("tcp", snapshot.ProbeEndpoint)
	if err != nil {
		return nil, errors.Join(err, issuer.Close())
	}
	running := &transitIssuerListener{listener: listener, issuer: issuer, limit: make(chan struct{}, local.ConnectionLimit), done: make(chan error, 1)}
	limited := &transitIssuerLimitedListener{Listener: listener, running: running}
	running.server = &http.Server{Handler: issuer.Handler(), ReadHeaderTimeout: time.Second, ReadTimeout: 15 * time.Second,
		WriteTimeout: 15 * time.Second, IdleTimeout: time.Second, MaxHeaderBytes: 1024}
	go func() {
		serveErr := running.server.Serve(tls.NewListener(limited, tlsConfig))
		if errors.Is(serveErr, http.ErrServerClosed) || errors.Is(serveErr, net.ErrClosed) {
			serveErr = nil
		}
		running.done <- serveErr
	}()
	return &probeServer{Done: running.done, Protect: running.protectAdmission, Usage: running.usage,
		Stop: running.stopAdmission, Drain: func(ctx context.Context) { running.drain(ctx, local.DrainTimeout) }}, nil
}

type transitIssuerLimitedListener struct {
	net.Listener
	running *transitIssuerListener
}

func (listener *transitIssuerLimitedListener) Accept() (net.Conn, error) {
	for {
		connection, err := listener.Listener.Accept()
		if err != nil {
			return nil, err
		}
		if listener.running.protect.Load() {
			_ = connection.Close()
			continue
		}
		select {
		case listener.running.limit <- struct{}{}:
			listener.running.active.Add(1)
			return &transitIssuerConnection{Conn: connection, release: func() {
				<-listener.running.limit
				listener.running.active.Add(^uint32(0))
			}}, nil
		default:
			_ = connection.Close()
		}
	}
}

type transitIssuerConnection struct {
	net.Conn
	once    sync.Once
	release func()
}

func (connection *transitIssuerConnection) Close() error {
	err := connection.Conn.Close()
	connection.once.Do(connection.release)
	return err
}

func (running *transitIssuerListener) protectAdmission(value bool) { running.protect.Store(value) }
func (running *transitIssuerListener) usage() (uint64, uint64, uint64) {
	return uint64(running.active.Load()), uint64(running.active.Load()), 0
}
func (running *transitIssuerListener) stopAdmission() {
	running.stopOnce.Do(func() { _ = running.listener.Close() })
}
func (running *transitIssuerListener) drain(ctx context.Context, timeout time.Duration) {
	running.stopAdmission()
	drainCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	_ = running.server.Shutdown(drainCtx)
	_ = running.issuer.Close()
}
