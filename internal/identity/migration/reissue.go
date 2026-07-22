package migration

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"ardents/internal/config"
	"ardents/internal/storage"
)

const (
	reissueMarkerName     = "pia004b-state.json"
	reissueNodeMarkerName = "pia004b-node-state.json"
	reissueBackupDirName  = "pia004b-backup"
)

type reissuePhase string

const (
	reissuePhaseBackup    reissuePhase = "backup"
	reissuePhaseAuthority reissuePhase = "authority_reissued"
	reissuePhaseNodes     reissuePhase = "nodes_reissuing"
	reissuePhaseVerified  reissuePhase = "pia004b_verified"
	reissuePhaseRestored  reissuePhase = "restored"
)

type reissueMarker struct {
	SchemaVersion   uint32            `json:"schema_version"`
	Phase           reissuePhase      `json:"phase"`
	InventoryDigest string            `json:"inventory_sha256"`
	NodeIndex       int               `json:"node_index"`
	NodeComponent   string            `json:"node_component,omitempty"`
	BackupHashes    map[string]string `json:"backup_sha256"`
	LocalReissued   int               `json:"local_reissued"`
	RemoteExpired   int               `json:"remote_expired"`
}

type reissueHook func(reissuePhase, int, string) error

func RunReissueArtifacts(args []string, stdout io.Writer) error {
	set := flag.NewFlagSet("ardentsd identity-migration reissue-artifacts", flag.ContinueOnError)
	set.SetOutput(io.Discard)
	manifestPath := set.String("manifest", "", "migration manifest JSON")
	inventoryPath := set.String("inventory", "", "reviewed inventory JSON")
	if err := set.Parse(args); err != nil || set.NArg() != 0 || *manifestPath == "" || *inventoryPath == "" {
		return fmt.Errorf("--manifest and --inventory are required")
	}
	manifest, inventory, err := loadReviewedInventory(*manifestPath, *inventoryPath)
	if err != nil {
		return err
	}
	if err := reissueArtifacts(manifest, inventory, rand.Reader, nil); err != nil {
		return err
	}
	_, err = fmt.Fprintln(stdout, "signed identity artifacts reached pia004b_verified; do not start Nodes before PIA-004C activation")
	return err
}

func RunRestoreArtifacts(args []string, stdout io.Writer) error {
	set := flag.NewFlagSet("ardentsd identity-migration restore-artifacts", flag.ContinueOnError)
	set.SetOutput(io.Discard)
	manifestPath := set.String("manifest", "", "migration manifest JSON")
	if err := set.Parse(args); err != nil || set.NArg() != 0 || *manifestPath == "" {
		return fmt.Errorf("--manifest is required")
	}
	raw, err := os.ReadFile(*manifestPath)
	if err != nil {
		return fmt.Errorf("migration manifest is unavailable")
	}
	var manifest InventoryManifest
	if err := decodeStrict(raw, &manifest); err != nil || manifest.SchemaVersion != 1 || manifest.AuthorityDir == "" || len(manifest.Nodes) == 0 {
		return fmt.Errorf("migration manifest is invalid or unsupported")
	}
	if err := restoreArtifacts(manifest, nil); err != nil {
		return err
	}
	_, err = fmt.Fprintln(stdout, "signed-state consistency groups restored; run PIA-004A restore for every Node before starting the old epoch")
	return err
}

