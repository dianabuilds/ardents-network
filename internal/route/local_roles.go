package route

import (
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/localroles"
)

func retainLocalRoute(input Actor, terminal time.Time) ([32]byte, error) {
	if input.Role != "client" {
		return [32]byte{}, nil
	}
	if input.LocalRoleStateRoot == "" || len(input.Plan.Positions) != 4 || input.Deadline <= 0 {
		return [32]byte{}, errors.New("client local-role projection is incomplete")
	}
	roles, err := localroles.Open(localroles.Config{Root: input.LocalRoleStateRoot, Clock: time.Now, Create: true})
	if err != nil {
		return [32]byte{}, err
	}
	duties := make([]localroles.Duty, len(input.Plan.Positions))
	for index, position := range input.Plan.Positions {
		class, ok := routeDutyClass(position.Role)
		if !ok {
			_ = roles.Close()
			return [32]byte{}, errors.New("client Route has an unknown local duty")
		}
		duties[index] = localroles.Duty{Identity: position.NodeID, Family: sha256.Sum256([]byte(position.Family)),
			Class: class, State: "live", NotAfter: terminal}
	}
	var nonce [32]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		_ = roles.Close()
		return [32]byte{}, errors.New("create local Route producer ID")
	}
	seed := append([]byte("ardents-h3-local-route-producer-v1\x00"), input.ManifestDigest[:]...)
	seed = append(seed, input.Plan.Seed[:]...)
	producer := sha256.Sum256(append(seed, nonce[:]...))
	return producer, errors.Join(roles.Replace(producer, duties), roles.Close())
}

func releaseLocalRoute(input Actor, producer [32]byte) error {
	if producer == ([32]byte{}) {
		return nil
	}
	roles, err := localroles.Open(localroles.Config{Root: input.LocalRoleStateRoot, Clock: time.Now})
	if err != nil {
		return err
	}
	return errors.Join(roles.Remove(producer), roles.Close())
}

func routeDutyClass(role string) (string, bool) {
	switch role {
	case "initiator":
		return "ordinary-initiator", true
	case "introduction":
		return "route-introduction", true
	case "rendezvous":
		return "route-rendezvous", true
	case "responder":
		return "route-responder", true
	default:
		return "", false
	}
}
