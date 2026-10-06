//go:build linux

package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/issuer"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/quota"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/receiving"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/stock"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/token"
	"github.com/dianabuilds/ardents-network/internal/successor/hosting"
	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
	"github.com/dianabuilds/ardents-network/internal/successor/route/bootstrap"
	framing "github.com/dianabuilds/ardents-network/internal/successor/route/channel"
	issuertransport "github.com/dianabuilds/ardents-network/internal/successor/route/issuer"
	routeprefix "github.com/dianabuilds/ardents-network/internal/successor/route/prefix"
	routereceiver "github.com/dianabuilds/ardents-network/internal/successor/route/receiver"
	"github.com/dianabuilds/ardents-network/internal/successor/route/role"
	"github.com/dianabuilds/ardents-network/internal/successor/route/selection"
	"github.com/dianabuilds/ardents-network/internal/successor/route/transport"
)

type routeIssuerOutputConn struct {
	net.Conn
	started chan struct{}
	once    sync.Once
}

func (c *routeIssuerOutputConn) Write(raw []byte) (int, error) {
	c.once.Do(func() { close(c.started) })
	return c.Conn.Write(raw)
}

// Genuine signed Network, Stock/issuing journals and Hosting feed the actual
// receiving exchange. Real inner role TLS writes to an unread net.Pipe; no fake
// successful result or writer gate is supplied. This isolates physical output,
// not a complete TCP/QUIC path or successful bootstrap stock delivery.
func TestRouteIssuerSignedOutputBlocksPhysicalTLSUntilCancellation(t *testing.T) {
	f := newNetworkAdmissionFixture(t)
	holder := routePermissionStock(t, f, admission.AllocationUser, [3]uint32{2, 0, 0})
	budget := networkTestBudget(t)
	view, err := f.current()
	if err != nil {
		t.Fatal(err)
	}
	duty, err := view.RetainDuty(f.profile.IssuerNodeID, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	a := role.Authority{Current: f.current, Duty: duty, Profile: view.Profile().ProfileBinding}
	end := time.Now().Add(8 * time.Second).UTC().Truncate(time.Second)
	adjacency := bootstrap.NewBudget().Adjacency()
	claim, err := adjacency.Reserve(end)
	if err != nil {
		t.Fatal(err)
	}
	defer adjacency.Seal()
	defer claim.ReleaseAfterJoin()
	release, err := networkTestReservation(t, budget, claim.Deadline())
	if err != nil {
		t.Fatal(err)
	}
	var returns atomic.Int32
	var returnOnce sync.Once
	returnWork := func() {
		returnOnce.Do(func() {
			returns.Add(1)
			if err := release(); err != nil {
				t.Error(err)
			}
		})
	}
	defer returnWork()
	h, err := a.FreshHello(claim.Deadline(), ardp.PurposeIssuer, false)
	if err != nil {
		t.Fatal(err)
	}
	member, err := a.Member()
	if err != nil {
		t.Fatal(err)
	}
	challenge := token.ClosedTokenContext{NetworkID: f.profile.NetworkID, ProfileDigest: f.profile.Digest, IssuerNodeID: member.NodeID, ReceiverNodeID: member.NodeID, ReceiverDutyGeneration: member.DutyGeneration, Class: 1, WindowStart: time.Now().UTC().Truncate(time.Hour)}
	attempt, err := holder.Begin(stock.IssuanceIntent{Challenges: []token.ClosedTokenContext{challenge}, Selection: stock.ExchangeBinding{ID: h.ChannelNonce, ProfileDigest: f.profile.Digest}, Bootstrap: true, Deadline: h.Deadline})
	if err != nil {
		t.Fatal(err)
	}
	defer attempt.Complete(nil, context.Canceled)
	raw, _, err := attempt.Request()
	if err != nil {
		t.Fatal(err)
	}
	defer clear(raw)
	request, err := admission.DecodeClosedTokenBatch(raw)
	if err != nil || request.Class != 1 || len(request.BlindedRequests) != 1 || holder.Status().Remaining[0] != 1 {
		t.Fatal("one requested token must consume one of two allocated rights", err)
	}
	for _, blinded := range request.BlindedRequests {
		clear(blinded)
	}
	local, peer := net.Pipe()
	defer local.Close()
	defer peer.Close()
	server := tls.Server(local, transport.RoleServerTLS(routeTestCertificate(t, f.spec.Nodes[0].PrivateKey)))
	handshake := make(chan error, 1)
	go func() { handshake <- server.HandshakeContext(t.Context()) }()
	client, err := routeFixtureRoleTLS(t, peer, member.PublicKey, h.Deadline)
	if err != nil {
		_ = local.Close()
		<-handshake
		t.Fatal(err)
	}
	if err := <-handshake; err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	check := func() error {
		_, authorityErr := a.Hello(h, false)
		_, claimErr := claim.Restriction()
		return errors.Join(authorityErr, claimErr, ctx.Err())
	}
	physical := &routeIssuerOutputConn{Conn: server, started: make(chan struct{})}
	var signed []byte
	finished := make(chan error, 1)
	joined := false
	defer func() {
		cancel()
		_ = local.Close()
		_ = peer.Close()
		if !joined {
			<-finished
		}
		clear(signed)
	}()
	go func() {
		finished <- issuertransport.Serve(ctx, physical, h, bootstrap.LaneBytes, check, func(ctx context.Context, batch []byte) ([]byte, error) {
			response, err := issueRouteBatch(ctx, f.authority, f.plan, batch, true)
			if err == nil {
				signed = append([]byte(nil), response...)
			}
			return response, err
		})
	}()
	body, err := ardp.EncodeIssuerRequest([32]byte{73}, raw)
	if err != nil {
		t.Fatal(err)
	}
	defer clear(body)
	if err := ardp.WriteFrame(client, ardp.Frame{Kind: ardp.KindOperation, Body: body}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-physical.started:
	case err := <-finished:
		joined = true
		t.Fatal("actual signed output did not reach physical writer", err)
	case <-time.After(3 * time.Second):
		t.Fatal("actual signed output did not start")
	}
	select {
	case err := <-finished:
		joined = true
		t.Fatal("unread role TLS output completed", err)
	default:
	}
	observed, err := budget.Observe(t.Context())
	const reserved = 2 * ((2 << 20) + (64 << 10))
	if err != nil || observed.ReservedBytes != reserved || returns.Load() != 0 {
		t.Fatal("blocked signed output returned actual work", observed.ReservedBytes, err)
	}
	cancel()
	result := <-finished
	joined = true
	if !errors.Is(result, context.Canceled) || !errors.Is(result, os.ErrDeadlineExceeded) {
		t.Fatal("actual signed write lost cancellation/physical timeout", result)
	}
	// Independently verify the actual committed blind signatures, while denying
	// token deposit with the original canceled guard. This is not wire delivery.
	verified := false
	if err := attempt.CompleteBound(signed, nil, func() error { verified = true; return ctx.Err() }); !verified || !errors.Is(err, context.Canceled) {
		t.Fatal("blocked result was not genuinely signed or guard was lost", verified, err)
	}
	if s := holder.Status(); s.Pending || s.Busy || s.Remaining[0] != 1 {
		t.Fatal("canceled signed result reconstructed/refunded allocation", s)
	}
	_ = local.Close()
	_ = peer.Close()
	returnWork()
	returnWork()
	observed, err = budget.Observe(t.Context())
	if err != nil || observed.ReservedBytes != 0 || returns.Load() != 1 {
		t.Fatal("joined physical result did not return exactly once", observed.ReservedBytes, returns.Load(), err)
	}
}

// One genuine receiving installation: separate signed authority, spend roots
// and Hosting budgets. Gates observe actual issuance/release; none mint a Grant
// or replace IssueCurrent. Used by retirement and competing-holder scenarios.
type routeIssuerFixture struct {
	network                                                                       *networkAdmissionFixture
	servers                                                                       []*routereceiver.Receiver
	budgets                                                                       []*hosting.Budget
	issuerBudget                                                                  *hosting.Budget
	stopIssuer                                                                    context.CancelFunc
	bootstrapHolds, bootstrapReturns, admissions, ordinaryIssues, bootstrapIssues atomic.Int32
	issuerAdmissionCalls, issuerAdmissions, issuerReturns                         atomic.Int32
}

func newRouteIssuerFixture(t *testing.T, profile transport.CarrierProfile, afterIssue func(context.Context, bool, []byte, []byte) error, afterIssuerReturn func() error, admissionPoint func(string), issuerObservation func(int)) *routeIssuerFixture {
	t.Helper()
	f, sockets, certificates := newRoleRouteFixture(t, profile, 1, false, true)
	x := &routeIssuerFixture{network: f}
	view, err := f.current()
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range [][32]byte{{12}, {13}, {14}, {15}, f.profile.IssuerNodeID} {
		member, err := view.Member(id, view.ObservedAt())
		if err != nil {
			t.Fatal(err)
		}
		duty, err := view.RetainDuty(id, view.ObservedAt())
		if err != nil {
			t.Fatal(err)
		}
		binding := receiving.Receiver{NetworkID: f.profile.NetworkID, StateGeneration: f.profile.StateGeneration, StateDigest: f.profile.StateDigest, ProfileDigest: f.profile.Digest, NodeID: id, DutyGeneration: member.DutyGeneration}
		root := t.TempDir()
		if err := os.Chmod(root, 0700); err != nil {
			t.Fatal(err)
		}
		owner, err := receiving.Open(root, binding, func() (receiving.Observation, error) { return f.authority.receiver(binding, member.NotAfter()) })
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := owner.Close(); err != nil {
				t.Error(err)
			}
		})
		budget := networkTestBudget(t)
		x.budgets = append(x.budgets, budget)
		isIssuer := id == f.profile.IssuerNodeID
		if isIssuer {
			x.issuerBudget = budget
		}
		config := routereceiver.ReceiverConfig{Authority: role.Authority{Current: f.current, Duty: duty, Profile: view.Profile().ProfileBinding}, Certificate: certificates[id],
			ReserveBootstrap: func(ctx context.Context, end time.Time) (func() error, error) {
				release, err := networkTestReservation(t, budget, end)
				if err != nil {
					return nil, err
				}
				x.bootstrapHolds.Add(1)
				return func() error { x.bootstrapReturns.Add(1); return release() }, nil
			},
			Admit: func(ctx context.Context, c routereceiver.Channel, raw []byte) (receiving.Grant, error) {
				if isIssuer {
					x.issuerAdmissionCalls.Add(1)
				}
				class, err := role.AdmissionClass(c.Hello.Purpose)
				if err != nil {
					return receiving.Grant{}, err
				}
				if isIssuer && admissionPoint != nil {
					admissionPoint("before-accept")
				}
				grant, err := owner.Accept(ctx, class, raw, c.Hello.Deadline, func() (func() error, error) {
					release, err := networkTestReservation(t, budget, c.Hello.Deadline)
					if err != nil {
						return nil, err
					}
					if isIssuer && admissionPoint != nil {
						admissionPoint("after-reservation")
					}
					return c.HoldReservation(func() error {
						err := release()
						if isIssuer {
							x.issuerReturns.Add(1)
							if afterIssuerReturn != nil {
								err = errors.Join(err, afterIssuerReturn())
							}
						}
						return err
					})
				})
				if err == nil {
					x.admissions.Add(1)
					if isIssuer {
						x.issuerAdmissions.Add(1)
						if admissionPoint != nil {
							admissionPoint("after-accept")
						}
					}
				}
				return grant, err
			},
		}
		if isIssuer {
			config.Issue = func(ctx context.Context, batch []byte, bootstrap bool) ([]byte, error) {
				if bootstrap {
					x.bootstrapIssues.Add(1)
				} else {
					x.ordinaryIssues.Add(1)
				}
				authority := f.authority
				if !bootstrap && issuerObservation != nil {
					var observation int
					original := authority.issuer
					authority.issuer = func(signer [32]byte) (admission.AuthorityFacts, time.Time, error) {
						observation++
						issuerObservation(observation)
						return original(signer)
					}
				}
				payload, err := issueRouteBatch(ctx, authority, f.plan, batch, bootstrap)
				if err == nil && afterIssue != nil {
					// A post-signing gate must observe actual successful signing,
					// not a same-sized unavailable/exhausted response.
					if !bootstrap {
						outcome, decodeErr := admission.DecodeClosedTokenBatchResult(payload)
						if decodeErr != nil || outcome.Status != admission.ClosedTokenIssued || len(outcome.Signatures) == 0 {
							clear(payload)
							return nil, errors.Join(errors.New("actual issuer did not sign the gated result"), decodeErr)
						}
						for _, signature := range outcome.Signatures {
							clear(signature)
						}
					}
					err = afterIssue(ctx, bootstrap, batch, payload)
				}
				if err != nil {
					clear(payload)
					return nil, err
				}
				return payload, nil
			}
		}
		sockets[id]()
		listenContext, stop := context.WithCancel(t.Context())
		t.Cleanup(stop)
		if isIssuer {
			x.stopIssuer = stop
		}
		server, err := routereceiver.Listen(listenContext, config)
		if err != nil {
			t.Fatal(err)
		}
		x.servers = append(x.servers, server)
		// Scenarios explicitly inspect the retained Close result. Always join
		// again here on an early assertion failure before storage cleanup runs.
		t.Cleanup(func() { _ = server.Close() })
	}
	return x
}

