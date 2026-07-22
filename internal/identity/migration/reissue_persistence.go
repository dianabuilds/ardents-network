package migration

import (
	"bytes"
	"crypto/ed25519"
	"crypto/hkdf"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	discoveryrecords "ardents/internal/discovery/records"
	identityprincipal "ardents/internal/identity/principal"
	"ardents/internal/storage"
	"go.etcd.io/bbolt"
	"golang.org/x/crypto/chacha20poly1305"
)

func rewriteCapabilityStore(storePath, keyPath string, mappings map[string]string, issuerKeys map[string]ed25519.PublicKey, localLegacy string, localPrivate ed25519.PrivateKey, entropy io.Reader) (ReissueCounts, error) {
	key, err := readCapabilityStoreKey(keyPath)
	if err != nil {
		return ReissueCounts{}, err
	}
	derived, err := hkdf.Key(sha256.New, key, nil, "ardents-capability-store-encryption/1", chacha20poly1305.KeySize)
	if err != nil {
		return ReissueCounts{}, fmt.Errorf("derive capability migration key")
	}
	aead, err := chacha20poly1305.NewX(derived)
	if err != nil {
		return ReissueCounts{}, fmt.Errorf("create capability migration cipher")
	}
	db, err := bbolt.Open(storePath, 0o600, nil)
	if err != nil {
		return ReissueCounts{}, fmt.Errorf("open capability store for reissue: %w", err)
	}
	defer db.Close()
	var ledger capabilityLedger
	err = db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte("identity-capabilities"))
		if bucket == nil || bucket.Get([]byte("sealed-ledger")) == nil {
			return fmt.Errorf("capability store is unavailable")
		}
		var sealed sealedCapabilityLedger
		if err := decodeStrict(bucket.Get([]byte("sealed-ledger")), &sealed); err != nil || sealed.Version != 1 || len(sealed.Nonce) != aead.NonceSize() {
			return fmt.Errorf("capability store has an unknown or invalid schema")
		}
		plain, err := aead.Open(nil, sealed.Nonce, sealed.Ciphertext, []byte(capabilityStoreAAD))
		if err != nil {
			return fmt.Errorf("capability store authentication failed")
		}
		if err := decodeStrict(plain, &ledger); err != nil {
			return fmt.Errorf("capability ledger has an unknown or invalid schema")
		}
		return nil
	})
	if err != nil {
		return ReissueCounts{}, err
	}
	if capabilityLedgerIsReissued(ledger, mappings) {
		return ReissueCounts{}, verifyReissuedCapabilityLedger(ledger, mappings[localLegacy], localPrivate.Public().(ed25519.PublicKey))
	}
	ledger, counts, err := reissueCapabilityLedger(ledger, mappings, issuerKeys, localLegacy, localPrivate)
	if err != nil {
		return ReissueCounts{}, err
	}
	plain, err := json.Marshal(ledger)
	if err != nil {
		return ReissueCounts{}, err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(entropy, nonce); err != nil {
		return ReissueCounts{}, fmt.Errorf("generate capability migration nonce")
	}
	sealed := sealedCapabilityLedger{Version: 1, Nonce: nonce, Ciphertext: aead.Seal(nil, nonce, plain, []byte(capabilityStoreAAD))}
	raw, err := json.Marshal(sealed)
	if err != nil {
		return ReissueCounts{}, err
	}
	err = db.Update(func(tx *bbolt.Tx) error {
		return tx.Bucket([]byte("identity-capabilities")).Put([]byte("sealed-ledger"), raw)
	})
	if err != nil {
		return ReissueCounts{}, err
	}
	return counts, nil
}

func capabilityLedgerIsReissued(ledger capabilityLedger, mappings map[string]string) bool {
	for _, grants := range []map[string]capabilityGrant{ledger.Grants, ledger.SenderGrants} {
		for _, grant := range grants {
			if _, legacy := mappings[grant.IssuerPrincipal]; legacy {
				return false
			}
		}
	}
	for _, rev := range ledger.Revocations {
		if _, legacy := mappings[rev.IssuerPrincipal]; legacy {
			return false
		}
	}
	return true
}

