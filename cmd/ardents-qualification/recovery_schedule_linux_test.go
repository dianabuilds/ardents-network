//go:build linux

package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/streamqualification"
)

func scheduleCLIInputs(t *testing.T) ([]string, []recoveryFaultRecord, pairedWorkloadVerdict, pairedWorkloadVerdict, qualificationNetworkManifest) {
	t.Helper()
	manifest, recovery, baseline, episode := qualificationNET14VBitratePair(t, nil, 0)
	basePath, episodePath := writeQualificationJSON(t, manifest), writeQualificationJSON(t, recovery)
	_, baseline.Relay.ManifestSHA256, _ = readNetworkManifest(basePath)
	_, episode.Relay.ManifestSHA256, _ = readNetworkManifest(episodePath)
	records := []recoveryFaultRecord{}
	origin := episode.ReaderNetwork.Started.UnixMilli()
	for _, failure := range recovery.Failures {
		at := origin + int64(failure.AtMillis)
		record := recoveryFaultRecord{Host: "reader", ManifestSHA256: episode.Relay.ManifestSHA256, RunStartedMillis: origin, Episode: failure.Episode, Segment: failure.SegmentID, ScheduledMillis: at, ActualMillis: at, Kind: "recovery-fault-start"}
		records = append(records, record)
		record.Kind = "recovery-fault-stop"
		record.ScheduledMillis += int64(failure.DurationMillis)
		record.ActualMillis = record.ScheduledMillis
		records = append(records, record)
	}
	records = append(records, recoveryFaultRecord{Kind: "recovery-faults-complete", Host: "reader", ManifestSHA256: episode.Relay.ManifestSHA256, RunStartedMillis: origin, ActualMillis: origin + 420_000, Episodes: len(recovery.Failures)})
	return []string{basePath, episodePath, writeQualificationJSON(t, baseline), writeQualificationJSON(t, episode)}, records, baseline, episode, recovery
}

