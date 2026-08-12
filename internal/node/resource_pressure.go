package node

import "github.com/dianabuilds/ardents-network/internal/node/probe"

type pressureLevel byte

const (
	pressureNormal pressureLevel = iota
	pressureProtect
	pressureDrain
)

func (config runtimeConfig) resourcePressure(server *probe.Server) (pressureLevel, error) {
	if config.pressure == nil {
		return pressureNormal, nil
	}
	timers, queueItems, queueBytes := server.Usage()
	observation, err := config.pressure.Observe(timers, queueItems, queueBytes)
	if err != nil || observation.Drain {
		return pressureDrain, err
	}
	if observation.Protect {
		return pressureProtect, nil
	}
	return pressureNormal, nil
}
