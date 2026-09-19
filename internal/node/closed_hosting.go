package node

import (
	"context"
	"errors"
	"path/filepath"
	"time"

	"github.com/dianabuilds/ardents-network/internal/resource"
	"github.com/dianabuilds/ardents-network/internal/route"
)

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
	config.host, err = openClosedForwardingHost(root)
	return err
}

func closedControlTokenVerifier(config runtimeConfig, receiver route.ClosedRoleReceiver) route.ClosedAdmissionVerifier {
	verify := closedRoleTokenVerifier(config, receiver)
	return func(input route.ClosedAdmissionVerification) (route.ClosedAdmissionApproval, error) {
		approval, err := verify(input)
		if err != nil {
			return route.ClosedAdmissionApproval{}, err
		}
		// Reserve the entire class lifetime, including retained Introduction
		// deliveries. A request-size bound does not bound a registration.
		var admitted uint64
		switch input.Class {
		case 1:
			admitted = 64 << 10
		case 3:
			admitted = route.ClosedIntroductionRegistrationByteLimit
		default:
			return route.ClosedAdmissionApproval{}, errors.New("control duty cannot admit forwarding class")
		}
		release, err := reserveClosedForwarding(config.host, ClosedForwardingProfile{
			AdmissionTraffic:   resource.HostingTraffic{Tx: 2 * admitted, Rx: 2 * admitted},
			TerminationTraffic: resource.HostingTraffic{Tx: 16 << 10, Rx: 16 << 10},
		}, input.Deadline)
		if err != nil {
			return route.ClosedAdmissionApproval{}, err
		}
		approval.Release = release
		return approval, nil
	}
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
	if installed, ok := config.host.(*installedClosedForwardingHost); ok {
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
