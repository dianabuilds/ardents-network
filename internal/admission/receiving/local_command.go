package receiving

import (
	"context"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/admission"
	"github.com/dianabuilds/ardents-network/internal/platform/artifacts"
)

// HandleLocalCommand executes a bounded receiving operation against this allowance.
func (o *LocalAllowance) HandleLocalCommand(ctx context.Context, raw []byte) (any, bool, error) {
	var c struct {
		Operation string          `json:"operation"`
		Token     []byte          `json:"token"`
		Class     admission.Class `json:"class"`
		Deadline  time.Time       `json:"deadline"`
		Remaining uint64          `json:"remaining"`
	}
	if err := artifacts.DecodeObject(raw, &c); err != nil {
		return nil, false, err
	}
	var err error
	switch c.Operation {
	case "accept":
		err = o.Accept(ctx, c.Class, c.Token, c.Deadline)
	case "refill":
		err = o.Refill(ctx, c.Remaining, c.Token)
	case "release":
		err = o.Release()
	case "close":
		return map[string]string{"outcome": "closed"}, true, nil
	default:
		return nil, false, errors.New("unknown receiver operation")
	}
	if err != nil {
		return nil, false, err
	}
	allowance, active := o.Current()
	return map[string]any{"outcome": "completed", "active": active, "deadline": allowance.Deadline(), "bytes": allowance.Bytes()}, false, nil
}