func reissueArtifacts(manifest InventoryManifest, inventory NetworkInventory, entropy io.Reader, hook reissueHook) error {
	locks, err := acquireMigrationLocks(manifest)
	if err != nil {
		return err
	}
	defer closeMigrationLocks(locks)
	markerPath := filepath.Join(manifest.AuthorityDir, applyDirName, reissueMarkerName)
	digest, err := inventoryDigest(inventory)
	if err != nil {
		return err
	}
	marker, found, err := loadReissueMarker(markerPath)
	if err != nil {
		return err
	}
	if found && (marker.SchemaVersion != 1 || marker.InventoryDigest != digest) {
		return fmt.Errorf("signed-artifact migration marker is ambiguous or belongs to another inventory")
	}
	if !found || marker.Phase == reissuePhaseRestored {
		if err := verifyPIA004APreconditions(manifest, inventory); err != nil {
			return err
		}
		marker = reissueMarker{SchemaVersion: 1, Phase: reissuePhaseBackup, InventoryDigest: digest, BackupHashes: map[string]string{}}
		if err := createReissueBackups(manifest, &marker); err != nil {
			return err
		}
		for _, node := range manifest.Nodes {
			if err := writeReissueNodeMarker(node.DataDir, reissuePhaseBackup); err != nil {
				return err
			}
		}
		if err := writeReissueMarker(markerPath, marker); err != nil {
			return err
		}
		if err := runReissueHook(hook, reissuePhaseBackup, -1, ""); err != nil {
			return err
		}
	}
	mappings := mappingsFromInventory(inventory)
	if marker.Phase == reissuePhaseBackup {
		if _, err := rewriteRealmAuthority(filepath.Join(manifest.AuthorityDir, "authority.json"), mappings); err != nil {
			return err
		}
		marker.Phase = reissuePhaseAuthority
		if err := writeReissueMarker(markerPath, marker); err != nil {
			return err
		}
		if err := runReissueHook(hook, reissuePhaseAuthority, -1, "authority"); err != nil {
			return err
		}
	}
	if marker.Phase == reissuePhaseAuthority {
		marker.Phase = reissuePhaseNodes
		if err := writeReissueMarker(markerPath, marker); err != nil {
			return err
		}
	}
	if marker.Phase == reissuePhaseNodes {
		authorityPrivate, err := rewriteRealmAuthority(filepath.Join(manifest.AuthorityDir, "authority.json"), mappings)
		if err != nil {
			return err
		}
		for marker.NodeIndex < len(manifest.Nodes) {
			spec := manifest.Nodes[marker.NodeIndex]
			expected := inventory.Nodes[marker.NodeIndex]
			if marker.NodeComponent == "" {
				if err := rewriteRealmNode(filepath.Join(spec.SecretDir, "local-realm-node.json"), expected.Identity.Node.LegacyPrincipal, inventory.Realm.LegacyIssuer, mappings); err != nil {
					return fmt.Errorf("reissue Node %s realm state: %w", spec.Name, err)
				}
				marker.NodeComponent = "realm"
				if err := writeReissueNodeMarker(spec.DataDir, reissuePhaseNodes); err != nil {
					return err
				}
				if err := writeReissueMarker(markerPath, marker); err != nil {
					return err
				}
				if err := runReissueHook(hook, reissuePhaseNodes, marker.NodeIndex, "realm"); err != nil {
					return err
				}
			}
			if marker.NodeComponent == "realm" {
				issuerKeys, err := migratedIssuerKeys(spec.ConfigPath, mappings, inventory.Realm.LegacyIssuer, authorityPrivate.Public().(ed25519.PublicKey))
				if err != nil {
					return err
				}
				counts, err := rewriteCapabilityStore(filepath.Join(spec.DataDir, "capabilities.db"), filepath.Join(spec.SecretDir, "capability-store.key"), mappings, issuerKeys, inventory.Realm.LegacyIssuer, authorityPrivate, entropy)
				if err != nil {
					return fmt.Errorf("reissue Node %s capability artifacts: %w", spec.Name, err)
				}
				marker.LocalReissued += counts.LocalReissued
				marker.RemoteExpired += counts.RemoteExpired
				marker.NodeComponent = "capability"
				if err := writeReissueMarker(markerPath, marker); err != nil {
					return err
				}
				if err := runReissueHook(hook, reissuePhaseNodes, marker.NodeIndex, "capability"); err != nil {
					return err
				}
			}
			if marker.NodeComponent == "capability" {
				nodePrivate, err := readNodePrivateKey(filepath.Join(spec.DataDir, "identity_key.json"))
				if err != nil {
					return err
				}
				counts, err := rewriteDiscoveryStore(filepath.Join(spec.DataDir, "ardents.db"), mappings, expected.Identity.Node.LegacyPrincipal, nodePrivate)
				if err != nil {
					return fmt.Errorf("reissue Node %s discovery artifacts: %w", spec.Name, err)
				}
				marker.LocalReissued += counts.LocalReissued
				marker.RemoteExpired += counts.RemoteExpired
				marker.NodeComponent = "discovery"
				if err := writeReissueMarker(markerPath, marker); err != nil {
					return err
				}
				if err := runReissueHook(hook, reissuePhaseNodes, marker.NodeIndex, "discovery"); err != nil {
					return err
				}
			}
			marker.NodeIndex++
			marker.NodeComponent = ""
			if err := writeReissueMarker(markerPath, marker); err != nil {
				return err
			}
		}
		marker.Phase = reissuePhaseVerified
		if err := verifyReissuedArtifacts(manifest, inventory); err != nil {
			return err
		}
		if err := writeReissueMarker(markerPath, marker); err != nil {
			return err
		}
		for _, node := range manifest.Nodes {
			if err := writeReissueNodeMarker(node.DataDir, reissuePhaseVerified); err != nil {
				return err
			}
		}
		if err := runReissueHook(hook, reissuePhaseVerified, -1, ""); err != nil {
			return err
		}
	}
	if marker.Phase != reissuePhaseVerified {
		return fmt.Errorf("signed-artifact migration marker has unknown phase")
	}
	if err := verifyReissuedArtifacts(manifest, inventory); err != nil {
		return err
	}
	for _, node := range manifest.Nodes {
		if err := writeReissueNodeMarker(node.DataDir, reissuePhaseVerified); err != nil {
			return err
		}
	}
	return nil
}

