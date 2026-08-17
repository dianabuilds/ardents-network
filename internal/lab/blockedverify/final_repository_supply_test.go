package blockedverify

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"
)

func TestFinalSupplyLockBindsVerifierSpec(t *testing.T) {
	spec := validFinalSpec()
	lock := finalVerifierSupplyLock{Schema: "ardents-h3-s5-supply-lock-v1",
		GoBuilderImageID: spec.GoBuilderImageID, GoBuilderVersion: spec.GoBuilderVersion,
		GoArchiveSHA256: "708effb774be8237570d0add163225abbdfaf4fca28b2611df167beba4feef89",
		GoRecipeSHA256:  spec.ProductReceipt.GoRecipeSHA256,
		GoModuleSHA256:  spec.ProductReceipt.GoModuleSHA256,
		ToolImageID:     spec.ToolImageID, ToolLockSHA256: spec.ToolReceipt.ToolLockSHA256,
		CarrierSHA256: spec.ToolReceipt.CarrierSHA256}
	raw, err := json.MarshalIndent(lock, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	raw = append(raw, '\n')
	digest := sha256.Sum256(raw)
	spec.SupplyLock.SHA256, spec.SupplyLock.Bytes = hex.EncodeToString(digest[:]), int64(len(raw))
	if err := verifyFinalSupplyLockBytes(raw, spec); err != nil {
		t.Fatal(err)
	}
	spec.ToolImageID = spec.ProductImageID
	if err := verifyFinalSupplyLockBytes(raw, spec); err == nil {
		t.Fatal("verifier accepted a supply identity outside the repository lock")
	}
}
