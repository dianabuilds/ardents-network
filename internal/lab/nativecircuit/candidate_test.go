package nativecircuit

import (
	"context"
	"crypto/ecdh"
	"crypto/rand"
	"testing"
	"time"
)

func TestNativeC5C2UsesSeparateIntroductionAndJoinedDataPaths(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	userEntry := newTestNode(t, "user-entry")
	userInterior := newTestNode(t, "user-interior")
	rendezvous := newTestNode(t, "rendezvous")
	serviceInterior := newTestNode(t, "service-interior")
	dataServiceEntry := newTestNode(t, "data-service-entry")
	introductionForwarder := newTestNode(t, "introduction-forwarder")
	introductionNode := newTestNode(t, "introduction-node")
	introductionInterior := newTestNode(t, "introduction-interior")
	introductionEntry := newTestNode(t, "introduction-entry")

	servers := make(chan error, 9)
	go func() {
		servers <- serveRelay(ctx, userEntry.listener, userEntry.certificate, []string{userInterior.address}, 2)
	}()
	go func() {
		servers <- serveRelay(ctx, userInterior.listener, userInterior.certificate, []string{rendezvous.address, introductionForwarder.address}, 2)
	}()
	go func() { servers <- serveRendezvous(ctx, rendezvous.listener, rendezvous.certificate, 2) }()
	go func() {
		servers <- serveRelay(ctx, serviceInterior.listener, serviceInterior.certificate, []string{rendezvous.address}, 1)
	}()
	go func() {
		servers <- serveRelay(ctx, dataServiceEntry.listener, dataServiceEntry.certificate, []string{serviceInterior.address}, 1)
	}()
	go func() {
		servers <- serveRelay(ctx, introductionForwarder.listener, introductionForwarder.certificate, []string{introductionNode.address}, 1)
	}()
	go func() { servers <- serveIntroduction(ctx, introductionNode.listener, introductionNode.certificate, 2) }()
	go func() {
		servers <- serveRelay(ctx, introductionInterior.listener, introductionInterior.certificate, []string{introductionNode.address}, 1)
	}()
	go func() {
		servers <- serveRelay(ctx, introductionEntry.listener, introductionEntry.certificate, []string{introductionInterior.address}, 1)
	}()

	hpkePrivate, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	endpointCertificate, endpointTrust := newEndpointFixture(t, "active-instance")
	slot := randomHandle(t)
	runID := "20260810T130000Z-native"
	serviceDone := make(chan endpointObservation, 1)
	serviceErr := make(chan error, 1)
	serviceRegistered := make(chan struct{})
	go func() {
		observation, err := runCandidateService(ctx, candidateServicePlan{
			Profile: candidateProfile, RunID: runID, Rendezvous: rendezvous.address, Slot: slot,
			IntroductionPath: []circuitHop{
				{Address: introductionEntry.address, CertificateSHA256: introductionEntry.digest},
				{Address: introductionInterior.address, CertificateSHA256: introductionInterior.digest},
				{Address: introductionNode.address, CertificateSHA256: introductionNode.digest},
			},
			DataPath: []circuitHop{
				{Address: dataServiceEntry.address, CertificateSHA256: dataServiceEntry.digest},
				{Address: serviceInterior.address, CertificateSHA256: serviceInterior.digest},
				{Address: rendezvous.address, CertificateSHA256: rendezvous.digest},
			},
			HPKEPrivate: hpkePrivate, EndpointCertificate: endpointCertificate,
			Registered: func() error { close(serviceRegistered); return nil },
		})
		serviceDone <- observation
		serviceErr <- err
	}()
	select {
	case <-serviceRegistered:
	case <-ctx.Done():
		t.Fatal("Service did not register its Introduction slot")
	}

	payload := []byte("native C-5/C2 verified Application stream")
	userObservation, err := runCandidateUser(ctx, candidateUserPlan{
		Profile: candidateProfile, RunID: runID, Rendezvous: rendezvous.address, Slot: slot,
		IntroductionPath: []circuitHop{
			{Address: userEntry.address, CertificateSHA256: userEntry.digest},
			{Address: userInterior.address, CertificateSHA256: userInterior.digest},
			{Address: introductionForwarder.address, CertificateSHA256: introductionForwarder.digest},
			{Address: introductionNode.address, CertificateSHA256: introductionNode.digest},
		},
		DataPath: []circuitHop{
			{Address: userEntry.address, CertificateSHA256: userEntry.digest},
			{Address: userInterior.address, CertificateSHA256: userInterior.digest},
			{Address: rendezvous.address, CertificateSHA256: rendezvous.digest},
		},
		HPKEPublic: hpkePrivate.PublicKey(), EndpointTrust: endpointTrust, Payload: payload,
	})
	if err != nil {
		t.Fatal(err)
	}
	serviceObservation := <-serviceDone
	if err := <-serviceErr; err != nil {
		t.Fatal(err)
	}
	if !userObservation.ApplicationBytesVerified || !serviceObservation.ApplicationBytesVerified {
		t.Fatalf("joined Application stream was not verified: user=%#v service=%#v", userObservation, serviceObservation)
	}
	for range 9 {
		if err := <-servers; err != nil {
			t.Fatal(err)
		}
	}
}
