package node

import (
	"errors"
	"path/filepath"
	"time"

	"github.com/dianabuilds/ardents-network/internal/entry"
	"github.com/dianabuilds/ardents-network/internal/network/duty"
	"github.com/dianabuilds/ardents-network/internal/route"
)

// openStateEntryAdmitter owns the Initiator-local durable Entry ledger for one
// running duty. State facts are read through the narrow copied view; neither
// the Entry package nor the Node listener receives State persistence/source.
func openStateEntryAdmitter(root string, snapshot dutyFacts, current func() (dutyFacts, error), now func() time.Time) (route.EntryBindingAdmitter, func() error, error) {
	if root == "" || current == nil || now == nil || snapshot.Assignment != "initiator" {
		return nil, nil, errors.New("initiator Entry admission is incomplete")
	}
	verification := entry.Verification{
		Current: func() (entry.View, error) {
			fresh, err := currentDutyForAdmission(snapshot, current, now)
			if err != nil {
				return entry.View{}, err
			}
			return entryView(fresh)
		},
		Conflict: func(identity, family [32]byte) (bool, error) {
			roles, err := duty.Open(duty.Config{Root: root, Clock: now, Create: true})
			if err != nil {
				return false, err
			}
			conflict, conflictErr := roles.Conflict(identity, family)
			return conflict, errors.Join(conflictErr, roles.Close())
		},
		Clock: now,
		TimeConfident: func() bool {
			_, err := currentDutyForAdmission(snapshot, current, now)
			return err == nil
		},
	}
	// Entry owns its own durable grammar and marker. It must therefore be a
	// sibling of, rather than a child inside, the duty root whose owner-only
	// inventory intentionally refuses unknown entries.
	admitter, err := entry.OpenAdmitter(entry.AdmitterConfig{Root: filepath.Join(filepath.Dir(root), filepath.Base(root)+"-initiator-entry"), Verification: verification})
	if err != nil {
		return nil, nil, err
	}
	return func(raw []byte, attachment, clientKey, recipient [32]byte, notAfter time.Time) (route.EntryAdmission, error) {
		authorization, admitErr := admitter.AdmitAndConsume(raw, attachment, clientKey, recipient, notAfter)
		if admitErr != nil {
			return route.EntryAdmission{}, admitErr
		}
		return route.EntryAdmission{InviteID: authorization.InviteID, NetworkID: authorization.NetworkID, Digest: authorization.Digest, RecipientPublicKey: authorization.RecipientPublicKey,
			Epoch: authorization.Epoch, InitiatorNodeID: authorization.InitiatorNodeID, NotAfter: authorization.NotAfter}, nil
	}, admitter.Close, nil
}

func entryView(snapshot dutyFacts) (entry.View, error) {
	if snapshot.NetworkID == [32]byte{} || snapshot.Epoch == 0 || snapshot.Digest == [32]byte{} || snapshot.Profile != route.Profile || !snapshot.Fresh {
		return entry.View{}, errors.New("initiator State Entry view is incomplete")
	}
	view := entry.View{NetworkID: snapshot.NetworkID, Epoch: snapshot.Epoch, Digest: snapshot.Digest, Profile: snapshot.Profile, Fresh: snapshot.Fresh,
		Candidates: make([]entry.Candidate, 0, snapshot.CandidateCount)}
	for index := uint8(0); index < snapshot.CandidateCount; index++ {
		candidate := snapshot.Candidates[index]
		view.Candidates = append(view.Candidates, entry.Candidate{NodeID: candidate.NodeID, PublicKey: candidate.PublicKey, KeyID: candidate.KeyID,
			FamilyID: candidate.FamilyID, RecordDigest: candidate.RecordDigest, DomainProofDigest: candidate.DomainProofDigest,
			Endpoint: candidate.Endpoint, Capacity: candidate.Capacity, Domain: candidate.Assignment,
			ValidFrom: candidate.ValidFrom, ValidUntil: candidate.ValidUntil, AssignmentNotAfter: candidate.AssignmentNotAfter})
	}
	return view, nil
}
