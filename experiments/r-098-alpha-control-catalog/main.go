//go:build ignore

// R-098 is a disposable alpha-control separation experiment.
package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

type component struct {
	Class     string `json:"class"`
	SignerID  string `json:"signer_id"`
	Version   uint64 `json:"version"`
	ExpiresAt int64  `json:"expires_at"`
	Payload   string `json:"payload"`
	Signature string `json:"signature"`
}

type descriptor struct {
	Class           string `json:"class"`
	SignerID        string `json:"signer_id"`
	ComponentSHA256 string `json:"component_sha256"`
}

type catalog struct {
	Cohort    string       `json:"cohort"`
	Revision  uint64       `json:"revision"`
	ExpiresAt int64        `json:"expires_at"`
	Entries   []descriptor `json:"entries"`
	Signature string       `json:"signature"`
}

type verification struct {
	keys         map[string]ed25519.PublicKey
	floors       map[string]uint64
	catalogFloor uint64
	allowedClass map[string]bool
	now          time.Time
}

func main() {
	disclosurePublic, disclosurePrivate := keypair()
	releasePublic, releasePrivate := keypair()
	networkPublic, networkPrivate := keypair()
	keys := map[string]ed25519.PublicKey{
		"release-key": releasePublic,
		"network-key": networkPublic,
	}
	verify := verification{
		keys:         keys,
		floors:       map[string]uint64{"release": 3, "network-profile": 5},
		catalogFloor: 4,
		allowedClass: map[string]bool{"release": true, "network-profile": true},
		now:          time.Unix(1_893_456_000, 0),
	}

	release := signComponent(component{
		Class: "release", SignerID: "release-key", Version: 3,
		ExpiresAt: verify.now.Add(time.Hour).Unix(), Payload: "release-v3",
	}, releasePrivate)
	network := signComponent(component{
		Class: "network-profile", SignerID: "network-key", Version: 5,
		ExpiresAt: verify.now.Add(time.Hour).Unix(), Payload: "network-v5",
	}, networkPrivate)
	validComponents := map[string][]byte{"release": encode(release), "network-profile": encode(network)}
	validCatalog := signCatalog(catalogFor("alpha-1", 4, verify.now.Add(time.Hour), validComponents, release, network), disclosurePrivate)

	printReport("valid", verifyCatalog(verify, disclosurePublic, [][]byte{encode(validCatalog)}, validComponents))

	changedRelease := append([]byte(nil), validComponents["release"]...)
	changedRelease[len(changedRelease)-2] ^= 1
	printReport("changed_component", verifyCatalog(verify, disclosurePublic, [][]byte{encode(validCatalog)}, map[string][]byte{
		"release": changedRelease, "network-profile": validComponents["network-profile"],
	}))

	expiredNetwork := signComponent(component{
		Class: "network-profile", SignerID: "network-key", Version: 5,
		ExpiresAt: verify.now.Add(-time.Second).Unix(), Payload: "network-v5",
	}, networkPrivate)
	expiredComponents := map[string][]byte{"release": validComponents["release"], "network-profile": encode(expiredNetwork)}
	expiredCatalog := signCatalog(catalogFor("alpha-1", 4, verify.now.Add(time.Hour), expiredComponents, release, expiredNetwork), disclosurePrivate)
	printReport("expired_component", verifyCatalog(verify, disclosurePublic, [][]byte{encode(expiredCatalog)}, expiredComponents))

	lowerRelease := signComponent(component{
		Class: "release", SignerID: "release-key", Version: 2,
		ExpiresAt: verify.now.Add(time.Hour).Unix(), Payload: "release-v2",
	}, releasePrivate)
	lowerComponents := map[string][]byte{"release": encode(lowerRelease), "network-profile": validComponents["network-profile"]}
	lowerCatalog := signCatalog(catalogFor("alpha-1", 4, verify.now.Add(time.Hour), lowerComponents, lowerRelease, network), disclosurePrivate)
	printReport("lower_floor", verifyCatalog(verify, disclosurePublic, [][]byte{encode(lowerCatalog)}, lowerComponents))

	replayedCatalog := signCatalog(catalogFor("alpha-1", 3, verify.now.Add(time.Hour), validComponents, release, network), disclosurePrivate)
	printReport("replayed_catalog", verifyCatalog(verify, disclosurePublic, [][]byte{encode(replayedCatalog)}, validComponents))

	unknownCatalog := catalogFor("alpha-1", 4, verify.now.Add(time.Hour), validComponents, release, network)
	unknownCatalog.Entries = append(unknownCatalog.Entries, descriptor{Class: "unknown", SignerID: "unknown-key", ComponentSHA256: digest([]byte("unknown"))})
	unknownCatalog = signCatalog(unknownCatalog, disclosurePrivate)
	printReport("unknown_component", verifyCatalog(verify, disclosurePublic, [][]byte{encode(unknownCatalog)}, validComponents))

	otherCatalog := signCatalog(catalogFor("alpha-2", 4, verify.now.Add(time.Hour), validComponents, release, network), disclosurePrivate)
	printReport("conflicting_catalogs", verifyCatalog(verify, disclosurePublic, [][]byte{encode(validCatalog), encode(otherCatalog)}, validComponents))
	printReport("withheld_catalog", verifyCatalog(verify, disclosurePublic, nil, validComponents))
	exerciseBoundedFileReader()
}

