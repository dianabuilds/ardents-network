package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/dianabuilds/ardents-network/internal/node"
	"github.com/dianabuilds/ardents-network/internal/route"
)

type drainingTransit interface {
	Drain(context.Context) error
	Close() error
}

func runTransitRole(input config, role string) error {
	deadline, _ := input.deadline()
	var running drainingTransit
	var err error
	switch role {
	case "rendezvous":
		running, err = openRendezvous(input, deadline)
	case "initiator":
		running, err = openInitiator(input, deadline)
	case "introduction":
		running, err = openIntroduction(input, deadline)
	case "responder":
		running, err = openResponder(input, deadline)
	default:
		return errors.New("C2 fixture transit role is unsupported")
	}
	if err != nil {
		return err
	}
	defer running.Close()
	if err := writeTransitReady(input.ReadyRoot, role); err != nil {
		return err
	}
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()
	if err := waitForTransitCompletion(ctx, input.CompletePath); err != nil {
		return err
	}
	if err := running.Drain(ctx); err != nil {
		return err
	}
	if err := verifyTransitUsage(role, running); err != nil {
		return err
	}
	return jsonResult(role, "drained")
}

func openRendezvous(input config, deadline time.Time) (*node.Rendezvous, error) {
	rendezvous, err := input.Rendezvous.decode()
	if err != nil {
		return nil, err
	}
	initiator, err := input.Initiator.decode()
	if err != nil {
		return nil, err
	}
	responder, err := input.Responder.decode()
	if err != nil {
		return nil, err
	}
	certificate, err := input.Rendezvous.tlsCertificate()
	if err != nil {
		return nil, err
	}
	network, _ := fixed(input.Network)
	digest, _ := fixed(input.Digest)
	return node.StartRendezvous(node.RendezvousConfig{ListenAddress: rendezvous.Endpoint, Certificate: certificate,
		NetworkID: network, EpochDigest: digest, NodeID: rendezvous.NodeID, NodePublicKey: rendezvous.PublicKey, Epoch: input.Epoch, NotAfter: deadline,
		Peers: []node.RendezvousPeer{{NodeID: initiator.NodeID, PublicKey: initiator.PublicKey, Role: route.InitiatorRole},
			{NodeID: responder.NodeID, PublicKey: responder.PublicKey, Role: route.ResponderRole}},
		HandshakeLimit: 2, WaitingLimit: 2, PairLimit: 1, PairByteLimit: 256 << 10, DrainTimeout: time.Second})
}

func openInitiator(input config, deadline time.Time) (*node.Initiator, error) {
	initiator, err := input.Initiator.decode()
	if err != nil {
		return nil, err
	}
	rendezvous, err := input.Rendezvous.decode()
	if err != nil {
		return nil, err
	}
	certificate, err := input.Initiator.tlsCertificate()
	if err != nil {
		return nil, err
	}
	network, _ := fixed(input.Network)
	digest, _ := fixed(input.Digest)
	attachment, _ := fixed(input.ServiceAttachment)
	resolutionAttachment, _ := fixed(input.ResolutionAttachment)
	gateway, err := input.Gateway.decode()
	if err != nil {
		return nil, err
	}
	inviteID, _ := fixed(input.InviteID)
	return node.StartInitiator(node.InitiatorConfig{ListenAddress: initiator.Endpoint, Certificate: certificate,
		NetworkID: network, EpochDigest: digest, NodeID: initiator.NodeID, NodePublicKey: initiator.PublicKey, Epoch: input.Epoch, NotAfter: deadline,
		Rendezvous:        node.InitiatorPeer{NodeID: rendezvous.NodeID, PublicKey: rendezvous.PublicKey, Endpoint: rendezvous.Endpoint},
		ResolutionGateway: node.ResolutionGateway{NodeID: gateway.NodeID, PublicKey: gateway.PublicKey, URL: "https://" + gateway.Endpoint},
		Admit: func(raw []byte, received, key [32]byte, notAfter time.Time) (route.EntryAdmission, error) {
			service := received == attachment && notAfter.Equal(deadline)
			resolution := received == resolutionAttachment && time.Now().UTC().Before(notAfter) && notAfter.Before(deadline) && !notAfter.After(time.Now().UTC().Add(15*time.Second))
			if string(raw) != input.Invite || key == [32]byte{} || (!service && !resolution) {
				return route.EntryAdmission{}, errors.New("unexpected process Entry admission")
			}
			return route.EntryAdmission{InviteID: inviteID, NetworkID: network, Digest: digest, Epoch: input.Epoch, InitiatorNodeID: initiator.NodeID, NotAfter: deadline}, nil
		}, HandshakeLimit: 2, RelayLimit: 1, RelayByteLimit: 256 << 10, DrainTimeout: time.Second})
}

