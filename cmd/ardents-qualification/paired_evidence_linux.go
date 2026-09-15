//go:build linux

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/streamqualification"
)

type pairedWorkloadVerdict struct {
	Kind                                string
	CandidateSHA256, EndpointUnitSHA256 string
	Profile                             streamqualification.Profile
	Condition                           streamqualification.NetworkCondition
	ReaderNetwork, PublisherNetwork     ownerNetworkVerdict
	Relay                               relayTrafficVerdict
	NodeOwners                          nodeOwnersVerdict
	Criteria                            []streamqualification.Criterion
}

// verifyPair consumes the unedited local runner outputs. It does not convert
// successful workload evidence into a NET-14V, P5, P7 or P8 acceptance claim.
func verifyPair(paths []string, output io.Writer) error {
	if len(paths) != 6 {
		return errors.New("usage: ardents-qualification verify-pair <reader-jsonl> <publisher-jsonl> <network-manifest.json> <relay-results.json> <node-results.json> <node-inventory-sha256>")
	}
	manifest, manifestHash, manifestErr := readNetworkManifest(paths[2])
	criteria := []streamqualification.Criterion{{
		Name: "network-manifest-valid", Observed: boolNumber(manifestErr == nil), Relation: "=", Bound: 1, Passed: manifestErr == nil,
	}}
	var paired streamqualification.PairedReports
	var profile streamqualification.Profile
	var condition streamqualification.NetworkCondition
	var seed, binaryIdentity, unitIdentity string
	readers := make(map[int]bool)
	publisher := false
	ownerNetworks := make(map[streamqualification.Role]ownerNetworkVerdict, 2)

	evidenceErrors := make([]error, 2)
	for evidenceIndex, path := range paths[:2] {
		err := readCompletedEvidence(path, func(raw []byte) error {
			var record struct {
				Seed, BinarySHA256, PlanSHA256, Kind, Failure string
				EndpointArtifact                              qualificationEndpointArtifact
				Role                                          streamqualification.Role
				Profile                                       streamqualification.Profile
				Condition                                     streamqualification.NetworkCondition
				ReaderIndex                                   int
				Report                                        streamqualification.Report
				OwnerNetwork                                  ownerNetworkVerdict
				Criteria                                      []streamqualification.Criterion
			}
			if err := json.Unmarshal(raw, &record); err != nil {
				return err
			}
			if record.Kind == "candidate" {
				if _, err := decodeIdentity(record.BinarySHA256); err != nil {
					return err
				}
				if _, err := decodeIdentity(record.EndpointArtifact.ManifestSHA256); err != nil || len(record.EndpointArtifact.Files) != 3 ||
					record.EndpointArtifact.Files[qualificationBinaryPath] != record.BinarySHA256 ||
					record.EndpointArtifact.Files[qualificationPlanPath] != record.PlanSHA256 {
					return errors.New("runner installed Endpoint artifact evidence is incomplete")
				}
				unit := record.EndpointArtifact.Files[qualificationUnitPath]
				if _, err := decodeIdentity(unit); err != nil {
					return errors.New("runner installed Endpoint unit identity is invalid")
				}
				if binaryIdentity != "" && binaryIdentity != record.BinarySHA256 {
					return errors.New("runner candidates differ")
				}
				if unitIdentity != "" && unitIdentity != unit {
					return errors.New("runner Endpoint units differ")
				}
				binaryIdentity = record.BinarySHA256
				unitIdentity = unit
			}
			if record.Kind != "result" {
				return nil
			}
			if len(record.Criteria) == 0 || !streamqualification.CriteriaPassed(record.Criteria) {
				return errors.New("failed or missing participant criteria")
			}
			if record.Failure != "" || record.Report.Failure != "" {
				return errors.New("failed attempt cannot qualify")
			}
			if profile != 0 && profile != record.Profile {
				return errors.New("paired workload profiles differ")
			}
			if condition != 0 && condition != record.Condition {
				return errors.New("paired network conditions differ")
			}
			if _, err := record.Condition.CarrierRatioLimit(); err != nil {
				return err
			}
			if _, err := decodeIdentity(record.Seed); err != nil {
				return err
			}
			if seed != "" && seed != record.Seed {
				return errors.New("paired workload seeds differ")
			}
			seed = record.Seed
			profile = record.Profile
			condition = record.Condition
			if prior, present := ownerNetworks[record.Role]; present && prior != record.OwnerNetwork {
				return errors.New("one owner has inconsistent network evidence")
			}
			ownerNetworks[record.Role] = record.OwnerNetwork
			if record.Report.StoppedElapsed-record.Report.StartedElapsed < 600_000_000_000 || record.Report.MeasuredDuration != record.Report.StoppedElapsed-record.Report.StartedElapsed {
				return errors.New("monotonic workload interval missing")
			}
			switch record.Role {
			case streamqualification.ReaderRole:
				if record.ReaderIndex < 0 || record.ReaderIndex >= 4 || readers[record.ReaderIndex] {
					return errors.New("duplicate or invalid Reader evidence")
				} else {
					readers[record.ReaderIndex] = true
					paired.Readers[record.ReaderIndex] = record.Report
				}
			case streamqualification.PublisherRole:
				if publisher {
					return errors.New("duplicate Publisher evidence")
				} else {
					publisher = true
					paired.Publisher = record.Report
				}
			default:
				return errors.New("unknown workload role")
			}
			return nil
		})
		evidenceErrors[evidenceIndex] = err
		criteria = append(criteria, streamqualification.Criterion{
			Name:     []string{"reader-evidence-complete", "publisher-evidence-complete"}[evidenceIndex],
			Observed: boolNumber(err == nil), Relation: "=", Bound: 1, Passed: err == nil,
		})
	}
	completePair := len(readers) == 4 && publisher
	criteria = append(criteria, streamqualification.Criterion{Name: "paired-endpoint-set", Observed: float64(len(readers)) + boolNumber(publisher), Relation: "=", Bound: 5, Passed: completePair})
	criteria = append(criteria, streamqualification.EvaluatePairedConditionWorkload(paired, profile, condition)...)
	var relay relayTrafficVerdict
	var relayErr, nodeErr error
	var nodeOwners nodeOwnersVerdict
	if manifestErr == nil {
		var relayCriteria []streamqualification.Criterion
		relay, relayCriteria, relayErr = readRelayResults(paths[3], manifest, manifestHash)
		criteria = append(criteria, evaluatePairedOwnerNetwork(paired, profile, condition, ownerNetworks, relay)...)
		criteria = append(criteria, relayCriteria...)
		criteria = append(criteria, relayWindowCriteria(relay, ownerNetworks[streamqualification.ReaderRole].Started, ownerNetworks[streamqualification.ReaderRole].Stopped)...)
		var nodeCriteria []streamqualification.Criterion
		nodeOwners, nodeCriteria, nodeErr = readNodeResults(paths[4], manifest, paths[5], ownerNetworks)
		criteria = append(criteria, nodeCriteria...)
	} else {
		criteria = append(criteria,
			streamqualification.Criterion{Name: "relay-evidence", Relation: "blocked by network manifest", Passed: false},
			streamqualification.Criterion{Name: "node-owner-evidence", Relation: "blocked by network manifest", Passed: false},
		)
	}
	verdict := pairedWorkloadVerdict{Kind: "paired-workload", CandidateSHA256: binaryIdentity, EndpointUnitSHA256: unitIdentity, Profile: profile, Condition: condition, ReaderNetwork: ownerNetworks[streamqualification.ReaderRole], PublisherNetwork: ownerNetworks[streamqualification.PublisherRole], Relay: relay, NodeOwners: nodeOwners, Criteria: criteria}
	if err := json.NewEncoder(output).Encode(verdict); err != nil {
		return err
	}
	outcome := errors.Join(manifestErr, errors.Join(evidenceErrors...))
	if !streamqualification.CriteriaPassed(criteria) {
		outcome = errors.Join(outcome, errors.New("paired workload criteria failed"))
	}
	return errors.Join(outcome, relayErr, nodeErr)
}

