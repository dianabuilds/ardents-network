package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

// Public consistency is independent of the outside controller's native custody
// and the Product Owner's separate bootstrap decision.
type forwardingBootstrapDocument struct {
	Schema         string                       `json:"schema"`
	CaseInputs     map[string]map[string]string `json:"case_inputs"`
	SourceManifest string                       `json:"source_manifest_sha256"`
	PinnedSource   string                       `json:"source_manifest"`
	Artifacts      map[string]string            `json:"artifacts"`
	Cases          []string                     `json:"cases"`
	RuntimeSources map[string]string            `json:"runtime_source_manifests"`
	SelectedCase   string                       `json:"selected_case"`
	NativeStand    map[string]string            `json:"native_stand"`
	Controllers    map[string]string            `json:"controllers"`
	Campaign       string                       `json:"campaign_sha256"`
}

func forwardingBootstrapHash(value string) bool {
	return len(value) == 64 && strings.Trim(value, "0123456789abcdef") == ""
}

func forwardingBootstrapStandName(name string) bool {
	number, found := strings.CutPrefix(name, "installed")
	return found && len(number) > 0 && number[0] >= '1' && number[0] <= '9' && strings.Trim(number, "0123456789") == ""
}

func forwardingBootstrapCampaign(receipt forwardingBootstrapDocument) (string, error) {
	// Every value is already a fixed ASCII name or lowercase digest. Requiring
	// printable ASCII artifact names also keeps Python's ensure_ascii encoding
	// and Go's non-HTML-escaped JSON representation identical.
	for name := range receipt.Artifacts {
		for _, char := range name {
			if char < 32 || char > 126 {
				return "", errors.New("non-ASCII artifact declaration")
			}
		}
	}
	scope := map[string]any{
		"source_manifest_sha256":   receipt.SourceManifest,
		"runtime_source_manifests": receipt.RuntimeSources,
		"artifacts":                receipt.Artifacts,
		"cases":                    receipt.Cases,
		"controllers":              receipt.Controllers,
	}
	var raw bytes.Buffer
	encoder := json.NewEncoder(&raw)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(scope); err != nil {
		return "", err
	}
	sum := sha256.Sum256(bytes.TrimSuffix(raw.Bytes(), []byte("\n")))
	return hex.EncodeToString(sum[:]), nil
}

func forwardingInstalledBootstrapScope(raw, pinnedRaw []byte, caseName string) (forwardingBootstrapDocument, error) {
	var receipt, pins forwardingBootstrapDocument
	if json.Unmarshal(raw, &receipt) != nil || json.Unmarshal(pinnedRaw, &pins) != nil {
		return receipt, errors.New("original bootstrap and artifact declarations required")
	}
	wantCases := []string{"lifetime-refill", "retirement-history-missing", "retirement-history-damaged", "unknown-domain-join"}
	required := []string{"ardents-node", "ardents-control", "forwarding-installed.test"}
	switch receipt.Schema {
	case "ardents-forwarding-bootstrap-receipt-v1":
	case "ardents-forwarding-bootstrap-receipt-v2", "ardents-forwarding-bootstrap-receipt-v3":
		wantCases = forwardingInstalledTransitionCases()
		required = append(required, "ardents-node-successor", "ardents-control-successor", "forwarding-control.test")
		if len(receipt.RuntimeSources) != 2 || !maps.Equal(receipt.RuntimeSources, pins.RuntimeSources) ||
			!forwardingBootstrapHash(receipt.RuntimeSources["initial"]) ||
			!forwardingBootstrapHash(receipt.RuntimeSources["successor"]) ||
			receipt.RuntimeSources["initial"] == receipt.RuntimeSources["successor"] ||
			pins.Artifacts["ardents-node"] == pins.Artifacts["ardents-node-successor"] {
			return receipt, errors.New("genuine distinct program and exact runtime sources required")
		}
	default:
		return receipt, errors.New("Forwarding bootstrap schema differs")
	}
	if !forwardingBootstrapHash(receipt.SourceManifest) || receipt.SourceManifest != pins.PinnedSource ||
		!maps.Equal(receipt.Artifacts, pins.Artifacts) || !slices.Equal(receipt.Cases, wantCases) ||
		!slices.Contains(wantCases, caseName) {
		return receipt, errors.New("actual source, artifact or complete catalog differs")
	}
	for _, name := range required {
		if !forwardingBootstrapHash(receipt.Artifacts[name]) {
			return receipt, fmt.Errorf("required actual artifact absent: %s", name)
		}
	}
	for _, sum := range receipt.Artifacts {
		if !forwardingBootstrapHash(sum) {
			return receipt, errors.New("malformed actual artifact digest")
		}
	}
	prepared := wantCases
	if receipt.Schema == "ardents-forwarding-bootstrap-receipt-v3" {
		if receipt.SelectedCase != caseName || pins.SelectedCase != caseName ||
			len(receipt.NativeStand) != 2 || !maps.Equal(receipt.NativeStand, pins.NativeStand) ||
			!forwardingBootstrapStandName(receipt.NativeStand["case"]) ||
			!forwardingBootstrapHash(receipt.NativeStand["reservation_sha256"]) ||
			len(receipt.Controllers) != 3 || !maps.Equal(receipt.Controllers, pins.Controllers) {
			return receipt, errors.New("serial original stand or controller declarations differ")
		}
		for _, name := range []string{"controller", "worker", "input_preparer"} {
			if !forwardingBootstrapHash(receipt.Controllers[name]) {
				return receipt, errors.New("complete serial controller declarations required")
			}
		}
		campaign, err := forwardingBootstrapCampaign(receipt)
		if err != nil || receipt.Campaign != campaign || pins.Campaign != campaign {
			return receipt, errors.New("complete serial campaign commitment differs")
		}
		prepared = []string{caseName}
	}
	if len(receipt.CaseInputs) != len(prepared) {
		return receipt, errors.New("prepared case set differs")
	}
	for _, name := range prepared {
		names := forwardingInstalledInputNames(name)
		if len(receipt.CaseInputs[name]) != len(names) {
			return receipt, errors.New("incomplete case-specific bootstrap inputs")
		}
		for _, input := range names {
			if !forwardingBootstrapHash(receipt.CaseInputs[name][input]) {
				return receipt, errors.New("case-specific bootstrap digest differs")
			}
		}
	}
	return receipt, nil
}

