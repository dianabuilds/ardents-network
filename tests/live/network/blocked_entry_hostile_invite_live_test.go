//go:build live

package network_test

import (
	"bufio"
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	localroles "github.com/dianabuilds/ardents-network/internal/network/duty"
)

var hostileInviteTerminals = map[string]string{
	"malformed": "invalid", "non-canonical": "invalid", "oversized": "invalid",
	"duplicate-field": "invalid", "trailing-field": "invalid", "wrong-signature": "invalid",
	"wrong-network": "incompatible", "wrong-epoch": "incompatible", "wrong-profile": "incompatible",
	"expired": "expired", "not-yet-valid": "incompatible", "insufficient-time-confidence": "incompatible",
}

var hostileDomainTerminals = map[string]string{
	"responder": "wrong-domain", "introduction": "wrong-domain", "rendezvous": "wrong-domain",
	"resolution": "wrong-domain", "unknown-domain": "wrong-domain",
	"conflicting-retained-family": "conflicting-role", "direct-source-exposure": "conflicting-role",
	"interior-live-route": "conflicting-role", "drain": "conflicting-role", "quarantine": "conflicting-role",
}

func TestBlockedEntryFinalHostileInviteValidation(t *testing.T) {
	if os.Getenv("ARDENTS_BLOCKED_ROLE") != "" {
		t.Skip("host orchestrator only")
	}
	cell, variant, ok := selectedHostileInviteCell(os.Getenv("ARDENTS_FINAL_CELL"))
	if !ok {
		t.Skip("selected Invite-validation hostile cell only")
	}
	if _, err := exec.LookPath("docker"); err != nil {
		t.Fatalf("live tests require Docker: %v", err)
	}
	client := requireBlockedCandidate(t, "ARDENTS_WEBTUNNEL_CLIENT", blockedClientHash)
	server := requireBlockedCandidate(t, "ARDENTS_WEBTUNNEL_SERVER", blockedServerHash)
	repository := repositoryRoot(t)
	image, ownedImage := finalProductImage(t, fmt.Sprintf("ardents-s55-invite-%d:test", time.Now().UnixNano()))
	fixture := newBlockedEntryFixture(t, client, server)
	project := finalProjectName(fmt.Sprintf("ardents-s55-invite-%d", time.Now().UnixNano()))
	compose := blockedCompose(repository, project, image, fixture, "final-hostile")
	cleanup := blockedProjectCleanup(t, compose, project)
	t.Cleanup(cleanup)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	if ownedImage {
		if output, err := compose(ctx, "build", "endpoint"); err != nil {
			t.Fatalf("build hostile Invite product image: %v\n%s", err, output)
		}
	}
	unknownLedgerField := strings.Contains(cell, "/G9-ledger-leakage/unknown-invite-field/")
	var faultBefore, faultAfter []byte
	var stateBefore []byte
	if unknownLedgerField {
		startHostileInviteObservers(t, ctx, compose, fixture.root)
		if baseline := runHostileInviteImport(t, ctx, compose, "/run/input/import.json"); baseline != "accepted" {
			t.Fatalf("establish G9 durable baseline: %s", baseline)
		}
		faultBefore, faultAfter = prepareHostileInviteInput(t, fixture, variant)
		stateBefore = blockedStateTreeHash(t, filepath.Join(fixture.root, "state", "endpoint", "bridge"))
	} else {
		faultBefore, faultAfter = prepareHostileInviteInput(t, fixture, variant)
		startHostileInviteObservers(t, ctx, compose, fixture.root)
	}
	if variant != "insufficient-time-confidence" {
		now := time.Now()
		if err := os.Chtimes(filepath.Join(fixture.root, "input", "endpoint", "time-confidence"), now, now); err != nil {
			t.Fatal(err)
		}
	}
	started := time.Now()
	terminal := hostileInviteTerminals[variant]
	armFinalWorkerTerminal(terminal)
	observed := runHostileInviteImport(t, ctx, compose)
	if observed != terminal {
		t.Fatalf("Invite validation %s terminal=%s want=%s", variant, observed, terminal)
	}
	if stateBefore != nil {
		stateAfter := blockedStateTreeHash(t, filepath.Join(fixture.root, "state", "endpoint", "bridge"))
		receipt, err := json.Marshal(struct {
			Schema           string `json:"schema"`
			BaselineTerminal string `json:"baseline_terminal"`
			Terminal         string `json:"terminal"`
			BeforeInput      []byte `json:"before_input"`
			AfterInput       []byte `json:"after_input"`
		}{"ardents-h3-g9-unknown-invite-v1", "accepted", observed, faultBefore, faultAfter})
		if err != nil {
			t.Fatal(err)
		}
		recordFinalFault(cell, stateBefore, stateAfter, receipt)
	} else {
		recordFinalFault(cell, faultBefore, faultAfter, []byte(observed))
	}
	publishFinalWorkerTerminal()
	stopHostileInviteObservers(t, ctx, compose, fixture.root)
	cleanup()
	emitFinalWorkerCell(t, cell, terminal, started, fixture.root)
}

