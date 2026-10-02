//go:build linux

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/streamqualification"
)

func ownerCPUFixture(t *testing.T, wallStep, monotonicStep time.Duration, cpuStep uint64) (nodeResultInput, nodeResultInput, ownerSliceResultInput) {
	t.Helper()
	origin := time.Unix(1000, 0).UTC()
	n := qualificationNodeInputs(t, []string{fmt.Sprintf("%064x", 1)})[0]
	s := qualificationSourceInputs(t)[0]
	o := qualificationOwnerSliceInputs()[0]
	nodeEvent, sourceEvent := n.Journal[1], s.Journal[1]
	n.Journal, s.Journal = n.Journal[:1], s.Journal[:1]
	var samples []nodeOwnerSampleInput
	for i := 0; i <= 600; i++ {
		at := origin.Add(time.Duration(i) * wallStep)
		samples = append(samples, nodeOwnerSampleInput{At: at, MonotonicNS: uint64(time.Second) + uint64(i)*uint64(monotonicStep), MemoryCurrent: 4 << 20, CPUUsageNSec: uint64(i) * cpuStep, IPIngressBytes: uint64(i) * 100, IPEgressBytes: uint64(i) * 200})
		for _, item := range []struct {
			raw  string
			dest *[]string
		}{{nodeEvent, &n.Journal}, {sourceEvent, &s.Journal}} {
			var event map[string]any
			if err := json.Unmarshal([]byte(item.raw), &event); err != nil {
				t.Fatal(err)
			}
			event["at"] = at
			body, err := json.Marshal(event)
			if err != nil {
				t.Fatal(err)
			}
			*item.dest = append(*item.dest, string(body))
		}
	}
	withdrawn, _ := json.Marshal(map[string]any{"schema": "ardents-node-event-v1", "kind": "lifecycle", "state": "WITHDRAWN", "at": samples[600].At})
	n.Journal = append(n.Journal, string(withdrawn))
	n.Samples, s.Samples, o.Samples = samples, samples, samples
	return n, s, o
}

func ownerCPUConsumers(n, s nodeResultInput, o ownerSliceResultInput) ([]float64, []bool) {
	started, stopped := o.Samples[0].At, o.Samples[len(o.Samples)-1].At
	no, nok := evaluateNodeOwner(n)
	so, sok := evaluateSourceOwner(s)
	oo, ook := evaluateOwnerSliceWindow(o, started, stopped)
	_, nc, ncok := nodeResourceWindow(n, started, stopped)
	_, sc, scok := sourceResourceWindow(s, started, stopped)
	return []float64{no.MeanCPUPercent, so.MeanCPUPercent, oo.MeanCPUPercent, nc, sc}, []bool{nok, sok, ook, ncok, scok}
}

func TestOwnerCPUIndependentWallClock(t *testing.T) {
	for _, wallStep := range []time.Duration{time.Second, 1010 * time.Millisecond, 990 * time.Millisecond} {
		t.Run(wallStep.String(), func(t *testing.T) {
			n, s, o := ownerCPUFixture(t, wallStep, time.Second, 500_000_000)
			values, valid := ownerCPUConsumers(n, s, o)
			for i, cpu := range values {
				if !valid[i] || math.Abs(cpu-50) > 1e-9 {
					t.Errorf("consumer %d CPU %.12f complete=%v, want 50", i, cpu, valid[i])
				}
			}
		})
	}
	n, s, o := ownerCPUFixture(t, time.Second, 1100*time.Millisecond, 500_000_000)
	values, valid := ownerCPUConsumers(n, s, o)
	for i, cpu := range values {
		if !valid[i] || math.Abs(cpu-50/1.1) > 1e-9 {
			t.Errorf("monotonic consumer %d CPU=%v complete=%v", i, cpu, valid[i])
		}
	}
}