func verifyCatalog(verify verification, disclosureKey ed25519.PublicKey, catalogs [][]byte, components map[string][]byte) map[string]string {
	if len(catalogs) == 0 {
		return map[string]string{"catalog": "unavailable"}
	}
	firstDigest := digest(catalogs[0])
	for _, raw := range catalogs[1:] {
		if digest(raw) != firstDigest {
			return map[string]string{"catalog": "conflict"}
		}
	}
	var current catalog
	if err := json.Unmarshal(catalogs[0], &current); err != nil || !verifyCatalogSignature(current, disclosureKey) {
		return map[string]string{"catalog": "invalid"}
	}
	if verify.now.Unix() >= current.ExpiresAt {
		return map[string]string{"catalog": "expired"}
	}
	if current.Revision < verify.catalogFloor {
		return map[string]string{"catalog": "lower-floor"}
	}
	report := make(map[string]string, len(current.Entries))
	for _, entry := range current.Entries {
		if !verify.allowedClass[entry.Class] {
			return map[string]string{"catalog": "unknown-component"}
		}
		raw, found := components[entry.Class]
		if !found {
			report[entry.Class] = "unavailable"
			continue
		}
		if digest(raw) != entry.ComponentSHA256 {
			report[entry.Class] = "digest-mismatch"
			continue
		}
		var item component
		if err := json.Unmarshal(raw, &item); err != nil || item.Class != entry.Class || item.SignerID != entry.SignerID {
			report[entry.Class] = "invalid"
			continue
		}
		key, found := verify.keys[item.SignerID]
		if !found || !verifyComponentSignature(item, key) {
			report[entry.Class] = "invalid-signature"
			continue
		}
		if verify.now.Unix() >= item.ExpiresAt {
			report[entry.Class] = "expired"
			continue
		}
		if item.Version < verify.floors[item.Class] {
			report[entry.Class] = "lower-floor"
			continue
		}
		report[entry.Class] = "accepted"
	}
	return report
}

func catalogFor(cohort string, revision uint64, expiresAt time.Time, components map[string][]byte, release, network component) catalog {
	return catalog{Cohort: cohort, Revision: revision, ExpiresAt: expiresAt.Unix(), Entries: []descriptor{
		{Class: release.Class, SignerID: release.SignerID, ComponentSHA256: digest(components[release.Class])},
		{Class: network.Class, SignerID: network.SignerID, ComponentSHA256: digest(components[network.Class])},
	}}
}

func signComponent(item component, key ed25519.PrivateKey) component {
	item.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(key, componentBytes(item)))
	return item
}

func signCatalog(value catalog, key ed25519.PrivateKey) catalog {
	value.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(key, catalogBytes(value)))
	return value
}

func verifyComponentSignature(item component, key ed25519.PublicKey) bool {
	signature, err := base64.StdEncoding.DecodeString(item.Signature)
	return err == nil && ed25519.Verify(key, componentBytes(item), signature)
}

func verifyCatalogSignature(value catalog, key ed25519.PublicKey) bool {
	signature, err := base64.StdEncoding.DecodeString(value.Signature)
	return err == nil && ed25519.Verify(key, catalogBytes(value), signature)
}

func componentBytes(item component) []byte {
	item.Signature = ""
	return encode(item)
}

func catalogBytes(value catalog) []byte {
	value.Signature = ""
	return encode(value)
}

func encode(value any) []byte {
	raw, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return raw
}

func digest(raw []byte) string {
	result := sha256.Sum256(raw)
	return fmt.Sprintf("%x", result)
}

func keypair() (ed25519.PublicKey, ed25519.PrivateKey) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		panic(errors.New("generate synthetic key: " + err.Error()))
	}
	return public, private
}

func printReport(name string, report map[string]string) {
	if value, found := report["catalog"]; found {
		fmt.Printf("%s=catalog:%s\n", name, value)
		return
	}
	fmt.Printf("%s=release:%s,network-profile:%s\n", name, report["release"], report["network-profile"])
}
