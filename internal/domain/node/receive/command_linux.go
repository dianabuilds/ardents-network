//go:build linux

package receive

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/domain/admission"
	"github.com/dianabuilds/ardents-network/internal/domain/admission/issuer"
	"github.com/dianabuilds/ardents-network/internal/domain/admission/quota"
	"github.com/dianabuilds/ardents-network/internal/domain/admission/receiving"
	"github.com/dianabuilds/ardents-network/internal/domain/hosting"
	"github.com/dianabuilds/ardents-network/internal/domain/network/state"
	nodeadmission "github.com/dianabuilds/ardents-network/internal/domain/node/admission"
	"github.com/dianabuilds/ardents-network/internal/domain/reachability"
	"github.com/dianabuilds/ardents-network/internal/domain/route/ardp"
	framing "github.com/dianabuilds/ardents-network/internal/domain/route/channel"
	"github.com/dianabuilds/ardents-network/internal/domain/route/introduction"
	routereceiver "github.com/dianabuilds/ardents-network/internal/domain/route/receiver"
	"github.com/dianabuilds/ardents-network/internal/domain/route/role"
	"github.com/dianabuilds/ardents-network/internal/pathcheck"
	"github.com/dianabuilds/ardents-network/internal/platform/artifacts"
)

func issueRouteBatch(ctx context.Context, authority routeAdmissionAuthority, plan issuer.Plan, batch []byte, bootstrap bool) ([]byte, error) {
	kind := quota.Admitted
	if bootstrap {
		kind = quota.Bootstrap
	}
	return issuer.IssueCurrentBatch(ctx, plan, batch, kind, func() (admission.AuthorityFacts, time.Time, error) {
		return authority.issuer(plan.KeyBinding.Signer)
	})
}

