package fixture

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dianabuilds/ardents-network/internal/qualification/byteio"
)

func (fixture nodeFixture) write(root string) error {
	for _, relative := range []string{"artifacts/inputs", "plans", "clock", "evidence"} {
		if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(relative)), 0o700); err != nil {
			return err
		}
	}
	for _, zone := range []string{"e", "s1", "s2", "n1", "n2"} {
		if err := os.MkdirAll(filepath.Join(root, "state", zone), 0o700); err != nil {
			return err
		}
	}
	for _, zone := range []string{"e", "s1", "s2", "n1", "n2", "h"} {
		if err := os.MkdirAll(filepath.Join(root, "secrets", zone), 0o700); err != nil {
			return err
		}
	}
	if err := fixture.writeArtifacts(root); err != nil {
		return err
	}
	if err := fixture.writeSecrets(root); err != nil {
		return err
	}
	if err := fixture.writePlans(root); err != nil {
		return err
	}
	for _, zone := range []string{"e", "n1", "n2"} {
		if err := os.WriteFile(filepath.Join(root, "clock", zone+".observation"), []byte("external clock observation\n"), 0o600); err != nil {
			return err
		}
	}
	manifest := map[string]any{"schema": "ardents-h3-node-manifest-v1", "created_at": fixture.now,
		"network_id": hex.EncodeToString(fixture.network[:]), "authority_public": hex.EncodeToString(fixture.authorityPublic),
		"zones": []string{"e", "s1", "s2", "n1", "n2"}, "control_family": "project-controlled",
		"epochs":   []string{hex.EncodeToString(fixture.epochs[0].digest[:]), hex.EncodeToString(fixture.epochs[1].digest[:])},
		"profiles": []string{"h3-s-v1", "h3-np1-v1"}}
	return byteio.WriteJSON(filepath.Join(root, "manifest.json"), manifest, 64<<10)
}

func (fixture nodeFixture) writeArtifacts(root string) error {
	base := filepath.Join(root, "artifacts")
	for index, epoch := range fixture.epochs {
		if err := os.WriteFile(filepath.Join(base, fmt.Sprintf("epoch-%04d.bin", index+1)), epoch.raw, 0o600); err != nil {
			return err
		}
		for materialIndex, material := range epoch.materials {
			name := fmt.Sprintf("material-%04d-%04d.bin", index+1, materialIndex)
			if err := os.WriteFile(filepath.Join(base, name), material, 0o600); err != nil {
				return err
			}
		}
	}
	for index, record := range fixture.records {
		if err := os.WriteFile(filepath.Join(base, "inputs", fmt.Sprintf("%04d.bin", index)), record.raw, 0o600); err != nil {
			return err
		}
	}
	return nil
}

func (fixture nodeFixture) writeSecrets(root string) error {
	for index, zone := range []string{"e", "n1", "n2"} {
		values := map[string][]byte{"source-client-cert.pem": fixture.sourceClients[index].certificate,
			"source-client-key.pem": fixture.sourceClients[index].key}
		for source := range 2 {
			values[fmt.Sprintf("source-%d-root.pem", source+1)] = fixture.sourceServers[source].root
		}
		if strings.HasPrefix(zone, "n") {
			node := index - 1
			identity, err := nodePrivatePEM(fixture.records[node].private)
			if err != nil {
				return err
			}
			values["identity-key.pem"] = identity
			values["role-server-cert.pem"] = fixture.roleServers[node].certificate
			values["role-server-key.pem"] = fixture.roleServers[node].key
			values["harness-root.pem"] = fixture.roleCA.root
		}
		if err := writeNodeSecretSet(filepath.Join(root, "secrets", zone), values); err != nil {
			return err
		}
	}
	for index, zone := range []string{"s1", "s2"} {
		values := map[string][]byte{"source-server-cert.pem": fixture.sourceServers[index].certificate,
			"source-server-key.pem": fixture.sourceServers[index].key, "source-client-root.pem": fixture.sourceCA.root}
		if err := writeNodeSecretSet(filepath.Join(root, "secrets", zone), values); err != nil {
			return err
		}
	}
	values := map[string][]byte{"harness-cert.pem": fixture.harness.certificate, "harness-key.pem": fixture.harness.key}
	for index := range 2 {
		values[fmt.Sprintf("role-%d-root.pem", index+1)] = fixture.roleServers[index].root
	}
	return writeNodeSecretSet(filepath.Join(root, "secrets", "h"), values)
}

func writeNodeSecretSet(root string, values map[string][]byte) error {
	for name, raw := range values {
		if len(raw) == 0 || len(raw) > 64<<10 {
			return errors.New("node secret exceeds its bound")
		}
		if err := os.WriteFile(filepath.Join(root, name), raw, 0o600); err != nil {
			return err
		}
	}
	return nil
}

func nodeSourceIdentity(index int) [32]byte {
	return sha256.Sum256([]byte(fmt.Sprintf("ardents-h3-node-source-%d", index+1)))
}
