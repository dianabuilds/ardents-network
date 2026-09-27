package node

import (
	"context"
	"errors"
	"path/filepath"
	"time"

	"github.com/dianabuilds/ardents-network/internal/node/hosting"
	"github.com/dianabuilds/ardents-network/internal/resource"
)

type closedHostingHandle = hosting.Handle

// The shared period is opened once for the complete Node lifecycle, before
// listener activation, and retained until every admitted child has joined.
func (config *runtimeConfig) openClosedHosting() error {
	closed := config.ClosedForwarding.Certificate.PrivateKey != nil || config.ClosedIssuer.Certificate.PrivateKey != nil ||
		config.ClosedResolution.Certificate.PrivateKey != nil || config.ClosedIntroduction.Certificate.PrivateKey != nil ||
		config.ClosedDataJoin.Certificate.PrivateKey != nil
	if !closed {
		return nil
	}
	root := config.HostingRoot
	for _, selected := range []string{config.ClosedForwarding.HostingRoot, config.ClosedDataJoin.HostingRoot} {
		if selected == "" {
			continue
		}
		if root != "" && root != selected {
			return errors.New("node duties selected different host periods")
		}
		root = selected
	}
	if root == "" || !filepath.IsAbs(root) || filepath.Clean(root) != root {
		return errors.New("closed Node hosting period unavailable")
	}
	config.measurementOrigin = time.Now()
	var err error
	config.host, err = hosting.Open(root)
	return err
}

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
	var observation resource.HostingObservation
	var err error
	if installed, ok := config.host.(*hosting.Ledger); ok {
		sample, sampleErr := installed.Sample(ctx, time.Second)
		config.hostingSample = &sample
		config.hostingUsage, err = resource.MeasureOwnerCgroups(nil)
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
