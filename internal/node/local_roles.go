package node

import (
	"crypto/sha256"
	"errors"

	"github.com/dianabuilds/ardents-network/internal/network/duty"
)

func retainLocalDuty(config runtimeConfig, snapshot dutyFacts, state string) error {
	roles, err := duty.Open(duty.Config{Root: config.LocalRoleStateRoot, Clock: config.now, Create: true})
	if err != nil {
		return err
	}
	class := "node-duty"
	switch snapshot.Assignment {
	case "initiator":
		class = "ordinary-initiator"
	case "rendezvous":
		class = "route-rendezvous"
	case "introduction":
		class = "route-introduction"
	case "responder":
		class = "route-responder"
	}
	notAfter := snapshot.ValidUntil
	if snapshot.RecordValidUntil.Before(notAfter) {
		notAfter = snapshot.RecordValidUntil
	}
	retained := duty.Duty{Identity: snapshot.NodeID, Family: sha256.Sum256([]byte(snapshot.DeclaredFamily)),
		Class: class, State: state, NotAfter: notAfter}
	return errors.Join(roles.Replace(localDutyProducer(config.NodeID), []duty.Duty{retained}), roles.Close())
}

func releaseLocalDuty(config runtimeConfig) error {
	roles, err := duty.Open(duty.Config{Root: config.LocalRoleStateRoot, Clock: config.now})
	if err != nil {
		return err
	}
	return errors.Join(roles.Remove(localDutyProducer(config.NodeID)), roles.Close())
}

func localDutyProducer(identity [32]byte) [32]byte {
	return sha256.Sum256(append([]byte("ardents-h3-local-node-producer-v1\x00"), identity[:]...))
}