func openIntroduction(input config, deadline time.Time) (*node.Introduction, error) {
	introduction, err := input.Introduction.decode()
	if err != nil {
		return nil, err
	}
	certificate, err := input.Introduction.tlsCertificate()
	if err != nil {
		return nil, err
	}
	network, _ := fixed(input.Network)
	digest, _ := fixed(input.Digest)
	return node.StartIntroduction(node.IntroductionConfig{ListenAddress: introduction.Endpoint, Certificate: certificate,
		NetworkID: network, EpochDigest: digest, NodeID: introduction.NodeID, NodePublicKey: introduction.PublicKey, Epoch: input.Epoch, NotAfter: deadline,
		Admit: func(raw []byte, attachment, key [32]byte, role byte, nodeID [32]byte, notAfter time.Time) (route.EndpointTransitAdmission, error) {
			if len(raw) == 0 || attachment == [32]byte{} || key == [32]byte{} || role != route.IntroductionRole || nodeID != introduction.NodeID || !notAfter.Equal(deadline) {
				return route.EndpointTransitAdmission{}, errors.New("unexpected process Introduction admission")
			}
			return route.EndpointTransitAdmission{AuthorizationID: identifier(81), NetworkID: network, Digest: digest, Epoch: input.Epoch,
				TransitRole: route.IntroductionRole, TransitNodeID: introduction.NodeID, NotAfter: deadline}, nil
		}, HandshakeLimit: 3, SlotLimit: 1, DeliveryLimit: 1, DrainTimeout: time.Second})
}

func openResponder(input config, deadline time.Time) (*node.Responder, error) {
	responder, err := input.Responder.decode()
	if err != nil {
		return nil, err
	}
	rendezvous, err := input.Rendezvous.decode()
	if err != nil {
		return nil, err
	}
	certificate, err := input.Responder.tlsCertificate()
	if err != nil {
		return nil, err
	}
	network, _ := fixed(input.Network)
	digest, _ := fixed(input.Digest)
	attachment, _ := fixed(input.ServiceAttachment)
	return node.StartResponder(node.ResponderConfig{ListenAddress: responder.Endpoint, Certificate: certificate,
		NetworkID: network, EpochDigest: digest, NodeID: responder.NodeID, NodePublicKey: responder.PublicKey, Epoch: input.Epoch, NotAfter: deadline,
		Rendezvous: node.ResponderPeer{NodeID: rendezvous.NodeID, PublicKey: rendezvous.PublicKey, Endpoint: rendezvous.Endpoint},
		Admit: func(raw []byte, received, key [32]byte, role byte, nodeID [32]byte, notAfter time.Time) (route.EndpointTransitAdmission, error) {
			if string(raw) != input.ResponderAuthorization || received != attachment || key == [32]byte{} || role != route.ResponderRole || nodeID != responder.NodeID || !notAfter.Equal(deadline) {
				return route.EndpointTransitAdmission{}, errors.New("unexpected process Responder admission")
			}
			return route.EndpointTransitAdmission{AuthorizationID: identifier(82), NetworkID: network, Digest: digest, Epoch: input.Epoch,
				TransitRole: route.ResponderRole, TransitNodeID: responder.NodeID, NotAfter: deadline}, nil
		}, HandshakeLimit: 2, RelayLimit: 1, RelayByteLimit: 256 << 10, DrainTimeout: time.Second})
}

func (value peer) tlsCertificate() (tls.Certificate, error) {
	if value.Certificate == "" || value.PrivateKey == "" {
		return tls.Certificate{}, errors.New("C2 fixture transit certificate is unavailable")
	}
	return tls.X509KeyPair([]byte(value.Certificate), []byte(value.PrivateKey))
}

func writeTransitReady(root, role string) error {
	if root == "" || filepath.Base(root) == "." || filepath.Base(root) == string(filepath.Separator) {
		return errors.New("C2 fixture ready root is invalid")
	}
	file, err := os.OpenFile(filepath.Join(root, role), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	if _, err := file.WriteString("ready\n"); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}

func waitForTransitCompletion(ctx context.Context, path string) error {
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() && info.Size() > 0 {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("C2 fixture transit completion is unavailable: %w", ctx.Err())
		case <-ticker.C:
		}
	}
}

func verifyTransitUsage(role string, running drainingTransit) error {
	switch value := running.(type) {
	case *node.Initiator:
		usage := value.Usage()
		if role != "initiator" || usage.CompletedRelays != 2 || usage.ActiveRelays != 0 || usage.Connections != 0 {
			return fmt.Errorf("Initiator terminal usage = %+v", usage)
		}
	case *node.Introduction:
		usage := value.Usage()
		if role != "introduction" || usage.Deliveries != 0 || usage.Slots != 0 || usage.Connections != 0 || usage.Registered != 1 || usage.Delivered != 1 {
			return fmt.Errorf("Introduction terminal usage = %+v", usage)
		}
	case *node.Responder:
		usage := value.Usage()
		if role != "responder" || usage.CompletedRelays != 1 || usage.ActiveRelays != 0 || usage.Connections != 0 {
			return fmt.Errorf("Responder terminal usage = %+v", usage)
		}
	case *node.Rendezvous:
		usage := value.Usage()
		if role != "rendezvous" || usage.CompletedPairs != 1 || usage.ActivePairs != 0 || usage.Connections != 0 {
			return fmt.Errorf("Rendezvous terminal usage = %+v", usage)
		}
	default:
		return errors.New("C2 fixture transit usage is unavailable")
	}
	return nil
}

func jsonResult(role, class string) error {
	return json.NewEncoder(os.Stdout).Encode(result{Schema: "ardents-e2e-reference-c2-result-v1", Role: role, Class: class, Passed: true})
}
