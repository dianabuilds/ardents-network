//go:build ignore

// PROTOTYPE ONLY. This file is disposable R-053 design evidence, not Stage 7 code.
package main

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"golang.org/x/crypto/argon2"
)

const (
	profileName    = "ardents-authority-envelope-v1"
	stateName      = "ardents-authority-state-v1"
	argonVersion   = 19
	argonPasses    = 3
	argonLanes     = 4
	keyBytes       = 32
	maxEnvelope    = 16 << 20
	maxPlaintext   = 8 << 20
	maxHeader      = 64 << 10
	minPassword    = 16
	maxPassword    = 1024
	encodedDigest  = 64
	encodedRootMax = 64 << 10
)

var rawURL = base64.RawURLEncoding

type kdfSpec struct {
	Name      string `json:"name"`
	Version   int    `json:"version"`
	MemoryKiB uint32 `json:"memory_kib"`
	Passes    uint32 `json:"passes"`
	Lanes     uint8  `json:"lanes"`
	Salt      string `json:"salt"`
}

type envelopeHeader struct {
	Profile       string  `json:"profile"`
	SchemaVersion int     `json:"schema_version"`
	Purpose       string  `json:"purpose"`
	KDF           kdfSpec `json:"kdf"`
	AEAD          string  `json:"aead"`
}

type envelope struct {
	Profile       string  `json:"profile"`
	SchemaVersion int     `json:"schema_version"`
	Purpose       string  `json:"purpose"`
	KDF           kdfSpec `json:"kdf"`
	AEAD          string  `json:"aead"`
	Ciphertext    string  `json:"ciphertext"`
}

type watermark struct {
	Domain string `json:"domain"`
	Value  uint64 `json:"value"`
}

type authorityState struct {
	Kind         string      `json:"kind"`
	IDCommitment string      `json:"id_commitment"`
	RootMaterial string      `json:"root_material"`
	Generation   uint64      `json:"generation"`
	Revision     uint64      `json:"revision"`
	Watermarks   []watermark `json:"watermarks"`
}

type payload struct {
	Profile       string         `json:"profile"`
	SchemaVersion int            `json:"schema_version"`
	Purpose       string         `json:"purpose"`
	Environment   string         `json:"environment"`
	Network       string         `json:"network"`
	Root          string         `json:"root"`
	Authority     authorityState `json:"authority"`
}

type machine struct {
	CustodyState string
	Generation   uint64
	Revision     uint64
	Exported     bool
	TestVerified bool
	Grants       bool
	InstanceKey  string
}

type action struct {
	Kind       string
	Generation uint64
	Revision   uint64
}

func reduce(current machine, event action) (machine, string) {
	next := current
	switch event.Kind {
	case "exported":
		next.Exported = true
		return next, "exported"
	case "test-verified":
		if !current.Exported {
			return current, "denied: export required"
		}
		next.TestVerified = true
		return next, "test-verified"
	case "restored":
		if !current.TestVerified {
			return current, "denied: test verification required"
		}
		next.CustodyState = "authority-locked"
		next.Generation = event.Generation
		next.Revision = event.Revision
		next.Grants = false
		next.InstanceKey = ""
		return next, "authority-locked"
	case "reconcile":
		if current.CustodyState != "authority-locked" {
			return current, "denied: restored authority is not locked"
		}
		if event.Generation <= current.Generation || event.Revision <= current.Revision {
			return current, "authority-locked: reconciliation is not strictly higher"
		}
		next.CustodyState = "active"
		next.Generation = event.Generation
		next.Revision = event.Revision
		next.Grants = false
		next.InstanceKey = "fresh-runtime-key-commitment"
		return next, "active: fresh runtime key; grants remain absent"
	default:
		return current, "denied: unknown action"
	}
}

