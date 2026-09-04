package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/entry"
	localroles "github.com/dianabuilds/ardents-network/internal/network/duty"
	"github.com/dianabuilds/ardents-network/internal/network/state"
)

func TestHeadlessRuntimeRetainsNetworkOwnersAndBothApplicationSurfacesUntilStop(t *testing.T) {
	directory := t.TempDir()
	now := time.Now().UTC().Truncate(time.Second)
	network := prepareCommandNetwork(t, directory, now, "ardents-interactive-route-v1")
	confidence := filepath.Join(directory, "time-confidence")
	if err := os.WriteFile(confidence, []byte("observed\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	rolesRoot := filepath.Join(directory, "local-roles")
	roles, err := localroles.Open(localroles.Config{Root: rolesRoot, Clock: time.Now, Create: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := roles.Close(); err != nil {
		t.Fatal(err)
	}
	entryRoot := filepath.Join(directory, "entry")
	importRuntimeEntry(t, entryRoot, rolesRoot, confidence, network)
	planPath := filepath.Join(directory, "headless-runtime.json")
	applicationSocket := filepath.Join(os.TempDir(), fmt.Sprintf("ahi-%d.sock", time.Now().UnixNano()))
	administrationSocket := filepath.Join(os.TempDir(), fmt.Sprintf("ahs-%d.sock", time.Now().UnixNano()))
	t.Cleanup(func() {
		_ = os.Remove(applicationSocket)
		_ = os.Remove(administrationSocket)
	})
	plan := map[string]any{
		"schema":                   "ardents-headless-runtime-v1",
		"network_state_root":       network.root,
		"entry_state_root":         entryRoot,
		"transit_acquisition_root": filepath.Join(directory, "transit-acquisition"),
		"application_socket":       applicationSocket,
		"administration_socket":    administrationSocket,
		"publication_root":         filepath.Join(directory, "publication"),
		"local_role_state_root":    rolesRoot,
		"time_confidence_file":     confidence,
		"network_id":               hex32(network.snapshot.NetworkID),
		"network_authorities":      []string{hex.EncodeToString(network.authorityPublic)},
		"network_threshold":        1,
		"network_profile":          "ardents-interactive-route-v1",
		"broker_id":                hex32([32]byte{71}),
		"connection_principal":     hex32([32]byte{72}),
		"administration_principal": hex32([32]byte{74}),
		"bytes_each_direction":     4096,
	}
	rawPlan, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(planPath, rawPlan, 0o600); err != nil {
		t.Fatal(err)
	}
	// Import and corpus preparation may legitimately exceed the two-second
	// operator-observation window under a parallel quality run. Record the
	// fresh observation for the runtime operation itself.
	if err := os.WriteFile(confidence, []byte("observed\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	writer := &headlessRuntimeWriter{ready: make(chan struct{}, 1)}
	done := make(chan error, 1)
	go func() {
		done <- runHeadlessRuntime(ctx, planPath, writer)
	}()
	select {
	case <-writer.ready:
	case err := <-done:
		t.Fatalf("headless runtime stopped before ready: %v", err)
	case <-time.After(15 * time.Second):
		t.Fatal("headless runtime did not become ready within the bounded test window")
	}
	if _, err := os.Stat(applicationSocket); err != nil {
		t.Fatalf("headless Application socket was not published: %v", err)
	}
	if _, err := os.Stat(administrationSocket); err != nil {
		t.Fatalf("headless Service Administration socket was not published: %v", err)
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(applicationSocket); !os.IsNotExist(err) {
		t.Fatalf("headless Application socket remained after runtime stop: %v", err)
	}
	if _, err := os.Stat(administrationSocket); !os.IsNotExist(err) {
		t.Fatalf("headless Service Administration socket remained after runtime stop: %v", err)
	}
	var events []headlessRuntimeEvent
	for _, line := range bytes.Split(bytes.TrimSpace(writer.Bytes()), []byte{'\n'}) {
		var event headlessRuntimeEvent
		if err := json.Unmarshal(line, &event); err != nil {
			t.Fatal(err)
		}
		events = append(events, event)
	}
	if len(events) != 2 || events[0].Kind != "headless-runtime-ready" || events[1].Kind != "headless-runtime-stopped" ||
		events[0].NetworkID != hex32(network.snapshot.NetworkID) || events[1].NetworkID != events[0].NetworkID ||
		events[0].ApplicationSocket != applicationSocket || events[1].ApplicationSocket != "" ||
		events[0].AdministrationSocket != administrationSocket || events[1].AdministrationSocket != "" {
		t.Fatalf("headless runtime events = %+v", events)
	}
}

func TestHeadlessRuntimePlanRejectsRouteAuthorityInput(t *testing.T) {
	path := filepath.Join(t.TempDir(), "runtime.json")
	if err := os.WriteFile(path, []byte(`{"schema":"ardents-headless-runtime-v1","target":"forbidden"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadHeadlessRuntimePlan(path); err == nil {
		t.Fatal("headless runtime plan accepted a route authority field")
	}
}

func TestHeadlessRuntimeSourcePlanMustShareItsStateOwners(t *testing.T) {
	directory := t.TempDir()
	sharedRoles, sharedClock := filepath.Join(directory, "roles"), filepath.Join(directory, "clock")
	public := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{9}, ed25519.SeedSize)).Public().(ed25519.PublicKey)
	identity := [32]byte{81}
	plan := decodedHeadlessRuntimePlan{NetworkID: [32]byte{80}, NetworkAuthorities: map[[32]byte]ed25519.PublicKey{identity: public},
		headlessRuntimePlan: headlessRuntimePlan{NetworkThreshold: 1, LocalRoleStateRoot: sharedRoles, TimeConfidenceFile: sharedClock}}
	matching := state.Config{NetworkID: plan.NetworkID, Authorities: map[[32]byte]ed25519.PublicKey{identity: public}, Threshold: 1,
		LocalRoleStateRoot: sharedRoles, ClockObservationFile: sharedClock, AutomaticRefreshInterval: time.Second}
	if !matchesHeadlessSourcePlan(plan, matching) {
		t.Fatal("matching source plan was rejected")
	}
	matching.LocalRoleStateRoot = filepath.Join(directory, "other-roles")
	if matchesHeadlessSourcePlan(plan, matching) {
		t.Fatal("source plan with a different local duty owner was accepted")
	}
	matching.LocalRoleStateRoot, matching.ClockObservationFile = sharedRoles, filepath.Join(directory, "other-clock")
	if matchesHeadlessSourcePlan(plan, matching) {
		t.Fatal("source plan with a different clock owner was accepted")
	}
	matching.ClockObservationFile, matching.AutomaticRefreshInterval = sharedClock, 0
	if matchesHeadlessSourcePlan(plan, matching) {
		t.Fatal("source plan without automatic refresh was accepted")
	}
}

func importRuntimeEntry(t *testing.T, root, rolesRoot, confidence string, network commandNetwork) {
	t.Helper()
	opened, err := state.Open(state.Config{Root: network.root, NetworkID: network.snapshot.NetworkID,
		Authorities: map[[32]byte]ed25519.PublicKey{network.snapshot.EpochAuthorityIDs[0]: network.authorityPublic},
		Threshold:   1, AcceptedProfile: "ardents-interactive-route-v1", Clock: time.Now})
	if err != nil {
		t.Fatal(err)
	}
	defer opened.Close()
	owner, err := entry.Open(entry.Config{Root: root, Current: func() (entry.View, error) {
		current, currentErr := opened.Current()
		if currentErr != nil {
			return entry.View{}, currentErr
		}
		return entryView(current), nil
	}, Conflict: func(identity, family [32]byte) (bool, error) {
		return localroles.ReadConflict(rolesRoot, time.Now, identity, family)
	}, Clock: time.Now, TimeConfident: freshOperatorRegularFile(confidence, time.Now, 2*time.Second)})
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Close()
	result, err := owner.Import(commandInvite(network, time.Now().UTC()))
	if err != nil || result.Class != entry.Accepted {
		t.Fatalf("import runtime Entry = %+v, %v", result, err)
	}
}

type headlessRuntimeWriter struct {
	mu    sync.Mutex
	bytes bytes.Buffer
	ready chan struct{}
}

func (writer *headlessRuntimeWriter) Write(value []byte) (int, error) {
	writer.mu.Lock()
	_, err := writer.bytes.Write(value)
	writer.mu.Unlock()
	if err == nil && bytes.Contains(value, []byte("headless-runtime-ready")) {
		select {
		case writer.ready <- struct{}{}:
		default:
		}
	}
	return len(value), err
}

func (writer *headlessRuntimeWriter) Bytes() []byte {
	writer.mu.Lock()
	defer writer.mu.Unlock()
	return append([]byte(nil), writer.bytes.Bytes()...)
}