// The maintained holder composition obtains its own Stock, joins bootstrap,
// then spends genuine class-2/class-1 tokens on fresh physical role channels.
func TestRouteHolderBootstrapRetiresBeforeFreshAdmittedIssuerBothCarriers(t *testing.T) {
	for _, profile := range []transport.CarrierProfile{transport.ClosedCarrierTCP, transport.ClosedCarrierQUIC} {
		t.Run(string(profile), func(t *testing.T) {
			testRouteHolderIssuerRetirement(t, profile, false)
		})
	}
}

// One genuine issuance is sufficient to expose retirement overtaking the
// receiving Interior's final CREDIT. No fresh admitted prefix or extra batch
// is required to reproduce that physical failure.
func TestRouteSingleBootstrapIssuerJoinsBothCarriers(t *testing.T) {
	for _, profile := range []transport.CarrierProfile{transport.ClosedCarrierTCP, transport.ClosedCarrierQUIC} {
		t.Run(string(profile), func(t *testing.T) {
			testRouteHolderIssuerRetirement(t, profile, true)
		})
	}
}

func testRouteHolderIssuerRetirement(t *testing.T, profile transport.CarrierProfile, singleBootstrap bool) {
	t.Helper()
	x := newRouteIssuerFixture(t, profile, nil, nil, nil, nil)
	f := x.network
	holder := routePermissionStock(t, f, admission.AllocationUser, [3]uint32{2, 4, 0})
	servers, budgets := x.servers, x.budgets
	bootstrapHolds, bootstrapReturns := &x.bootstrapHolds, &x.bootstrapReturns
	admissions, ordinaryIssues, bootstrapIssues := &x.admissions, &x.ordinaryIssues, &x.bootstrapIssues
	local := t.TempDir()
	plan := routePrefixPlan{EntryRoot: filepath.Join(local, "entry"), InteriorRoot: filepath.Join(local, "interior"), HostingRoot: routeProcessBudget(t), Domain: 1, Deadline: time.Now().Add(25 * time.Second).UTC().Truncate(time.Second), Work: hosting.Traffic{Tx: 2 << 20, Rx: 2 << 20}, Termination: hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}}
	boot, err := startRouteBootstrap(t.Context(), plan, f.authority, holder)
	if err != nil {
		t.Fatal("actual holder bootstrap opening", err)
	}
	defer boot.close()
	classes := []uint8{2, 1}
	if singleBootstrap {
		classes = classes[:1]
	}
	for _, class := range classes {
		if err := boot.issue(t.Context(), class, nil); err != nil {
			t.Fatal("actual holder bootstrap batch", class, err)
		}
	}
	if !singleBootstrap {
		if err := boot.issue(t.Context(), 1, nil); err == nil {
			t.Fatal("third bootstrap batch accepted")
		}
	}
	if bootstrapIssues.Load() != int32(len(classes)) || ordinaryIssues.Load() != 0 || admissions.Load() != 0 {
		t.Fatal("bootstrap crossed genuine private Admission", bootstrapIssues.Load(), ordinaryIssues.Load(), admissions.Load())
	}
	if err := boot.close(); err != nil {
		t.Fatal("bootstrap did not join before fresh Prefix", err)
	}
	if err := boot.issue(t.Context(), 1, nil); err == nil {
		t.Fatal("retired bootstrap reopened an exchange")
	}
	if !singleBootstrap {
		ordinary, err := startRoutePrefix(t.Context(), plan, f.authority, holder)
		if err != nil {
			t.Fatal("fresh admitted holder Prefix", err)
		}
		defer ordinary.close()
		if ordinary.bootstrap || ordinary.replenish == nil {
			t.Fatal("bootstrap was relabeled as ordinary")
		}
		if err := ordinary.issue(t.Context(), 2, nil); err != nil {
			t.Fatal("actual ordinary class-1 issuer exchange", err)
		}
		if admissions.Load() != 3 || ordinaryIssues.Load() != 1 || bootstrapIssues.Load() != 2 {
			t.Fatal("fresh ordinary channels skipped genuine spend", admissions.Load(), ordinaryIssues.Load(), bootstrapIssues.Load())
		}
		if err := ordinary.close(); err != nil {
			t.Fatal("ordinary Prefix retirement", err)
		}
	}
	for index, server := range servers {
		if err := server.Close(); err != nil {
			t.Fatal("receiving retirement at fixture duty index", index, err)
		}
	}
	expectedHolds := int32(2 + len(classes))
	if bootstrapHolds.Load() != expectedHolds || bootstrapReturns.Load() != expectedHolds {
		t.Fatal("bootstrap claims were reused or returned early", bootstrapHolds.Load(), bootstrapReturns.Load())
	}
	for _, budget := range budgets {
		observed, err := budget.Observe(t.Context())
		if err != nil || observed.ReservedBytes != 0 {
			t.Fatal("joined issuer retained Hosting", err, observed.ReservedBytes)
		}
	}
}

