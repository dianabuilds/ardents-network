package access

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	identitycontract "ardents/api/ardents/identity/v1"
	"ardents/internal/storage"

	"github.com/stretchr/testify/require"
)

func consumeBootstrapForTest(ctx context.Context, database storage.Database, node string, ticket BootstrapTicket, now time.Time) error {
	return database.Update(ctx, func(tx storage.WriteTransaction) error {
		return consumeBootstrapTicket(tx, node, ticket, canonicalNow(now))
	})
}

func TestBootstrapTicketsAreDisabledByDefault(t *testing.T) {
	f := newServiceFixture(t)
	_, err := f.service.IssueBootstrapTicket(f.ctx, f.nodeID)
	require.ErrorIs(t, err, ErrFeatureDisabled)
}

func TestBootstrapTicketOneUseDigestOnlyAndRedacted(t *testing.T) {
	f := newServiceFixture(t)
	f.service.bootstrapEnabled = true
	ticket, err := f.service.IssueBootstrapTicket(f.ctx, f.nodeID)
	require.NoError(t, err)
	var stored []byte
	require.NoError(t, f.database.View(f.ctx, func(tx storage.ReadTransaction) error {
		stored, _, err = tx.Get(bootstrapTicketsBucket, []byte(f.nodeID))
		return err
	}))
	require.NotContains(t, stored, ticket[:])
	require.NotContains(t, fmt.Sprintf("%v %#v %s", ticket, ticket, ticket), string(ticket[:]))
	raw, err := json.Marshal(ticket)
	require.NoError(t, err)
	require.Equal(t, `"[redacted bootstrap ticket]"`, string(raw))

	wrong := ticket
	wrong[0] ^= 0xff
	require.ErrorIs(t, consumeBootstrapForTest(f.ctx, f.database, f.nodeID, wrong, f.clock.Now()), ErrUnauthenticated)
	require.NoError(t, consumeBootstrapForTest(f.ctx, f.database, f.nodeID, ticket, f.clock.Now()))
	require.ErrorIs(t, consumeBootstrapForTest(f.ctx, f.database, f.nodeID, ticket, f.clock.Now()), ErrUnauthenticated)
	_, err = f.service.IssueBootstrapTicket(f.ctx, f.nodeID)
	require.ErrorIs(t, err, ErrConflict)
}

func TestBootstrapTicketExpiryAndReissue(t *testing.T) {
	f := newServiceFixture(t)
	f.service.bootstrapEnabled = true
	first, err := f.service.IssueBootstrapTicket(f.ctx, f.nodeID)
	require.NoError(t, err)
	f.clock.Advance(identitycontract.BootstrapTicketLifetime)
	require.ErrorIs(t, consumeBootstrapForTest(f.ctx, f.database, f.nodeID, first, f.clock.Now()), ErrUnauthenticated)
	second, err := f.service.IssueBootstrapTicket(f.ctx, f.nodeID)
	require.NoError(t, err)
	require.NotEqual(t, first, second)
	require.ErrorIs(t, consumeBootstrapForTest(f.ctx, f.database, f.nodeID, first, f.clock.Now()), ErrUnauthenticated)
	require.NoError(t, consumeBootstrapForTest(f.ctx, f.database, f.nodeID, second, f.clock.Now()))
}

func TestBootstrapTicketCannotBeIssuedAfterPrincipalEnrollment(t *testing.T) {
	f := newServiceFixture(t)
	f.service.bootstrapEnabled = true
	ticket, err := f.service.IssueBootstrapTicket(f.ctx, f.nodeID)
	require.NoError(t, err)
	var root [32]byte
	copy(root[:], f.root.Public().(ed25519.PublicKey))
	require.NoError(t, f.service.enrollments.record(f.ctx, EnrollmentRecord{Node: f.nodeID, Principal: f.principal, RootPublicKey: root, EnrolledAt: f.clock.Now()}))
	_, err = f.service.IssueBootstrapTicket(f.ctx, f.nodeID)
	require.ErrorIs(t, err, ErrConflict)
	require.ErrorIs(t, consumeBootstrapForTest(f.ctx, f.database, f.nodeID, ticket, f.clock.Now()), ErrConflict)
}

func TestBootstrapTicketConcurrentConsumeHasOneWinner(t *testing.T) {
	f := newServiceFixture(t)
	f.service.bootstrapEnabled = true
	ticket, err := f.service.IssueBootstrapTicket(f.ctx, f.nodeID)
	require.NoError(t, err)
	var successes atomic.Int32
	var failures atomic.Int32
	var wg sync.WaitGroup
	for range 32 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := consumeBootstrapForTest(context.Background(), f.database, f.nodeID, ticket, f.clock.Now())
			if err == nil {
				successes.Add(1)
			} else if errors.Is(err, ErrUnauthenticated) {
				failures.Add(1)
			}
		}()
	}
	wg.Wait()
	require.Equal(t, int32(1), successes.Load())
	require.Equal(t, int32(31), failures.Load())
}

func TestBootstrapTicketConsumeRollsBackWithEnclosingMutation(t *testing.T) {
	f := newServiceFixture(t)
	f.service.bootstrapEnabled = true
	ticket, err := f.service.IssueBootstrapTicket(f.ctx, f.nodeID)
	require.NoError(t, err)
	sentinel := errors.New("later enrollment write failed")
	err = f.database.Update(f.ctx, func(tx storage.WriteTransaction) error {
		require.NoError(t, consumeBootstrapTicket(tx, f.nodeID, ticket, f.clock.Now()))
		return sentinel
	})
	require.ErrorIs(t, err, sentinel)
	require.NoError(t, consumeBootstrapForTest(f.ctx, f.database, f.nodeID, ticket, f.clock.Now()))
}

func TestBootstrapTicketSurvivesServiceRestartAndCorruptionFailsClosed(t *testing.T) {
	f := newServiceFixture(t)
	f.service.bootstrapEnabled = true
	ticket, err := f.service.IssueBootstrapTicket(f.ctx, f.nodeID)
	require.NoError(t, err)
	restarted, err := NewService(Config{Database: f.database, Clock: f.clock, Entropy: &sequentialEntropy{next: 90}, EnableBootstrapTickets: true})
	require.NoError(t, err)
	require.NoError(t, consumeBootstrapForTest(f.ctx, restarted.grants.database, f.nodeID, ticket, f.clock.Now()))

	otherNode := f.principal
	require.NoError(t, f.database.Update(f.ctx, func(tx storage.WriteTransaction) error {
		return tx.Put(bootstrapTicketsBucket, []byte(otherNode), bytes.Repeat([]byte{0xff}, bootstrapTicketRecordBytes))
	}))
	_, err = restarted.IssueBootstrapTicket(f.ctx, otherNode)
	require.ErrorIs(t, err, ErrUnavailable)
}
