package node

import "github.com/dianabuilds/ardents-network/internal/resource"

type pressureLevel byte

const (
	pressureNormal pressureLevel = iota
	pressureProtect
	pressureDrain
)

func (config runtimeConfig) resourcePressure(server *probeServer) (pressureLevel, resource.Sample, error) {
	if config.pressure == nil {
		return pressureNormal, resource.Sample{}, nil
	}
	timers, queueItems, queueBytes := server.Usage()
	observation, err := config.pressure.Observe(timers, queueItems, queueBytes)
	if err != nil || observation.Drain {
		return pressureDrain, observation.Sample, err
	}
	if observation.Protect {
		return pressureProtect, observation.Sample, nil
	}
	return pressureNormal, observation.Sample, nil
}
