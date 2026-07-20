package main

import (
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
	"os"
	"runtime"
	"strings"

	identitycontinuity "ardents/internal/identity/continuity"
	identityprincipal "ardents/internal/identity/principal"
	noderecovery "ardents/internal/node/recovery"
)

func readProtectedKey(path, label string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, fmt.Errorf("%s is unavailable", label)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%s must be a regular file", label)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0 {
		return nil, fmt.Errorf("%s permissions must not allow group or other access", label)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("%s cannot be read", label)
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(raw)))
	if err != nil || len(decoded) != 32 {
		return nil, fmt.Errorf("%s must contain one base64-encoded 32-byte key", label)
	}
	return decoded, nil
}

func operatorIdentityPrivate(dataDir, subject string) (ed25519.PrivateKey, error) {
	encoded, err := identitycontinuity.NewKeyStoreInDir(dataDir).Load()
	if err != nil || encoded == "" {
		return nil, fmt.Errorf("privacy requires an existing protected node identity")
	}
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil || len(raw) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("protected node identity is invalid")
	}
	private := ed25519.PrivateKey(raw)
	principal := identityprincipal.DeriveID("p", private.Public().(ed25519.PublicKey))
	if principal != subject {
		return nil, fmt.Errorf("privacy.subject does not match the protected node identity")
	}
	state := noderecovery.NewStoreInDir(dataDir)
	if err := state.Load(); err != nil {
		return nil, fmt.Errorf("privacy requires readable canonical node identity state")
	}
	statePrincipal, device, encodedPublic := state.LoadIdentity()
	public, decodeErr := base64.StdEncoding.DecodeString(encodedPublic)
	if statePrincipal != principal || device == "" || decodeErr != nil ||
		!private.Public().(ed25519.PublicKey).Equal(ed25519.PublicKey(public)) {
		return nil, fmt.Errorf("privacy requires complete matching canonical node identity state")
	}
	return private, nil
}

func operatorTrustedIssuers(configured map[string]string) (map[string]ed25519.PublicKey, error) {
	issuers := make(map[string]ed25519.PublicKey, len(configured))
	for principal, encoded := range configured {
		raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(encoded))
		if err != nil || len(raw) != ed25519.PublicKeySize {
			return nil, fmt.Errorf("privacy.trusted_issuers contains an invalid public key")
		}
		if identityprincipal.DeriveID("p", raw) != principal {
			return nil, fmt.Errorf("privacy.trusted_issuers principal does not match its public key")
		}
		issuers[principal] = ed25519.PublicKey(append([]byte(nil), raw...))
	}
	return issuers, nil
}
