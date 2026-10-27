package duty

import (
	"context"
	"errors"
)

// SourceInspection independently retains complete existing participation under
// its own lease. Only original direct-source exposure records may remain; this
// observation never creates history, clears exposure or grants Source rights.
type SourceInspection struct{ original *participationInspection }

func OpenSourceInspection(ctx context.Context, path string, uid, gid uint32) (*SourceInspection, error) {
	original, err := openParticipationInspection(ctx, path, uid, gid)
	if err != nil {
		return nil, err
	}
	inspection := &SourceInspection{original: original}
	if err := inspection.Check(); err != nil {
		return nil, errors.Join(err, inspection.Close())
	}
	return inspection, nil
}

func sourceInspectionRecords(records []dutyRecord) error {
	for _, record := range records {
		if record.Class != "direct-source" || record.State != "exposed" {
			return errors.New("Source participation has an unresolved original producer")
		}
	}
	return nil
}

func (s *SourceInspection) Check() error {
	if s == nil {
		return errParticipationInspectionClosed
	}
	return s.original.check(sourceInspectionRecords)
}

func (s *SourceInspection) Close() error {
	if s == nil {
		return nil
	}
	return s.original.Close()
}