func TestBlockedEntryFinalHostileDomainCollision(t *testing.T) {
	if os.Getenv("ARDENTS_BLOCKED_ROLE") != "" {
		t.Skip("host orchestrator only")
	}
	cell, variant, ok := selectedHostileDomainCell(os.Getenv("ARDENTS_FINAL_CELL"))
	if !ok {
		t.Skip("selected G2 final cell only")
	}
	if _, err := exec.LookPath("docker"); err != nil {
		t.Fatalf("live tests require Docker: %v", err)
	}
	client := requireBlockedCandidate(t, "ARDENTS_WEBTUNNEL_CLIENT", blockedClientHash)
	server := requireBlockedCandidate(t, "ARDENTS_WEBTUNNEL_SERVER", blockedServerHash)
	repository := repositoryRoot(t)
	image, ownedImage := finalProductImage(t, fmt.Sprintf("ardents-s55-g2-%d:test", time.Now().UnixNano()))
	fixture := newBlockedEntryFixture(t, client, server)
	project := finalProjectName(fmt.Sprintf("ardents-s55-g2-%d", time.Now().UnixNano()))
	compose := blockedCompose(repository, project, image, fixture, "final-hostile")
	cleanup := blockedProjectCleanup(t, compose, project)
	t.Cleanup(cleanup)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	if ownedImage {
		if output, err := compose(ctx, "build", "endpoint"); err != nil {
			t.Fatalf("build G2 product image: %v\n%s", err, output)
		}
	}
	faultBefore, faultAfter := prepareHostileDomainInput(t, fixture, variant)
	startHostileInviteObservers(t, ctx, compose, fixture.root)
	started, terminal := time.Now(), hostileDomainTerminals[variant]
	armFinalWorkerTerminal(terminal)
	if observed := runHostileInviteImport(t, ctx, compose); observed != terminal {
		t.Fatalf("G2 %s terminal=%s want=%s", variant, observed, terminal)
	} else {
		recordFinalFault(cell, faultBefore, faultAfter, []byte(observed))
	}
	publishFinalWorkerTerminal()
	stopHostileInviteObservers(t, ctx, compose, fixture.root)
	cleanup()
	emitFinalWorkerCell(t, cell, terminal, started, fixture.root)
}

func selectedHostileInviteCell(cell string) (string, string, bool) {
	parts := strings.Split(cell, "/")
	if len(parts) != 4 || parts[0] != "hostile" {
		return "", "", false
	}
	episode, err := strconv.Atoi(parts[3])
	variant := parts[2]
	if parts[1] == "G6-substitution" {
		variant = mapHostileInviteSubstitution(variant)
	} else if parts[1] == "G9-ledger-leakage" {
		variant = mapHostileInviteLedgerLeakage(variant)
	} else if parts[1] != "G1-invite" {
		return "", "", false
	}
	_, known := hostileInviteTerminals[variant]
	return cell, variant, err == nil && episode >= 0 && episode < 5 && known
}

func mapHostileInviteLedgerLeakage(variant string) string {
	if variant == "unknown-invite-field" {
		return "trailing-field"
	}
	return ""
}

