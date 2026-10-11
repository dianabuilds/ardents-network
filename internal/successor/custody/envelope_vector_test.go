package custody

import (
	"bytes"
	"encoding/hex"
	"strings"
	"testing"
)

// Generated outside the Custody owner using the exact reviewed Argon2 package,
// standard NewGCM, a fixed salt/nonce and a manually specified AAD transcript.
// The production random-nonce sealer and canonical marshaler are not its oracle.
const independentEnvelope = `{"profile":"ardents-authority-envelope-v1","schema_version":1,"purpose":"authority-vault","kdf":{"name":"argon2id","version":19,"memory_kib":262144,"passes":3,"lanes":4,"salt":"AAECAwQFBgcICQoLDA0ODw"},"aead":"aes-256-gcm-random-nonce","ciphertext":"EBESExQVFhcYGRobnTxpPcF6rb_2yFI8CTb_OWlsyhS8qKO_pYCn0Kz9cZ7-eWxFlNe9lQ"}`

func TestIndependentEnvelopeVectorAndFixedResourceProfile(t *testing.T) {
	password := []byte("independent envelope password")
	purpose, plaintext, _, err := openEnvelope([]byte(independentEnvelope), password)
	if err != nil || purpose != PurposeVault || !bytes.Equal(plaintext, []byte(`{"independent":"vector"}`)) {
		t.Fatalf("independent AES-GCM vector: %v", err)
	}
	defer zero(plaintext)
	salt := []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15}
	key := deriveKey(password, salt)
	defer zero(key)
	if hex.EncodeToString(key) != "9b433f849ff0e8af0729a12cd86295dcb97ddee021dd3a3a413dc2682ee7198c" || kdfMemoryKiB != 256*1024 || kdfPasses != 3 || kdfLanes != 4 {
		t.Fatal("fixed resource derivation changed")
	}
	for _, raw := range []string{
		strings.Replace(independentEnvelope, `"memory_kib":262144`, `"memory_kib":1024`, 1),
		strings.Replace(independentEnvelope, `"profile":"ardents-authority-envelope-v1"`, `"profile":"foreign"`, 1),
		independentEnvelope + "\n",
		strings.Repeat("x", maximumEnvelopeBytes+1),
	} {
		if _, err := inspectEnvelope([]byte(raw)); err == nil {
			t.Fatal("unsupported/noncanonical envelope accepted")
		}
	}
	altered := strings.Replace(independentEnvelope, "EBESExQ", "FBESExQ", 1)
	if _, _, _, err := openEnvelope([]byte(altered), password); err == nil {
		t.Fatal("altered nonce authenticated")
	}
}
