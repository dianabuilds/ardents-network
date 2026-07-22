package migration

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"go.etcd.io/bbolt"
)

const InventorySchemaVersion uint32 = 1

type InventoryReport struct {
	SchemaVersion uint32                `json:"schema_version"`
	Node          NodeIdentityInventory `json:"node"`
	Occurrences   []Occurrence          `json:"occurrences"`
}

type NodeIdentityInventory struct {
	LegacyPrincipal         string `json:"legacy_principal"`
	PrincipalV1             string `json:"principal_v1"`
	LegacyDeviceDisposition string `json:"legacy_device_disposition"`
}

type Occurrence struct {
	Location       string `json:"location"`
	Classification string `json:"classification"`
	LegacyID       string `json:"legacy_id,omitempty"`
	PrincipalV1    string `json:"principal_v1,omitempty"`
}

type nodeState struct {
	Identity nodeIdentity `json:"identity"`
}
type nodeIdentity struct {
	Principal string `json:"principal"`
	Device    string `json:"device"`
	PublicKey string `json:"public_key"`
}
type keyLedger struct {
	PrivateKey string `json:"private_key"`
}

func InventoryNodeIdentity(dataDir string) (InventoryReport, error) {
	if dataDir == "" {
		return InventoryReport{}, fmt.Errorf("identity inventory data directory is required")
	}
	state, err := readNodeState(filepath.Join(dataDir, "ardents.db"))
	if err != nil {
		return InventoryReport{}, err
	}
	publicRaw, err := base64.StdEncoding.Strict().DecodeString(state.Identity.PublicKey)
	if err != nil || len(publicRaw) != ed25519.PublicKeySize {
		return InventoryReport{}, fmt.Errorf("node identity public key is invalid")
	}
	public := ed25519.PublicKey(publicRaw)
	legacy, err := ParseLegacyPrincipalID(state.Identity.Principal)
	if err != nil {
		return InventoryReport{}, fmt.Errorf("node identity Principal is invalid")
	}
	v1, err := legacy.MapToV1(public)
	if err != nil {
		return InventoryReport{}, fmt.Errorf("node identity Principal does not match its public key")
	}
	private, err := readNodePrivateKey(filepath.Join(dataDir, "identity_key.json"))
	if err != nil {
		return InventoryReport{}, err
	}
	if !bytes.Equal(private.Public().(ed25519.PublicKey), public) {
		return InventoryReport{}, fmt.Errorf("node identity keyring does not match persisted public key")
	}
	if state.Identity.Device != legacyDeviceFromSeed(private.Seed()) {
		return InventoryReport{}, fmt.Errorf("legacy Device does not match the protected node key")
	}
	return InventoryReport{
		SchemaVersion: InventorySchemaVersion,
		Node:          NodeIdentityInventory{LegacyPrincipal: legacy.String(), PrincipalV1: v1.String(), LegacyDeviceDisposition: "retain_legacy_projection_until_pia015a"},
		Occurrences: []Occurrence{
			{Location: "ardents.db/node-runtime/state/identity/principal", Classification: "rewrite_in_place_metadata", LegacyID: legacy.String(), PrincipalV1: v1.String()},
			{Location: "ardents.db/node-runtime/state/identity/device", Classification: "retain_legacy_projection_until_pia015a", LegacyID: state.Identity.Device},
		},
	}, nil
}

func readNodeState(path string) (nodeState, error) {
	if _, err := os.Stat(path); err != nil {
		return nodeState{}, fmt.Errorf("node identity state is unavailable")
	}
	db, err := bbolt.Open(path, 0o600, &bbolt.Options{ReadOnly: true})
	if err != nil {
		return nodeState{}, fmt.Errorf("node identity state is unavailable")
	}
	defer db.Close()
	var raw []byte
	err = db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte("node-runtime"))
		if bucket == nil {
			return errors.New("bucket missing")
		}
		value := bucket.Get([]byte("state"))
		if value == nil {
			return errors.New("state missing")
		}
		raw = append([]byte(nil), value...)
		return nil
	})
	if err != nil {
		return nodeState{}, fmt.Errorf("node identity state is unavailable")
	}
	var state nodeState
	if err := decodeStrict(raw, &state); err != nil {
		return nodeState{}, fmt.Errorf("node identity state has an unknown or invalid schema")
	}
	if state.Identity.Principal == "" || state.Identity.Device == "" || state.Identity.PublicKey == "" {
		return nodeState{}, fmt.Errorf("node identity state is incomplete")
	}
	return state, nil
}

func readNodePrivateKey(path string) (ed25519.PrivateKey, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("node identity keyring is unavailable")
	}
	var ledger keyLedger
	if err := decodeStrict(raw, &ledger); err != nil {
		return nil, fmt.Errorf("node identity keyring has an unknown or invalid schema")
	}
	decoded, err := base64.StdEncoding.Strict().DecodeString(ledger.PrivateKey)
	if err != nil || len(decoded) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("node identity keyring is invalid")
	}
	private := ed25519.PrivateKey(decoded)
	seed := private.Seed()
	rebuilt := ed25519.NewKeyFromSeed(seed)
	if !bytes.Equal(private, rebuilt) {
		return nil, fmt.Errorf("node identity keyring is invalid")
	}
	return private, nil
}

func decodeStrict(raw []byte, out any) error {
	if err := rejectDuplicateJSON(raw); err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(out); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("trailing JSON value")
	}
	return nil
}

func rejectDuplicateJSON(raw []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := scanJSON(decoder, "$"); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("multiple JSON values")
		}
		return err
	}
	return nil
}
func scanJSON(decoder *json.Decoder, path string) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delimiter, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delimiter {
	case '{':
		seen := map[string]bool{}
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return err
			}
			key, ok := keyToken.(string)
			if !ok {
				return errors.New("invalid JSON key")
			}
			if seen[key] {
				return fmt.Errorf("duplicate JSON field at %s.%s", path, key)
			}
			seen[key] = true
			if err := scanJSON(decoder, path+"."+key); err != nil {
				return err
			}
		}
		_, err = decoder.Token()
		return err
	case '[':
		for index := 0; decoder.More(); index++ {
			if err := scanJSON(decoder, fmt.Sprintf("%s[%d]", path, index)); err != nil {
				return err
			}
		}
		_, err = decoder.Token()
		return err
	default:
		return errors.New("invalid JSON delimiter")
	}
}
func legacyDeviceFromSeed(seed []byte) string {
	sum := sha256.Sum256(seed)
	return "d_" + hex.EncodeToString(sum[:8])
}