func forwardingInstalledValidateBootstrap(raw, pinnedRaw []byte, caseName string, inputs map[string][]byte) error {
	receipt, err := forwardingInstalledBootstrapScope(raw, pinnedRaw, caseName)
	if err != nil {
		return err
	}
	if len(inputs) != len(forwardingInstalledInputNames(caseName)) {
		return errors.New("actual prepared input set differs")
	}
	for name, body := range inputs {
		sum := sha256.Sum256(body)
		if receipt.CaseInputs[caseName][name] != hex.EncodeToString(sum[:]) {
			return errors.New("actual prepared input differs from reviewed receipt")
		}
	}
	return nil
}

// The non-root peer observes only its original public plan and certificate;
// it cannot open the root driver's private presentation inputs. It independently
// validates the entire public receipt scope before comparing either exact body.
func forwardingInstalledValidatePeerBootstrap(raw, pinnedRaw []byte, caseName, name string, body []byte) error {
	receipt, err := forwardingInstalledBootstrapScope(raw, pinnedRaw, caseName)
	if err != nil {
		return err
	}
	if name != "peer_plan" && name != "peer_certificate" {
		return errors.New("only original peer public plan or certificate may be checked")
	}
	sum := sha256.Sum256(body)
	if receipt.CaseInputs[caseName][name] != hex.EncodeToString(sum[:]) {
		return errors.New("original peer public input differs from reviewed receipt")
	}
	return nil
}

func forwardingInstalledTransitionCases() []string {
	cases := []string{"lifetime-refill", "retirement-history-missing", "retirement-history-damaged", "unknown-domain-join", "replacement-lifetime-refill", "replacement-unknown-domain-join", "replacement-changed-account", "replacement-stale-authority"}
	for _, role := range []string{"issuer", "descriptor", "introduction", "join"} {
		for _, kind := range []string{"service", "socket"} {
			cases = append(cases, "replacement-competing-"+role+"-"+kind)
		}
	}
	for _, group := range []struct {
		prefix string
		count  int
	}{{"initial", 120}, {"replacement", 62}, {"start-initial", 26}, {"start-selected", 26}, {"restart", 26}, {"retirement-initial", 18}, {"retirement-selected", 18}} {
		for number := 1; number <= group.count; number++ {
			cases = append(cases, fmt.Sprintf("%s-cut-%03d", group.prefix, number))
		}
	}
	return cases
}

