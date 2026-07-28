package authority

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"sort"
	"strings"
	"time"

	identityapi "ardents/internal/identity"
	identitycapability "ardents/internal/identity/capability"
	identityprincipal "ardents/internal/identity/principal"
)

const (
	localV2AuthorityVersion = "ardents.local-realm/v2"
	localV2NodeVersion      = "ardents.local-realm-node/v2"
)

type LocalV2ManagerFence struct {
	OldProcessStopped      bool   `json:"old_process_stopped"`
	SharedAuthorityRemoved bool   `json:"shared_authority_removed"`
	EvidenceDigest         string `json:"evidence_digest"`
}

type LocalV2MemberEvidence struct {
	NodeState      []byte                        `json:"node_state"`
	ReceiverGrants []identityapi.CapabilityGrant `json:"receiver_grants"`
}

type MigrateLocalV2Request struct {
	Version         uint32                  `json:"version"`
	RequestID       string                  `json:"request_id"`
	AuthorityState  []byte                  `json:"authority_state"`
	Members         []LocalV2MemberEvidence `json:"members"`
	OldManagerFence LocalV2ManagerFence     `json:"old_manager_fence"`
}

type MigrateLocalV2Result struct {
	Version               uint32   `json:"version"`
	RealmID               string   `json:"realm_id"`
	AuthorityEpoch        uint64   `json:"authority_epoch"`
	AuthoritySequence     uint64   `json:"authority_sequence"`
	CheckpointDigest      string   `json:"checkpoint_digest"`
	DiscoveryChannelID    [16]byte `json:"discovery_channel_id"`
	DataChannelID         [16]byte `json:"data_channel_id"`
	Phase                 string   `json:"phase"`
	Readiness             string   `json:"readiness"`
	FreshRotationRequired bool     `json:"fresh_rotation_required"`
	OldManagerFenceDigest string   `json:"old_manager_fence_digest"`
}

type localV2AuthorityState struct {
	Version       string                      `json:"version"`
	IssuerPrivate string                      `json:"issuer_private"`
	Discovery     localV2ChannelState         `json:"discovery"`
	Data          localV2ChannelState         `json:"data"`
	Members       map[string]localV2NodeState `json:"members"`
}

type localV2ChannelState struct {
	ID         string `json:"id"`
	Secret     string `json:"secret"`
	Generation uint32 `json:"generation"`
}

type localV2NodeState struct {
	Version   string            `json:"version"`
	Subject   string            `json:"subject"`
	Issuer    string            `json:"issuer"`
	Discovery localV2GrantState `json:"discovery"`
	Data      localV2GrantState `json:"data"`
}

type localV2GrantState struct {
	ID        string    `json:"id"`
	NotBefore time.Time `json:"not_before"`
	NotAfter  time.Time `json:"not_after"`
}

type preparedLocalV2Migration struct {
	principal   string
	public      ed25519.PublicKey
	members     []MemberRecord
	channels    []ChannelRecord
	discoveryID [16]byte
	dataID      [16]byte
	payloadHash string
}

