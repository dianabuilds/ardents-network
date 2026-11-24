//go:build linux && node_process_native

package node

import (
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/admission/issuance"
	"github.com/dianabuilds/ardents-network/internal/admission/issuer"
	"github.com/dianabuilds/ardents-network/internal/admission/quota"
)

// RequireIssuerRootsReleased reopens the original quota, keys and result store.
func RequireIssuerRootsReleased(t testing.TB, p issuer.Plan) {
	t.Helper()
	ledger, err := quota.Open(p.AdmissionRoot, p.AdmissionBinding)
	if err != nil {
		t.Fatal("joined quota lease unavailable", err)
	}
	defer func() {
		if err := ledger.Close(); err != nil {
			t.Error("close reopened quota lease", err)
		}
	}()
	keys, err := issuance.Open(t.Context(), p.KeyRoot, p.KeyBinding)
	if err != nil {
		t.Fatal("joined original key custody unavailable", err)
	}
	defer func() {
		if err := keys.Close(); err != nil {
			t.Error("close reopened key custody", err)
		}
	}()
	results, err := issuance.OpenResults(t.Context(), p.ResultRoot, keys, p.AdmissionBinding)
	if err != nil {
		t.Fatal("joined original result lease unavailable", err)
	}
	if err := results.Close(); err != nil {
		t.Fatal(err)
	}
}

// IssuerDeclaration encodes the public CLI plan for the independently prepared issuer.
func IssuerDeclaration(p issuer.Plan) map[string]any {
	b := p.AdmissionBinding
	keys := make([]map[string]string, 0, len(b.Keys))
	for _, key := range b.Keys {
		keys = append(keys, map[string]string{
			"window": time.Unix(int64(key.Window), 0).UTC().Format(time.RFC3339),
			"class":  strconv.Itoa(int(key.Class)), "spki": fmt.Sprintf("%x", key.SPKI),
		})
	}
	k := p.KeyBinding
	return map[string]any{
		"admission_root": p.AdmissionRoot, "key_root": p.KeyRoot, "result_root": p.ResultRoot,
		"admission_binding": map[string]any{
			"network": fmt.Sprintf("%x", b.Network), "issuer": fmt.Sprintf("%x", b.Issuer),
			"authority": fmt.Sprintf("%x", b.Authority), "profile": fmt.Sprintf("%x", b.Profile),
			"duty": strconv.FormatUint(b.Duty, 10), "start": b.Start.Format(time.RFC3339),
			"end": b.End.Format(time.RFC3339), "keys": keys,
		},
		"key_binding": map[string]string{
			"network": fmt.Sprintf("%x", k.Network), "issuer": fmt.Sprintf("%x", k.Issuer),
			"signer": fmt.Sprintf("%x", k.Signer), "start": k.Start.Format(time.RFC3339), "end": k.End.Format(time.RFC3339),
		},
	}
}
