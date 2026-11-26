//go:build linux && document_worker_installed

package processfixture

import (
	"bytes"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/admission"
	"github.com/dianabuilds/ardents-network/internal/admission/receiving"
	"github.com/dianabuilds/ardents-network/internal/domain/hosting"
)

// ObserveInstalledReceivingReopen captures an admitted token and verifies the reopened Receiving and Hosting roots.
func ObserveInstalledReceivingReopen(t testing.TB, root string, binding receiving.Receiver, observe func() (receiving.Observation, error),
	original *receiving.Owner, budgetRoot string, budget *hosting.Budget, reserve func(*hosting.Budget, time.Time) (func() error, error)) (func(admission.Class, []byte, time.Time), func() int) {
	t.Helper()
	var mu sync.Mutex
	var token []byte
	var class admission.Class
	var deadline time.Time
	record := func(acceptedClass admission.Class, raw []byte, acceptedDeadline time.Time) {
		mu.Lock()
		defer mu.Unlock()
		if token == nil {
			token, class, deadline = bytes.Clone(raw), acceptedClass, acceptedDeadline
		}
	}
	t.Cleanup(func() {
		mu.Lock()
		defer mu.Unlock()
		clear(token)
	})
	verify := func() int {
		before, err := budget.Observe(t.Context())
		if err != nil || before.ReservedBytes != 0 {
			t.Fatal("original physical reservations not joined", before, err)
		}
		if err := errors.Join(original.Close(), budget.Close()); err != nil {
			t.Fatal(err)
		}
		reopenedBudget, err := hosting.Open(budgetRoot)
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			if err := reopenedBudget.Close(); err != nil {
				t.Error(err)
			}
		}()
		after, err := reopenedBudget.Observe(t.Context())
		if err != nil || after.ReservedBytes != 0 || after.UsedBytes < before.UsedBytes {
			t.Fatal("Hosting reopen reset original accounting", before, after, err)
		}
		reopened, err := receiving.Open(root, binding, observe)
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			if err := reopened.Close(); err != nil {
				t.Error(err)
			}
		}()
		if token == nil {
			return 0
		}
		observation, err := observe()
		if err != nil {
			t.Fatal(err)
		}
		if _, err := receiving.VerifyToken(observation.Profile, binding, class, token, observation.Now); err != nil || !observation.Now.Before(deadline) {
			t.Fatal("replay oracle lost original valid token or deadline", err)
		}
		reserved, returned := 0, 0
		grant, err := reopened.Accept(t.Context(), class, token, deadline, func() (func() error, error) {
			release, err := reserve(reopenedBudget, deadline)
			if err != nil {
				return nil, err
			}
			reserved++
			return func() error { returned++; return release() }, nil
		})
		if err == nil {
			t.Fatalf("reopened receiver refunded an actually spent token, release: %v", grant.Release())
		}
		if !strings.Contains(err.Error(), "closed token is already spent") || reserved != 1 || returned != 1 {
			t.Fatal("replay failed outside durable spend or leaked reservation", reserved, returned, err)
		}
		rolledBack, err := reopenedBudget.Observe(t.Context())
		if err != nil || rolledBack.ReservedBytes != 0 || rolledBack.UsedBytes < after.UsedBytes {
			t.Fatal("durable spend refusal did not return original physical capacity", rolledBack, err)
		}
		return 1
	}
	return record, verify
}