func (s *Service) MigrateLocalV2(
	ctx context.Context,
	command Command,
	request MigrateLocalV2Request,
) (MigrateLocalV2Result, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := validateMigrateLocalV2Command(command); err != nil {
		return MigrateLocalV2Result{}, err
	}
	prepared, err := prepareLocalV2Migration(request)
	if err != nil {
		return MigrateLocalV2Result{}, err
	}
	if err := s.continuationMutationFence(); err != nil {
		return MigrateLocalV2Result{}, err
	}
	if s.store == nil || s.signer == nil || s.repository == nil || s.policy == nil {
		return MigrateLocalV2Result{}, ErrUnavailable
	}
	if err := s.policy.AdmitRealmGenesis(ctx, command); err != nil {
		return MigrateLocalV2Result{}, ErrPermissionDenied
	}
	if err := s.policy.AdmitAuthorityRecovery(ctx, command); err != nil {
		return MigrateLocalV2Result{}, ErrPermissionDenied
	}

	current, found, err := s.store.Load(ctx)
	if err != nil {
		s.setUnavailable(ReasonStoreUnavailable)
		return MigrateLocalV2Result{}, ErrUnavailable
	}
	if found {
		if validateLedger(current) != nil || current.Migration == nil {
			return MigrateLocalV2Result{}, ErrConflict
		}
		if current.Migration.RequestID != request.RequestID ||
			current.Migration.PayloadHash != prepared.payloadHash {
			return MigrateLocalV2Result{}, ErrConflict
		}
		if current.Phase == PhaseCheckpointing {
			if err := s.reconcileLoaded(ctx, &current); err != nil {
				return MigrateLocalV2Result{}, err
			}
		}
		s.applyMigrationStatus(current)
		return migrateLocalV2Result(current), nil
	}

	principal, publicKey, err := s.signerBinding(ctx)
	if err != nil {
		s.setUnavailable(ReasonSignerUnavailable)
		return MigrateLocalV2Result{}, ErrUnavailable
	}
	if principal != prepared.principal || !publicKey.Equal(prepared.public) {
		return MigrateLocalV2Result{}, ErrPermissionDenied
	}
	realmID, err := s.randomID("r1_", 16)
	if err != nil {
		return MigrateLocalV2Result{}, ErrUnavailable
	}
	head, occupied, err := s.repository.ReadHead(ctx, realmID)
	if err != nil {
		s.setUnavailable(ReasonRepositoryUnavailable)
		return MigrateLocalV2Result{}, ErrUnavailable
	}
	if occupied || head.Digest != "" {
		return MigrateLocalV2Result{}, ErrConflict
	}
	operationID, err := s.randomID("rao1_", 16)
	if err != nil {
		return MigrateLocalV2Result{}, ErrUnavailable
	}
	auditID, err := s.randomID("raa1_", 16)
	if err != nil {
		return MigrateLocalV2Result{}, ErrUnavailable
	}
	now := s.clock().UTC().Truncate(time.Second)
	audit := AuditRecord{
		Version: ContractVersion, ID: auditID,
		Actor: command.Actor, Effective: command.Effective,
		Action: command.Action, ResourceKind: command.ResourceKind,
		ResourceID: command.ResourceID, OperationID: operationID,
		EvidenceDigest: migrationCommitEvidenceDigest(
			prepared.payloadHash, request.OldManagerFence.EvidenceDigest,
		),
		Outcome: "accepted", CreatedAt: now,
	}
	audit.Hash = auditHash(audit)
	checkpoint, err := SignCheckpoint(ctx, s.signer, Checkpoint{
		Version: ContractVersion, SchemaVersion: SchemaVersion,
		RealmID: realmID, AuthorityPrincipal: principal,
		AuthorityPublicKey: publicKey, AuthorityEpoch: 1,
		AuthoritySequence: 1, AuditHead: audit.Hash, CreatedAt: now,
	})
	if err != nil {
		s.setUnavailable(ReasonSignerUnavailable)
		return MigrateLocalV2Result{}, ErrUnavailable
	}
	createResult := CreateResult{
		Version: ContractVersion, RealmID: realmID, OperationID: operationID,
		AuthorityEpoch: 1, AuthoritySequence: 1,
		CheckpointDigest: checkpoint.Digest, Phase: PhaseCheckpointing,
	}
	state := Ledger{
		Version: ContractVersion, SchemaVersion: SchemaVersion, Revision: 1,
		RealmID: realmID, RealmClass: RealmClassProduction,
		AuthorityPrincipal: principal, AuthorityPublicKey: publicKey,
		AuthorityEpoch: 1, AuthoritySequence: 1,
		Phase: PhaseCheckpointing, Readiness: ReadinessDegraded,
		Reason: ReasonCheckpointMissing, AuditHead: audit.Hash,
		GenesisCheckpointDigest: checkpoint.Digest, Checkpoint: checkpoint,
		Members: prepared.members, Channels: prepared.channels,
		Operations: []OperationRecord{{
			Version: ContractVersion, ID: operationID, RequestID: request.RequestID,
			Kind: "realm_genesis", Phase: PhaseCheckpointing,
			CreatedAt: now, Deadline: now.Add(MaxOperationLifetime),
		}},
		Idempotency: []IdempotencyRecord{{
			Version: ContractVersion, RequestID: request.RequestID,
			PayloadHash: prepared.payloadHash, Result: createResult,
		}},
		AuditLog: []AuditRecord{audit}, AuditOutbox: []AuditRecord{audit},
		Migration: &MigrationRecord{
			Version: ContractVersion, SourceVersion: localV2AuthorityVersion,
			RequestID: request.RequestID, PayloadHash: prepared.payloadHash,
			OldManagerFenceDigest:      request.OldManagerFence.EvidenceDigest,
			CommitEvidenceDigest:       audit.EvidenceDigest,
			RequiredRotationChannelIDs: [][16]byte{prepared.discoveryID, prepared.dataID},
		},
	}
	sort.Slice(state.Migration.RequiredRotationChannelIDs, func(i, j int) bool {
		return bytes.Compare(
			state.Migration.RequiredRotationChannelIDs[i][:],
			state.Migration.RequiredRotationChannelIDs[j][:],
		) < 0
	})
	if err := validateLedger(state); err != nil {
		return MigrateLocalV2Result{}, ErrInvalidArgument
	}
	if err := s.store.Create(ctx, state); err != nil {
		return MigrateLocalV2Result{}, ErrConflict
	}
	if err := s.reconcileLoaded(ctx, &state); err != nil {
		return MigrateLocalV2Result{}, err
	}
	s.applyMigrationStatus(state)
	return migrateLocalV2Result(state), nil
}