func verifyReissuedCapabilityLedger(ledger capabilityLedger, localV1 string, public ed25519.PublicKey) error {
	if localV1 == "" || len(public) != ed25519.PublicKeySize || ledger.Grants == nil || ledger.SenderGrants == nil || ledger.Revocations == nil {
		return fmt.Errorf("reissued capability ledger is incomplete")
	}
	for _, grants := range []map[string]capabilityGrant{ledger.Grants, ledger.SenderGrants} {
		for _, grant := range grants {
			if grant.Version != 1 || grant.Generation == 0 || zero16(grant.ChannelID) || zero16(grant.GrantID) || !nonzeroBytes(grant.Secret, 32) || grant.NotBefore.IsZero() || !grant.NotAfter.After(grant.NotBefore) || grant.NotBefore.Nanosecond() != 0 || grant.NotAfter.Nanosecond() != 0 || grant.Permissions == 0 || grant.Permissions&^63 != 0 || !knownCapabilityScope(grant.Scope) || grant.IssuerPrincipal != localV1 || !strings.HasPrefix(grant.SubjectPrincipal, "p1_") || !ed25519.Verify(public, capabilityDigest("ardents-capability-grant/1", canonicalCapabilityGrant(grant)), grant.Signature) {
				return fmt.Errorf("reissued capability grant is invalid")
			}
		}
	}
	for _, rev := range ledger.Revocations {
		if rev.Version != 1 || zero16(rev.GrantID) || rev.RevokedAt.IsZero() || rev.RevokedAt.Nanosecond() != 0 || rev.IssuerPrincipal != localV1 || !ed25519.Verify(public, capabilityDigest("ardents-capability-revocation/1", canonicalCapabilityRevocation(rev)), rev.Signature) {
			return fmt.Errorf("reissued capability revocation is invalid")
		}
	}
	return nil
}

func rewriteDiscoveryStore(path string, mappings map[string]string, localLegacy string, localPrivate ed25519.PrivateKey) (ReissueCounts, error) {
	db, err := bbolt.Open(path, 0o600, nil)
	if err != nil {
		return ReissueCounts{}, fmt.Errorf("open discovery store for reissue: %w", err)
	}
	defer db.Close()
	var snapshot discoverySnapshot
	err = db.View(func(tx *bbolt.Tx) error {
		var inner error
		snapshot, inner = strictBoltJSON[discoverySnapshot](tx, "discovery", "records")
		return inner
	})
	if err != nil {
		return ReissueCounts{}, err
	}
	if discoverySnapshotIsReissued(snapshot, mappings) {
		return ReissueCounts{}, verifyReissuedDiscovery(snapshot, mappings[localLegacy], localPrivate.Public().(ed25519.PublicKey))
	}
	snapshot, counts, err := reissueDiscoverySnapshot(snapshot, mappings, localLegacy, localPrivate)
	if err != nil {
		return ReissueCounts{}, err
	}
	err = db.Update(func(tx *bbolt.Tx) error { return putBoltJSON(tx, "discovery", "records", snapshot) })
	if err != nil {
		return ReissueCounts{}, err
	}
	return counts, nil
}

func discoverySnapshotIsReissued(snapshot discoverySnapshot, mappings map[string]string) bool {
	for _, entry := range snapshot.Records {
		if _, legacy := mappings[entry.Record.Node]; legacy {
			return false
		}
	}
	return true
}

