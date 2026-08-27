//go:build ignore

// R-098 file reader is disposable evidence for a bounded alpha-control format.
package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

const (
	maximumCatalogBytes   = 4 * 1024
	maximumComponentBytes = 4 * 1024
)

type fileEnvelope struct {
	Body      string `json:"body_b64"`
	Signature string `json:"signature_b64"`
}

type fileComponent struct {
	Class     string `json:"class"`
	SignerID  string `json:"signer_id"`
	Version   uint64 `json:"version"`
	ExpiresAt int64  `json:"expires_at"`
	Payload   string `json:"payload"`
}

type fileCatalog struct {
	Cohort    string       `json:"cohort"`
	Revision  uint64       `json:"revision"`
	ExpiresAt int64        `json:"expires_at"`
	Entries   []descriptor `json:"entries"`
}

type fileReaderConfig struct {
	DisclosureKey ed25519.PublicKey
	ComponentKeys map[string]ed25519.PublicKey
	Floors        map[string]uint64
	CatalogFloor  uint64
	Now           time.Time
}

func exerciseBoundedFileReader() {
	directory, err := os.MkdirTemp("", "ardents-r098-")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(directory)

	disclosurePublic, disclosurePrivate := fileKeypair()
	releasePublic, releasePrivate := fileKeypair()
	networkPublic, networkPrivate := fileKeypair()
	config := fileReaderConfig{
		DisclosureKey: disclosurePublic,
		ComponentKeys: map[string]ed25519.PublicKey{
			"release-key": releasePublic,
			"network-key": networkPublic,
		},
		Floors:       map[string]uint64{"release": 3, "network-profile": 5},
		CatalogFloor: 4,
		Now:          time.Unix(1_893_456_000, 0),
	}

	files := map[string][]byte{
		"release.json": makeEnvelope(encode(fileComponent{
			Class: "release", SignerID: "release-key", Version: 3,
			ExpiresAt: config.Now.Add(time.Hour).Unix(), Payload: "release-v3",
		}), releasePrivate),
		"network-profile.json": makeEnvelope(encode(fileComponent{
			Class: "network-profile", SignerID: "network-key", Version: 5,
			ExpiresAt: config.Now.Add(time.Hour).Unix(), Payload: "network-v5",
		}), networkPrivate),
	}
	catalogValue := fileCatalog{
		Cohort: "alpha-1", Revision: 4, ExpiresAt: config.Now.Add(time.Hour).Unix(),
		Entries: []descriptor{
			{Class: "release", SignerID: "release-key", ComponentSHA256: digest(files["release.json"])},
			{Class: "network-profile", SignerID: "network-key", ComponentSHA256: digest(files["network-profile.json"])},
		},
	}
	files["catalog.json"] = makeEnvelope(encode(catalogValue), disclosurePrivate)
	if err := writeFixtureFiles(directory, files); err != nil {
		panic(err)
	}

	printFileReport("bounded_files", readCatalogDirectory(directory, config))
	if err := os.WriteFile(filepath.Join(directory, "catalog.json"), bytes.Repeat([]byte{'x'}, maximumCatalogBytes+1), 0o600); err != nil {
		panic(err)
	}
	printFileReport("oversized_catalog", readCatalogDirectory(directory, config))
}

func readCatalogDirectory(directory string, config fileReaderConfig) map[string]string {
	catalogRaw, err := readBoundedRegularFile(filepath.Join(directory, "catalog.json"), maximumCatalogBytes)
	if err != nil {
		return map[string]string{"catalog": fileReadStatus(err)}
	}
	catalogBody, err := verifyEnvelope(catalogRaw, config.DisclosureKey)
	if err != nil {
		return map[string]string{"catalog": "invalid"}
	}
	var current fileCatalog
	if err := decodeStrict(catalogBody, &current); err != nil || !validCatalog(current) {
		return map[string]string{"catalog": "invalid"}
	}
	if config.Now.Unix() >= current.ExpiresAt {
		return map[string]string{"catalog": "expired"}
	}
	if current.Revision < config.CatalogFloor {
		return map[string]string{"catalog": "lower-floor"}
	}

	report := make(map[string]string, len(current.Entries))
	seen := make(map[string]bool, len(current.Entries))
	for _, entry := range current.Entries {
		if seen[entry.Class] || !allowedFileClass(entry.Class) || entry.SignerID == "" || entry.ComponentSHA256 == "" {
			return map[string]string{"catalog": "invalid"}
		}
		seen[entry.Class] = true
		raw, err := readBoundedRegularFile(filepath.Join(directory, entry.Class+".json"), maximumComponentBytes)
		if err != nil {
			report[entry.Class] = fileReadStatus(err)
			continue
		}
		if digest(raw) != entry.ComponentSHA256 {
			report[entry.Class] = "digest-mismatch"
			continue
		}
		key, found := config.ComponentKeys[entry.SignerID]
		if !found {
			report[entry.Class] = "invalid-signature"
			continue
		}
		body, err := verifyEnvelope(raw, key)
		if err != nil {
			report[entry.Class] = "invalid-signature"
			continue
		}
		var item fileComponent
		if err := decodeStrict(body, &item); err != nil || item.Class != entry.Class || item.SignerID != entry.SignerID || !validComponent(item) {
			report[entry.Class] = "invalid"
			continue
		}
		if config.Now.Unix() >= item.ExpiresAt {
			report[entry.Class] = "expired"
			continue
		}
		if item.Version < config.Floors[item.Class] {
			report[entry.Class] = "lower-floor"
			continue
		}
		report[entry.Class] = "accepted"
	}
	if len(seen) != 2 || !seen["release"] || !seen["network-profile"] {
		return map[string]string{"catalog": "invalid"}
	}
	return report
}