// Two holders reach the same genuine issuer through independently admitted
// prefixes. The gate delays delivery after real durable debit/signing; it is
// neither a simulated signing success nor an artificial receiving Grant.
func TestRouteIssuerExclusiveWorkJoinsBeforeReturnBothCarriers(t *testing.T) {
	for _, profile := range []transport.CarrierProfile{transport.ClosedCarrierTCP, transport.ClosedCarrierQUIC} {
		for _, mode := range []string{"joined-result", "original-cancellation", "lost-result-retry", "late-release-failure"} {
			t.Run(string(profile)+"/"+mode, func(t *testing.T) {
				started, resume := make(chan struct{}), make(chan struct{})
				var releaseOnce sync.Once
				releaseGate := func() { releaseOnce.Do(func() { close(resume) }) }
				defer releaseGate()
				var gated atomic.Bool
				var deliveries, physicalReturns atomic.Int32
				returned := make(chan struct{})
				var firstRequest, firstResponse, retryRequest, retryResponse [32]byte
				late := errors.New("late issuer Hosting return failure")
				x := newRouteIssuerFixture(t, profile, func(ctx context.Context, bootstrap bool, request, response []byte) error {
					if !bootstrap && gated.Load() {
						switch deliveries.Add(1) {
						case 1:
							firstRequest, firstResponse = sha256.Sum256(request), sha256.Sum256(response)
							close(started)
							<-resume // original context remains checked by serveIssuer afterward
						case 2:
							retryRequest, retryResponse = sha256.Sum256(request), sha256.Sum256(response)
						}
					}
					return nil
				}, func() error {
					if physicalReturns.Add(1) == 1 {
						close(returned)
						if mode == "late-release-failure" {
							return late
						}
					}
					return nil
				}, nil, nil)
				f := x.network
				var holders [2]*stock.Owner
				var prefixes [2]routeHandle
				for i := range holders {
					holders[i] = routePermissionStock(t, f, admission.AllocationUser, [3]uint32{4, 8, 0})
					local := t.TempDir()
					plan := routePrefixPlan{EntryRoot: filepath.Join(local, "entry"), InteriorRoot: filepath.Join(local, "interior"), HostingRoot: routeProcessBudget(t), Domain: 1, Deadline: time.Now().Add(25 * time.Second).UTC().Truncate(time.Second), Work: hosting.Traffic{Tx: 2 << 20, Rx: 2 << 20}, Termination: hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}}
					boot, err := startRouteBootstrap(t.Context(), plan, f.authority, holders[i])
					if err != nil {
						t.Fatal("genuine bootstrap", err)
					}
					defer boot.close()
					if err := boot.issue(t.Context(), 2, nil); err != nil {
						t.Fatal("forwarding stock", err)
					}
					issuer := f.profile.IssuerNodeID
					if err := boot.issue(t.Context(), 1, [][32]byte{issuer, issuer, issuer}); err != nil {
						t.Fatal("Control stock", err)
					}
					if err := boot.close(); err != nil {
						t.Fatal("bootstrap join", err)
					}
					prefixes[i], err = startRoutePrefix(t.Context(), plan, f.authority, holders[i])
					if err != nil {
						t.Fatal("fresh admitted prefix", err)
					}
					defer prefixes[i].close()
				}
				gated.Store(true)
				caller, cancel := context.WithCancel(t.Context())
				defer cancel()
				completed := make(chan error, 1)
				go func() { completed <- prefixes[0].issue(caller, 2, nil) }()
				select {
				case <-started:
				case err := <-completed:
					t.Fatal("issuer did not reach actual signed result", err)
				}
				if err := prefixes[1].issue(t.Context(), 2, nil); err == nil {
					t.Fatal("concurrent issuer operation accepted")
				}
				if x.issuerAdmissionCalls.Load() != 1 || x.issuerAdmissions.Load() != 1 || x.ordinaryIssues.Load() != 1 || x.issuerReturns.Load() != 0 {
					t.Fatal("busy operation spent or released held issuer work", x.issuerAdmissionCalls.Load(), x.issuerAdmissions.Load(), x.ordinaryIssues.Load(), x.issuerReturns.Load())
				}
				if s := holders[1].Status(); !s.Pending || s.Busy || s.Remaining[1] != 4 {
					t.Fatal("busy exchange lost pending batch or refunded quota", s)
				}
				var firstErr error
				cancelled := mode == "original-cancellation" || mode == "lost-result-retry"
				if cancelled {
					cancel()
					firstErr = <-completed
					if !errors.Is(firstErr, context.Canceled) {
						t.Error("original cancellation absent from completion", firstErr)
					}
					if s := holders[0].Status(); !s.Pending || s.Busy || s.Remaining[1] != 4 {
						t.Error("cancelled result deposited stock or refunded quota", s)
					}
				}
				observed, err := x.issuerBudget.Observe(t.Context())
				if err != nil || observed.ReservedBytes == 0 || x.issuerReturns.Load() != 0 {
					t.Fatal("actual work returned before delayed owner joined", observed.ReservedBytes, err)
				}
				releaseGate()
				if !cancelled {
					firstErr = <-completed
				}
				if mode == "joined-result" {
					if firstErr != nil {
						t.Fatal("first issuer completion", firstErr)
					}
					if err := prefixes[1].issue(t.Context(), 2, nil); err != nil {
						t.Fatal("explicit same-prefix retry after owner join", err)
					}
					if x.issuerAdmissions.Load() != 2 || x.ordinaryIssues.Load() != 2 || holders[1].Status().Pending || holders[1].Status().Remaining[1] != 4 {
						t.Fatal("healthy sibling did not complete genuine retry")
					}
				} else if cancelled && firstErr == nil {
					t.Error("failed original became successful completion")
				}
				if mode == "lost-result-retry" {
					<-returned
					// This is a human-selected retry on the same live Prefix, not
					// automatic recovery or replacement delivery identity. Each
					// IssueCurrent call reopens its actual durable issuer owners.
					if err := prefixes[0].issue(t.Context(), 2, nil); err != nil {
						t.Fatal("lost-result exact retry", err)
					}
					if firstRequest != retryRequest || firstResponse != retryResponse {
						t.Error("durable retry changed request or committed response")
					}
					if s := holders[0].Status(); s.Pending || s.Busy || s.Remaining[1] != 4 {
						t.Error("retry reserved quota again or failed deposit", s)
					}
				}
				var retired error
				for _, p := range prefixes {
					retired = errors.Join(retired, p.close())
				}
				var receiverErr error
				for _, server := range x.servers {
					receiverErr = errors.Join(receiverErr, server.Close())
				}
				if mode == "joined-result" && (retired != nil || receiverErr != nil) {
					t.Error("healthy sibling retirement", retired, receiverErr)
				}
				if mode == "late-release-failure" && !errors.Is(receiverErr, late) {
					// Hosting return follows physical joining. It is retained by
					// this receiving owner; no closed peer channel can convey a
					// later local callback failure or revoke a verified signature.
					t.Error("late genuine return failure erased", receiverErr)
				}
				for _, budget := range x.budgets {
					observed, err := budget.Observe(t.Context())
					if err != nil || observed.ReservedBytes != 0 {
						t.Error("joined receiving owner retained Hosting", observed.ReservedBytes, err)
					}
				}
				wantReturns := int32(1)
				if mode == "joined-result" || mode == "lost-result-retry" {
					wantReturns = 2
				}
				if x.issuerReturns.Load() != wantReturns {
					t.Error("issuer reservation return was not exactly once", x.issuerReturns.Load(), wantReturns)
				}
			})
		}
	}
}

// The successor is signed and accepted by the actual Network owner. The gates
// surround actual receiving Accept or actual committed issuer output, not a
// supplied authority boolean. Every original binding remains unchanged.
func TestRouteIssuerStateLossAtRealBoundariesBothCarriers(t *testing.T) {
	routeIssuerRetirementCases(t, false)
}

func TestRouteIssuerReceiverCancellationAtRealBoundariesBothCarriers(t *testing.T) {
	routeIssuerRetirementCases(t, true)
}

