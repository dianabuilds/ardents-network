//go:build linux && node_process_native

package node

import (
	"fmt"
	"strconv"
	"time"

	"github.com/dianabuilds/ardents-network/internal/admission/issuer"
)

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