func evaluatePairedOwnerNetwork(reports streamqualification.PairedReports, profile streamqualification.Profile, condition streamqualification.NetworkCondition, owners map[streamqualification.Role]ownerNetworkVerdict, relay relayTrafficVerdict) []streamqualification.Criterion {
	var readerTx, readerRx uint64
	for _, report := range reports.Readers {
		for _, stream := range report.Streams {
			if stream.ID <= 127 {
				readerTx += stream.Tx
				readerRx += stream.Rx
			}
		}
	}
	var publisherTx, publisherRx uint64
	for _, stream := range reports.Publisher.Streams {
		if stream.ID <= 127 {
			publisherTx += stream.Tx
			publisherRx += stream.Rx
		}
	}
	reader, readerPresent := owners[streamqualification.ReaderRole]
	publisher, publisherPresent := owners[streamqualification.PublisherRole]
	ratioLimit, conditionErr := condition.CarrierRatioLimit()
	validHost := func(owner ownerNetworkVerdict, tx, rx uint64) bool {
		if owner.InterfaceTx > math.MaxUint64-owner.InterfaceRx {
			return false
		}
		return owner.UsefulTx == tx && owner.UsefulRx == rx && owner.InterfaceTx+owner.InterfaceRx == owner.DirectionalWire &&
			owner.CountedBytes == owner.HostingLedgerDelta && owner.OneSecondSampleCount >= 598
	}
	readerValid := conditionErr == nil && readerPresent && validHost(reader, readerTx, readerRx)
	publisherValid := conditionErr == nil && publisherPresent && validHost(publisher, publisherTx, publisherRx)
	readerUseful, publisherUseful := readerRx, publisherTx
	if profile == streamqualification.ClientToPublisher {
		readerUseful, publisherUseful = readerTx, publisherRx
	}
	endpoint := func(id string, useful uint64) (relayNodeTraffic, float64, bool) {
		for _, observed := range relay.Endpoints {
			if observed.ID != id || observed.Tx > math.MaxUint64-observed.Rx {
				continue
			}
			wire := observed.Tx + observed.Rx
			ratio := float64(0)
			if useful != 0 {
				ratio = float64(wire) / float64(useful)
			}
			return observed, ratio, useful != 0 && wire >= useful && ratio <= ratioLimit
		}
		return relayNodeTraffic{}, 0, false
	}
	readerEndpoint, readerRatio, readerEndpointValid := endpoint(userEndpoint, readerUseful)
	publisherEndpoint, publisherRatio, publisherEndpointValid := endpoint(publisherEndpoint, publisherUseful)
	return []streamqualification.Criterion{
		{Name: "reader-owner-hosting-reconciliation", Observed: float64(reader.HostingLedgerDelta), Relation: "= host interface policy bytes", Bound: float64(reader.CountedBytes), Passed: readerValid},
		{Name: "publisher-owner-hosting-reconciliation", Observed: float64(publisher.HostingLedgerDelta), Relation: "= host interface policy bytes", Bound: float64(publisher.CountedBytes), Passed: publisherValid},
		{Name: "reader-endpoint-carrier-ratio", Observed: readerRatio, Relation: "<=", Bound: ratioLimit, Passed: readerEndpointValid},
		{Name: "publisher-endpoint-carrier-ratio", Observed: publisherRatio, Relation: "<=", Bound: ratioLimit, Passed: publisherEndpointValid},
		{Name: "reader-endpoint-wire-bytes", Observed: float64(readerEndpoint.Tx + readerEndpoint.Rx), Relation: ">= useful", Bound: float64(readerUseful), Passed: readerEndpointValid},
		{Name: "publisher-endpoint-wire-bytes", Observed: float64(publisherEndpoint.Tx + publisherEndpoint.Rx), Relation: ">= useful", Bound: float64(publisherUseful), Passed: publisherEndpointValid},
		{Name: "paired-directional-useful-bytes", Observed: float64(readerUseful), Relation: "= peer", Bound: float64(publisherUseful), Passed: readerUseful != 0 && readerUseful == publisherUseful},
		{Name: fmt.Sprintf("profile-%d-owner-network", profile), Observed: float64(len(owners)), Relation: "=", Bound: 2, Passed: len(owners) == 2},
	}
}
func relayWindowCriteria(relay relayTrafficVerdict, started, stopped time.Time) []streamqualification.Criterion {
	complete := !started.IsZero() && started.Before(stopped) && len(relay.Segments) == 12
	for _, segment := range relay.Segments {
		before, after := false, false
		for _, sample := range segment.Samples {
			before = before || !sample.At.After(started)
			after = after || !sample.At.Before(stopped)
		}
		complete = complete && before && after
	}
	return []streamqualification.Criterion{{Name: "relay-one-second-workload-window", Observed: float64(len(relay.Segments)), Relation: "complete", Bound: 12, Passed: complete}}
}
