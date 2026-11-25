//go:build linux || darwin

package node_test

import (
	"encoding/hex"
	"github.com/dianabuilds/ardents-network/internal/admission"
	"strconv"
	"time"
)

func admissionFactsConfig(f admission.Facts) map[string]string {
	return map[string]string{"network": hex.EncodeToString(f.Network[:]), "issuer": hex.EncodeToString(f.Issuer[:]), "authority": hex.EncodeToString(f.Authority[:]), "holder": hex.EncodeToString(f.Holder[:]), "duty": strconv.FormatUint(f.Duty, 10), "duty_not_before": f.DutyNotBefore.Format(time.RFC3339), "duty_not_after": f.DutyNotAfter.Format(time.RFC3339), "now": f.Now.Format(time.RFC3339), "class": strconv.Itoa(int(f.Class)), "count": strconv.Itoa(int(f.Count))}
}
