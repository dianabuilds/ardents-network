//go:build heapdumpcapture

package endpoint

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/route/credential"
)

var heapObservationPhases = []string{"startup", "published", "withdrawn"}

type heapRoleMapReport struct {
	ReceiptSHA256 string              `json:"receiptSHA256"`
	ArtifactCount int                 `json:"artifactCount"`
	Rows          []heapRoleMapRow    `json:"rows"`
	NotObservable []heapMapLimitation `json:"notObservable"`
}

type heapRoleMapRow struct {
	Carrier    string            `json:"carrier"`
	Role       string            `json:"role"`
	Phase      string            `json:"phase"`
	Input      string            `json:"input"`
	Heap       string            `json:"heap"`
	HeapSHA256 string            `json:"heapSHA256"`
	Parsed     int               `json:"parsedBytes"`
	Categories []heapMapCategory `json:"categories"`
}

type heapMapCategory struct {
	Name         string          `json:"name"`
	OwnerField   string          `json:"ownerField"`
	MarkerSHA256 string          `json:"markerSHA256,omitempty"`
	Matches      []heapDumpMatch `json:"matches,omitempty"`
	Observation  string          `json:"observation"`
}

type heapMapLimitation struct {
	Category string `json:"category"`
	Reason   string `json:"reason"`
}

type heapRoleInput struct {
	Role         string
	Key          []byte
	Certificates [][]byte
	Snapshot     struct{ NetworkID [32]byte }
	View         struct{ Nodes json.RawMessage }
}

type heapReaderExchange struct {
	Descriptor []byte
	Permission []byte
	Request    []byte
}

type heapMapMarker struct {
	name, ownerField, observation string
	value                         []byte
}

func TestHeapDumpRoleMapObservation(t *testing.T) {
	root, output := os.Getenv("ARDENTS_HEAPDUMP_INPUT_ROOT"), os.Getenv("ARDENTS_HEAPDUMP_ROLE_MAP")
	if root == "" && output == "" {
		return
	}
	if !filepath.IsAbs(root) || !filepath.IsAbs(output) {
		t.Fatal("heap dump input root and role-map path must be absolute")
	}
	manifest, receipt, err := verifiedObservationManifest(root, true)
	if err != nil {
		t.Fatal(err)
	}
	report := heapRoleMapReport{ReceiptSHA256: receipt, ArtifactCount: len(manifest.Artifacts), NotObservable: []heapMapLimitation{
		{Category: "runtime-type", Reason: "Go heap dumps do not attach a runtime type record to every object; direct matches cannot establish every owning type."},
		{Category: "peer-channel-metadata", Reason: "View.Nodes is bound as a role-input field, but the heap format has no object-type association for proving a specific channel owner."},
		{Category: "retention", Reason: "A point-in-time heap dump cannot establish durable retention or post-stop deletion; T03 records each role's durable root separately."},
	}}
	carriers, err := heapObservationCarriers(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, carrier := range carriers {
		controls, err := heapCarrierMarkers(root, carrier)
		if err != nil {
			t.Fatal(err)
		}
		roles, err := heapCarrierRoles(root, carrier)
		if err != nil {
			t.Fatal(err)
		}
		for _, role := range roles {
			input, err := heapRoleInputAt(filepath.Join(root, carrier, role, "input.json"))
			if err != nil {
				t.Fatal(err)
			}
			if input.Role == "" || !strings.HasSuffix(role, "-"+input.Role) {
				t.Fatalf("role input %s does not bind its directory", filepath.Join(carrier, role))
			}
			markers := append([]heapMapMarker{}, controls...)
			markers = append(markers, heapMapMarker{name: "own_identity_key", ownerField: "input.json:Key", observation: "exact child identity-key bytes", value: input.Key})
			markers = append(markers, heapMapMarker{name: "role_certificate", ownerField: "input.json:Certificates[0]", observation: "exact first role certificate bytes", value: input.Certificates[0]})
			markers = append(markers, heapMapMarker{name: "public_state_network", ownerField: "input.json:Snapshot.NetworkID", observation: "exact public State network identifier", value: input.Snapshot.NetworkID[:]})
			for _, phase := range heapObservationPhases {
				row, err := heapRolePhase(root, carrier, role, phase, markers, len(input.View.Nodes) > 0)
				if err != nil {
					t.Fatal(err)
				}
				report.Rows = append(report.Rows, row)
			}
		}
	}
	if len(report.Rows) != 90 {
		t.Fatalf("role-phase rows = %d, want 90", len(report.Rows))
	}
	encoded, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(output, append(encoded, '\n'), 0600); err != nil {
		t.Fatal(err)
	}
}

func heapObservationCarriers(root string) ([]string, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	var carriers []string
	for _, entry := range entries {
		if entry.IsDir() && strings.HasPrefix(entry.Name(), "ardents-carrier-") {
			carriers = append(carriers, entry.Name())
		}
	}
	sort.Strings(carriers)
	if len(carriers) != 2 {
		return nil, fmt.Errorf("carrier directories = %d, want 2", len(carriers))
	}
	return carriers, nil
}

func heapCarrierRoles(root, carrier string) ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(root, carrier))
	if err != nil {
		return nil, err
	}
	var roles []string
	for _, entry := range entries {
		if entry.IsDir() {
			roles = append(roles, entry.Name())
		}
	}
	sort.Strings(roles)
	if len(roles) != 15 {
		return nil, fmt.Errorf("%s role directories = %d, want 15", carrier, len(roles))
	}
	return roles, nil
}