func TestOwnerCPURefusesIncompleteMonotonicEvidence(t *testing.T) {
	changes := map[string]func([]nodeOwnerSampleInput) []nodeOwnerSampleInput{
		"missing": func(s []nodeOwnerSampleInput) []nodeOwnerSampleInput { s[300].MonotonicNS = 0; return s },
		"repeated": func(s []nodeOwnerSampleInput) []nodeOwnerSampleInput {
			s[300].MonotonicNS = s[299].MonotonicNS
			return s
		},
		"regressed": func(s []nodeOwnerSampleInput) []nodeOwnerSampleInput {
			s[300].MonotonicNS = s[299].MonotonicNS - 1
			return s
		},
		"gap": func(s []nodeOwnerSampleInput) []nodeOwnerSampleInput {
			for i := 300; i < len(s); i++ {
				s[i].MonotonicNS += uint64(time.Second)
			}
			return s
		},
		"cpu-reset": func(s []nodeOwnerSampleInput) []nodeOwnerSampleInput { s[300].CPUUsageNSec = 0; return s },
		"truncated": func(s []nodeOwnerSampleInput) []nodeOwnerSampleInput { return s[:597] },
		"short-monotonic-window": func(s []nodeOwnerSampleInput) []nodeOwnerSampleInput {
			for i := range s {
				s[i].MonotonicNS = uint64(time.Second) + uint64(i)*uint64(990*time.Millisecond)
			}
			return s
		},
		"unbounded": func(s []nodeOwnerSampleInput) []nodeOwnerSampleInput {
			return append(s, make([]nodeOwnerSampleInput, 1326-len(s))...)
		},
	}
	for name, change := range changes {
		t.Run(name, func(t *testing.T) {
			n, s, o := ownerCPUFixture(t, time.Second, time.Second, 500_000_000)
			samples := change(n.Samples)
			n.Samples, s.Samples, o.Samples = samples, samples, samples
			_, valid := ownerCPUConsumers(n, s, o)
			for i, ok := range valid {
				if ok {
					t.Errorf("consumer %d accepted %s", i, name)
				}
			}
		})
	}
}

func TestOwnerCPUExactWholeOwnerLimits(t *testing.T) {
	for index, host := range []string{"reader", "publisher"} {
		limit := uint64(500_000_000)
		role := streamqualification.ReaderRole
		if host == "publisher" {
			limit = 1_000_000_000
			role = streamqualification.PublisherRole
		}
		for _, excess := range []uint64{0, 1} {
			_, _, o := ownerCPUFixture(t, 1010*time.Millisecond, time.Second, limit+excess)
			template := qualificationOwnerSliceInputs()[index]
			template.Samples = o.Samples
			owners := map[streamqualification.Role]ownerNetworkVerdict{role: {Started: o.Samples[0].At, Stopped: o.Samples[600].At}}
			_, criteria := evaluateOwnerSlices([]ownerSliceResultInput{template}, owners)
			found := false
			for _, c := range criteria {
				if c.Name == host+"-whole-owner-slice-CPU-percent-one-core" {
					found = true
					if c.Passed != (excess == 0) {
						t.Fatalf("%s excess=%d criterion=%+v", host, excess, c)
					}
				}
			}
			if !found {
				t.Fatal("missing whole-owner CPU criterion")
			}
		}
	}
}