func routeIssuerRetirementCases(t *testing.T, cancelReceiver bool) {
	for _, profile := range []transport.CarrierProfile{transport.ClosedCarrierTCP, transport.ClosedCarrierQUIC} {
		for _, phase := range []string{"before-accept", "after-reservation", "after-accept", "before-debit", "after-debit", "after-signing"} {
			t.Run(string(profile)+"/"+phase, func(t *testing.T) {
				entered, resume := make(chan struct{}), make(chan struct{})
				var releaseOnce sync.Once
				release := func() { releaseOnce.Do(func() { close(resume) }) }
				defer release()
				pause := func(point string) {
					if point == phase {
						close(entered)
						<-resume
					}
				}
				x := newRouteIssuerFixture(t, profile, func(ctx context.Context, bootstrap bool, request, result []byte) error {
					if !bootstrap {
						pause("after-signing")
					}
					return nil
				}, nil, pause, func(observation int) {
					// IssueCurrent's initial binding check precedes its effect
					// checks. Journal assertions below independently verify which
					// real durable transaction has completed at these callbacks.
					switch observation {
					case 2:
						pause("before-debit")
					case 3:
						pause("after-debit")
					}
				})
				f := x.network
				holder := routePermissionStock(t, f, admission.AllocationUser, [3]uint32{2, 8, 0})
				local := t.TempDir()
				plan := routePrefixPlan{EntryRoot: filepath.Join(local, "entry"), InteriorRoot: filepath.Join(local, "interior"), HostingRoot: routeProcessBudget(t), Domain: 1, Deadline: time.Now().Add(25 * time.Second).UTC().Truncate(time.Second), Work: hosting.Traffic{Tx: 2 << 20, Rx: 2 << 20}, Termination: hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}}
				boot, err := startRouteBootstrap(t.Context(), plan, f.authority, holder)
				if err != nil {
					t.Fatal("bootstrap", err)
				}
				defer boot.close()
				for _, class := range []uint8{2, 1} {
					if err := boot.issue(t.Context(), class, nil); err != nil {
						t.Fatal("genuine bootstrap stock", err)
					}
				}
				if err := boot.close(); err != nil {
					t.Fatal("bootstrap join", err)
				}
				journalSize := func(root, name string) int64 {
					info, err := os.Stat(filepath.Join(root, name))
					if err != nil {
						t.Fatal("actual issuer journal unavailable", err)
					}
					return info.Size()
				}
				debitSize := journalSize(f.plan.AdmissionRoot, "admission.journal")
				resultSize := journalSize(f.plan.ResultRoot, "results.journal")
				original, err := startRoutePrefix(t.Context(), plan, f.authority, holder)
				if err != nil {
					t.Fatal("actual admitted original", err)
				}
				defer original.close()
				completed := make(chan error, 1)
				go func() { completed <- original.issue(t.Context(), 2, nil) }()
				select {
				case <-entered:
				case err := <-completed:
					t.Fatal("actual issuer boundary not reached", phase, err)
				}
				wantAdmitted, wantIssued := int32(0), int32(0)
				if phase != "before-accept" && phase != "after-reservation" {
					wantAdmitted = 1
				}
				wantReserved := int32(0)
				if phase != "before-accept" {
					wantReserved = 1
				}
				if phase == "before-debit" || phase == "after-debit" || phase == "after-signing" {
					wantIssued = 1
				}
				if x.issuerAdmissions.Load() != wantAdmitted || x.ordinaryIssues.Load() != wantIssued || x.issuerReturns.Load() != 0 {
					t.Fatal("boundary did not retain actual effects", phase, x.issuerAdmissions.Load(), x.ordinaryIssues.Load(), x.issuerReturns.Load())
				}
				before, err := x.issuerBudget.Observe(t.Context())
				// These are the explicit test operator inputs to real Hosting,
				// not a multiplier inferred from token bytes or TLS overhead.
				wantReservedBytes := uint64(wantReserved) * 2 * ((2 << 20) + (64 << 10))
				if err != nil || before.ReservedBytes != wantReservedBytes {
					t.Fatal("real Hosting boundary differs", before.ReservedBytes, err)
				}
				wantDebit := phase == "after-debit" || phase == "after-signing"
				committedDebit := journalSize(f.plan.AdmissionRoot, "admission.journal")
				committedResult := journalSize(f.plan.ResultRoot, "results.journal")
				if (committedDebit > debitSize) != wantDebit {
					t.Fatal("durable quota effect differs at actual boundary", phase)
				}
				if (committedResult > resultSize) != (phase == "after-signing") {
					t.Fatal("committed signing effect differs at actual boundary", phase)
				}
				if cancelReceiver {
					// Cancel the real listener generation, not the holder caller.
					// Its borrower is gated, so cleanup cannot already have joined.
					x.stopIssuer()
					retiring, err := x.issuerBudget.Observe(t.Context())
					if err != nil || retiring.ReservedBytes != wantReservedBytes {
						t.Fatal("receiver cancellation returned capacity before borrower join", retiring.ReservedBytes, err)
					}
				} else {
					// Durable successor intake, never a fabricated RuntimeView.
					f.successor(t)
				}
				if t.Context().Err() != nil {
					t.Fatal("test caller ended instead of State loss")
				}
				release()
				if err := <-completed; err == nil {
					t.Error("obsolete State produced successful issuer completion", phase)
				}
				if x.issuerAdmissions.Load() != wantAdmitted || x.ordinaryIssues.Load() != wantIssued {
					t.Error("old request acquired a successor effect", phase)
				}
				if s := holder.Status(); s.Busy || s.Pending != cancelReceiver || s.Remaining[1] != 4 {
					t.Error("State loss deposited/rebound or refunded original batch", s)
				}
				terminal := original.close() // retain genuine authority retirement, never renew it
				if repeated := original.close(); terminal == nil && repeated != nil || terminal != nil && !errors.Is(repeated, terminal) {
					t.Error("original retirement result changed", terminal, repeated)
				}
				for _, server := range x.servers {
					terminal := server.Close()
					if repeated := server.Close(); terminal == nil && repeated != nil || terminal != nil && !errors.Is(repeated, terminal) {
						t.Error("receiving retirement result changed", terminal, repeated)
					}
				}
				if journalSize(f.plan.AdmissionRoot, "admission.journal") != committedDebit || journalSize(f.plan.ResultRoot, "results.journal") != committedResult {
					t.Error("obsolete issuance changed durable debit/result history", phase)
				}
				if x.issuerReturns.Load() != wantReserved {
					t.Error("original reservation return differed", x.issuerReturns.Load(), wantReserved)
				}
				for _, budget := range x.budgets {
					observed, err := budget.Observe(t.Context())
					if err != nil || observed.ReservedBytes != 0 {
						t.Error("obsolete physical work not joined", observed.ReservedBytes, err)
					}
				}
			})
		}
	}
}

// This public Prefix composition uses actual selection, Hosting and Stock.
// The local completion guard only revokes the original caller, after real
// signature verification; it supplies no response, authority or deposit.
func TestRouteIssuerOriginalCallerAtVerifiedDepositBothCarriers(t *testing.T) {
	for _, profile := range []transport.CarrierProfile{transport.ClosedCarrierTCP, transport.ClosedCarrierQUIC} {
		t.Run(string(profile), func(t *testing.T) {
			x := newRouteIssuerFixture(t, profile, nil, nil, nil, nil)
			f := x.network
			holder := routePermissionStock(t, f, admission.AllocationUser, [3]uint32{2, 8, 0})
			local := t.TempDir()
			plan := routePrefixPlan{EntryRoot: filepath.Join(local, "entry"), InteriorRoot: filepath.Join(local, "interior"), HostingRoot: routeProcessBudget(t), Domain: 1, Deadline: time.Now().Add(25 * time.Second).UTC().Truncate(time.Second), Work: hosting.Traffic{Tx: 2 << 20, Rx: 2 << 20}, Termination: hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}}
			boot, err := startRouteBootstrap(t.Context(), plan, f.authority, holder)
			if err != nil {
				t.Fatal(err)
			}
			defer boot.close()
			for _, class := range []uint8{2, 1} {
				if err := boot.issue(t.Context(), class, nil); err != nil {
					t.Fatal(err)
				}
			}
			if err := boot.close(); err != nil {
				t.Fatal(err)
			}
			selected, err := selection.Open(selection.Config{EntryRoot: plan.EntryRoot, InteriorRoot: plan.InteriorRoot, Domain: 1, Current: f.current})
			if err != nil {
				t.Fatal(err)
			}
			defer selected.Close()
			leg, err := selected.Select()
			if err != nil {
				t.Fatal(err)
			}
			budget, err := hosting.Open(plan.HostingRoot)
			if err != nil {
				t.Fatal(err)
			}
			defer budget.Close()
			held, err := budget.Reserve(t.Context(), hosting.ReservationRequest{Work: plan.Work, Termination: plan.Termination, WorkUntil: plan.Deadline, HoldUntil: plan.Deadline.Add(5 * time.Second)})
			if err != nil {
				t.Fatal(err)
			}
			var once sync.Once
			var returnErr error
			release := func() error { once.Do(func() { returnErr = releaseRouteReservation(held) }); return returnErr }
			defer release()
			var forwardingPresentation stock.Presentation
			opened, err := routeprefix.Open(t.Context(), routeprefix.Config{Leg: leg, Current: f.current, Deadline: plan.Deadline, Release: release, Present: func(ctx context.Context, hello ardp.Hello) ([]byte, error) {
				presentation := stock.Presentation{NetworkID: hello.NetworkID, StateGeneration: hello.StateGeneration, StateDigest: hello.StateDigest, ProfileDigest: hello.ProfileDigest, RecipientNodeID: hello.RecipientNodeID, RecipientDutyGeneration: hello.RecipientDutyGeneration, ChannelNonce: hello.ChannelNonce, Deadline: hello.Deadline}
				if err := f.authority.presentation(presentation); err != nil {
					return nil, err
				}
				class, err := role.AdmissionClass(hello.Purpose)
				if err != nil {
					return nil, err
				}
				if uint8(class) == 2 {
					forwardingPresentation = presentation
				}
				return holder.Take(ctx, presentation, uint8(class))
			}})
			if err != nil {
				t.Fatal(err)
			}
			defer opened.Close()
			caller, cancel := context.WithCancel(t.Context())
			defer cancel()
			prepare := routeIssuerPreparation(caller, f.authority, holder, leg, 2, nil)
			var verified atomic.Bool
			err = opened.Issue(caller, plan.Deadline, func(binding routeprefix.IssuerBinding) (routeprefix.IssuerBatch, error) {
				batch, err := prepare(binding)
				if err != nil {
					return batch, err
				}
				complete := batch.Complete
				batch.Complete = func(payload []byte, exchangeErr error, check func() error) error {
					return complete(payload, exchangeErr, func() error {
						verified.Store(true)
						cancel() // no I/O or Stock callback while its lock is held
						return check()
					})
				}
				return batch, nil
			})
			if !verified.Load() || !errors.Is(err, context.Canceled) {
				t.Fatal("original caller not checked after actual signature verification", verified.Load(), err)
			}
			if s := holder.Status(); s.Busy || s.Pending || s.Remaining[1] != 4 {
				t.Fatal("retired verified completion retained blinders or refunded allocation", s)
			}
			forwardingPresentation.ChannelNonce = [32]byte{97}
			if err := f.authority.presentation(forwardingPresentation); err != nil {
				t.Fatal("actual Stock probe authority unavailable", err)
			}
			probe, takeErr := holder.Take(t.Context(), forwardingPresentation, 2)
			clear(probe)
			if stock.TransferFailureStage(takeErr) != "stock" {
				t.Fatal("retired verified tokens available or probe failed elsewhere", takeErr)
			}
			if err := opened.Close(); err != nil {
				t.Fatal("prefix join", err)
			}
			observed, err := budget.Observe(t.Context())
			if err != nil || observed.ReservedBytes != 0 {
				t.Fatal("actual holder Hosting not returned after join", observed.ReservedBytes, err)
			}
			for _, server := range x.servers {
				if err := server.Close(); err != nil {
					t.Error("receiving join", err)
				}
			}
			for _, budget := range x.budgets {
				observed, err := budget.Observe(t.Context())
				if err != nil || observed.ReservedBytes != 0 {
					t.Error("receiving Hosting not joined", observed.ReservedBytes, err)
				}
			}
		})
	}
}

