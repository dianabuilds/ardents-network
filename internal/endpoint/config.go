package endpoint

import (
	"crypto/ed25519"
	"errors"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
	serviceconnection "github.com/dianabuilds/ardents-network/internal/service/connection"
)

const maximumEndpointStreamBytes = uint32(768 << 20)

type endpointPlan struct {
	Role, NetworkID, BrokerID, AuthorityPublic, ConnectionPrincipal   string
	AdministrationPrincipal, Target                                   string
	CandidateView, IsolationContext, DestinationBinding, RouteProfile string
	IntroductionSocket, IntroductionPublic                            string
	ApplicationSocket, RouteSocket, AdministrationSocket              string
	PublicationFile, CredentialFile, InstanceKeyFile                  string
	PublicationRoot, LegacyGenerationFloor                            string
	At, Deadline, Lifetime                                            string
	BytesEachDirection                                                uint32
	SendBytes, ReceiveBytes                                           uint32
	MaximumConnections                                                uint16
	PersistentAdministration                                          bool
	WorkSafetyNotAfter, WorkSafetyMaximum, NoNewRecoveryAfter         int64
}

func readPlan(path string) (endpointPlan, error) {
	var value endpointPlan
	if err := decodeEndpointPlan(path, 64<<10, &value); err != nil {
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
		value.At == "" || value.Deadline == "" || !value.validStreamBounds() || value.MaximumConnections > 16 {
		return errors.New("endpoint plan is incomplete or outside its bound")
	}
	if value.IntroductionPublic == "" {
		return errors.New("endpoint plan lacks the Introduction verification key")
	}
	if value.Role == "client" && (value.Target == "" || value.AdministrationSocket != "" ||
		value.CredentialFile != "" || value.InstanceKeyFile != "" || value.AdministrationPrincipal != "" ||
		value.IntroductionSocket != "" || value.PublicationRoot != "" || value.LegacyGenerationFloor != "" || value.PersistentAdministration) {
		return errors.New("client plan contains publisher administration input")
	}
	if value.Role == "publisher" && (value.AdministrationSocket == "" || value.CredentialFile == "" ||
		value.InstanceKeyFile == "" || value.IntroductionSocket == "" || value.IntroductionPublic == "" ||
		value.PublicationRoot == "") {
		return errors.New("publisher plan lacks its administration input")
	}
	at, err := time.Parse(time.RFC3339, value.At)
	if err != nil {
		return err
	}
	deadline, err := time.ParseDuration(value.Deadline)
	if err != nil || deadline <= 0 || deadline > 15*time.Second {
		return errors.New("endpoint deadline is outside the frozen bound")
	}
	if _, err := value.connectionLifetime(deadline); err != nil {
		return err
	}
	fields := []bool{value.CandidateView != "", value.IsolationContext != "", value.DestinationBinding != "",
		value.RouteProfile != "", value.WorkSafetyNotAfter != 0, value.WorkSafetyMaximum != 0,
		value.NoNewRecoveryAfter != 0}
	enabled := fields[0]
	for _, present := range fields[1:] {
		if present != enabled {
			return errors.New("recovery binding is partial")
		}
	}
	if enabled && (value.RouteProfile != serviceconnection.Profile || value.WorkSafetyNotAfter <= at.Unix() ||
		value.WorkSafetyMaximum < value.WorkSafetyNotAfter || value.NoNewRecoveryAfter <= at.Unix() ||
		value.NoNewRecoveryAfter > value.WorkSafetyNotAfter) {
		return errors.New("recovery binding safety bounds are invalid")
	}
	return nil
}

func (value endpointPlan) connectionLifetime(deadline time.Duration) (time.Duration, error) {
	if value.Lifetime == "" {
		return deadline, nil
	}
	lifetime, err := time.ParseDuration(value.Lifetime)
	if err != nil || lifetime < deadline || lifetime > 30*time.Minute {
		return 0, errors.New("endpoint connection lifetime is outside the frozen development bound")
	}
	return lifetime, nil
}

func (value endpointPlan) validStreamBounds() bool {
	if value.SendBytes == 0 && value.ReceiveBytes == 0 {
		return value.BytesEachDirection > 0 && value.BytesEachDirection <= maximumEndpointStreamBytes
	}
	return value.BytesEachDirection == 0 && value.SendBytes <= maximumEndpointStreamBytes &&
		value.ReceiveBytes <= maximumEndpointStreamBytes &&
		(value.SendBytes > 0 || value.ReceiveBytes > 0)
}

func (value endpointPlan) recoveryEnabled() bool { return value.CandidateView != "" }

func (value endpointPlan) recoveryBinding() (serviceconnection.Recovery, error) {
	var binding serviceconnection.Recovery
	if !value.recoveryEnabled() {
		return binding, nil
	}
	for _, field := range []struct {
		encoded     string
		destination []byte
	}{{value.CandidateView, binding.CandidateView[:]}, {value.IsolationContext, binding.IsolationContext[:]},
		{value.DestinationBinding, binding.DestinationBinding[:]}} {
		if err := decodeEndpointFixedHex(field.encoded, field.destination); err != nil {
			return serviceconnection.Recovery{}, err
		}
	}
	binding.RouteProfile = value.RouteProfile
	binding.WorkSafetyNotAfter, binding.WorkSafetyMaximum = value.WorkSafetyNotAfter, value.WorkSafetyMaximum
	binding.NoNewRecoveryAfter = value.NoNewRecoveryAfter
	return binding, nil
}
func endpointSetup(plan endpointPlan) (Setup, time.Time, time.Duration, error) {
	var setup Setup
	for _, field := range []struct {
		encoded     string
		destination []byte
	}{
		{plan.NetworkID, setup.NetworkID[:]}, {plan.BrokerID, setup.BrokerID[:]},
		{plan.ConnectionPrincipal, setup.ConnectionPrincipal[:]}, {plan.AdministrationPrincipal, setup.AdministrationPrincipal[:]}} {
		if field.encoded != "" {
			if err := decodeEndpointFixedHex(field.encoded, field.destination); err != nil {
				return setup, time.Time{}, 0, err
			}
		}
	}
	setup.AuthorityPublic = make([]byte, ed25519.PublicKeySize)
	if err := decodeEndpointFixedHex(plan.AuthorityPublic, setup.AuthorityPublic); err != nil {
		return setup, time.Time{}, 0, err
	}
	setup.IntroductionPublic = make([]byte, ed25519.PublicKeySize)
	if err := decodeEndpointFixedHex(plan.IntroductionPublic, setup.IntroductionPublic); err != nil {
		return setup, time.Time{}, 0, err
	}
	setup.PublicationRoot, setup.LegacyGenerationFloor = plan.PublicationRoot, plan.LegacyGenerationFloor
	grants := []broker.Grant{{Principal: setup.ConnectionPrincipal, Surface: broker.Connection}}
	if setup.AdministrationPrincipal != [32]byte{} {
		grants = append(grants, broker.Grant{Principal: setup.AdministrationPrincipal, Surface: broker.Administration})
	}
	admission, err := broker.New(broker.Config{ID: setup.BrokerID, Grants: grants})
	if err != nil {
		return setup, time.Time{}, 0, err
	}
	setup.Admission = admission
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
