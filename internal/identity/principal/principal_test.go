package principal

import (
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

type vectorFile struct {
	Version int `json:"version"`
	Vectors []struct {
		Name         string `json:"name"`
		PublicKeyHex string `json:"public_key_hex"`
		PrincipalID  string `json:"principal_id"`
		DeviceID     string `json:"device_id"`
	} `json:"vectors"`
}

func loadVectors(t *testing.T) vectorFile {
	t.Helper()
	_, source, _, ok := runtime.Caller(0)
	require.True(t, ok)
	path := filepath.Clean(filepath.Join(filepath.Dir(source), "..", "..", "..", "api", "ardents", "identity", "v1", "testdata", "principal-id-vectors.json"))
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	var vectors vectorFile
	require.NoError(t, json.Unmarshal(raw, &vectors))
	require.Equal(t, 1, vectors.Version)
	return vectors
}

func TestPublishedIdentifierVectors(t *testing.T) {
	for _, vector := range loadVectors(t).Vectors {
		t.Run(vector.Name, func(t *testing.T) {
			key, err := hex.DecodeString(vector.PublicKeyHex)
			require.NoError(t, err)
			principalID, err := FromEd25519PublicKey(ed25519.PublicKey(key))
			require.NoError(t, err)
			deviceID, err := DeviceFromEd25519PublicKey(ed25519.PublicKey(key))
			require.NoError(t, err)
			require.Equal(t, vector.PrincipalID, principalID.String())
			require.Equal(t, vector.DeviceID, deviceID.String())
			parsed, err := Parse(vector.PrincipalID)
			require.NoError(t, err)
			require.True(t, principalID.Equal(parsed))
			parsedDevice, err := ParseDeviceID(vector.DeviceID)
			require.NoError(t, err)
			require.True(t, deviceID.Equal(parsedDevice))
		})
	}
}

func TestIdentifierParsingRejectsNonCanonicalValues(t *testing.T) {
	valid := loadVectors(t).Vectors[0]
	for _, value := range []string{"", "p_1234567890abcdef", "p2_" + valid.PrincipalID[3:], strings.ToUpper(valid.PrincipalID), valid.PrincipalID + "=", " " + valid.PrincipalID, valid.PrincipalID[:len(valid.PrincipalID)-1]} {
		_, err := Parse(value)
		require.Error(t, err, value)
	}
	for _, value := range []string{"", "d2_" + valid.DeviceID[3:], strings.ToUpper(valid.DeviceID), valid.DeviceID + "=", valid.DeviceID[:len(valid.DeviceID)-1]} {
		_, err := ParseDeviceID(value)
		require.Error(t, err, value)
	}
}

func TestIdentifierTextRoundTripAndZeroRejection(t *testing.T) {
	id, err := Parse(loadVectors(t).Vectors[1].PrincipalID)
	require.NoError(t, err)
	text, err := id.MarshalText()
	require.NoError(t, err)
	var decoded ID
	require.NoError(t, decoded.UnmarshalText(text))
	require.True(t, id.Equal(decoded))
	var zero ID
	_, err = zero.MarshalText()
	require.Error(t, err)
	var zeroDevice DeviceID
	_, err = zeroDevice.MarshalText()
	require.Error(t, err)
	_, err = FromEd25519PublicKey(make([]byte, ed25519.PublicKeySize-1))
	require.Error(t, err)
	_, err = DeviceFromEd25519PublicKey(make([]byte, ed25519.PublicKeySize+1))
	require.Error(t, err)
}

func FuzzPrincipalIDCanonicalRoundTrip(f *testing.F) {
	f.Add("p1_755gnz2wffu3osamddsj7ggiasqtwnwomsooe5mxh2yipr2urmwq")
	f.Fuzz(func(t *testing.T, value string) {
		id, err := Parse(value)
		if err != nil {
			return
		}
		require.Equal(t, value, id.String())
	})
}

func TestOneByteTextMutationsNeverAlias(t *testing.T) {
	original, err := Parse(loadVectors(t).Vectors[2].PrincipalID)
	require.NoError(t, err)
	text := original.String()
	for i := range text {
		mutated := []byte(text)
		if mutated[i] == 'a' {
			mutated[i] = 'b'
		} else {
			mutated[i] = 'a'
		}
		parsed, parseErr := Parse(string(mutated))
		if parseErr == nil {
			require.False(t, original.Equal(parsed), "mutation at byte %d aliased", i)
		}
	}
}
