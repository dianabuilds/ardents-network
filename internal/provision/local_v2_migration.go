package provision

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	authoritydomain "ardents/internal/authority"
	identityapi "ardents/internal/identity"
	identitycapability "ardents/internal/identity/capability"
	identityprincipal "ardents/internal/identity/principal"
	identitytrust "ardents/internal/identity/trust"
	"ardents/internal/storage"
)

type LocalV2MigrationSource struct {
	RequestID             string
	AuthorityBackupPath   string
	OriginalAuthorityPath string
	OldManagerStateDir    string
	Nodes                 []NodeOptions
}

// LocalV2MigrationEvidence holds the old-manager state lock until Close. The
// caller must keep it open through Service.MigrateLocalV2, which makes the
// stopped-manager fence an observed OS lock rather than a caller assertion.
type LocalV2MigrationEvidence struct {
	Request authoritydomain.MigrateLocalV2Request
	lock    *storage.StateDirLock
}

func (e *LocalV2MigrationEvidence) Close() error {
	if e == nil || e.lock == nil {
		return nil
	}
	err := e.lock.Close()
	e.lock = nil
	return err
}

func BuildLocalV2MigrationEvidence(
	source LocalV2MigrationSource,
) (*LocalV2MigrationEvidence, error) {
	if source.RequestID == "" ||
		filepath.Clean(source.AuthorityBackupPath) == filepath.Clean(source.OriginalAuthorityPath) {
		return nil, fmt.Errorf("local-v2 migration source is invalid")
	}
	if _, err := os.Lstat(source.OriginalAuthorityPath); !os.IsNotExist(err) {
		return nil, fmt.Errorf("shared local-v2 authority is still present")
	}
	lock, err := storage.AcquireStateDirLock(source.OldManagerStateDir)
	if err != nil {
		return nil, fmt.Errorf("old local-v2 manager is not fenced: %w", err)
	}
	fail := func(cause error) (*LocalV2MigrationEvidence, error) {
		_ = lock.Close()
		return nil, cause
	}
	authorityRaw, found, err := storage.ReadPrivateFileBounded(
		source.AuthorityBackupPath, 4<<20,
	)
	if err != nil || !found {
		return fail(fmt.Errorf("read local-v2 authority backup"))
	}
	var legacyAuthority authorityState
	if storage.DecodeJSONStrict(authorityRaw, &legacyAuthority) != nil {
		return fail(fmt.Errorf("decode local-v2 authority backup"))
	}
	issuerPrivate, err := validateAuthority(legacyAuthority)
	if err != nil {
		return fail(fmt.Errorf("validate local-v2 authority backup"))
	}
	issuerPublic := issuerPrivate.Public().(ed25519.PublicKey)
	principalID, err := identityprincipal.FromEd25519PublicKey(issuerPublic)
	if err != nil {
		return fail(fmt.Errorf("derive local-v2 issuer principal"))
	}
	principal := principalID.String()
	trust, err := identitytrust.NewRegistry([]identitytrust.Entry{{
		Principal: principal, PublicKey: issuerPublic,
		Purposes: []identitytrust.Purpose{identitytrust.PurposeChannelIssue},
	}})
	if err != nil {
		return fail(fmt.Errorf("build local-v2 issuer trust"))
	}
	nodes := append([]NodeOptions(nil), source.Nodes...)
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].SecretDir < nodes[j].SecretDir
	})
	members := make([]authoritydomain.LocalV2MemberEvidence, 0, len(nodes))
	evidenceParts := []string{
		filepath.Clean(source.OldManagerStateDir),
		filepath.Clean(source.OriginalAuthorityPath),
		digestBytes(authorityRaw),
	}
	for _, node := range nodes {
		nodeRaw, found, readErr := storage.ReadPrivateFileBounded(
			filepath.Join(node.SecretDir, "local-realm-node.json"), 64<<10,
		)
		if readErr != nil || !found {
			return fail(fmt.Errorf("read local-v2 node state"))
		}
		var state nodeState
		if storage.DecodeJSONStrict(nodeRaw, &state) != nil ||
			state.Version != nodeVersion || state.Issuer != principal {
			return fail(fmt.Errorf("validate local-v2 node state"))
		}
		keyRaw, found, readErr := storage.ReadPrivateFileBounded(
			filepath.Join(node.SecretDir, "channel-grant-store.key"), 64,
		)
		key, decodeErr := base64.StdEncoding.DecodeString(string(keyRaw))
		if readErr != nil || !found || decodeErr != nil || len(key) != 32 {
			return fail(fmt.Errorf("read local-v2 capability store key"))
		}
		storePath := filepath.Join(node.DataDir, "channel-grants.db")
		service, openErr := identitycapability.NewService(
			storePath, key, state.Subject, trust, migrationReadAdmission{}, nil,
		)
		clear(key)
		if openErr != nil {
			return fail(fmt.Errorf("open local-v2 capability store"))
		}
		grants, snapshotErr := service.ReceiverGrantSnapshot()
		if snapshotErr != nil {
			return fail(fmt.Errorf("verify local-v2 capability store"))
		}
		storeRaw, readErr := os.ReadFile(storePath)
		if readErr != nil {
			return fail(fmt.Errorf("hash local-v2 capability store"))
		}
		members = append(members, authoritydomain.LocalV2MemberEvidence{
			NodeState: nodeRaw, ReceiverGrants: grants,
		})
		evidenceParts = append(
			evidenceParts, digestBytes(nodeRaw), digestBytes(storeRaw),
		)
	}
	fenceRaw, err := json.Marshal(evidenceParts)
	if err != nil {
		return fail(fmt.Errorf("encode local-v2 fence evidence"))
	}
	evidence := &LocalV2MigrationEvidence{lock: lock}
	evidence.Request = authoritydomain.MigrateLocalV2Request{
		Version:        authoritydomain.ContractVersion,
		RequestID:      source.RequestID,
		AuthorityState: authorityRaw,
		Members:        members,
		OldManagerFence: authoritydomain.LocalV2ManagerFence{
			OldProcessStopped: true, SharedAuthorityRemoved: true,
			EvidenceDigest: digestBytes(fenceRaw),
		},
	}
	return evidence, nil
}

type migrationReadAdmission struct{}

func (migrationReadAdmission) AllowCapabilityUse(identityapi.CapabilityUse) error {
	return nil
}

func digestBytes(raw []byte) string {
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:])
}
