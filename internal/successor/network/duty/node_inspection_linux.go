package duty

import (
	"context"
	"errors"
)

// NodeInspection retains complete existing participation without opening a
// producer. No records may remain, including expired or prepared records; no
// clock or process-exit observation proves their original physical join.
// Copies share original local denial, readers, lease and terminal Close.
type NodeInspection struct{ original *participationInspection }

func OpenNodeInspection(ctx context.Context, path string, uid, gid uint32) (NodeInspection, error) {
	if ctx == nil {
		return NodeInspection{}, errors.New("Node participation inspection requires caller")
	}
	if _, finite := ctx.Deadline(); !finite {
		return NodeInspection{}, errors.New("Node participation inspection requires finite caller")
	}
	original, err := openParticipationInspection(ctx, path, uid, gid)
	if err != nil {
		return NodeInspection{}, err
	}
	inspection := NodeInspection{original: original}
	if err := inspection.Check(); err != nil {
		return NodeInspection{}, errors.Join(err, inspection.Close())
	}
	return inspection, nil
}

func nodeInspectionRecords(records []dutyRecord) error {
	if len(records) != 0 {
		return errors.New("Node participation retains an unresolved original producer")
	}
	return nil
}

// Check cannot clear, prune, quarantine or adopt an original producer.
func (n NodeInspection) Check() error { return n.original.check(nodeInspectionRecords) }

func (n NodeInspection) Close() error { return n.original.Close() }