func prepareLocalV2Migration(request MigrateLocalV2Request) (preparedLocalV2Migration, error) {
	if request.Version != ContractVersion {
		return preparedLocalV2Migration{}, ErrUnsupportedVersion
	}
	if len(request.RequestID) == 0 || len(request.RequestID) > MaxRequestIDBytes ||
		strings.TrimSpace(request.RequestID) != request.RequestID ||
		!request.OldManagerFence.OldProcessStopped ||
		!request.OldManagerFence.SharedAuthorityRemoved ||
		!sha256Pattern.MatchString(request.OldManagerFence.EvidenceDigest) {
		return preparedLocalV2Migration{}, ErrInvalidArgument
	}
	var legacy localV2AuthorityState
	if strictJSON(request.AuthorityState, &legacy) != nil ||
		legacy.Version != localV2AuthorityVersion ||
		len(legacy.Members) == 0 || len(legacy.Members) > MaxRealmMembers ||
		len(request.Members) != len(legacy.Members) {
		return preparedLocalV2Migration{}, ErrInvalidArgument
	}
	privateRaw, err := base64.StdEncoding.Strict().DecodeString(legacy.IssuerPrivate)
	if err != nil || len(privateRaw) != ed25519.PrivateKeySize {
		return preparedLocalV2Migration{}, ErrInvalidArgument
	}
	private := ed25519.PrivateKey(append([]byte(nil), privateRaw...))
	public := append(ed25519.PublicKey(nil), private.Public().(ed25519.PublicKey)...)
	principal, err := identityprincipal.FromEd25519PublicKey(public)
	if err != nil {
		return preparedLocalV2Migration{}, ErrInvalidArgument
	}
	discoveryID, discoverySecret, err := decodeLocalV2Channel(legacy.Discovery)
	if err != nil {
		return preparedLocalV2Migration{}, ErrInvalidArgument
	}
	dataID, dataSecret, err := decodeLocalV2Channel(legacy.Data)
	if err != nil || dataID == discoveryID {
		return preparedLocalV2Migration{}, ErrInvalidArgument
	}

	evidenceBySubject := make(map[string]LocalV2MemberEvidence, len(request.Members))
	nodeBySubject := make(map[string]localV2NodeState, len(request.Members))
	for _, evidence := range request.Members {
		var node localV2NodeState
		if strictJSON(evidence.NodeState, &node) != nil ||
			node.Version != localV2NodeVersion || node.Subject == "" ||
			len(evidence.ReceiverGrants) != 2 {
			return preparedLocalV2Migration{}, ErrInvalidArgument
		}
		if _, duplicate := evidenceBySubject[node.Subject]; duplicate {
			return preparedLocalV2Migration{}, ErrInvalidArgument
		}
		evidenceBySubject[node.Subject] = evidence
		nodeBySubject[node.Subject] = node
	}

	memberRecords := make([]MemberRecord, 0, len(legacy.Members))
	discoveryGrants := make([]CapabilityGrantRecord, 0, len(legacy.Members))
	dataGrants := make([]CapabilityGrantRecord, 0, len(legacy.Members))
	for subject, authorityNode := range legacy.Members {
		evidence, ok := evidenceBySubject[subject]
		node := nodeBySubject[subject]
		if !ok || authorityNode != node ||
			node.Subject != subject || node.Issuer != principal.String() {
			return preparedLocalV2Migration{}, ErrInvalidArgument
		}
		expected := map[identityapi.CapabilityScope]identityapi.CapabilityGrant{}
		for _, item := range []struct {
			record  localV2GrantState
			channel localV2ChannelState
			id      [16]byte
			secret  identityapi.CapabilitySecret
			scope   identityapi.CapabilityScope
		}{
			{node.Discovery, legacy.Discovery, discoveryID, discoverySecret, identityapi.CapabilityRealmDiscovery},
			{node.Data, legacy.Data, dataID, dataSecret, identityapi.CapabilityDataExchange},
		} {
			grantID, err := decodeLocalV2GrantID(item.record.ID)
			if err != nil || !canonicalSecond(item.record.NotBefore) ||
				!canonicalSecond(item.record.NotAfter) ||
				!item.record.NotBefore.Before(item.record.NotAfter) {
				return preparedLocalV2Migration{}, ErrInvalidArgument
			}
			grant, err := identitycapability.SignGrant(identityapi.CapabilityGrant{
				Version: ContractVersion, ChannelID: item.id,
				Generation: item.channel.Generation, Secret: item.secret,
				GrantID: grantID, IssuerPrincipal: principal.String(),
				SubjectPrincipal: subject,
				Permissions: identityapi.CapabilityPublish |
					identityapi.CapabilitySubscribe |
					identityapi.CapabilityStoreFetch,
				Scope: item.scope, NotBefore: item.record.NotBefore,
				NotAfter: item.record.NotAfter,
			}, private)
			if err != nil {
				return preparedLocalV2Migration{}, ErrInvalidArgument
			}
			expected[item.scope] = grant
		}
		seen := make(map[identityapi.CapabilityScope]struct{}, 2)
		for _, retained := range evidence.ReceiverGrants {
			want, ok := expected[retained.Scope]
			if !ok || !capabilityGrantsEqual(want, retained) ||
				identitycapability.VerifyGrant(retained, public) != nil {
				return preparedLocalV2Migration{}, ErrInvalidArgument
			}
			if _, duplicate := seen[retained.Scope]; duplicate {
				return preparedLocalV2Migration{}, ErrInvalidArgument
			}
			seen[retained.Scope] = struct{}{}
		}
		if len(seen) != 2 {
			return preparedLocalV2Migration{}, ErrInvalidArgument
		}
		memberRecords = append(memberRecords, MemberRecord{
			Version: ContractVersion, Principal: subject,
		})
		discoveryGrants = append(discoveryGrants, capabilityGrantRecord(expected[identityapi.CapabilityRealmDiscovery]))
		dataGrants = append(dataGrants, capabilityGrantRecord(expected[identityapi.CapabilityDataExchange]))
	}
	sort.Slice(memberRecords, func(i, j int) bool {
		return memberRecords[i].Principal < memberRecords[j].Principal
	})
	sortGrantRecords(discoveryGrants)
	sortGrantRecords(dataGrants)
	channels := []ChannelRecord{
		localV2ChannelRecord(legacy.Discovery, identityapi.CapabilityRealmDiscovery, discoveryID, discoveryGrants),
		localV2ChannelRecord(legacy.Data, identityapi.CapabilityDataExchange, dataID, dataGrants),
	}
	sort.Slice(channels, func(i, j int) bool {
		return bytes.Compare(channels[i].ID[:], channels[j].ID[:]) < 0
	})
	return preparedLocalV2Migration{
		principal: principal.String(), public: public,
		members: memberRecords, channels: channels,
		discoveryID: discoveryID, dataID: dataID,
		payloadHash: migrateLocalV2PayloadHash(request),
	}, nil
}

