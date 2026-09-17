//go:build linux

package main

import (
	"encoding/json"
	"net/netip"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestRunCreatesCompleteBoundedNetworkFixture(t *testing.T) {
	output := filepath.Join(t.TempDir(), "fixture")
	seed := strings.Repeat("12", 32)
	if err := run([]string{"-output", output, "-reader-host", "192.0.2.10",
		"-publisher-host", "192.0.2.20", "-carrier", "tcp-tls", "-cell", "net14ad",
		"-profile", "client-to-publisher", "-seed", seed, "-at", "2026-09-15T10:00:00Z"}); err != nil {
		t.Fatal(err)
	}
	var bundle fixtureBundle
	readFixtureJSON(t, filepath.Join(output, "fixture.json"), &bundle)
	if bundle.Schema != "ardents-qualification-network-fixture-v1" || bundle.Seed != seed ||
		len(bundle.Nodes) != 16 || len(bundle.Participants) != 5 || len(bundle.Sources) != 2 ||
		bundle.ReaderEntry == "" || bundle.ReaderInterior == "" || bundle.PublisherEntry == "" ||
		bundle.PublisherInterior == "" || bundle.DataJoin != bundle.Nodes[15].ID || bundle.NotAfter != "2026-09-15T16:00:00Z" {
		t.Fatalf("fixture bundle is incomplete: %+v", bundle)
	}
	if bundle.RemoteRoot != "/var/lib/ardents/qualification/issue60-121212121212-tcp-tls-net14ad-client-to-publisher" ||
		bundle.HostingRoot != "/var/lib/ardents/qualification/issue60-121212121212-hosting" ||
		bundle.NodeInventory != "node-inventory.json" || len(bundle.NodePlans) != 16 ||
		len(bundle.SourcePlans) != 2 || bundle.HostingRoot == "" || bundle.ClockObservationFile == "" ||
		bundle.ReaderPlanTemplate == "" || bundle.PublisherPlan == "" || bundle.Net32Plan == "" || bundle.ClosedProfilePlan == "" ||
		bundle.IssuerInitialization == "" || bundle.PreparationInventory == "" || len(bundle.ServiceInitializationPlans) != 6 {
		t.Fatalf("fixture runtime plans are incomplete: %+v", bundle)
	}
	var provision provisioningDocument
	readFixtureJSON(t, filepath.Join(output, bundle.PreparationInventory), &provision)
	if provision.Schema != "ardents-qualification-provisioning-v1" || provision.Seed != seed {
		t.Fatalf("provisioning inventory lost fixture identity: %+v", provision)
	}
	type expectedStateOwner struct {
		host, stateRoot, localRoleStateRoot, dutyParent string
		dutyRoots                                       []string
	}
	expectedState := make(map[string]expectedStateOwner, 23)
	for index, item := range bundle.Nodes {
		owner := fourDigits(index)[2:]
		base := path.Join(bundle.RemoteRoot, "duty", item.ID)
		var dutyRoots []string
		switch {
		case item.RoleDomain == 2 && item.Subrole == 6:
			dutyRoots = []string{path.Join(base, "issuer"), path.Join(base, "admission")}
		case item.RoleDomain == 2 && item.Subrole == 5:
			dutyRoots = []string{path.Join(base, "descriptors"), path.Join(base, "admission")}
		case item.RoleDomain == 4 && item.Subrole == 3:
			dutyRoots = []string{path.Join(base, "admission")}
		case item.RoleDomain == 2 && item.Subrole == 4:
			dutyRoots = []string{path.Join(base, "admission")}
		default:
			dutyRoots = []string{path.Join(base, "spends")}
		}
		expectedState["node-"+owner] = expectedStateOwner{item.Host,
			path.Join(bundle.RemoteRoot, "state", "node-"+owner), path.Join(bundle.RemoteRoot, "roles", "node-"+owner),
			base, dutyRoots}
	}
	for _, item := range bundle.Participants {
		expectedState[item.Name] = expectedStateOwner{item.Host,
			path.Join(bundle.RemoteRoot, "state", item.Name), path.Join(bundle.RemoteRoot, "state-roles", item.Name), "", nil}
	}
	for _, item := range bundle.Sources {
		expectedState[item.Name] = expectedStateOwner{item.Host,
			path.Join(bundle.RemoteRoot, "state", item.Name), path.Join(bundle.RemoteRoot, "roles", item.Name), "", nil}
	}
	if len(expectedState) != 23 || len(provision.State) != 23 {
		t.Fatalf("runtime State owner inventory is incomplete: expected=%d actual=%d", len(expectedState), len(provision.State))
	}
	for _, item := range provision.State {
		want, ok := expectedState[item.Owner]
		if !ok || item.DutyRoots == nil || item.Host != want.host || item.Root != want.stateRoot || item.LocalRoleStateRoot != want.localRoleStateRoot || item.DutyParent != want.dutyParent ||
			strings.Join(item.DutyRoots, "\x00") != strings.Join(want.dutyRoots, "\x00") {
			t.Fatalf("runtime State owner %q does not bind its exact writable roots: got=%+v want=%+v", item.Owner, item, want)
		}
		delete(expectedState, item.Owner)
	}
	if len(expectedState) != 0 {
		t.Fatalf("runtime State owners are absent from provisioning: %+v", expectedState)
	}
	materializations := make(map[string]uint64, 23)
	for _, item := range bundle.Nodes {
		materializations[item.ID] = item.MaterializationIndex
	}
	for _, item := range bundle.Participants {
		materializations[item.ID] = item.MaterializationIndex
	}
	for _, item := range bundle.Sources {
		materializations[item.StateNodeID] = item.MaterializationIndex
	}
	orderedIDs := make([]string, 0, len(materializations))
	for id := range materializations {
		orderedIDs = append(orderedIDs, id)
	}
	sort.Strings(orderedIDs)
	if len(orderedIDs) != 23 {
		t.Fatalf("materialization owner set has %d identities", len(orderedIDs))
	}
	for index, id := range orderedIDs {
		if materializations[id] != uint64(index) {
			t.Fatalf("materialization for sorted owner %s = %d, want %d", id, materializations[id], index)
		}
	}
	var inventory nodeInventoryDocument
	readFixtureJSON(t, filepath.Join(output, bundle.NodeInventory), &inventory)
	if inventory.Schema != "ardents-qualification-node-inventory-v2" || len(inventory.Nodes) != 16 || len(inventory.Sources) != 2 {
		t.Fatalf("runtime inventory is incomplete: %+v", inventory)
	}
	for _, source := range inventory.Sources {
		var plan sourcePlanDocument
		readFixtureJSON(t, filepath.Join(output, filepath.FromSlash(source.Plan)), &plan)
		if plan.Listen != source.Endpoint {
			t.Fatalf("source plan listens on %q, want inventory endpoint %q", plan.Listen, source.Endpoint)
		}
		if _, err := netip.ParseAddrPort(plan.Listen); err != nil {
			t.Fatalf("source plan listen %q is not a literal IP and port: %v", plan.Listen, err)
		}
	}
	runtimePlans := append(append([]string{}, bundle.NodePlans...), bundle.SourcePlans...)
	runtimePlans = append(runtimePlans, bundle.ServiceInitializationPlans...)
	runtimePlans = append(runtimePlans, bundle.ReaderPlanTemplate, bundle.PublisherPlan, bundle.Net32Plan, bundle.ClosedProfilePlan, bundle.IssuerInitialization, bundle.PreparationInventory)
	for _, relative := range runtimePlans {
		if info, err := os.Stat(filepath.Join(output, filepath.FromSlash(relative))); err != nil || info.Size() == 0 {
			t.Fatalf("runtime plan %s is absent: %v", relative, err)
		}
	}
	var issuerPlan struct {
		Schema      string `json:"schema"`
		Root        string `json:"root"`
		NetworkID   string `json:"network_id"`
		NodeID      string `json:"node_id"`
		IdentityKey string `json:"identity_key"`
		NotBefore   string `json:"not_before"`
		NotAfter    string `json:"not_after"`
	}
	readFixtureJSON(t, filepath.Join(output, bundle.IssuerInitialization), &issuerPlan)
	if issuerPlan.NotBefore != "2026-09-15T10:00:00Z" || issuerPlan.NotAfter != "2026-09-15T16:00:00Z" {
		t.Fatalf("closed issuer window is not an accepted six-hour boundary: %+v", issuerPlan)
	}
	duties := map[string]int{}
	privateOverrides, carrierRelays := 0, 0
	for _, relative := range bundle.NodePlans {
		var plan map[string]json.RawMessage
		readFixtureJSON(t, filepath.Join(output, filepath.FromSlash(relative)), &plan)
		selected := 0
		for _, duty := range []string{"closed_forwarding", "closed_issuer", "closed_resolution", "closed_introduction", "closed_data_join"} {
			if _, ok := plan[duty]; ok {
				duties[duty]++
				selected++
			}
		}
		if selected != 1 {
			t.Fatalf("node plan %s selected %d duties", relative, selected)
		}
		if _, ok := plan["closed_listen_private_override"]; ok {
			privateOverrides++
		}
		var forwarding map[string]json.RawMessage
		if body, ok := plan["closed_forwarding"]; ok {
			if err := json.Unmarshal(body, &forwarding); err != nil {
				t.Fatal(err)
			}
			if _, ok := forwarding["carrier_relay_endpoint"]; ok {
				carrierRelays++
			}
		}
	}
	if duties["closed_forwarding"] != 12 || duties["closed_issuer"] != 1 || duties["closed_resolution"] != 1 ||
		duties["closed_introduction"] != 1 || duties["closed_data_join"] != 1 || privateOverrides != 5 || carrierRelays != 1 {
		t.Fatalf("runtime topology mismatch: duties=%v private_overrides=%d carrier_relays=%d", duties, privateOverrides, carrierRelays)
	}
	var readerPlan, publisherPlan qualificationOwnerDocument
	readFixtureJSON(t, filepath.Join(output, bundle.ReaderPlanTemplate), &readerPlan)
	readFixtureJSON(t, filepath.Join(output, bundle.PublisherPlan), &publisherPlan)
	if len(readerPlan.Participants) != 4 || len(publisherPlan.Participants) != 1 ||
		readerPlan.Participants[0].Profile != 1 || publisherPlan.Participants[0].Profile != 1 {
		t.Fatalf("owner plans do not select the complete workload: %+v / %+v", readerPlan, publisherPlan)
	}
	var readerPermissionTotal uint64
	for _, participant := range readerPlan.Participants {
		readerPermissionTotal += qualificationPermissionTotal(t, participant, "ReaderPermission")
	}
	if readerPermissionTotal != 4096 || qualificationPermissionTotal(t, publisherPlan.Participants[0], "PublisherPermission") != 16384 {
		t.Fatalf("owner plans multiply the installation permission budget: reader=%d publisher=%d", readerPermissionTotal,
			qualificationPermissionTotal(t, publisherPlan.Participants[0], "PublisherPermission"))
	}
	var net32 qualificationOwnerDocument
	readFixtureJSON(t, filepath.Join(output, bundle.Net32Plan), &net32)
	if net32.Mode != "net32-idle" || len(net32.Participants) != 1 || net32.Participants[0].Role != 1 ||
		net32.Participants[0].Profile != 1 || net32.Participants[0].Condition != 1 || net32.Participants[0].Link != "" ||
		qualificationPermissionTotal(t, net32.Participants[0], "ReaderPermission") != 4096 {
		t.Fatalf("NET-32 plan is not the fixed User projection: %+v", net32)
	}
	var profile closedProfileTemplate
	readFixtureJSON(t, filepath.Join(output, bundle.ClosedProfilePlan), &profile)
	if profile.IssuanceAuthorityKey != "REPLACE_WITH_ADMISSION_AUTHORITY" || len(profile.Nodes) != 16 || len(profile.TokenKeys) != 0 {
		t.Fatalf("closed profile template lost its explicit late bindings: %+v", profile)
	}
	for index := 1; index < len(profile.Nodes); index++ {
		if profile.Nodes[index-1].NodeID >= profile.Nodes[index].NodeID {
			t.Fatalf("closed profile nodes are not canonical: %s then %s", profile.Nodes[index-1].NodeID, profile.Nodes[index].NodeID)
		}
	}
	var manifest networkManifest
	readFixtureJSON(t, filepath.Join(output, bundle.NetworkManifest), &manifest)
	if manifest.Version != 1 || manifest.Cell != "net14ad" || manifest.Carrier != "tcp-tls" ||
		len(manifest.Paths) != 2 || len(manifest.Relays) != 6 || len(manifest.Failures) != 0 {
		t.Fatalf("network manifest is incomplete: %+v", manifest)
	}
	for index := 0; index < 23; index++ {
		for _, relative := range []string{
			filepath.Join("state", "inputs", fourDigits(index)+".bin"),
			filepath.Join("state", "materializations", fourDigits(index)+".bin"),
		} {
			if info, err := os.Stat(filepath.Join(output, relative)); err != nil || info.Size() == 0 {
				t.Fatalf("fixture artifact %s is absent: %v", relative, err)
			}
		}
	}
	for _, participant := range bundle.Participants {
		entries, err := os.ReadDir(filepath.Join(output, participant.EntryRoot))
		if err != nil || len(entries) < 4 {
			t.Fatalf("participant Entry root %s is incomplete: %v", participant.EntryRoot, err)
		}
	}
	if err := run([]string{"-output", output}); err == nil {
		t.Fatal("generator replaced an existing private fixture")
	}
}

func qualificationPermissionTotal(t *testing.T, participant qualificationOwnerParticipant, name string) uint64 {
	t.Helper()
	body, err := json.Marshal(participant.Participant[name])
	if err != nil {
		t.Fatal(err)
	}
	var permission struct{ Maxima [3]uint32 }
	if err := json.Unmarshal(body, &permission); err != nil {
		t.Fatal(err)
	}
	return uint64(permission.Maxima[0]) + uint64(permission.Maxima[1]) + uint64(permission.Maxima[2])
}

func TestRunRejectsNonHourAlignedFixtureTime(t *testing.T) {
	err := run([]string{"-output", filepath.Join(t.TempDir(), "fixture"), "-reader-host", "192.0.2.10",
		"-publisher-host", "192.0.2.20", "-carrier", "tcp-tls", "-cell", "net14ad",
		"-profile", "client-to-publisher", "-seed", strings.Repeat("12", 32), "-at", "2026-09-15T10:01:00Z"})
	if err == nil || !strings.Contains(err.Error(), "hour-aligned") {
		t.Fatalf("non-hour fixture time error = %v", err)
	}
}

func TestRunRejectsRedundantRecoveryBaselineCell(t *testing.T) {
	output := filepath.Join(t.TempDir(), "baseline")
	if err := run([]string{"-output", output, "-reader-host", "192.0.2.10",
		"-publisher-host", "192.0.2.20", "-carrier", "tcp-tls", "-cell", "net14-recovery-baseline",
		"-profile", "client-to-publisher", "-seed", strings.Repeat("33", 32), "-at", "2026-09-15T10:00:00Z"}); err == nil {
		t.Fatal("generator accepted a duplicate recovery baseline instead of reusing NET-14AD")
	}
}

func TestRunCreatesRecoveryScheduleAndRejectsAmbiguousHosts(t *testing.T) {
	output := filepath.Join(t.TempDir(), "recovery")
	if err := run([]string{"-output", output, "-reader-host", "192.0.2.10",
		"-publisher-host", "192.0.2.20", "-carrier", "quic", "-cell", "net14-recovery",
		"-profile", "publisher-to-client", "-seed", strings.Repeat("34", 32), "-at", "2026-09-15T10:00:00Z"}); err != nil {
		t.Fatal(err)
	}
	var manifest networkManifest
	readFixtureJSON(t, filepath.Join(output, "network-manifest.json"), &manifest)
	if manifest.Carrier != "quic" || len(manifest.Failures) != 3 {
		t.Fatalf("recovery fixture has no exact schedule: %+v", manifest)
	}
	for _, relay := range manifest.Relays {
		if relay.Network != "udp" {
			t.Fatal("QUIC fixture emitted a non-UDP relay")
		}
	}
	if err := run([]string{"-output", filepath.Join(t.TempDir(), "invalid"),
		"-reader-host", "192.0.2.10", "-publisher-host", "192.0.2.10", "-carrier", "quic",
		"-cell", "net14ad", "-profile", "client-to-publisher", "-seed", strings.Repeat("56", 32), "-at", "2026-09-15T10:00:00Z"}); err == nil {
		t.Fatal("fixture accepted one machine as both owners")
	}
}

func readFixtureJSON(t *testing.T, path string, destination any) {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		t.Fatal(err)
	}
}

func fourDigits(value int) string {
	const digits = "0123456789"
	return string([]byte{digits[value/1000%10], digits[value/100%10], digits[value/10%10], digits[value%10]})
}
