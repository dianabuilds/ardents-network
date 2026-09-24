package node

import (
	"context"
	"errors"
	"strings"

	"github.com/dianabuilds/ardents-network/internal/resource"
)

type pressureLevel byte

const (
	pressureNormal pressureLevel = iota
	pressureProtect
	pressureDrain
)

func (config *runtimeConfig) resourcePressure(server *probeServer) (pressureLevel, resource.Sample, error) {
	hosting, hostingErr := config.hostingPressure()
	if hostingErr != nil || hosting == pressureDrain {
		return pressureDrain, resource.Sample{}, hostingErr
	}
	if config.pressure == nil {
		return hosting, config.hostingUsage, nil
	}
	timers, queueItems, queueBytes := server.Usage()
	observation, err := config.pressure.Observe(timers, queueItems, queueBytes)
	if err != nil || observation.Drain {
		return pressureDrain, observation.Sample, err
	}
	if config.hostingSample != nil {
		observation.Sample.RSSBytes = config.hostingUsage.RSSBytes
	}
	if observation.Protect || hosting == pressureProtect {
		return pressureProtect, observation.Sample, nil
	}
	return pressureNormal, observation.Sample, nil
}

// resourceSampleFailureStage retains a fixed local owner category for the
// lifecycle evidence path. It never returns an underlying operating-system or
// provider error.
func resourceSampleFailureStage(err error) string {
	switch {
	case errors.Is(err, context.Canceled):
		return "canceled"
	case errors.Is(err, context.DeadlineExceeded):
		return "deadline"
	case strings.Contains(err.Error(), "hosting lock"):
		return "hosting-lock"
	case strings.Contains(err.Error(), "hosting state"):
		return "hosting-state"
	case strings.Contains(err.Error(), "hosting observation"):
		return "hosting-observation"
	case strings.Contains(err.Error(), "hosting interface"):
		return "hosting-interface"
	case strings.Contains(err.Error(), "owner cgroup"):
		return "owner-cgroup"
	default:
		return "other"
	}
}
