package inspection

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/platform/artifacts"
)

func TestInspectionRefusesRepeatedOriginalPath(t *testing.T) {
	plan := filepath.Join(t.TempDir(), "plan")
	if err := os.WriteFile(plan, []byte("bounded input"), 0600); err != nil {
		t.Fatal(err)
	}
	response, err := InspectLatest(t.Context(), Request{ServingPlan: plan, ClientCertificate: plan})
	if !errors.Is(err, artifacts.ErrInput) || response.Status != "" || response.Payload != nil {
		t.Fatalf("repeated input was not refused: status %q, error %v", response.Status, err)
	}
}
