package migration

import (
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	identityprincipal "ardents/internal/identity/principal"
)

type RealmInventory struct {
	LegacyIssuer string                 `json:"legacy_issuer"`
	IssuerV1     string                 `json:"issuer_v1"`
	Members      []RealmMemberInventory `json:"members"`
}

type RealmMemberInventory struct {
	LegacySubject  string `json:"legacy_subject"`
	SubjectV1      string `json:"subject_v1"`
	Classification string `json:"classification"`
}

type realmAuthorityState struct {
	Version       string                    `json:"version"`
	IssuerPrivate string                    `json:"issuer_private"`
	Discovery     realmChannelState         `json:"discovery"`
	Data          realmChannelState         `json:"data"`
	Members       map[string]realmNodeState `json:"members,omitempty"`
}
type realmChannelState struct {
	ID         string `json:"id"`
	Secret     string `json:"secret"`
	Generation uint32 `json:"generation"`
}
type realmNodeState struct {
	Version   string          `json:"version"`
	Subject   string          `json:"subject"`
	Issuer    string          `json:"issuer"`
	Discovery realmGrantState `json:"discovery"`
	Data      realmGrantState `json:"data"`
}
type realmGrantState struct {
	ID        string    `json:"id"`
	NotBefore time.Time `json:"not_before"`
	NotAfter  time.Time `json:"not_after"`
}

func InventoryRealm(authorityDir string, principalMappings map[string]string) (RealmInventory, error) {
	raw, err := os.ReadFile(filepath.Join(authorityDir, "authority.json"))
	if err != nil {
		return RealmInventory{}, fmt.Errorf("local realm authority is unavailable")
	}
	var state realmAuthorityState
	if err := decodeStrict(raw, &state); err != nil {
		return RealmInventory{}, fmt.Errorf("local realm authority has an unknown or invalid schema")
	}
	if state.Version != "ardents.local-realm/v1" || !validRealmChannel(state.Discovery) || !validRealmChannel(state.Data) {
		return RealmInventory{}, fmt.Errorf("local realm authority version or channel state is unsupported")
	}
	privateRaw, err := base64.StdEncoding.Strict().DecodeString(state.IssuerPrivate)
	if err != nil || len(privateRaw) != ed25519.PrivateKeySize {
		return RealmInventory{}, fmt.Errorf("local realm authority key is invalid")
	}
	private := ed25519.PrivateKey(privateRaw)
	rebuilt := ed25519.NewKeyFromSeed(private.Seed())
	if string(rebuilt) != string(private) {
		return RealmInventory{}, fmt.Errorf("local realm authority key is invalid")
	}
	issuerLegacy, err := LegacyPrincipalIDFromEd25519PublicKey(private.Public().(ed25519.PublicKey))
	if err != nil {
		return RealmInventory{}, fmt.Errorf("local realm authority key is invalid")
	}
	issuerV1, err := identityprincipal.FromEd25519PublicKey(private.Public().(ed25519.PublicKey))
	if err != nil {
		return RealmInventory{}, fmt.Errorf("local realm authority key is invalid")
	}
	report := RealmInventory{LegacyIssuer: issuerLegacy.String(), IssuerV1: issuerV1.String()}
	keys := make([]string, 0, len(state.Members))
	for key := range state.Members {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		member := state.Members[key]
		mapped, ok := principalMappings[key]
		if !ok {
			return RealmInventory{}, fmt.Errorf("local realm member Principal has no verified Node mapping")
		}
		if _, err := identityprincipal.Parse(mapped); err != nil {
			return RealmInventory{}, fmt.Errorf("local realm member mapping is invalid")
		}
		if member.Version != "ardents.local-realm-node/v1" || member.Subject != key || member.Issuer != issuerLegacy.String() || !validRealmGrant(member.Discovery) || !validRealmGrant(member.Data) {
			return RealmInventory{}, fmt.Errorf("local realm member state is inconsistent")
		}
		report.Members = append(report.Members, RealmMemberInventory{LegacySubject: key, SubjectV1: mapped, Classification: "reissue_signed_channel_grants"})
	}
	return report, nil
}

func validRealmChannel(value realmChannelState) bool {
	id, e1 := base64.StdEncoding.Strict().DecodeString(value.ID)
	secret, e2 := base64.StdEncoding.Strict().DecodeString(value.Secret)
	return e1 == nil && e2 == nil && len(id) == 16 && len(secret) == 32 && value.Generation > 0
}
func validRealmGrant(value realmGrantState) bool {
	id, err := base64.StdEncoding.Strict().DecodeString(value.ID)
	return err == nil && len(id) == 16 && !value.NotBefore.IsZero() && value.NotAfter.After(value.NotBefore)
}
