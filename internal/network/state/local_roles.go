package state

import (
	"crypto/sha256"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/duty"
)

func (s *networkState) retainSourceExposures(notAfter time.Time) error {
	roles, err := duty.Open(duty.Config{Root: s.config.localRoles, Clock: s.config.clock, Create: true})
	if err != nil {
		return err
	}
	duties := make([]duty.Duty, len(s.config.sourceInfo.Identities))
	for index, identity := range s.config.sourceInfo.Identities {
		duties[index] = duty.Duty{Identity: identity,
			Family: sha256.Sum256([]byte(s.config.sourceInfo.Families[index])),
			Class:  "direct-source", State: "exposed", NotAfter: notAfter}
	}
	if err := roles.Replace(sourceProducer("exposure", s.config.root), duties); err != nil {
		return errors.Join(wrapSourceReplaceError(err), roles.Close())
	}
	return roles.Close()
}

func wrapSourceReplaceError(err error) error {
	if errors.Is(err, duty.ErrInstallationSourceExhausted) {
		return errors.New("source exposure set is full: " + err.Error())
	}
	return err
}

func (s *networkState) retainSourceServer() error {
	if s.current == nil {
		return errors.New("direct Source server has no current identity")
	}
	roles, err := duty.Open(duty.Config{Root: s.config.localRoles, Clock: s.config.clock, Create: true})
	if err != nil {
		return err
	}
	retained := duty.Duty{Identity: s.current.NodeID, Family: sha256.Sum256([]byte(s.current.DeclaredFamily)),
		Class: "direct-source", State: "live", NotAfter: s.current.ValidUntil}
	return errors.Join(roles.Replace(sourceProducer("server", s.config.root), []duty.Duty{retained}), roles.Close())
}

func (s *networkState) releaseSourceServer() error {
	if !s.config.sourceInfo.Serving {
		return nil
	}
	roles, err := duty.Open(duty.Config{Root: s.config.localRoles, Clock: s.config.clock})
	if err != nil {
		return err
	}
	return errors.Join(roles.Remove(sourceProducer("server", s.config.root)), roles.Close())
}

func sourceProducer(kind, root string) [32]byte {
	return sha256.Sum256([]byte("ardents-h3-local-source-" + kind + "-v1\x00" + root))
}