func mapHostileInviteSubstitution(variant string) string {
	switch variant {
	case "network":
		return "wrong-network"
	case "route-profile":
		return "wrong-profile"
	default:
		return ""
	}
}

func selectedHostileDomainCell(cell string) (string, string, bool) {
	parts := strings.Split(cell, "/")
	if len(parts) != 4 || parts[0] != "hostile" || parts[1] != "G2-domain-collision" {
		return "", "", false
	}
	episode, err := strconv.Atoi(parts[3])
	_, known := hostileDomainTerminals[parts[2]]
	return cell, parts[2], err == nil && episode >= 0 && episode < 5 && known
}

func prepareHostileInviteInput(t *testing.T, fixture blockedEntryFixture, variant string) ([]byte, []byte) {
	t.Helper()
	input := filepath.Join(fixture.root, "input", "endpoint")
	invitePath := filepath.Join(input, "invite.bin")
	raw, err := os.ReadFile(invitePath)
	if err != nil {
		t.Fatal(err)
	}
	before := bytes.Clone(raw)
	if variant == "insufficient-time-confidence" {
		stale := time.Now().Add(-time.Minute)
		if err := os.Chtimes(filepath.Join(input, "time-confidence"), stale, stale); err != nil {
			t.Fatal(err)
		}
		raw = []byte(stale.UTC().Format(time.RFC3339Nano))
	} else {
		raw = hostileInviteMutation(t, raw, variant)
		writeLiveFile(t, invitePath, raw)
	}
	plan := map[string]any{
		"state_root": "/run/state/bridge", "network_state_root": "/run/state/bridge-network",
		"invite_file": "/run/input/invite.bin", "network_id": readPlanString(t, filepath.Join(input, "import.json"), "network_id"),
		"network_authorities": readPlanValue(t, filepath.Join(input, "import.json"), "network_authorities"),
		"network_threshold":   1, "network_profile": "h3-role-probe-v1", "route_profile": "h3-route-tracer-v1",
		"local_role_state_root": "/run/state/local-roles", "time_confidence_file": "/run/input/time-confidence",
	}
	writeLivePlan(t, input, "hostile-import", plan)
	return before, raw
}

func prepareHostileDomainInput(t *testing.T, fixture blockedEntryFixture, variant string) ([]byte, []byte) {
	t.Helper()
	input := filepath.Join(fixture.root, "input", "endpoint")
	invitePath := filepath.Join(input, "invite.bin")
	raw, err := os.ReadFile(invitePath)
	if err != nil {
		t.Fatal(err)
	}
	before := bytes.Clone(raw)
	if hostileDomainTerminals[variant] == "wrong-domain" {
		body := hostileInviteBody(t, raw)
		proof := hostileInviteProofOffset(t, body)
		body[proof] ^= 1
		writeLiveFile(t, invitePath, signHostileInvite(body))
	} else {
		identity, family := hostileInviteIdentityFamily(t, raw)
		roles, openErr := localroles.Open(localroles.Config{Root: filepath.Join(input, "local-roles"), Clock: time.Now})
		if openErr != nil {
			t.Fatal(openErr)
		}
		class, state := hostileDomainDuty(variant)
		replaceErr := roles.Replace([32]byte{2}, []localroles.Duty{{Identity: identity, Family: family,
			Class: class, State: state, NotAfter: time.Now().Add(time.Hour)}})
		if closeErr := roles.Close(); replaceErr == nil {
			replaceErr = closeErr
		}
		if replaceErr != nil {
			t.Fatal(replaceErr)
		}
		raw, err = os.ReadFile(filepath.Join(input, "local-roles", "current"))
		if err != nil {
			t.Fatal(err)
		}
	}
	plan := map[string]any{"state_root": "/run/state/bridge", "network_state_root": "/run/state/bridge-network",
		"invite_file": "/run/input/invite.bin", "network_id": readPlanString(t, filepath.Join(input, "import.json"), "network_id"),
		"network_authorities": readPlanValue(t, filepath.Join(input, "import.json"), "network_authorities"),
		"network_threshold":   1, "network_profile": "h3-role-probe-v1", "route_profile": "h3-route-tracer-v1",
		"local_role_state_root": "/run/state/local-roles", "time_confidence_file": "/run/input/time-confidence"}
	writeLivePlan(t, input, "hostile-import", plan)
	return before, raw
}