// A real lost Route result remains committed at the issuer. Reopening the
// holder's presentation root cannot reconstruct its erased permission/blinders
// from the retained public request/result; it cannot adopt that old response.
func TestRouteIssuerLostResultHolderReopenRefusesReconstructionBothCarriers(t *testing.T) {
	for _, profile := range []transport.CarrierProfile{transport.ClosedCarrierTCP, transport.ClosedCarrierQUIC} {
		t.Run(string(profile), func(t *testing.T) {
			caller, cancel := context.WithCancel(t.Context())
			defer cancel()
			var request, response []byte
			defer func() { clear(request); clear(response) }()
			x := newRouteIssuerFixture(t, profile, func(ctx context.Context, bootstrap bool, raw, signed []byte) error {
				if !bootstrap {
					request = bytes.Clone(raw)
					response = bytes.Clone(signed)
					cancel()
				}
				return nil
			}, nil, nil, nil)
			f := x.network
			root := t.TempDir()
			holder := routePermissionStockAt(t, f, admission.AllocationUser, [3]uint32{2, 8, 0}, root)
			permissionRequest, permissionDigest, err := holder.Request([3]uint32{2, 8, 0})
			clear(permissionRequest)
			if err != nil {
				t.Fatal(err)
			}
			local := t.TempDir()
			plan := routePrefixPlan{EntryRoot: filepath.Join(local, "entry"), InteriorRoot: filepath.Join(local, "interior"), HostingRoot: routeProcessBudget(t), Domain: 1, Deadline: time.Now().Add(25 * time.Second).UTC().Truncate(time.Second), Work: hosting.Traffic{Tx: 2 << 20, Rx: 2 << 20}, Termination: hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}}
			boot, err := startRouteBootstrap(t.Context(), plan, f.authority, holder)
			if err != nil {
				t.Fatal(err)
			}
			defer boot.close()
			for _, class := range []uint8{2, 1} {
				if err := boot.issue(t.Context(), class, nil); err != nil {
					t.Fatal(err)
				}
			}
			if err := boot.close(); err != nil {
				t.Fatal(err)
			}
			original, err := startRoutePrefix(t.Context(), plan, f.authority, holder)
			if err != nil {
				t.Fatal(err)
			}
			defer original.close()
			if err := original.issue(caller, 2, nil); !errors.Is(err, context.Canceled) {
				t.Fatal("actual signed result not lost to original cancellation", err)
			}
			if len(request) == 0 || len(response) == 0 || !holder.Status().Pending {
				t.Fatal("committed request/result and volatile pending boundary absent")
			}
			if err := original.close(); err != nil {
				t.Fatal(err)
			}
			for _, server := range x.servers {
				if err := server.Close(); err != nil {
					t.Fatal(err)
				}
			}
			journal := func(root, name string) []byte {
				raw, err := os.ReadFile(filepath.Join(root, name))
				if err != nil {
					t.Fatal(err)
				}
				return raw
			}
			debits := journal(f.plan.AdmissionRoot, "admission.journal")
			results := journal(f.plan.ResultRoot, "results.journal")
			defer clear(debits)
			defer clear(results)
			observe := func() (admission.AuthorityFacts, time.Time, error) {
				return f.authority.issuer(f.plan.KeyBinding.Signer)
			}
			retry := issuer.IssueCurrent(t.Context(), f.plan, request, quota.Admitted, observe)
			defer clear(retry.Response)
			if retry.Outcome != "already-issued" || !bytes.Equal(retry.Response, response) {
				t.Fatal("genuine durable issuer reopen changed retained signed result", retry.Outcome)
			}
			wrongKind := issuer.IssueCurrent(t.Context(), f.plan, request, quota.Bootstrap, observe)
			defer clear(wrongKind.Response)
			if wrongKind.Outcome != "request-conflict" || wrongKind.Response != nil {
				t.Fatal("retained issuer request changed kind", wrongKind.Outcome)
			}
			changed := bytes.Clone(request)
			defer clear(changed)
			changed[len(changed)-1] ^= 1 // changed signed digest/proof, never a forged valid holder proof
			wrongDigest := issuer.IssueCurrent(t.Context(), f.plan, changed, quota.Admitted, observe)
			defer clear(wrongDigest.Response)
			if wrongDigest.Response != nil || wrongDigest.Outcome == "issued-offline" || wrongDigest.Outcome == "already-issued" {
				t.Fatal("changed signed request accepted", wrongDigest.Outcome)
			}
			afterDebits, afterResults := journal(f.plan.AdmissionRoot, "admission.journal"), journal(f.plan.ResultRoot, "results.journal")
			defer clear(afterDebits)
			defer clear(afterResults)
			if !bytes.Equal(debits, afterDebits) || !bytes.Equal(results, afterResults) {
				t.Fatal("retained retry/refusal altered irreversible histories")
			}
			batch, err := admission.DecodeClosedTokenBatch(request)
			if err != nil {
				t.Fatal(err)
			}
			permission, err := admission.EncodePermission(batch.Permission)
			if err != nil {
				t.Fatal(err)
			}
			defer clear(permission)
			if err := holder.Close(); err != nil {
				t.Fatal(err)
			}
			reopened, err := stock.Open(root, admission.AllocationUser, f.authority.observe)
			if err != nil {
				t.Fatal(err)
			}
			defer reopened.Close()
			if s := reopened.Status(); s.Accepted || s.Pending || s.Busy || s.BootstrapRemaining != 0 {
				t.Fatal("reopen reconstructed volatile holder authority", s)
			}
			if err := reopened.Import(permissionDigest, permission); err == nil {
				t.Fatal("old public permission adopted without live request")
			}
			if err := reopened.Import(permissionDigest, retry.Response); err == nil {
				t.Fatal("old signed issuer result became new holder permission")
			}
			freshPublic, freshDigest, err := reopened.Request([3]uint32{2, 8, 0})
			if err != nil {
				t.Fatal(err)
			}
			defer clear(freshPublic)
			fresh, err := admission.DecodePermissionRequest(freshPublic)
			if err != nil || freshDigest == permissionDigest || fresh.Permission.HolderKey == batch.Permission.HolderKey {
				t.Fatal("reopened holder reconstructed the lost request/key", err)
			}
			if err := reopened.Import(freshDigest, permission); err == nil {
				t.Fatal("new live holder adopted the old holder permission")
			}
			if s := reopened.Status(); s.Accepted || s.Pending || s.Busy {
				t.Fatal("refused restoration created authority", s)
			}
			for _, budget := range x.budgets {
				observed, err := budget.Observe(t.Context())
				if err != nil || observed.ReservedBytes != 0 {
					t.Error("receiving Hosting not joined", observed.ReservedBytes, err)
				}
			}
		})
	}
}

