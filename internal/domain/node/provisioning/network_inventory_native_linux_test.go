//go:build linux && network_signing_native

package provisioning

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/domain/node/provisioning/networkdraft"
	"github.com/dianabuilds/ardents-network/internal/domain/node/provisioning/networksigning"
	artifactio "github.com/dianabuilds/ardents-network/internal/platform/artifacts"
	provisionfixture "github.com/dianabuilds/ardents-network/tests/networkfixture"
)

func timedNetworkSigningArguments(t *testing.T, now, until time.Time) []string {
	t.Helper()
	parent := sourceArtifactNativeParent(t)
	p, key, _, _ := provisionfixture.RecordSigningPlan(t)
	p["valid_from"], p["valid_until"] = now.Add(-time.Minute).Format(time.RFC3339), until.Format(time.RFC3339)
	_, message, err := networkdraft.Prepare("prepare-initial-node-record", provisionfixture.PlanJSON(t, p))
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(message)
	plan, prepared, keyPath := filepath.Join(parent, "plan"), filepath.Join(parent, "prepared"), filepath.Join(parent, "key")
	if err = os.WriteFile(plan, provisionfixture.SigningDeclaration(t, "node-record", p, now), 0600); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(prepared, message, 0600); err != nil {
		t.Fatal(err)
	}
	encoded, err := artifactio.EncodeEd25519Key(key)
	clear(key)
	if err != nil {
		t.Fatal(err)
	}
	defer clear(encoded)
	if err := os.WriteFile(keyPath, encoded, 0600); err != nil {
		t.Fatal(err)
	}
	return []string{"sign-initial-node-record", plan, prepared, hex.EncodeToString(digest[:]), keyPath, filepath.Join(parent, "signed")}
}

func TestNetworkSigningOriginalTimeAfterIO(t *testing.T) {
	for _, stage := range []string{"inputs-read", "before-directory-sync"} {
		t.Run(stage, func(t *testing.T) {
			now := time.Now().UTC().Truncate(time.Second)
			until := now.Add(3 * time.Second)
			args := timedNetworkSigningArguments(t, now, until)
			observed := false
			_, err := networksigning.Sign(t.Context(), args, func(step string, _ *os.File) {
				if step != stage {
					return
				}
				observed = true
				// This waits for an actual clock boundary, not physical completion.
				// The original synchronous operation retains every input descriptor.
				if d := time.Until(until); d > 0 {
					time.Sleep(d + time.Millisecond)
				}
				if time.Now().Before(until) {
					t.Fatal("original clock prerequisite changed")
				}
			})
			if !observed || err == nil {
				t.Fatal("expired original time accepted after native IO", err)
			}
			_, statErr := os.Stat(args[5])
			if stage == "inputs-read" && !os.IsNotExist(statErr) {
				t.Fatal("expiry before signature created output", statErr)
			}
			if stage == "before-directory-sync" && statErr != nil {
				t.Fatal("uncertain output prefix removed", statErr)
			}
			healthy := time.Now().UTC().Truncate(time.Second)
			control := timedNetworkSigningArguments(t, healthy, healthy.Add(time.Hour))
			if _, e := networksigning.Sign(t.Context(), control, nil); e != nil {
				t.Fatal("independent healthy time control", e)
			}
		})
	}
}
