package transit

import (
	"errors"
	"path/filepath"

	"github.com/dianabuilds/ardents-network/internal/route"
)

// Set gives each adjacent transit attachment its own
// at-most-once request/key journal. The root itself remains the Introduction
// owner for v1 compatibility; Responder state has a distinct child root and
// lease.
type Set struct {
	introduction *Acquisition
	responder    *Acquisition
}

func OpenSet(config Config) (*Set, error) {
	introduction, err := openAcquisition(config)
	if err != nil {
		return nil, err
	}
	responder, err := openAcquisition(Config{
		Root: filepath.Join(config.Root, "responder"), Create: config.Create, Clock: config.Clock,
	})
	if err != nil {
		return nil, errors.Join(err, introduction.Close())
	}
	return &Set{introduction: introduction, responder: responder}, nil
}

func (owners *Set) Owner(role byte) (*Acquisition, error) {
	if owners == nil {
		return nil, errors.New("endpoint transit acquisition owners are unavailable")
	}
	switch role {
	case route.IntroductionRole:
		return owners.introduction, nil
	case route.ResponderRole:
		return owners.responder, nil
	default:
		return nil, errors.New("endpoint transit acquisition role is unsupported")
	}
}

func (owners *Set) Close() error {
	if owners == nil {
		return nil
	}
	return errors.Join(owners.introduction.Close(), owners.responder.Close())
}
