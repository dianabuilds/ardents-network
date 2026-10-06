package prefix

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/network"
	"github.com/dianabuilds/ardents-network/internal/successor/route/selection"
)

// These refusal controls exercise the portable original lifetime; they supply
// no successful Network observation, permission, token or issuer response.
func TestIssuerRetiredCallerRefusesBeforeStockAndAuthorityEffects(t *testing.T) {
	for _, bootstrap := range []bool{false, true} {
		name := "ordinary"
		if bootstrap {
			name = "bootstrap"
		}
		t.Run(name, func(t *testing.T) { testIssuerRetiredCaller(t, bootstrap) })
	}
}

func testIssuerRetiredCaller(t *testing.T, bootstrap bool) {
	t.Helper()
	p, _, returns, cleanup := terminalSetupPhysicalPrefix(t)
	defer cleanup()
	p.bootstrap = bootstrap
	issue, closePrefix, done := p.Issue, p.Close, p.Done()
	if bootstrap {
		// This is a physical lifetime refusal, without successful bootstrap
		// construction or supplied Network/Admission authority.
		bounded := &BootstrapPrefix{lifetime: p}
		issue, closePrefix, done = bounded.Issue, bounded.Close, bounded.Done()
	}
	p.config.Deadline = time.Now().Add(time.Minute)
	var observations, preparations atomic.Int32
	p.config.Current = func() (network.RuntimeView, error) {
		observations.Add(1)
		return network.RuntimeView{}, errors.New("unexpected observation")
	}
	caller, cancel := context.WithCancel(t.Context())
	cancel()
	err := issue(caller, time.Now().Add(20*time.Second).UTC().Truncate(time.Second), func(IssuerBinding) (IssuerBatch, error) {
		preparations.Add(1)
		return IssuerBatch{}, errors.New("unexpected Stock preparation")
	})
	if !errors.Is(err, context.Canceled) || observations.Load() != 0 || preparations.Load() != 0 || returns.Load() != 0 {
		t.Fatal("retired issuer request crossed effect boundary", err, observations.Load(), preparations.Load(), returns.Load())
	}
	select {
	case <-done:
		t.Fatal("request refusal retired the original parent before physical close")
	default:
	}
	if err := closePrefix(); err != nil || returns.Load() != 1 {
		t.Fatal("original parents did not join", err, returns.Load())
	}
	select {
	case <-done:
	default:
		t.Fatal("typed original lifetime remained ready after physical close")
	}
}

func TestBootstrapConstructionRefusesWrongDomainAndUnboundedDeadlineBeforeEffects(t *testing.T) {
	for _, mode := range []string{"wrong-domain", "long-deadline"} {
		t.Run(mode, func(t *testing.T) {
			var observations, returns int
			leg := selection.Leg{}
			leg.EntryMember.RoleDomain = 1
			end := time.Now().Add(8 * time.Second).UTC().Truncate(time.Second)
			if mode == "wrong-domain" {
				leg.EntryMember.RoleDomain = 3
			} else {
				end = time.Now().Add(time.Minute).UTC().Truncate(time.Second)
			}
			leg.NotAfter = end.Add(time.Minute)
			opened, err := OpenBootstrap(t.Context(), BootstrapConfig{Leg: leg, Deadline: end, Current: func() (network.RuntimeView, error) {
				observations++
				return network.RuntimeView{}, errors.New("unexpected observation")
			}, Release: func() error { returns++; return nil }})
			if opened != nil || err == nil || observations != 0 || returns != 0 {
				t.Fatal("invalid bootstrap acquired effects or caller reservation", err, observations, returns)
			}
		})
	}
}
