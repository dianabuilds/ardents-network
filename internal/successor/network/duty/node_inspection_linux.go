package duty

import (
	"context"
	"errors"
	"os"
)

// NodeInspection retains complete existing participation without opening a
// producer. No records may remain, including expired or prepared records; no
// clock or process-exit observation proves their original physical join.
// Copies share original local denial, readers, lease and terminal Close.
type NodeInspection struct{ original *participationInspection }

func OpenNodeInspection(ctx context.Context, path string, uid, gid uint32) (NodeInspection, error) {
	return openNodeInspection(ctx, path, uid, gid, nil)
}

// InspectUnopenedNode retains the exact original empty participation directory.
// The caller owns proof that the installation never admitted a runtime start.
func InspectUnopenedNode(ctx context.Context, path string, uid, gid uint32, original os.FileInfo) (NodeInspection, error) {
	if original == nil {
		return NodeInspection{}, errors.New("unopened Node participation requires original directory")
	}
	return openNodeInspection(ctx, path, uid, gid, original)
}

func openNodeInspection(ctx context.Context, path string, uid, gid uint32, unopened os.FileInfo) (NodeInspection, error) {
	if ctx == nil {
		return NodeInspection{}, errors.New("inspection of Node participation requires caller")
	}
	if _, finite := ctx.Deadline(); !finite {
		return NodeInspection{}, errors.New("inspection of Node participation requires finite caller")
	}
	original, err := openParticipationInspection(ctx, path, uid, gid, unopened)
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
		return errors.New("unresolved original producer retained in Node participation")
	}
	return nil
}

// Check cannot clear, prune, quarantine or adopt an original producer.
func (n NodeInspection) Check() error { return n.original.check(nodeInspectionRecords) }

func (n NodeInspection) Close() error { return n.original.Close() }