func inventoryDigest(inventory NetworkInventory) (string, error) {
	raw, err := json.Marshal(inventory)
	if err != nil {
		return "", err
	}
	sum := sha256Bytes(raw)
	return hex.EncodeToString(sum), nil
}
func sha256Bytes(raw []byte) []byte { sum := sha256.Sum256(raw); return sum[:] }

func verifyPIA004APreconditions(manifest InventoryManifest, inventory NetworkInventory) error {
	if len(manifest.Nodes) != len(inventory.Nodes) {
		return fmt.Errorf("manifest and inventory Node counts differ")
	}
	for i, spec := range manifest.Nodes {
		if spec.Name != inventory.Nodes[i].Name {
			return fmt.Errorf("manifest and inventory Node order differs")
		}
		marker, found, err := loadApplyMarker(filepath.Join(spec.DataDir, applyDirName, applyMarkerName))
		if err != nil || !found || marker.Phase != phaseVerified || marker.Legacy != inventory.Nodes[i].Identity.Node.LegacyPrincipal || marker.PrincipalV1 != inventory.Nodes[i].Identity.Node.PrincipalV1 {
			return fmt.Errorf("Node %s has not reached verified PIA-004A state", spec.Name)
		}
	}
	return nil
}

func acquireMigrationLocks(manifest InventoryManifest) ([]*storage.StateDirLock, error) {
	dirs := []string{manifest.AuthorityDir}
	for _, node := range manifest.Nodes {
		dirs = append(dirs, node.DataDir)
	}
	sort.Strings(dirs)
	locks := []*storage.StateDirLock{}
	previous := ""
	for _, dir := range dirs {
		clean := filepath.Clean(dir)
		if clean == previous {
			closeMigrationLocks(locks)
			return nil, fmt.Errorf("migration state directories overlap")
		}
		previous = clean
		lock, err := storage.AcquireStateDirLock(clean)
		if err != nil {
			closeMigrationLocks(locks)
			return nil, fmt.Errorf("migration requires every daemon and authority to be stopped: %w", err)
		}
		locks = append(locks, lock)
	}
	return locks, nil
}

func closeMigrationLocks(locks []*storage.StateDirLock) {
	for i := len(locks) - 1; i >= 0; i-- {
		_ = locks[i].Close()
	}
}

func createReissueBackups(manifest InventoryManifest, marker *reissueMarker) error {
	files := reissueFiles(manifest)
	for key, pair := range files {
		if err := copyPrivateFile(pair[0], pair[1]); err != nil {
			return fmt.Errorf("backup signed migration state %s: %w", key, err)
		}
		hash, err := fileHash(pair[1])
		if err != nil {
			return err
		}
		marker.BackupHashes[key] = hash
	}
	return nil
}

func reissueFiles(manifest InventoryManifest) map[string][2]string {
	files := map[string][2]string{"authority": {filepath.Join(manifest.AuthorityDir, "authority.json"), filepath.Join(manifest.AuthorityDir, applyDirName, reissueBackupDirName, "authority.json")}}
	for _, node := range manifest.Nodes {
		base := filepath.Join(node.DataDir, applyDirName, reissueBackupDirName)
		files["node:"+node.Name+":database"] = [2]string{filepath.Join(node.DataDir, "ardents.db"), filepath.Join(base, "ardents.db")}
		files["node:"+node.Name+":capabilities"] = [2]string{filepath.Join(node.DataDir, "capabilities.db"), filepath.Join(base, "capabilities.db")}
		files["node:"+node.Name+":realm"] = [2]string{filepath.Join(node.SecretDir, "local-realm-node.json"), filepath.Join(base, "local-realm-node.json")}
	}
	return files
}

func migratedIssuerKeys(configPath string, mappings map[string]string, localLegacy string, localPublic ed25519.PublicKey) (map[string]ed25519.PublicKey, error) {
	doc, err := config.Load(configPath)
	if err != nil {
		return nil, fmt.Errorf("load migrated trusted issuers: %w", err)
	}
	keys := map[string]ed25519.PublicKey{localLegacy: localPublic}
	reverse := map[string]string{}
	for old, current := range mappings {
		reverse[current] = old
	}
	for current, encoded := range doc.Privacy.TrustedIssuers {
		old, ok := reverse[current]
		if !ok {
			return nil, fmt.Errorf("migrated trusted issuer has no reviewed mapping")
		}
		raw, err := base64.StdEncoding.Strict().DecodeString(encoded)
		if err != nil || len(raw) != ed25519.PublicKeySize {
			return nil, fmt.Errorf("migrated trusted issuer key is invalid")
		}
		keys[old] = ed25519.PublicKey(raw)
	}
	return keys, nil
}