func seal(password []byte, memoryKiB uint32, inner payload) ([]byte, time.Duration, error) {
	if err := validatePassword(password); err != nil {
		return nil, 0, err
	}
	if err := validateMemory(memoryKiB); err != nil {
		return nil, 0, err
	}
	plain, err := json.Marshal(inner)
	if err != nil || len(plain) > maxPlaintext {
		return nil, 0, errors.New("payload-invalid")
	}
	salt := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, 0, err
	}
	spec := kdfSpec{
		Name:      "argon2id",
		Version:   argonVersion,
		MemoryKiB: memoryKiB,
		Passes:    argonPasses,
		Lanes:     argonLanes,
		Salt:      rawURL.EncodeToString(salt),
	}
	header := envelopeHeader{
		Profile: profileName, SchemaVersion: 1, Purpose: inner.Purpose,
		KDF: spec, AEAD: "aes-256-gcm-random-nonce",
	}
	aad, err := json.Marshal(header)
	if err != nil || len(aad) > maxHeader {
		return nil, 0, errors.New("header-invalid")
	}
	started := time.Now()
	key := argon2.IDKey(password, salt, argonPasses, memoryKiB, argonLanes, keyBytes)
	duration := time.Since(started)
	block, err := aes.NewCipher(key)
	if err != nil {
		zero(key)
		return nil, 0, err
	}
	aead, err := cipher.NewGCMWithRandomNonce(block)
	if err != nil {
		zero(key)
		return nil, 0, err
	}
	ciphertext := aead.Seal(nil, nil, plain, aad)
	zero(key)
	zero(plain)
	outer := envelope{
		Profile: header.Profile, SchemaVersion: header.SchemaVersion,
		Purpose: header.Purpose, KDF: spec, AEAD: header.AEAD,
		Ciphertext: rawURL.EncodeToString(ciphertext),
	}
	encoded, err := json.Marshal(outer)
	if err != nil || len(encoded) > maxEnvelope {
		return nil, 0, errors.New("envelope-invalid")
	}
	return encoded, duration, nil
}

func openEnvelope(encoded, password []byte, environment string) (payload, time.Duration, error) {
	var zeroPayload payload
	if len(encoded) == 0 || len(encoded) > maxEnvelope {
		return zeroPayload, 0, errors.New("bundle-invalid")
	}
	var outer envelope
	if err := strictJSON(encoded, &outer); err != nil {
		return zeroPayload, 0, errors.New("bundle-invalid")
	}
	if err := validateOuter(outer); err != nil {
		return zeroPayload, 0, err
	}
	if err := validatePassword(password); err != nil {
		return zeroPayload, 0, errors.New("bundle-unlock-failed")
	}
	salt, err := rawURL.DecodeString(outer.KDF.Salt)
	if err != nil || len(salt) != 16 {
		return zeroPayload, 0, errors.New("bundle-invalid")
	}
	ciphertext, err := rawURL.DecodeString(outer.Ciphertext)
	if err != nil || len(ciphertext) < 28 || len(ciphertext) > maxPlaintext+28 {
		return zeroPayload, 0, errors.New("bundle-invalid")
	}
	header := envelopeHeader{
		Profile: outer.Profile, SchemaVersion: outer.SchemaVersion,
		Purpose: outer.Purpose, KDF: outer.KDF, AEAD: outer.AEAD,
	}
	aad, _ := json.Marshal(header)
	started := time.Now()
	key := argon2.IDKey(password, salt, outer.KDF.Passes, outer.KDF.MemoryKiB, outer.KDF.Lanes, keyBytes)
	duration := time.Since(started)
	block, _ := aes.NewCipher(key)
	aead, _ := cipher.NewGCMWithRandomNonce(block)
	plain, err := aead.Open(nil, nil, ciphertext, aad)
	zero(key)
	if err != nil {
		return zeroPayload, duration, errors.New("bundle-unlock-failed")
	}
	defer zero(plain)
	var inner payload
	if err := strictJSON(plain, &inner); err != nil || validatePayload(inner, outer.Purpose) != nil {
		return zeroPayload, duration, errors.New("bundle-invalid")
	}
	if inner.Environment != environment {
		return zeroPayload, duration, errors.New("bundle-wrong-environment")
	}
	return inner, duration, nil
}