// Genuine signed Network and ordinary Stock/spend/Hosting exercise the actual
// receiver. Pending bootstrap allocation is not successful issuer exchange.
// Linux is required by the real Network, spend and Hosting roots in this test;
// portable queue, framing and capacity controls stay with their rule owners.
func TestRoutePendingBootstrapLimitLeavesOrdinaryChildUsableOnSameCarrier(t *testing.T) {
	for _, profile := range []transport.CarrierProfile{transport.ClosedCarrierTCP, transport.ClosedCarrierQUIC} {
		t.Run(string(profile), func(t *testing.T) {
			f, sockets, certificates := newRoleRouteFixture(t, profile, 1, false)
			// Two distinct genuine tokens: refusal must not refund/reuse the
			// durably presented restricted token to make the sibling succeed.
			holder := routeRoleStock(t, f, false, true)
			view, err := f.current()
			if err != nil {
				t.Fatal(err)
			}
			member, err := view.Member([32]byte{14}, time.Now())
			if err != nil {
				t.Fatal(err)
			}
			duty, err := view.RetainDuty(member.NodeID, time.Now())
			if err != nil {
				t.Fatal(err)
			}
			a := role.Authority{Current: f.current, Duty: duty, Profile: view.Profile().ProfileBinding}
			binding := receiving.Receiver{NetworkID: f.profile.NetworkID, StateGeneration: f.profile.StateGeneration, StateDigest: f.profile.StateDigest, ProfileDigest: f.profile.Digest, NodeID: member.NodeID, DutyGeneration: member.DutyGeneration}
			owner, err := receiving.Open(t.TempDir(), binding, func() (receiving.Observation, error) { return f.authority.receiver(binding, member.NotAfter()) })
			if err != nil {
				t.Fatal(err)
			}
			defer owner.Close()
			budget := networkTestBudget(t)
			var admits, releases, bootstrapHolds, bootstrapReturns atomic.Int32
			sockets[member.NodeID]()
			server, err := routereceiver.Listen(t.Context(), routereceiver.ReceiverConfig{Authority: a, Certificate: certificates[member.NodeID], ReserveBootstrap: func(ctx context.Context, end time.Time) (func() error, error) {
				held, err := budget.Reserve(ctx, hosting.ReservationRequest{Work: hosting.Traffic{Tx: 2 << 20, Rx: 2 << 20}, Termination: hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}, WorkUntil: end, HoldUntil: end.Add(time.Second)})
				if err != nil {
					return nil, err
				}
				bootstrapHolds.Add(1)
				return func() error { bootstrapReturns.Add(1); return releaseRouteReservation(held) }, nil
			}, Admit: func(ctx context.Context, c routereceiver.Channel, raw []byte) (receiving.Grant, error) {
				admits.Add(1)
				return owner.Accept(ctx, admission.ForwardClass, raw, c.Hello.Deadline, func() (func() error, error) {
					release, err := networkTestReservation(t, budget, c.Hello.Deadline)
					if err != nil {
						return nil, err
					}
					return c.HoldReservation(func() error { releases.Add(1); return release() })
				})
			}})
			if err != nil {
				t.Fatal(err)
			}
			defer server.Close()
			end := time.Now().Add(8 * time.Second).UTC().Truncate(time.Second)
			request := transport.ClosedNodeCarrierRequest{CarrierProfile: profile, Endpoint: member.Endpoint, Certificate: certificates[[32]byte{12}], ExpectedPeerKey: member.PublicKey, Deadline: end}
			carrier := routeFixtureNodeCarrier(t, request)
			defer carrier.Close()
			conn, ok := carrier.(net.Conn)
			if !ok {
				t.Fatal("actual Node Carrier lacks connection contract")
			}
			if err := conn.SetDeadline(end); err != nil {
				t.Fatal(err)
			}
			outer, err := a.FreshHello(end, ardp.PurposeForwarding, true)
			if err != nil {
				t.Fatal(err)
			}
			if err := framing.SendHello(conn, outer); err != nil {
				t.Fatal(err)
			}
			if err := framing.Accepted(conn); err != nil {
				t.Fatal(err)
			}
			opened := ardp.Open{RecipientNodeID: member.NodeID, RecipientDutyGeneration: member.DutyGeneration, Purpose: uint8(ardp.PurposeForwarding), Deadline: end}
			restricted, err := ardp.EncodeNodeOpen(ardp.NodeOpen{Recipient: opened, Restriction: ardp.IssuerBootstrapChild})
			if err != nil {
				t.Fatal(err)
			}
			unknownRestriction := append([]byte(nil), restricted...)
			unknownRestriction[49] = 2
			// Raw canonical frame I/O tests hostile input at the real receiving
			// boundary. Session.Open correctly refuses unknown restriction locally.
			for index, malformed := range [][]byte{restricted[:49], unknownRestriction} {
				id := uint32(1 + 2*index)
				if err := ardp.WriteFrame(conn, ardp.Frame{Kind: ardp.KindOpen, Lane: id, Body: malformed}); err != nil {
					t.Fatal(err)
				}
				if err := conn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
					t.Fatal(err)
				}
				refusal, err := ardp.ReadFrame(conn)
				if err != nil || refusal.Kind != ardp.KindClose || refusal.Lane != id || len(refusal.Body) != 1 || refusal.Body[0] == 0 || admits.Load() != 0 || bootstrapHolds.Load() != 0 {
					t.Fatal("missing/unknown Node restriction reached TLS capacity or omitted peer refusal", err, refusal, admits.Load(), bootstrapHolds.Load())
				}
			}
			if err := carrier.Close(); err != nil {
				t.Fatal("malformed-input Carrier cleanup", err)
			}
			// Fresh physical carrier and nonce preserve monotonically increasing
			// lane IDs without injecting state into the actual portable Session.
			carrier = routeFixtureNodeCarrier(t, request)
			defer carrier.Close()
			conn, ok = carrier.(net.Conn)
			if !ok {
				t.Fatal("actual Node Carrier lacks connection contract")
			}
			if err := conn.SetDeadline(end); err != nil {
				t.Fatal(err)
			}
			outer, err = a.FreshHello(end, ardp.PurposeForwarding, true)
			if err != nil {
				t.Fatal(err)
			}
			if err := framing.SendHello(conn, outer); err != nil {
				t.Fatal(err)
			}
			if err := framing.Accepted(conn); err != nil {
				t.Fatal(err)
			}
			s := framing.New(t.Context(), conn, end, 32<<20, func() error { _, err := a.Hello(outer, true); return err }, false, framing.NewBudget(1<<20), nil)
			defer s.Close()
			var restrictedLanes []*framing.Lane
			for range 5 {
				lane, err := s.Open(t.Context(), t.Context(), restricted)
				if err != nil {
					t.Fatal(err)
				}
				defer lane.Finish()
				restrictedLanes = append(restrictedLanes, lane)
			}
			fifth := restrictedLanes[4]
			// Four original children still own their pending TLS reservations.
			// The fifth must refuse promptly, before its original deadline.
			if err := fifth.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
				t.Fatal(err)
			}
			if n, err := fifth.Read(make([]byte, 1)); n != 0 || err == nil || errors.Is(err, os.ErrDeadlineExceeded) || !time.Now().Before(end) {
				t.Fatal("fifth pending bootstrap was not refused before its original bound", n, err)
			}
			if !s.Live() || admits.Load() != 0 {
				t.Fatal("bootstrap capacity reached private spend or retired shared Carrier", admits.Load())
			}
			waitUntil := time.Now().Add(2 * time.Second)
			for bootstrapHolds.Load() != 4 && time.Now().Before(waitUntil) {
				time.Sleep(time.Millisecond)
			}
			heldObservation, err := budget.Observe(t.Context())
			if err != nil || bootstrapHolds.Load() != 4 || bootstrapReturns.Load() != 0 || heldObservation.ReservedBytes == 0 {
				t.Fatal("pending TLS did not retain genuine Hosting work", err, bootstrapHolds.Load(), bootstrapReturns.Load())
			}
			// Complete TLS on one restricted child, then attempt actual private
			// admission with a genuinely issued and durably presented token.
			// Route must reject before invoking receiving Admission at all.
			restrictedTLS, err := routeFixtureRoleTLS(t, restrictedLanes[0], member.PublicKey, end)
			if err != nil {
				t.Fatal("restricted TLS setup", err)
			}
			restrictedHello, err := a.FreshHello(end, ardp.PurposeForwarding, false)
			if err != nil {
				t.Fatal(err)
			}
			presented := false
			err = role.Present(t.Context(), t.Context(), restrictedTLS, a, restrictedHello, func(ctx context.Context, hello ardp.Hello) ([]byte, error) {
				p := stock.Presentation{NetworkID: hello.NetworkID, StateGeneration: hello.StateGeneration, StateDigest: hello.StateDigest, ProfileDigest: hello.ProfileDigest, RecipientNodeID: hello.RecipientNodeID, RecipientDutyGeneration: hello.RecipientDutyGeneration, ChannelNonce: hello.ChannelNonce, Deadline: hello.Deadline}
				if err := f.authority.presentation(p); err != nil {
					return nil, err
				}
				raw, err := holder.Take(ctx, p, 2)
				if err != nil {
					return nil, err
				}
				presented = true // Take has independently verified the signature.
				return raw, nil
			})
			if err == nil || errors.Is(err, os.ErrDeadlineExceeded) || !presented || admits.Load() != 0 || !s.Live() || !time.Now().Before(end) {
				t.Fatal("restricted valid-token admission crossed receiving boundary", err, presented, admits.Load(), s.Live())
			}
			if n, err := restrictedLanes[0].Read(make([]byte, 1)); n != 0 || !errors.Is(err, io.EOF) {
				t.Fatal("restricted refusal omitted actual peer lane termination", n, err)
			}
			// This intentional protocol violation ends at the peer's CLOSE.
			// Retain the resulting local TLS shutdown failure rather than
			// reclassifying the forbidden operation as a successful exchange.
			if err := restrictedTLS.Close(); !errors.Is(err, io.EOF) {
				t.Fatal("restricted TLS refusal lost peer-retirement failure", err)
			}
			ordinary, err := ardp.EncodeNodeOpen(ardp.NodeOpen{Recipient: opened, Restriction: ardp.OrdinaryChild})
			if err != nil {
				t.Fatal(err)
			}
			lane, err := s.Open(t.Context(), t.Context(), ordinary)
			if err != nil {
				t.Fatal(err)
			}
			defer lane.Finish()
			secured, err := routeFixtureRoleTLS(t, lane, member.PublicKey, end)
			if err != nil {
				t.Fatal("ordinary TLS failed alongside pending restricted children", err)
			}
			defer secured.Close()
			h, err := a.FreshHello(end, ardp.PurposeForwarding, false)
			if err != nil {
				t.Fatal(err)
			}
			if err := role.Present(t.Context(), t.Context(), secured, a, h, func(ctx context.Context, hello ardp.Hello) ([]byte, error) {
				p := stock.Presentation{NetworkID: hello.NetworkID, StateGeneration: hello.StateGeneration, StateDigest: hello.StateDigest, ProfileDigest: hello.ProfileDigest, RecipientNodeID: hello.RecipientNodeID, RecipientDutyGeneration: hello.RecipientDutyGeneration, ChannelNonce: hello.ChannelNonce, Deadline: hello.Deadline}
				if err := f.authority.presentation(p); err != nil {
					return nil, err
				}
				return holder.Take(ctx, p, 2)
			}); err != nil {
				t.Fatal("ordinary genuine admission unavailable on same Carrier", err)
			}
			if admits.Load() != 1 {
				t.Fatal("restricted allocations entered receiving Admission", admits.Load())
			}
			for _, l := range restrictedLanes {
				if err := l.Close(); err != nil {
					t.Fatal("restricted child retirement failed", err)
				}
			}
			// Keep the shared Carrier alive until the peer has completed its
			// TLS shutdown and emitted this child's actual terminal frame.
			if err := secured.CloseWrite(); err != nil {
				t.Fatal("ordinary TLS half-close failed", err)
			}
			if _, err := io.Copy(io.Discard, secured); err != nil {
				t.Fatal("ordinary TLS peer shutdown failed", err)
			}
			if n, err := lane.Read(make([]byte, 1)); n != 0 || err == nil || errors.Is(err, os.ErrDeadlineExceeded) || !s.Live() {
				t.Fatal("ordinary child did not reach its peer terminal", n, err)
			}
			if err := secured.Close(); err != nil {
				t.Fatal("ordinary TLS retirement failed", err)
			}
			if err := s.Close(); err != nil {
				t.Fatal("shared Carrier retirement failed", err)
			}
			if err := server.Close(); err != nil {
				t.Fatal("receiving mixed Carrier did not join cleanly", err)
			}
			observation, err := budget.Observe(t.Context())
			if err != nil || observation.ReservedBytes != 0 || releases.Load() != 1 || bootstrapHolds.Load() != 4 || bootstrapReturns.Load() != 4 {
				t.Fatal("joined mixed Carrier lost physical reservation", err, releases.Load(), bootstrapHolds.Load(), bootstrapReturns.Load())
			}
		})
	}
}

