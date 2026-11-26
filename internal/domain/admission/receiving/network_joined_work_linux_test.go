package receiving_test

import (
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/domain/admission"
	"github.com/dianabuilds/ardents-network/internal/domain/admission/issuer"
	"github.com/dianabuilds/ardents-network/internal/domain/admission/receiving"
	"github.com/dianabuilds/ardents-network/internal/domain/network/state"
	"github.com/dianabuilds/ardents-network/tests/networkfixture"
)

func TestNetworkAdmissionHostingSignedStateAndJoinedWork(t *testing.T) {
	fixture := networkfixture.OpenSignedAdmission(t)
	facts, _, err := fixture.ObserveIssuer()
	if err != nil {
		t.Fatal(err)
	}
	reopen := func() {
		t.Helper()
		if err := fixture.State.Close(); err != nil {
			t.Fatal(err)
		}
		owner, err := state.OpenRetained(fixture.Config)
		if err != nil {
			t.Fatal(err)
		}
		fixture.State = owner
	}
	reopen()
	if recovered, _, err := fixture.ObserveIssuer(); err != nil || recovered != facts {
		t.Fatal("Network recovery changed authority", err)
	}
	networkView, err := fixture.State.CurrentRuntime()
	if err != nil {
		t.Fatal(err)
	}
	if err := issuer.ValidateCurrentAuthority(networkView, facts, networkView.ObservedAt(), [32]byte{99}); err == nil {
		t.Fatal("foreign issuer signing key accepted")
	}
	binding, tokens, _ := issueFixtureTokens(t, fixture, admission.ForwardClass, 1)
	budget := openReceivingBudget(t)
	deadline := time.Now().Add(5 * time.Second)
	observe := func() (receiving.Observation, error) { return observeReceiver(fixture, binding) }
	root := t.TempDir()
	receiver, err := receiving.Open(root, binding, observe)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = receiver.Close() })
	grant, err := receiver.Accept(t.Context(), admission.ForwardClass, tokens[0], deadline, func() (func() error, error) { return reserveForwardWork(t, budget, deadline) })
	if err != nil {
		t.Fatal(err)
	}
	defer grant.Release()
	// The real socket reader can stop but still await its owner's join. Closing
	// either authority owner must leave transferred Hosting capacity retained.
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	started, stopped, join := make(chan struct{}), make(chan struct{}), make(chan struct{})
	done := make(chan struct{})
	var workErr error
	allowJoin := sync.OnceFunc(func() { close(join) })
	defer func() {
		cancel()
		allowJoin()
		<-done
	}()
	go func() {
		defer close(done)
		workErr = networkfixture.TransferLocal(ctx, 64<<10, grant.Allowance().Deadline(), networkfixture.WorkAuthority(facts, time.Now(), deadline, fixture.ObserveIssuer), func(reader io.Reader, n int64) error {
			close(started)
			<-ctx.Done()
			err := networkfixture.DiscardPayload(reader, n)
			close(stopped)
			<-join
			return err
		})
	}()
	select {
	case <-started:
	case <-done:
		t.Fatal("socket child did not start", workErr)
	case <-t.Context().Done():
		t.Fatal(t.Context().Err())
	}
	if err := receiver.Close(); err != nil {
		t.Fatal(err)
	}
	if err := fixture.State.Close(); err != nil {
		t.Fatal(err)
	}
	cancel()
	select {
	case <-stopped:
	case <-done:
		t.Fatal("socket child returned before its join barrier", workErr)
	case <-t.Context().Done():
		t.Fatal(t.Context().Err())
	}
	view, observeErr := budget.Observe(t.Context())
	allowJoin()
	<-done
	if observeErr != nil || view.ReservedBytes == 0 {
		t.Fatal("capacity released before socket join", view, observeErr)
	}
	if !errors.Is(workErr, context.Canceled) {
		t.Fatal("socket cancellation lost", workErr)
	}
	if err := grant.Release(); err != nil {
		t.Fatal(err)
	}
	view, err = budget.Observe(t.Context())
	if err != nil || view.ReservedBytes != 0 {
		t.Fatal("capacity retained after joined release", view, err)
	}
	reopen()
	reopened, err := receiving.Open(root, binding, observe)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	deadline = time.Now().Add(5 * time.Second)
	if _, err := reopened.Accept(t.Context(), admission.ForwardClass, tokens[0], deadline, func() (func() error, error) { return reserveForwardWork(t, budget, deadline) }); err == nil || !strings.Contains(err.Error(), "closed token is already spent") {
		t.Fatalf("reopened owners did not report retained burn: %v", err)
	}
	view, err = budget.Observe(t.Context())
	if err != nil || view.ReservedBytes != 0 {
		t.Fatal("refused replay leaked capacity", view, err)
	}
}
