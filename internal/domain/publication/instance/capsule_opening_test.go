package instance

import (
	"bytes"
	"context"
	"encoding/hex"
	"testing"
)

// RFC9180 base-mode X25519/HKDF-SHA256/AES-128-GCM vector, sequence zero.
// Static independently published input: pinned CIRCL v1.6.5 testdata
// vectors_rfc9180_5f503c5.json.gz, matching Go's rfc9180.json key/enc/info.
// This proves the selected suite mechanism only, never Instance authority.
func TestCapsuleOpeningRFC9180SelectedSuite(t *testing.T) {
	decode := func(value string) []byte {
		t.Helper()
		raw, err := hex.DecodeString(value)
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	private := decode("4612c550263fc8ad58375df3f557aac531d26850903e55a9f23f21d8534e8ac8")
	enc := decode("37fda3567bdbd628e88668c3c8d7e97d1d1253b6d4ea6d44c150f741f1bf4431")
	info := decode("4f6465206f6e2061204772656369616e2055726e")
	aad := decode("436f756e742d30")
	cipher := decode("f938558b5d72f1a23810b4be2ab4f84331acc02fc97babc53a52ae8218a355a96d8770ac83d07bea87e13c512a")
	expected := decode("4265617574792069732074727574682c20747275746820626561757479")
	got, err := openHPKE(private, enc, info, aad, cipher)
	if err != nil || !bytes.Equal(got, expected) {
		t.Fatal("selected suite differs from independent RFC vector", err)
	}
	clear(got)
	for _, item := range []struct {
		name string
		raw  []byte
	}{{"private", private}, {"encapsulation", enc}, {"info", info}, {"aad", aad}, {"ciphertext", cipher}} {
		t.Run(item.name, func(t *testing.T) {
			item.raw[0] ^= 8
			defer func() { item.raw[0] ^= 8 }()
			plain, err := openHPKE(private, enc, info, aad, cipher)
			defer clear(plain)
			if err == nil {
				t.Fatal("altered RFC vector authenticated")
			}
		})
	}
}

func TestDetachedOpeningCannotSupplyCandidateOrAuthority(t *testing.T) {
	for _, opening := range []*Opening{nil, {}} {
		if _, _, err := opening.Request(context.Background()); err == nil {
			t.Fatal("detached opening returned candidate")
		}
		opening.Close()
	}
}
