//go:build linux

package stock

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/admission/attempts"
)

func TestInvalidIssuedStockNeverReachesJournal(t *testing.T) {
	owner, host, hello := issuedStockFixture(t)
	owner.permission.stock[0].Tokens[0][0] ^= 0xff
	token, err := owner.TakeTokenLocked(host.profile, host.now, hello, 2, t.Context())
	if len(token) != 0 || TransferFailureStage(err) != "verification" || owner.permission.StockCountForDuty(host.profile.Digest, hello.RecipientNodeID, hello.RecipientDutyGeneration, 2) != 0 {
		t.Fatalf("invalid stock escaped or survived: bytes=%d err=%v", len(token), err)
	}
	if host.journal != nil {
		t.Fatal("invalid stock opened durable journal")
	}
}

func TestCanceledPresentationRetainsExactBurnAfterReopen(t *testing.T) {
	owner, host, hello := issuedStockFixture(t)
	original := bytes.Clone(owner.permission.stock[0].Tokens[0])
	defer clear(original)
	canceled, cancel := context.WithCancel(t.Context())
	cancel()
	token, err := owner.TakeTokenLocked(host.profile, host.now, hello, 2, canceled)
	if len(token) != 0 || TransferFailureStage(err) != "owner" || owner.permission.StockCountForDuty(host.profile.Digest, hello.RecipientNodeID, hello.RecipientDutyGeneration, 2) != 0 {
		t.Fatalf("canceled stock escaped or survived: bytes=%d err=%v", len(token), err)
	}
	if err := host.journal.Close(); err != nil {
		t.Fatal(err)
	}
	host.journal = nil
	reopened, err := host.Journal()
	if err != nil {
		t.Fatal(err)
	}
	if err := reopened.Mark(original, attempts.Attempt{Profile: host.profile.Digest, Receiver: hello.RecipientNodeID,
		Duty: hello.RecipientDutyGeneration, Window: host.now.Truncate(time.Hour), Class: 2, Nonce: hello.ChannelNonce}); err == nil || err.Error() != "text token already potentially spent" {
		t.Fatalf("canceled token was not retained as spent: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(host.root, "attempts"))
	if err != nil || bytes.Contains(raw, original) {
		t.Fatal("receipt unavailable or contains reusable token bytes")
	}
}

func TestPermissionRetirementErasesRetainedSecrets(t *testing.T) {
	owner, _, _ := issuedStockFixture(t)
	holder, public := owner.permission.holder, owner.permission.public
	token := owner.permission.stock[0].Tokens[0]
	owner.StopLocked().Join()
	for name, secret := range map[string][]byte{"holder": holder, "public backing": public, "unspent stock": token} {
		if len(secret) == 0 || !bytes.Equal(secret, make([]byte, len(secret))) {
			t.Fatalf("retirement did not erase %s", name)
		}
	}
	if owner.PermissionLocked().Present() {
		t.Fatal("retirement retained permission")
	}
}