func strictJSON(raw []byte, target any) error {
	if len(raw) == 0 {
		return ErrInvalidArgument
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return ErrInvalidArgument
	}
	return nil
}

func decodeLocalV2Channel(
	channel localV2ChannelState,
) ([16]byte, identityapi.CapabilitySecret, error) {
	var channelID [16]byte
	rawID, idErr := base64.StdEncoding.Strict().DecodeString(channel.ID)
	rawSecret, secretErr := base64.StdEncoding.Strict().DecodeString(channel.Secret)
	secret, ok := identityapi.NewCapabilitySecret(rawSecret)
	if idErr != nil || secretErr != nil || len(rawID) != len(channelID) ||
		!ok || !secret.Valid() || channel.Generation == 0 {
		return channelID, identityapi.CapabilitySecret{}, ErrInvalidArgument
	}
	copy(channelID[:], rawID)
	if zeroFixedID(channelID) {
		return channelID, identityapi.CapabilitySecret{}, ErrInvalidArgument
	}
	return channelID, secret, nil
}

func decodeLocalV2GrantID(encoded string) ([16]byte, error) {
	var grantID [16]byte
	raw, err := base64.StdEncoding.Strict().DecodeString(encoded)
	if err != nil || len(raw) != len(grantID) {
		return grantID, ErrInvalidArgument
	}
	copy(grantID[:], raw)
	if zeroFixedID(grantID) {
		return grantID, ErrInvalidArgument
	}
	return grantID, nil
}

