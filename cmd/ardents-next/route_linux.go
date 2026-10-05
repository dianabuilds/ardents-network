//go:build linux

package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	routeprefix "github.com/dianabuilds/ardents-network/internal/successor/route/prefix"
	routereceiver "github.com/dianabuilds/ardents-network/internal/successor/route/receiver"
	"io"
	"os"
	"sync"
	"syscall"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/receiving"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/stock"
	"github.com/dianabuilds/ardents-network/internal/successor/hosting"
	"github.com/dianabuilds/ardents-network/internal/successor/network/state"
	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
	framing "github.com/dianabuilds/ardents-network/internal/successor/route/channel"
	"github.com/dianabuilds/ardents-network/internal/successor/route/introduction"

	"github.com/dianabuilds/ardents-network/internal/successor/route/role"
	"github.com/dianabuilds/ardents-network/internal/successor/route/selection"
)

func releaseRouteReservation(reservation *hosting.Reservation) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return reservation.Release(ctx)
}

func startRoutePrefix(ctx context.Context, plan routePrefixPlan, authority admissionAuthority, holder *stock.Owner) (_ routeHandle, result error) {
	if authority.current == nil {
		return routeHandle{}, errors.New("route requires opened Network")
	}
	if !independentRouteRoots(plan.EntryRoot, plan.InteriorRoot, plan.HostingRoot) {
		return routeHandle{}, errors.New("route roots invalid")
	}
	selected, err := selection.Open(selection.Config{EntryRoot: plan.EntryRoot, InteriorRoot: plan.InteriorRoot, Domain: plan.Domain, Current: authority.current, Exclusions: plan.Exclusions})
	if err != nil {
		return routeHandle{}, err
	}
	budget, err := hosting.Open(plan.HostingRoot)
	if err != nil {
		return routeHandle{}, errors.Join(err, selected.Close())
	}
	defer func() {
		if result != nil {
			result = errors.Join(result, selected.Close(), budget.Close())
		}
	}()
	leg, err := selected.Select()
	if err != nil {
		return routeHandle{}, err
	}
	reservation, err := budget.Reserve(ctx, hosting.ReservationRequest{Work: plan.Work, Termination: plan.Termination, WorkUntil: plan.Deadline, HoldUntil: plan.Deadline.Add(5 * time.Second)})
	if err != nil {
		return routeHandle{}, err
	}
	var releaseOnce sync.Once
	var releaseErr error
	release := func() error {
		releaseOnce.Do(func() {
			releaseErr = errors.Join(releaseRouteReservation(reservation), selected.Close(), budget.Close())
		})
		return releaseErr
	}
	prefix, err := routeprefix.Open(ctx, routeprefix.Config{Leg: leg, Current: authority.current, Deadline: plan.Deadline, Release: release,
		HoldRefill: func(ctx context.Context, additional uint64) (func() error, error) {
			limit := admission.ForwardClass.ByteLimit()
			if err := budget.CoversJoint(ctx, reservation, hosting.JointTraffic{Tx: limit, Rx: limit, Total: limit}); err != nil {
				return nil, err
			}
			held, err := budget.ReserveAdditionalJoint(ctx, reservation, hosting.JointTraffic{Tx: additional, Rx: additional, Total: additional})
			if err != nil {
				return nil, err
			}
			return func() error { return releaseRouteReservation(held) }, nil
		},
		Present: func(ctx context.Context, h ardp.Hello) ([]byte, error) {
			presentation := stock.Presentation{NetworkID: h.NetworkID, StateGeneration: h.StateGeneration, StateDigest: h.StateDigest, ProfileDigest: h.ProfileDigest, RecipientNodeID: h.RecipientNodeID, RecipientDutyGeneration: h.RecipientDutyGeneration, ChannelNonce: h.ChannelNonce, Deadline: h.Deadline}
			if err := authority.presentation(presentation); err != nil {
				return nil, err
			}
			class := uint8(2)
			if h.Purpose == ardp.PurposeIntroduction {
				class = 3
			}
			return holder.Take(ctx, presentation, class)
		}})
	if err != nil {
		return routeHandle{}, errors.Join(err, release())
	}
	return routeHandle{close: prefix.Close, done: prefix.Done(), replenish: prefix.Replenish, register: func(ctx context.Context, revision uint64) (routeRegistration, error) {
		view, err := authority.current()
		if err != nil {
			return routeRegistration{}, err
		}
		duty, err := leg.IntroductionDuty(view, plan.Exclusions)
		if err != nil {
			return routeRegistration{}, err
		}
		end := minRouteDeadline(plan.Deadline, view.Profile().NotAfter, duty.RecordValidUntil, duty.Epoch.ValidUntil, time.Now().Add(admission.RegistrationClass.Lifetime()).UTC().Truncate(time.Second))
		if bound, exists := ctx.Deadline(); exists && bound.Before(end) {
			end = bound.UTC().Truncate(time.Second)
		}
		registration, err := introduction.Register(ctx, prefix, introduction.RegistrationConfig{Duty: duty, Revision: revision, Deadline: end})
		if err != nil {
			return routeRegistration{}, err
		}
		return routeRegistration{close: registration.Close, withdraw: registration.Withdraw, done: registration.Done(), slot: registration.Slot()}, nil
	}}, nil
}

func minRouteDeadline(end time.Time, bounds ...time.Time) time.Time {
	for _, bound := range bounds {
		if bound.Before(end) {
			end = bound
		}
	}
	return end
}

