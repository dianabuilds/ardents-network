//go:build linux

package main

import (
	"context"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/receiving"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/spending"
)

func TestRegistrationInitializationFactUsesGenuineNetworkReceivingOwner(t *testing.T) {
	f := newNetworkAdmissionFixture(t, func(f *networkAdmissionFixture) { f.spec.Nodes[1].RoleDomain, f.spec.Nodes[1].Subrole = 4, 3 })
	binding := spending.Binding{NetworkID: f.receiver.NetworkID, ProfileDigest: f.receiver.ProfileDigest, ReceiverNodeID: f.receiver.NodeID, ReceiverDutyGeneration: f.receiver.DutyGeneration}
	observe := func() (receiving.Observation, error) { return f.authority.receiver(f.receiver, f.profile.NotAfter) }
	for _, attempt := range []string{"canceled", "absent reservation", "invalid token", "foreign refill", "closed owner"} {
		t.Run(attempt, func(t *testing.T) {
			root := t.TempDir()
			owner, err := receiving.Open(root, f.receiver, observe)
			if err != nil {
				t.Fatal(err)
			}
			defer owner.Close()
			fact, err := owner.TakeFreshRoot()
			if err != nil {
				t.Fatal(err)
			}
			initialization, err := fact.Begin(binding)
			if err != nil {
				t.Fatal(err)
			}
			if err := initialization.Check(); err != nil {
				t.Fatal(err)
			}
			ctx := t.Context()
			if attempt == "canceled" {
				canceled, cancel := context.WithCancel(ctx)
				cancel()
				ctx = canceled
			}
			reserve := func() (func() error, error) { t.Fatal("invalid attempt reached capacity"); return nil, nil }
			if attempt == "absent reservation" {
				reserve = nil
			}
			switch attempt {
			case "closed owner":
				err = owner.Close()
			case "foreign refill":
				_, err = owner.Refill(ctx, receiving.Grant{}, 0, nil, reserve)
			default:
				_, err = owner.Accept(ctx, admission.RegistrationClass, nil, time.Now().Add(time.Minute), reserve)
			}
			if attempt != "closed owner" && err == nil {
				t.Fatal("invalid admission succeeded")
			}
			if err := initialization.Check(); err == nil {
				t.Fatal("admission attempt retained effect authority")
			}
			if err := initialization.Complete(); err == nil {
				t.Fatal("late completion published initialization")
			}
			if _, err := owner.TakeFreshRoot(); err == nil {
				t.Fatal("reissued initialization after admission")
			}
			if err := owner.Close(); err != nil {
				t.Fatal(err)
			}
			reopened, err := receiving.Open(root, f.receiver, observe)
			if err != nil {
				t.Fatal(err)
			}
			defer reopened.Close()
			if _, err := reopened.TakeFreshRoot(); err == nil {
				t.Fatal("retained no-spend reopen issued initialization")
			}
		})
	}
}
