//go:build linux && descriptor_installed_native

package descriptor_test

import (
	"bytes"
	"crypto/tls"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
)

// One original holder survives server retirement without lending its client
// State, Stock or keys to the server. Each physical operation still consumes a
// fresh genuine Control right and joins before the original peer stops.
func runDescriptorNativeRestartExchange(t *testing.T, p descriptorPreparedStock, certificate tls.Certificate, proof descriptorNativeProofs, replacement bool) {
	t.Helper()
	perform := func(name string, r ardp.DescriptorRequest, want uint8, expected []byte) {
		t.Helper()
		before := descriptorNativeRead(t, "runtime/spend/closed-token-spends")
		status, raw := descriptorNativeOperation(t, p, certificate, r)
		if status != want || !bytes.Equal(raw, expected) {
			t.Fatal(name, "exact result differs", status, want)
		}
		after := descriptorNativeRead(t, "runtime/spend/closed-token-spends")
		if len(after) != len(before)+41 || !bytes.Equal(before, after[:len(before)]) || after[len(after)-1] != 1 {
			t.Fatal(name, "retained irreversible spend history changed")
		}
		t.Log(name, "actual operation and physical join verified")
	}
	perform("original revision2 Store ACK", ardp.DescriptorRequest{Operation: ardp.DescriptorPublish, Proof: proof.second}, 0, nil)
	perform("original exact lookup", ardp.DescriptorRequest{Operation: ardp.DescriptorLookup, Target: proof.target}, 0, proof.second)
	descriptorNativeWrite(t, "exchange-phase-one", []byte("original operations joined\n"), 0600)
	if replacement {
		return
	}
	timer := time.NewTimer(time.Minute)
	defer timer.Stop()
	tick := time.NewTicker(20 * time.Millisecond)
	defer tick.Stop()
	for {
		raw, err := os.ReadFile(filepath.Join(descriptorNativeParent, "exchange-restarted"))
		if err == nil {
			if string(raw) != "original retained Descriptor ready\n" {
				t.Fatal("foreign restart marker")
			}
			break
		}
		if !os.IsNotExist(err) {
			t.Fatal(err)
		}
		select {
		case <-t.Context().Done():
			t.Fatal(t.Context().Err())
		case <-timer.C:
			t.Fatal("retained restart unavailable")
		case <-tick.C:
		}
	}
	perform("retained exact revision2 lookup", ardp.DescriptorRequest{Operation: ardp.DescriptorLookup, Target: proof.target}, 0, proof.second)
	perform("retained lower revision refusal", ardp.DescriptorRequest{Operation: ardp.DescriptorPublish, Proof: proof.first}, 3, nil)
	perform("retained refusal preserved exact revision2", ardp.DescriptorRequest{Operation: ardp.DescriptorLookup, Target: proof.target}, 0, proof.second)
}