func verifyReissuedDiscovery(snapshot discoverySnapshot, localV1 string, public ed25519.PublicKey) error {
	if localV1 == "" || len(public) != ed25519.PublicKeySize {
		return fmt.Errorf("reissued discovery identity is invalid")
	}
	for _, entry := range snapshot.Records {
		record := entry.Record
		if entry.Source != discoveryrecords.Local || record.Node != localV1 {
			return fmt.Errorf("remote or foreign discovery record remains after reissue")
		}
		publicRaw, err := base64.StdEncoding.Strict().DecodeString(record.PublicKey)
		if err != nil || !bytes.Equal(publicRaw, public) {
			return fmt.Errorf("reissued discovery public key is invalid")
		}
		signature, err := base64.StdEncoding.Strict().DecodeString(record.Signature)
		if err != nil {
			return fmt.Errorf("reissued discovery signature is invalid")
		}
		canonical, err := discoveryrecords.Canonical(record)
		if err != nil || !ed25519.Verify(public, canonical, signature) {
			return fmt.Errorf("reissued discovery signature is invalid")
		}
		if record.Kind == "node" && (record.Subject != localV1 || record.ID != localV1+":node") {
			return fmt.Errorf("reissued discovery Node identity is inconsistent")
		}
		derived, deriveErr := identityprincipal.FromEd25519PublicKey(ed25519.PublicKey(publicRaw))
		if deriveErr != nil || derived.String() != record.Node {
			return fmt.Errorf("reissued discovery Node does not match signing key")
		}
	}
	return nil
}

func rewriteRealmAuthority(path string, mappings map[string]string) (ed25519.PrivateKey, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("local realm authority is unavailable")
	}
	var state realmAuthorityState
	if err := decodeStrict(raw, &state); err != nil {
		return nil, fmt.Errorf("local realm authority is invalid")
	}
	if state.Version == "ardents.local-realm/v2" {
		return verifyReissuedRealmAuthority(state, mappings)
	}
	state, private, err := reissueRealmAuthority(state, mappings)
	if err != nil {
		return nil, err
	}
	raw, err = json.Marshal(state)
	if err != nil {
		return nil, err
	}
	if err := writePrivateMigrationFile(path, raw); err != nil {
		return nil, err
	}
	return private, nil
}

func verifyReissuedRealmAuthority(state realmAuthorityState, mappings map[string]string) (ed25519.PrivateKey, error) {
	privateRaw, err := base64.StdEncoding.Strict().DecodeString(state.IssuerPrivate)
	if err != nil || len(privateRaw) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("reissued local realm key is invalid")
	}
	private := ed25519.PrivateKey(privateRaw)
	if !bytes.Equal(private, ed25519.NewKeyFromSeed(private.Seed())) || !validRealmChannel(state.Discovery) || !validRealmChannel(state.Data) {
		return nil, fmt.Errorf("reissued local realm state is invalid")
	}
	public := private.Public().(ed25519.PublicKey)
	legacy, _ := LegacyPrincipalIDFromEd25519PublicKey(public)
	issuerV1 := mappings[legacy.String()]
	if issuerV1 == "" {
		return nil, fmt.Errorf("reissued local realm issuer mapping is invalid")
	}
	for subject, member := range state.Members {
		if !isMappedValue(subject, mappings) || member.Version != "ardents.local-realm-node/v2" || member.Subject != subject || member.Issuer != issuerV1 || !validRealmGrant(member.Discovery) || !validRealmGrant(member.Data) {
			return nil, fmt.Errorf("reissued local realm member is invalid")
		}
	}
	return private, nil
}

func rewriteRealmNode(path, legacySubject, legacyIssuer string, mappings map[string]string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("local realm Node state is unavailable")
	}
	var state realmNodeState
	if err := decodeStrict(raw, &state); err != nil {
		return fmt.Errorf("local realm Node state is invalid")
	}
	if state.Version == "ardents.local-realm-node/v2" {
		if state.Subject != mappings[legacySubject] || state.Issuer != mappings[legacyIssuer] || !validRealmGrant(state.Discovery) || !validRealmGrant(state.Data) {
			return fmt.Errorf("reissued local realm Node state is invalid")
		}
		return nil
	}
	state, err = reissueRealmNode(state, legacySubject, legacyIssuer, mappings)
	if err != nil {
		return err
	}
	raw, err = json.Marshal(state)
	if err != nil {
		return err
	}
	return writePrivateMigrationFile(path, raw)
}

func writePrivateMigrationFile(path string, raw []byte) error {
	return storage.AtomicWritePrivateFile(path, append(raw, '\n'))
}
