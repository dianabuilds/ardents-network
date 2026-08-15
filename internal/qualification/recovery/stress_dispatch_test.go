package recovery

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestVerifyDispatchesStage43AttemptAndCampaign(t *testing.T) {
	manifest, err := json.Marshal(stressAttemptManifest{Schema: stressAttemptManifestSchema})
	if err != nil {
		t.Fatal(err)
	}
	attempt := Verify(Evidence{Schema: replacementAttemptEnvelopeSchema,
		AttemptManifest: manifest, AttemptReceipt: json.RawMessage(`{}`)})
	if attempt.Verdict != "invalid" || !strings.Contains(attempt.Reason, "S4.3 attempt identity") {
		t.Fatalf("unexpected S4.3 attempt dispatch: %+v", attempt)
	}
	campaign := Verify(Evidence{Schema: replacementCampaignEnvelopeSchema,
		AttemptManifest: manifest, AttemptCampaign: json.RawMessage(`{}`)})
	if campaign.Verdict != "invalid" || !strings.Contains(campaign.Reason, "S4.3 campaign identity") {
		t.Fatalf("unexpected S4.3 campaign dispatch: %+v", campaign)
	}
}
