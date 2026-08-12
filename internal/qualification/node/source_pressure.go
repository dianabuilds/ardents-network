package node

import (
	"context"
	"errors"
	"strings"
	"time"
)

func (observer *nodeObserver) injectSourcePressure(ctx context.Context) error {
	if _, err := observer.compose(ctx, "stop", "--timeout", "5", "endpoint"); err != nil {
		return err
	}
	observer.captureLogs(ctx, "endpoint")
	if _, err := observer.compose(ctx, "stop", "--timeout", "5", "source2"); err != nil {
		return err
	}
	id, err := observer.serviceID(ctx, "source1")
	if err != nil {
		return err
	}
	if _, err := observer.docker(ctx, "exec", "-d", id, "/usr/local/bin/ardents-qualify", "inject-node", "--mode", "cpu"); err != nil {
		return err
	}
	if err := waitNode(ctx, 5*time.Second); err != nil {
		return err
	}
	once := []string{"run", "--rm", "--no-deps", "endpoint", "/usr/local/bin/ardents", "refresh-sources",
		"--state-root", "/run/ardents/state", "--source-plan", "/run/ardents/config.json", "--once"}
	if _, err := observer.compose(ctx, once...); err == nil {
		return errors.New("H3-S source admitted acquisition while its governor was protected")
	} else if !strings.Contains(err.Error(), "refresh network state: finite sources are temporarily unavailable") ||
		!strings.Contains(err.Error(), "finite source wave produced no valid state") {
		return err
	}
	if err := waitNode(ctx, 165*time.Second); err != nil {
		return err
	}
	if _, err := observer.compose(ctx, once...); err != nil {
		return nodeCandidateFailure("H3-S source did not recover after 120 low-watermark seconds", err)
	}
	if _, err := observer.compose(ctx, "up", "-d", "source2"); err != nil {
		return err
	}
	if _, err := observer.compose(ctx, "up", "-d", "--force-recreate", "endpoint"); err != nil {
		return err
	}
	return observer.waitServiceRunning(ctx, 15*time.Second, "endpoint")
}
