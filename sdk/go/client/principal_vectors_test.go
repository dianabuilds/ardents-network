package client_test

import (
	"crypto/sha256"
	"encoding/base32"
	"encoding/hex"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPrincipalIdentifierVectorsAreConsumableWithoutInternalImports(t *testing.T) {
	raw, err := os.ReadFile("../../../api/ardents/identity/v1/testdata/principal-id-vectors.json")
	require.NoError(t, err)
	var file struct {
		Vectors []struct {
			PublicKeyHex string `json:"public_key_hex"`
			PrincipalID  string `json:"principal_id"`
			DeviceID     string `json:"device_id"`
		} `json:"vectors"`
	}
	require.NoError(t, json.Unmarshal(raw, &file))
	for _, vector := range file.Vectors {
		key, err := hex.DecodeString(vector.PublicKeyHex)
		require.NoError(t, err)
		require.Equal(t, vector.PrincipalID, vectorID("p1_", "ardents:principal:v1\x00", key))
		require.Equal(t, vector.DeviceID, vectorID("d1_", "ardents:device:v1\x00", key))
	}
}

func vectorID(prefix, domain string, key []byte) string {
	payload := append(append([]byte(domain), byte(1)), key...)
	sum := sha256.Sum256(payload)
	return prefix + strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(sum[:]))
}