// RunRouteReceive owns the development receiving Route and its original borrowers.
func RunRouteReceive(ctx context.Context, args []string, out, diagnostic io.Writer) (code int) {
	var plan struct {
		Network          *state.AuthorityDeclaration `json:"network"`
		NodeID           [32]byte                    `json:"node_id"`
		SpendRoot        string                      `json:"spend_root"`
		IntroductionRoot string                      `json:"introduction_root"`
		DescriptorRoot   string                      `json:"descriptor_root"`
		HostingRoot      string                      `json:"hosting_root"`
		Certificate      string                      `json:"certificate"`
		PrivateKey       string                      `json:"private_key"`
		Work             hosting.Traffic             `json:"work"`
		Termination      hosting.Traffic             `json:"termination"`
		Issuer           *issuer.Plan                `json:"issuer,omitempty"`
	}
	if ctx == nil || len(args) < 1 || args[0] != "receive" || decodeRouteConfig(args[1:], &plan) != nil || plan.Network == nil ||
		!plan.Network.ValidShape() || !pathcheck.DisjointRoots(plan.Network.Root, plan.SpendRoot, plan.HostingRoot) {
		return 2
	}
	config, err := plan.Network.Config()
	if err != nil {
		return 2
	}
	networkOwner, err := state.Open(config)
	if err != nil {
		return 1
	}
	defer func() {
		if networkOwner.Close() != nil {
			code = 1
		}
	}()
	authority := routeAdmissionAuthority{current: networkOwner.CurrentRuntime}
	view, err := networkOwner.CurrentRuntime()
	if err != nil {
		return 1
	}
	m, err := view.Member(plan.NodeID, view.ObservedAt())
	if err != nil {
		return 1
	}
	if err := routereceiver.ValidateCarrier(m.CarrierProfile); err != nil {
		return 1
	}
	if plan.IntroductionRoot != "" && !pathcheck.DisjointRoots(plan.Network.Root, plan.SpendRoot, plan.HostingRoot, plan.IntroductionRoot) {
		return 2
	}
	duty, err := view.RetainDuty(plan.NodeID, view.ObservedAt())
	if err != nil {
		return 1
	}
	if (m.RoleDomain == 4 && m.Subrole == 3) != (plan.IntroductionRoot != "") {
		return 2
	}
	resolutionDuty := m.RoleDomain == 2 && m.Subrole == 5
	if resolutionDuty != (plan.DescriptorRoot != "") || plan.DescriptorRoot != "" && !pathcheck.DisjointRoots(plan.Network.Root, plan.SpendRoot, plan.HostingRoot, plan.DescriptorRoot) {
		return 2
	}
	issuerDuty := m.RoleDomain == 2 && m.Subrole == 6
	if issuerDuty != (plan.Issuer != nil) {
		return 2
	}
	var issue func(context.Context, []byte, bool) ([]byte, error)
	if plan.Issuer != nil {
		if plan.Issuer.KeyBinding.Issuer != m.NodeID || !pathcheck.DisjointRoots(plan.Network.Root, plan.SpendRoot, plan.HostingRoot, plan.Issuer.AdmissionRoot, plan.Issuer.KeyRoot, plan.Issuer.ResultRoot) {
			return 2
		}
		issue = func(ctx context.Context, batch []byte, bootstrap bool) ([]byte, error) {
			return issueRouteBatch(ctx, authority, *plan.Issuer, batch, bootstrap)
		}
	}
	if !pathcheck.IsCleanAbsolute(plan.Certificate) || !pathcheck.IsCleanAbsolute(plan.PrivateKey) {
		return 2
	}
	certificate, err := artifacts.ReadOwnerKeyPair(plan.Certificate, plan.PrivateKey)
	if err != nil {
		return 2
	}
	budget, err := hosting.Open(plan.HostingRoot)
	if err != nil {
		return 1
	}
	defer func() {
		if budget.Close() != nil {
			code = 1
		}
	}()
	p := view.ProfileBinding()
	receiver := receiving.Receiver{NetworkID: p.Network, StateGeneration: p.Generation, StateDigest: p.EpochDigest, ProfileDigest: p.Digest, NodeID: m.NodeID, DutyGeneration: m.DutyGeneration}
	admissionOwner, err := receiving.Open(plan.SpendRoot, receiver, func() (receiving.Observation, error) { return authority.receiver(receiver, m.NotAfter()) })
	if err != nil {
		return 1
	}
	defer func() {
		if admissionOwner.Close() != nil {
			code = 1
		}
	}()
	var introductionRegistry *introduction.Registry
	var reservationMu sync.Mutex
	reservations := make(map[routereceiver.Channel]*hosting.Reservation)
	if m.RoleDomain == 4 && m.Subrole == 3 {
		introductionRegistry, err = introduction.OpenReceivingRegistry(plan.IntroductionRoot, introduction.Binding{NetworkID: p.Network, ProfileDigest: p.Digest, ReceiverNodeID: m.NodeID, ReceiverDutyGeneration: m.DutyGeneration}, admissionOwner)
		if err != nil {
			return 1
		}
		defer func() {
			if introductionRegistry.Close() != nil {
				code = 1
			}
		}()
	}
	var descriptorStore *reachability.Store
	if resolutionDuty {
		descriptorStore, err = reachability.OpenStore(reachability.StoreConfig{Root: plan.DescriptorRoot, Network: p.Network})
		if err != nil {
			return 1
		}
		defer func() {
			if descriptorStore.Close() != nil {
				code = 1
			}
		}()
	}
	server, err := routereceiver.Listen(ctx, routereceiver.ReceiverConfig{Authority: role.Authority{Current: networkOwner.CurrentRuntime, Duty: duty, Profile: p}, Certificate: certificate, Introduction: introductionRegistry, Issue: issue, DescriptorStore: descriptorStore,
		ReserveBootstrap: func(ctx context.Context, end time.Time) (func() error, error) {
			reservation, err := budget.Reserve(ctx, hosting.ReservationRequest{Work: plan.Work, Termination: plan.Termination, WorkUntil: end, HoldUntil: end.Add(5 * time.Second)})
			if err != nil {
				return nil, err
			}
			return reservation.ReleaseJoined, nil
		},
		Admit: func(ctx context.Context, c routereceiver.Channel, raw []byte) (receiving.Grant, error) {
			class, err := role.AdmissionClass(c.Hello.Purpose)
			if err != nil {
				return receiving.Grant{}, err
			}
			return admissionOwner.Accept(ctx, class, raw, c.Hello.Deadline, func() (func() error, error) {
				reservation, err := budget.Reserve(ctx, hosting.ReservationRequest{Work: plan.Work, Termination: plan.Termination, WorkUntil: c.Hello.Deadline, HoldUntil: c.Hello.Deadline.Add(5 * time.Second)})
				if err != nil {
					return nil, err
				}
				if c.Hello.Purpose == ardp.PurposeIssuer {
					return nodeadmission.HoldIssuerReservation(c, reservation)
				}
				reservationMu.Lock()
				reservations[c] = reservation
				reservationMu.Unlock()
				return c.HoldReservation(func() error {
					reservationMu.Lock()
					delete(reservations, c)
					reservationMu.Unlock()
					return reservation.ReleaseJoined()
				})
			})
		},
		Refill: func(ctx context.Context, c routereceiver.Channel, original receiving.Grant, remaining uint64, raw []byte) (receiving.Grant, error) {
			reservationMu.Lock()
			held := reservations[c]
			reservationMu.Unlock()
			return nodeadmission.RefillForwarding(ctx, c, original, remaining, raw, admissionOwner, budget, held)
		}})
	if err != nil {
		return 1
	}
	if json.NewEncoder(out).Encode(map[string]string{"operation": "route.receive", "phase": "listening", "outcome": "original-duty"}) != nil {
		_ = server.Close()
		return 2
	}
	select {
	case <-ctx.Done():
	case <-server.Done():
	}
	err = server.Close()
	if err != nil {
		_ = json.NewEncoder(diagnostic).Encode(map[string]string{"operation": "route.receive", "phase": "joined", "outcome": "failed", "stage": framing.TerminalFailureStage(err)})
		return 1
	}
	_ = json.NewEncoder(diagnostic).Encode(map[string]string{"operation": "route.receive", "phase": "joined", "outcome": "canceled"})
	return 130
}

func decodeRouteConfig(args []string, target any) error {
	if len(args) != 2 || args[0] != "--config" {
		return errors.New("invalid arguments")
	}
	raw, err := artifacts.ReadBoundedFile(args[1], 64<<10)
	if err != nil {
		return err
	}
	return artifacts.DecodeObject(raw, target)
}
