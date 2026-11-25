package releasefixture

import (
	"bytes"
	"testing"
	"time"

	"github.com/theupdateframework/go-tuf/v2/metadata"
)

func TestAuthorityRetainsSignedIndependentRoot(t *testing.T) {
	authority := NewReleaseAuthority(t, time.Now().Add(time.Hour))
	original := authority.RootBytes()
	parsed := metadata.Root()
	if _, err := parsed.FromBytes(original); err != nil {
		t.Fatal("parse independently signed root", err)
	}
	if len(parsed.Signed.Keys) != 5 || len(parsed.Signatures) != 3 || parsed.Signed.Roles["root"].Threshold != 3 {
		t.Fatal("fixture root does not retain its five keys and signing quorum")
	}
	changed := authority.RootBytes()
	changed[0] ^= 1
	if !bytes.Equal(authority.RootBytes(), original) {
		t.Fatal("exported root bytes changed retained authority")
	}
}
