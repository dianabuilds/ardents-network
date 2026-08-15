package recoverysmoke

import "testing"

func TestSourceCommitFromManifestAcceptsFullCampaignManifest(t *testing.T) {
	t.Parallel()

	manifest := []byte(`{
		"Schema":"ardents-h3-campaign-manifest-v1",
		"SourceCommit":"ca1c5c34bed7efe3064aad1aa1653f3f69117caf",
		"Cells":[{"ID":"c2p-isolated-initiator"}]
	}`)

	got, err := sourceCommitFromManifest(manifest)
	if err != nil {
		t.Fatalf("sourceCommitFromManifest() error = %v", err)
	}
	if want := "ca1c5c34bed7efe3064aad1aa1653f3f69117caf"; got != want {
		t.Fatalf("sourceCommitFromManifest() = %q, want %q", got, want)
	}
}

func TestSourceCommitFromManifestRejectsMultipleValues(t *testing.T) {
	t.Parallel()

	if _, err := sourceCommitFromManifest([]byte(`{"SourceCommit":"first"} {"SourceCommit":"second"}`)); err == nil {
		t.Fatal("sourceCommitFromManifest() error = nil, want malformed manifest error")
	}
}
