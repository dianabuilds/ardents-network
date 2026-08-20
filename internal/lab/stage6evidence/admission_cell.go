package stage6evidence

import (
	"crypto/sha256"
	"encoding/json"
	"errors"

	"github.com/dianabuilds/ardents-network/internal/nameadmission"
)

type admissionCellEvidence struct {
	Node       [32]byte
	Network    [32]byte
	Epoch      uint64
	Now        int64
	Isolation  [32]byte
	Profiles   []admissionProfileEvidence
	Replay     string
	Expired    string
	Restarted  string
	CrossScope string
}

type admissionProfileEvidence struct {
	Surface         string
	WorkBits        uint8
	MaximumSpent    int
	MaximumInFlight int
	Proof           nameadmission.Proof
	Accepted        bool
	Capacity        admissionCapacityEvidence
}

type admissionCapacityEvidence struct {
	WorkNonces    []uint64
	SolveHashes   []uint64
	Overflow      string
	BusyOutcomes  []string
	PressureNanos []int64
}

func runAdmissionCell(trace *traceRecord, secret [32]byte) error {
	evidence := admissionCellEvidence{Node: [32]byte{31}, Network: [32]byte{32}, Epoch: 9,
		Now: 950, Isolation: sha256.Sum256([]byte("stage-6-admission-isolation"))}
	gate, err := nameadmission.NewAdmission(evidence.Node, evidence.Network, evidence.Epoch, secret)
	if err != nil {
		return err
	}
	profiles := []admissionProfileEvidence{{Surface: "resolution", WorkBits: 16, MaximumSpent: 4096, MaximumInFlight: 64},
		{Surface: "renewal-update", WorkBits: 16, MaximumSpent: 2048, MaximumInFlight: 32},
		{Surface: "policy-recovery", WorkBits: 17, MaximumSpent: 1024, MaximumInFlight: 16},
		{Surface: "root-claim", WorkBits: 18, MaximumSpent: 1024, MaximumInFlight: 8}}
	for index := range profiles {
		challenge, issueErr := gate.Issue(900, profiles[index].Surface,
			sha256.Sum256([]byte{byte(index + 1)}), evidence.Isolation, 1_000, [16]byte{byte(index + 1)})
		if issueErr != nil || challenge.WorkBits != profiles[index].WorkBits || !challenge.BindsIsolation(evidence.Isolation) {
			return errors.New("admission profile challenge is not exact")
		}
		profiles[index].Proof, _ = challenge.Solve()
		profiles[index].Accepted, _ = gate.Verify(evidence.Now, profiles[index].Proof)
		if !profiles[index].Accepted {
			return errors.New("valid admission proof was rejected")
		}
		profiles[index].Capacity, issueErr = measureAdmissionCapacity(secret, evidence, index, profiles[index])
		if issueErr != nil {
			return issueErr
		}
	}
	evidence.Profiles = profiles
	_, evidence.Replay = gate.Verify(evidence.Now, profiles[0].Proof)
	_, evidence.Expired = gate.Verify(1_000, profiles[1].Proof)
	restarted, restartErr := nameadmission.NewAdmission(evidence.Node, evidence.Network, evidence.Epoch, [32]byte{99})
	if restartErr != nil {
		return restartErr
	}
	_, evidence.Restarted = restarted.Verify(evidence.Now, profiles[2].Proof)
	cross := profiles[3].Proof
	cross.Challenge.Surface = "resolution"
	_, evidence.CrossScope = gate.Verify(evidence.Now, cross)
	if evidence.Replay != "replay" || evidence.Expired != "invalid-scope" ||
		evidence.Restarted != "invalid-challenge" || evidence.CrossScope != "invalid-scope" {
		return errors.New("admission hostile outcomes are incomplete")
	}
	raw, err := json.Marshal(evidence)
	if err != nil {
		return err
	}
	trace.Auxiliary = raw
	trace.Fields = []string{"accepted", "replay", "expired", "restart", "cross-scope"}
	return nil
}