func capabilityGrantsEqual(left, right identityapi.CapabilityGrant) bool {
	return reflect.DeepEqual(capabilityGrantRecord(left), capabilityGrantRecord(right))
}

func sortGrantRecords(records []CapabilityGrantRecord) {
	sort.Slice(records, func(i, j int) bool {
		return records[i].SubjectPrincipal < records[j].SubjectPrincipal
	})
}

func localV2ChannelRecord(
	legacy localV2ChannelState,
	scope identityapi.CapabilityScope,
	id [16]byte,
	grants []CapabilityGrantRecord,
) ChannelRecord {
	record := ChannelRecord{
		Version: ContractVersion, ID: id, Class: string(scope),
		MemberCount: uint32(len(grants)), CurrentGeneration: legacy.Generation,
		CurrentGrants: grants,
	}
	if len(grants) > 0 {
		record.Grant = grants[0]
	}
	return record
}

func migrateLocalV2PayloadHash(request MigrateLocalV2Request) string {
	hash := sha256.New()
	_, _ = hash.Write(request.AuthorityState)
	_, _ = hash.Write([]byte(request.OldManagerFence.EvidenceDigest))
	for _, member := range request.Members {
		_, _ = hash.Write(member.NodeState)
		for _, grant := range member.ReceiverGrants {
			raw, _ := json.Marshal(capabilityGrantRecord(grant))
			_, _ = hash.Write(raw)
		}
	}
	return hex.EncodeToString(hash.Sum(nil))
}

func validateMigrateLocalV2Command(command Command) error {
	if command.Actor == "" || command.Actor != command.Effective ||
		command.Action != ActionMigrateLocalV2 ||
		command.ResourceKind != ResourceKindAuthorityInstance ||
		command.ResourceID != PrimaryAuthorityInstance {
		return ErrPermissionDenied
	}
	return nil
}

func validateMigrationRecord(state Ledger) error {
	record := state.Migration
	if record == nil {
		return nil
	}
	if record.Version != ContractVersion ||
		record.SourceVersion != localV2AuthorityVersion ||
		len(record.RequestID) == 0 || len(record.RequestID) > MaxRequestIDBytes ||
		len(record.PayloadHash) != sha256.Size*2 ||
		!sha256Pattern.MatchString(record.OldManagerFenceDigest) ||
		!sha256Pattern.MatchString(record.CommitEvidenceDigest) ||
		record.CommitEvidenceDigest != migrationCommitEvidenceDigest(
			record.PayloadHash, record.OldManagerFenceDigest,
		) ||
		len(record.RequiredRotationChannelIDs) != 2 ||
		len(record.RotatedChannelIDs) > len(record.RequiredRotationChannelIDs) {
		return ErrCorruptState
	}
	if _, err := hex.DecodeString(record.PayloadHash); err != nil {
		return ErrCorruptState
	}
	required := make(map[[16]byte]struct{}, len(record.RequiredRotationChannelIDs))
	requiredClasses := make(map[identityapi.CapabilityScope]struct{}, 2)
	for _, channelID := range record.RequiredRotationChannelIDs {
		channelIndex := channelRecordIndex(state, channelID)
		if zeroFixedID(channelID) || channelIndex < 0 {
			return ErrCorruptState
		}
		if _, duplicate := required[channelID]; duplicate {
			return ErrCorruptState
		}
		required[channelID] = struct{}{}
		class := identityapi.CapabilityScope(state.Channels[channelIndex].Class)
		if class != identityapi.CapabilityRealmDiscovery &&
			class != identityapi.CapabilityDataExchange {
			return ErrCorruptState
		}
		requiredClasses[class] = struct{}{}
	}
	if len(requiredClasses) != 2 {
		return ErrCorruptState
	}
	rotated := make(map[[16]byte]struct{}, len(record.RotatedChannelIDs))
	for _, channelID := range record.RotatedChannelIDs {
		if _, ok := required[channelID]; !ok {
			return ErrCorruptState
		}
		if _, duplicate := rotated[channelID]; duplicate {
			return ErrCorruptState
		}
		rotated[channelID] = struct{}{}
	}
	return nil
}