func verifyReissuedArtifacts(manifest InventoryManifest, inventory NetworkInventory) error {
	mappings := mappingsFromInventory(inventory)
	authorityPrivate, err := rewriteRealmAuthority(filepath.Join(manifest.AuthorityDir, "authority.json"), mappings)
	if err != nil {
		return err
	}
	for i, spec := range manifest.Nodes {
		expected := inventory.Nodes[i]
		if err := rewriteRealmNode(filepath.Join(spec.SecretDir, "local-realm-node.json"), expected.Identity.Node.LegacyPrincipal, inventory.Realm.LegacyIssuer, mappings); err != nil {
			return err
		}
		keys, err := migratedIssuerKeys(spec.ConfigPath, mappings, inventory.Realm.LegacyIssuer, authorityPrivate.Public().(ed25519.PublicKey))
		if err != nil {
			return err
		}
		if _, err := rewriteCapabilityStore(filepath.Join(spec.DataDir, "capabilities.db"), filepath.Join(spec.SecretDir, "capability-store.key"), mappings, keys, inventory.Realm.LegacyIssuer, authorityPrivate, io.LimitReader(rand.Reader, 0)); err != nil {
			return err
		}
		nodePrivate, err := readNodePrivateKey(filepath.Join(spec.DataDir, "identity_key.json"))
		if err != nil {
			return err
		}
		if _, err := rewriteDiscoveryStore(filepath.Join(spec.DataDir, "ardents.db"), mappings, expected.Identity.Node.LegacyPrincipal, nodePrivate); err != nil {
			return err
		}
	}
	return nil
}

func loadReissueMarker(path string) (reissueMarker, bool, error) {
	raw, found, err := storage.ReadPrivateFile(path)
	if err != nil {
		return reissueMarker{}, false, fmt.Errorf("signed-artifact migration marker is unavailable")
	}
	if !found {
		return reissueMarker{}, false, nil
	}
	var marker reissueMarker
	if err := decodeStrict(raw, &marker); err != nil {
		return reissueMarker{}, false, fmt.Errorf("signed-artifact migration marker is invalid")
	}
	return marker, true, nil
}
func writeReissueMarker(path string, marker reissueMarker) error {
	raw, err := json.MarshalIndent(marker, "", "  ")
	if err != nil {
		return err
	}
	return storage.AtomicWritePrivateFile(path, append(raw, '\n'))
}
func runReissueHook(hook reissueHook, phase reissuePhase, index int, component string) error {
	if hook == nil {
		return nil
	}
	return hook(phase, index, component)
}

func writeReissueNodeMarker(dataDir string, phase reissuePhase) error {
	raw, err := json.Marshal(struct {
		SchemaVersion uint32       `json:"schema_version"`
		Phase         reissuePhase `json:"phase"`
	}{SchemaVersion: 1, Phase: phase})
	if err != nil {
		return err
	}
	return storage.AtomicWritePrivateFile(filepath.Join(dataDir, applyDirName, reissueNodeMarkerName), append(raw, '\n'))
}

func restoreArtifacts(manifest InventoryManifest, hook func(string) error) error {
	locks, err := acquireMigrationLocks(manifest)
	if err != nil {
		return err
	}
	defer closeMigrationLocks(locks)
	markerPath := filepath.Join(manifest.AuthorityDir, applyDirName, reissueMarkerName)
	marker, found, err := loadReissueMarker(markerPath)
	if err != nil || !found || len(marker.BackupHashes) == 0 {
		return fmt.Errorf("signed-artifact backup marker is unavailable")
	}
	files := reissueFiles(manifest)
	keys := sortedKeys(files)
	for _, key := range keys {
		pair, ok := files[key]
		expected, hashOK := marker.BackupHashes[key]
		if !ok || !hashOK {
			return fmt.Errorf("signed-artifact backup set is incomplete")
		}
		actual, err := fileHash(pair[1])
		if err != nil || actual != expected {
			return fmt.Errorf("signed-artifact backup consistency check failed")
		}
	}
	for _, key := range keys {
		pair := files[key]
		raw, err := os.ReadFile(pair[1])
		if err != nil {
			return err
		}
		if err := storage.AtomicWritePrivateFile(pair[0], raw); err != nil {
			return err
		}
		if hook != nil {
			if err := hook(key); err != nil {
				return err
			}
		}
	}
	marker.Phase, marker.NodeIndex, marker.NodeComponent = reissuePhaseRestored, 0, ""
	if err := writeReissueMarker(markerPath, marker); err != nil {
		return err
	}
	for _, node := range manifest.Nodes {
		if err := writeReissueNodeMarker(node.DataDir, reissuePhaseRestored); err != nil {
			return err
		}
	}
	return nil
}
