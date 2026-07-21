package identity

import (
	"sync"

	"ardents/internal/storage"
)

type persistedIdentity struct {
	Principal string `json:"principal,omitempty"`
	Device    string `json:"device,omitempty"`
	PublicKey string `json:"public_key,omitempty"`
}

type persistedState struct {
	Identity persistedIdentity `json:"identity"`
}

type Store struct {
	mu   sync.Mutex
	path string
	data persistedState
}

func NewStore(path string) *Store {
	return &Store{path: path}
}

func NewStoreInDir(dir string) *Store {
	return NewStore(storage.PathInDir(dir))
}

func (s *Store) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.path == "" {
		return nil
	}
	var data persistedState
	found, err := storage.LoadJSON(s.path, "node-runtime", "state", &data)
	if err != nil {
		return err
	}
	if found {
		s.data = data
	}
	return nil
}

func (s *Store) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saveLocked()
}

func (s *Store) LoadIdentity() (string, string, string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	identity := s.data.Identity
	return identity.Principal, identity.Device, identity.PublicKey
}

func (s *Store) SaveIdentity(principal, device, publicKey string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.Identity = persistedIdentity{Principal: principal, Device: device, PublicKey: publicKey}
	return s.saveLocked()
}

func (s *Store) saveLocked() error {
	if s.path == "" {
		return nil
	}
	return storage.SaveJSON(s.path, "node-runtime", "state", s.data)
}
