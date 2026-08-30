//go:build browsercompat

package main

import (
	"context"
	"crypto/ed25519"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/dianabuilds/ardents-network/internal/service/reachability"
)

// runGateway is the separately run Destination Resolution Gateway fixture. It
// receives a Publisher-produced descriptor only through the fixture's bounded
// publication handoff and never receives a Target Link from the User process.
func runGateway(input config) error {
	deadline, _ := input.deadline()
	envelope, err := waitForPublication(context.Background(), input.PublicationPath, deadline)
	if err != nil {
		return err
	}
	raw, err := base64.RawStdEncoding.DecodeString(envelope.Descriptor)
	if err != nil || len(raw) == 0 {
		return errors.New("C2 fixture reachability descriptor is invalid")
	}
	network, _ := fixed(input.Network)
	gateway, err := input.Gateway.decode()
	if err != nil {
		return err
	}
	certificate, err := input.Gateway.tlsCertificate()
	if err != nil {
		return err
	}
	identity, ok := certificate.PrivateKey.(ed25519.PrivateKey)
	if !ok {
		return errors.New("C2 fixture Gateway key is invalid")
	}
	store, err := reachability.OpenStore(reachability.StoreConfig{Root: input.GatewayRoot, NetworkID: network})
	if err != nil {
		return err
	}
	defer store.Close()
	running, err := reachability.NewGateway(reachability.GatewayConfig{NetworkID: network, NodeID: gateway.NodeID, IdentityKey: identity,
		AssignmentNotAfter: deadline, Store: store, Clock: func() time.Time { return time.Now().UTC() },
		AuthorizeDescriptor: func(value reachability.Descriptor, at time.Time) bool {
			introduction, introductionErr := input.Introduction.decode()
			rendezvous, rendezvousErr := input.Rendezvous.decode()
			digest, digestErr := fixed(input.Digest)
			return introductionErr == nil && rendezvousErr == nil && digestErr == nil && at.Before(deadline) &&
				value.Introduction.StateDigest == digest && value.Introduction.Epoch == input.Epoch &&
				value.Introduction.IntroductionNodeID == introduction.NodeID && value.Introduction.RendezvousNodeID == rendezvous.NodeID
		}})
	if err != nil {
		return err
	}
	if result, err := running.Publish(raw, time.Now().UTC()); err != nil || result.Class != reachability.StoreAccepted {
		return errors.Join(err, errors.New("C2 fixture Gateway did not accept descriptor"))
	}
	listener, err := net.Listen("tcp", gateway.Endpoint)
	if err != nil {
		return err
	}
	server := &http.Server{Handler: running.Handler()}
	secured := tls.NewListener(listener, &tls.Config{MinVersion: tls.VersionTLS13, Certificates: []tls.Certificate{certificate}})
	serve := make(chan error, 1)
	go func() { serve <- server.Serve(secured) }()
	if err := writeGatewayProfile(input.GatewayProfilePath, running.Profile()); err != nil {
		_ = server.Close()
		return err
	}
	if err := writeTransitReady(input.ReadyRoot, "gateway"); err != nil {
		_ = server.Close()
		return err
	}
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()
	if err := waitForTransitCompletion(ctx, input.CompletePath); err != nil {
		_ = server.Close()
		return err
	}
	if err := server.Shutdown(ctx); err != nil {
		return err
	}
	if err := <-serve; err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return jsonResult("gateway", "drained")
}

func writeGatewayProfile(path string, profile reachability.GatewayProfile) error {
	if filepath.Dir(path) == "." {
		return errors.New("C2 fixture Gateway profile path is invalid")
	}
	raw, err := json.Marshal(profile)
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o600)
}

func readGatewayProfile(path string) (reachability.GatewayProfile, error) {
	raw, err := os.ReadFile(path)
	if err != nil || len(raw) == 0 || len(raw) > 8<<10 {
		return reachability.GatewayProfile{}, errors.New("C2 fixture Gateway profile is unavailable")
	}
	var profile reachability.GatewayProfile
	if err := json.Unmarshal(raw, &profile); err != nil {
		return reachability.GatewayProfile{}, err
	}
	return profile, nil
}

func waitForPublication(ctx context.Context, path string, deadline time.Time) (publicationEnvelope, error) {
	attempt, cancel := context.WithDeadline(ctx, deadline)
	defer cancel()
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		if value, err := readPublication(path); err == nil && value.Descriptor != "" {
			return value, nil
		}
		select {
		case <-attempt.Done():
			return publicationEnvelope{}, errors.New("C2 fixture Gateway publication is unavailable")
		case <-ticker.C:
		}
	}
}
