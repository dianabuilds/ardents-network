//go:build linux

package node_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/admission/receiving"
	"github.com/dianabuilds/ardents-network/internal/domain/endpoint/funding"
	"github.com/dianabuilds/ardents-network/internal/domain/endpoint/session"
	"github.com/dianabuilds/ardents-network/internal/domain/hosting"
	"github.com/dianabuilds/ardents-network/internal/route/capsule"
	"github.com/dianabuilds/ardents-network/internal/route/introduction"
	"github.com/dianabuilds/ardents-network/internal/route/prefix"
	"github.com/dianabuilds/ardents-network/internal/route/receiver"
	"github.com/dianabuilds/ardents-network/internal/route/role"
	"github.com/dianabuilds/ardents-network/internal/route/selection"
	"github.com/dianabuilds/ardents-network/tests/e2e/node/processfixture"
)

// Genuine signed Network, Stock, receiving spend, Hosting, REGISTER and Source
// command composition carry unchanged opaque bytes over TCP/TLS. The input
// is deliberately unauthenticated and its holder replies refusal; this proves
// transport reachability, not a qualified Publisher or successful Connection.
func TestRouteSourceSubmissionRefusalTCP(t *testing.T) {
	fixture, authority, _, sockets, certificates := openRouteReceivingFixture(t, 1, false, true)
	current := fixture.State.CurrentRuntime
	facts, _, err := authority.ObserveFacts()
	if err != nil {
		t.Fatal(err)
	}
	holder := processfixture.IntroductionStock(t, fixture.Spec, fixture.Plan, current)
	for _, id := range [][32]byte{{12}, {13}, {14}, {15}, {21}, {22}, {23}, {24}, {25}} {
		view, err := current()
		if err != nil {
			t.Fatal(err)
		}
		member, err := view.Member(id, view.ObservedAt())
		if err != nil {
			t.Fatal(err)
		}
		duty, err := view.RetainDuty(id, view.ObservedAt())
		if err != nil {
			t.Fatal(err)
		}
		binding := receiving.Receiver{NetworkID: facts.NetworkID, StateGeneration: facts.StateGeneration, StateDigest: facts.StateDigest, ProfileDigest: facts.Digest, NodeID: id, DutyGeneration: 9}
		owner, err := receiving.Open(t.TempDir(), binding, func() (receiving.Observation, error) { return authority.Receiver(binding, member.NotAfter()) })
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := owner.Close(); err != nil {
				t.Error(err)
			}
		})
		budget := openReceivingFixtureBudget(t)
		config := receiver.ReceiverConfig{Authority: role.Authority{Current: current, Duty: duty, Profile: view.Profile().ProfileBinding}, Certificate: certificates[id]}
		if id == [32]byte{25} {
			introductionRoot := t.TempDir()
			if err := os.Chmod(introductionRoot, 0700); err != nil {
				t.Fatal(err)
			}
			config.Introduction = openRegistrationFixtureRegistry(t, introductionRoot, owner, binding)
		}
		config.Admit = func(ctx context.Context, channel receiver.Channel, raw []byte) (receiving.Grant, error) {
			class, err := role.AdmissionClass(channel.Hello.Purpose)
			if err != nil {
				return receiving.Grant{}, err
			}
			return owner.Accept(ctx, class, raw, channel.Hello.Deadline, func() (func() error, error) {
				release, err := reserveReceivingFixtureWork(ctx, budget, channel.Hello.Deadline)
				if err != nil {
					return nil, err
				}
				return channel.HoldReservation(release)
			})
		}
		sockets[id]()
		server, err := receiver.Listen(t.Context(), config)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := server.Close(); err != nil {
				t.Errorf("receiver %d retirement: %v", id[0], err)
			}
		})
	}
	plan := funding.Plan{Deadline: time.Now().Add(40 * time.Second).UTC().Truncate(time.Second), Work: hosting.Traffic{Tx: 2 << 20, Rx: 2 << 20}, Termination: hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}}
	root := t.TempDir()
	selected, err := selection.Open(selection.Config{EntryRoot: filepath.Join(root, "entry"), InteriorRoot: filepath.Join(root, "interior"), Domain: 4, Current: current})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := selected.Close(); err != nil {
			t.Error(err)
		}
	}()
	budget, err := hosting.Open(processfixture.ProcessBudget(t))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := budget.Close(); err != nil {
			t.Error(err)
		}
	}()
	routeAuthority := funding.Observe(authority.Current, authority.Witness, authority.Presentation)
	prepared, err := funding.PreparePrefix(t.Context(), plan, selected, budget, routeAuthority, holder)
	if err != nil {
		t.Fatal(err)
	}
	intro, err := prefix.Open(t.Context(), prepared)
	if err != nil {
		t.Fatal(errors.Join(err, prepared.Release()))
	}
	defer func() {
		if err := intro.Close(); err != nil {
			t.Error(err)
		}
	}()
	view, err := current()
	if err != nil {
		t.Fatal(err)
	}
	duty, err := prepared.Leg.IntroductionDuty(view, nil)
	if err != nil {
		t.Fatal(err)
	}
	registration, err := introduction.Register(t.Context(), intro, introduction.RegistrationConfig{Duty: duty, Revision: 1, Deadline: plan.Deadline})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := registration.Close(); err != nil {
			t.Error(err)
		}
	}()
	sourceRoot := t.TempDir()
	sourcePlan := plan
	sourcePlan.EntryRoot, sourcePlan.InteriorRoot, sourcePlan.HostingRoot, sourcePlan.Domain = filepath.Join(sourceRoot, "entry"), filepath.Join(sourceRoot, "interior"), processfixture.ProcessBudget(t), 1
	source, err := session.OpenPrefix(t.Context(), sourcePlan, routeAuthority, holder)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := source.Close(); err != nil {
			t.Error(err)
		}
	}()
	receipt, err := registration.Receipt()
	if err != nil {
		t.Fatal(err)
	}
	raw := make([]byte, 474)
	registrationFacts := receipt.Facts()
	copy(raw[:32], registrationFacts.Slot[:])
	binary.BigEndian.PutUint64(raw[32:40], 1)
	binary.BigEndian.PutUint64(raw[40:48], uint64(time.Now().Add(8*time.Second).Unix()))
	raw[48], raw[80], raw[114] = 30, 40, 50
	binary.BigEndian.PutUint16(raw[112:114], 360)
	if _, err := capsule.Parse(raw); err != nil {
		t.Fatal(err)
	}
	replied := make(chan error, 1)
	go func() {
		child, err := registration.NextDelivery(t.Context())
		if err == nil {
			if !bytes.Equal(child.Capsule().Bytes(), raw) {
				err = errors.New("Introduction changed opaque capsule")
			} else {
				err = child.Reply(child.Context(), 1)
			}
			child.Close()
		}
		replied <- err
	}()
	err = source.SubmitCapsule(t.Context(), session.Recipient{Node: duty.NodeID, Generation: duty.RecordGeneration}, raw)
	if !submissionRefusalOnly(err) {
		t.Fatal("genuine exact refusal/physical join differs", err)
	}
	if err := <-replied; err != nil {
		t.Fatal("recipient refusal did not join matching CLOSE", err)
	}
	if err := registration.Withdraw(t.Context()); err != nil {
		t.Fatal("owning withdrawal after delivery", err)
	}
}

func submissionRefusalOnly(err error) bool {
	if err == nil {
		return false
	}
	if refusal, ok := err.(introduction.SubmissionRefusal); ok {
		return refusal.Status == 1
	}
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		for _, child := range joined.Unwrap() {
			if !submissionRefusalOnly(child) {
				return false
			}
		}
		return len(joined.Unwrap()) > 0
	}
	return false
}
