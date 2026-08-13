package servicesmoke

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"path/filepath"

	"github.com/dianabuilds/ardents-network/internal/qualification/byteio"
	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/serviceconn"
)

type attemptEvidence struct {
	Schema                string               `json:"schema"`
	SourceCommit          string               `json:"source_commit"`
	ImageID               string               `json:"image_id"`
	ManifestDigest        string               `json:"manifest_digest"`
	EvidenceDigest        string               `json:"evidence_digest"`
	NetworkID             [32]byte             `json:"network_id"`
	AuthorityPublic       [32]byte             `json:"authority_public"`
	IntroductionPublic    [32]byte             `json:"introduction_public"`
	RouteManifestDigest   [32]byte             `json:"route_manifest_digest"`
	Target                [32]byte             `json:"target"`
	Topology              string               `json:"topology"`
	Generations           []generationEvidence `json:"generations"`
	Negatives             map[string]bool      `json:"negatives"`
	NegativeMechanisms    map[string]string    `json:"negative_mechanisms"`
	OperationObservations map[string]bool      `json:"operation_observations"`
	OperationClasses      map[string]string    `json:"operation_classes"`
	OperationCounts       map[string]uint32    `json:"operation_counts"`
	ShortcutsAbsent       map[string]bool      `json:"shortcuts_absent"`
	Cleanup               map[string]bool      `json:"cleanup"`
	PrivateMaterialAbsent bool                 `json:"private_material_absent"`
	CleanupObservation    cleanupObservation   `json:"cleanup_observation"`
}

type cleanupObservation struct {
	Observed      bool     `json:"observed"`
	Project       string   `json:"project"`
	FixtureAbsent bool     `json:"fixture_absent"`
	Containers    []string `json:"containers"`
	Networks      []string `json:"networks"`
	Volumes       []string `json:"volumes"`
}

type generationEvidence struct {
	Generation                  uint64                 `json:"generation"`
	Credential                  serviceconn.Credential `json:"credential"`
	IntroductionAcknowledgement []byte                 `json:"introduction_acknowledgement"`
	PublicationReady            bool                   `json:"publication_ready"`
	ClientEndpoint              endpointEvidence       `json:"client_endpoint"`
	PublisherEndpoint           endpointEvidence       `json:"publisher_endpoint"`
	ClientApplication           applicationEvidence    `json:"client_application"`
	PublisherApplication        applicationEvidence    `json:"publisher_application"`
	Roles                       []roleEvidence         `json:"roles"`
	ContainerIDs                []string               `json:"container_ids"`
	ClientGrant                 grantEvidence          `json:"client_grant"`
	PublisherGrant              grantEvidence          `json:"publisher_grant"`
	HostileSibling              hostileObservation     `json:"hostile_sibling"`
}

type hostileObservation struct {
	RuntimeID         string   `json:"runtime_id"`
	ExitCode          int      `json:"exit_code"`
	Running           bool     `json:"running"`
	MountDestinations []string `json:"mount_destinations"`
	Output            string   `json:"output"`
}

type grantEvidence struct {
	Broker    [32]byte `json:"broker"`
	Principal [32]byte `json:"principal"`
	Surface   string   `json:"surface"`
}

type endpointEvidence struct {
	Class                       string   `json:"class"`
	AuthenticatedTarget         [32]byte `json:"authenticated_target"`
	Generation                  uint64   `json:"generation"`
	AcceptedBytes               uint32   `json:"accepted_bytes"`
	ReceivedBytes               uint32   `json:"received_bytes"`
	ConnectionCanary            [32]byte `json:"connection_canary"`
	PrincipalCommitment         [32]byte `json:"principal_commitment"`
	SessionCommitment           [32]byte `json:"session_commitment"`
	GrantSurface                string   `json:"grant_surface"`
	SessionConsumed             bool     `json:"session_consumed"`
	BrokerCommitment            [32]byte `json:"broker_commitment"`
	GrantCommitment             [32]byte `json:"grant_commitment"`
	SessionIssuedAt             int64    `json:"session_issued_at"`
	SessionExpiresAt            int64    `json:"session_expires_at"`
	MemoryHighWater             uint64   `json:"memory_high_water"`
	CPUSeconds                  float64  `json:"cpu_seconds"`
	OpenFilesHighWater          uint32   `json:"open_files_high_water"`
	GoroutinesHighWater         uint32   `json:"goroutines_high_water"`
	ActiveSessions              uint32   `json:"active_sessions"`
	TimerHighWater              uint32   `json:"timer_high_water"`
	QueueHighWater              uint32   `json:"queue_high_water"`
	AcceptedIPCHighWater        uint32   `json:"accepted_ipc_high_water"`
	ServiceConnectionsHighWater uint32   `json:"service_connections_high_water"`
	ControlFilesHighWater       uint32   `json:"control_files_high_water"`
}