func hostileInviteProofOffset(t *testing.T, body []byte) int {
	t.Helper()
	position := 74 + 1 + int(body[74]) + 1 + 32 + 32 + 32
	if position+3 > len(body) || binary.BigEndian.Uint16(body[position:position+2]) == 0 {
		t.Fatal("fixture Invite proof is invalid")
	}
	return position + 2
}

func hostileInviteIdentityFamily(t *testing.T, raw []byte) ([32]byte, [32]byte) {
	t.Helper()
	body := hostileInviteBody(t, raw)
	position := 74 + 1 + int(body[74]) + 1
	if position+64 > len(body) {
		t.Fatal("fixture Invite identity is truncated")
	}
	var identity, family [32]byte
	copy(identity[:], body[position:position+32])
	copy(family[:], body[position+32:position+64])
	return identity, family
}

func hostileDomainDuty(variant string) (string, string) {
	switch variant {
	case "direct-source-exposure":
		return "direct-source", "exposed"
	case "interior-live-route":
		return "route-interior", "live"
	case "drain":
		return "node-duty", "prepared"
	default:
		return "route-rendezvous", "quarantined"
	}
}

func runHostileInviteImport(t *testing.T, ctx context.Context, compose composeCall, plans ...string) string {
	t.Helper()
	plan := "/run/input/hostile-import.json"
	if len(plans) == 1 {
		plan = plans[0]
	} else if len(plans) != 0 {
		t.Fatal("hostile Invite import accepts at most one plan")
	}
	output, err := compose(ctx, "exec", "-T", "endpoint", "/usr/local/bin/ardents-bridge", "import", plan)
	if err != nil {
		logs, _ := compose(ctx, "logs", "--no-color", "--no-log-prefix", "endpoint", "endpoint-control")
		t.Fatalf("run hostile Invite import: %v\n%s\n%s", err, output, logs)
	}
	return decodeHostileImportClass(t, output)
}

func decodeHostileImportClass(t *testing.T, output []byte) string {
	t.Helper()
	var event struct {
		Class      string `json:"class"`
		InviteID   string `json:"invite_id"`
		Slot       uint8  `json:"slot"`
		Generation uint8  `json:"generation"`
	}
	var decoded int
	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 || line[0] != '{' {
			continue
		}
		decoder := json.NewDecoder(bytes.NewReader(line))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&event); err != nil || decoder.Decode(&struct{}{}) != io.EOF {
			t.Fatalf("decode hostile Invite result: %v\n%s", err, output)
		}
		decoded++
	}
	if err := scanner.Err(); err != nil || decoded != 1 || event.Class == "" {
		t.Fatalf("hostile Invite result count=%d: %v\n%s", decoded, err, output)
	}
	return event.Class
}

func startHostileInviteObservers(t *testing.T, ctx context.Context, compose composeCall, root string) {
	t.Helper()
	services := hostileInviteServices()
	arguments := append([]string{"up", "-d", "--no-build"}, services...)
	if output, err := compose(ctx, arguments...); err != nil {
		t.Fatalf("start G1 observers: %v\n%s", err, output)
	}
	for _, role := range hostileInviteRoles() {
		waitForBlockedHostFile(t, ctx, filepath.Join(root, "sync", role, "path-ready"))
	}
}

func stopHostileInviteObservers(t *testing.T, ctx context.Context, compose composeCall, root string) {
	t.Helper()
	for _, role := range hostileInviteRoles() {
		writeLiveFile(t, filepath.Join(root, "sync", role, "observe-stop"), []byte("stop\n"))
	}
	for _, role := range hostileInviteRoles() {
		waitForBlockedHostFile(t, ctx, filepath.Join(root, "sync", role, "path-result.json"))
		waitForBlockedHostFile(t, ctx, filepath.Join(root, "sync", role, "result.json"))
	}
	for _, service := range hostileInviteServices() {
		waitBlockedContainer(t, ctx, compose, service)
	}
}