func migrationPending(record *MigrationRecord) bool {
	return record != nil &&
		len(record.RotatedChannelIDs) < len(record.RequiredRotationChannelIDs)
}

func migrationCommitEvidenceDigest(payloadHash, fenceDigest string) string {
	sum := sha256.Sum256([]byte(
		"ardents:local-v2-migration-evidence:v1\x00" +
			payloadHash + "\x00" + fenceDigest,
	))
	return "sha256:" + hex.EncodeToString(sum[:])
}

func (s *Service) rotationMutationFence(
	ctx context.Context,
	command Command,
	request RotationRequest,
) error {
	if s.recoveryOnly || s.status.Readiness == ReadinessRecoveryRequired {
		return ErrRecoveryRequired
	}
	if !s.migrationPending && !s.transitionPending {
		return nil
	}
	if command.Action != ActionRotateGeneration ||
		request.MembershipChange != "" || request.Renewal {
		return ErrRecoveryRequired
	}
	state, found, err := s.store.Load(ctx)
	if err != nil || !found || validateLedger(state) != nil {
		return ErrRecoveryRequired
	}
	if s.migrationPending &&
		!migrationRotationAllowed(state.Migration, request.ChannelID) {
		return ErrRecoveryRequired
	}
	if s.transitionPending &&
		!authorityTransitionRotationAllowed(state.Transition, request.ChannelID) {
		return ErrRecoveryRequired
	}
	return nil
}

func migrationRotationAllowed(record *MigrationRecord, channelID [16]byte) bool {
	if record == nil {
		return false
	}
	required := false
	for _, candidate := range record.RequiredRotationChannelIDs {
		required = required || candidate == channelID
	}
	for _, completed := range record.RotatedChannelIDs {
		if completed == channelID {
			return false
		}
	}
	return required
}

func completeMigrationRotation(state *Ledger, channelID [16]byte) {
	if state.Migration == nil ||
		!migrationRotationAllowed(state.Migration, channelID) {
		return
	}
	state.Migration.RotatedChannelIDs = append(
		state.Migration.RotatedChannelIDs, channelID,
	)
	sort.Slice(state.Migration.RotatedChannelIDs, func(i, j int) bool {
		return bytes.Compare(
			state.Migration.RotatedChannelIDs[i][:],
			state.Migration.RotatedChannelIDs[j][:],
		) < 0
	})
}

func migrateLocalV2Result(state Ledger) MigrateLocalV2Result {
	result := MigrateLocalV2Result{
		Version: ContractVersion, RealmID: state.RealmID,
		AuthorityEpoch:    state.AuthorityEpoch,
		AuthoritySequence: state.AuthoritySequence,
		CheckpointDigest:  state.Checkpoint.Digest,
		Phase:             state.Phase, Readiness: state.Readiness,
	}
	if state.Migration != nil {
		result.FreshRotationRequired = migrationPending(state.Migration)
		result.OldManagerFenceDigest = state.Migration.OldManagerFenceDigest
		for _, channel := range state.Channels {
			switch identityapi.CapabilityScope(channel.Class) {
			case identityapi.CapabilityRealmDiscovery:
				result.DiscoveryChannelID = channel.ID
			case identityapi.CapabilityDataExchange:
				result.DataChannelID = channel.ID
			}
		}
	}
	if result.FreshRotationRequired {
		result.Phase = PhaseMigrationRotationRequired
		result.Readiness = ReadinessDegraded
	}
	return result
}
