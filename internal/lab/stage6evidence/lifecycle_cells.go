package stage6evidence

import (
	"encoding/binary"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/namelease"
)

func runLifecycleCell(trace *traceRecord) error {
	policy := namelease.Policy{DefaultLeaseDuration: 1_000 * time.Hour, DefaultGraceDuration: time.Hour}
	switch trace.Cell {
	case "A2":
		claimed, err := claimRecord("alice", "authority-a", 100, policy, nil)
		if err != nil {
			return err
		}
		published, err := namelease.Apply(&claimed, 101, namelease.Op{Kind: "publish", Name: "alice",
			Authority: claimed.Authority, Target: [32]byte{1}, ExpectedGeneration: 1, ExpectedRevision: 1}, policy)
		return setRecordTrace(trace, nil, []namelease.Record{claimed, published}, []int64{100, 101}, err)
	case "A3":
		short := namelease.Policy{DefaultLeaseDuration: 100 * time.Second, DefaultGraceDuration: 100 * time.Second}
		initial, err := claimRecord("alice", "authority-a", 100, short, nil)
		if err != nil {
			return err
		}
		active, err := namelease.Apply(&initial, 150, namelease.Op{Kind: "renew", Name: "alice",
			Authority: initial.Authority, ExpectedGeneration: 1, ExpectedRevision: 1}, short)
		if err != nil {
			return err
		}
		grace, err := namelease.Apply(&initial, 201, namelease.Op{Kind: "advance", Name: "alice",
			ExpectedGeneration: 1, ExpectedRevision: 1}, short)
		if err != nil {
			return err
		}
		graceRenewed, err := namelease.Apply(&grace, 202, namelease.Op{Kind: "renew", Name: "alice",
			Authority: grace.Authority, ExpectedGeneration: 1, ExpectedRevision: grace.Revision}, short)
		return setRecordTrace(trace, []namelease.Record{initial},
			[]namelease.Record{active, grace, graceRenewed}, []int64{150, 201, 202}, err)
	case "A4":
		short := namelease.Policy{DefaultLeaseDuration: 10 * time.Second, DefaultGraceDuration: 10 * time.Second}
		initial, err := claimRecord("alice", "authority-a", 100, short, nil)
		if err != nil {
			return err
		}
		grace, err := namelease.Apply(&initial, 111, namelease.Op{Kind: "advance", Name: "alice",
			ExpectedGeneration: 1, ExpectedRevision: 1}, short)
		if err != nil {
			return err
		}
		released, err := namelease.Apply(&grace, 121, namelease.Op{Kind: "advance", Name: "alice",
			ExpectedGeneration: 1, ExpectedRevision: grace.Revision}, short)
		if err != nil {
			return err
		}
		reclaimed, err := namelease.Apply(&released, 122, namelease.Op{Kind: "claim", Name: "alice",
			Generation: 2, ExpectedGeneration: 1, ExpectedRevision: released.Revision, Authority: "authority-b"}, short)
		return setRecordTrace(trace, []namelease.Record{initial},
			[]namelease.Record{grace, released, reclaimed}, []int64{111, 121, 122}, err)
	case "A5":
		parent, err := claimRecord("root", "parent-authority", 100, policy, nil)
		if err != nil {
			return err
		}
		child, err := claimRecord("leaf.root", "child-authority", 101, policy, []namelease.Record{parent})
		if err != nil {
			return err
		}
		released, err := namelease.Apply(&parent, 102, namelease.Op{Kind: "release", Name: parent.Name,
			Authority: parent.Authority, ExpectedGeneration: 1, ExpectedRevision: 1}, policy)
		return setRecordTrace(trace, []namelease.Record{parent}, []namelease.Record{child, released}, []int64{101, 102}, err)
	case "B2":
		initial, err := claimRecord("alice", "authority-a", 100, policy, nil)
		if err != nil {
			return err
		}
		delay := 72 * time.Hour
		activation := int64(101_000) + delay.Milliseconds()
		scheduled, err := namelease.Apply(&initial, 101, namelease.Op{Kind: "schedule-recovery-policy", Name: "alice",
			Authority: initial.Authority, ExpectedGeneration: 1, ExpectedRevision: 1,
			PolicyDigest: [32]byte{2}, PolicyRevision: 1, PolicyDelay: delay, PolicyActivatesAt: activation}, policy)
		if err != nil {
			return err
		}
		active, err := namelease.Apply(&scheduled, activation/1_000, namelease.Op{Kind: "activate-recovery-policy",
			Name: "alice", ExpectedGeneration: 1, ExpectedRevision: scheduled.Revision}, policy)
		return setRecordTrace(trace, []namelease.Record{initial}, []namelease.Record{scheduled, active},
			[]int64{101, activation}, err)
	case "B4":
		initial, err := claimRecord("alice", "authority-a", 100, policy, nil)
		if err != nil {
			return err
		}
		_, denied := namelease.Apply(&initial, 101, namelease.Op{Kind: "start-recovery", Name: "alice",
			ExpectedGeneration: 1, ExpectedRevision: 1}, policy)
		if denied == nil {
			return errors.New("recovery without a policy was accepted")
		}
		trace.Fields = []string{"recovery-policy-absent"}
		return setRecordTrace(trace, []namelease.Record{initial}, []namelease.Record{initial}, []int64{101}, nil)
	case "C0":
		initial, err := claimRecord("alice", "authority-a", 100, policy, nil)
		if err != nil {
			return err
		}
		bound, err := namelease.Apply(&initial, 101, namelease.Op{Kind: "publish", Name: "alice",
			Authority: initial.Authority, Target: [32]byte{1}, ExpectedGeneration: 1, ExpectedRevision: 1}, policy)
		return setRecordTrace(trace, []namelease.Record{bound}, []namelease.Record{bound}, []int64{102}, err)
	case "C1":
		initial, err := claimRecord("alice", "authority-a", 100, policy, nil)
		if err != nil {
			return err
		}
		first, err := namelease.Apply(&initial, 101, namelease.Op{Kind: "publish", Name: "alice",
			Authority: initial.Authority, Target: [32]byte{1}, ExpectedGeneration: 1, ExpectedRevision: 1}, policy)
		if err != nil {
			return err
		}
		second, err := namelease.Apply(&first, 102, namelease.Op{Kind: "publish", Name: "alice",
			Authority: first.Authority, Target: [32]byte{2}, ExpectedGeneration: 1, ExpectedRevision: first.Revision}, policy)
		return setRecordTrace(trace, []namelease.Record{first}, []namelease.Record{second}, []int64{102}, err)
	case "D5":
		initial, err := claimRecord("alice", "authority-a", 100, policy, nil)
		if err != nil {
			return err
		}
		current, err := namelease.Apply(&initial, 101, namelease.Op{Kind: "renew", Name: "alice",
			Authority: initial.Authority, ExpectedGeneration: 1, ExpectedRevision: 1}, policy)
		if err != nil {
			return err
		}
		_, stale := namelease.Apply(&current, 102, namelease.Op{Kind: "renew", Name: "alice",
			Authority: current.Authority, ExpectedGeneration: 1, ExpectedRevision: 1}, policy)
		if stale == nil {
			return errors.New("stale revision was accepted")
		}
		trace.Fields = []string{"stale-proof"}
		return setRecordTrace(trace, []namelease.Record{initial}, []namelease.Record{current}, []int64{102}, nil)
	}
	return errors.New("unknown lifecycle cell")
}

func claimRecord(name, authority string, now int64, policy namelease.Policy, parents []namelease.Record) (namelease.Record, error) {
	return namelease.Apply(nil, now, namelease.Op{Kind: "claim", Name: name, Generation: 1,
		Authority: authority, Parents: parents}, policy)
}

func setRecordTrace(trace *traceRecord, before, after []namelease.Record, values []int64, err error) error {
	if err != nil {
		return err
	}
	var encodeErr error
	if trace.Input, encodeErr = packRecords(before); encodeErr != nil {
		return encodeErr
	}
	trace.Output, encodeErr = packRecords(after)
	trace.Values = values
	return encodeErr
}

func packRecords(records []namelease.Record) ([]byte, error) {
	out := binary.BigEndian.AppendUint16(nil, uint16(len(records)))
	for _, record := range records {
		raw, err := namelease.EncodeRecord(record)
		if err != nil {
			return nil, err
		}
		out = binary.BigEndian.AppendUint32(out, uint32(len(raw)))
		out = append(out, raw...)
	}
	return out, nil
}
