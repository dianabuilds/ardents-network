package messaging

import (
	"bytes"
	"crypto/hkdf"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base32"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"strings"

	identityapi "ardents/internal/identity"
)

const DefaultPubsubTopic = "/waku/2/default-waku/proto"

type Material struct {
	ContentTopic string
	envelopeKey  [32]byte
}

func (Material) String() string   { return "privacy-material[redacted]" }
func (Material) GoString() string { return "privacy-material[redacted]" }
func (Material) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct{ State string }{State: "available"})
}

func (m Material) EnvelopeKey() []byte {
	return append([]byte(nil), m.envelopeKey[:]...)
}

func Derive(resolved identityapi.ResolvedCapability) (Material, error) {
	if resolved.Generation == 0 || !resolved.Secret.Valid() || zeroID(resolved.ChannelID) ||
		resolved.Scope == "" {
		return Material{}, fmt.Errorf("resolved capability material is invalid")
	}
	generationKey, err := deriveGenerationKey(resolved)
	if err != nil {
		return Material{}, err
	}
	selectorKey, err := hkdf.Key(sha256.New, generationKey, nil, "selector-key", 32)
	if err != nil {
		return Material{}, err
	}
	envelopeKey, err := hkdf.Key(sha256.New, generationKey, nil, "envelope-key", 32)
	if err != nil {
		return Material{}, err
	}
	mac := hmac.New(sha256.New, selectorKey)
	mac.Write([]byte("channel-selector"))
	token := mac.Sum(nil)[:20]
	encoded := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(token)
	var material Material
	material.ContentTopic = "/ardents/1/" + strings.ToLower(encoded) + "/proto"
	copy(material.envelopeKey[:], envelopeKey)
	return material, nil
}

func deriveGenerationKey(resolved identityapi.ResolvedCapability) ([]byte, error) {
	saltInput := frame([]byte("ardents-private/1"), resolved.ChannelID[:])
	salt := sha256.Sum256(saltInput)
	generation := make([]byte, 4)
	binary.BigEndian.PutUint32(generation, resolved.Generation)
	info := frame([]byte("generation-key"), generation, []byte(resolved.Scope))
	return hkdf.Key(sha256.New, resolved.Secret.Bytes(), salt[:], string(info), 32)
}

func frame(parts ...[]byte) []byte {
	var out bytes.Buffer
	for _, part := range parts {
		out.Write(binary.BigEndian.AppendUint16(nil, uint16(len(part))))
		out.Write(part)
	}
	return out.Bytes()
}

func zeroID(id [16]byte) bool {
	var combined byte
	for _, value := range id {
		combined |= value
	}
	return combined == 0
}
