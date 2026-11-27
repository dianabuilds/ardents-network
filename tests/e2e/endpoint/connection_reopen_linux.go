//go:build linux && document_worker_installed

package endpoint

import (
	"bytes"
	"errors"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/domain/network"
	"github.com/dianabuilds/ardents-network/internal/domain/publication/durable"
	"github.com/dianabuilds/ardents-network/internal/domain/publication/instance"
	"github.com/dianabuilds/ardents-network/internal/domain/reachability"
	descriptorstore "github.com/dianabuilds/ardents-network/internal/domain/reachability/store"
)

// VerifyInstalledConnectionReopen checks the original durable roots after joined retirement.
func VerifyInstalledConnectionReopen(t *testing.T, current func() (network.RuntimeView, error), reopen func(), afterJoin []func() int,
	store *descriptorstore.Store, storeRoot, publicationRoot, instanceRoot string, networkID, profileDigest, target [32]byte) {
	t.Helper()
	before, err := current()
	if err != nil {
		t.Fatal(err)
	}
	reopen()
	after, err := current()
	if err != nil || before.Profile().ProfileBinding != after.Profile().ProfileBinding {
		t.Fatal("Network reopen changed accepted binding", err)
	}
	checked := 0
	for _, verify := range afterJoin {
		checked += verify()
	}
	if checked < 4 {
		t.Fatal("insufficient actual receiving spends to check", checked)
	}
	raw, status, err := store.Lookup(target, profileDigest, time.Now())
	if err != nil || status != reachability.Accepted {
		t.Fatal("original stored Descriptor unavailable", status, err)
	}
	proof, err := reachability.Verify(raw, target, networkID, profileDigest, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := descriptorstore.Open(descriptorstore.Config{Root: storeRoot, Network: networkID})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := reopened.Close(); err != nil {
			t.Error(err)
		}
	}()
	retained, status, err := reopened.Lookup(target, profileDigest, time.Now())
	if err != nil || status != reachability.Accepted || !bytes.Equal(raw, retained) {
		t.Fatal("Store reopen lost exact accepted Descriptor", status, err)
	}
	history, err := durable.Open(t.Context(), durable.Config{Root: publicationRoot, Target: target, Network: networkID})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := history.Close(); err != nil {
			t.Error(err)
		}
	}()
	floor, err := history.Floor(t.Context())
	if err != nil || floor != proof.Publication.Delegation().Generation {
		t.Fatal("Publication reopen reset consumed generation", floor, err)
	}
	consumed, err := instance.Open(t.Context(), instanceRoot)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := consumed.Close(); err != nil {
			t.Error(err)
		}
	}()
	if _, err := consumed.Credential(t.Context()); !errors.Is(err, instance.ErrSuccessorRequired) {
		t.Fatal("reopen revived consumed Instance", err)
	}
	t.Log("original Network, receiving spends, Hosting, Descriptor and Publication retained across closed-owner reopen; consumed Instance unavailable", checked)
}
