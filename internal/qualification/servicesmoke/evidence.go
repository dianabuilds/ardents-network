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
	Target                [32]byte             `json:"target"`
	Generations           []generationEvidence `json:"generations"`
	Negatives             map[string]bool      `json:"negatives"`
	ShortcutsAbsent       map[string]bool      `json:"shortcuts_absent"`
	Cleanup               map[string]bool      `json:"cleanup"`
	PrivateMaterialAbsent bool                 `json:"private_material_absent"`
}

type generationEvidence struct {
	Generation                  uint64                 `json:"generation"`
	Credential                  serviceconn.Credential `json:"credential"`
	IntroductionAcknowledgement [32]byte               `json:"introduction_acknowledgement"`
	PublicationReady            bool                   `json:"publication_ready"`
	ClientEndpoint              endpointEvidence       `json:"client_endpoint"`
	PublisherEndpoint           endpointEvidence       `json:"publisher_endpoint"`
	ClientApplication           applicationEvidence    `json:"client_application"`
	PublisherApplication        applicationEvidence    `json:"publisher_application"`
	Roles                       []roleEvidence         `json:"roles"`
	ContainerIDs                []string               `json:"container_ids"`
}

type endpointEvidence struct {
	Class               string   `json:"class"`
	AuthenticatedTarget [32]byte `json:"authenticated_target"`
	Generation          uint64   `json:"generation"`
	AcceptedBytes       uint32   `json:"accepted_bytes"`
	ReceivedBytes       uint32   `json:"received_bytes"`
}

type applicationEvidence struct {
	Schema         string   `json:"schema"`
	Role           string   `json:"role"`
	Terminal       string   `json:"terminal"`
	SentBytes      uint32   `json:"sent_bytes"`
	ReceivedBytes  uint32   `json:"received_bytes"`
	SentDigest     [32]byte `json:"sent_digest"`
	ReceivedDigest [32]byte `json:"received_digest"`
}

type roleEvidence struct {
	Role      string `json:"role"`
	RuntimeID string `json:"runtime_id"`
	Terminal  string `json:"terminal"`
	PID       int    `json:"pid"`
	Cleanup   bool   `json:"cleanup"`
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