// This genuine network roundtrip obtains both bootstrap batches through one
// retained Entry/Interior selection. Ordinary issuance and compiled holder
// orchestration are separate acceptance obligations; no offline issue fills stock.
func TestRouteBootstrapObtainsStockFromActualIssuer(t *testing.T) {
	for _, profile := range []transport.CarrierProfile{transport.ClosedCarrierTCP, transport.ClosedCarrierQUIC} {
		t.Run(string(profile), func(t *testing.T) {
			f, sockets, certificates := newRoleRouteFixture(t, profile, 1, false, true)
			holder := routePermissionStock(t, f, admission.AllocationUser, [3]uint32{2, 4, 0})
			entryRoot, interiorRoot := t.TempDir(), t.TempDir()
			for _, root := range []string{entryRoot, interiorRoot} {
				if err := os.Chmod(root, 0700); err != nil {
					t.Fatal(err)
				}
			}
			selected, err := selection.Open(selection.Config{EntryRoot: entryRoot, InteriorRoot: interiorRoot, Domain: 1, Current: f.current})
			if err != nil {
				t.Fatal(err)
			}
			defer selected.Close()
			leg, err := selected.Select()
			if err != nil {
				t.Fatal(err)
			}
			view, err := f.current()
			if err != nil {
				t.Fatal(err)
			}
			ids := [][32]byte{leg.Entry.NodeID, leg.Interior.NodeID, f.profile.IssuerNodeID}
			var authorities []role.Authority
			var servers []*routereceiver.Receiver
			var budgets []*hosting.Budget
			var holds, returns, admits, issues atomic.Int32
			t.Cleanup(func() {
				if t.Failed() {
					t.Log("bootstrap lifecycle counts", holds.Load(), returns.Load(), admits.Load(), issues.Load())
				}
			})
			for index, id := range ids {
				duty, err := view.RetainDuty(id, time.Now())
				if err != nil {
					t.Fatal(err)
				}
				a := role.Authority{Current: f.current, Duty: duty, Profile: view.Profile().ProfileBinding}
				authorities = append(authorities, a)
				budget := networkTestBudget(t)
				budgets = append(budgets, budget)
				config := routereceiver.ReceiverConfig{Authority: a, Certificate: certificates[id],
					ReserveBootstrap: func(ctx context.Context, end time.Time) (func() error, error) {
						held, err := budget.Reserve(ctx, hosting.ReservationRequest{Work: hosting.Traffic{Tx: 2 << 20, Rx: 2 << 20}, Termination: hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}, WorkUntil: end, HoldUntil: end.Add(time.Second)})
						if err != nil {
							return nil, err
						}
						holds.Add(1)
						return func() error { returns.Add(1); return releaseRouteReservation(held) }, nil
					},
					Admit: func(context.Context, routereceiver.Channel, []byte) (receiving.Grant, error) {
						admits.Add(1)
						return receiving.Grant{}, errors.New("private Admission in bootstrap-only roundtrip")
					},
				}
				if index == 2 {
					config.Issue = func(ctx context.Context, batch []byte, bootstrap bool) ([]byte, error) {
						if !bootstrap {
							return nil, errors.New("bootstrap kind lost before actual issuer")
						}
						issues.Add(1)
						return issueRouteBatch(ctx, f.authority, f.plan, batch, bootstrap)
					}
				}
				sockets[id]()
				server, err := routereceiver.Listen(t.Context(), config)
				if err != nil {
					t.Fatal(err)
				}
				servers = append(servers, server)
				defer func() {
					err := server.Close()
					if t.Failed() && err != nil {
						t.Log("bootstrap receiver joined failure", err)
					}
				}()
			}
			end := time.Now().Add(8 * time.Second).UTC().Truncate(time.Second)
			entry := leg.EntryMember
			conn, err := routeTestOpenEndpoint(t.Context(), transport.ClosedRoleCarrierRequest{CarrierProfile: profile, Endpoint: entry.Endpoint, ExpectedServer: entry.PublicKey, Deadline: end})
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			if err := conn.SetDeadline(end); err != nil {
				t.Fatal(err)
			}
			h, err := authorities[0].FreshHello(end, ardp.PurposeForwarding, false)
			if err != nil {
				t.Fatal(err)
			}
			sendBootstrap := func(conn net.Conn, h ardp.Hello) {
				t.Helper()
				if err := framing.SendHello(conn, h); err != nil {
					t.Fatal(err)
				}
				if err := ardp.WriteFrame(conn, ardp.Frame{Kind: ardp.KindBootstrap, Body: []byte{2}}); err != nil {
					t.Fatal(err)
				}
				if err := framing.Accepted(conn); err != nil {
					t.Fatal("real bootstrap refused", err)
				}
			}
			sendBootstrap(conn, h)
			entrySession := framing.New(t.Context(), conn, end, 128<<10, func() error {
				v, err := f.current()
				if err != nil {
					return err
				}
				return leg.Check(v, time.Now())
			}, false, framing.NewBudget(256<<10), nil)
			defer entrySession.Close()
			interior := leg.InteriorMember
			lane, err := entrySession.Open(t.Context(), t.Context(), ardp.EncodeOpen(ardp.Open{RecipientNodeID: interior.NodeID, RecipientDutyGeneration: interior.DutyGeneration, Purpose: 7, Deadline: end}, false))
			if err != nil {
				t.Fatal(err)
			}
			defer lane.Finish()
			interiorTLS, err := routeFixtureRoleTLS(t, lane, interior.PublicKey, end)
			if err != nil {
				t.Fatal(err)
			}
			innerHello, err := authorities[1].FreshHello(end, ardp.PurposeForwarding, false)
			if err != nil {
				t.Fatal(err)
			}
			sendBootstrap(interiorTLS, innerHello)
			interiorSession := framing.New(t.Context(), interiorTLS, end, 128<<10, func() error { _, err := authorities[1].Hello(innerHello, false); return err }, false, framing.NewBudget(256<<10), nil)
			defer interiorSession.Close()
			issuerMember, err := authorities[2].Member()
			if err != nil {
				t.Fatal(err)
			}
			for batchIndex, class := range []uint8{2, 1} {
				var challenges []token.ClosedTokenContext
				receivers := []struct {
					id   [32]byte
					duty uint64
				}{{entry.NodeID, entry.DutyGeneration}, {interior.NodeID, interior.DutyGeneration}}
				if class == 1 {
					receivers = []struct {
						id   [32]byte
						duty uint64
					}{{issuerMember.NodeID, issuerMember.DutyGeneration}}
				}
				for _, receiver := range receivers {
					challenges = append(challenges, token.ClosedTokenContext{NetworkID: f.profile.NetworkID, ProfileDigest: f.profile.Digest, IssuerNodeID: issuerMember.NodeID, ReceiverNodeID: receiver.id, ReceiverDutyGeneration: receiver.duty, Class: class, WindowStart: time.Now().UTC().Truncate(time.Hour)})
				}
				attempt, err := holder.Begin(stock.IssuanceIntent{Challenges: challenges, Selection: stock.ExchangeBinding{ID: [32]byte{byte(81 + batchIndex)}, ProfileDigest: f.profile.Digest}, Bootstrap: true, Deadline: end})
				if err != nil {
					t.Fatal(err)
				}
				batch, _, err := attempt.Request()
				if err != nil {
					t.Fatal(err)
				}
				terminalLane, err := interiorSession.Open(t.Context(), t.Context(), ardp.EncodeOpen(ardp.Open{RecipientNodeID: issuerMember.NodeID, RecipientDutyGeneration: issuerMember.DutyGeneration, Purpose: uint8(ardp.PurposeIssuer), Deadline: end}, false))
				if err != nil {
					t.Fatal(err)
				}
				secured, err := routeFixtureRoleTLS(t, terminalLane, issuerMember.PublicKey, end)
				if err != nil {
					t.Fatal("actual issuer TLS unavailable", err)
				}
				issuerHello, err := authorities[2].FreshHello(end, ardp.PurposeIssuer, false)
				if err != nil {
					t.Fatal(err)
				}
				sendBootstrap(secured, issuerHello)
				nonce := [32]byte{byte(91 + batchIndex)}
				body, err := ardp.EncodeIssuerRequest(nonce, batch)
				if err != nil {
					t.Fatal(err)
				}
				if err := ardp.WriteFrame(secured, ardp.Frame{Kind: ardp.KindOperation, Body: body}); err != nil {
					t.Fatal(err)
				}
				result, err := ardp.ReadFrame(secured)
				if err != nil || result.Kind != ardp.KindResult || result.Lane != 0 {
					t.Fatal("actual issuer RESULT unavailable", batchIndex, err, result.Kind, result.Lane)
				}
				status, payload, err := ardp.DecodeIssuerResult(result.Body, nonce)
				if err != nil || status != 0 {
					t.Fatal("issuer refused actual batch", err, status)
				}
				if err := attempt.Complete(payload, nil); err != nil {
					t.Fatal("actual network stock finalization failed", err)
				}
				clear(batch)
				clear(body)
				clear(result.Body)
				if err := secured.CloseWrite(); err != nil {
					t.Fatal("issuer client TLS half-close failed", err)
				}
				if _, err := io.Copy(io.Discard, secured); err != nil {
					t.Fatal("issuer TLS shutdown failed", err)
				}
				if n, err := terminalLane.Read(make([]byte, 1)); n != 0 || err == nil || errors.Is(err, os.ErrDeadlineExceeded) {
					t.Fatal("issuer peer terminal unavailable", batchIndex, n, err)
				}
				if err := secured.Close(); err != nil {
					t.Fatal(err)
				}
				terminalLane.Finish()
			}
			if issues.Load() != 2 || admits.Load() != 0 {
				t.Fatal("stock bypassed real bootstrap issuer", issues.Load(), admits.Load())
			}
			if err := interiorSession.Close(); err != nil {
				t.Fatal(err)
			}
			if err := entrySession.Close(); err != nil {
				t.Fatal(err)
			}
			for _, server := range servers {
				if err := server.Close(); err != nil {
					t.Fatal("issuer bootstrap did not join", err)
				}
			}
			if holds.Load() != 4 || returns.Load() != 4 {
				t.Fatal("bootstrap Hosting returns lost", holds.Load(), returns.Load())
			}
			for _, budget := range budgets {
				observation, err := budget.Observe(t.Context())
				if err != nil || observation.ReservedBytes != 0 {
					t.Fatal("joined bootstrap retained genuine Hosting", err)
				}
			}
		})
	}
}

