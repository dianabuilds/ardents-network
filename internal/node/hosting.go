package node

import (
	"context"
	"errors"
	"time"

	hostingbudget "github.com/dianabuilds/ardents-network/internal/hosting"
	"github.com/dianabuilds/ardents-network/internal/node/hosting"
	"github.com/dianabuilds/ardents-network/internal/resource"
)

type closedHostingHandle = hosting.Handle

func (config *runtimeConfig) hostingPressure() (pressureLevel, error) {
	if config.host == nil {
		return pressureNormal, nil
	}
	now := config.now()
	if now.Before(config.hostingNext) {
		return config.hostingLevel, nil
	}
	config.hostingNext = now.Add(time.Second)
	ctx, stop := context.WithTimeout(context.Background(), time.Second)
	defer stop()
	var observation hostingbudget.Observation
	var err error
	if installed, ok := config.host.(*hosting.Ledger); ok {
		sample, sampleErr := installed.Sample(ctx, time.Second)
		config.hostingSample = &sample
		if config.ResourceMeasure != nil {
			config.hostingUsage, err = config.ResourceMeasure()
		} else {
			config.hostingUsage, err = resource.MeasureOwnerCgroups(nil)
		}
		err = errors.Join(err, sampleErr)
		observation = sample.Observation
	} else {
		sample, sampleErr := config.host.Sample(ctx, time.Second)
		err = sampleErr
		observation = sample.Observation
	}
	if err != nil || observation.Drain {
		config.hostingLevel = pressureDrain
		return pressureDrain, err
	}
	config.hostingLevel = pressureNormal
	if observation.Protect {
		config.hostingLevel = pressureProtect
	}
	return config.hostingLevel, nil
}
