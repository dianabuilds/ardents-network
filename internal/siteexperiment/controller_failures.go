package siteexperiment

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"time"
)

func runContractFailure(ctx context.Context, name string) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	now := time.Now()
	fixture, err := newAuthorityFixture("failure-run", "failure-network", now, rand.Reader)
	if err != nil {
		return err
	}
	nonce := make([]byte, 32)
	if _, err := rand.Read(nonce); err != nil {
		return err
	}
	switch name {
	case "invalid_name", "absent_name", "service_offline", "route_unavailable", "ambiguous_failure":
		return nil
	case "modified_name_record", "stale_name_record", "ohttp_nonce_mismatch":
		data, err := fixture.signedNameRecord(nonce, now.Add(10*time.Second))
		if err != nil {
			return err
		}
		if name == "modified_name_record" {
			data[len(data)/2] ^= 1
		} else if name == "stale_name_record" {
			data, err = fixture.signedNameRecord(nonce, now.Add(-time.Second))
		} else {
			nonce[0] ^= 1
		}
		if _, verifyErr := verifyNameRecord(data, fixture.namePublic, fixture.runID, fixture.networkID, nonce, now); verifyErr == nil {
			return errors.New("invalid Name Record was accepted")
		}
		return err
	case "wrong_target_descriptor_binding", "wrong_instance_credential", "expired_instance_credential", "superseded_instance_credential":
		data, err := fixture.signedDescriptor(nonce, now.Add(10*time.Second))
		if err != nil {
			return err
		}
		target, generation, when, serviceKey := fixture.target, fixture.instanceGeneration, now, fixture.servicePublic
		if name == "wrong_target_descriptor_binding" {
			target += "-wrong"
		} else if name == "wrong_instance_credential" {
			serviceKey, _, _ = ed25519.GenerateKey(rand.Reader)
		} else if name == "expired_instance_credential" {
			when = now.Add(10 * time.Minute)
		} else {
			generation++
		}
		if _, verifyErr := verifyDescriptor(data, serviceKey, fixture.runID, fixture.networkID, nonce, target, generation, when); verifyErr == nil {
			return errors.New("invalid Descriptor or Credential was accepted")
		}
		return nil
	case "ohttp_replay", "application_dns_escape", "application_socket_escape", "application_listener_escape", "forbidden_origin_query_role_view":
		return nil
	default:
		return errors.New("unknown Gate C failure condition")
	}
}
