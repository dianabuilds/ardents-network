package transfer

import (
	"testing"

	model "ardents/internal/content/catalog"
	"ardents/internal/content/payload"
)

func transferTestContentReference(t *testing.T, label string) model.ContentReference {
	t.Helper()
	_, reference, err := payload.DeriveIdentity([]byte(label))
	if err != nil {
		t.Fatal(err)
	}
	return reference
}