func heapCarrierMarkers(root, carrier string) ([]heapMapMarker, error) {
	target, err := observationTarget(filepath.Join(root, carrier, "reader-input.json"))
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(filepath.Join(root, carrier, "reader-exchange.json"))
	if err != nil {
		return nil, err
	}
	var exchange heapReaderExchange
	if err := json.Unmarshal(raw, &exchange); err != nil {
		return nil, err
	}
	if _, err := credential.DecodePermission(exchange.Permission); err != nil {
		return nil, fmt.Errorf("%s issuer permission: %w", carrier, err)
	}
	slot, err := heapDescriptorSlot(exchange.Descriptor)
	if err != nil {
		return nil, fmt.Errorf("%s introduction slot: %w", carrier, err)
	}
	return []heapMapMarker{
		{name: "gateway_target", ownerField: "reader-input.json:Target", observation: "exact Target selected for the Gateway lookup", value: target[:]},
		{name: "introduction_slot", ownerField: "reader-exchange.json:Descriptor.Private.Slot", observation: "exact live recipient Introduction slot", value: slot},
		{name: "issuer_permission", ownerField: "reader-exchange.json:Permission", observation: "exact canonical issuer permission bytes", value: exchange.Permission},
	}, nil
}

func heapDescriptorSlot(raw []byte) ([]byte, error) {
	const header, signature, slotOffset = 284, 64, 202
	if len(raw) <= header+signature || binary.BigEndian.Uint16(raw[:2]) != 3 {
		return nil, errors.New("private Descriptor framing is invalid")
	}
	length := int(binary.BigEndian.Uint16(raw[header-2 : header]))
	if header+length+signature != len(raw) {
		return nil, errors.New("private Descriptor proof length is invalid")
	}
	return append([]byte(nil), raw[slotOffset:slotOffset+32]...), nil
}

func heapRoleInputAt(path string) (heapRoleInput, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return heapRoleInput{}, err
	}
	var input heapRoleInput
	if err := json.Unmarshal(raw, &input); err != nil {
		return heapRoleInput{}, err
	}
	if len(input.Key) == 0 || len(input.Certificates) == 0 || input.Snapshot.NetworkID == [32]byte{} {
		return heapRoleInput{}, errors.New("role input lacks required key, certificate, or public State")
	}
	return input, nil
}

func heapRolePhase(root, carrier, role, phase string, markers []heapMapMarker, hasPeers bool) (heapRoleMapRow, error) {
	heapPath := filepath.Join(root, carrier, role, phase+".heap")
	raw, err := os.ReadFile(heapPath)
	if err != nil {
		return heapRoleMapRow{}, err
	}
	digest := sha256.Sum256(raw)
	row := heapRoleMapRow{Carrier: carrier, Role: role, Phase: phase, Input: filepath.Join(carrier, role, "input.json"), Heap: filepath.Join(carrier, role, phase+".heap"), HeapSHA256: hex.EncodeToString(digest[:])}
	for _, marker := range markers {
		if len(marker.value) == 0 {
			return heapRoleMapRow{}, fmt.Errorf("%s has empty marker", marker.name)
		}
		parsed, err := parseHeapDump(raw, marker.value)
		if err != nil {
			return heapRoleMapRow{}, fmt.Errorf("%s %s %s: %w", carrier, role, phase, err)
		}
		row.Parsed = parsed.ParsedBytes
		row.Categories = append(row.Categories, heapMapCategory{Name: marker.name, OwnerField: marker.ownerField, MarkerSHA256: parsed.TargetSHA256, Matches: parsed.Matches, Observation: marker.observation})
	}
	peerObservation := "View.Nodes is absent from the role input"
	if hasPeers {
		peerObservation = "peer/channel metadata is supplied through input.json:View.Nodes; see report limitation for typed heap ownership"
	}
	row.Categories = append(row.Categories,
		heapMapCategory{Name: "peer_channel_metadata", OwnerField: "input.json:View.Nodes", Observation: peerObservation},
		heapMapCategory{Name: "retention", OwnerField: "no heap field", Observation: "not observable from a point-in-time heap; deferred to durable-root capture T03"},
	)
	return row, nil
}
