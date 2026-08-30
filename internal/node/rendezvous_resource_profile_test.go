package node_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/node"
)

func TestFunctionalAlphaResourceProfileAcceptsOnlyRendezvousDuty(t *testing.T) {
	_, identity, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	stateErr := errors.New("state unavailable for profile acceptance test")
	base := node.Config{
		IdentityKey: identity, NetworkStateRoot: t.TempDir(), LocalRoleStateRoot: t.TempDir(), PollInterval: time.Second,
		ResourceProfile: "ardents-rendezvous-dedicated-host-v1",
		Current:         func() (node.DutyView, error) { return nil, stateErr },
		Emit:            func(context.Context, node.Event) error { return nil },
		CheckPlacement:  func() error { return nil },
	}
	base.Rendezvous = node.RendezvousProfile{Certificate: tls.Certificate{PrivateKey: identity}}
	if _, runErr := node.Run(context.Background(), base); !errors.Is(runErr, stateErr) {
		t.Fatalf("Rendezvous profile run error = %v, want State boundary error", runErr)
	}
	base.ResourceProfile = "h4-5-rendezvous-alpha-v1"
	if _, runErr := node.Run(context.Background(), base); runErr == nil || !strings.Contains(runErr.Error(), "not supported") {
		t.Fatalf("legacy planning identity runtime error = %v", runErr)
	}

	base.ResourceProfile = "ardents-rendezvous-dedicated-host-v1"
	base.Rendezvous = node.RendezvousProfile{}
	base.Initiator = node.InitiatorProfile{Certificate: tls.Certificate{PrivateKey: identity}}
	if _, runErr := node.Run(context.Background(), base); runErr == nil || !strings.Contains(runErr.Error(), "Rendezvous duty") {
		t.Fatalf("non-Rendezvous profile run error = %v", runErr)
	}
}