func writeFaultRecords(t *testing.T, records []recoveryFaultRecord) string {
	t.Helper()
	var body bytes.Buffer
	for _, record := range records {
		if err := json.NewEncoder(&body).Encode(record); err != nil {
			t.Fatal(err)
		}
	}
	path := filepath.Join(t.TempDir(), "faults.jsonl")
	if err := os.WriteFile(path, body.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestNET14VScheduleBindingRealCLI(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "ardents-qualification")
	if output, err := exec.Command("go", "build", "-trimpath", "-buildvcs=false", "-o", binary, ".").CombinedOutput(); err != nil {
		t.Fatalf("build CLI: %v / %s", err, output)
	}
	for _, test := range []struct {
		name   string
		change func(*[]recoveryFaultRecord, *pairedWorkloadVerdict)
		pass   bool
	}{
		{"aligned", func(*[]recoveryFaultRecord, *pairedWorkloadVerdict) {}, true},
		{"translated-hour", func(r *[]recoveryFaultRecord, _ *pairedWorkloadVerdict) {
			for i := range *r {
				(*r)[i].ScheduledMillis += 3_600_000
				(*r)[i].ActualMillis += 3_600_000
			}
		}, false},
		{"translated-origin-too", func(r *[]recoveryFaultRecord, _ *pairedWorkloadVerdict) {
			for i := range *r {
				(*r)[i].ScheduledMillis += 3_600_000
				(*r)[i].ActualMillis += 3_600_000
				(*r)[i].RunStartedMillis += 3_600_000
			}
		}, false},
		{"before-workload", func(r *[]recoveryFaultRecord, _ *pairedWorkloadVerdict) {
			(*r)[0].ActualMillis = (*r)[0].RunStartedMillis - 1
		}, false},
		{"after-workload", func(r *[]recoveryFaultRecord, _ *pairedWorkloadVerdict) {
			(*r)[1].ActualMillis = (*r)[1].RunStartedMillis + 600_001
		}, false},
		{"missing-stop", func(r *[]recoveryFaultRecord, _ *pairedWorkloadVerdict) { *r = append((*r)[:1], (*r)[2:]...) }, false},
		{"missing-completion", func(r *[]recoveryFaultRecord, _ *pairedWorkloadVerdict) { *r = (*r)[:len(*r)-1] }, false},
		{"missing-run", func(r *[]recoveryFaultRecord, _ *pairedWorkloadVerdict) { (*r)[0].RunStartedMillis = 0 }, false},
		{"mismatched-run", func(r *[]recoveryFaultRecord, _ *pairedWorkloadVerdict) { (*r)[0].RunStartedMillis += 2000 }, false},
		{"mismatched-host", func(r *[]recoveryFaultRecord, _ *pairedWorkloadVerdict) { (*r)[0].Host = "publisher" }, false},
		{"mismatched-manifest", func(r *[]recoveryFaultRecord, _ *pairedWorkloadVerdict) {
			(*r)[0].ManifestSHA256 = strings.Repeat("cd", 32)
		}, false},
		{"completion-before-stop", func(r *[]recoveryFaultRecord, _ *pairedWorkloadVerdict) {
			(*r)[len(*r)-1].ActualMillis = (*r)[0].ActualMillis
		}, false},
		{"duplicate-start", func(r *[]recoveryFaultRecord, _ *pairedWorkloadVerdict) { *r = append(*r, (*r)[0]) }, false},
		{"missing-workload", func(_ *[]recoveryFaultRecord, e *pairedWorkloadVerdict) { e.ReaderNetwork.Started = time.Time{} }, false},
		{"publisher-ended-early", func(_ *[]recoveryFaultRecord, e *pairedWorkloadVerdict) {
			e.PublisherNetwork.Stopped = e.PublisherNetwork.Started.Add(25 * time.Second)
		}, false},
		{"actual-tail-accounted", func(r *[]recoveryFaultRecord, e *pairedWorkloadVerdict) {
			(*r)[0].ActualMillis += 1000
			(*r)[1].ActualMillis += 1000
			for i := range e.Relay.Segments {
				if e.Relay.Segments[i].ID == (*r)[0].Segment {
					// This traffic lies outside the old nominal stop+8s window, inside the
					// actual stop+8s window; whole-set addition remains below 3*8 MiB.
					s := &e.Relay.Segments[i]
					s.Samples = append([]relaySegmentSample(nil), s.Samples...)
					for j := 39; j < len(s.Samples); j++ {
						s.Samples[j].Bytes += 9 << 20
					}
					s.Bytes += 9 << 20
				}
			}
		}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			args, records, _, episode, _ := scheduleCLIInputs(t)
			test.change(&records, &episode)
			args[3] = writeQualificationJSON(t, episode)
			args = append([]string{"verify-net14v"}, append(args, writeFaultRecords(t, records))...)
			output, err := exec.Command(binary, args...).Output()
			if (err == nil) != test.pass {
				t.Fatalf("pass=%v err=%v output=%s", test.pass, err, output)
			}
			var verdict net14vVerdict
			if err := json.Unmarshal(output, &verdict); err != nil {
				t.Fatal(err)
			}
			found := false
			for _, c := range verdict.Criteria {
				if c.Name == "net14v-recovery-schedule-evidence" {
					found = true
					want := test.pass || test.name == "actual-tail-accounted"
					if c.Passed != want {
						t.Fatalf("schedule criterion=%+v", c)
					}
				}
				if test.name == "actual-tail-accounted" && c.Name == "net14v-episode-fault-0-added-route-bytes" && c.Passed {
					t.Fatal("actual event tail was not counted")
				}
			}
			if !found {
				t.Fatal("missing schedule criterion")
			}
		})
	}
	// Failed consumer is independently exercised, including complete failed
	// workload reports. Its refusal is not inferred from the successful consumer.
	for _, test := range []struct {
		name   string
		broken bool
	}{{"aligned-failed-byte-evidence", false}, {"translated-failed-byte-evidence", true}, {"missing-failed-journal", true}, {"truncated-failed-journal", true}, {"failed-journal-wrong-seed", true}, {"failed-journal-missing-stop", true}} {
		t.Run(test.name, func(t *testing.T) {
			args, records, baseline, episode, manifest := scheduleCLIInputs(t)
			relays := []failedRelayResult{}
			for _, relay := range manifest.Relays {
				samples := make([]relayCounterSample, 601)
				for i := range samples {
					samples[i] = relayCounterSample{At: episode.ReaderNetwork.Started.Add(time.Duration(i) * time.Second), Elapsed: time.Duration(i) * time.Second, UpstreamBytes: uint64(i), ClientBytes: uint64(i)}
				}
				relays = append(relays, failedRelayResult{ID: relay.ID, Host: relay.Host, Container: relay.Container, UpstreamSegment: relay.UpstreamSegment, ClientSegment: relay.ClientSegment, BinarySHA256: baseline.Relay.BinarySHA256, Samples: samples, TrafficControl: []string{`[{"kind":"htb","handle":"1:10","bytes":600},{"kind":"htb","handle":"1:20","bytes":600}]`}})
			}
			if test.name == "translated-failed-byte-evidence" {
				for i := range records {
					records[i].ScheduledMillis += 3_600_000
					records[i].ActualMillis += 3_600_000
				}
			}
			args[3] = writeQualificationJSON(t, relays)
			args = append(args, writeFailedWorkloadJournal(t, streamqualification.ReaderRole, baseline, episode.ReaderNetwork), writeFailedWorkloadJournal(t, streamqualification.PublisherRole, baseline, episode.PublisherNetwork), writeFaultRecords(t, records))
			switch test.name {
			case "missing-failed-journal":
				args[4] = filepath.Join(t.TempDir(), "missing")
			case "truncated-failed-journal":
				body, err := os.ReadFile(args[4])
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(args[4], body[:len(body)-1], 0600); err != nil {
					t.Fatal(err)
				}
			case "failed-journal-wrong-seed":
				substituted := baseline
				substituted.Seed = strings.Repeat("cd", 32)
				args[4] = writeFailedWorkloadJournal(t, streamqualification.ReaderRole, substituted, episode.ReaderNetwork)
			case "failed-journal-missing-stop":
				incomplete := episode.ReaderNetwork
				incomplete.Stopped = time.Time{}
				args[4] = writeFailedWorkloadJournal(t, streamqualification.ReaderRole, baseline, incomplete)
			}
			output, err := exec.Command(binary, append([]string{"verify-failed-net14v"}, args...)...).Output()
			if (err == nil) == test.broken {
				t.Fatalf("err=%v output=%s", err, output)
			}
		})
	}
}

