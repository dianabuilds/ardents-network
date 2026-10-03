//go:build linux

package stock

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/route/client"
	"github.com/dianabuilds/ardents-network/internal/route/credential"
)

type blockedIssuerExchange struct {
	entered chan struct{}
	release chan struct{}
	calls   atomic.Int32
}

func (*blockedIssuerExchange) Loaded() bool { return true }
func (exchange *blockedIssuerExchange) ExchangeIssuer(ctx context.Context, _ client.ClosedTokenPresenter, _ []byte) (client.ClosedIssuanceExchangeResult, error) {
	exchange.calls.Add(1)
	close(exchange.entered)
	select {
	case <-exchange.release:
		return client.ClosedIssuanceExchangeResult{}, errors.New("explicit interrupted transport fixture")
	case <-ctx.Done():
		return client.ClosedIssuanceExchangeResult{}, ctx.Err()
	}
}

func TestOperationCopiesPermitExactlyOneExchange(t *testing.T) {
	owner, host, hello := issuedStockFixture(t)
	permission := owner.permission
	challenge := credential.ClosedTokenContext{NetworkID: host.profile.NetworkID, ProfileDigest: host.profile.Digest,
		IssuerNodeID: host.profile.IssuerNodeID, ReceiverNodeID: hello.RecipientNodeID,
		ReceiverDutyGeneration: hello.RecipientDutyGeneration, Class: 2, WindowStart: permission.Grant().NotBefore}
	prepared, err := permission.reserveBatch(host.profile, host.now, []credential.ClosedTokenContext{challenge},
		client.ClosedBootstrapSelection{}, false, nil, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	exchange := &blockedIssuerExchange{entered: make(chan struct{}), release: make(chan struct{})}
	defer close(exchange.release)
	// Real blind batch and debit, with a controlled transport boundary. Endpoint
	// scenarios separately exercise the TCP/TLS and QUIC transports.
	prepared.Prefix = exchange
	private := newOperation(owner, permission, host.profile, prepared, false)
	owner.issuance = private
	handle := Operation{value: private}
	saved := handle
	handle = Operation{}
	handle.Cancel()
	handle.Join()
	if owner.issuance != saved.value {
		t.Fatal("overwriting handle replaced owner operation")
	}
	reserved, batches := permission.reserved, permission.batches
	result := make(chan error, 1)
	go func() {
		result <- saved.Run(t.Context(), nil, client.ClosedBootstrapSelection{})
	}()
	<-exchange.entered
	if err := saved.Run(t.Context(), nil, client.ClosedBootstrapSelection{}); err == nil {
		t.Fatal("concurrent run accepted")
	}
	if private.context.Err() != nil {
		t.Fatal("duplicate run canceled admitted exchange")
	}
	owner.mu.Lock()
	unchanged := owner.issuance == private && permission.reserved == reserved && permission.batches == batches
	owner.mu.Unlock()
	if !unchanged {
		t.Fatal("duplicate run changed slot or debit")
	}
	saved.Cancel()
	if err := <-result; err == nil {
		t.Fatal("canceled exchange succeeded")
	}
	saved.Join()
	if err := saved.Run(t.Context(), nil, client.ClosedBootstrapSelection{}); err == nil {
		t.Fatal("terminal operation restarted")
	}
	owner.mu.Lock()
	defer owner.mu.Unlock()
	if exchange.calls.Load() != 1 || owner.BusyLocked() || permission.reserved != reserved || permission.batches != batches || !permission.HasPending() {
		t.Fatal("operation duplicated exchange, refunded debit, or lost exact retry")
	}
}