func forwardingInstalledInputNames(caseName string) []string {
	if strings.HasPrefix(caseName, "initial-cut-") {
		return []string{"initial_cut_inputs"}
	}
	names := []string{"binding", "plan", "bundle", "profile", "presentations", "peer_plan", "peer_presentations", "peer_certificate"}
	if (caseName == "replacement-lifetime-refill" || caseName == "replacement-unknown-domain-join" || caseName == "replacement-changed-account" || caseName == "replacement-stale-authority") || strings.HasPrefix(caseName, "replacement-competing-") || strings.HasPrefix(caseName, "replacement-cut-") || strings.HasPrefix(caseName, "start-selected-cut-") || strings.HasPrefix(caseName, "restart-cut-") || strings.HasPrefix(caseName, "retirement-selected-cut-") {
		names = append(names, "successors")
	}
	return names
}

// Exercise the parser called by every actual installed entrypoint, using bytes
// independently produced and checked by the Python runner. No native setup,
// installed execution or Product Owner decision is supplied by these fixtures.
func TestForwardingBootstrapReceiptConsumerMatchesRunner(t *testing.T) {
	python := "python3"
	if runtime.GOOS == "windows" {
		python = "python"
	}
	command := exec.CommandContext(t.Context(), python, "-I", "-S", "-c", `
import base64, copy, hashlib, json, runpy, sys
tool=runpy.run_path(sys.argv[1])
artifacts={"ardents-node":"a"*64,"ardents-control":"b"*64,"forwarding-installed.test":"c"*64,
           "ardents-node-successor":"e"*64,"ardents-control-successor":"f"*64,"forwarding-control.test":"1"*64}
sources={"initial":"2"*64,"successor":"3"*64}
controllers={"controller":"4"*64,"worker":"5"*64,"input_preparer":"6"*64}
catalog=list(tool["CASES"])
rows=[]
def emit(label, receipt, pins, case, bodies):
    actual={name:hashlib.sha256(body).hexdigest() for name,body in bodies.items()}
    valid=tool["matches_scope"](receipt,pins,"d"*64) and tool["matches_inputs"](receipt,case,actual)
    rows.append(dict(label=label,receipt=receipt,pins=pins,case=case,valid=valid,
                     inputs={name:base64.b64encode(body).decode() for name,body in bodies.items()}))
for version in (1,2,3):
    cases=list(tool["V1_CASES"]) if version==1 else catalog
    selected=cases if version==3 else [cases[0],cases[-1]]
    for number,case in enumerate(selected,1):
        all_bodies={name:{key:(name+key).encode() for key in tool["input_names"](name)} for name in ([case] if version==3 else cases)}
        bodies=all_bodies[case]
        receipt=dict(schema=f"ardents-forwarding-bootstrap-receipt-v{version}",source_manifest_sha256="d"*64,
                     runtime_source_manifests=sources.copy(),artifacts=artifacts.copy(),cases=cases.copy(),
                     case_inputs={name:{key:hashlib.sha256(body).hexdigest() for key,body in inputs.items()} for name,inputs in all_bodies.items()})
        pins=dict(source_manifest="d"*64,runtime_source_manifests=sources.copy(),artifacts=artifacts.copy())
        if version==3:
            receipt.update(selected_case=case,native_stand=dict(case=f"installed{number}",reservation_sha256="7"*64),controllers=controllers.copy())
            receipt["campaign_sha256"]=tool["campaign_digest"](receipt)
            for key in ("selected_case","native_stand","controllers","campaign_sha256"): pins[key]=copy.deepcopy(receipt[key])
        emit(f"v{version}-{case}",receipt,pins,case,bodies)
        if version==3:
            for field,value in (("selected_case","unprepared"),("native_stand",{}),("controllers",{}),("campaign_sha256","0"*64),("case_inputs",{}),("cases",[case])):
                changed=copy.deepcopy(receipt);changed[field]=value
                emit(f"bad-{field}-{case}",changed,pins,case,bodies)
            other=catalog[(catalog.index(case)+1)%len(catalog)]
            emit(f"foreign-case-{case}",receipt,pins,other,bodies)
            changed=copy.deepcopy(pins);changed["native_stand"]["reservation_sha256"]="0"*64
            emit(f"foreign-stand-{case}",receipt,changed,case,bodies)
            changed=dict(bodies);changed[next(iter(changed))]=b"changed original bytes"
            emit(f"changed-bytes-{case}",receipt,pins,case,changed)
json.dump(rows,sys.stdout,separators=(",",":"))
`, filepath.Join("..", "..", "scripts", "installation", "run-forwarding-installed.py"))
	raw, err := command.Output()
	if err != nil {
		t.Fatal("independent runner scope fixtures", err)
	}
	var rows []struct {
		Label   string            `json:"label"`
		Receipt json.RawMessage   `json:"receipt"`
		Pins    json.RawMessage   `json:"pins"`
		Case    string            `json:"case"`
		Inputs  map[string][]byte `json:"inputs"`
		Valid   bool              `json:"valid"`
	}
	if err := json.Unmarshal(raw, &rows); err != nil || len(rows) != 3120+4 {
		t.Fatal("complete runner/consumer control inventory", len(rows), err)
	}
	for _, row := range rows {
		if err := forwardingInstalledValidateBootstrap(row.Receipt, row.Pins, row.Case, row.Inputs); (err == nil) != row.Valid {
			t.Errorf("%s: consumer error=%v, runner valid=%v", row.Label, err, row.Valid)
		}
		if row.Valid {
			for _, name := range []string{"peer_plan", "peer_certificate"} {
				body, exists := row.Inputs[name]
				if err := forwardingInstalledValidatePeerBootstrap(row.Receipt, row.Pins, row.Case, name, body); (err == nil) != exists {
					t.Errorf("%s/%s: peer error=%v, input present=%v", row.Label, name, err, exists)
				}
				if exists && forwardingInstalledValidatePeerBootstrap(row.Receipt, row.Pins, row.Case, name, append([]byte("changed"), body...)) == nil {
					t.Errorf("%s/%s: changed peer input accepted", row.Label, name)
				}
			}
		}
	}
}