func writeFailedWorkloadJournal(t *testing.T, role streamqualification.Role, baseline pairedWorkloadVerdict, owner ownerNetworkVerdict) string {
	t.Helper()
	var body bytes.Buffer
	count := 4
	if role == streamqualification.PublisherRole {
		count = 1
	}
	encoder := json.NewEncoder(&body)
	candidate := map[string]any{"Kind": "candidate", "Participants": count, "PlanSHA256": baseline.CandidateSHA256, "BinarySHA256": baseline.CandidateSHA256, "EndpointArtifact": qualificationEndpointArtifact{ManifestSHA256: baseline.CandidateSHA256, Files: map[string]string{qualificationUnitPath: baseline.EndpointUnitSHA256, qualificationBinaryPath: baseline.CandidateSHA256, qualificationPlanPath: baseline.CandidateSHA256}}}
	if err := encoder.Encode(candidate); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < count; i++ {
		record := map[string]any{"Kind": "result", "Participant": i, "Role": role, "Profile": baseline.Profile, "Condition": streamqualification.RecoveryNetwork, "Seed": baseline.Seed, "Report": streamqualification.Report{Started: owner.Started, Stopped: owner.Stopped, StoppedElapsed: owner.Stopped.Sub(owner.Started), MeasuredDuration: owner.Stopped.Sub(owner.Started)}}
		if err := encoder.Encode(record); err != nil {
			t.Fatal(err)
		}
	}
	digest := sha256.Sum256(body.Bytes())
	if err := encoder.Encode(evidenceTerminal{Kind: "terminal", Records: uint64(count + 1), SHA256: hex.EncodeToString(digest[:]), Failure: "workload resource gate failed"}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), fmt.Sprintf("workload-%d.jsonl", role))
	if err := os.WriteFile(path, body.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}