func readBoundedRegularFile(path string, maximum int64) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, errUnexpectedFile
	}
	if info.Size() > maximum {
		return nil, errTooLarge
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	raw, err := io.ReadAll(io.LimitReader(file, maximum+1))
	if err != nil {
		return nil, err
	}
	if int64(len(raw)) > maximum {
		return nil, errTooLarge
	}
	return raw, nil
}

var (
	errTooLarge       = errors.New("too-large")
	errUnexpectedFile = errors.New("unexpected-file")
)

func fileReadStatus(err error) string {
	switch {
	case errors.Is(err, errTooLarge):
		return "too-large"
	case errors.Is(err, errUnexpectedFile):
		return "unexpected-file"
	case errors.Is(err, os.ErrNotExist):
		return "unavailable"
	default:
		return "unreadable"
	}
}

func verifyEnvelope(raw []byte, key ed25519.PublicKey) ([]byte, error) {
	var envelope fileEnvelope
	if err := decodeStrict(raw, &envelope); err != nil {
		return nil, err
	}
	body, err := base64.StdEncoding.DecodeString(envelope.Body)
	if err != nil {
		return nil, err
	}
	signature, err := base64.StdEncoding.DecodeString(envelope.Signature)
	if err != nil || !ed25519.Verify(key, body, signature) {
		return nil, errors.New("invalid signature")
	}
	return body, nil
}

func makeEnvelope(body []byte, key ed25519.PrivateKey) []byte {
	return encode(fileEnvelope{
		Body:      base64.StdEncoding.EncodeToString(body),
		Signature: base64.StdEncoding.EncodeToString(ed25519.Sign(key, body)),
	})
}

func decodeStrict(raw []byte, value any) error {
	if err := rejectDuplicateJSONKeys(raw); err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return errors.New("trailing JSON value")
	}
	return nil
}

func rejectDuplicateJSONKeys(raw []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	if err := scanJSONValue(decoder); err != nil {
		return err
	}
	if _, err := decoder.Token(); err != io.EOF {
		return errors.New("trailing JSON value")
	}
	return nil
}

func scanJSONValue(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delimiter, isDelimiter := token.(json.Delim)
	if !isDelimiter {
		return nil
	}
	switch delimiter {
	case '{':
		keys := make(map[string]struct{})
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return err
			}
			key, ok := keyToken.(string)
			if !ok {
				return errors.New("object key is not a string")
			}
			if _, exists := keys[key]; exists {
				return errors.New("duplicate JSON key")
			}
			keys[key] = struct{}{}
			if err := scanJSONValue(decoder); err != nil {
				return err
			}
		}
		end, err := decoder.Token()
		if err != nil || end != json.Delim('}') {
			return errors.New("unterminated JSON object")
		}
	case '[':
		for decoder.More() {
			if err := scanJSONValue(decoder); err != nil {
				return err
			}
		}
		end, err := decoder.Token()
		if err != nil || end != json.Delim(']') {
			return errors.New("unterminated JSON array")
		}
	default:
		return errors.New("unexpected JSON delimiter")
	}
	return nil
}

func validCatalog(value fileCatalog) bool {
	return value.Cohort != "" && value.Revision > 0 && value.ExpiresAt > 0 && len(value.Entries) > 0 && len(value.Entries) <= 2
}

func validComponent(value fileComponent) bool {
	return value.Class != "" && value.SignerID != "" && value.Version > 0 && value.ExpiresAt > 0 && value.Payload != ""
}

func allowedFileClass(class string) bool {
	return class == "release" || class == "network-profile"
}

func writeFixtureFiles(directory string, files map[string][]byte) error {
	for name, raw := range files {
		if err := os.WriteFile(filepath.Join(directory, name), raw, 0o600); err != nil {
			return fmt.Errorf("write %s: %w", name, err)
		}
	}
	return nil
}

func fileKeypair() (ed25519.PublicKey, ed25519.PrivateKey) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		panic(err)
	}
	return public, private
}

func printFileReport(name string, report map[string]string) {
	if value, found := report["catalog"]; found {
		fmt.Printf("%s=catalog:%s\n", name, value)
		return
	}
	fmt.Printf("%s=release:%s,network-profile:%s\n", name, report["release"], report["network-profile"])
}