func TestOwnerCPUProducerCollectorPath(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	producer := filepath.Join(root, "tests", "qualification", "stream-network-two-host", "node_owner_samples.py")
	runner := filepath.Join(root, "tests", "qualification", "stream-network-two-host", "run-windows.ps1")
	// Actual script and exact systemctl invocation, deterministic host seam.
	// This does not qualify a live installed systemd owner.
	injection := `import runpy,sys,time,subprocess,datetime
count=0
unit=sys.argv[2]
class Wall(datetime.datetime):
 @classmethod
 def now(cls,tz=None):
  return datetime.datetime(2026,1,1,tzinfo=datetime.timezone.utc)+datetime.timedelta(seconds=count*1.01)
datetime.datetime=Wall
def show(argv,**kwargs):
 global count
 assert argv == ["systemctl","show",unit,"-p","ActiveState","-p","MemoryCurrent","-p","CPUUsageNSec","-p","IPIngressBytes","-p","IPEgressBytes"],argv
 count+=1
 state="active" if count<=601 else "inactive"
 class Result: pass
 r=Result()
 r.stdout=f"ActiveState={state}\nMemoryCurrent=4194304\nCPUUsageNSec={(count-1)*500000000}\nIPIngressBytes={count*100}\nIPEgressBytes={count*200}\n"
 return r
subprocess.run=show
time.sleep=lambda _:None
time.monotonic_ns=lambda:count*1000000000
path=sys.argv[1];sys.argv=[path,sys.argv[2]]
runpy.run_path(path,run_name="__main__")
`
	for _, unit := range []string{"ardents-qualification-node-" + strings.Repeat("a", 32) + "-0.service", "ardents-qualification-source-" + strings.Repeat("b", 32) + "-0.service"} {
		output, err := exec.Command("python3", "-c", injection, producer, unit).CombinedOutput()
		if err != nil {
			t.Fatalf("producer %s: %v\n%s", unit, err, output)
		}
		rawPath := filepath.Join(t.TempDir(), "samples.jsonl")
		if err := os.WriteFile(rawPath, output, 0600); err != nil {
			t.Fatal(err)
		}
		collector := `$tokens=$null; $errors=$null
$ast=[System.Management.Automation.Language.Parser]::ParseFile($env:OWNER_RUNNER,[ref]$tokens,[ref]$errors)
if($errors.Count){throw $errors[0]}
foreach($name in @('Read-OwnerCounterSamples','Stop-OwnerSlices','Stop-StateSources','Stop-RouteNodes')) {
 $fn=$ast.Find({param($a) $a -is [System.Management.Automation.Language.FunctionDefinitionAst] -and $a.Name -ceq $name},$true)
 if(-not $fn){throw "missing collector $name"}
 Invoke-Expression $fn.Extent.Text
}
$lines=@(Get-Content -LiteralPath $env:OWNER_SAMPLES)
$lines += '{"At":"2026-01-01T00:00:00Z","CPUUsageNSec":1}'
function Invoke-SSH($machine,$command,$description) {
 if($description -like '*owner samples' -or $description -like '*whole-owner samples'){return $lines}
 if($description -like '*sampler state'){return 'inactive'}
 if($description -like '*terminal*state'){return @('ActiveState=inactive','Result=success','ExecMainStatus=0','Slice=ardents-qualification-owner.slice')}
 return @()
}
$ownerSlices=@(@{Machine='fixture';Host='reader';Unit='ardents-qualification-owner.slice';SamplerUnit='fixture';CPUQuota='50%';CPUMax='50000 100000';MemoryMax=536870912;Receipt=@()})
$startedSources=@(@{Machine='fixture';Host='reader';Unit='fixture';SamplerUnit='fixture';ID='fixture';PlanSHA256='fixture';InvocationID='fixture'})
$startedNodes=@(@{Machine='fixture';Host='reader';Unit='fixture';SamplerUnit='fixture';ID='fixture';PlanSHA256='fixture';InvocationID='fixture'})
$inputFiles=@{node='fixture'}
Stop-OwnerSlices
Stop-StateSources
Stop-RouteNodes
foreach($record in @($ownerSliceRecords[0],$sourceRecords[0],$nodeRecords[0])) {
 if($record.Samples.Count -ne 602 -or $record.Samples[600].MonotonicNS -ne 601000000000 -or $record.Samples[601].PSObject.Properties['MonotonicNS']) {throw 'collector changed raw or missing monotonic samples'}
}
ConvertTo-Json -Depth 8 -InputObject @(Read-OwnerCounterSamples $lines) -Compress
`
		command := exec.Command("pwsh", "-NoProfile", "-Command", collector)
		command.Env = append(os.Environ(), "OWNER_RUNNER="+runner, "OWNER_SAMPLES="+rawPath)
		collected, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("real collector: %v\n%s", err, collected)
		}
		var samples []nodeOwnerSampleInput
		if err := json.Unmarshal(collected, &samples); err != nil {
			t.Fatalf("collector JSON: %v\n%s", err, collected)
		}
		if len(samples) != 602 || samples[0].MonotonicNS != 1_000_000_000 || samples[600].MonotonicNS != 601_000_000_000 || samples[601].MonotonicNS != 0 {
			t.Fatalf("collector lost raw monotonic evidence: count=%d", len(samples))
		}
		_, _, o := ownerCPUFixture(t, time.Second, time.Second, 500_000_000)
		o.Samples = samples[:601]
		observed, ok := evaluateOwnerSliceWindow(o, samples[0].At, samples[600].At)
		if !ok || observed.MeanCPUPercent != 50 {
			t.Fatalf("producer->collector->consumer: %+v complete=%v", observed, ok)
		}
	}
}

