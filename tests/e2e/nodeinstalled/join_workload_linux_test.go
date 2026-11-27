//go:build linux && join_platform_native

package nodeinstalled_test

import (
	"context"
	"crypto/rand"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/domain/admission"
	"github.com/dianabuilds/ardents-network/internal/domain/admission/stock"
	"github.com/dianabuilds/ardents-network/internal/domain/endpoint/session"
	"github.com/dianabuilds/ardents-network/tests/e2e/node/processfixture"
)

func (f installedRouteFixture) verifyJoin(t *testing.T, binary, spendRoot string, installed [32]byte, phase string) {
	t.Helper()
	if !t.Run(phase+" authenticated JOIN exchange", func(t *testing.T) {
		peers := f.startForwardingPeers(t, binary, installed)
		userStock := processfixture.FundRouteStock(t, f.spec, f.bundle, f.issuer, f.holder.CurrentRuntime, admission.AllocationUser, [32]byte{})
		publisherStock := processfixture.FundRouteStock(t, f.spec, f.bundle, f.issuer, f.holder.CurrentRuntime, admission.AllocationPublisher, [32]byte{})
		deadline := time.Now().Add(60 * time.Second).UTC().Truncate(time.Second)
		ctx, cancel := context.WithDeadline(t.Context(), deadline)
		defer cancel()
		before := installedSpends(t, spendRoot)
		open := func(domain uint8, holder *stock.Owner) (session.JoinContext, session.Prefix) {
			t.Helper()
			plan := installedHolderPlan(t, domain)
			plan.Deadline = deadline
			owner, err := session.OpenJoinContext(ctx, plan, f.authority(), holder)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = owner.Close() })
			prefix, err := owner.Open(ctx)
			if err != nil {
				t.Fatal(err)
			}
			return owner, prefix
		}
		userContext, user := open(1, userStock)
		defer user.Close()
		publisherContext, publisher := open(3, publisherStock)
		defer publisher.Close()

		choice, recipient := installedJoinRecipient(t, user, installed)
		end := deadline
		if recipient.NotAfter.Before(end) {
			end = recipient.NotAfter
		}
		intent := session.JoinIntent{Choice: choice, Node: recipient.Node, Generation: recipient.Generation,
			Deadline: end, SetupDeadline: time.Now().Add(20 * time.Second).UTC().Truncate(time.Second)}
		if _, err := rand.Read(intent.Secret[:]); err != nil {
			t.Fatal(err)
		}
		if _, err := rand.Read(intent.Context[:]); err != nil {
			t.Fatal(err)
		}

		type outcome struct {
			side   int
			stream net.Conn
			err    error
		}
		opened := make(chan outcome, 2)
		go func() { stream, err := user.Join(ctx, intent); opened <- outcome{0, stream, err} }()
		go func() { stream, err := publisher.Join(ctx, intent); opened <- outcome{1, stream, err} }()
		var streams [2]net.Conn
		var joinedErr error
		for range streams {
			result := <-opened
			streams[result.side] = result.stream
			joinedErr = errors.Join(joinedErr, result.err)
			if result.stream != nil {
				defer result.stream.Close()
			}
		}
		if joinedErr != nil {
			t.Fatal("installed paired JOIN", joinedErr)
		}
		for side, stream := range streams {
			processfixture.TransferJoinDirection(t, stream, streams[1-side], byte(71+side))
			processfixture.FinishJoinDirection(t, stream, streams[1-side])
		}
		if err := errors.Join(streams[0].Close(), streams[1].Close(), user.Close(), publisher.Close(), userContext.Close(), publisherContext.Close()); err != nil {
			t.Fatal("authenticated paired JOIN did not join", err)
		}
		requireInstalledSpend(t, spendRoot, before)
		stopInstalledPeers(t, peers)
	}) {
		t.FailNow()
	}
}

func installedJoinRecipient(t *testing.T, prefix session.Prefix, installed [32]byte) (uint8, session.Recipient) {
	t.Helper()
	for slot := range uint8(2) {
		recipient, err := prefix.Recipient(slot)
		if err != nil {
			t.Fatal("authenticated JOIN recipient unavailable", err)
		}
		if recipient.Node == installed {
			return slot, recipient
		}
	}
	t.Fatal("original installed JOIN is absent from both retained recipient choices")
	return 0, session.Recipient{}
}