func hostileInviteRoles() []string {
	return []string{"endpoint", "bridge", "initiator", "introduction", "rendezvous", "responder", "publisher"}
}

func hostileInviteServices() []string {
	result := make([]string, 0, 21)
	for _, role := range hostileInviteRoles() {
		result = append(result, role, role+"-observer", role+"-control")
	}
	return result
}

func hostileInviteMutation(t *testing.T, raw []byte, variant string) []byte {
	t.Helper()
	switch variant {
	case "malformed":
		return []byte("not-an-ardents-invite")
	case "oversized":
		return bytes.Repeat([]byte{'x'}, 4097)
	case "trailing-field":
		return append(bytes.Clone(raw), 0)
	case "wrong-signature":
		mutated := bytes.Clone(raw)
		mutated[len(mutated)-1] ^= 1
		return mutated
	}
	body := hostileInviteBody(t, raw)
	switch variant {
	case "non-canonical":
		body[75] = ' '
	case "duplicate-field":
		body = append(body, body[len(body)-32:]...)
	case "wrong-network":
		body[2] ^= 1
	case "wrong-epoch":
		binary.BigEndian.PutUint64(body[34:42], binary.BigEndian.Uint64(body[34:42])+1)
	case "wrong-profile":
		copy(body[75:93], "h3-route-invalidv1")
	case "expired":
		_, notAfter := hostileInviteTimeOffsets(t, body)
		binary.BigEndian.PutUint64(body[notAfter:notAfter+8], uint64(time.Now().Add(-time.Minute).Unix()))
	case "not-yet-valid":
		notBefore, _ := hostileInviteTimeOffsets(t, body)
		binary.BigEndian.PutUint64(body[notBefore:notBefore+8], uint64(time.Now().Add(time.Minute).Unix()))
	default:
		t.Fatalf("unsupported G1 mutation %q", variant)
	}
	return signHostileInvite(body)
}

func hostileInviteBody(t *testing.T, raw []byte) []byte {
	t.Helper()
	const magic = "ardents-h3-bi1"
	if len(raw) < len(magic)+2+64 {
		t.Fatal("fixture Invite is truncated")
	}
	length := int(binary.BigEndian.Uint16(raw[len(magic) : len(magic)+2]))
	start := len(magic) + 2
	if start+length+64 != len(raw) {
		t.Fatal("fixture Invite framing is invalid")
	}
	return bytes.Clone(raw[start : start+length])
}

func hostileInviteTimeOffsets(t *testing.T, body []byte) (int, int) {
	t.Helper()
	position := 74
	if position >= len(body) {
		t.Fatal("fixture Invite body is truncated")
	}
	position += 1 + int(body[position]) + 1 + 32 + 32 + 32
	if position+2 > len(body) {
		t.Fatal("fixture Invite proof is truncated")
	}
	position += 2 + int(binary.BigEndian.Uint16(body[position:position+2])) + 8
	if position+16 > len(body) {
		t.Fatal("fixture Invite interval is truncated")
	}
	return position, position + 8
}

func signHostileInvite(body []byte) []byte {
	private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x11}, ed25519.SeedSize))
	var result bytes.Buffer
	result.WriteString("ardents-h3-bi1")
	_ = binary.Write(&result, binary.BigEndian, uint16(len(body)))
	result.Write(body)
	result.Write(ed25519.Sign(private,
		append([]byte("ardents-h3-bridge-invite-signature-v1\x00"), body...)))
	return result.Bytes()
}

func readPlanString(t *testing.T, path, key string) string {
	t.Helper()
	value := readPlanValue(t, path, key)
	text, ok := value.(string)
	if !ok || text == "" {
		t.Fatalf("plan %s is not a string", key)
	}
	return text
}

func readPlanValue(t *testing.T, path, key string) any {
	t.Helper()
	raw, err := os.ReadFile(path)
	var plan map[string]any
	if err != nil || json.Unmarshal(raw, &plan) != nil {
		t.Fatalf("read fixture plan: %v", err)
	}
	value, ok := plan[key]
	if !ok {
		t.Fatalf("fixture plan omits %s", key)
	}
	return value
}