// This proves a genuine bounded Entry/Interior bootstrap channel, not token
// issuance: no holder stock, successful Admission substitute or issuer result
// participates. The issuer terminal and stock finalization remain separate.
func TestRouteHostedEntryPropagatesBootstrapToInterior(t *testing.T) {
	for _, profile := range []transport.CarrierProfile{transport.ClosedCarrierTCP, transport.ClosedCarrierQUIC} {
		t.Run(string(profile), func(t *testing.T) {
			f, sockets, certificates := newRoleRouteFixture(t, profile, 1, false)
			view, err := f.current()
			if err != nil {
				t.Fatal(err)
			}
			var authorities []role.Authority
			var servers []*routereceiver.Receiver
			var budgets []*hosting.Budget
			var holds, returns, admits atomic.Int32
			for _, id := range [][32]byte{{12}, {14}} {
				duty, err := view.RetainDuty(id, time.Now())
				if err != nil {
					t.Fatal(err)
				}
				a := role.Authority{Current: f.current, Duty: duty, Profile: view.Profile().ProfileBinding}
				authorities = append(authorities, a)
				budget := networkTestBudget(t)
				budgets = append(budgets, budget)
				sockets[id]()
				server, err := routereceiver.Listen(t.Context(), routereceiver.ReceiverConfig{Authority: a, Certificate: certificates[id],
					ReserveBootstrap: func(ctx context.Context, end time.Time) (func() error, error) {
						held, err := budget.Reserve(ctx, hosting.ReservationRequest{Work: hosting.Traffic{Tx: 2 << 20, Rx: 2 << 20}, Termination: hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}, WorkUntil: end, HoldUntil: end.Add(time.Second)})
						if err != nil {
							return nil, err
						}
						holds.Add(1)
						return func() error { returns.Add(1); return releaseRouteReservation(held) }, nil
					},
					Admit: func(context.Context, routereceiver.Channel, []byte) (receiving.Grant, error) {
						admits.Add(1)
						return receiving.Grant{}, errors.New("unexpected private Admission in bootstrap-only scenario")
					},
				})
				if err != nil {
					t.Fatal(err)
				}
				servers = append(servers, server)
				defer server.Close()
			}
			end := time.Now().Add(8 * time.Second).UTC().Truncate(time.Second)
			entry, err := authorities[0].Member()
			if err != nil {
				t.Fatal(err)
			}
			conn, err := routeTestOpenEndpoint(t.Context(), transport.ClosedRoleCarrierRequest{CarrierProfile: profile, Endpoint: entry.Endpoint, ExpectedServer: entry.PublicKey, Deadline: end})
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			if err := conn.SetDeadline(end); err != nil {
				t.Fatal(err)
			}
			h, err := authorities[0].FreshHello(end, ardp.PurposeForwarding, false)
			if err != nil {
				t.Fatal(err)
			}
			if err := framing.SendHello(conn, h); err != nil {
				t.Fatal(err)
			}
			if err := ardp.WriteFrame(conn, ardp.Frame{Kind: ardp.KindBootstrap, Body: []byte{2}}); err != nil {
				t.Fatal(err)
			}
			if err := framing.Accepted(conn); err != nil {
				t.Fatal("actual Entry bootstrap refused", err)
			}
			s := framing.New(t.Context(), conn, end, 128<<10, func() error { _, err := authorities[0].Hello(h, false); return err }, false, framing.NewBudget(1<<20), nil)
			defer s.Close()
			interior, err := authorities[1].Member()
			if err != nil {
				t.Fatal(err)
			}
			body := ardp.EncodeOpen(ardp.Open{RecipientNodeID: interior.NodeID, RecipientDutyGeneration: interior.DutyGeneration, Purpose: uint8(ardp.PurposeForwarding), Deadline: end}, false)
			lane, err := s.Open(t.Context(), t.Context(), body)
			if err != nil {
				t.Fatal(err)
			}
			defer lane.Finish()
			secured, err := routeFixtureRoleTLS(t, lane, interior.PublicKey, end)
			if err != nil {
				t.Fatal("actual restricted Interior TLS refused", err)
			}
			defer secured.Close()
			innerHello, err := authorities[1].FreshHello(end, ardp.PurposeForwarding, false)
			if err != nil {
				t.Fatal(err)
			}
			if err := framing.SendHello(secured, innerHello); err != nil {
				t.Fatal(err)
			}
			if err := ardp.WriteFrame(secured, ardp.Frame{Kind: ardp.KindBootstrap, Body: []byte{2}}); err != nil {
				t.Fatal(err)
			}
			if err := framing.Accepted(secured); err != nil {
				t.Fatal("actual incoming restricted bootstrap refused", err)
			}
			if holds.Load() != 2 || returns.Load() != 0 || admits.Load() != 0 {
				t.Fatal("bootstrap lost original physical claims or entered private Admission", holds.Load(), returns.Load(), admits.Load())
			}
			for _, budget := range budgets {
				observation, err := budget.Observe(t.Context())
				if err != nil || observation.ReservedBytes == 0 {
					t.Fatal("ready bootstrap lost actual Hosting work", err)
				}
			}
			if err := secured.CloseWrite(); err != nil {
				t.Fatal(err)
			}
			if _, err := io.Copy(io.Discard, secured); err != nil {
				t.Fatal("bootstrap peer TLS shutdown", err)
			}
			if n, err := lane.Read(make([]byte, 1)); n != 0 || err == nil || errors.Is(err, os.ErrDeadlineExceeded) {
				t.Fatal("bootstrap child terminal unavailable", n, err)
			}
			if err := s.Close(); err != nil {
				t.Fatal(err)
			}
			for _, server := range servers {
				if err := server.Close(); err != nil {
					t.Fatal("bootstrap receiver did not join", err)
				}
			}
			for _, budget := range budgets {
				observation, err := budget.Observe(t.Context())
				if err != nil || observation.ReservedBytes != 0 {
					t.Fatal("joined bootstrap retained provider reserve", err)
				}
			}
			if returns.Load() != 2 || admits.Load() != 0 {
				t.Fatal("bootstrap return was not once-only", returns.Load(), admits.Load())
			}
		})
	}
}