func TestForwardingBootstrapOriginalPeerPublicInputs(t *testing.T) {
	receipt := forwardingBootstrapDocument{
		Schema:         "ardents-forwarding-bootstrap-receipt-v1",
		SourceManifest: strings.Repeat("d", 64),
		Artifacts:      map[string]string{"ardents-node": strings.Repeat("a", 64), "ardents-control": strings.Repeat("b", 64), "forwarding-installed.test": strings.Repeat("c", 64)},
		Cases:          forwardingInstalledTransitionCases()[:4],
		CaseInputs:     make(map[string]map[string]string),
	}
	bodies := make(map[string][]byte)
	for _, name := range forwardingInstalledInputNames("lifetime-refill") {
		bodies[name] = []byte("original public fixture " + name)
	}
	for _, name := range receipt.Cases {
		receipt.CaseInputs[name] = make(map[string]string)
		for input, body := range bodies {
			sum := sha256.Sum256(body)
			receipt.CaseInputs[name][input] = hex.EncodeToString(sum[:])
		}
	}
	raw, err := json.Marshal(receipt)
	if err != nil {
		t.Fatal(err)
	}
	pinned, err := json.Marshal(forwardingBootstrapDocument{PinnedSource: receipt.SourceManifest, Artifacts: receipt.Artifacts})
	if err != nil {
		t.Fatal(err)
	}
	if err := forwardingInstalledValidateBootstrap(raw, pinned, "lifetime-refill", bodies); err != nil {
		t.Fatal("complete original root check", err)
	}
	for _, name := range []string{"peer_plan", "peer_certificate"} {
		// These are the two actual Interior call shapes, before that component
		// opens genuine State, TLS credentials or any receiving work.
		if err := forwardingInstalledValidateBootstrap(raw, pinned, "lifetime-refill", map[string][]byte{name: bodies[name]}); err == nil {
			t.Fatal("root driver accepted incomplete input set", name)
		}
		if err := forwardingInstalledValidatePeerBootstrap(raw, pinned, "lifetime-refill", name, bodies[name]); err != nil {
			t.Fatalf("original Interior public input %s rejected: %v", name, err)
		}
		if forwardingInstalledValidatePeerBootstrap(raw, pinned, "lifetime-refill", name, []byte("changed")) == nil {
			t.Fatal("changed peer public input accepted", name)
		}
		if forwardingInstalledValidatePeerBootstrap(raw, pinned, "unreviewed", name, bodies[name]) == nil {
			t.Fatal("unreviewed peer case accepted", name)
		}
	}
	for _, name := range []string{"binding", "plan", "presentations", "peer_presentations", "private_key", "", "initial_cut_inputs"} {
		if forwardingInstalledValidatePeerBootstrap(raw, pinned, "lifetime-refill", name, bodies[name]) == nil {
			t.Fatal("peer validator widened to foreign input", name)
		}
	}
	broken := receipt
	broken.CaseInputs = make(map[string]map[string]string)
	changed, err := json.Marshal(broken)
	if err != nil {
		t.Fatal(err)
	}
	if forwardingInstalledValidatePeerBootstrap(changed, pinned, "lifetime-refill", "peer_plan", bodies["peer_plan"]) == nil {
		t.Fatal("peer input bytes bypassed complete public receipt scope")
	}
}
