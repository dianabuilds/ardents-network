package nativecircuit

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"os"
	"time"
)

func runDirectUserRole(ctx context.Context, config roleConfig, evidenceDir string, result *roleResult) error {
	rootPEM, err := os.ReadFile(config.TargetRootPath)
	if err != nil {
		return err
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(rootPEM) {
		return errors.New("direct Target root is not a certificate")
	}
	leaf, err := parseDigest(config.ExpectedLeafSHA256)
	if err != nil {
		return err
	}
	nonce, err := parseRoleHandle(config.SlotHex)
	if err != nil {
		return err
	}
	if err := writeRoleReady(evidenceDir, config); err != nil {
		return err
	}
	if err := waitForRoleStart(ctx, config.StartPath); err != nil {
		return err
	}
	connection, err := (&net.Dialer{Timeout: setupDeadline}).DialContext(ctx, "tcp", config.DirectAddress)
	if err != nil {
		return fmt.Errorf("dial Direct Service: %w", err)
	}
	setupVerified := func() error { return writeRoleMarker(evidenceDir, config, "setup-ready.json", "authenticated") }
	var observation endpointObservation
	if config.StreamDirection != "" {
		observation, err = runEndpointUserStream(ctx, connection, endpointTrust{Roots: roots, LeafSHA256: leaf}, nonce, roleStreamSpec(config), setupVerified)
	} else {
		observation, err = runEndpointUserWithCallbacks(ctx, connection, endpointTrust{Roots: roots, LeafSHA256: leaf}, nonce, seededPayload(config.PayloadSeed, config.PayloadBytes), setupVerified, nil)
	}
	result.applyEndpoint(observation)
	result.ObservedFields = append(result.ObservedFields, "direct.service_address", "target.instance_certificate", "application.protected_stream")
	if err != nil {
		return err
	}
	return writeRoleMarker(evidenceDir, config, "attempt-ready.json", "completed")
}

func runDirectServiceRole(ctx context.Context, config roleConfig, evidenceDir string, result *roleResult) error {
	certificate, err := tls.LoadX509KeyPair(config.EndpointCertificate, config.EndpointPrivateKey)
	if err != nil {
		return err
	}
	nonce, err := parseRoleHandle(config.SlotHex)
	if err != nil {
		return err
	}
	listener, err := (&net.ListenConfig{}).Listen(ctx, "tcp", config.DirectAddress)
	if err != nil {
		return err
	}
	defer listener.Close()
	if err := writeRoleReady(evidenceDir, config); err != nil {
		return err
	}
	connection, err := acceptDirect(ctx, listener)
	if err != nil {
		return err
	}
	var observation endpointObservation
	if config.StreamDirection != "" {
		observation, err = runEndpointServiceStream(ctx, connection, certificate, nonce, roleStreamSpec(config))
	} else {
		observation, err = runEndpointService(ctx, connection, certificate, nonce)
	}
	result.applyEndpoint(observation)
	result.ObservedFields = append(result.ObservedFields, "direct.user_address", "application.protected_stream")
	if err != nil {
		return err
	}
	return writeRoleMarker(evidenceDir, config, "attempt-ready.json", "completed")
}

func acceptDirect(ctx context.Context, listener net.Listener) (net.Conn, error) {
	if tcp, ok := listener.(*net.TCPListener); ok {
		_ = tcp.SetDeadline(time.Now().Add(30 * time.Second))
	}
	connection, err := listener.Accept()
	if err != nil {
		return nil, err
	}
	stop := context.AfterFunc(ctx, func() { _ = connection.Close() })
	if !stop() && ctx.Err() != nil {
		_ = connection.Close()
		return nil, ctx.Err()
	}
	return connection, nil
}

func roleStreamSpec(config roleConfig) streamSpec {
	return streamSpec{Direction: config.StreamDirection, Seed: config.StreamSeed, Duration: time.Duration(config.StreamDuration) * time.Second}
}
