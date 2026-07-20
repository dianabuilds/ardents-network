package recovery

import (
	"sync"

	db "ardents/internal/persistence"
)

type Identity struct {
	Principal string `json:"principal,omitempty"`
	Device    string `json:"device,omitempty"`
	PublicKey string `json:"public_key,omitempty"`
}

type snapshot struct {
	Identity Identity `json:"identity"`
}

type Store struct {
	mu   sync.Mutex
	path string
	data snapshot
}

func NewStore(path string) *Store {
	return &Store{
		path: path,
		data: snapshot{},
	}
}

func NewStoreInDir(dir string) *Store {
	return NewStore(db.PathInDir(dir))
}

func (s *Store) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.path == "" {
		return nil
	}

	var data snapshot
	found, err := db.LoadJSON(s.path, "node-runtime", "state", &data)
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

func (s *Store) Identity() Identity {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.data.Identity
}

func (s *Store) LoadIdentity() (string, string, string) {
	identity := s.Identity()
	return identity.Principal, identity.Device, identity.PublicKey
}

func (s *Store) SetIdentity(identity Identity) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.Identity = identity
	return s.saveLocked()
}

func (s *Store) SaveIdentity(principal string, device string, publicKey string) error {
	return s.SetIdentity(Identity{
		Principal: principal,
		Device:    device,
		PublicKey: publicKey,
	})
}

func (s *Store) saveLocked() error {
	if s.path == "" {
		return nil
	}
	return db.SaveJSON(s.path, "node-runtime", "state", s.data)
}
