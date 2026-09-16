package node

import "github.com/dianabuilds/ardents-network/internal/resource"

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