func TestOwnerCPUVerifyPairCLI(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "ardents-qualification")
	command := exec.Command("go", "build", "-trimpath", "-buildvcs=false", "-o", binary, ".")
	if out, err := command.CombinedOutput(); err != nil {
		t.Fatalf("CLI build: %v\n%s", err, out)
	}
	manifest, ids := qualificationNodeManifest()
	manifest.Relays = qualificationRelays(manifest)
	for i := 6; i <= 16; i++ {
		ids = append(ids, fmt.Sprintf("%064x", i))
	}
	inputs := qualificationNodeInputs(t, ids)
	sources := qualificationSourceInputs(t)
	slices := qualificationOwnerSliceInputs()
	inventory := strings.Repeat("ef", 32)
	owners := map[streamqualification.Role]ownerNetworkVerdict{
		streamqualification.ReaderRole:    {Started: inputs[0].Samples[0].At, Stopped: inputs[0].Samples[597].At, P95RSSBytes: 4 << 20},
		streamqualification.PublisherRole: {Started: inputs[0].Samples[0].At, Stopped: inputs[0].Samples[597].At, P95RSSBytes: 4 << 20},
	}
	journals := []string{}
	for _, role := range []streamqualification.Role{streamqualification.ReaderRole, streamqualification.PublisherRole} {
		var output bytes.Buffer
		journal := newEvidenceJournal(&output)
		count := 1
		if role == streamqualification.ReaderRole {
			count = 4
		}
		for i := 0; i < count; i++ {
			if err := journal.emit(map[string]any{"Kind": "result", "Role": role, "ReaderIndex": i, "Seed": strings.Repeat("01", 32), "Profile": streamqualification.ClientToPublisher, "Condition": streamqualification.NormalNetwork, "OwnerNetwork": owners[role], "Report": streamqualification.Report{StartedElapsed: time.Second, StoppedElapsed: 601 * time.Second, MeasuredDuration: 600 * time.Second}, "Criteria": []streamqualification.Criterion{{Name: "fixture-participant", Passed: true}}}); err != nil {
				t.Fatal(err)
			}
		}
		if err := journal.finish(nil); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(t.TempDir(), "journal.jsonl")
		if err := os.WriteFile(path, output.Bytes(), 0600); err != nil {
			t.Fatal(err)
		}
		journals = append(journals, path)
	}
	manifestPath := writeQualificationJSON(t, manifest)
	empty := writeQualificationJSON(t, []any{})
	for _, missing := range []bool{false, true} {
		if missing {
			inputs[0].Samples[300].MonotonicNS = 0
		}
		inputPath := writeQualificationJSON(t, nodeResultsInput{InventorySHA256: inventory, Nodes: inputs, Sources: sources, OwnerSlices: slices})
		command := exec.Command(binary, append([]string{"verify-pair"}, append(journals, manifestPath, empty, inputPath, inventory, empty)...)...)
		var output, stderr bytes.Buffer
		command.Stdout, command.Stderr = &output, &stderr
		// Traffic/release qualification remains incomplete in this resource fixture.
		if err := command.Run(); err == nil {
			t.Fatal("incomplete qualification fixture passed")
		}
		var verdict pairedWorkloadVerdict
		if err := json.Unmarshal(output.Bytes(), &verdict); err != nil {
			t.Fatalf("CLI verdict: %v\n%s\n%s", err, &output, &stderr)
		}
		found := false
		for _, c := range verdict.Criteria {
			if c.Name == "node-owner-"+inputs[0].ID {
				found = true
				if c.Passed == missing {
					t.Fatalf("CLI missing=%v resource criterion=%+v", missing, c)
				}
			}
		}
		if !found || len(verdict.NodeOwners.Nodes) != 16 {
			t.Fatal("CLI did not reach mandatory owner consumer")
		}
	}
}

func TestOwnerCPUExactNodeAndHostLimits(t *testing.T) {
	for _, excess := range []uint64{0, 1} {
		n, _, _ := ownerCPUFixture(t, time.Second, time.Second, 1_000_000_000+excess)
		_, ok := evaluateNodeOwner(n)
		if ok != (excess == 0) {
			t.Fatalf("Node one-core excess=%d complete=%v", excess, ok)
		}
	}
	_, ids := qualificationNodeManifest()
	for i := 6; i <= 16; i++ {
		ids = append(ids, fmt.Sprintf("%064x", i))
	}
	inputs, sources := qualificationNodeInputs(t, ids), qualificationSourceInputs(t)
	for i := range inputs {
		for j := range inputs[i].Samples {
			inputs[i].Samples[j].CPUUsageNSec = 0
		}
	}
	for i := range sources {
		for j := range sources[i].Samples {
			sources[i].Samples[j].CPUUsageNSec = 0
		}
	}
	for _, excess := range []float64{0, 0.000001} {
		owners := map[streamqualification.Role]ownerNetworkVerdict{
			streamqualification.ReaderRole:    {Started: inputs[0].Samples[0].At, Stopped: inputs[0].Samples[597].At, P95RSSBytes: 4 << 20, MeanCPUPercent: 50 + excess},
			streamqualification.PublisherRole: {Started: inputs[0].Samples[0].At, Stopped: inputs[0].Samples[597].At, P95RSSBytes: 4 << 20, MeanCPUPercent: 100 + excess},
		}
		_, criteria := evaluateNodeHostResources(inputs, sources, owners)
		for _, c := range criteria {
			if strings.HasSuffix(c.Name, "-complete-owner-CPU-percent-one-core") && c.Passed != (excess == 0) {
				t.Fatalf("host excess=%v criterion=%+v", excess, c)
			}
		}
	}
}