type applicationEvidence struct {
	Schema              string   `json:"schema"`
	Role                string   `json:"role"`
	Terminal            string   `json:"terminal"`
	SentBytes           uint32   `json:"sent_bytes"`
	ReceivedBytes       uint32   `json:"received_bytes"`
	SentDigest          [32]byte `json:"sent_digest"`
	ReceivedDigest      [32]byte `json:"received_digest"`
	ResultClass         string   `json:"result_class"`
	AuthenticatedTarget [32]byte `json:"authenticated_target"`
	SendSeed            [32]byte `json:"send_seed"`
	ExpectSeed          [32]byte `json:"expect_seed"`
}

type roleEvidence struct {
	Role                string                  `json:"role"`
	RuntimeID           string                  `json:"runtime_id"`
	Terminal            string                  `json:"terminal"`
	PID                 int                     `json:"pid"`
	Cleanup             bool                    `json:"cleanup"`
	ManifestDigest      [32]byte                `json:"manifest_digest"`
	NetworkID           [32]byte                `json:"network_id"`
	EpochDigest         [32]byte                `json:"epoch_digest"`
	OpaqueBytes         uint64                  `json:"opaque_bytes"`
	SourceID            string                  `json:"source_id"`
	BuildDigest         [32]byte                `json:"build_digest"`
	OpaqueDigest        [32]byte                `json:"opaque_digest"`
	ReverseOpaqueBytes  uint64                  `json:"reverse_opaque_bytes"`
	ReverseOpaqueDigest [32]byte                `json:"reverse_opaque_digest"`
	NodeID              [32]byte                `json:"node_id"`
	NextNodeID          [32]byte                `json:"next_node_id"`
	Positions           []routePositionEvidence `json:"positions,omitempty"`
}

type routePositionEvidence struct {
	Role     string   `json:"role"`
	NodeID   [32]byte `json:"node_id"`
	Endpoint string   `json:"endpoint"`
}

func writeAttempt(root string, input attemptEvidence) (string, error) {
	input.EvidenceDigest = ""
	raw, err := json.Marshal(input)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(raw)
	input.EvidenceDigest = hex.EncodeToString(digest[:])
	path := filepath.Join(root, "evidence.json")
	if err := byteio.WriteJSON(path, input, 4<<20); err != nil {
		return "", err
	}
	return path, nil
}

func terminalRoute(raw []byte) (route.Evidence, error) {
	var terminal route.Evidence
	for _, line := range splitLines(raw) {
		var value route.Evidence
		if json.Unmarshal(line, &value) == nil && value.Kind == "complete" {
			terminal = value
		}
	}
	if terminal.Kind != "complete" {
		return terminal, errors.New("route terminal evidence is missing")
	}
	return terminal, nil
}

func terminalEndpoint(raw []byte) (serviceconn.Result, error) {
	var terminal serviceconn.Result
	for _, line := range splitLines(raw) {
		var value serviceconn.Result
		if json.Unmarshal(line, &value) == nil && value.Class != "" {
			terminal = value
		}
	}
	if terminal.Class == "" {
		return terminal, errors.New("endpoint terminal evidence is missing")
	}
	return terminal, nil
}

func terminalApplication(raw []byte) (applicationEvidence, error) {
	var terminal applicationEvidence
	for _, line := range splitLines(raw) {
		if json.Unmarshal(line, &terminal) == nil && terminal.Terminal != "" {
			break
		}
	}
	if terminal.Terminal == "" {
		return terminal, errors.New("application terminal evidence is missing")
	}
	return terminal, nil
}

func splitLines(raw []byte) [][]byte {
	var lines [][]byte
	start := 0
	for index, value := range raw {
		if value == '\n' {
			if index > start {
				lines = append(lines, raw[start:index])
			}
			start = index + 1
		}
	}
	if start < len(raw) {
		lines = append(lines, raw[start:])
	}
	return lines
}