func validateOuter(outer envelope) error {
	if outer.Profile != profileName || outer.SchemaVersion != 1 {
		return errors.New("bundle-unsupported")
	}
	if outer.Purpose != "recovery-bundle" && outer.Purpose != "authority-vault" {
		return errors.New("bundle-invalid")
	}
	if outer.KDF.Name != "argon2id" || outer.KDF.Version != argonVersion ||
		outer.KDF.Passes != argonPasses || outer.KDF.Lanes != argonLanes ||
		validateMemory(outer.KDF.MemoryKiB) != nil {
		return errors.New("bundle-unsupported")
	}
	if outer.AEAD != "aes-256-gcm-random-nonce" {
		return errors.New("bundle-unsupported")
	}
	if len(outer.KDF.Salt)+len(outer.Profile)+len(outer.Purpose)+len(outer.AEAD) > maxHeader {
		return errors.New("bundle-invalid")
	}
	return nil
}

func validatePayload(inner payload, purpose string) error {
	if inner.Profile != stateName || inner.SchemaVersion != 1 || inner.Purpose != purpose {
		return errors.New("payload-invalid")
	}
	if !digest(inner.Environment) || !digest(inner.Network) || !digest(inner.Root) ||
		!digest(inner.Authority.IDCommitment) {
		return errors.New("payload-invalid")
	}
	if inner.Authority.Kind != "service" && inner.Authority.Kind != "name" {
		return errors.New("payload-invalid")
	}
	root, err := rawURL.DecodeString(inner.Authority.RootMaterial)
	if err != nil || len(root) == 0 || len(root) > encodedRootMax {
		return errors.New("payload-invalid")
	}
	if len(inner.Authority.Watermarks) == 0 || len(inner.Authority.Watermarks) > 32 {
		return errors.New("payload-invalid")
	}
	last := ""
	for _, mark := range inner.Authority.Watermarks {
		if mark.Domain <= last || len(mark.Domain) == 0 || len(mark.Domain) > 64 {
			return errors.New("payload-invalid")
		}
		last = mark.Domain
	}
	return nil
}

func strictJSON(encoded []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		return errors.New("trailing data")
	}
	reencoded, err := json.Marshal(target)
	if err != nil || !bytes.Equal(encoded, reencoded) {
		return errors.New("non-canonical JSON")
	}
	return nil
}

func tamperCiphertext(encoded []byte) []byte {
	var outer envelope
	if strictJSON(encoded, &outer) != nil {
		return nil
	}
	ciphertext, err := rawURL.DecodeString(outer.Ciphertext)
	if err != nil || len(ciphertext) == 0 {
		return nil
	}
	ciphertext[len(ciphertext)-1] ^= 1
	outer.Ciphertext = rawURL.EncodeToString(ciphertext)
	mutated, err := json.Marshal(outer)
	if err != nil {
		return nil
	}
	return mutated
}

func validateMemory(memoryKiB uint32) error {
	if memoryKiB != 64*1024 && memoryKiB != 128*1024 && memoryKiB != 256*1024 {
		return fmt.Errorf("unsupported memory: %d KiB", memoryKiB)
	}
	return nil
}

func validatePassword(password []byte) error {
	if len(password) < minPassword || len(password) > maxPassword {
		return errors.New("password must be 16..1024 bytes")
	}
	return nil
}

func digest(value string) bool {
	if len(value) != encodedDigest {
		return false
	}
	for _, r := range value {
		if !strings.ContainsRune("0123456789abcdef", r) {
			return false
		}
	}
	return true
}

func zero(value []byte) {
	for i := range value {
		value[i] = 0
	}
}