func runRoute(ctx context.Context, args []string, out, diagnostic io.Writer) (code int) {
	var plan struct {
		Network          *networkAuthorityPlan `json:"network"`
		NodeID           [32]byte              `json:"node_id"`
		SpendRoot        string                `json:"spend_root"`
		IntroductionRoot string                `json:"introduction_root"`
		HostingRoot      string                `json:"hosting_root"`
		Certificate      string                `json:"certificate"`
		PrivateKey       string                `json:"private_key"`
		Work             hosting.Traffic       `json:"work"`
		Termination      hosting.Traffic       `json:"termination"`
	}
	if ctx == nil || len(args) < 1 || args[0] != "receive" || admissionConfig(args[1:], &plan) != nil || plan.Network == nil ||
		!validAdmissionAuthority("", plan.Network, plan.SpendRoot, plan.HostingRoot) || !independentRouteRoots(plan.Network.Root, plan.SpendRoot, plan.HostingRoot) {
		return 2
	}
	config, err := networkStateConfig(plan.Network)
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
	authority := networkAdmissionAuthority(networkOwner.CurrentRuntime, networkOwner.Close)
	view, err := networkOwner.CurrentRuntime()
	if err != nil {
		return 1
	}
	m, err := view.Member(plan.NodeID, view.ObservedAt())
	if err != nil {
		return 1
	}
	if plan.IntroductionRoot != "" && !independentRouteRoots(plan.Network.Root, plan.SpendRoot, plan.HostingRoot, plan.IntroductionRoot) {
		return 2
	}
	duty, err := view.RetainDuty(plan.NodeID, view.ObservedAt())
	if err != nil {
		return 1
	}
	if m.RoleDomain == 4 && m.Subrole == 3 && plan.IntroductionRoot == "" {
		return 2
	}
	certificate, err := readRouteCertificate(plan.Certificate, plan.PrivateKey)
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
	p := view.Profile()
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
	var initializationOwner *receiving.Owner
	var reservationMu sync.Mutex
	reservations := make(map[routereceiver.Channel]*hosting.Reservation)
	if m.RoleDomain == 4 && m.Subrole == 3 {
		initializationOwner = admissionOwner
	}
	server, err := routereceiver.Listen(ctx, routereceiver.ReceiverConfig{Authority: role.Authority{Current: networkOwner.CurrentRuntime, Duty: duty, Profile: p.ProfileBinding}, Certificate: certificate, IntroductionRoot: plan.IntroductionRoot, Receiving: initializationOwner,
		Admit: func(ctx context.Context, c routereceiver.Channel, raw []byte) (receiving.Grant, error) {
			class := admission.ForwardClass
			if c.Hello.Purpose == ardp.PurposeIntroduction {
				class = admission.RegistrationClass
			}
			return admissionOwner.Accept(ctx, class, raw, c.Hello.Deadline, func() (func() error, error) {
				reservation, err := budget.Reserve(ctx, hosting.ReservationRequest{Work: plan.Work, Termination: plan.Termination, WorkUntil: c.Hello.Deadline, HoldUntil: c.Hello.Deadline.Add(5 * time.Second)})
				if err != nil {
					return nil, err
				}
				reservationMu.Lock()
				reservations[c] = reservation
				reservationMu.Unlock()
				return c.HoldReservation(func() error {
					reservationMu.Lock()
					delete(reservations, c)
					reservationMu.Unlock()
					return releaseRouteReservation(reservation)
				})
			})
		},
		Refill: func(ctx context.Context, c routereceiver.Channel, original receiving.Grant, remaining uint64, raw []byte) (receiving.Grant, error) {
			limit := admission.ForwardClass.ByteLimit()
			if c.Hello.Purpose != ardp.PurposeForwarding || remaining == 0 || remaining >= limit {
				return receiving.Grant{}, errors.New("route refill allowance unavailable")
			}
			reservationMu.Lock()
			held := reservations[c]
			reservationMu.Unlock()
			return admissionOwner.Refill(ctx, original, remaining, raw, func() (func() error, error) {
				if err := budget.CoversJoint(ctx, held, hosting.JointTraffic{Tx: limit, Rx: limit, Total: limit}); err != nil {
					return nil, err
				}
				delta := limit - remaining
				addition, err := budget.ReserveAdditionalJoint(ctx, held, hosting.JointTraffic{Tx: delta, Rx: delta, Total: delta})
				if err != nil {
					return nil, err
				}
				return c.HoldReservation(func() error { return releaseRouteReservation(addition) })
			})
		}})
	if err != nil {
		return 1
	}
	if json.NewEncoder(out).Encode(map[string]string{"operation": "route.receive", "phase": "listening"}) != nil {
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

func readRouteCertificate(certPath, keyPath string) (tls.Certificate, error) {
	if !absoluteAdmissionPath(certPath) || !absoluteAdmissionPath(keyPath) {
		return tls.Certificate{}, errors.New("route certificate paths invalid")
	}
	cert, err := readBounded(certPath, 8192)
	if err != nil {
		return tls.Certificate{}, err
	}
	key, err := readRouteKey(keyPath)
	if err != nil {
		return tls.Certificate{}, err
	}
	defer clear(key)
	return tls.X509KeyPair(cert, key)
}

func readRouteKey(path string) (key []byte, result error) {
	// Nonblocking/no-follow open cannot stall on a substituted FIFO or follow
	// a substituted symlink; authority is checked on the opened descriptor.
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, errors.New("route private key unavailable")
	}
	defer func() {
		result = errors.Join(result, f.Close())
		if result != nil {
			clear(key)
			key = nil
		}
	}()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
		return nil, errors.New("route private key permissions invalid")
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Uid != uint32(os.Geteuid()) {
		return nil, errors.New("route private key owner differs")
	}
	raw, err := io.ReadAll(io.LimitReader(f, 8193))
	if err != nil || len(raw) > 8192 {
		clear(raw)
		return nil, errors.New("route private key unavailable")
	}
	return raw, nil
}
