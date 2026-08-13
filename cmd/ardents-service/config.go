package main

import (
	"crypto/ed25519"
	"errors"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/planfile"
	"github.com/dianabuilds/ardents-network/internal/serviceconn"
)

type endpointPlan struct {
	Role, NetworkID, BrokerID, AuthorityPublic, ConnectionPrincipal       string
	AdministrationPrincipal, Target                                       string
	IntroductionSocket, IntroductionPublic                                string
	ApplicationSocket, RouteSocket, AdministrationSocket                  string
	PublicationFile, CredentialFile, InstanceKeyFile, GenerationStateFile string
	At, Deadline                                                          string
	BytesEachDirection                                                    uint32
}

func readPlan(path string) (endpointPlan, error) {
	var value endpointPlan
	if err := planfile.Decode(path, 64<<10, &value); err != nil {
		return endpointPlan{}, err
	}
	if err := value.validate(); err != nil {
		return endpointPlan{}, err
	}
	return value, nil
}
func (value endpointPlan) validate() error {
	if value.Role != "client" && value.Role != "publisher" {
		return errors.New("endpoint role is invalid")
	}
	if value.ApplicationSocket == "" || value.RouteSocket == "" || value.PublicationFile == "" ||
		value.At == "" || value.Deadline == "" || value.BytesEachDirection == 0 || value.BytesEachDirection > 64<<10 {
		return errors.New("endpoint plan is incomplete or outside its bound")
	}
	if value.IntroductionPublic == "" {
		return errors.New("endpoint plan lacks the Introduction verification key")
	}
	if value.Role == "client" && (value.Target == "" || value.AdministrationSocket != "" ||
		value.CredentialFile != "" || value.InstanceKeyFile != "" || value.AdministrationPrincipal != "" ||
		value.IntroductionSocket != "" || value.GenerationStateFile != "") {
		return errors.New("client plan contains publisher administration input")
	}
	if value.Role == "publisher" && (value.AdministrationSocket == "" || value.CredentialFile == "" ||
		value.InstanceKeyFile == "" || value.IntroductionSocket == "" || value.IntroductionPublic == "" ||
		value.GenerationStateFile == "") {
		return errors.New("publisher plan lacks its administration input")
	}
	if _, err := time.Parse(time.RFC3339, value.At); err != nil {
		return err
	}
	deadline, err := time.ParseDuration(value.Deadline)
	if err != nil || deadline <= 0 || deadline > 15*time.Second {
		return errors.New("endpoint deadline is outside the frozen bound")
	}
	return nil
}
func endpointSetup(plan endpointPlan) (serviceconn.Setup, time.Time, time.Duration, error) {
	var setup serviceconn.Setup
	for _, field := range []struct {
		encoded     string
		destination []byte
	}{
		{plan.NetworkID, setup.NetworkID[:]}, {plan.BrokerID, setup.BrokerID[:]},
		{plan.ConnectionPrincipal, setup.ConnectionPrincipal[:]}, {plan.AdministrationPrincipal, setup.AdministrationPrincipal[:]}} {
		if field.encoded != "" {
			if err := planfile.FixedHex(field.encoded, field.destination); err != nil {
				return setup, time.Time{}, 0, err
			}
		}
	}
	setup.AuthorityPublic = make([]byte, ed25519.PublicKeySize)
	if err := planfile.FixedHex(plan.AuthorityPublic, setup.AuthorityPublic); err != nil {
		return setup, time.Time{}, 0, err
	}
	setup.IntroductionPublic = make([]byte, ed25519.PublicKeySize)
	if err := planfile.FixedHex(plan.IntroductionPublic, setup.IntroductionPublic); err != nil {
		return setup, time.Time{}, 0, err
	}
	setup.GenerationStateFile = plan.GenerationStateFile
	setup.Resources = resourceObserver()
	at, err := time.Parse(time.RFC3339, plan.At)
	if err != nil {
		return setup, time.Time{}, 0, err
	}
	deadline, err := time.ParseDuration(plan.Deadline)
	return setup, at, deadline, err
}

func resourceObserver() func(string, int) uint32 {
	var mu sync.Mutex
	current, highWater := map[string]uint32{}, map[string]uint32{}
	return func(kind string, delta int) uint32 {
		mu.Lock()
		defer mu.Unlock()
		if delta > 0 {
			current[kind] += uint32(delta)
			if current[kind] > highWater[kind] {
				highWater[kind] = current[kind]
			}
		} else if delta < 0 && current[kind] >= uint32(-delta) {
			current[kind] -= uint32(-delta)
		}
		return highWater[kind]
	}
}
